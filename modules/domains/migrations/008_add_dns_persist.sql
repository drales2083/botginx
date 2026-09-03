-- Migration: Add dns-persist-01 support columns
-- These columns store the persistent TXT record value for wildcard SSL

BEGIN;

-- The persistent TXT record value (never changes after initial setup)
ALTER TABLE domains ADD COLUMN IF NOT EXISTS persist_txt_value TEXT;

-- Whether the TXT record has been verified as present in DNS
ALTER TABLE domains ADD COLUMN IF NOT EXISTS persist_txt_verified BOOLEAN DEFAULT FALSE;

-- The ACME account URI used for this domain's SSL
ALTER TABLE domains ADD COLUMN IF NOT EXISTS lego_account_uri TEXT;

-- Index for quick lookup of verified domains
CREATE INDEX IF NOT EXISTS idx_domains_persist_verified
    ON domains(persist_txt_verified)
    WHERE persist_txt_verified = TRUE;

COMMIT;
