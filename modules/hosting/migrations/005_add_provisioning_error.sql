-- Add provisioning_error column to track why auto-provisioning failed
ALTER TABLE hosting_accounts ADD COLUMN IF NOT EXISTS provisioning_error TEXT;
