-- Add marketplace columns to domains table
ALTER TABLE domains ADD COLUMN IF NOT EXISTS is_marketplace BOOLEAN DEFAULT FALSE;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS marketplace_price DECIMAL(10,2);
ALTER TABLE domains ADD COLUMN IF NOT EXISTS marketplace_description TEXT;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS marketplace_listed_at TIMESTAMP;

-- Sales history table
CREATE TABLE IF NOT EXISTS marketplace_sales (
    id VARCHAR(26) PRIMARY KEY,
    domain_id VARCHAR(26) NOT NULL,
    seller_user_id VARCHAR(36) NOT NULL,
    buyer_user_id VARCHAR(36) NOT NULL,
    price DECIMAL(10,2) NOT NULL,
    purchased_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_marketplace_sales_buyer ON marketplace_sales(buyer_user_id);
CREATE INDEX IF NOT EXISTS idx_marketplace_sales_domain ON marketplace_sales(domain_id);
