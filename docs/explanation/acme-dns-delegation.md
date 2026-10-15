# ACME-DNS Delegation Plan

## Scope

**This plan applies ONLY to the Redirect Links module (`modules/domains/` + `modules/redirectlinks/`).**

```
┌─────────────────────────────────────────────────────────────────────┐
│                    TWO INDEPENDENT SYSTEMS                          │
├──────────────────────────────────┬──────────────────────────────────┤
│  HOSTING MODULE                  │  REDIRECT LINKS MODULE           │
│  (NOT covered by this plan)      │  (THIS PLAN)                     │
├──────────────────────────────────┼──────────────────────────────────┤
│  modules/hosting/                │  modules/domains/                │
│  Table: hosting_servers          │  modules/redirectlinks/          │
│  Admin: /admin/hosting/servers   │  Table: servers                  │
│  Server: CloudPanel VPS          │  Admin: /admin/servers           │
│  Purpose: Website hosting        │  Server: Deploy VPS              │
│  SSL: CloudPanel Let's Encrypt   │  Purpose: Redirect links         │
│                                  │  SSL: Certbot + acme-dns         │
└──────────────────────────────────┴──────────────────────────────────┘

These systems are COMPLETELY INDEPENDENT. Do not confuse them.
```

## Problem

Wildcard SSL certificates (`*.domain.com`) for **redirect links** require DNS-01 challenge. Current flow fails because:
1. Certbot generates a NEW token each run
2. User adds OLD pre-generated token to DNS
3. Token mismatch → SSL generation fails

## Solution

**CNAME Delegation** - User points `_acme-challenge` to our DNS server. We control the TXT value.

```
User's DNS (cPanel/Cloudflare/any):
_acme-challenge.example.com  CNAME  abc123.acme.guardbot.sbs

Our DNS server (on Deploy VPS):
abc123.acme.guardbot.sbs  TXT  "actual-certbot-token"
```

## Server Management (Redirect Links Only)

**Admin adds Deploy Servers via `/admin/servers`:**
```
┌─────────────────────────────────────────────────────────────────────┐
│  Admin Panel → Servers → Add Server                                 │
├─────────────────────────────────────────────────────────────────────┤
│  Name:         Deploy VPS 1                                         │
│  IP:           <dynamic - admin enters>                             │
│  Port:         22                                                   │
│  SSH User:     root                                                 │
│  SSH Password: ********                                             │
│  Status:       ready  ← Must be 'ready' to be used                 │
└─────────────────────────────────────────────────────────────────────┘

-- Database: servers table
SELECT ip, port, ssh_user, ssh_password 
FROM servers 
WHERE status = 'ready' 
ORDER BY created_at LIMIT 1;
```

**Server can change anytime** - Admin can:
- Add new servers
- Remove old servers  
- Change server IPs
- The system dynamically picks the first 'ready' server

## System Architecture (Redirect Links Module)

```
┌─────────────────────────────────────────────────────────────────────┐
│                  REDIRECT LINKS ARCHITECTURE                         │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌──────────────────┐         SSH          ┌──────────────────────┐ │
│  │  Guardbot VPS    │ ───────────────────► │  Deploy VPS          │ │
│  │  (Botginx Panel) │                      │  (from `servers`)    │ │
│  │                  │  - Push settings     │                      │ │
│  │  - Database      │  - Generate SSL      │  - Botection proxy   │ │
│  │  - User mgmt     │  - Setup nginx       │  - Nginx + SSL certs │ │
│  │  - Domain mgmt   │  - Update acme-dns   │  - /etc/botection/   │ │
│  │                  │                      │  - acme-dns server   │ │
│  └──────────────────┘                      └──────────────────────┘ │
│         │                                          ▲               │
│         │ GetDeployServer()                        │               │
│         ▼                                  User's wildcard domain   │
│  ┌──────────────────┐                      *.example.com            │
│  │  `servers` table │                      A → <Deploy VPS IP>      │
│  │  status='ready'  │                                               │
│  └──────────────────┘                                               │
│                                                                      │
│  Code paths:                                                         │
│  - modules/domains/services/domain_service.go → GetDeployServer()  │
│  - modules/domains/services/verification_service.go → SSL/nginx    │
│  - modules/redirectlinks/ → uses domains for link generation       │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘

Deploy VPS (from `servers` table) handles:
- Incoming traffic for *.example.com (redirect links)
- SSL certificates (generated via certbot)
- acme-dns server for CNAME delegation
- Botection reverse proxy (bot filtering)
- Nginx serving redirect links
```

