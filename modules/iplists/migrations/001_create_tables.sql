-- Visitor IP Whitelists (bypass antibot for these visitors)
CREATE TABLE IF NOT EXISTS ip_whitelists (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    ip TEXT NOT NULL,
    note TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, ip)
);
CREATE INDEX IF NOT EXISTS idx_ip_whitelists_user ON ip_whitelists(user_id);
CREATE INDEX IF NOT EXISTS idx_ip_whitelists_ip ON ip_whitelists(ip);

-- Visitor IP Blocklists (always block these visitors)
CREATE TABLE IF NOT EXISTS ip_blocklists (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    ip TEXT NOT NULL,
    note TEXT,
    source TEXT DEFAULT 'manual',
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, ip)
);
CREATE INDEX IF NOT EXISTS idx_ip_blocklists_user ON ip_blocklists(user_id);
CREATE INDEX IF NOT EXISTS idx_ip_blocklists_ip ON ip_blocklists(ip);
