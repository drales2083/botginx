# CloudPanel Hosting Module

This guide covers setting up CloudPanel servers for the botginx hosting module with integrated antibot protection.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                         BOTGINX PANEL                               │
│  (Central management - users, accounts, domains, settings)          │
├─────────────────────────────────────────────────────────────────────┤
│                              │                                      │
│          SSH Push Settings   │   SSH Fetch Stats                    │
│                              ▼                                      │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │                    CLOUDPANEL SERVER                          │  │
│  │                                                               │  │
│  │  Internet → :443 → Botection (:8080) → CloudPanel (:8081)    │  │
│  │                                                               │  │
│  │  /etc/botection/links/{domain}.json  ← Settings from botginx │  │
│  │  127.0.0.1:8080/api/stats            → Stats to botginx      │  │
│  └───────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

## Components

| Component | Port | Purpose |
|-----------|------|---------|
| CloudPanel Panel | 8443 | Server admin panel (create sites, manage PHP/DB) |
| CloudPanel nginx | 8081 | Serves hosted websites (backend) |
| Botection | 8080 | Antibot reverse proxy (filters traffic) |
| External nginx/CF | 443 | SSL termination, routes to botection |

## Quick Setup

### 1. Deploy CloudPanel Server

```bash
# From your local machine
ssh root@NEW_SERVER 'bash -s' < deploy-cloudpanel.sh
```

This installs CloudPanel and prepares directories for botection integration.

### 2. Add Server to Botginx

In botginx panel: **Admin → Hosting → Servers → Add Server**

| Field | Value |
|-------|-------|
| Name | CloudPanel-{IP} |
| Type | CloudPanel |
| Hostname | Server IP |
| Panel URL | https://{IP}:8443 |
| Port | 22 |
| Username | root |
| Password | SSH password |

### 3. Deploy Botection

```bash
ssh root@SERVER
cd /var/www
git clone https://github.com/robertp2083/antibot.git
SERVER_TYPE=cloudpanel bash /var/www/antibot/packaging/deploy.sh
```

### 4. Configure Botection Callback

Edit `/var/www/antibot/config/config.yaml`:

```yaml
callback:
  url: https://YOUR_BOTGINX_PANEL/api/botection/should-block
  secret: your-webhook-secret
  cache_ttl: 30s
```

### 5. Route Traffic

Configure your edge (Cloudflare, external nginx) to route port 443 traffic to botection on port 8080.

## How Settings Work

### Settings Flow

1. User configures domain settings in botginx: **Hosting → Domain → Settings**
2. When saved, botginx pushes settings via SSH to the CloudPanel server
3. Settings are written to `/etc/botection/links/{domainID}.json`
4. Botection reads these settings when making block decisions

### Settings File Format

```json
{
  "domain": "example.com",
  "country_mode": "block",
  "country_list": ["CN", "RU"],
  "device_mode": "off",
  "device_list": [],
  "block_bots": true,
  "block_tor": true,
  "block_proxy": true,
  "block_datacenter": true,
  "block_headless": true,
  "min_behavior_score": 0,
  "redirect_on_block": "https://google.com"
}
```

### Settings Options

| Setting | Values | Description |
|---------|--------|-------------|
| country_mode | off, allow, block | Country filtering mode |
| country_list | ISO codes | Countries to allow/block |
| device_mode | off, allow, block | Device filtering mode |
| device_list | desktop, mobile, tablet | Devices to allow/block |
| block_bots | true/false | Block known bots |
| block_tor | true/false | Block Tor exit nodes |
| block_proxy | true/false | Block proxy/VPN IPs |
| block_datacenter | true/false | Block datacenter IPs |
| block_headless | true/false | Block headless browsers |
| min_behavior_score | 0-100 | Minimum behavior score required |
| redirect_on_block | URL | Redirect blocked visitors here |

## How Analytics Work

### Analytics Flow

1. User views domain settings in botginx: **Hosting → Domain → Settings**
2. Botginx fetches stats via SSH: `curl 127.0.0.1:8080/api/stats`
3. Stats are filtered by domain and displayed in the Analytics tab

### Available Metrics

| Metric | Description |
|--------|-------------|
| Total Requests | All requests to the domain |
| Allowed Requests | Requests that passed filtering |
| Blocked Requests | Requests that were blocked |
| Block Rate | Percentage of requests blocked |
| Block Reasons | Breakdown by block reason (bot, tor, proxy, etc.) |
| Recent Blocks | Last N blocked requests with IP, path, reason |

### Stats API Response

```json
{
  "total_requests": 15420,
  "blocked_requests": 1823,
  "allowed_requests": 13597,
  "block_rate": 11.82,
  "module_blocks": {
    "bot": 892,
    "tor": 156,
    "proxy": 423,
    "datacenter": 352
  },
  "recent_blocks": [
    {
      "time": "2024-01-15T10:23:45Z",
      "ip": "45.33.32.156",
      "host": "example.com",
      "path": "/wp-login.php",
      "reason": "bot_detected",
      "module": "bot"
    }
  ]
}
```

