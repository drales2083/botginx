-- Add verify_token column for TXT record verification
ALTER TABLE domains ADD COLUMN IF NOT EXISTS verify_token TEXT DEFAULT '';
