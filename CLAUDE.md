# Botginx

Go-based redirect generator panel with AdminLTE v4.9.1, dark mode, red primary theme.

## Project Structure

```
cmd/server/          # Main application entry point
modules/             # Feature modules (auth, analytics, redirectlinks, etc.)
pkg/                 # Shared packages (subscription, module framework)
web/templates/       # HTML templates (base layout, partials)
docs/                # Documentation
```

## Key Features

- **Modular architecture** — Each feature is a self-contained module with routes, templates, migrations
- **Subscription system** — Admin grants access, lapsed users get view-only mode
- **Bot protection** — Per-link settings enforced via botection callback API
- **Analytics** — Visit tracking with country, device, bot detection
- **DataTables** — All tables use simple-datatables with search/sort/paging
- **Toast notifications** — All user feedback via toast system

## Deployment

### Auto-Deploy Requirements

**For auto-deploy to work, GitHub releases must include prebuilt binaries:**

| Asset | Description |
|-------|-------------|
| `botginx-linux-amd64` | Binary for x86_64 servers |
| `botginx-linux-arm64` | Binary for ARM64 servers |
| `SHA256SUMS` | Checksums file |

Build commands:
```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o botginx-linux-amd64 ./cmd/server
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o botginx-linux-arm64 ./cmd/server
sha256sum botginx-linux-* > SHA256SUMS
```

See `docs/DEPLOYMENT.md` for full deployment guide.

### Deploy Scripts

| Script | Purpose |
|--------|---------|
| `deploy.sh` | Production deploy with SSL (local build → remote VPS) |
| `deploy-onion.sh` | Full Tor VPS setup (PostgreSQL, Go, Nginx, systemd) |
| `auto-deploy.sh` | Auto-update from GitHub releases (cron every 2 min) |
| `deploy-cloudpanel.sh` | Provision CloudPanel hosting server for botginx |
| `deploy-hestia.sh` | Provision HestiaCP hosting server (legacy) |

Usage:
```bash
# Production (real domain with SSL)
cp deploy.env.example deploy.env  # Edit: SSH_HOST, PANEL_DOMAIN, CERTBOT_EMAIL
./deploy.sh --setup               # First time: installs PostgreSQL, nginx, SSL
./deploy.sh                       # Deploy binary

# Tor hidden service
DEPLOY_BRANCH="main" ssh root@SERVER 'bash -s' < deploy-onion.sh
```

### Antibot Mode

When botection is installed on the same VPS, set `ANTIBOT_MODE=true` in deploy.env:

```
Traffic: Internet → :443 (nginx) → :8080 (botection) → :3001 (botginx)
```

## Botection Integration

Botginx provides a callback API for botection (antibot reverse proxy):

- **Endpoint:** `POST /api/botection/should-block`
- **Purpose:** Per-link blocking decisions based on user settings
- **Docs:** `docs/BOTECTION-CALLBACK-API.md`

Botection calls this endpoint before making block decisions. Settings changes take effect within 30 seconds (cache TTL).

## Development

```bash
# Run locally
go run ./cmd/server

# Build
go build -o botginx ./cmd/server

# Test
go test ./...
```

## Environment Variables

**Path:** `.env` (gitignored, never commit)

### Application

| Variable | Description |
|----------|-------------|
| `UI_APP_NAME` | Display name in UI (default: GuardBot) |
| `PORT` | Server port (default: 3001) |
| `DATABASE_URL` | PostgreSQL connection string |
| `SESSION_SECRET` | Session encryption key |
| `ANTIBOT_WEBHOOK_SECRET` | Webhook signature verification |
| `TOR_MODE` | Enable Tor-specific behavior |

### Supabase (Global Whitelist Sync)

| Variable | Description |
|----------|-------------|
| `SUPABASE_URL` | Supabase project URL (e.g. `https://xxx.supabase.co`) |
| `SUPABASE_KEY` | Supabase service role key (for REST API) |
| `PANEL_URL` | This panel's hostname (default: `guardbot.sbs`) |

Global whitelist IPs sync to Supabase central server so all antibot instances can access them.

### Deploy VPS Servers

Used by settings push (`pkg/settingspush/`) to SCP link settings to deploy servers.

| Variable | Description |
|----------|-------------|
| `GUARD_VPS_IP` | Guard Bot VPS (botection server) IP |
| `GUARD_VPS_USER` | SSH user |
| `GUARD_VPS_PASSWORD` | SSH password |
| `GUARD_VPS_PORT` | SSH port (default: 22) |
| `DEPLOY_VPS_IP` | Deploy VPS (abrow_s4) IP |
| `DEPLOY_VPS_USER` | SSH user |
| `DEPLOY_VPS_PASSWORD` | SSH password |
| `DEPLOY_VPS_PORT` | SSH port (default: 22) |

### Telegram Notifications

| Variable | Description |
|----------|-------------|
| `TELEGRAM_BOT_TOKEN` | Telegram bot token from @BotFather |
| `TELEGRAM_CHAT_ID` | Chat/group ID to send notifications to |

Sends notifications for new support tickets and user replies.

### ACME DNS (Wildcard SSL)

| Variable | Description |
|----------|-------------|
| `ACME_DNS_DOMAIN` | Domain for acme-dns CNAME delegation (e.g. `acme.pamach.xyz`) |

## Related Documentation

- `docs/DEPLOYMENT.md` — Full deployment guide
- `docs/BOTECTION-CALLBACK-API.md` — Callback API specification
- `ENFORCEMENT-PLAN.md` — Bot protection architecture
