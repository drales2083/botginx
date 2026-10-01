---
title: Domains
description: Manage custom domains for your redirect links.
---

## List Domains

```http
GET /api/v1/domains
```

**Scope:** `domains:read`

```bash
curl "https://guardbot.sbs/api/v1/domains" \
  -H "Authorization: Bearer YOUR_API_KEY"
```

---

## Add Domain

```http
POST /api/v1/domains
```

**Scope:** `domains:manage`

```bash
curl -X POST "https://guardbot.sbs/api/v1/domains" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name": "links.example.com"}'
```

### Request Body

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Domain name (e.g., example.com) |
| `serverId` | string | No | Server ID for hosting integration |

:::note
After adding a domain, point its DNS to the server IP shown in the response. SSL is provisioned automatically once DNS propagates.
:::

---

## Verify Domain

```http
POST /api/v1/domains/{id}/verify
```

**Scope:** `domains:manage`

Trigger DNS verification for a domain:

```bash
curl -X POST "https://guardbot.sbs/api/v1/domains/dom_abc123/verify" \
  -H "Authorization: Bearer YOUR_API_KEY"
```
