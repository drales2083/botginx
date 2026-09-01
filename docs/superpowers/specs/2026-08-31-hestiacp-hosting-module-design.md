# Bullet Proof Hosting Module Design

## Overview

White-label bulletproof hosting for botginx. Each hosting account is protected by antibot (same protection as redirect links - blocks bots, Tor, VPN, datacenter IPs, headless browsers). Users purchase hosting using balance, system provisions on HestiaCP servers via SSH. Users manage everything from botginx UI - HestiaCP is invisible.

## Key Features

- **Antibot Protected**: Every hosted domain routes through antibot with configurable settings
- **Balance System**: Admin tops up, users spend
- **Auto Load Balance**: Multiple HestiaCP servers, auto-distribute accounts
- **Full Control**: Domains, emails, databases, FTP, SSL - all from one UI
- **Monthly Billing**: Auto-deduct, suspend if no balance

## Database Schema

### balance column (add to existing users table)
```sql
ALTER TABLE users ADD COLUMN balance DECIMAL(10,2) DEFAULT 0;
```

### balance_transactions
```sql
CREATE TABLE balance_transactions (
    id VARCHAR(24) PRIMARY KEY,
    user_id VARCHAR(24) NOT NULL REFERENCES users(id),
    amount DECIMAL(10,2) NOT NULL,
    type VARCHAR(20) NOT NULL, -- 'topup', 'deduct'
    description TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);
CREATE INDEX idx_balance_tx_user ON balance_transactions(user_id);
```

### hosting_servers
```sql
CREATE TABLE hosting_servers (
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
```

### hosting_packages
```sql
CREATE TABLE hosting_packages (
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
```

### hosting_accounts
```sql
CREATE TABLE hosting_accounts (
    id VARCHAR(24) PRIMARY KEY,
    user_id VARCHAR(24) NOT NULL REFERENCES users(id),
    server_id VARCHAR(24) NOT NULL REFERENCES hosting_servers(id),
    package_id VARCHAR(24) REFERENCES hosting_packages(id),
    hestia_username VARCHAR(50) NOT NULL,
    hestia_password_encrypted TEXT NOT NULL,
    status VARCHAR(20) DEFAULT 'active', -- 'active', 'suspended', 'cancelled'
    custom_price DECIMAL(10,2), -- nullable, for admin custom quotes
    next_billing_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
CREATE INDEX idx_hosting_accounts_user ON hosting_accounts(user_id);
CREATE INDEX idx_hosting_accounts_status ON hosting_accounts(status);
```

### hosting_domains
```sql
CREATE TABLE hosting_domains (
    id VARCHAR(24) PRIMARY KEY,
    account_id VARCHAR(24) NOT NULL REFERENCES hosting_accounts(id) ON DELETE CASCADE,
    domain VARCHAR(255) NOT NULL,
    ssl_enabled BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT NOW()
);
CREATE INDEX idx_hosting_domains_account ON hosting_domains(account_id);
```

### hosting_domain_settings (antibot protection per domain)
```sql
CREATE TABLE hosting_domain_settings (
    id VARCHAR(24) PRIMARY KEY,
    domain_id VARCHAR(24) NOT NULL REFERENCES hosting_domains(id) ON DELETE CASCADE,
    -- Country filtering
    country_mode VARCHAR(20) DEFAULT 'all', -- 'all', 'whitelist', 'blacklist'
    country_list TEXT DEFAULT '[]', -- JSON array
    -- Device filtering  
    device_mode VARCHAR(20) DEFAULT 'all',
    device_list TEXT DEFAULT '[]',
    -- Bot protection (ON by default for bulletproof hosting)
    block_bots BOOLEAN DEFAULT TRUE,
    block_tor BOOLEAN DEFAULT TRUE,
    block_proxy BOOLEAN DEFAULT TRUE,
    block_datacenter BOOLEAN DEFAULT TRUE,
    block_headless BOOLEAN DEFAULT TRUE,
    -- Behavior score
    min_behavior_score INT DEFAULT 0,
    -- Redirect when blocked
    redirect_on_block VARCHAR(500) DEFAULT 'https://www.google.com',
    updated_at TIMESTAMP DEFAULT NOW()
);
CREATE INDEX idx_hosting_domain_settings_domain ON hosting_domain_settings(domain_id);
```

