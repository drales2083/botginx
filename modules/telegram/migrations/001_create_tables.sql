-- Telegram bot configurations
CREATE TABLE IF NOT EXISTS telegram_bots (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    name TEXT NOT NULL,
    bot_token TEXT NOT NULL,
    bot_username TEXT,
    chat_id TEXT,
    webhook_secret TEXT,
    enabled BOOLEAN DEFAULT TRUE,
    use_for_support BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Pending replies (admin clicked Reply, waiting for message)
CREATE TABLE IF NOT EXISTS telegram_pending_replies (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    bot_id TEXT REFERENCES telegram_bots(id) ON DELETE CASCADE,
    telegram_user_id BIGINT NOT NULL,
    telegram_chat_id BIGINT NOT NULL,
    ticket_id TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP DEFAULT NOW() + INTERVAL '5 minutes'
);

CREATE INDEX IF NOT EXISTS idx_pending_replies_user ON telegram_pending_replies(telegram_user_id, bot_id);
CREATE INDEX IF NOT EXISTS idx_pending_replies_expires ON telegram_pending_replies(expires_at);

-- Add telegram tracking columns to support tables
ALTER TABLE support_tickets ADD COLUMN IF NOT EXISTS telegram_message_id BIGINT;
ALTER TABLE support_tickets ADD COLUMN IF NOT EXISTS telegram_bot_id TEXT;
ALTER TABLE support_tickets ADD COLUMN IF NOT EXISTS telegram_chat_id BIGINT;

ALTER TABLE ticket_messages ADD COLUMN IF NOT EXISTS telegram_message_id BIGINT;
ALTER TABLE ticket_messages ADD COLUMN IF NOT EXISTS telegram_bot_id TEXT;
