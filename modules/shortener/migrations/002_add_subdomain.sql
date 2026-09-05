-- Add subdomain column for wildcard domain support
ALTER TABLE short_links ADD COLUMN IF NOT EXISTS subdomain TEXT NOT NULL DEFAULT '';

-- Drop old constraint
ALTER TABLE short_links DROP CONSTRAINT IF EXISTS unique_short_link_domain_path;

-- Add new constraint: domain + subdomain + path must be unique
ALTER TABLE short_links ADD CONSTRAINT unique_short_link_domain_subdomain_path UNIQUE(domain_id, subdomain, path);

-- Index for subdomain queries
CREATE INDEX IF NOT EXISTS idx_short_links_subdomain ON short_links(subdomain);
