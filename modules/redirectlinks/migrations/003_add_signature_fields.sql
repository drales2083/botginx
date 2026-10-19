-- Add signature verification fields to redirect_links
-- When enabled, verified visitors receive a cryptographically signed destination URL

ALTER TABLE redirect_links ADD COLUMN IF NOT EXISTS append_signature BOOLEAN DEFAULT FALSE;
ALTER TABLE redirect_links ADD COLUMN IF NOT EXISTS signature_expiry_seconds INTEGER DEFAULT 300;

COMMENT ON COLUMN redirect_links.append_signature IS 'When true, append Ed25519 signature to destination URL for verification';
COMMENT ON COLUMN redirect_links.signature_expiry_seconds IS 'Signature validity period in seconds (default 300 = 5 minutes)';
