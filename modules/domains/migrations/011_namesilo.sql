-- NameSilo integration
-- Store API key per user (single connection per user, unlike cPanel which allows multiple)

ALTER TABLE users ADD COLUMN IF NOT EXISTS namesilo_api_key_encrypted TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS namesilo_connected_at TIMESTAMPTZ;

-- Add source tracking to domains if not exists
ALTER TABLE domains ADD COLUMN IF NOT EXISTS source VARCHAR(50) DEFAULT 'manual';
ALTER TABLE domains ADD COLUMN IF NOT EXISTS provider_domain_id VARCHAR(255);

-- dns_status for tracking auto-configuration
ALTER TABLE domains ADD COLUMN IF NOT EXISTS dns_auto_configured BOOLEAN DEFAULT FALSE;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS dns_config_error TEXT;

-- Index for provider lookups
CREATE INDEX IF NOT EXISTS idx_domains_source ON domains(source);
CREATE INDEX IF NOT EXISTS idx_domains_provider_domain_id ON domains(provider_domain_id);
