---
title: Rate Limiting
description: Request limits and rate limit headers.
---

Each API key is limited to **1000 requests per hour**.

## Response Headers

Every response includes rate limit information:

| Header | Description |
|--------|-------------|
| `X-RateLimit-Limit` | Maximum requests per hour |
| `X-RateLimit-Remaining` | Requests remaining in current window |
| `X-RateLimit-Reset` | Unix timestamp when limit resets |

Example:

```http
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 950
X-RateLimit-Reset: 1695700800
```

## Rate Limit Exceeded

When the limit is exceeded, the API returns HTTP 429:

```json
{
  "success": false,
  "error": {
    "code": "rate_limit_exceeded",
    "message": "Rate limit exceeded. Try again in 45 minutes."
  }
}
```

## Best Practices

- Cache responses to avoid redundant requests
- Use [webhooks](/api/webhooks/) for real-time updates instead of polling
- Batch operations when possible
- Implement exponential backoff for retries