**NOT to be confused with:**
```
┌─────────────────────────────────────────────────────────────────────┐
│                  HOSTING MODULE (SEPARATE SYSTEM)                    │
├─────────────────────────────────────────────────────────────────────┤
│  modules/hosting/                                                    │
│  Table: `hosting_servers`                                            │
│  Server: CloudPanel VPS                                              │
│  SSL: CloudPanel's built-in Let's Encrypt                           │
│  Purpose: User website hosting (PHP, MySQL, etc.)                   │
│  This plan does NOT affect the hosting module!                      │
└─────────────────────────────────────────────────────────────────────┘
```

## Domain Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│                         User Flow                                    │
├─────────────────────────────────────────────────────────────────────┤
│  1. User adds domain *.example.com in Botginx panel                 │
│  2. Botginx SSH → Deploy VPS: register with acme-dns                │
│  3. System shows user 3 DNS records to add:                         │
│                                                                      │
│     A record:                                                        │
│       Name: *                                                        │
│       Value: <DEPLOY_SERVER_IP> (Deploy VPS)                            │
│                                                                      │
│     TXT record (ownership verification):                            │
│       Name: _guardbot-verify                                        │
│       Value: gbot_abc123xyz                                         │
│                                                                      │
│     CNAME record (SSL delegation - one-time):                       │
│       Name: _acme-challenge                                         │
│       Value: abc123.acme.guardbot.sbs                               │
│                                                                      │
│  4. User adds all 3 records in cPanel/Cloudflare/any DNS            │
│  5. System verifies all records are present                         │
│  6. User clicks "Generate SSL"                                       │
│  7. Botginx SSH → Deploy VPS:                                       │
│     a. Update TXT on acme-dns (localhost:8053)                      │
│     b. Run certbot with DNS-01 challenge                            │
│     c. SSL certificate generated on Deploy VPS                      │
│  8. Done - SSL active, no user action during generation             │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│                      DNS Resolution Flow                             │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Let's Encrypt asks: _acme-challenge.example.com TXT?               │
│           │                                                          │
│           ▼                                                          │
│  User's DNS (cPanel): CNAME → abc123.acme.guardbot.sbs              │
│           │                                                          │
│           ▼                                                          │
│  acme-dns on Deploy VPS: TXT "LH1vRZT...certbot-token..."           │
│           │                                                          │
│           ▼                                                          │
│  Let's Encrypt: ✓ Verified → Issue wildcard certificate             │
│           │                                                          │
│           ▼                                                          │
│  Certificate saved to Deploy VPS: /etc/letsencrypt/live/example.com │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

## Redirect Link Traffic Flow

After SSL is set up, redirect links work like this:

