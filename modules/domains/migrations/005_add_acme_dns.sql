-- Add acme-dns delegation fields for 100% reliable wildcard SSL
-- This applies to the domains table (redirect links), NOT hosting_domains

-- acme-dns registration info
ALTER TABLE domains ADD COLUMN IF NOT EXISTS acme_subdomain TEXT;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS acme_password TEXT;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS acme_fulldomain TEXT;

-- CNAME verification status
ALTER TABLE domains ADD COLUMN IF NOT EXISTS acme_cname_verified BOOLEAN DEFAULT false;

-- Index for finding domains needing CNAME setup
CREATE INDEX IF NOT EXISTS idx_domains_acme_pending
ON domains(acme_cname_verified)
WHERE acme_subdomain IS NOT NULL AND acme_cname_verified = false;
