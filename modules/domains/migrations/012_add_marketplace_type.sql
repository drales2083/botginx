-- Add marketplace_type column for domain categorization
-- Options: brand_new, cpanel, hacked

ALTER TABLE domains ADD COLUMN IF NOT EXISTS marketplace_type TEXT DEFAULT 'cpanel';
