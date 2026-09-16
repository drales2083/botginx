-- Add fields for automatic domain sync
ALTER TABLE domains ADD COLUMN IF NOT EXISTS last_sync_at TIMESTAMP;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS last_sync_error TEXT;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS sync_status TEXT DEFAULT 'pending';
-- sync_status values: pending, syncing, dns_waiting, ssl_generating, active, error
