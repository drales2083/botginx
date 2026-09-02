-- Add status tracking fields to hosting_domains
ALTER TABLE hosting_domains ADD COLUMN IF NOT EXISTS dns_verified BOOLEAN DEFAULT FALSE;
ALTER TABLE hosting_domains ADD COLUMN IF NOT EXISTS setup_status VARCHAR(20) DEFAULT 'pending_dns';
ALTER TABLE hosting_domains ADD COLUMN IF NOT EXISTS ssl_error TEXT;
ALTER TABLE hosting_domains ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT NOW();

-- Update existing domains to active if they have SSL
UPDATE hosting_domains SET setup_status = 'active', dns_verified = TRUE WHERE ssl_enabled = TRUE;