```
┌─────────────────────────────────────────────────────────────────────┐
│                    Redirect Link Traffic                             │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Visitor: https://promo.example.com/abc123                          │
│           │                                                          │
│           ▼                                                          │
│  DNS: *.example.com A → <DEPLOY_SERVER_IP>                              │
│           │                                                          │
│           ▼                                                          │
│  ┌─────────────────────────────────────────┐                        │
│  │  Deploy VPS (<DEPLOY_SERVER_IP>)            │                        │
│  │                                          │                        │
│  │  Nginx (port 443)                        │                        │
│  │    ↓ SSL termination                     │                        │
│  │  Botection (port 8080)                   │                        │
│  │    ↓ Bot check using /etc/botection/links/abc123.json           │
│  │    ↓ Pass or block                       │                        │
│  │  Redirect to destination URL             │                        │
│  │                                          │                        │
│  └─────────────────────────────────────────┘                        │
│           │                                                          │
│           ▼                                                          │
│  Visitor redirected to: https://target-site.com/offer               │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

## Components

### 1. acme-dns Server

Lightweight DNS server that only handles TXT records for ACME challenges.

**Option A: Use existing acme-dns** (https://github.com/joohoi/acme-dns)
- Battle-tested, widely used
- REST API for updating TXT records
- SQLite/PostgreSQL storage
- Easy to deploy

**Option B: Build minimal DNS server**
- More control, less dependencies
- Only need TXT record responses
- Can integrate directly with botginx DB

**Recommendation: Option A** - Use acme-dns, proven and reliable.

### 2. Database Schema

```sql
-- Store ACME DNS delegation info per domain
ALTER TABLE domains ADD COLUMN acme_subdomain TEXT;        -- e.g., "abc123"
ALTER TABLE domains ADD COLUMN acme_password TEXT;         -- API password for updates
ALTER TABLE domains ADD COLUMN acme_cname_verified BOOLEAN DEFAULT false;
```

### 3. Botginx Integration

**New service:** `pkg/acmedns/client.go`
```go
type Client struct {
    ServerURL string  // https://acme.guardbot.sbs
}

func (c *Client) Register(domain string) (*Registration, error)
func (c *Client) UpdateTXT(subdomain, password, value string) error
```

**Modified SSL flow:**
```go
func (s *VerificationService) GenerateWildcardSSLWithACME(domain string) error {
    // 1. Get acme-dns credentials from DB
    // 2. Update TXT record via acme-dns API
    // 3. Run certbot with DNS-01 challenge
    // 4. Certbot verifies → success
}
```

### 4. UI Changes

**Domain Setup Page:**

```
Step 1: A Record
  Type: A
  Name: *
  Value: <DEPLOY_SERVER_IP>  ✓ Verified

Step 2: Ownership Verification
  Type: TXT
  Name: _guardbot-verify
  Value: gbot_abc123xyz  ✓ Verified

Step 3: SSL Delegation (NEW - one-time setup)
  Type: CNAME
  Name: _acme-challenge
  Value: abc123.acme.guardbot.sbs  ✓ Verified
  
  [Generate SSL] ← Now 100% reliable
```

## Implementation Steps

### Phase 1: Deploy acme-dns Server on Deploy Server

**Target:** The Deploy Server from `servers` table (admin-managed, can change)

**Option A: Manual Setup (one-time per server)**
```bash
# Get server IP from admin panel or database
# SSH to Deploy Server and install acme-dns

# Install acme-dns
wget https://github.com/joohoi/acme-dns/releases/download/v1.0/acme-dns_1.0_linux_amd64.tar.gz
tar xzf acme-dns_1.0_linux_amd64.tar.gz
mv acme-dns /usr/local/bin/
mkdir -p /etc/acme-dns /var/lib/acme-dns
```

**Option B: Auto-Setup via Botginx (recommended)**

Create a setup script that Botginx runs via SSH when:
- A new server is added with status='ready'
- Admin clicks "Setup acme-dns" button

```go
// pkg/acmedns/setup.go
func SetupOnServer(serverIP string, sshClient *sshexec.Client) error {
    // 1. Install acme-dns binary
    // 2. Create config with dynamic IP
    // 3. Setup systemd service
    // 4. Open firewall
    // 5. Return success
}
```

**DNS Records on guardbot.sbs:**

Admin must update guardbot.sbs DNS when server changes:
```
acme.guardbot.sbs    A      <DEPLOY_SERVER_IP>   (from servers table)
acme.guardbot.sbs    NS     acme.guardbot.sbs
```

**NOTE:** When Deploy Server IP changes, admin must:
1. Update the server in `/admin/servers`
2. Update `acme.guardbot.sbs` A record to new IP
3. Run acme-dns setup on new server

**acme-dns config** (`/etc/acme-dns/config.cfg` on Deploy Server):
```toml
[general]
listen = "0.0.0.0:53"
protocol = "both"
domain = "acme.guardbot.sbs"
nsname = "acme.guardbot.sbs"
nsadmin = "admin.guardbot.sbs"
# IP is read from A record, not hardcoded here

