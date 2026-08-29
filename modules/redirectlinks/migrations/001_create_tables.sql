-- Redirect Links module tables

-- Domains (user's connected domains)
CREATE TABLE IF NOT EXISTS domains (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL,
    name            TEXT NOT NULL,
    server_id       TEXT,  -- VPS server where this domain is deployed
    dns_verified    BOOLEAN DEFAULT FALSE,
    ssl_enabled     BOOLEAN DEFAULT FALSE,
    created_at      TIMESTAMP DEFAULT NOW(),
    updated_at      TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, name)
);

CREATE INDEX IF NOT EXISTS idx_domains_user_id ON domains(user_id);
CREATE INDEX IF NOT EXISTS idx_domains_server_id ON domains(server_id);

-- Redirect Links
CREATE TABLE IF NOT EXISTS redirect_links (
    id                    TEXT PRIMARY KEY,
    user_id               TEXT NOT NULL,
    domain_id             TEXT NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    subdomain             TEXT NOT NULL,
    path                  TEXT NOT NULL,
    type                  TEXT DEFAULT 'REDIRECT',  -- REDIRECT | HTML
    destination_urls      JSONB DEFAULT '[]',
    html_content          TEXT,
    customization         JSONB,
    animation_duration    INTEGER DEFAULT 3,
    turnstile_enabled     BOOLEAN DEFAULT FALSE,
    turnstile_site_key    TEXT,
    turnstile_secret_key  TEXT,
    bot_protection        BOOLEAN DEFAULT FALSE,
    deploy_status         TEXT DEFAULT 'pending',  -- pending | deployed | failed
    deploy_error          TEXT,
    deployed_url          TEXT,
    is_active             BOOLEAN DEFAULT TRUE,
    created_at            TIMESTAMP DEFAULT NOW(),
    updated_at            TIMESTAMP DEFAULT NOW(),
    UNIQUE(domain_id, subdomain, path)
);

CREATE INDEX IF NOT EXISTS idx_redirect_links_user_id ON redirect_links(user_id, is_active);
CREATE INDEX IF NOT EXISTS idx_redirect_links_domain_id ON redirect_links(domain_id);
