---
title: Authentication
description: API key authentication and scopes.
---

All API requests require authentication via API key.

## Headers

Use either header format:

```http
Authorization: Bearer bg_live_xxxxxxxxxxxxxxxxxxxx
```

```http
X-API-Key: bg_live_xxxxxxxxxxxxxxxxxxxx
```

## Key Types

| Prefix | Environment |
|--------|-------------|
| `bg_live_*` | Production |
| `bg_test_*` | Test/development |

## Scopes

API keys can be restricted to specific permissions:

| Scope | Access |
|-------|--------|
| `links:read` | Read link data |
| `links:write` | Create and update links |
| `links:delete` | Delete links |
| `domains:read` | Read domain data |
| `domains:manage` | Add and verify domains |
| `analytics:read` | View analytics |
| `qrcodes:read` | Read QR codes |
| `qrcodes:write` | Create QR codes |
| `shortener:read` | Read short links |
| `shortener:write` | Create short links |
| `iplists:read` | Read IP lists |
| `iplists:write` | Modify IP lists |
| `webhooks:read` | Read webhooks |
| `webhooks:manage` | Create and delete webhooks |
| `account:read` | Read account info |

## Managing Keys

Create and revoke API keys at [guardbot.sbs/user/apikeys](https://guardbot.sbs/user/apikeys).

:::caution
If your key is compromised, revoke it immediately and create a new one.
:::
