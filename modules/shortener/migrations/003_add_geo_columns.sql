-- Add columns for dashboard/analytics integration
ALTER TABLE short_link_clicks ADD COLUMN IF NOT EXISTS user_id TEXT;
ALTER TABLE short_link_clicks ADD COLUMN IF NOT EXISTS latitude REAL DEFAULT 0;
ALTER TABLE short_link_clicks ADD COLUMN IF NOT EXISTS longitude REAL DEFAULT 0;
ALTER TABLE short_link_clicks ADD COLUMN IF NOT EXISTS blocked BOOLEAN DEFAULT false;

-- Index for user queries (dashboard)
CREATE INDEX IF NOT EXISTS idx_short_link_clicks_user_id ON short_link_clicks(user_id);
