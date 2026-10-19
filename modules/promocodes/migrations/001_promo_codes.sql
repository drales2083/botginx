-- Promo codes for subscription discounts
CREATE TABLE IF NOT EXISTS promo_codes (
    id TEXT PRIMARY KEY,
    code TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',

    -- Discount: 'percentage' or 'fixed'
    discount_type TEXT NOT NULL CHECK (discount_type IN ('percentage', 'fixed')),
    discount_value DECIMAL(10,2) NOT NULL,

    -- Eligibility: 'any', 'new_users', 'expired_users', 'no_active_sub'
    eligibility TEXT DEFAULT 'any',

    -- Usage limits
    max_uses INTEGER,                    -- NULL = unlimited
    max_uses_per_user INTEGER DEFAULT 1, -- NULL = unlimited, 1 = one-time
    current_uses INTEGER DEFAULT 0,

    -- Time window
    valid_from TIMESTAMP,
    valid_until TIMESTAMP,               -- NULL = never expires

    -- Status
    is_active BOOLEAN DEFAULT TRUE,
    created_by TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_promo_codes_code ON promo_codes(code);
CREATE INDEX IF NOT EXISTS idx_promo_codes_active ON promo_codes(is_active) WHERE is_active = TRUE;

-- Redemption history
CREATE TABLE IF NOT EXISTS promo_redemptions (
    id TEXT PRIMARY KEY,
    promo_id TEXT NOT NULL REFERENCES promo_codes(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subscription_id TEXT,                -- links to subscriptions table
    original_amount DECIMAL(10,2),       -- price before discount
    discount_amount DECIMAL(10,2),       -- actual savings
    final_amount DECIMAL(10,2),          -- price after discount
    redeemed_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_promo_redemptions_promo ON promo_redemptions(promo_id);
CREATE INDEX IF NOT EXISTS idx_promo_redemptions_user ON promo_redemptions(user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_promo_redemptions_unique ON promo_redemptions(promo_id, user_id, subscription_id);
