-- Telegram alert dismissed flag (for dashboard prompt)
ALTER TABLE users ADD COLUMN IF NOT EXISTS telegram_alert_dismissed BOOLEAN NOT NULL DEFAULT FALSE;

-- Default domain for Telegram link creation
ALTER TABLE users ADD COLUMN IF NOT EXISTS telegram_default_domain_id TEXT REFERENCES domains(id) ON DELETE SET NULL;
