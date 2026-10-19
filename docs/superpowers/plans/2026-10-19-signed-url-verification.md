# Signed URL Verification System

**Date:** 2026-10-19
**Status:** Implemented
**Author:** Claude + User

## Overview

Implement a cryptographic signature system that appends verification proof to destination URLs when visitors pass bot detection. Destination servers can verify the signature locally using a public key - no API calls, no secret key management, zero configuration.

## Problem

When a verified visitor is redirected to the destination URL, the destination has no way to know:
- Did this visitor actually come through Botginx?
- Did they pass bot detection?
- Is this a forged/spoofed request?

## Solution

Append Ed25519 signature to destination URLs. Destinations verify using Botginx's public key (published in docs, same for everyone).

```
Original:  https://offer.com/page?ref=abc
Signed:    https://offer.com/page?ref=abc&_bl=abc123&_bt=1729345678&_bs=BASE64SIG
                                          ↑          ↑              ↑
                                      link ID    timestamp      signature
```

## Why Ed25519 (Public Key) vs HMAC (Secret Key)

| Aspect | HMAC (Secret Key) | Ed25519 (Public Key) |
|--------|-------------------|----------------------|
| User setup | Copy secret key per link/user | Copy public key once (same for all) |
| Key management | Per-user secrets | One global public key |
| If key leaks | That user compromised | No risk (public key is public) |
| Forgery possible | Yes (if secret leaked) | No (private key never shared) |
| Config needed | Yes (secret per link) | None (key in docs) |

**Winner: Ed25519** - Zero configuration for users, more secure.

## URL Parameters

| Param | Type | Description |
|-------|------|-------------|
| `_bl` | string | Link ID (for user reference/logging, included in signature) |
| `_bt` | int | Unix timestamp when signature was created |
| `_bs` | string | Base64-encoded Ed25519 signature |

**Signature covers:** Full URL with `_bl` and `_bt`, but without `_bs` itself.

## Technical Design

### 1. Key Management

Store Ed25519 keypair at application level (not per-user):

```go
// pkg/signedurl/keys.go

// Private key: stored in environment variable or secure config
// SIGNED_URL_PRIVATE_KEY=base64_encoded_private_key

// Public key: derived from private key, published in docs
// Can also be served at GET /api/public-key for programmatic access
```

**Key generation (one-time setup):**
```go
publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
// Store privateKey securely (env var)
// Publish publicKey in documentation
```

### 2. Database Changes

```sql
-- Add to redirect_links table
ALTER TABLE redirect_links ADD COLUMN append_signature BOOLEAN DEFAULT FALSE;
ALTER TABLE redirect_links ADD COLUMN signature_expiry_seconds INTEGER DEFAULT 300;
```

### 3. Signing Flow

When visitor passes botection and is redirected:

```go
// pkg/signedurl/signer.go

func SignURL(destURL string, linkID string, privateKey ed25519.PrivateKey, expiry time.Duration) string {
    // 1. Parse destination URL
    u, _ := url.Parse(destURL)
    q := u.Query()
    
    // 2. Add link ID and timestamp
    q.Set("_bl", linkID)
    q.Set("_bt", strconv.FormatInt(time.Now().Unix(), 10))
    
    // 3. Sort params for deterministic signing
    u.RawQuery = sortedQueryString(q)
    
    // 4. Sign the URL (without _bs)
    message := []byte(u.String())
    signature := ed25519.Sign(privateKey, message)
    
    // 5. Append signature
    q.Set("_bs", base64.RawURLEncoding.EncodeToString(signature))
    u.RawQuery = sortedQueryString(q)
    
    return u.String()
}
```

### 4. Verification (User's Code)

Users copy this from docs - works for all links, zero config:

```go
func VerifyBotginxSignature(fullURL string) bool {
    // Botginx public key (from docs, same for everyone)
    publicKeyB64 := "MCowBQYDK2VwAyEA..." // 44 chars base64
    publicKey, _ := base64.StdEncoding.DecodeString(publicKeyB64)
    
    // Parse URL
    u, _ := url.Parse(fullURL)
    q := u.Query()
    
    // Extract and remove signature
    sigB64 := q.Get("_bs")
    if sigB64 == "" {
        return false
    }
    signature, _ := base64.RawURLEncoding.DecodeString(sigB64)
    
    // Check expiry (default 5 minutes)
    timestamp, _ := strconv.ParseInt(q.Get("_bt"), 10, 64)
    if time.Now().Unix() - timestamp > 300 {
        return false
    }
    
    // Rebuild URL without signature
    q.Del("_bs")
    u.RawQuery = sortedQueryString(q)
    message := []byte(u.String())
    
    // Verify
    return ed25519.Verify(publicKey, message, signature)
}
```

## Implementation Checklist

### Phase 1: Core Signing Infrastructure
- [x] Create `pkg/signedurl/` package
- [x] Implement key loading from environment
- [x] Implement `SignURL()` function
- [x] Implement `sortedQueryString()` helper (deterministic param ordering)
- [x] Add unit tests for signing

### Phase 2: Database & Link Settings
- [x] Add migration for `append_signature` and `signature_expiry_seconds` columns
- [x] Update redirect link model
- [x] Update link form UI (checkbox + expiry input)

### Phase 3: Integration with Redirect Flow
- [x] Modify redirect handler to check `append_signature` setting
- [x] Sign destination URL before redirect when enabled
- [x] Handle edge cases (destination already has query params, special chars)

### Phase 4: Public Key Endpoint
- [x] Add `GET /api/public-key` endpoint returning the public key
- [x] Return in multiple formats: base64, JSON, PEM

