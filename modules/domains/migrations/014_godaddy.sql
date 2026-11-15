-- Add GoDaddy credential columns to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS godaddy_api_key_encrypted TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS godaddy_api_secret_encrypted TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS godaddy_connected_at TIMESTAMP;
