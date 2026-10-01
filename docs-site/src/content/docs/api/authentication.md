---
title: Authentication
description: Authenticate API requests with API keys.
---

All requests require an API key. Include it in the `Authorization` header:

```bash
Authorization: Bearer bg_live_xxxxxxxxxxxxxxxxxxxx
```

You can also use the `X-API-Key` header:

```bash
X-API-Key: bg_live_xxxxxxxxxxxxxxxxxxxx
```

## Key Types

API keys come in two types:

| Prefix | Environment | Description |
|--------|-------------|-------------|
| `bg_live_*` | Production | For live environments |
| `bg_test_*` | Test | For development |

## Scopes

API keys can have restricted permissions:

| Scope | Description |
|-------|-------------|
| `links:read` | Read link data |
| `links:write` | Create/update links |
| `links:delete` | Delete links |
| `domains:read` | Read domain data |
| `domains:manage` | Add/verify domains |
| `analytics:read` | View analytics |
| `qrcodes:read` | Read QR codes |
| `qrcodes:write` | Create QR codes |
| `shortener:read` | Read short links |
| `shortener:write` | Create short links |
| `iplists:read` | Read IP lists |
| `iplists:write` | Modify IP lists |
| `webhooks:read` | Read webhooks |
| `webhooks:manage` | Create/delete webhooks |
| `account:read` | Read account info |

:::caution
If your key is compromised, revoke it immediately in the [API Keys](https://guardbot.sbs/user/apikeys) section and create a new one.
:::