### hosting_emails
```sql
CREATE TABLE hosting_emails (
    id VARCHAR(24) PRIMARY KEY,
    account_id VARCHAR(24) NOT NULL REFERENCES hosting_accounts(id) ON DELETE CASCADE,
    domain_id VARCHAR(24) NOT NULL REFERENCES hosting_domains(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    quota_mb INT DEFAULT 1024,
    created_at TIMESTAMP DEFAULT NOW()
);
```

### hosting_databases
```sql
CREATE TABLE hosting_databases (
    id VARCHAR(24) PRIMARY KEY,
    account_id VARCHAR(24) NOT NULL REFERENCES hosting_accounts(id) ON DELETE CASCADE,
    db_name VARCHAR(100) NOT NULL,
    db_user VARCHAR(100) NOT NULL,
    db_password_encrypted TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);
```

### hosting_ftp
```sql
CREATE TABLE hosting_ftp (
    id VARCHAR(24) PRIMARY KEY,
    account_id VARCHAR(24) NOT NULL REFERENCES hosting_accounts(id) ON DELETE CASCADE,
    username VARCHAR(100) NOT NULL,
    password_encrypted TEXT NOT NULL,
    path VARCHAR(255) DEFAULT '/',
    created_at TIMESTAMP DEFAULT NOW()
);
```

## Module Structure

```
modules/hosting/
├── module.go                 # Routes, menu items, migrations
├── handlers/
│   ├── admin.go              # All admin handlers (servers, packages, accounts, balance)
│   ├── user.go               # All user handlers (dashboard, domains, emails, etc.)
│   └── api.go                # JSON endpoints for AJAX
├── services/
│   ├── hosting_service.go    # Core business logic
│   ├── hestia_client.go      # SSH commands to HestiaCP
│   ├── billing_service.go    # Monthly billing, suspension
│   └── loadbalancer.go       # Server selection
├── models/
│   └── models.go             # All structs
├── migrations/
│   └── 001_create_tables.sql
├── templates/
│   ├── admin.html            # Admin dashboard with tabs (Servers, Packages, Accounts)
│   └── user.html             # User dashboard with tabs (Overview, Domains, Emails, Databases, FTP, Settings)

pkg/hestia/
├── client.go                 # SSH connection pool
└── commands.go               # HestiaCP CLI wrappers
```

## Menu Structure

**User sidebar** (single item):
```
Hosting (bi-shield-check) → /user/hosting
```

Inside /user/hosting - tabs:
- Overview (account status, usage, credentials)
- Domains (add/remove, SSL, antibot settings per domain)
- Emails (mail accounts)
- Databases (MySQL databases)
- FTP (FTP accounts)

**Admin sidebar** (single item):
```
Hosting (bi-shield-check) → /admin/hosting
```

Inside /admin/hosting - tabs:
- Servers (HestiaCP server management)
- Packages (hosting plans)
- Accounts (all user accounts, custom quotes)

Balance management stays in existing /admin/users page (add balance column + top up button).

## Antibot Integration

Each hosted domain gets antibot protection. When user adds a domain:

1. Domain added to HestiaCP via `v-add-domain`
2. Create `hosting_domain_settings` record with defaults:
   - block_bots: TRUE
   - block_tor: TRUE
   - block_proxy: TRUE
   - block_datacenter: TRUE
   - block_headless: TRUE
   - redirect_on_block: "https://www.google.com"

3. Domain config pushed to antibot (same as redirect links)

User can customize per-domain from Domains tab → Settings button.

### Antibot Callback

Extend existing `/api/botection/should-block` endpoint to also check `hosting_domain_settings` table. When antibot queries:

```
POST /api/botection/should-block
{
  "host": "client-domain.com",
  "ip": "...",
  "country": "US",
  ...
}
```

System checks:
1. Is this a redirect link domain? → Use link_settings
2. Is this a hosting domain? → Use hosting_domain_settings
3. Neither? → Default allow

