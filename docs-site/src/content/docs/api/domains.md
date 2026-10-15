---
title: Domains
description: Manage custom domains for redirect links.
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
| `name` | string | Yes | Domain name (e.g., `example.com`) |
| `serverId` | string | No | Server ID for hosting integration |

**Response** includes the server IP to configure in DNS. SSL is provisioned automatically once DNS propagates.

---

## Verify Domain

```http
POST /api/v1/domains/{id}/verify
```

**Scope:** `domains:manage`

Triggers DNS verification for a domain:

```bash
curl -X POST "https://guardbot.sbs/api/v1/domains/dom_abc123/verify" \
  -H "Authorization: Bearer YOUR_API_KEY"
```

---

## Delete Domain

```http
DELETE /api/v1/domains/{id}
```

**Scope:** `domains:manage`

```bash
curl -X DELETE "https://guardbot.sbs/api/v1/domains/dom_abc123" \
  -H "Authorization: Bearer YOUR_API_KEY"
```
