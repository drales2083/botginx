-- Domains module tables
-- Note: domains table may already exist from redirectlinks module
-- This migration ensures the table exists

CREATE TABLE IF NOT EXISTS domains (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL,
    name            TEXT NOT NULL,
    server_id       TEXT,
    dns_verified    BOOLEAN DEFAULT FALSE,
    ssl_enabled     BOOLEAN DEFAULT FALSE,
    created_at      TIMESTAMP DEFAULT NOW(),
    updated_at      TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, name)
);

CREATE INDEX IF NOT EXISTS idx_domains_user_id ON domains(user_id);
CREATE INDEX IF NOT EXISTS idx_domains_server_id ON domains(server_id);
