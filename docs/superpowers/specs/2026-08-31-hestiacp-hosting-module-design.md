# HestiaCP Hosting Module Design

## Overview

White-label hosting reseller module for botginx. Users purchase hosting packages using balance, system provisions accounts on HestiaCP servers via SSH API. Users manage domains, emails, databases, and FTP from botginx UI. HestiaCP branding is invisible to end users.

## Requirements

- **Balance system**: Admin tops up user balance manually, users spend on hosting
- **Packages**: Fixed plans created by admin + custom quotes for individual users
- **Servers**: Multiple HestiaCP servers with automatic load balancing
- **User control**: Full management (domains, emails, databases, FTP, SSL) from botginx
- **Billing**: Monthly recurring, auto-deduct from balance, suspend if insufficient

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
│   ├── admin.go              # Server/package/account management
│   ├── user.go               # Purchase, dashboard, resource management
│   └── api.go                # JSON endpoints for AJAX operations
├── services/
│   ├── hosting_service.go    # Core business logic
│   ├── hestia_client.go      # SSH commands to HestiaCP API
│   ├── billing_service.go    # Monthly deductions, suspensions
│   └── loadbalancer.go       # Pick best server for new account
├── models/
│   └── models.go             # All structs, inputs, enums
├── migrations/
│   └── 001_create_tables.sql
├── templates/
│   ├── admin_servers.html    # Manage HestiaCP servers
│   ├── admin_packages.html   # Manage hosting packages
│   ├── admin_accounts.html   # View all accounts, custom quotes
│   ├── user_index.html       # User hosting dashboard
│   ├── user_purchase.html    # Buy package
│   ├── user_domains.html     # Manage domains
│   ├── user_emails.html      # Manage email accounts
│   ├── user_databases.html   # Manage databases
│   └── user_ftp.html         # Manage FTP accounts

pkg/hestia/
├── client.go                 # SSH connection, command execution
└── commands.go               # v-add-user, v-add-domain, etc.
```

## HestiaCP API Commands

```bash
# Account management
v-add-user {username} {password} {email} {package} {name}
v-delete-user {username}
v-suspend-user {username}
v-unsuspend-user {username}
v-change-user-package {username} {package}

# Domains
v-add-domain {username} {domain}
v-delete-domain {username} {domain}
v-list-domains {username} json

# SSL
v-add-letsencrypt-domain {username} {domain}
v-delete-letsencrypt-domain {username} {domain}

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
1. User visits /user/hosting
2. Sees packages list (if no account yet)
3. Clicks "Buy" on a package
4. System checks balance >= price
5. If sufficient:
   - Loadbalancer picks server (least accounts, under max)
   - Generate username (e.g., u12345)
   - Generate secure password (16 chars)
   - SSH: v-add-user on selected server
   - Create hosting_account record
   - Deduct balance, log transaction
   - Set next_billing_at = now + 30 days
   - Show credentials + dashboard
6. If insufficient: "Insufficient balance, contact admin"

### Monthly Billing Flow (cron every hour)
1. Find accounts where next_billing_at <= now AND status = active
2. For each account:
   - Get price (custom_price or package.price_monthly)
   - Check user balance >= price
   - If sufficient:
     - Deduct balance, log transaction
     - Set next_billing_at += 30 days
   - If insufficient:
     - SSH: v-suspend-user
     - Set status = suspended

### Reactivation Flow
1. Admin tops up user balance
2. User visits /user/hosting (sees "Suspended")
3. Clicks "Reactivate"
4. System checks balance >= price
5. If sufficient:
   - Deduct balance
   - SSH: v-unsuspend-user
   - Set status = active, next_billing_at = now + 30 days

## Admin Flows

### Server Management (/admin/hosting/servers)
- List all HestiaCP servers (name, hostname, accounts used/max, status)
- Add server: hostname, port, SSH user, password, max accounts
- Test connection button (SSH handshake + v-list-users)
- Edit/disable server
- View accounts on server

### Package Management (/admin/hosting/packages)
- List packages (name, price, specs, active/inactive)
- Add/edit package with all resource limits
- Reorder packages
- Disable package (existing accounts keep it)

### Accounts Overview (/admin/hosting/accounts)
- List all hosting accounts with filters
- Actions: suspend, unsuspend, cancel
- Create custom quote: select user, server, custom resources + price

### Balance Management (extend /admin/users)
- Add "Balance" column to users table
- "Top Up" button per user with amount modal
- Balance transaction history

## Load Balancer

```go
func (lb *LoadBalancer) PickServer() (*HostingServer, error) {
    // 1. Get all active servers
    // 2. Filter: current_accounts < max_accounts
    // 3. Sort by: current_accounts ASC (least loaded first)
    // 4. Return first, or error if none available
}
```

## Error Handling

| Scenario | Action |
|----------|--------|
| SSH connection fails | Retry 2x, mark server unhealthy, alert admin |
| HestiaCP command fails | Parse error, user-friendly message, log full output |
| No servers available | "No hosting servers available, contact admin" |
| Server at capacity | Skip to next server |
| User creation fails | Don't deduct balance, rollback, show error |
| Domain already exists | "Domain already in use on this server" |
| SSL fails | Domain added, SSL marked pending, user can retry |

## Security

- Server passwords: AES-256 encrypted in DB, key from HOSTING_ENCRYPTION_KEY env
- User passwords: 16 chars, alphanumeric + symbols, stored encrypted
- SSH timeouts: 10s connection, 30s command
- Input validation: domain format, email format, username alphanumeric

## Environment Variables

```
HOSTING_ENCRYPTION_KEY=<32-byte-key-for-aes-256>
```

## Menu Items

**User sidebar:**
- Hosting (icon: bi-hdd-stack) → /user/hosting

**Admin sidebar:**
- Hosting Servers → /admin/hosting/servers
- Hosting Packages → /admin/hosting/packages
- Hosting Accounts → /admin/hosting/accounts
