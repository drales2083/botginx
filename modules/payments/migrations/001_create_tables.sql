-- Crypto wallets for user deposit addresses
CREATE TABLE IF NOT EXISTS crypto_wallets (
    id VARCHAR(26) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    wallet_id VARCHAR(64) NOT NULL,
    address VARCHAR(128) NOT NULL UNIQUE,
    coin VARCHAR(10) NOT NULL DEFAULT 'btc',
    label VARCHAR(255),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_crypto_wallets_user ON crypto_wallets(user_id);
CREATE INDEX IF NOT EXISTS idx_crypto_wallets_coin ON crypto_wallets(user_id, coin);

-- Crypto transactions (deposits/withdrawals)
CREATE TABLE IF NOT EXISTS crypto_transactions (
    id VARCHAR(26) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    wallet_id VARCHAR(26) REFERENCES crypto_wallets(id),
    txid VARCHAR(128) NOT NULL UNIQUE,
    amount DECIMAL(18,8) NOT NULL,
    amount_usd DECIMAL(10,2) NOT NULL,
    confirmations INTEGER DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    type VARCHAR(20) NOT NULL DEFAULT 'deposit',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_crypto_tx_user ON crypto_transactions(user_id);
CREATE INDEX IF NOT EXISTS idx_crypto_tx_status ON crypto_transactions(status);
CREATE INDEX IF NOT EXISTS idx_crypto_tx_type ON crypto_transactions(type);
