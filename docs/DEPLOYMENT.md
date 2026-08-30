# Botginx Deployment Guide

## Overview

Botginx can be deployed to a Tor-only VPS using two scripts:

| Script | Purpose |
|--------|---------|
| `deploy-onion.sh` | Full server setup (Tor, PostgreSQL, Go, Nginx, systemd) |
| `auto-deploy.sh` | Automatic updates from GitHub releases (runs via cron) |

## Quick Start

```bash
# Deploy to a fresh VPS
DEPLOY_BRANCH="main" ssh root@YOUR_SERVER 'bash -s' < deploy-onion.sh
```

## deploy-onion.sh

Full deployment script that sets up everything from scratch.

### What It Does

1. **Tor Hidden Service** — Generates .onion address, configures torrc
2. **PostgreSQL 16** — Creates database and user
3. **Go 1.23** — Installs Go toolchain
4. **Build** — Compiles botginx binary from source
5. **Systemd** — Creates and enables botginx.service
6. **Nginx** — Reverse proxy on port 80 → 3001
7. **Firewall** — UFW rules for SSH and HTTP
8. **Auto-deploy** — Registers cron job for updates

### Usage

```bash
# Basic deployment
DEPLOY_BRANCH="main" ssh root@SERVER 'bash -s' < deploy-onion.sh

# With antibot (botection) integration
ANTIBOT_MODE=true DEPLOY_BRANCH="main" ssh root@SERVER 'bash -s' < deploy-onion.sh

# Custom database password
DB_PASS="your-secure-password" DEPLOY_BRANCH="main" ssh root@SERVER 'bash -s' < deploy-onion.sh
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DEPLOY_BRANCH` | (required) | Git branch to deploy |
| `APP_DIR` | `/var/www/botginx` | Installation directory |
| `DB_PASS` | (random) | PostgreSQL password |
| `SESSION_SECRET` | (random) | Session encryption key |
| `ANTIBOT_MODE` | `false` | Enable botection integration |
| `SSH_DEPLOY_KEY` | (none) | Base64-encoded SSH private key |

### Traffic Flow

**Direct mode (default):**
```
Tor → :80 (nginx) → :3001 (botginx)
```

**Antibot mode (`ANTIBOT_MODE=true`):**
```
Tor → :8080 (botection) → :3001 (botginx)
```

### Files Created

| Path | Description |
|------|-------------|
| `/var/www/botginx/` | Application directory |
| `/var/www/botginx/.env` | Environment configuration |
| `/var/www/botginx/botginx` | Compiled binary |
| `/var/www/botginx/logs/` | Application logs |
| `/var/lib/tor/botginx/` | Tor hidden service keys (BACKUP!) |
| `/root/botginx-credentials.txt` | Database credentials |
| `/etc/systemd/system/botginx.service` | Systemd unit |
| `/etc/nginx/sites-available/botginx` | Nginx config |

### Post-Deploy

```bash
# Check service status
systemctl status botginx

# View logs
journalctl -u botginx -f
tail -f /var/www/botginx/logs/botginx.log

# Restart
systemctl restart botginx

# Get .onion address
cat /var/lib/tor/botginx/hostname
```

---

## auto-deploy.sh

Automatic update script that runs every 2 minutes via cron.

### What It Does

1. **Git Pull** — Updates scripts/templates (preserves .env and data/)
2. **Check Release** — Queries GitHub API for latest release
3. **Download Binary** — Fetches prebuilt binary for architecture
4. **Verify Checksum** — Validates SHA256 hash
5. **Atomic Swap** — Replaces binary with mv (no partial state)
6. **Restart Service** — systemctl restart + health check

### How It Works

```
Every 2 minutes:
  1. Is there a newer release tag than installed version?
     └─ NO → exit (nothing to do)
     └─ YES → continue
  
  2. Download botginx-linux-{amd64,arm64} asset
  
  3. Verify SHA256SUMS checksum
     └─ FAIL → abort, keep old binary
  
  4. Test new binary executes
     └─ FAIL → abort, keep old binary
  
  5. mv binary.new → binary (atomic)
  
  6. systemctl restart botginx
  
  7. Verify service is running
     └─ FAIL → log error, exit 1
     └─ OK → log success
```

### Cron Entry

Registered automatically by deploy-onion.sh:

```cron
*/2 * * * * DEPLOY_BRANCH="main" /var/www/botginx/auto-deploy.sh >> /var/www/botginx/logs/auto-deploy.log 2>&1
```

### Manual Run

```bash
# Test auto-deploy manually
DEPLOY_BRANCH="main" /var/www/botginx/auto-deploy.sh

# View auto-deploy logs
tail -f /var/www/botginx/logs/auto-deploy.log
```

### Private Repository

For private repos, set `GITHUB_TOKEN` in `.env`:

```bash
# In /var/www/botginx/.env
GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxx
```

---

## GitHub Releases Setup

**For auto-deploy to work, you must create GitHub releases with prebuilt binaries.**

### Required Release Assets

