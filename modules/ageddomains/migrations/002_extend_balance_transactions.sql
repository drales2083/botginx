-- Extend existing balance_transactions table with idempotency
-- This migration adds columns if they don't exist

-- Add idempotency_key column
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'balance_transactions' AND column_name = 'idempotency_key'
    ) THEN
        ALTER TABLE balance_transactions ADD COLUMN idempotency_key VARCHAR(64);
    END IF;
END $$;

-- Add lock_id column
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'balance_transactions' AND column_name = 'lock_id'
    ) THEN
        ALTER TABLE balance_transactions ADD COLUMN lock_id VARCHAR(26);
    END IF;
END $$;

-- Create unique index on idempotency_key (only for non-null values)
CREATE UNIQUE INDEX IF NOT EXISTS idx_balance_tx_idempotency
    ON balance_transactions(idempotency_key) WHERE idempotency_key IS NOT NULL;
