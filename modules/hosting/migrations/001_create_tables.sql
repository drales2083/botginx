-- Hosting Module Database Schema

-- Balance transactions
CREATE TABLE IF NOT EXISTS balance_transactions (
    id VARCHAR(24) PRIMARY KEY,
    user_id VARCHAR(24) NOT NULL REFERENCES users(id),
    amount DECIMAL(10,2) NOT NULL,
    type VARCHAR(20) NOT NULL,
    description TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_balance_tx_user ON balance_transactions(user_id);

-- Hosting servers (HestiaCP nodes)
CREATE TABLE IF NOT EXISTS hosting_servers (
    id VARCHAR(24) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    hostname VARCHAR(255) NOT NULL,
    port INT DEFAULT 22,
    username VARCHAR(100) NOT NULL,
    password_encrypted TEXT NOT NULL,
    max_accounts INT DEFAULT 100,
    current_accounts INT DEFAULT 0,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Hosting packages
CREATE TABLE IF NOT EXISTS hosting_packages (
    id VARCHAR(24) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    price_monthly DECIMAL(10,2) NOT NULL,
    disk_mb INT NOT NULL,
    bandwidth_mb INT NOT NULL,
    max_domains INT NOT NULL,
    max_subdomains INT NOT NULL,
    max_mail_accounts INT NOT NULL,
    max_databases INT NOT NULL,
    max_ftp_accounts INT NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    sort_order INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Hosting accounts
CREATE TABLE IF NOT EXISTS hosting_accounts (
    id VARCHAR(24) PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    server_id VARCHAR(24) REFERENCES hosting_servers(id),
    package_id VARCHAR(24) REFERENCES hosting_packages(id),
    panel_username VARCHAR(50) NOT NULL DEFAULT '',
    panel_password_encrypted TEXT NOT NULL DEFAULT '',
    status VARCHAR(20) DEFAULT 'active',
    custom_price DECIMAL(10,2),
    next_billing_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_hosting_accounts_user ON hosting_accounts(user_id);
CREATE INDEX IF NOT EXISTS idx_hosting_accounts_status ON hosting_accounts(status);

-- Hosting domains
CREATE TABLE IF NOT EXISTS hosting_domains (
    id VARCHAR(24) PRIMARY KEY,
    account_id VARCHAR(24) NOT NULL REFERENCES hosting_accounts(id) ON DELETE CASCADE,
    domain VARCHAR(255) NOT NULL,
    ssl_enabled BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_hosting_domains_account ON hosting_domains(account_id);
CREATE INDEX IF NOT EXISTS idx_hosting_domains_domain ON hosting_domains(domain);

-- Hosting domain settings (antibot protection)
CREATE TABLE IF NOT EXISTS hosting_domain_settings (
    id VARCHAR(24) PRIMARY KEY,
    domain_id VARCHAR(24) NOT NULL REFERENCES hosting_domains(id) ON DELETE CASCADE,
    country_mode VARCHAR(20) DEFAULT 'all',
    country_list TEXT DEFAULT '[]',
    device_mode VARCHAR(20) DEFAULT 'all',
    device_list TEXT DEFAULT '[]',
    block_bots BOOLEAN DEFAULT TRUE,
    block_tor BOOLEAN DEFAULT TRUE,
    block_proxy BOOLEAN DEFAULT TRUE,
    block_datacenter BOOLEAN DEFAULT TRUE,
    block_headless BOOLEAN DEFAULT TRUE,
    min_behavior_score INT DEFAULT 0,
    redirect_on_block VARCHAR(500) DEFAULT 'https://www.google.com',
    updated_at TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_hosting_domain_settings_domain ON hosting_domain_settings(domain_id);

-- Hosting emails
CREATE TABLE IF NOT EXISTS hosting_emails (
    id VARCHAR(24) PRIMARY KEY,
    account_id VARCHAR(24) NOT NULL REFERENCES hosting_accounts(id) ON DELETE CASCADE,
    domain_id VARCHAR(24) NOT NULL REFERENCES hosting_domains(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    quota_mb INT DEFAULT 1024,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Hosting databases
CREATE TABLE IF NOT EXISTS hosting_databases (
    id VARCHAR(24) PRIMARY KEY,
    account_id VARCHAR(24) NOT NULL REFERENCES hosting_accounts(id) ON DELETE CASCADE,
    db_name VARCHAR(100) NOT NULL,
    db_user VARCHAR(100) NOT NULL,
    db_password_encrypted TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Hosting FTP
CREATE TABLE IF NOT EXISTS hosting_ftp (
    id VARCHAR(24) PRIMARY KEY,
    account_id VARCHAR(24) NOT NULL REFERENCES hosting_accounts(id) ON DELETE CASCADE,
    username VARCHAR(100) NOT NULL,
    password_encrypted TEXT NOT NULL,
    path VARCHAR(255) DEFAULT '/',
    created_at TIMESTAMP DEFAULT NOW()
);

-- Add balance to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS balance DECIMAL(10,2) DEFAULT 0;
