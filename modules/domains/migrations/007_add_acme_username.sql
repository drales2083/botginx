-- Add acme_username column for acme-dns API authentication
-- X-Api-User requires the username, not the subdomain

ALTER TABLE domains ADD COLUMN IF NOT EXISTS acme_username TEXT;