## Directory Structure

On the CloudPanel server:

```
/etc/botection/
├── links/                    # Domain settings (pushed from botginx)
│   ├── {domainID1}.json
│   ├── {domainID2}.json
│   └── ...
├── config/                   # Botection configuration
│   └── config.yaml
├── default-settings.json     # Default settings template
└── cloudpanel.env            # CloudPanel-specific env vars

/var/www/antibot/             # Botection installation
├── antibot                   # Binary
├── config/
│   └── config.yaml           # Main config
└── logs/
```

## Troubleshooting

### Settings Not Applying

1. Check SSH connection from botginx server:
   ```bash
   ssh root@CLOUDPANEL_SERVER "cat /etc/botection/links/{domainID}.json"
   ```

2. Verify botection is reading the settings:
   ```bash
   tail -f /var/www/antibot/logs/antibot.log
   ```

3. Check settings file permissions:
   ```bash
   ls -la /etc/botection/links/
   ```

### Analytics Not Loading

1. Check botection is running:
   ```bash
   systemctl status antibot
   ```

2. Test stats API locally:
   ```bash
   curl http://127.0.0.1:8080/api/stats
   ```

3. Check SSH connection from botginx.

### Traffic Not Routing Through Botection

1. Verify botection is listening:
   ```bash
   netstat -tlnp | grep 8080
   ```

2. Check CloudPanel nginx is on backend port:
   ```bash
   netstat -tlnp | grep 8081
   ```

3. Verify site configs have been modified:
   ```bash
   grep "listen 8081" /etc/nginx/sites-enabled/*.conf
   ```

## Security Notes

- SSH access is required from botginx to CloudPanel servers
- Use strong SSH passwords or SSH keys
- Botection stats API only listens on 127.0.0.1 (not exposed externally)
- Settings files are readable by botection process only

## Migration from Antibot-Dashboard

If you previously used antibot-dashboard:

1. **No action needed** - settings are now managed directly in botginx
2. Users access settings at: **Hosting → Domain → Settings**
3. Analytics are displayed in the same Settings page (Analytics tab)
4. You can remove antibot-dashboard from CloudPanel servers:
   ```bash
   systemctl stop antibot-dashboard
   systemctl disable antibot-dashboard
   rm -rf /home/clp/antibot-dashboard
   ```

## Custom Branding

CloudPanel can be customized with your own branding (logo, name, colors, dark theme).

### Quick Branding Setup

```bash
# Upload branding script
scp scripts/cloudpanel-branding.sh root@SERVER:/opt/

# Run with defaults (GuardHost name, red theme, dark mode)
ssh root@SERVER 'bash /opt/cloudpanel-branding.sh'

# Or with custom settings
ssh root@SERVER 'PANEL_NAME=MyHost PRIMARY_COLOR=#007bff bash /opt/cloudpanel-branding.sh'
```

### Branding Options

| Variable | Default | Description |
|----------|---------|-------------|
| PANEL_NAME | GuardHost | Custom panel name (replaces "CloudPanel") |
| PRIMARY_COLOR | #dc3545 | Theme color (hex) for buttons, links |
| FORCE_DARK_THEME | true | Set dark mode as default |
| LOGO_LIGHT | - | Path to light theme logo |
| LOGO_DARK | - | Path to dark theme logo |
| LOGO_FAVICON | - | Path to favicon |

### What Gets Customized

1. **Panel Name** - "CloudPanel" text replaced throughout UI
2. **Primary Color** - Buttons, links, accents use your color
3. **Dark Theme** - Set as default for all users
4. **Logos** - Custom logos and favicon (optional)

### Re-applying After Updates

The script creates a daily cron job that checks if branding needs to be re-applied after CloudPanel updates. You can also manually re-run:

```bash
ssh root@SERVER 'bash /opt/cloudpanel-branding.sh'
```

### Branding Files Location

```
/opt/cloudpanel-branding/
├── css/
│   └── custom-branding.css   # CSS overrides
├── logos/
│   ├── logo-light.png        # Light theme logo
│   ├── logo-dark.png         # Dark theme logo
│   └── favicon.ico           # Favicon
└── cloudpanel-branding.sh    # Script copy
```

## Related Documentation

- [DEPLOYMENT.md](DEPLOYMENT.md) - Main botginx deployment guide
- [BOTECTION-CALLBACK-API.md](BOTECTION-CALLBACK-API.md) - Callback API spec
- [CPANEL-DOMAIN-FLOW.md](CPANEL-DOMAIN-FLOW.md) - Domain setup flow
