---
title: Webhooks
description: Receive real-time notifications when events occur.
---

Webhooks push event data to your server when links are visited, bots are blocked, or domains are verified. Configure webhooks in the [Webhooks](https://guardbot.sbs/user/webhooks) section.

## Event Types

| Event | Description |
|-------|-------------|
| `link.visited` | A link received a click |
| `bot.blocked` | A bot was detected and blocked |
| `domain.verified` | Domain DNS verification completed |
| `domain.ssl_ready` | SSL certificate was issued |

## Payload Format

Webhook payloads include an HMAC-SHA256 signature in the `X-Webhook-Signature` header:

```json
{
  "id": "evt_abc123",
  "type": "link.visited",
  "timestamp": "2024-01-15T10:30:00Z",
  "data": {
    "linkId": "lnk_xyz789",
    "ip": "203.0.113.42",
    "country": "US",
    "device": "mobile",
    "isBot": false
  }
}
```

## Verifying Signatures

Verify the webhook signature to ensure the request came from GuardBot:

```javascript
const crypto = require('crypto');

function verifySignature(payload, signature, secret) {
  const expected = crypto
    .createHmac('sha256', secret)
    .update(payload)
    .digest('hex');
  return crypto.timingSafeEqual(
    Buffer.from(signature),
    Buffer.from(expected)
  );
}
```

## Retry Policy

If your endpoint returns a non-2xx status code, GuardBot will retry:

- 1st retry: 1 minute
- 2nd retry: 5 minutes
- 3rd retry: 30 minutes
- Final retry: 2 hours

After 4 failed attempts, the webhook is marked as failed.
