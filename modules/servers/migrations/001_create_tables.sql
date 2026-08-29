-- Servers module tables

CREATE TABLE IF NOT EXISTS servers (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL,
    name            TEXT NOT NULL,
    ip              TEXT NOT NULL,
    port            INTEGER DEFAULT 22,
    ssh_user        TEXT DEFAULT 'root',
    ssh_password    TEXT,  -- Encrypted password for password auth
    auth_method     TEXT DEFAULT 'key',  -- 'password' or 'key'
    status          TEXT DEFAULT 'pending',
    provider        TEXT,
    os              TEXT,
    created_at      TIMESTAMP DEFAULT NOW(),
    updated_at      TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_servers_user_id ON servers(user_id);
CREATE INDEX IF NOT EXISTS idx_servers_status ON servers(status);

CREATE TABLE IF NOT EXISTS server_logs (
    id              TEXT PRIMARY KEY,
    server_id       TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    type            TEXT NOT NULL,
    content         TEXT,
    created_at      TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_server_logs_server_id ON server_logs(server_id, created_at DESC);
