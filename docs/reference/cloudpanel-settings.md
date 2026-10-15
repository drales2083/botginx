# CloudPanel Settings Reference

## Settings File Format

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

## Available Metrics

| Metric | Description |
|--------|-------------|
| Total Requests | All requests to the domain |
| Allowed Requests | Requests that passed filtering |
| Blocked Requests | Requests that were blocked |
| Block Rate | Percentage of requests blocked |
| Block Reasons | Breakdown by block reason (bot, tor, proxy, etc.) |
| Recent Blocks | Last N blocked requests with IP, path, reason |

## Stats API Response

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

## Directory structure

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

## Security Notes

- SSH access is required from botginx to CloudPanel servers
- Use strong SSH passwords or SSH keys
- Botection stats API only listens on 127.0.0.1 (not exposed externally)
- Settings files are readable by botection process only

