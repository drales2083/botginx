-- Track which user's API key should be used for DNS operations
-- When admin adds domain via API and sells to user, DNS updates should use admin's key
ALTER TABLE domains ADD COLUMN IF NOT EXISTS dns_api_user_id TEXT;

-- IMPORTANT: Fix sold domains BEFORE general backfill
-- Otherwise backfill sets dns_api_user_id = current owner (buyer) instead of original adder

-- Fix cPanel domains that were sold via marketplace
-- Set dns_api_user_id to the original seller (who added via cPanel API)
UPDATE domains
SET dns_api_user_id = ms.seller_user_id
FROM marketplace_sales ms
WHERE domains.id = ms.domain_id
  AND domains.source = 'cpanel';

-- Fix known NameSilo domains that were added by admin and sold to users
UPDATE domains
SET dns_api_user_id = 'ce030cd1d4aee6688da43b1c'
WHERE name IN ('laststopin.cfd', 'laststopin.sbs')
  AND source = 'namesilo';

-- Backfill remaining: for domains NOT already fixed above, use current user_id
UPDATE domains
SET dns_api_user_id = user_id
WHERE source IS NOT NULL
  AND source != ''
  AND source != 'manual'
  AND dns_api_user_id IS NULL;
