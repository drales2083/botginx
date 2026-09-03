# BitGo Payment Gateway Implementation Plan

## Overview

Port the BitGo cryptocurrency payment gateway from jurassicws to botginx, enabling users to deposit BTC and have their balance credited automatically.

## Architecture

```
User Dashboard                    BitGo Cloud                      Botginx Backend
     │                                 │                                 │
     │  1. Request deposit address     │                                 │
     ├────────────────────────────────────────────────────────────────────►
     │                                 │                                 │
     │                                 │  2. Create wallet address       │
     │                                 ◄─────────────────────────────────┤
     │                                 │                                 │
     │  3. Return BTC address          │                                 │
     ◄────────────────────────────────────────────────────────────────────┤
     │                                 │                                 │
     │  4. User sends BTC              │                                 │
     ├─────────────────────────────────►                                 │
     │                                 │                                 │
     │                                 │  5. Webhook: transaction event  │
     │                                 ├─────────────────────────────────►
     │                                 │                                 │
     │                                 │  6. Fetch tx details            │
     │                                 ◄─────────────────────────────────┤
     │                                 │                                 │
     │                                 │                                 │ 7. Credit user balance
     │                                 │                                 │    + record transaction
     │                                 │                                 │
     │  8. Balance updated (toast/notification)                          │
     ◄────────────────────────────────────────────────────────────────────┤
```

## Module Structure

```
modules/payments/
├── handlers/
│   └── handler.go           # HTTP handlers (deposit page, webhook, API)
├── services/
│   ├── payment_service.go   # Business logic
│   └── bitgo_client.go      # BitGo API client
├── models/
│   └── models.go            # Wallet, Transaction structs
├── migrations/
│   └── 001_create_tables.sql
├── templates/
│   ├── deposit.html         # User deposit page
│   └── transactions.html    # Transaction history
└── module.go                # Module registration
```

## Database Schema

### Table: `crypto_wallets`

| Column | Type | Description |
|--------|------|-------------|
| id | VARCHAR(26) | Primary key |
| user_id | VARCHAR(36) | FK to users |
| wallet_id | VARCHAR(64) | BitGo wallet ID |
| address | VARCHAR(128) | BTC deposit address |
| coin | VARCHAR(10) | Cryptocurrency (btc, ltc, etc.) |
| label | VARCHAR(255) | Human-readable label |
| created_at | TIMESTAMP | |
| updated_at | TIMESTAMP | |

**Indexes:** user_id, address (unique), coin

### Table: `crypto_transactions`

| Column | Type | Description |
|--------|------|-------------|
| id | VARCHAR(26) | Primary key |
| user_id | VARCHAR(36) | FK to users |
| wallet_id | VARCHAR(26) | FK to crypto_wallets |
| txid | VARCHAR(128) | Blockchain transaction hash |
| amount | DECIMAL(18,8) | Amount in crypto |
| amount_usd | DECIMAL(10,2) | USD value at time of deposit |
| confirmations | INTEGER | Number of confirmations |
| status | VARCHAR(20) | pending, confirmed, failed |
| type | VARCHAR(20) | deposit, withdrawal |
| created_at | TIMESTAMP | |
| updated_at | TIMESTAMP | |

**Indexes:** user_id, txid (unique), status

## Environment Variables

```env
# BitGo Configuration
BITGO_API_KEY=v2x...           # BitGo API access token
BITGO_WALLET_ID=6800d33f...    # Main wallet ID
BITGO_TESTNET=false            # true for test environment
BITGO_SERVER_IP=               # Optional: IP whitelist for requests
BITGO_WEBHOOK_SECRET=          # Webhook signature verification

# Deposit Settings
DEPOSIT_MIN_USD=10             # Minimum deposit amount
DEPOSIT_MAX_USD=500000         # Maximum deposit amount
DEPOSIT_CONFIRMATIONS=1        # Required confirmations before credit
```

## API Endpoints

### User Endpoints (require auth)

| Method | Path | Description |
|--------|------|-------------|
| GET | /user/payments | Deposit page with address |
| GET | /user/payments/transactions | Transaction history |
| POST | /user/payments/api/generate-address | Generate/get deposit address |
| GET | /user/payments/api/transactions | List user transactions |

