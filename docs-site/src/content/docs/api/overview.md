---
title: API Overview
description: REST API for managing links, domains, analytics, and more.
---

The GuardBot API provides programmatic access to redirect links, custom domains, analytics data, QR codes, and IP lists. All endpoints return JSON and require authentication.

## Base URL

```
https://guardbot.sbs/api/v1
```

## Authentication

Include your API key in the `Authorization` header:

```bash
curl "https://guardbot.sbs/api/v1/links" \
  -H "Authorization: Bearer bg_live_xxxxxxxxxxxxxxxxxxxx"
```

See [Authentication](/api/authentication/) for key types and scopes.

## Response Format

Successful responses:

```json
{
  "success": true,
  "data": { ... }
}
```

Error responses:

```json
{
  "success": false,
  "error": {
    "code": "error_code",
    "message": "Description"
  }
}
```

See [Errors](/api/errors/) for error codes.

## Endpoints

| Resource | Description |
|----------|-------------|
| [Links](/api/links/) | Create and manage redirect links |
| [Domains](/api/domains/) | Manage custom domains |
| [Analytics](/api/analytics/) | Access click statistics |
| [QR Codes](/api/qrcodes/) | Generate QR codes |
| [Shortener](/api/shortener/) | Create short links |
| [IP Lists](/api/iplists/) | Manage IP whitelists |
| [Account](/api/account/) | Account information |
| [Webhooks](/api/webhooks/) | Real-time event notifications |

## Rate Limits

Default limit: 1000 requests per hour per API key.

See [Rate Limiting](/api/rate-limiting/) for headers and best practices.
