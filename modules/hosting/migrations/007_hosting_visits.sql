-- Hosting visits table for domain analytics
-- Separate from redirect link visits to avoid conflicts

CREATE TABLE IF NOT EXISTS hosting_visits (
    id                  TEXT PRIMARY KEY,
    domain_id           TEXT NOT NULL REFERENCES hosting_domains(id) ON DELETE CASCADE,
    account_id          TEXT NOT NULL REFERENCES hosting_accounts(id) ON DELETE CASCADE,

    -- Request info
    ip                  TEXT,
    path                TEXT DEFAULT '',
    method              TEXT DEFAULT 'GET',
    country             TEXT DEFAULT '',
    city                TEXT DEFAULT '',
    asn                 INTEGER DEFAULT 0,
    asn_org             TEXT DEFAULT '',

    -- Device info
    device              TEXT DEFAULT 'desktop',
    browser             TEXT DEFAULT '',
    os                  TEXT DEFAULT '',
    user_agent          TEXT DEFAULT '',
    language            TEXT DEFAULT '',
    timezone            TEXT DEFAULT '',
    screen_resolution   TEXT DEFAULT '',

    -- Referrer
    referrer            TEXT DEFAULT '',
    referrer_domain     TEXT DEFAULT '',

    -- UTM tracking
    utm_source          TEXT DEFAULT '',
    utm_medium          TEXT DEFAULT '',
    utm_campaign        TEXT DEFAULT '',
    utm_term            TEXT DEFAULT '',
    utm_content         TEXT DEFAULT '',

    -- Bot detection
    is_bot              BOOLEAN DEFAULT FALSE,
    bot_score           REAL DEFAULT 0,
    behavior_score      INTEGER DEFAULT 0,
    automation_tool     TEXT DEFAULT '',
    is_headless         BOOLEAN DEFAULT FALSE,
    is_tor              BOOLEAN DEFAULT FALSE,
    is_proxy            BOOLEAN DEFAULT FALSE,
    is_datacenter       BOOLEAN DEFAULT FALSE,
    fingerprint         TEXT DEFAULT '',

    -- Action taken
    action              TEXT DEFAULT 'allowed',
    blocked             BOOLEAN DEFAULT FALSE,
    block_reason        TEXT DEFAULT '',

    -- Session
    session_id          TEXT DEFAULT '',

    -- Timestamps
    created_at          TIMESTAMP DEFAULT NOW()
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_hosting_visits_domain ON hosting_visits(domain_id);
CREATE INDEX IF NOT EXISTS idx_hosting_visits_account ON hosting_visits(account_id);
CREATE INDEX IF NOT EXISTS idx_hosting_visits_created ON hosting_visits(created_at);
CREATE INDEX IF NOT EXISTS idx_hosting_visits_country ON hosting_visits(country);
CREATE INDEX IF NOT EXISTS idx_hosting_visits_blocked ON hosting_visits(blocked);
CREATE INDEX IF NOT EXISTS idx_hosting_visits_is_bot ON hosting_visits(is_bot);
