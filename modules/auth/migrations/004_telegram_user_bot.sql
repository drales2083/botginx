-- modules/auth/migrations/004_telegram_user_bot.sql
ALTER TABLE users ADD COLUMN IF NOT EXISTS telegram_bot_token_encrypted TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS telegram_bot_username TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS telegram_chat_id BIGINT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS telegram_webhook_secret TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS telegram_connected_at TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN IF NOT EXISTS telegram_last_message_id BIGINT;

-- Notification preferences (all default to true except traffic which needs threshold)
ALTER TABLE users ADD COLUMN IF NOT EXISTS telegram_notify_bot_attacks BOOLEAN DEFAULT TRUE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS telegram_notify_traffic_milestones BOOLEAN DEFAULT FALSE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS telegram_notify_ssl_expiring BOOLEAN DEFAULT TRUE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS telegram_notify_balance_low BOOLEAN DEFAULT TRUE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS telegram_traffic_threshold INTEGER DEFAULT 100;

-- Index for webhook lookups
CREATE INDEX IF NOT EXISTS idx_users_telegram_webhook_secret ON users(telegram_webhook_secret) WHERE telegram_webhook_secret IS NOT NULL;
