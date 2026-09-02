-- Add SSL retry tracking and certificate info (DirectAdmin-inspired)

-- SSL retry tracking
ALTER TABLE hosting_domains ADD COLUMN IF NOT EXISTS ssl_retry_count INTEGER DEFAULT 0;
ALTER TABLE hosting_domains ADD COLUMN IF NOT EXISTS ssl_last_attempt TIMESTAMP;
ALTER TABLE hosting_domains ADD COLUMN IF NOT EXISTS ssl_expiry TIMESTAMP;

-- DNS tracking
ALTER TABLE hosting_domains ADD COLUMN IF NOT EXISTS dns_last_check TIMESTAMP;
ALTER TABLE hosting_domains ADD COLUMN IF NOT EXISTS dns_current_ip TEXT;

-- Index for finding domains needing SSL retry
CREATE INDEX IF NOT EXISTS idx_domains_ssl_retry ON hosting_domains(ssl_enabled, ssl_retry_count)
WHERE ssl_enabled = false AND ssl_retry_count < 5;
