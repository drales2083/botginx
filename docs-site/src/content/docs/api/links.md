---
title: Links
description: Create and manage redirect links with bot protection.
---

## List Links

```http
GET /api/v1/links
```

**Scope:** `links:read`

```bash
curl "https://guardbot.sbs/api/v1/links" \
  -H "Authorization: Bearer YOUR_API_KEY"
```

**Response:**

```json
{
  "success": true,
  "data": [
    {
      "id": "lnk_abc123",
      "name": "Landing Page",
      "domain": "example.com",
      "path": "/promo",
      "targetUrl": "https://example.com/landing",
      "clicks": 1250,
      "createdAt": "2024-01-15T10:30:00Z"
    }
  ]
}
```

---

## Create Link

```http
POST /api/v1/links
```

**Scope:** `links:write`

```bash
curl -X POST "https://guardbot.sbs/api/v1/links" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "domainId": "dom_abc123",
    "type": "redirect",
    "destinationUrls": ["https://example.com/offer"],
    "path": "bf-offer",
    "botProtection": true
  }'
```

### Request Body

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `domainId` | string | Yes | Domain ID to host the link |
| `type` | string | No | `redirect`, `html`, or `rotation` (default: redirect) |
| `destinationUrls` | array | Yes* | Destination URLs (required for redirect/rotation) |
| `htmlContent` | string | Yes* | HTML content (required for html type) |
| `subdomain` | string | No | Subdomain for the link |
| `path` | string | No | URL path (auto-generated if empty) |
| `botProtection` | boolean | No | Enable bot protection (default: false) |
| `passParams` | boolean | No | Pass query params to destination (default: false) |

---

## Get Link

```http
GET /api/v1/links/{id}
```

**Scope:** `links:read`

```bash
curl "https://guardbot.sbs/api/v1/links/lnk_abc123" \
  -H "Authorization: Bearer YOUR_API_KEY"
```

---

## Update Link

```http
PUT /api/v1/links/{id}
```

**Scope:** `links:write`

```bash
curl -X PUT "https://guardbot.sbs/api/v1/links/lnk_abc123" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name": "Updated Name", "targetUrl": "https://new-url.com"}'
```

---

## Delete Link

```http
DELETE /api/v1/links/{id}
```

**Scope:** `links:delete`

```bash
curl -X DELETE "https://guardbot.sbs/api/v1/links/lnk_abc123" \
  -H "Authorization: Bearer YOUR_API_KEY"
```
