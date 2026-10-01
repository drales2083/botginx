---
title: Shortener
description: Create and manage short links.
---

## List Short Links

```http
GET /api/v1/shortener
```

**Scope:** `shortener:read`

```bash
curl "https://guardbot.sbs/api/v1/shortener" \
  -H "Authorization: Bearer YOUR_API_KEY"
```

---

## Create Short Link

```http
POST /api/v1/shortener
```

**Scope:** `shortener:write`

```bash
curl -X POST "https://guardbot.sbs/api/v1/shortener" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "domainId": "dom_abc123",
    "destinations": ["https://example.com/landing"],
    "rotationMode": "sequential",
    "qrEnabled": true
  }'
```

### Request Body

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `domainId` | string | Yes | Domain ID to host the short link |
| `destinations` | array | Yes | Destination URLs (at least one) |
| `subdomain` | string | No | Subdomain (auto-generated if empty) |
| `path` | string | No | URL path (auto-generated if empty) |
| `rotationMode` | string | No | `sequential`, `random`, or `weighted` |
| `botError` | integer | No | HTTP error code for bots (0 = pass through) |
| `qrEnabled` | boolean | No | Enable QR code generation |
| `protectionSettings` | object | No | Bot protection settings |

### Protection Settings

| Field | Type | Description |
|-------|------|-------------|
| `blockTor` | boolean | Block Tor exit nodes |
| `blockProxy` | boolean | Block proxy servers |
| `blockVPN` | boolean | Block VPN connections |
| `blockDatacenter` | boolean | Block datacenter IPs |
| `minBehaviorScore` | number | Minimum behavior score (0.0-1.0) |

---

## Get Stats

```http
GET /api/v1/shortener/{id}/stats
```

**Scope:** `shortener:read`

Get click statistics for a short link:

```bash
curl "https://guardbot.sbs/api/v1/shortener/sht_abc123/stats" \
  -H "Authorization: Bearer YOUR_API_KEY"
```