Each release must include:

| Asset | Description |
|-------|-------------|
| `botginx-linux-amd64` | Binary for x86_64 servers |
| `botginx-linux-arm64` | Binary for ARM64 servers |
| `SHA256SUMS` | Checksums file |

### Building Release Binaries

```bash
# Build for amd64
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=v1.0.0" -o botginx-linux-amd64 ./cmd/server

# Build for arm64
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=v1.0.0" -o botginx-linux-arm64 ./cmd/server

# Generate checksums
sha256sum botginx-linux-* > SHA256SUMS
```

### SHA256SUMS Format

```
a1b2c3d4e5f6...  botginx-linux-amd64
f6e5d4c3b2a1...  botginx-linux-arm64
```

### Creating a Release

1. Tag the commit: `git tag v1.0.0 && git push --tags`
2. Go to GitHub → Releases → Draft new release
3. Select the tag
4. Upload: `botginx-linux-amd64`, `botginx-linux-arm64`, `SHA256SUMS`
5. Publish release

### GitHub Actions (Recommended)

Add `.github/workflows/release.yml` to automate:

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

jobs:
  build:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        goos: [linux]
        goarch: [amd64, arm64]
    
    steps:
      - uses: actions/checkout@v4
      
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'
      
      - name: Build
        env:
          GOOS: ${{ matrix.goos }}
          GOARCH: ${{ matrix.goarch }}
        run: |
          CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=${{ github.ref_name }}" \
            -o botginx-${{ matrix.goos }}-${{ matrix.goarch }} ./cmd/server
      
      - uses: actions/upload-artifact@v4
        with:
          name: botginx-${{ matrix.goos }}-${{ matrix.goarch }}
          path: botginx-${{ matrix.goos }}-${{ matrix.goarch }}

  release:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4
      
      - name: Generate checksums
        run: |
          sha256sum botginx-*/botginx-* > SHA256SUMS
          cat SHA256SUMS
      
      - uses: softprops/action-gh-release@v1
        with:
          files: |
            botginx-*/botginx-*
            SHA256SUMS
```

---

## Troubleshooting

### Service won't start

```bash
# Check logs
journalctl -u botginx -n 50 --no-pager

# Check binary runs
/var/www/botginx/botginx -version

# Check .env file
cat /var/www/botginx/.env
```

### Auto-deploy not running

```bash
# Check cron is registered
crontab -l | grep auto-deploy

# Check logs
tail -50 /var/www/botginx/logs/auto-deploy.log

# Test manually
DEPLOY_BRANCH="main" /var/www/botginx/auto-deploy.sh
```

### Can't access .onion

```bash
# Check Tor is running
systemctl status tor

# Check hidden service
cat /var/lib/tor/botginx/hostname

# Check nginx
nginx -t
systemctl status nginx
```

### GitHub rate limiting

```bash
# Add token to .env
echo "GITHUB_TOKEN=ghp_xxx" >> /var/www/botginx/.env
```

---

## Backup & Restore

### Critical Files to Backup

```bash
# Tor hidden service keys (preserves .onion address)
/var/lib/tor/botginx/

# Application config
/var/www/botginx/.env

# Database
pg_dump -U botginx botginx_db > backup.sql
```

### Restore Tor Keys

```bash
# Stop Tor
systemctl stop tor

# Restore keys
cp -r backup/tor/botginx /var/lib/tor/
chown -R debian-tor:debian-tor /var/lib/tor/botginx
chmod 700 /var/lib/tor/botginx

# Restart Tor
systemctl start tor

# Verify same .onion address
cat /var/lib/tor/botginx/hostname
```

---

## Analytics Setup

For redirect link analytics to work, the deploy VPS's botection must send webhooks to the panel.

### Deploy VPS Setup

When running `deploy-vps-setup.sh`, provide `PANEL_URL`:

```bash
PANEL_URL=https://your-panel-domain.com ssh root@DEPLOY_VPS 'bash -s' < deploy-vps-setup.sh
```

This configures:
- `panel_callback` - botection calls panel for blocking decisions
- `webhooks` - botection sends visit data to panel

The script outputs a `WEBHOOK_SECRET` - add it to the panel's `.env`:

```bash
# On panel VPS
echo "ANTIBOT_WEBHOOK_SECRET=<secret-from-output>" >> /etc/botginx/botginx.env
systemctl restart botginx
```

### Panel VPS Botection Bypass Paths

If botection runs in front of the panel, add these to bypass paths:

```yaml
# /var/www/antibot/config/config.yaml on PANEL VPS
server:
  api_bypass_paths:
    - "/webhooks/"        # Webhook endpoint
    - "/api/botection/"   # Callback endpoint
    - "/api/"             # Panel API
    - "/.well-known/"
    - "/health"
```

Then restart: `systemctl restart botection`

### Verification

After setup, visits to redirect links should appear in analytics:

```bash
# Check visits in database
sudo -u postgres psql -d botginx -c "SELECT COUNT(*) FROM visits;"
```