### Phase 5: Documentation
- [x] Create reference doc (`docs/reference/signed-url-verification.md`)
- [x] Document URL parameter format
- [x] Create how-to guide (`docs/how-to/verify-signed-urls.md`)
- [x] Provide verification code examples:
  - [x] PHP
  - [x] Python
  - [x] Node.js
  - [x] Go
  - [x] Ruby
  - [ ] Java (future)
  - [ ] C# (future)
- [x] Add troubleshooting section

### Phase 6: Admin UI
- [x] Add toggle in link edit form: "Append verification signature"
- [x] Add expiry seconds input (default 300)
- [ ] Show public key in Settings page for reference (future)

## File Structure

```
pkg/signedurl/
├── signer.go           # SignURL, key loading
├── signer_test.go      # Unit tests
└── keys.go             # Key management utilities

modules/redirectlinks/
├── handlers/handler.go # Add signing to redirect
├── models/models.go    # Add append_signature field
└── templates/
    └── form.html       # Add UI toggle

docs/reference/
└── signed-url-verification.md  # User documentation
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `SIGNED_URL_PRIVATE_KEY` | Base64-encoded Ed25519 private key (32 bytes) |

## Security Considerations

1. **Private key protection:** Store in environment, never log or expose
2. **Clock skew:** Allow small tolerance (e.g., 30 seconds) in expiry check
3. **URL encoding:** Must be consistent between signing and verification
4. **Parameter ordering:** Sort alphabetically for deterministic signatures
5. **Key rotation:** Plan for key rotation (versioned signatures with `_bv` param)

## Future Enhancements

1. **Signature versioning:** Add `_bv` param for key rotation support
2. **Additional claims:** Include visitor fingerprint hash, country code
3. **Webhook on verification:** Optional callback when destination verifies
4. **Analytics:** Track how many signed URLs are verified

## Documentation Code Examples

### PHP
```php
function verifyBotginxSignature(string $url): bool {
    // Public key from Botginx docs
    $publicKey = sodium_base642bin('MCowBQYDK2VwAyEA...', SODIUM_BASE64_VARIANT_ORIGINAL);
    
    $parsed = parse_url($url);
    parse_str($parsed['query'] ?? '', $params);
    
    $signature = sodium_base642bin($params['_bs'] ?? '', SODIUM_BASE64_VARIANT_URLSAFE_NO_PADDING);
    $timestamp = (int)($params['_bt'] ?? 0);
    
    // Check expiry
    if (time() - $timestamp > 300) return false;
    
    // Rebuild URL without signature
    unset($params['_bs']);
    ksort($params);
    $query = http_build_query($params);
    $urlWithoutSig = $parsed['scheme'] . '://' . $parsed['host'] . $parsed['path'];
    if ($query) $urlWithoutSig .= '?' . $query;
    
    return sodium_crypto_sign_verify_detached($signature, $urlWithoutSig, $publicKey);
}
```

### Python
```python
from nacl.signing import VerifyKey
from nacl.encoding import Base64Encoder, URLSafeBase64Encoder
from urllib.parse import urlparse, parse_qs, urlencode
import time

def verify_botginx_signature(url: str) -> bool:
    # Public key from Botginx docs
    public_key = VerifyKey(b"...", encoder=Base64Encoder)
    
    parsed = urlparse(url)
    params = {k: v[0] for k, v in parse_qs(parsed.query).items()}
    
    signature = URLSafeBase64Encoder.decode(params.get('_bs', ''))
    timestamp = int(params.get('_bt', 0))
    
    # Check expiry
    if time.time() - timestamp > 300:
        return False
    
    # Rebuild URL without signature
    del params['_bs']
    query = urlencode(sorted(params.items()))
    url_without_sig = f"{parsed.scheme}://{parsed.netloc}{parsed.path}"
    if query:
        url_without_sig += f"?{query}"
    
    try:
        public_key.verify(url_without_sig.encode(), signature)
        return True
    except:
        return False
```

### Node.js
```javascript
const nacl = require('tweetnacl');
const { URL } = require('url');

function verifyBotginxSignature(fullUrl) {
    // Public key from Botginx docs
    const publicKey = Buffer.from('MCowBQYDK2VwAyEA...', 'base64');
    
    const parsed = new URL(fullUrl);
    const params = parsed.searchParams;
    
    const signature = Buffer.from(params.get('_bs') || '', 'base64url');
    const timestamp = parseInt(params.get('_bt') || '0', 10);
    
    // Check expiry
    if (Date.now() / 1000 - timestamp > 300) return false;
    
    // Rebuild URL without signature
    params.delete('_bs');
    params.sort();
    const urlWithoutSig = `${parsed.origin}${parsed.pathname}${params.toString() ? '?' + params.toString() : ''}`;
    
    return nacl.sign.detached.verify(
        Buffer.from(urlWithoutSig),
        signature,
        publicKey
    );
}
```

## Testing Plan

1. **Unit tests:**
   - Sign URL, verify signature matches
   - Tampered URL fails verification
   - Expired timestamp fails
   - Missing params fail gracefully

2. **Integration tests:**
   - Create link with signing enabled
   - Simulate redirect, verify URL has params
   - Verify signature with test public key

3. **Cross-language tests:**
   - Generate signed URL in Go
   - Verify in PHP, Python, Node to ensure compatibility

## Rollout Plan

1. Deploy with feature flag disabled
2. Test internally with staging links
3. Enable for beta users
4. Document and announce
5. Enable by default for new links (optional)
