-- Fix ID column sizes: UUID is 36 chars, not 26
-- All id columns were defined as VARCHAR(26) but code uses uuid.New().String() which is 36 chars

-- domain_providers
ALTER TABLE domain_providers ALTER COLUMN id TYPE VARCHAR(36);

-- domain_inventory
ALTER TABLE domain_inventory ALTER COLUMN id TYPE VARCHAR(36);
ALTER TABLE domain_inventory ALTER COLUMN provider_id TYPE VARCHAR(36);

-- balance_locks
ALTER TABLE balance_locks ALTER COLUMN id TYPE VARCHAR(36);

-- aged_domain_purchases
ALTER TABLE aged_domain_purchases ALTER COLUMN id TYPE VARCHAR(36);
ALTER TABLE aged_domain_purchases ALTER COLUMN inventory_id TYPE VARCHAR(36);
ALTER TABLE aged_domain_purchases ALTER COLUMN provider_id TYPE VARCHAR(36);
ALTER TABLE aged_domain_purchases ALTER COLUMN balance_lock_id TYPE VARCHAR(36);
ALTER TABLE aged_domain_purchases ALTER COLUMN domain_id TYPE VARCHAR(36);

-- purchase_events
ALTER TABLE purchase_events ALTER COLUMN id TYPE VARCHAR(36);
ALTER TABLE purchase_events ALTER COLUMN purchase_id TYPE VARCHAR(36);

-- aged_domain_favorites
ALTER TABLE aged_domain_favorites ALTER COLUMN id TYPE VARCHAR(36);
ALTER TABLE aged_domain_favorites ALTER COLUMN inventory_id TYPE VARCHAR(36);
