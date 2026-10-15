# About the CloudPanel Hosting Architecture

How the CloudPanel hosting module fits together with botginx and botection. For setup steps see [How to set up a CloudPanel hosting server](../how-to/setup-cloudpanel-hosting.md); for settings and metrics see the [CloudPanel settings reference](../reference/cloudpanel-settings.md).

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

## Settings flow

1. User configures domain settings in botginx: **Hosting → Domain → Settings**
2. When saved, botginx pushes settings via SSH to the CloudPanel server
3. Settings are written to `/etc/botection/links/{domainID}.json`
4. Botection reads these settings when making block decisions


## Analytics flow

1. User views domain settings in botginx: **Hosting → Domain → Settings**
2. Botginx fetches stats via SSH: `curl 127.0.0.1:8080/api/stats`
3. Stats are filtered by domain and displayed in the Analytics tab
