-- Pending 2FA sessions for two-factor authentication verification
CREATE TABLE IF NOT EXISTS pending_2fa_sessions (
    token VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(24) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pending_2fa_sessions_user_id ON pending_2fa_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_pending_2fa_sessions_expires_at ON pending_2fa_sessions(expires_at);
