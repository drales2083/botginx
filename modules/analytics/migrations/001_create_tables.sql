-- Analytics tables

CREATE TABLE IF NOT EXISTS visits (
    id                  TEXT PRIMARY KEY,
    link_id             TEXT NOT NULL,
    user_id             TEXT NOT NULL,
    session_id          TEXT,
    ip                  TEXT,
    country             TEXT DEFAULT '',
    city                TEXT DEFAULT '',
    latitude            REAL DEFAULT 0,
    longitude           REAL DEFAULT 0,
    asn                 INTEGER DEFAULT 0,
    asn_org             TEXT DEFAULT '',
    device              TEXT DEFAULT 'desktop',
    browser             TEXT DEFAULT '',
    os                  TEXT DEFAULT '',
    referrer            TEXT DEFAULT '',
    referrer_domain     TEXT DEFAULT '',
    user_agent          TEXT DEFAULT '',
    language            TEXT DEFAULT '',
    timezone            TEXT DEFAULT '',
    screen_resolution   TEXT DEFAULT '',

    -- UTM Tracking
    utm_source          TEXT DEFAULT '',
    utm_medium          TEXT DEFAULT '',
    utm_campaign        TEXT DEFAULT '',
    utm_term            TEXT DEFAULT '',
    utm_content         TEXT DEFAULT '',

    -- Bot Detection
    is_bot              BOOLEAN DEFAULT FALSE,
    bot_type            TEXT DEFAULT '',
    bot_score           REAL DEFAULT 0,
    bot_module          TEXT DEFAULT '',
    behavior_score      INTEGER DEFAULT 0,
    automation_tool     TEXT DEFAULT '',
    is_headless         BOOLEAN DEFAULT FALSE,
    is_tor              BOOLEAN DEFAULT FALSE,
    is_proxy            BOOLEAN DEFAULT FALSE,
    is_datacenter       BOOLEAN DEFAULT FALSE,
    cookies_enabled     BOOLEAN DEFAULT TRUE,
    js_enabled          BOOLEAN DEFAULT TRUE,
    fingerprint         TEXT DEFAULT '',

    -- Action
    action              TEXT DEFAULT 'allow',
    is_unique           BOOLEAN DEFAULT TRUE,
    blocked             BOOLEAN DEFAULT FALSE,
    block_reason        TEXT DEFAULT '',
    duration_us         BIGINT DEFAULT 0,

    created_at          TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_visits_link_id ON visits(link_id);
CREATE INDEX IF NOT EXISTS idx_visits_user_id ON visits(user_id);
CREATE INDEX IF NOT EXISTS idx_visits_session_id ON visits(session_id);
CREATE INDEX IF NOT EXISTS idx_visits_created_at ON visits(created_at);
CREATE INDEX IF NOT EXISTS idx_visits_country ON visits(country);
CREATE INDEX IF NOT EXISTS idx_visits_is_bot ON visits(is_bot);
CREATE INDEX IF NOT EXISTS idx_visits_utm_source ON visits(utm_source);
CREATE INDEX IF NOT EXISTS idx_visits_behavior_score ON visits(behavior_score);

-- Visitor sessions table (separate from auth sessions)
CREATE TABLE IF NOT EXISTS visitor_sessions (
    id                  TEXT PRIMARY KEY,
    session_id          TEXT NOT NULL UNIQUE,
    link_id             TEXT NOT NULL,
    user_id             TEXT NOT NULL,
    ip                  TEXT,
    country             TEXT DEFAULT '',
    user_agent          TEXT DEFAULT '',
    fingerprint         TEXT DEFAULT '',
    started_at          TIMESTAMP DEFAULT NOW(),
    ended_at            TIMESTAMP,
    duration            INTEGER DEFAULT 0,
    page_views          INTEGER DEFAULT 0,

    -- Attribution
    referrer_domain     TEXT DEFAULT '',
    utm_source          TEXT DEFAULT '',
    utm_medium          TEXT DEFAULT '',
    utm_campaign        TEXT DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_visitor_sessions_link_id ON visitor_sessions(link_id);
CREATE INDEX IF NOT EXISTS idx_visitor_sessions_user_id ON visitor_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_visitor_sessions_session_id ON visitor_sessions(session_id);
CREATE INDEX IF NOT EXISTS idx_visitor_sessions_started_at ON visitor_sessions(started_at);

-- Conversions table
CREATE TABLE IF NOT EXISTS conversions (
    id                  TEXT PRIMARY KEY,
    session_id          TEXT,
    link_id             TEXT NOT NULL,
    user_id             TEXT NOT NULL,
    ip                  TEXT,
    country             TEXT DEFAULT '',
    path                TEXT DEFAULT '',
    time_to_convert     INTEGER DEFAULT 0,
    challenge_type      TEXT DEFAULT '',
    behavior_score      INTEGER DEFAULT 0,
    trust_token_solves  INTEGER DEFAULT 0,

    -- Attribution
    referrer_domain     TEXT DEFAULT '',
    utm_source          TEXT DEFAULT '',
    utm_medium          TEXT DEFAULT '',
    utm_campaign        TEXT DEFAULT '',

    created_at          TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_conversions_link_id ON conversions(link_id);
CREATE INDEX IF NOT EXISTS idx_conversions_user_id ON conversions(user_id);
CREATE INDEX IF NOT EXISTS idx_conversions_session_id ON conversions(session_id);
CREATE INDEX IF NOT EXISTS idx_conversions_created_at ON conversions(created_at);
CREATE INDEX IF NOT EXISTS idx_conversions_utm_source ON conversions(utm_source);

-- Link settings table
CREATE TABLE IF NOT EXISTS link_settings (
    id                  TEXT PRIMARY KEY,
    link_id             TEXT NOT NULL UNIQUE,
    country_mode        TEXT DEFAULT 'all',
    country_list        TEXT DEFAULT '[]',
    device_mode         TEXT DEFAULT 'all',
    device_list         TEXT DEFAULT '[]',
    block_bots          BOOLEAN DEFAULT FALSE,
    block_tor           BOOLEAN DEFAULT FALSE,
    block_proxy         BOOLEAN DEFAULT FALSE,
    block_datacenter    BOOLEAN DEFAULT FALSE,
    block_headless      BOOLEAN DEFAULT FALSE,
    min_behavior_score  INTEGER DEFAULT 0,
    redirect_on_block   TEXT DEFAULT '',
    updated_at          TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_link_settings_link_id ON link_settings(link_id);

-- Add ASN filtering columns (v1.8.150+)
ALTER TABLE link_settings ADD COLUMN IF NOT EXISTS asn_mode TEXT DEFAULT 'allow';
ALTER TABLE link_settings ADD COLUMN IF NOT EXISTS asn_list TEXT DEFAULT '[]';

-- Add challenge template column (v1.9.0+)
ALTER TABLE link_settings ADD COLUMN IF NOT EXISTS template TEXT DEFAULT '';
