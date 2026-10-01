---
title: Rate Limiting
description: API rate limits and headers.
---

Each API key has a request limit per hour. The default is **1000 requests per hour**.

## Rate Limit Headers

Every response includes rate limit information:

```http
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 950
X-RateLimit-Reset: 1695700800
```

| Header | Description |
|--------|-------------|
| `X-RateLimit-Limit` | Maximum requests per hour |
| `X-RateLimit-Remaining` | Requests remaining |
| `X-RateLimit-Reset` | Unix timestamp when limit resets |

## Exceeding the Limit

When the limit is exceeded, the API returns `429 Too Many Requests`:

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

- **Cache responses** — Don't fetch the same data repeatedly
- **Use webhooks** — For real-time updates instead of polling
- **Batch operations** — Combine multiple operations when possible
- **Implement backoff** — Wait before retrying failed requests