[database]
engine = "sqlite3"
connection = "/var/lib/acme-dns/acme-dns.db"

[api]
listen = "127.0.0.1:8053"
disable_registration = false

[logconfig]
loglevel = "info"
logformat = "text"
```

**Systemd service** (`/etc/systemd/system/acme-dns.service`):
```ini
[Unit]
Description=acme-dns server
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/acme-dns -c /etc/acme-dns/config.cfg
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
systemctl daemon-reload
systemctl enable acme-dns
systemctl start acme-dns
ufw allow 53/udp
ufw allow 53/tcp
```

### Phase 2: Botginx Integration

Botginx (on Guardbot VPS) connects to Deploy VPS via SSH to:
- Register domains with acme-dns
- Update TXT records before SSL generation
- Run certbot for SSL certificate

1. **Add acme-dns SSH client** (`pkg/acmedns/client.go`)
   ```go
   // Client manages acme-dns via SSH to Deploy VPS
   type Client struct {
       sshClient *sshexec.Client
   }
   
   // Register creates a new subdomain on acme-dns
   func (c *Client) Register(domain string) (*Registration, error) {
       // SSH to Deploy VPS
       // curl -X POST http://127.0.0.1:8053/register
       // Returns subdomain, password, fulldomain
   }
   
   // UpdateTXT sets the TXT record value
   func (c *Client) UpdateTXT(subdomain, password, value string) error {
       // SSH to Deploy VPS  
       // curl -X POST http://127.0.0.1:8053/update -d '{"subdomain":"...","txt":"..."}'
   }
   ```

2. **Update domain model**
   - Add `acme_subdomain`, `acme_password`, `acme_cname_verified`
   - Migration file

3. **Update domain service**
   - On domain create: SSH to Deploy VPS, register with acme-dns
   - Return CNAME instructions to user

4. **Update verification service**
   - New method: `GenerateWildcardSSLWithACME()`
   - SSH to Deploy VPS:
     1. Update TXT via acme-dns API (localhost:8053)
     2. Run certbot with DNS-01 challenge
     3. SSL cert generated locally on Deploy VPS

### Phase 3: UI Updates

1. **Setup wizard** - Add Step 3 for CNAME delegation
2. **CNAME verification** - Check if CNAME resolves correctly
3. **Generate SSL button** - Now works 100% reliably
4. **Progress indicator** - Show "Updating DNS... Running certbot..."

## File Changes

**Only affects Redirect Links module (modules/domains/) - NOT hosting module!**

```
pkg/acmedns/
  client.go              # acme-dns API client (SSH-based)
  setup.go               # Auto-setup acme-dns on Deploy VPS

modules/domains/          # ← REDIRECT LINKS domains
  migrations/
    00X_add_acme_dns.sql # New columns: acme_subdomain, acme_password, acme_cname_verified
  services/
    domain_service.go    # Register with acme-dns on domain create
    verification_service.go  # New GenerateWildcardSSLWithACME() method
  handlers/
    handler.go           # CNAME verification endpoint
  templates/
    external_setup.html  # Add Step 3: CNAME delegation

modules/servers/          # Admin server management (Deploy VPS)
  templates/
    show.html            # Add "Setup acme-dns" button

scripts/
  setup-acmedns.sh       # Manual setup script for Deploy VPS
```

**NOT CHANGED:**
```
modules/hosting/          # CloudPanel hosting - UNCHANGED
modules/hosting/services/ # Uses hosting_servers table - UNCHANGED
```

## Security Considerations

1. **acme-dns API** - Only accessible from localhost (nginx proxy with auth)
2. **Per-domain passwords** - Each domain has unique acme-dns password
3. **HTTPS** - acme-dns API behind nginx with SSL
4. **Rate limiting** - Prevent abuse of DNS updates

## Rollout Plan

1. **Phase 1: Deploy acme-dns on current Deploy Server**
   - Get current server IP from `/admin/servers` (status='ready')
   - Install acme-dns binary on that server
   - Add DNS records to guardbot.sbs: `acme.guardbot.sbs A <SERVER_IP>`
   - Test: `dig abc123.acme.guardbot.sbs TXT` should resolve
   - Test API: `curl -X POST http://127.0.0.1:8053/register`

