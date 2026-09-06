-- Add subdomain column for wildcard domain support
ALTER TABLE short_links ADD COLUMN IF NOT EXISTS subdomain TEXT NOT NULL DEFAULT '';

-- Drop old constraint
ALTER TABLE short_links DROP CONSTRAINT IF EXISTS unique_short_link_domain_path;

-- Add new constraint: domain + subdomain + path must be unique (idempotent)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'unique_short_link_domain_subdomain_path'
    ) THEN
        ALTER TABLE short_links ADD CONSTRAINT unique_short_link_domain_subdomain_path UNIQUE(domain_id, subdomain, path);
    END IF;
END $$;

-- Index for subdomain queries
CREATE INDEX IF NOT EXISTS idx_short_links_subdomain ON short_links(subdomain);
