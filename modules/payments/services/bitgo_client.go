package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/botginx/botginx/modules/payments/models"
)

const (
	BitGoProductionURL = "https://www.bitgo.com/api/v2"
	BitGoTestnetURL    = "https://test.bitgo.com/api/v2"
)

type BitGoClient struct {
	apiKey   string
	walletID string
	baseURL  string
	coin     string
	serverIP string
	http     *http.Client
}

func NewBitGoClient() *BitGoClient {
	testnet := os.Getenv("BITGO_TESTNET") == "true"
	coin := "btc"
	if testnet {
		coin = "tbtc"
	}

	baseURL := BitGoProductionURL
	if testnet {
		baseURL = BitGoTestnetURL
	}

	serverIP := os.Getenv("BITGO_SERVER_IP")

	// Create HTTP client with optional IP binding
	httpClient := &http.Client{Timeout: 30 * time.Second}

	// If server IP is set and not testnet, bind outgoing requests to that IP
	// This is required when BitGo has IP whitelisting enabled
	if serverIP != "" && !testnet {
		dialer := &net.Dialer{
			LocalAddr: &net.TCPAddr{IP: net.ParseIP(serverIP)},
			Timeout:   30 * time.Second,
		}
		transport := &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.DialContext(ctx, network, addr)
			},
		}
		httpClient = &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		}
	}

	return &BitGoClient{
		apiKey:   os.Getenv("BITGO_API_KEY"),
		walletID: os.Getenv("BITGO_WALLET_ID"),
		baseURL:  baseURL,
		coin:     coin,
		serverIP: serverIP,
		http:     httpClient,
	}
}

// CreateWalletAddress creates a new deposit address on BitGo
func (c *BitGoClient) CreateWalletAddress(label string) (*models.BitGoAddressResponse, error) {
	url := fmt.Sprintf("%s/%s/wallet/%s/address", c.baseURL, c.coin, c.walletID)

	payload := map[string]interface{}{
		"label": label,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	c.setHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result models.BitGoAddressResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetTransaction fetches transaction details from BitGo
func (c *BitGoClient) GetTransaction(txid string) (*models.BitGoTxDetails, error) {
	url := fmt.Sprintf("%s/%s/wallet/%s/tx/%s", c.baseURL, c.coin, c.walletID, txid)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Log raw response for debugging deposit issues
	log.Printf("[bitgo] Raw tx response for %s: %s", txid, string(respBody))

	var result models.BitGoTxDetails
	if err := json.Unmarshal(respBody, &result); err != nil {
		log.Printf("[bitgo] Failed to parse tx %s: %v", txid, err)
		return nil, err
	}

	// Log parsed outputs
	for i, out := range result.Outputs {
		log.Printf("[bitgo] tx=%s output[%d]: addr=%s value=%d wallet=%s",
			txid, i, out.Address, out.Value, out.Wallet)
	}

	return &result, nil
}

// ListTransfers fetches recent incoming transfers from BitGo
func (c *BitGoClient) ListTransfers() ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/%s/wallet/%s/transfer?limit=50", c.baseURL, c.coin, c.walletID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Transfers []map[string]interface{} `json:"transfers"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	return result.Transfers, nil
}

// AddWebhook registers a webhook URL with BitGo
func (c *BitGoClient) AddWebhook(webhookURL string, numConfirmations int) error {
	url := fmt.Sprintf("%s/%s/wallet/%s/webhooks", c.baseURL, c.coin, c.walletID)

	payload := map[string]interface{}{
		"url":              webhookURL,
		"type":             "transfer",
		"numConfirmations": numConfirmations,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	c.setHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("BitGo webhook error: %s", string(respBody))
	}

	return nil
}

// ListWebhooks lists all webhooks for the wallet
func (c *BitGoClient) ListWebhooks() ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/%s/wallet/%s/webhooks", c.baseURL, c.coin, c.walletID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Webhooks []map[string]interface{} `json:"webhooks"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	return result.Webhooks, nil
}

func (c *BitGoClient) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
}

// ToBTC converts satoshis to BTC
func ToBTC(satoshi int64) float64 {
	return float64(satoshi) / 100000000
}

// ToSatoshi converts BTC to satoshis
func ToSatoshi(btc float64) int64 {
	return int64(btc * 100000000)
}

// GetWalletID returns the configured wallet ID
func (c *BitGoClient) GetWalletID() string {
	return c.walletID
}

// GetCoin returns the coin type (btc or tbtc)
func (c *BitGoClient) GetCoin() string {
	return c.coin
}

// IsConfigured returns true if BitGo is configured
func (c *BitGoClient) IsConfigured() bool {
	return c.apiKey != "" && c.walletID != ""
}