2. **Phase 2: Botginx integration**
   - Add `pkg/acmedns/client.go` (SSH-based, uses GetDeployServer())
   - Add `pkg/acmedns/setup.go` (auto-setup acme-dns on new servers)
   - Add database migration for acme fields
   - Update verification service with new SSL method
   - Test with new domain

3. **Phase 3: UI updates**
   - Add CNAME step to setup wizard
   - CNAME verification check  
   - GetDeployIP() for showing server IP in instructions
   - Update progress indicators

4. **Phase 4: Admin server management**
   - Add "Setup acme-dns" button in server details page
   - Warning when server IP changes: "Update acme.guardbot.sbs DNS"
   - Auto-detect if acme-dns is installed on server

5. **Phase 5: Migration**
   - Generate acme-dns credentials for existing domains
   - Notify users to add CNAME record
   - Retry SSL for domains that previously failed

## Success Metrics

- SSL generation success rate: 100% (vs current ~30%)
- User setup time: < 5 minutes
- No manual intervention needed after CNAME setup

## Handling Server Changes

When admin changes the Deploy Server:

1. **Add new server in `/admin/servers`**
2. **Run acme-dns setup on new server** (button or auto)
3. **Update guardbot.sbs DNS:**
   ```
   acme.guardbot.sbs  A  <NEW_SERVER_IP>
   ```
4. **Wait for DNS propagation** (~5 min)
5. **Set new server status to 'ready'**
6. **Set old server status to 'disconnected'**

**Important:** User's CNAME records (`_acme-challenge.example.com → abc123.acme.guardbot.sbs`) 
don't need to change - they point to the subdomain, not the IP. Only the A record for 
`acme.guardbot.sbs` needs updating.

## Fallback

If acme-dns server is down:
- Show error: "SSL service temporarily unavailable"
- Domains with existing valid SSL continue working
- Queue failed generations for retry when service recovers

---

## Deploy Script Updates

When provisioning a fresh Deploy VPS, the deploy scripts must automatically install and configure acme-dns.

### Files to Update

