-- Add is_reseller_client flag to users
-- Reseller clients contact admin for subscription renewal instead of paying directly
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_reseller_client BOOLEAN DEFAULT false;
