# CloudPanel Settings Reference

Technical reference for CloudPanel hosting server settings, metrics, and directory structure.

## Settings File Format

Each domain gets a settings file at `/etc/botection/links/{domainID}.json`:

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

## Settings Options

| Setting | Values | Description |
|---------|--------|-------------|
| `country_mode` | `off`, `allow`, `block` | Country filtering mode |
| `country_list` | ISO codes | Countries to allow/block |
| `device_mode` | `off`, `allow`, `block` | Device filtering mode |
| `device_list` | `desktop`, `mobile`, `tablet` | Devices to allow/block |
| `block_bots` | `true`/`false` | Block known bots |
| `block_tor` | `true`/`false` | Block Tor exit nodes |
| `block_proxy` | `true`/`false` | Block proxy/VPN IPs |
| `block_datacenter` | `true`/`false` | Block datacenter IPs |
| `block_headless` | `true`/`false` | Block headless browsers |
| `min_behavior_score` | `0`-`100` | Minimum behavior score required |
| `redirect_on_block` | URL | Redirect blocked visitors here |

## Analytics Metrics

The stats API (`127.0.0.1:8080/api/stats`) returns:

| Metric | Description |
|--------|-------------|
| `total_requests` | All requests to the domain |
| `allowed_requests` | Requests that passed filtering |
| `blocked_requests` | Requests that were blocked |
| `block_rate` | Percentage of requests blocked |
| `module_blocks` | Breakdown by block reason |
| `recent_blocks` | Last N blocked requests |

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
│   └── ...
├── config/
│   └── config.yaml
├── default-settings.json
└── cloudpanel.env

/var/www/antibot/
├── antibot                   # Binary
├── config/
│   └── config.yaml
└── logs/
```

## Branding Files

```
/opt/cloudpanel-branding/
├── css/
│   └── custom-branding.css
├── logos/
│   ├── logo-light.png
│   ├── logo-dark.png
│   └── favicon.ico
└── cloudpanel-branding.sh
```

## Branding Options

| Variable | Default | Description |
|----------|---------|-------------|
| `PANEL_NAME` | GuardHost | Custom panel name |
| `PRIMARY_COLOR` | #dc3545 | Theme color (hex) |
| `FORCE_DARK_THEME` | true | Dark mode as default |
| `LOGO_LIGHT` | - | Light theme logo path |
| `LOGO_DARK` | - | Dark theme logo path |
| `LOGO_FAVICON` | - | Favicon path |

## Port Configuration

| Port | Service | Purpose |
|------|---------|---------|
| 8443 | CloudPanel Panel | Admin interface |
| 8081 | CloudPanel nginx | Backend web server |
| 8080 | Botection | Antibot reverse proxy |
| 443 | External nginx | SSL termination |

## Related Documentation

- [CloudPanel Architecture](../explanation/cloudpanel-architecture.md)
- [How to Setup CloudPanel Server](../how-to/setup-cloudpanel-hosting.md)
