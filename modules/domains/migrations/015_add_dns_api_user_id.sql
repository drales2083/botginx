-- Track which user's API key should be used for DNS operations
-- When admin adds domain via API and sells to user, DNS updates should use admin's key
ALTER TABLE domains ADD COLUMN IF NOT EXISTS dns_api_user_id TEXT;

-- Backfill: for existing domains with API source, use current user_id as the API user
UPDATE domains
SET dns_api_user_id = user_id
WHERE source IS NOT NULL
  AND source != ''
  AND source != 'manual'
  AND dns_api_user_id IS NULL;

-- Fix known domains that were added by admin and sold to users
-- These should use admin's API key for DNS operations
UPDATE domains
SET dns_api_user_id = 'ce030cd1d4aee6688da43b1c'
WHERE name IN ('laststopin.cfd', 'laststopin.sbs')
  AND source = 'namesilo';
