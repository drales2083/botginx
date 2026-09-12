package services

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/botginx/botginx/modules/payments/models"
	"github.com/jmoiron/sqlx"
)

// Price cache to avoid hitting API on every conversion
var (
	priceCache     = make(map[string]float64)
	priceCacheMu   sync.RWMutex
	priceCacheTime time.Time
	priceCacheTTL  = 5 * time.Minute
)

type PaymentService struct {
	db       *sqlx.DB
	registry *GatewayRegistry
}

func NewPaymentService(db *sqlx.DB) *PaymentService {
	registry := NewGatewayRegistry()

	// Register available gateways
	registry.Register(NewBitGoGateway())
	// Future: registry.Register(NewTronGateway())
	// Future: registry.Register(NewCoinbaseGateway())

	return &PaymentService{
		db:       db,
		registry: registry,
	}
}

func (s *PaymentService) generateID() string {
	b := make([]byte, 13)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// DB returns the database connection
func (s *PaymentService) DB() *sqlx.DB {
	return s.db
}

// GetOrCreateWallet gets existing wallet or creates new one for user
func (s *PaymentService) GetOrCreateWallet(userID, coin string) (*models.CryptoWallet, error) {
	// Check for existing wallet
	var wallet models.CryptoWallet
	err := s.db.Get(&wallet, `
		SELECT * FROM crypto_wallets
		WHERE user_id = $1 AND coin = $2
	`, userID, coin)

	if err == nil {
		return &wallet, nil
	}

	// Find gateway for this coin
	gateway := s.registry.GetForCoin(coin)
	if gateway == nil {
		return nil, fmt.Errorf("no gateway available for coin: %s", coin)
	}

	// Create new address
	label := fmt.Sprintf("User %s Wallet", userID)
	address, err := gateway.CreateAddress(userID, coin, label)
	if err != nil {
		return nil, fmt.Errorf("failed to create address: %w", err)
	}

	// Save to database
	wallet = models.CryptoWallet{
		ID:        s.generateID(),
		UserID:    userID,
		WalletID:  s.getGatewayWalletID(gateway),
		Address:   address,
		Coin:      coin,
		Label:     label,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err = s.db.NamedExec(`
		INSERT INTO crypto_wallets (id, user_id, wallet_id, address, coin, label, created_at, updated_at)
		VALUES (:id, :user_id, :wallet_id, :address, :coin, :label, :created_at, :updated_at)
	`, wallet)

	if err != nil {
		return nil, fmt.Errorf("failed to save wallet: %w", err)
	}

	return &wallet, nil
}

// GetExistingWallet retrieves a wallet if it exists (does not create)
func (s *PaymentService) GetExistingWallet(userID, coin string) (*models.CryptoWallet, error) {
	var wallet models.CryptoWallet
	err := s.db.Get(&wallet, `
		SELECT * FROM crypto_wallets
		WHERE user_id = $1 AND coin = $2
	`, userID, coin)
	if err != nil {
		return nil, err
	}
	return &wallet, nil
}

// GetWalletByAddress finds wallet by deposit address
func (s *PaymentService) GetWalletByAddress(address string) (*models.CryptoWallet, error) {
	var wallet models.CryptoWallet
	err := s.db.Get(&wallet, `SELECT * FROM crypto_wallets WHERE address = $1`, address)
	if err != nil {
		return nil, err
	}
	return &wallet, nil
}

// ProcessDeposit handles incoming deposit from webhook
func (s *PaymentService) ProcessDeposit(address, txid string, amountCrypto float64, confirmations int) (*models.CryptoTransaction, error) {
	// Find wallet by address
	wallet, err := s.GetWalletByAddress(address)
	if err != nil {
		return nil, fmt.Errorf("address not found: %s", address)
	}

	// Check if transaction already exists
	var tx models.CryptoTransaction
	err = s.db.Get(&tx, `SELECT * FROM crypto_transactions WHERE txid = $1`, txid)

	// Convert to USD
	amountUSD := s.convertToUSD(wallet.Coin, amountCrypto)

	// Don't create transaction if price lookup failed (would credit $0)
	if err != nil && amountUSD <= 0 {
		return nil, fmt.Errorf("price lookup failed for %s, cannot process deposit", wallet.Coin)
	}

	if err != nil {
		// Create new transaction
		tx = models.CryptoTransaction{
			ID:            s.generateID(),
			UserID:        wallet.UserID,
			WalletID:      wallet.ID,
			TxID:          txid,
			Amount:        amountCrypto,
			AmountUSD:     amountUSD,
			Confirmations: confirmations,
			Status:        models.TxStatusPending,
			Type:          models.TxTypeDeposit,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}

		_, err = s.db.NamedExec(`
			INSERT INTO crypto_transactions (id, user_id, wallet_id, txid, amount, amount_usd, confirmations, status, type, created_at, updated_at)
			VALUES (:id, :user_id, :wallet_id, :txid, :amount, :amount_usd, :confirmations, :status, :type, :created_at, :updated_at)
		`, tx)

		if err != nil {
			return nil, fmt.Errorf("failed to save transaction: %w", err)
		}

		log.Printf("[payments] New deposit: txid=%s user=%s amount=%.8f %s ($%.2f)", txid, wallet.UserID, amountCrypto, wallet.Coin, amountUSD)
	} else {
		// Update confirmations
		tx.Confirmations = confirmations
		tx.UpdatedAt = time.Now()
		s.db.Exec(`UPDATE crypto_transactions SET confirmations = $1, updated_at = $2 WHERE id = $3`,
			confirmations, tx.UpdatedAt, tx.ID)
	}

	// Check if ready to credit
	threshold := s.getConfirmationThreshold()
	if tx.Status == models.TxStatusPending && confirmations >= threshold {
		// Use tx.AmountUSD (stored value) not amountUSD (recalculated) to ensure
		// user is credited the amount from when deposit was first recorded
		if err := s.creditUser(wallet.UserID, tx.AmountUSD); err != nil {
			log.Printf("[payments] Failed to credit user %s: %v", wallet.UserID, err)
			return &tx, err
		}

		tx.Status = models.TxStatusConfirmed
		s.db.Exec(`UPDATE crypto_transactions SET status = $1, updated_at = $2 WHERE id = $3`,
			models.TxStatusConfirmed, time.Now(), tx.ID)

		log.Printf("[payments] Deposit confirmed: txid=%s user=%s credited=$%.2f", txid, wallet.UserID, tx.AmountUSD)
	}

	return &tx, nil
}

// creditUser adds balance to user account
func (s *PaymentService) creditUser(userID string, amountUSD float64) error {
	result, err := s.db.Exec(`
		UPDATE users SET balance = COALESCE(balance, 0) + $1 WHERE id = $2
	`, amountUSD, userID)

	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return errors.New("user not found")
	}

	// Record balance transaction
	s.db.Exec(`
		INSERT INTO balance_transactions (id, user_id, amount, type, description, created_at)
		VALUES ($1, $2, $3, 'credit', 'Crypto deposit', $4)
	`, s.generateID(), userID, amountUSD, time.Now())

	return nil
}

// GetUserTransactions returns deposit history for a user
func (s *PaymentService) GetUserTransactions(userID string, limit int) ([]models.CryptoTransaction, error) {
	var txs []models.CryptoTransaction
	err := s.db.Select(&txs, `
		SELECT * FROM crypto_transactions
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, userID, limit)
	return txs, err
}

// GetAllTransactions returns all transactions (admin)
func (s *PaymentService) GetAllTransactions(limit int) ([]models.CryptoTransaction, error) {
	var txs []models.CryptoTransaction
	err := s.db.Select(&txs, `
		SELECT * FROM crypto_transactions
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	return txs, err
}

// BalanceTransaction represents a balance change record
type BalanceTransaction struct {
	ID          string    `db:"id" json:"id"`
	UserID      string    `db:"user_id" json:"userId"`
	Amount      float64   `db:"amount" json:"amount"`
	Type        string    `db:"type" json:"type"`
	Description string    `db:"description" json:"description"`
	CreatedAt   time.Time `db:"created_at" json:"createdAt"`
}

// GetBalanceHistory returns all balance transactions for a user
func (s *PaymentService) GetBalanceHistory(userID string, limit int) ([]BalanceTransaction, error) {
	var txs []BalanceTransaction
	err := s.db.Select(&txs, `
		SELECT id, user_id, amount, type, description, created_at
		FROM balance_transactions
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, userID, limit)
	return txs, err
}

// GetTransaction returns a single transaction by ID
func (s *PaymentService) GetTransaction(txID string) (*models.CryptoTransaction, error) {
	var tx models.CryptoTransaction
	err := s.db.Get(&tx, `SELECT * FROM crypto_transactions WHERE id = $1 OR txid = $1`, txID)
	if err != nil {
		return nil, err
	}
	return &tx, nil
}

// GetAvailableCoins returns list of supported coins
func (s *PaymentService) GetAvailableCoins() []string {
	return s.registry.ListAllCoins()
}

// GetGatewayForCoin returns the gateway handling a specific coin
func (s *PaymentService) GetGatewayForCoin(coin string) PaymentGateway {
	return s.registry.GetForCoin(coin)
}

// GetBitGoGateway returns the BitGo gateway (for webhook handling)
func (s *PaymentService) GetBitGoGateway() *BitGoGateway {
	gw := s.registry.Get("bitgo")
	if gw != nil {
		return gw.(*BitGoGateway)
	}
	return nil
}

// Helper functions

func (s *PaymentService) getGatewayWalletID(gateway PaymentGateway) string {
	// Get wallet ID based on gateway type
	switch g := gateway.(type) {
	case *BitGoGateway:
		return g.GetClient().GetWalletID()
	default:
		return ""
	}
}

func (s *PaymentService) convertToUSD(coin string, amount float64) float64 {
	// Stablecoins are 1:1
	if coin == "usdt" || coin == "usdc" {
		return amount
	}

	// Check env override first
	envKey := fmt.Sprintf("%s_USD_PRICE", strings.ToUpper(coin))
	if priceStr := os.Getenv(envKey); priceStr != "" {
		if price, err := strconv.ParseFloat(priceStr, 64); err == nil {
			return amount * price
		}
	}

	// Fetch live price from CoinGecko (cached)
	price := fetchCryptoPrice(coin)
	return amount * price
}

// fetchCryptoPrice gets the USD price for a coin, trying multiple providers
func fetchCryptoPrice(coin string) float64 {
	// Map coin symbols to CoinGecko IDs
	coinIDs := map[string]string{
		"btc":  "bitcoin",
		"tbtc": "bitcoin", // testnet uses mainnet price
		"ltc":  "litecoin",
		"eth":  "ethereum",
		"doge": "dogecoin",
		"trx":  "tron",
	}

	coinLower := strings.ToLower(coin)
	cgID, ok := coinIDs[coinLower]
	if !ok {
		log.Printf("[payments] Unknown coin for price lookup: %s", coin)
		return 0
	}

	// Check cache
	priceCacheMu.RLock()
	if time.Since(priceCacheTime) < priceCacheTTL {
		if price, exists := priceCache[coinLower]; exists {
			priceCacheMu.RUnlock()
			return price
		}
	}
	priceCacheMu.RUnlock()

	client := &http.Client{Timeout: 10 * time.Second}

	// Try CoinGecko first (supports all coins)
	if price := fetchFromCoinGecko(client, cgID); price > 0 {
		cachePrice(coinLower, price, "CoinGecko")
		return price
	}

	// Fallback to BitPay (BTC only)
	if coinLower == "btc" || coinLower == "tbtc" {
		if price := fetchFromBitPay(client); price > 0 {
			cachePrice(coinLower, price, "BitPay")
			return price
		}
	}

	// Fallback to CoinDesk (BTC only)
	if coinLower == "btc" || coinLower == "tbtc" {
		if price := fetchFromCoinDesk(client); price > 0 {
			cachePrice(coinLower, price, "CoinDesk")
			return price
		}
	}

	log.Printf("[payments] All price providers failed for: %s", coin)
	return 0
}

// cachePrice stores price and logs the source
func cachePrice(coin string, price float64, source string) {
	priceCacheMu.Lock()
	priceCache[coin] = price
	priceCacheTime = time.Now()
	priceCacheMu.Unlock()
	log.Printf("[payments] Fetched %s price: $%.2f (from %s)", coin, price, source)
}

// fetchFromCoinGecko fetches price from CoinGecko API (no key required)
func fetchFromCoinGecko(client *http.Client, cgID string) float64 {
	url := fmt.Sprintf("https://api.coingecko.com/api/v3/simple/price?ids=%s&vs_currencies=usd", cgID)
	resp, err := client.Get(url)
	if err != nil {
		log.Printf("[payments] CoinGecko error: %v", err)
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("[payments] CoinGecko status: %d", resp.StatusCode)
		return 0
	}

	var result map[string]map[string]float64
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[payments] CoinGecko parse error: %v", err)
		return 0
	}

	if price, ok := result[cgID]["usd"]; ok && price > 0 {
		return price
	}
	return 0
}

// fetchFromBitPay fetches BTC price from BitPay API (no key required)
func fetchFromBitPay(client *http.Client) float64 {
	resp, err := client.Get("https://bitpay.com/api/rates")
	if err != nil {
		log.Printf("[payments] BitPay error: %v", err)
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("[payments] BitPay status: %d", resp.StatusCode)
		return 0
	}

	var rates []struct {
		Code string  `json:"code"`
		Rate float64 `json:"rate"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rates); err != nil {
		log.Printf("[payments] BitPay parse error: %v", err)
		return 0
	}

	for _, r := range rates {
		if r.Code == "USD" {
			return r.Rate
		}
	}
	return 0
}

// fetchFromCoinDesk fetches BTC price from CoinDesk API (no key required)
func fetchFromCoinDesk(client *http.Client) float64 {
	resp, err := client.Get("https://api.coindesk.com/v1/bpi/currentprice/USD.json")
	if err != nil {
		log.Printf("[payments] CoinDesk error: %v", err)
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("[payments] CoinDesk status: %d", resp.StatusCode)
		return 0
	}

	var result struct {
		BPI struct {
			USD struct {
				RateFloat float64 `json:"rate_float"`
			} `json:"USD"`
		} `json:"bpi"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[payments] CoinDesk parse error: %v", err)
		return 0
	}

	return result.BPI.USD.RateFloat
}

func (s *PaymentService) getConfirmationThreshold() int {
	if threshold := os.Getenv("DEPOSIT_CONFIRMATIONS"); threshold != "" {
		if n, err := strconv.Atoi(threshold); err == nil {
			return n
		}
	}
	return 1 // default 1 confirmation
}

// GetMinDeposit returns minimum deposit in USD
func (s *PaymentService) GetMinDeposit() float64 {
	if min := os.Getenv("DEPOSIT_MIN_USD"); min != "" {
		if n, err := strconv.ParseFloat(min, 64); err == nil {
			return n
		}
	}
	return 10 // default $10
}

// GetMaxDeposit returns maximum deposit in USD
func (s *PaymentService) GetMaxDeposit() float64 {
	if max := os.Getenv("DEPOSIT_MAX_USD"); max != "" {
		if n, err := strconv.ParseFloat(max, 64); err == nil {
			return n
		}
	}
	return 500000 // default $500k
}

// IsPaymentEnabled returns true if any payment gateway is configured
func (s *PaymentService) IsPaymentEnabled() bool {
	return len(s.registry.ListAvailable()) > 0
}
