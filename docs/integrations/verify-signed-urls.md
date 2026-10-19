# How to Verify Signed URLs

This guide shows you how to verify that incoming traffic passed GuardBot's bot detection.

## Prerequisites

- A redirect link with "Verification Signature" enabled
- The GuardBot public key (same for all users)
- An Ed25519 signature verification library for your language

## Get the Public Key

You can find the public key in [Bot Settings → Verification tab](https://guardbot.sbs/user/antibotcontrol?tab=verification), or fetch it via API:

```bash
curl https://guardbot.sbs/api/public-key
```

```json
{
  "publicKey": "Q6ung4zA9qeaPpZUXu6RbKbdm9F03iVIvvnbYcRWwRQ=",
  "algorithm": "Ed25519",
  "encoding": "base64"
}
```

## Verification Code

Copy the verification function for your language. Each function:
1. Extracts the signature from `_bs`
2. Checks the timestamp in `_bt` hasn't expired
3. Verifies the signature using the public key

### PHP

```php
<?php
function verifyGuardBotSignature(string $url, int $maxAge = 300): bool
{
    // GuardBot public key (base64)
    $publicKeyB64 = 'Q6ung4zA9qeaPpZUXu6RbKbdm9F03iVIvvnbYcRWwRQ=';
    $publicKey = sodium_base642bin($publicKeyB64, SODIUM_BASE64_VARIANT_ORIGINAL);
    
    // Parse URL
    $parsed = parse_url($url);
    parse_str($parsed['query'] ?? '', $params);
    
    // Extract signature (base64url, no padding)
    $sigB64 = $params['_bs'] ?? '';
    if ($sigB64 === '') return false;
    
    // Convert base64url to standard base64
    $sigB64 = str_replace(['-', '_'], ['+', '/'], $sigB64);
    $signature = base64_decode($sigB64);
    if ($signature === false || strlen($signature) !== 64) return false;
    
    // Check expiry
    $timestamp = (int)($params['_bt'] ?? 0);
    if (time() - $timestamp > $maxAge) return false;
    
    // Rebuild URL without signature
    unset($params['_bs']);
    ksort($params);
    $query = http_build_query($params);
    $urlWithoutSig = $parsed['scheme'] . '://' . $parsed['host'] . $parsed['path'];
    if ($query !== '') $urlWithoutSig .= '?' . $query;
    
    // Verify
    return sodium_crypto_sign_verify_detached($signature, $urlWithoutSig, $publicKey);
}

// Usage
$fullUrl = (isset($_SERVER['HTTPS']) ? 'https' : 'http') . '://' . 
           $_SERVER['HTTP_HOST'] . $_SERVER['REQUEST_URI'];

if (verifyGuardBotSignature($fullUrl)) {
    // Visitor passed bot detection - skip your captcha
} else {
    // Invalid or expired - show captcha or block
}
```

### Python

```python
import time
import base64
from urllib.parse import urlparse, parse_qs, urlencode
from nacl.signing import VerifyKey
from nacl.exceptions import BadSignature

def verify_guardbot_signature(url: str, max_age: int = 300) -> bool:
    # GuardBot public key
    public_key_b64 = 'Q6ung4zA9qeaPpZUXu6RbKbdm9F03iVIvvnbYcRWwRQ='
    public_key = VerifyKey(base64.b64decode(public_key_b64))
    
    # Parse URL
    parsed = urlparse(url)
    params = {k: v[0] for k, v in parse_qs(parsed.query).items()}
    
    # Extract signature (base64url, no padding)
    sig_b64 = params.pop('_bs', '')
    if not sig_b64:
        return False
    
    # Convert base64url to bytes
    sig_b64 += '=' * (4 - len(sig_b64) % 4)  # Add padding
    sig_b64 = sig_b64.replace('-', '+').replace('_', '/')
    signature = base64.b64decode(sig_b64)
    
    # Check expiry
    timestamp = int(params.get('_bt', 0))
    if time.time() - timestamp > max_age:
        return False
    
    # Rebuild URL without signature
    query = urlencode(sorted(params.items()))
    url_without_sig = f"{parsed.scheme}://{parsed.netloc}{parsed.path}"
    if query:
        url_without_sig += f"?{query}"
    
    # Verify
    try:
        public_key.verify(url_without_sig.encode(), signature)
        return True
    except BadSignature:
        return False

# Usage (Flask)
from flask import request

@app.route('/landing')
def landing():
    if verify_guardbot_signature(request.url):
        # Verified by GuardBot
        pass
```

### Node.js

```javascript
const nacl = require('tweetnacl');
const { URL } = require('url');

function verifyGuardBotSignature(fullUrl, maxAge = 300) {
    // GuardBot public key
    const publicKeyB64 = 'Q6ung4zA9qeaPpZUXu6RbKbdm9F03iVIvvnbYcRWwRQ=';
    const publicKey = Buffer.from(publicKeyB64, 'base64');
    
    // Parse URL
    const parsed = new URL(fullUrl);
    const params = parsed.searchParams;
    
    // Extract signature
    const sigB64 = params.get('_bs');
    if (!sigB64) return false;
    
    // Convert base64url to standard base64
    const sigB64Std = sigB64.replace(/-/g, '+').replace(/_/g, '/');
    const signature = Buffer.from(sigB64Std, 'base64');
    
    // Check expiry
    const timestamp = parseInt(params.get('_bt') || '0', 10);
    if (Date.now() / 1000 - timestamp > maxAge) return false;
    
    // Rebuild URL without signature
    params.delete('_bs');
    params.sort();
    const urlWithoutSig = `${parsed.origin}${parsed.pathname}${params.toString() ? '?' + params.toString() : ''}`;
    
    // Verify
    return nacl.sign.detached.verify(
        Buffer.from(urlWithoutSig),
        signature,
        publicKey
    );
}

// Usage (Express)
app.get('/landing', (req, res) => {
    const fullUrl = `${req.protocol}://${req.get('host')}${req.originalUrl}`;
    
    if (verifyGuardBotSignature(fullUrl)) {
        // Verified by GuardBot
    }
});
```

### Go

```go
package main

