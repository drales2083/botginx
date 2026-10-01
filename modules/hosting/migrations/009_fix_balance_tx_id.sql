-- Fix balance_transactions id column to support 26-char IDs
ALTER TABLE balance_transactions ALTER COLUMN id TYPE VARCHAR(26);
