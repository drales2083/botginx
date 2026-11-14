-- Cloudflare integration
-- Store email + API key per user (single connection per user)

ALTER TABLE users ADD COLUMN IF NOT EXISTS cloudflare_email VARCHAR(255);
ALTER TABLE users ADD COLUMN IF NOT EXISTS cloudflare_api_key_encrypted TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS cloudflare_connected_at TIMESTAMPTZ;

-- Index for finding connected users
CREATE INDEX IF NOT EXISTS idx_users_cloudflare_connected ON users(cloudflare_connected_at) WHERE cloudflare_connected_at IS NOT NULL;
