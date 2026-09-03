package models

import "time"

// CryptoWallet represents a user's crypto deposit address
type CryptoWallet struct {
	ID        string    `db:"id" json:"id"`
	UserID    string    `db:"user_id" json:"userId"`
	WalletID  string    `db:"wallet_id" json:"walletId"`
	Address   string    `db:"address" json:"address"`
	Coin      string    `db:"coin" json:"coin"`
	Label     string    `db:"label" json:"label"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt time.Time `db:"updated_at" json:"updatedAt"`
}

// CryptoTransaction represents a deposit or withdrawal
type CryptoTransaction struct {
	ID            string    `db:"id" json:"id"`
	UserID        string    `db:"user_id" json:"userId"`
	WalletID      string    `db:"wallet_id" json:"walletId"`
	TxID          string    `db:"txid" json:"txid"`
	Amount        float64   `db:"amount" json:"amount"`
	AmountUSD     float64   `db:"amount_usd" json:"amountUsd"`
	Confirmations int       `db:"confirmations" json:"confirmations"`
	Status        string    `db:"status" json:"status"` // pending, confirmed, failed
	Type          string    `db:"type" json:"type"`     // deposit, withdrawal
	CreatedAt     time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt     time.Time `db:"updated_at" json:"updatedAt"`
}

const (
	TxStatusPending   = "pending"
	TxStatusConfirmed = "confirmed"
	TxStatusFailed    = "failed"

	TxTypeDeposit    = "deposit"
	TxTypeWithdrawal = "withdrawal"
)

// BitGo webhook payload
type BitGoWebhookPayload struct {
	Hash     string `json:"hash"`
	Transfer string `json:"transfer"`
	Coin     string `json:"coin"`
	Type     string `json:"type"`
	State    string `json:"state"`
	Wallet   string `json:"wallet"`
}

// BitGo transaction details
type BitGoTxDetails struct {
	ID            string          `json:"id"`
	Confirmations int             `json:"confirmations"`
	Outputs       []BitGoTxOutput `json:"outputs"`
	FromWallet    string          `json:"fromWallet"`
	Error         string          `json:"error,omitempty"`
}

type BitGoTxOutput struct {
	Address string `json:"address"`
	Value   int64  `json:"value"` // satoshis
	Wallet  string `json:"wallet"`
}

// BitGo create address response
type BitGoAddressResponse struct {
	Address string `json:"address"`
	Error   string `json:"error,omitempty"`
}