| Script | Purpose | Changes Needed |
|--------|---------|----------------|
| `deploy.sh` | Production deploy (Guard VPS) | No changes (Guard VPS doesn't run acme-dns) |
| `deploy-cloudpanel.sh` | Provision CloudPanel hosting | No changes (Hosting module, not redirect links) |
| `deploy-hestia.sh` | Provision HestiaCP hosting | No changes (Hosting module) |
| **New: `setup-deploy-vps.sh`** | Provision fresh Deploy VPS | Install nginx, lego, acme-dns, firewall |

### New Script: setup-deploy-vps.sh

Create a script to provision a fresh Deploy VPS with all required components:

```bash
#!/bin/bash
# setup-deploy-vps.sh - Provision a fresh Deploy VPS for redirect links
# Usage: SSH_HOST=x.x.x.x SSH_USER=root SSH_PASS=xxx ./setup-deploy-vps.sh

set -e

# Required environment variables
: "${SSH_HOST:?SSH_HOST is required}"
: "${SSH_USER:?SSH_USER is required}"
: "${SSH_PASS:?SSH_PASS is required}"
: "${ACME_DOMAIN:=acme.guardbot.sbs}"

echo "=== Provisioning Deploy VPS at $SSH_HOST ==="

sshpass -p "$SSH_PASS" ssh -o StrictHostKeyChecking=no $SSH_USER@$SSH_HOST bash << 'REMOTE_SCRIPT'
set -e

echo ">>> Installing dependencies..."
apt-get update
apt-get install -y nginx certbot curl jq

echo ">>> Installing lego v5.3.0..."
cd /tmp
curl -fsSL "https://github.com/go-acme/lego/releases/download/v5.3.0/lego_v5.3.0_linux_amd64.tar.gz" -o lego.tar.gz
tar xzf lego.tar.gz lego
mv -f lego /usr/local/bin/
chmod +x /usr/local/bin/lego
rm -f lego.tar.gz
lego --version

echo ">>> Installing acme-dns..."
cd /tmp
curl -fsSL "https://github.com/joohoi/acme-dns/releases/download/v1.0/acme-dns_1.0_linux_amd64.tar.gz" -o acme-dns.tar.gz
tar xzf acme-dns_1.0_linux_amd64.tar.gz
mv -f acme-dns /usr/local/bin/
chmod +x /usr/local/bin/acme-dns
rm -f acme-dns.tar.gz

echo ">>> Configuring acme-dns..."
mkdir -p /etc/acme-dns /var/lib/acme-dns

# Get server's public IP
SERVER_IP=$(curl -s https://api.ipify.org)

cat > /etc/acme-dns/config.cfg << EOF
[general]
listen = "0.0.0.0:53"
protocol = "both"
domain = "ACME_DOMAIN_PLACEHOLDER"
nsname = "ACME_DOMAIN_PLACEHOLDER"
nsadmin = "admin.guardbot.sbs"
records = [
    "ACME_DOMAIN_PLACEHOLDER. A $SERVER_IP",
]
debug = false

[database]
engine = "sqlite3"
connection = "/var/lib/acme-dns/acme-dns.db"

[api]
ip = "127.0.0.1"
port = "8053"
tls = "none"

[logconfig]
loglevel = "info"
logformat = "text"
EOF

# Replace placeholder (done outside heredoc to preserve variable)
sed -i "s/ACME_DOMAIN_PLACEHOLDER/$ACME_DOMAIN/g" /etc/acme-dns/config.cfg

echo ">>> Creating acme-dns systemd service..."
cat > /etc/systemd/system/acme-dns.service << 'EOF'
[Unit]
Description=acme-dns server for ACME DNS-01 challenges
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/acme-dns -c /etc/acme-dns/config.cfg
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable acme-dns
systemctl start acme-dns

echo ">>> Configuring firewall..."
ufw allow 22/tcp   # SSH
ufw allow 53/udp   # DNS
ufw allow 53/tcp   # DNS
ufw allow 80/tcp   # HTTP (Let's Encrypt validation)
ufw allow 443/tcp  # HTTPS
ufw --force enable

echo ">>> Creating directories..."
mkdir -p /var/www/sites
mkdir -p /etc/letsencrypt/live

echo ">>> Verifying installation..."
lego --version
systemctl status acme-dns --no-pager || true
curl -s http://127.0.0.1:8053/health || echo "(acme-dns health check - may need DNS configured)"

echo ">>> Deploy VPS provisioning complete!"
echo ""
echo "NEXT STEPS:"
echo "1. Add DNS records for acme.guardbot.sbs:"
echo "   acme.guardbot.sbs  A   $SERVER_IP"
echo "   acme.guardbot.sbs  NS  acme.guardbot.sbs"
echo ""
echo "2. Add this server to botginx panel at /admin/servers"
echo ""
echo "3. Test acme-dns: dig @$SERVER_IP acme.guardbot.sbs"
REMOTE_SCRIPT

echo "=== Deploy VPS provisioning complete ==="
```

### Integration with Existing Deploy Scripts

The existing `deploy.sh` deploys the **botginx binary to Guard VPS**. It should NOT be modified for acme-dns since Guard VPS doesn't run acme-dns.

**Recommended approach:**
1. Keep `deploy.sh` for Guard VPS (botginx panel)
2. Add `setup-deploy-vps.sh` for Deploy VPS (nginx + lego + acme-dns)
3. Document the two-server setup in `docs/DEPLOYMENT.md`

### Auto-Setup via Botginx Panel (Future Enhancement)

Add a button in `/admin/servers` to auto-provision a new Deploy VPS:

```go
// POST /admin/servers/api/{id}/provision
func (h *Handler) APIProvisionServer(w http.ResponseWriter, r *http.Request) {
    // 1. SSH to server
    // 2. Run setup commands (nginx, lego, acme-dns)
    // 3. Return success/failure
}
```

This allows admins to add a fresh VPS IP and have botginx configure it automatically.
