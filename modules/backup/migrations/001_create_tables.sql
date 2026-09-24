-- Backup history
CREATE TABLE IF NOT EXISTS backup_history (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    started_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ,

    db_size_bytes BIGINT,
    config_size_bytes BIGINT,
    total_size_bytes BIGINT,
    chunk_count INT DEFAULT 0,

    telegram_meta JSONB,
    error_message TEXT,

    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_backup_history_status ON backup_history(status);
CREATE INDEX IF NOT EXISTS idx_backup_history_started ON backup_history(started_at DESC);

-- Backup settings (singleton)
CREATE TABLE IF NOT EXISTS backup_settings (
    id TEXT PRIMARY KEY DEFAULT 'default',
    enabled BOOLEAN DEFAULT FALSE,

    interval_hours INT DEFAULT 24,
    retention_count INT DEFAULT 7,

    telegram_bot_token_encrypted BYTEA,
    telegram_chat_ids TEXT[] DEFAULT ARRAY[]::TEXT[],
    telegram_chunk_size_mb INT DEFAULT 20,
    telegram_send_notification BOOLEAN DEFAULT TRUE,

    include_database BOOLEAN DEFAULT TRUE,
    include_config BOOLEAN DEFAULT TRUE,
    config_paths TEXT[] DEFAULT ARRAY['.env', '.env.local'],

    updated_at TIMESTAMPTZ DEFAULT NOW()
);

INSERT INTO backup_settings (id) VALUES ('default') ON CONFLICT DO NOTHING;