## HestiaCP Commands

```bash
# Account
v-add-user {username} {password} {email} {package} {name}
v-delete-user {username}
v-suspend-user {username}
v-unsuspend-user {username}

# Domains
v-add-domain {username} {domain}
v-delete-domain {username} {domain}
v-list-domains {username} json

# SSL
v-add-letsencrypt-domain {username} {domain}

# Email
v-add-mail-domain {username} {domain}
v-add-mail-account {username} {domain} {account} {password}
v-delete-mail-account {username} {domain} {account}
v-list-mail-accounts {username} {domain} json

# Databases
v-add-database {username} {db_name} {db_user} {db_password}
v-delete-database {username} {db_name}
v-list-databases {username} json

# FTP
v-add-ftp-account {username} {ftp_user} {ftp_password} {path}
v-delete-ftp-account {username} {ftp_user}

# Stats
v-list-user {username} json
```

## User Flows

### Purchase Flow
1. User visits /user/hosting (no account yet)
2. Sees packages with pricing
3. Clicks "Buy" → checks balance
4. If OK:
   - Load balancer picks server
   - Generate username (bp_XXXXX)
   - Generate password (16 chars)
   - SSH: v-add-user
   - Deduct balance
   - Set next_billing_at = +30 days
   - Show dashboard with credentials
5. If insufficient: "Top up your balance to continue"

### Add Domain Flow
1. User goes to Domains tab
2. Enters domain name
3. System:
   - SSH: v-add-domain
   - Create hosting_domains record
   - Create hosting_domain_settings with antibot ON
   - Push config to antibot
4. Show domain with "Setup DNS" instructions
5. User can click "Enable SSL" → v-add-letsencrypt-domain
6. User can click "Traffic Settings" → same UI as redirect link settings

### Monthly Billing (cron hourly)
1. Find accounts: next_billing_at <= now AND status = active
2. For each:
   - Price = custom_price OR package.price_monthly
   - If balance >= price:
     - Deduct, log transaction
     - next_billing_at += 30 days
   - Else:
     - v-suspend-user
     - status = suspended

### Reactivation
1. User with suspended account visits /user/hosting
2. Sees "Suspended - Reactivate" button
3. Clicks → checks balance
4. If OK:
   - Deduct balance
   - v-unsuspend-user
   - status = active
   - next_billing_at = +30 days

## Admin Flows

### /admin/hosting (tabbed interface)

**Servers Tab:**
- List servers (name, host, accounts used/max, status)
- Add/edit server (hostname, port, user, password, max accounts)
- Test connection button
- Disable server (no new accounts)

**Packages Tab:**
- List packages (name, price, specs)
- Add/edit package
- Reorder, disable

**Accounts Tab:**
- List all accounts (user, server, package, status, next billing)
- Filter by server, status
- Actions: suspend, unsuspend, cancel
- Create custom quote for user

### Balance (in /admin/users)
- Add Balance column
- Top Up button → modal with amount
- View transaction history

## Load Balancer

```go
func PickServer() (*HostingServer, error) {
    servers := GetActiveServers()
    available := filter(servers, s => s.CurrentAccounts < s.MaxAccounts)
    sort(available, by: CurrentAccounts ASC)
    if len(available) == 0 {
        return nil, errors.New("no servers available")
    }
    return available[0], nil
}
```

## Error Handling

| Scenario | Action |
|----------|--------|
| SSH fails | Retry 2x, mark unhealthy, alert |
| Command fails | Parse error, show message, log |
| No servers | "No servers available" |
| Domain exists | "Domain already in use" |
| SSL fails | Mark pending, user retries |
| Insufficient balance | Block action, show message |

## Security

- Passwords: AES-256 encrypted (HOSTING_ENCRYPTION_KEY env)
- Generated passwords: 16 chars alphanumeric + symbols
- SSH: 10s connect timeout, 30s command timeout
- Input validation: domain format, email format

## Environment Variables

```
HOSTING_ENCRYPTION_KEY=<32-byte-key>
```
