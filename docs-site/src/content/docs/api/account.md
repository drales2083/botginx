---
title: Account
description: Access account information and API usage.
---

## Account Info

```http
GET /api/v1/account
```

**Scope:** `account:read`

Get account information:

```bash
curl "https://guardbot.sbs/api/v1/account" \
  -H "Authorization: Bearer YOUR_API_KEY"
```

**Response:**

```json
{
  "success": true,
  "data": {
    "id": "usr_abc123",
    "email": "user@example.com",
    "subscriptionValid": true,
    "subscriptionEnds": "2024-12-31T23:59:59Z",
    "createdAt": "2024-01-01T00:00:00Z"
  }
}
```

---

## API Usage

```http
GET /api/v1/account/usage
```

**Scope:** `account:read`

Get API usage statistics:

```bash
curl "https://guardbot.sbs/api/v1/account/usage" \
  -H "Authorization: Bearer YOUR_API_KEY"
```
