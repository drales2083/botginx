-- Short links table
CREATE TABLE IF NOT EXISTS short_links (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    domain_id TEXT NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    destinations JSONB NOT NULL DEFAULT '[]',
    rotation_mode TEXT NOT NULL DEFAULT 'random',
    bot_error INTEGER NOT NULL DEFAULT 403,
    qr_enabled BOOLEAN NOT NULL DEFAULT false,
    protection_settings JSONB,
    click_count INTEGER NOT NULL DEFAULT 0,
    human_count INTEGER NOT NULL DEFAULT 0,
    bot_count INTEGER NOT NULL DEFAULT 0,
    last_click_at TIMESTAMP,
    deploy_status TEXT NOT NULL DEFAULT 'pending',
    deployed_url TEXT,
    deploy_error TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),

    CONSTRAINT unique_short_link_domain_path UNIQUE(domain_id, path)
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_short_links_user_id ON short_links(user_id);
CREATE INDEX IF NOT EXISTS idx_short_links_domain_id ON short_links(domain_id);
CREATE INDEX IF NOT EXISTS idx_short_links_is_active ON short_links(is_active);

-- Click tracking for short links
CREATE TABLE IF NOT EXISTS short_link_clicks (
    id TEXT PRIMARY KEY,
    link_id TEXT NOT NULL REFERENCES short_links(id) ON DELETE CASCADE,
    visitor_ip_hash TEXT,
    country TEXT,
    city TEXT,
    device TEXT,
    browser TEXT,
    os TEXT,
    is_bot BOOLEAN NOT NULL DEFAULT false,
    bot_type TEXT,
    bot_reason TEXT,
    destination_used TEXT,
    referer TEXT,
    user_agent TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_short_link_clicks_link_id ON short_link_clicks(link_id);
CREATE INDEX IF NOT EXISTS idx_short_link_clicks_created_at ON short_link_clicks(created_at);
CREATE INDEX IF NOT EXISTS idx_short_link_clicks_is_bot ON short_link_clicks(is_bot);
