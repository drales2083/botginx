-- Add Cloudflare Turnstile credentials per domain
-- Keys are used when "cloudflare" template is selected for link bot protection

ALTER TABLE domains ADD COLUMN IF NOT EXISTS turnstile_site_key TEXT;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS turnstile_secret_key TEXT;
