package services

import (
	"log"
	"os"
)

// BitGoGateway implements PaymentGateway for BitGo
type BitGoGateway struct {
	client *BitGoClient
}

func NewBitGoGateway() *BitGoGateway {
	return &BitGoGateway{
		client: NewBitGoClient(),
	}
}

func (g *BitGoGateway) GetName() string {
	return "bitgo"
}

func (g *BitGoGateway) GetSupportedCoins() []string {
	if os.Getenv("BITGO_TESTNET") == "true" {
		return []string{"tbtc"} // testnet bitcoin
	}
	return []string{"btc"}
}

func (g *BitGoGateway) CreateAddress(userID, coin, label string) (string, error) {
	resp, err := g.client.CreateWalletAddress(label)
	if err != nil {
		return "", err
	}
	if resp.Error != "" {
		return "", &GatewayError{Message: resp.Error}
	}
	return resp.Address, nil
}

func (g *BitGoGateway) GetTransaction(txid string) (*GatewayTransaction, error) {
	tx, err := g.client.GetTransaction(txid)
	if err != nil {
		return nil, err
	}
	if tx.Error != "" {
		return nil, &GatewayError{Message: tx.Error}
	}

	result := &GatewayTransaction{
		TxID:          tx.ID,
		Confirmations: tx.Confirmations,
		FromExternal:  tx.FromWallet == "" || tx.FromWallet == "external",
		Outputs:       make([]GatewayOutput, 0),
	}

	walletID := g.client.GetWalletID()
	for _, out := range tx.Outputs {
		isOurs := out.Wallet == walletID && out.Wallet != tx.FromWallet
		amountBTC := ToBTC(out.Value)
		log.Printf("[bitgo] Output conversion: %d satoshis -> %.8f BTC, isOurs=%v (wallet=%s, ourWallet=%s, fromWallet=%s)",
			out.Value, amountBTC, isOurs, out.Wallet, walletID, tx.FromWallet)
		result.Outputs = append(result.Outputs, GatewayOutput{
			Address: out.Address,
			Amount:  amountBTC,
			IsOurs:  isOurs,
		})
	}

	return result, nil
}

func (g *BitGoGateway) IsConfigured() bool {
	return g.client.IsConfigured()
}

// GetClient returns the underlying BitGo client (for webhook handling)
func (g *BitGoGateway) GetClient() *BitGoClient {
	return g.client
}

// GatewayError for gateway-specific errors
type GatewayError struct {
	Message string
}

func (e *GatewayError) Error() string {
	return e.Message
}
