---
title: Analytics
description: Access click statistics and visitor data.
---

## Overview

```http
GET /api/v1/analytics/overview
```

**Scope:** `analytics:read`

Get account-wide analytics summary:

```bash
curl "https://guardbot.sbs/api/v1/analytics/overview" \
  -H "Authorization: Bearer YOUR_API_KEY"
```

**Response:**

```json
{
  "success": true,
  "data": {
    "totalVisits": 15420,
    "uniqueVisits": 12350,
    "todayVisits": 230,
    "botVisits": 1580,
    "blockedVisits": 890,
    "conversionRate": 3.2
  }
}
```

---

## Link Stats

```http
GET /api/v1/analytics/links/{id}
```

**Scope:** `analytics:read`

Get analytics for a specific link:

```bash
curl "https://guardbot.sbs/api/v1/analytics/links/lnk_abc123" \
  -H "Authorization: Bearer YOUR_API_KEY"
```

---

## Timeline

```http
GET /api/v1/analytics/links/{id}/timeline
```

**Scope:** `analytics:read`

Get click timeline data:

```bash
curl "https://guardbot.sbs/api/v1/analytics/links/lnk_abc123/timeline?days=7" \
  -H "Authorization: Bearer YOUR_API_KEY"
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `days` | integer | 30 | Number of days to include |

---

## Countries

```http
GET /api/v1/analytics/links/{id}/countries
```

**Scope:** `analytics:read`

Get geographic breakdown:

```bash
curl "https://guardbot.sbs/api/v1/analytics/links/lnk_abc123/countries" \
  -H "Authorization: Bearer YOUR_API_KEY"
```

---

## Devices

```http
GET /api/v1/analytics/links/{id}/devices
```

**Scope:** `analytics:read`

Get device breakdown:

```bash
curl "https://guardbot.sbs/api/v1/analytics/links/lnk_abc123/devices" \
  -H "Authorization: Bearer YOUR_API_KEY"
```
