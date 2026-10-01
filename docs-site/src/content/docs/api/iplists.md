---
title: IP Lists
description: Manage IP whitelists for bot protection bypass.
---

## List Whitelist

```http
GET /api/v1/iplists/whitelist
```

**Scope:** `iplists:read`

Retrieve all whitelisted IPs:

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

Add an IP to the whitelist:

```bash
curl -X POST "https://guardbot.sbs/api/v1/iplists/whitelist" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"ip": "192.168.1.100"}'
```

### Request Body

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `ip` | string | Yes | IP address or CIDR range (e.g., 192.168.1.0/24) |

:::note
Whitelisted IPs bypass bot protection on all your links.
:::

---

## Remove from Whitelist

```http
DELETE /api/v1/iplists/whitelist/{id}
```

**Scope:** `iplists:write`

Remove an IP from the whitelist:

```bash
curl -X DELETE "https://guardbot.sbs/api/v1/iplists/whitelist/wl_abc123" \
  -H "Authorization: Bearer YOUR_API_KEY"
```
