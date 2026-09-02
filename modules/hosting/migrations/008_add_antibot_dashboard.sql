-- Add antibot dashboard fields to hosting_servers
-- Users access this dashboard for domain protection settings

ALTER TABLE hosting_servers ADD COLUMN IF NOT EXISTS antibot_dashboard_url TEXT DEFAULT '';
ALTER TABLE hosting_servers ADD COLUMN IF NOT EXISTS antibot_password_encrypted TEXT DEFAULT '';
