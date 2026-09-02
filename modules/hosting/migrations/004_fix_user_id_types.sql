-- Fix user_id column types to support UUID user IDs
-- The users table uses text/UUID IDs, not varchar(24)

ALTER TABLE hosting_accounts ALTER COLUMN user_id TYPE text;
ALTER TABLE balance_transactions ALTER COLUMN user_id TYPE text;
