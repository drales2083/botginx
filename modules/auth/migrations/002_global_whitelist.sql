-- User's global whitelisted IPs (their own device IPs for testing)
-- Synced to Supabase for cross-server access by all antibot instances
CREATE TABLE IF NOT EXISTS user_global_whitelists (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ip TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, ip)
);

CREATE INDEX IF NOT EXISTS idx_user_global_whitelists_user ON user_global_whitelists(user_id);
CREATE INDEX IF NOT EXISTS idx_user_global_whitelists_ip ON user_global_whitelists(ip);
