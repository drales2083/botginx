-- Migration: Simplify hosting module for CloudPanel
-- Removes HestiaCP-specific structures, adds generic panel support

-- Add panel_url to servers
ALTER TABLE hosting_servers ADD COLUMN IF NOT EXISTS panel_url VARCHAR(500);

-- Rename HestiaCP columns to generic panel columns
ALTER TABLE hosting_accounts RENAME COLUMN hestia_username TO panel_username;
ALTER TABLE hosting_accounts RENAME COLUMN hestia_password_encrypted TO panel_password_encrypted;

-- Make server_id nullable for pending accounts (before admin links them)
ALTER TABLE hosting_accounts ALTER COLUMN server_id DROP NOT NULL;

-- Allow empty panel credentials for pending accounts
ALTER TABLE hosting_accounts ALTER COLUMN panel_username SET DEFAULT '';
ALTER TABLE hosting_accounts ALTER COLUMN panel_password_encrypted SET DEFAULT '';

-- Drop unused tables (users manage these directly in CloudPanel)
DROP TABLE IF EXISTS hosting_ftp;
DROP TABLE IF EXISTS hosting_databases;
DROP TABLE IF EXISTS hosting_emails;

-- Add unique constraint on domain to prevent duplicates
CREATE UNIQUE INDEX IF NOT EXISTS hosting_domains_domain_unique ON hosting_domains(domain);
