package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/botginx/botginx/modules/payments/models"
	"github.com/jmoiron/sqlx"
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
		if err := s.creditUser(wallet.UserID, amountUSD); err != nil {
			log.Printf("[payments] Failed to credit user %s: %v", wallet.UserID, err)
			return &tx, err
		}

		tx.Status = models.TxStatusConfirmed
		s.db.Exec(`UPDATE crypto_transactions SET status = $1, updated_at = $2 WHERE id = $3`,
			models.TxStatusConfirmed, time.Now(), tx.ID)

		log.Printf("[payments] Deposit confirmed: txid=%s user=%s credited=$%.2f", txid, wallet.UserID, amountUSD)
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
	// TODO: Integrate with price API (CoinGecko, etc.)
	// For now, use rough estimates or env var
	switch coin {
	case "btc", "tbtc":
		priceStr := os.Getenv("BTC_USD_PRICE")
		if price, err := strconv.ParseFloat(priceStr, 64); err == nil {
			return amount * price
		}
		return amount * 60000 // fallback
	case "usdt":
		return amount // 1:1
	case "ltc":
		return amount * 80 // rough estimate
	default:
		return amount
	}
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
