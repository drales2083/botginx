-- Add backup name to settings
ALTER TABLE backup_settings ADD COLUMN IF NOT EXISTS backup_name TEXT DEFAULT 'Botginx Backup';
