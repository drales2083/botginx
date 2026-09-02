-- Add ssl_error column to store SSL generation errors for user feedback
ALTER TABLE domains ADD COLUMN IF NOT EXISTS ssl_error TEXT;
