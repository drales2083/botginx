-- Shared (platform) domains added by admins.
-- A domain added from /admin/domains is shared with every user.
-- A domain added from /user/domains belongs to that user alone -- including
-- when an admin adds it there.
ALTER TABLE domains ADD COLUMN IF NOT EXISTS is_shared BOOLEAN DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_domains_is_shared ON domains(is_shared);

-- Platform-wide settings (key/value).
CREATE TABLE IF NOT EXISTS platform_settings (
    key         TEXT PRIMARY KEY,
    value       TEXT NOT NULL,
    updated_at  TIMESTAMP DEFAULT NOW()
);

-- Shared domains are opt-in. Off by default; turning it on only has an effect
-- once an admin has actually added a shared domain.
INSERT INTO platform_settings (key, value)
VALUES ('shared_domains_enabled', 'true')
ON CONFLICT (key) DO NOTHING;
