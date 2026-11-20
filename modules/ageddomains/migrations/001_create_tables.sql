-- Aged Domains Marketplace Tables
-- Provider configuration (admin-only, encrypted credentials)
CREATE TABLE IF NOT EXISTS domain_providers (
    id              VARCHAR(26) PRIMARY KEY,
    name            VARCHAR(50) NOT NULL,
    display_name    VARCHAR(100) NOT NULL,
    provider_type   VARCHAR(50) NOT NULL,
    api_key         TEXT NOT NULL,
    api_secret      TEXT,
    is_active       BOOLEAN DEFAULT TRUE,
    markup_percent  DECIMAL(5,2) DEFAULT 0,
    min_age_years   INTEGER DEFAULT 0,
    max_price_cents INTEGER DEFAULT 0,
    allowed_tlds    TEXT DEFAULT '',
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Cached domain inventory (refreshed by background job)
CREATE TABLE IF NOT EXISTS domain_inventory (
    id                  VARCHAR(26) PRIMARY KEY,
    provider_id         VARCHAR(26) NOT NULL REFERENCES domain_providers(id) ON DELETE CASCADE,
    domain_name         VARCHAR(255) NOT NULL,
    tld                 VARCHAR(20) NOT NULL,
    registration_price  INTEGER NOT NULL,
    sale_price          INTEGER NOT NULL,
    age_years           INTEGER,
    is_premium          BOOLEAN DEFAULT FALSE,
    category            VARCHAR(50) DEFAULT 'aged',
    is_available        BOOLEAN DEFAULT TRUE,
    safety_status       VARCHAR(20) DEFAULT 'unchecked',
    safety_checked_at   TIMESTAMP,
    safety_reasons      TEXT DEFAULT '[]',
    fetched_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at          TIMESTAMP,
    UNIQUE(provider_id, domain_name)
);

CREATE INDEX IF NOT EXISTS idx_domain_inventory_tld ON domain_inventory(tld);
CREATE INDEX IF NOT EXISTS idx_domain_inventory_price ON domain_inventory(sale_price);
CREATE INDEX IF NOT EXISTS idx_domain_inventory_category ON domain_inventory(category);
CREATE INDEX IF NOT EXISTS idx_domain_inventory_available ON domain_inventory(is_available);
CREATE INDEX IF NOT EXISTS idx_domain_inventory_safety ON domain_inventory(safety_status);

-- Balance locks for double-spend prevention
CREATE TABLE IF NOT EXISTS balance_locks (
    id              VARCHAR(26) PRIMARY KEY,
    user_id         VARCHAR(36) NOT NULL,
    amount          INTEGER NOT NULL,
    idempotency_key VARCHAR(64) NOT NULL UNIQUE,
    purpose         VARCHAR(255) NOT NULL,
    status          VARCHAR(20) DEFAULT 'pending',
    locked_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at      TIMESTAMP NOT NULL,
    resolved_at     TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_balance_locks_user ON balance_locks(user_id);
CREATE INDEX IF NOT EXISTS idx_balance_locks_status ON balance_locks(status);
CREATE INDEX IF NOT EXISTS idx_balance_locks_expires ON balance_locks(status, expires_at);

-- Domain purchases (audit trail)
CREATE TABLE IF NOT EXISTS aged_domain_purchases (
    id                  VARCHAR(26) PRIMARY KEY,
    user_id             VARCHAR(36) NOT NULL,
    inventory_id        VARCHAR(26) REFERENCES domain_inventory(id),
    domain_name         VARCHAR(255) NOT NULL,
    provider_id         VARCHAR(26) REFERENCES domain_providers(id),
    provider_price      INTEGER NOT NULL,
    sale_price          INTEGER NOT NULL,
    balance_lock_id     VARCHAR(26) REFERENCES balance_locks(id),
    idempotency_key     VARCHAR(64) NOT NULL UNIQUE,
    status              VARCHAR(30) DEFAULT 'pending',
    provider_order_id   VARCHAR(100),
    error_message       TEXT,
    domain_id           VARCHAR(26),
    created_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at        TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_aged_purchases_user ON aged_domain_purchases(user_id);
CREATE INDEX IF NOT EXISTS idx_aged_purchases_status ON aged_domain_purchases(status);

-- Purchase status events (for real-time updates)
CREATE TABLE IF NOT EXISTS purchase_events (
    id              VARCHAR(26) PRIMARY KEY,
    purchase_id     VARCHAR(26) NOT NULL REFERENCES aged_domain_purchases(id) ON DELETE CASCADE,
    event_type      VARCHAR(50) NOT NULL,
    message         TEXT,
    details         TEXT,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_purchase_events_purchase ON purchase_events(purchase_id, created_at);