### Webhook Endpoint (public, verified)

| Method | Path | Description |
|--------|------|-------------|
| POST | /api/payments/webhook/bitgo | BitGo webhook receiver |

### Admin Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | /admin/payments | Overview dashboard |
| GET | /admin/payments/transactions | All transactions |
| POST | /admin/payments/api/resync/{txid} | Manually resync a transaction |

## Implementation Phases

### Phase 1: Core Infrastructure
1. Create module structure
2. Database migrations
3. BitGo client (HTTP wrapper for BitGo API)
   - `CreateWalletAddress(label string) (address string, err error)`
   - `GetTransaction(txid string) (*TxDetails, error)`
   - `GetWallet() (*WalletInfo, error)`
   - `AddWebhook(url string) error`

### Phase 2: Deposit Flow
1. Payment service
   - `GetOrCreateWallet(userID string) (*CryptoWallet, error)`
   - `ProcessDeposit(address, txid string, amount float64, confirmations int) error`
   - `CreditUser(userID string, amountUSD float64) error`
2. User deposit page (show address, QR code)
3. Generate address API endpoint

### Phase 3: Webhook Handler
1. Webhook endpoint with signature verification
2. Parse BitGo payload
3. Fetch full transaction details
4. Process each output to our addresses
5. Credit users on confirmation threshold

### Phase 4: UI & History
1. Transaction history page
2. Admin dashboard
3. Real-time balance updates (optional WebSocket)

### Phase 5: Testing & Hardening
1. Use BitGo testnet for development
2. Webhook signature verification
3. Idempotent transaction processing (prevent double-credit)
4. Error handling and logging
5. Rate limiting on webhook endpoint

## BitGo Client Implementation

```go
type BitGoClient struct {
    apiKey    string
    walletID  string
    baseURL   string
    serverIP  string
    testnet   bool
    http      *http.Client
}

func NewBitGoClient(apiKey, walletID string, testnet bool) *BitGoClient {
    baseURL := "https://www.bitgo.com/api/v2/btc"
    if testnet {
        baseURL = "https://test.bitgo.com/api/v2/tbtc"
    }
    return &BitGoClient{
        apiKey:   apiKey,
        walletID: walletID,
        baseURL:  baseURL,
        http:     &http.Client{Timeout: 30 * time.Second},
    }
}

func (c *BitGoClient) CreateWalletAddress(label string) (string, error)
func (c *BitGoClient) GetTransaction(txid string) (*TxDetails, error)
func (c *BitGoClient) ToBTC(satoshi int64) float64
func (c *BitGoClient) ToSatoshi(btc float64) int64
```

## Webhook Payload Example

```json
{
  "hash": "abc123...",
  "transfer": "def456...",
  "coin": "btc",
  "type": "transfer",
  "state": "confirmed",
  "wallet": "6800d33f93dbad3c04ad309e32bd1d20"
}
```

## Transaction Details Response

```json
{
  "id": "abc123...",
  "confirmations": 3,
  "outputs": [
    {
      "address": "bc1q...",
      "value": 100000,
      "wallet": "6800d33f..."
    }
  ],
  "fromWallet": "external"
}
```

## Security Considerations

1. **Webhook Verification** - Verify BitGo webhook signatures
2. **Idempotency** - Use txid as unique key to prevent double-processing
3. **IP Whitelisting** - Optional server IP for BitGo requests
4. **Rate Limiting** - Protect webhook endpoint from abuse
5. **Amount Validation** - Enforce min/max deposit limits
6. **Logging** - Log all transactions for audit trail

## Dependencies

- No external Go packages needed (use stdlib net/http for BitGo API)
- QR code generation: `github.com/skip2/go-qrcode` (optional, can use client-side JS)

## Checklist

- [ ] Create payments module structure
- [ ] Add database migrations
- [ ] Implement BitGo client
- [ ] Implement payment service
- [ ] Create webhook handler
- [ ] Create deposit page template
- [ ] Create transaction history template
- [ ] Add admin dashboard
- [ ] Test with BitGo testnet
- [ ] Add webhook signature verification
- [ ] Deploy and register webhook URL with BitGo
