-- User favorites for aged domains
CREATE TABLE IF NOT EXISTS aged_domain_favorites (
    id              VARCHAR(26) PRIMARY KEY,
    user_id         VARCHAR(36) NOT NULL,
    inventory_id    VARCHAR(26) NOT NULL REFERENCES domain_inventory(id) ON DELETE CASCADE,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, inventory_id)
);

CREATE INDEX IF NOT EXISTS idx_aged_favorites_user ON aged_domain_favorites(user_id);
CREATE INDEX IF NOT EXISTS idx_aged_favorites_inventory ON aged_domain_favorites(inventory_id);
