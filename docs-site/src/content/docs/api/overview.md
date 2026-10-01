---
title: API Overview
description: GuardBot REST API for managing links, domains, and analytics.
---

The GuardBot API provides full control over your redirect links, custom domains, analytics data, QR codes, and IP whitelists. All endpoints return JSON and require authentication via API key.

## Base URL

```
https://guardbot.sbs/api/v1
```

## Quick Example

```bash
curl "https://guardbot.sbs/api/v1/links" \
  -H "Authorization: Bearer bg_live_xxxxxxxxxxxxxxxxxxxx"
```

## Response Format

All successful responses follow this structure:

```json
{
  "success": true,
  "data": { ... }
}
```

## Available Endpoints

| Resource | Description |
|----------|-------------|
| [Links](/api/links/) | Create and manage redirect links |
| [Domains](/api/domains/) | Manage custom domains |
| [Analytics](/api/analytics/) | Access click statistics |
| [QR Codes](/api/qrcodes/) | Generate QR codes |
| [Shortener](/api/shortener/) | Create short links |
| [IP Lists](/api/iplists/) | Manage IP whitelists |
| [Account](/api/account/) | Account information |
| [Webhooks](/api/webhooks/) | Real-time notifications |
