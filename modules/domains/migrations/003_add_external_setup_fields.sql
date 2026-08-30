-- External domain setup tracking fields
-- For cPanel/external DNS users who need guided setup wizard

ALTER TABLE domains ADD COLUMN IF NOT EXISTS setup_type TEXT NOT NULL DEFAULT 'direct';
-- setup_type: 'direct' (A record points to us), 'external' (cPanel/other DNS)

ALTER TABLE domains ADD COLUMN IF NOT EXISTS setup_step TEXT NOT NULL DEFAULT 'complete';
-- setup_step: 'pending', 'dns_waiting', 'ssl_generating', 'complete'

ALTER TABLE domains ADD COLUMN IF NOT EXISTS acme_token TEXT;
-- The ACME challenge token for SSL DNS-01 verification

ALTER TABLE domains ADD COLUMN IF NOT EXISTS acme_token_expires_at TIMESTAMP;
-- When the ACME token expires (usually ~15 minutes)

ALTER TABLE domains ADD COLUMN IF NOT EXISTS is_wildcard BOOLEAN NOT NULL DEFAULT false;
-- Whether this is a wildcard domain (*.example.com)

-- Index for finding domains in setup process
CREATE INDEX IF NOT EXISTS idx_domains_setup_step ON domains(setup_step) WHERE setup_step != 'complete';
