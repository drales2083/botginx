-- Dashboard stats columns on users
ALTER TABLE users ADD COLUMN IF NOT EXISTS bots_detected BIGINT DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS humans_verified BIGINT DEFAULT 0;

-- Announcements for news feed
CREATE TABLE IF NOT EXISTS announcements (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    badge TEXT,
    is_pinned BOOLEAN DEFAULT FALSE,
    published_at TIMESTAMPTZ DEFAULT NOW(),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_announcements_published ON announcements(published_at DESC);
