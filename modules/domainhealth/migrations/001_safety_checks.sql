-- Add safety check columns to domains table
ALTER TABLE domains ADD COLUMN IF NOT EXISTS safety_status TEXT DEFAULT 'unknown';
ALTER TABLE domains ADD COLUMN IF NOT EXISTS safety_checked_at TIMESTAMPTZ;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS safety_threats JSONB DEFAULT '[]';

-- Index for efficient queries on safety status
CREATE INDEX IF NOT EXISTS idx_domains_safety_status ON domains(safety_status);
CREATE INDEX IF NOT EXISTS idx_domains_safety_checked_at ON domains(safety_checked_at);