import (
    "crypto/ed25519"
    "encoding/base64"
    "net/url"
    "sort"
    "strconv"
    "strings"
    "time"
)

// GuardBot public key
const publicKeyB64 = "Q6ung4zA9qeaPpZUXu6RbKbdm9F03iVIvvnbYcRWwRQ="

func VerifyGuardBotSignature(fullURL string, maxAge int64) bool {
    publicKey, _ := base64.StdEncoding.DecodeString(publicKeyB64)
    
    // Parse URL
    u, err := url.Parse(fullURL)
    if err != nil {
        return false
    }
    params := u.Query()
    
    // Extract signature (base64url, no padding)
    sigB64 := params.Get("_bs")
    if sigB64 == "" {
        return false
    }
    signature, err := base64.RawURLEncoding.DecodeString(sigB64)
    if err != nil {
        return false
    }
    
    // Check expiry
    timestamp, _ := strconv.ParseInt(params.Get("_bt"), 10, 64)
    if time.Now().Unix()-timestamp > maxAge {
        return false
    }
    
    // Rebuild URL without signature
    params.Del("_bs")
    keys := make([]string, 0, len(params))
    for k := range params {
        keys = append(keys, k)
    }
    sort.Strings(keys)
    
    var parts []string
    for _, k := range keys {
        for _, v := range params[k] {
            parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(v))
        }
    }
    query := strings.Join(parts, "&")
    urlWithoutSig := u.Scheme + "://" + u.Host + u.Path
    if query != "" {
        urlWithoutSig += "?" + query
    }
    
    // Verify
    return ed25519.Verify(publicKey, []byte(urlWithoutSig), signature)
}
```

### Ruby

```ruby
require 'ed25519'
require 'base64'
require 'uri'
require 'cgi'

def verify_guardbot_signature(url, max_age: 300)
  # GuardBot public key
  public_key_b64 = 'Q6ung4zA9qeaPpZUXu6RbKbdm9F03iVIvvnbYcRWwRQ='
  public_key = Ed25519::VerifyKey.new(Base64.decode64(public_key_b64))
  
  # Parse URL
  parsed = URI.parse(url)
  params = CGI.parse(parsed.query || '')
  
  # Extract signature
  sig_b64 = params.delete('_bs')&.first
  return false unless sig_b64
  
  # Convert base64url to standard base64
  sig_b64 = sig_b64.tr('-_', '+/')
  sig_b64 += '=' * (4 - sig_b64.length % 4) % 4
  signature = Base64.decode64(sig_b64)
  
  # Check expiry
  timestamp = params['_bt']&.first.to_i
  return false if Time.now.to_i - timestamp > max_age
  
  # Rebuild URL without signature
  sorted = params.sort.map { |k, v| "#{CGI.escape(k)}=#{CGI.escape(v.first)}" }
  query = sorted.join('&')
  url_without_sig = "#{parsed.scheme}://#{parsed.host}#{parsed.path}"
  url_without_sig += "?#{query}" unless query.empty?
  
  # Verify
  public_key.verify(signature, url_without_sig)
  true
rescue Ed25519::VerifyError
  false
end
```

## Troubleshooting

**Signature verification fails:**
- Ensure query parameters are sorted alphabetically before verification
- Check that you're using base64url decoding (not standard base64)
- Verify the URL scheme matches (http vs https)

**Signatures always expired:**
- Check your server's clock is synchronized (use NTP)
- Increase the expiry time in your link settings if needed

**Missing parameters:**
- Ensure "Verification Signature" is enabled in your link settings
- Check that the visitor was redirected (not a direct visit)

## Related

- [Signed URL Verification Reference](/integrations/signed-url-verification) - Technical specification
