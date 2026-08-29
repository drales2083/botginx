-- Subscriptions. One row per user; admins grant and extend access.
--
-- Access is derived from expires_at rather than stored as a status string, so
-- a subscription cannot be left reading "active" after it has lapsed. There is
-- no background job to keep a status column honest, and there should not need
-- to be one.
CREATE TABLE IF NOT EXISTS subscriptions (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    plan        TEXT NOT NULL DEFAULT 'standard',
    expires_at  TIMESTAMP NOT NULL,
    notes       TEXT NOT NULL DEFAULT '',
    granted_by  TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_subscriptions_user_id ON subscriptions(user_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_expires_at ON subscriptions(expires_at);
