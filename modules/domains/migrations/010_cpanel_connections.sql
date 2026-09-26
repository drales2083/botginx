-- cPanel connections for auto-DNS management
-- Each user can have multiple cPanel connections (different hosting accounts)

CREATE TABLE IF NOT EXISTS cpanel_connections (
    id VARCHAR(50) PRIMARY KEY,
    user_id VARCHAR(26) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,                     -- Display name (e.g., "My Hosting Account")
    host VARCHAR(255) NOT NULL,                     -- cPanel host:port (e.g., "example.com:2083")
    username VARCHAR(255) NOT NULL,                 -- cPanel username
    api_token_encrypted TEXT NOT NULL,              -- API token (encrypted)
    is_active BOOLEAN DEFAULT true,
    last_used_at TIMESTAMP,
    last_error TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    UNIQUE(user_id, host, username)
);

CREATE INDEX IF NOT EXISTS idx_cpanel_connections_user ON cpanel_connections(user_id);
CREATE INDEX IF NOT EXISTS idx_cpanel_connections_active ON cpanel_connections(user_id, is_active);

-- Add cpanel_connection_id to domains table
ALTER TABLE domains ADD COLUMN IF NOT EXISTS cpanel_connection_id VARCHAR(50) REFERENCES cpanel_connections(id) ON DELETE SET NULL;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS cpanel_auto_dns BOOLEAN DEFAULT false;

-- Index for finding domains by cpanel connection
CREATE INDEX IF NOT EXISTS idx_domains_cpanel ON domains(cpanel_connection_id) WHERE cpanel_connection_id IS NOT NULL;
