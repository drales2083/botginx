-- Add server type column to hosting_servers
-- Supports cloudpanel (default) and hestiacp

ALTER TABLE hosting_servers
ADD COLUMN IF NOT EXISTS type VARCHAR(20) DEFAULT 'cloudpanel';

-- Update any existing servers to cloudpanel by default
UPDATE hosting_servers SET type = 'cloudpanel' WHERE type IS NULL;
