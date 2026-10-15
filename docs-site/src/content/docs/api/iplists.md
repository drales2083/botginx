---
title: IP Lists
description: Manage IP whitelists for bot protection bypass.
---

## List Whitelist

```http
GET /api/v1/iplists/whitelist
```

**Scope:** `iplists:read`

Returns all whitelisted IPs.

```bash
curl "https://guardbot.sbs/api/v1/iplists/whitelist" \
  -H "Authorization: Bearer YOUR_API_KEY"
```

---

## Add to Whitelist

```http
POST /api/v1/iplists/whitelist
```

**Scope:** `iplists:write`

Adds an IP to the whitelist. Whitelisted IPs bypass bot protection on all links.

```bash
curl -X POST "https://guardbot.sbs/api/v1/iplists/whitelist" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"ip": "192.168.1.100"}'
```

### Request Body

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `ip` | string | Yes | IP address or CIDR range (e.g., `192.168.1.0/24`) |

---

## Remove from Whitelist

```http
DELETE /api/v1/iplists/whitelist/{id}
```

**Scope:** `iplists:write`

```bash
curl -X DELETE "https://guardbot.sbs/api/v1/iplists/whitelist/wl_abc123" \
  -H "Authorization: Bearer YOUR_API_KEY"
```
