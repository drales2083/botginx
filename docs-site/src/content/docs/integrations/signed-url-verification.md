---
title: Signed URL Verification
description: Technical specification for verifying cryptographic signatures appended to destination URLs.
---

> **Get your public key:** [Bot Settings → Verification tab](https://guardbot.sbs/user/antibotcontrol?tab=verification)

## URL Parameters

When signature verification is enabled for a redirect link, verified visitors receive these parameters appended to the destination URL:

| Parameter | Type | Description |
|-----------|------|-------------|
| `_bl` | string | Link ID that generated this redirect |
| `_bt` | integer | Unix timestamp when signature was created |
| `_bs` | string | Ed25519 signature, base64url encoded (no padding) |

Example signed URL:
```
https://example.com/landing?ref=abc&_bl=a1b2c3d4e5f6&_bt=1729345678&_bs=MEUCIQDKZx...
```

## Public Key

Copy the Ed25519 public key from [Bot Settings → Verification tab](https://guardbot.sbs/user/antibotcontrol?tab=verification).

The key is the same for all users:
```
VqWUlwO+LG4LoUOGeDcmZ0lmlJdeZOmaBdcEabtohiE=
```

## Signature Algorithm

**Algorithm:** Ed25519 (RFC 8032)

**What is signed:**
1. The full destination URL including scheme, host, path, and query parameters
2. The `_bl` (link ID) parameter
3. The `_bt` (timestamp) parameter
4. Query parameters are sorted alphabetically by key before signing

**What is NOT signed:**
- The `_bs` parameter itself (it contains the signature)
- Request headers, body, or any POST data

## Verification Steps

1. Extract `_bs` from the URL
2. Remove `_bs` from the query string
3. Sort remaining query parameters alphabetically by key
4. Reconstruct the URL: `scheme://host/path?sorted_params`
5. Compute Ed25519 signature verification using the public key
6. Check that `_bt` timestamp is within the expiry window (default: 300 seconds)

## Expiry

Signatures expire after a configurable period (default: 300 seconds / 5 minutes). The expiry is checked by comparing the current server time against the `_bt` timestamp.

Configure expiry per-link in the link settings (60 to 86400 seconds).

## Error Cases

| Condition | Action |
|-----------|--------|
| `_bs` parameter missing | Reject - no signature present |
| `_bt` parameter missing | Reject - cannot verify expiry |
| Signature verification fails | Reject - tampered or forged |
| `current_time - _bt > expiry` | Reject - signature expired |

## Security Considerations

- The public key is safe to embed in your application code
- Signatures cannot be forged without the private key (held only by GuardBot)
- Use constant-time comparison when comparing signatures to prevent timing attacks
- Clock skew between servers may cause valid signatures to appear expired
- Signatures prove the visitor passed bot detection; they do not authenticate the visitor's identity

## Related

- [How to verify signed URLs](/integrations/verify-signed-urls) - Step-by-step verification guide with code examples
