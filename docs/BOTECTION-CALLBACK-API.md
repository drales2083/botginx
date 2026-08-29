# Botection Callback API

## Overview

The botection callback API allows botection (the antibot reverse proxy) to query botginx for per-link blocking decisions **before** taking action on a request. This enables users to configure bot protection settings per redirect link in the botginx panel, and have those settings enforced at the edge.

## Architecture

```
                         ┌─────────────────────────────────────────┐
                         │         Botection (VPS)                 │
  visitor ──────────────▶│                                         │
                         │  1. Extract host from request           │
                         │  2. Collect signals (IP, country, etc.) │
                         │  3. Call botginx callback API           │
                         │  4. Apply returned decision              │
                         │  5. Fire webhook for logging (async)    │
                         └─────────────────────────────────────────┘
                                          │
                                          │ POST /api/botection/should-block
                                          ▼
                         ┌─────────────────────────────────────────┐
                         │         Botginx Panel                   │
                         │                                         │
                         │  1. Resolve host → link_id              │
                         │  2. Load link_settings from database    │
                         │  3. Apply blocking rules                │
                         │  4. Return decision                     │
                         └─────────────────────────────────────────┘
```

## Endpoint

```
POST /api/botection/should-block
```

**Authentication:** None (localhost only). This endpoint is designed for machine-to-machine communication between botection and botginx running on the same VPS.

**Content-Type:** `application/json`

## Request

Botection sends visitor context collected from the incoming request:

```json
{
  "host": "amber-canyon.example.com",
  "ip": "203.0.113.42",
  "country": "US",
  "is_tor": false,
  "is_proxy": false,
  "is_datacenter": true,
  "is_headless": false,
  "user_agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
  "score": 72
}
```

### Request Fields

| Field | Type | Description |
|-------|------|-------------|
| `host` | string | Hostname from the request (e.g., `link.example.com`) |
| `ip` | string | Visitor's IP address |
| `country` | string | Two-letter country code from GeoIP lookup |
| `is_tor` | bool | Whether IP is a Tor exit node |
| `is_proxy` | bool | Whether IP is a known proxy/VPN |
| `is_datacenter` | bool | Whether IP belongs to a datacenter ASN |
| `is_headless` | bool | Whether request appears to be from a headless browser |
| `user_agent` | string | Raw User-Agent header |
| `score` | int | Bot score from botection (0-100, higher = more bot-like) |

## Response

### Allow

```json
{
  "block": false
}
```

### Block

```json
{
  "block": true,
  "reason": "tor_blocked",
  "redirect": "https://google.com"
}
```

### Defer to Local Rules

When botginx doesn't recognize the host or has no settings for it:

```json
{
  "block": false,
  "defer_to_local": true
}
```

This tells botection to fall back to its local rules (from `config.yaml`).

### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `block` | bool | Whether to block the request |
| `defer_to_local` | bool | If true, botection should use its local rules instead |
| `reason` | string | Why the request was blocked (for logging) |
| `redirect` | string | Optional URL to redirect blocked visitors to |

### Block Reasons

| Reason | Triggered By |
|--------|--------------|
| `bot_blocked` | `block_bots` setting + score >= 70 |
| `tor_blocked` | `block_tor` setting + `is_tor` flag |
| `proxy_blocked` | `block_proxy` setting + `is_proxy` flag |
| `datacenter_blocked` | `block_datacenter` setting + `is_datacenter` flag |
| `headless_blocked` | `block_headless` setting + `is_headless` flag |
| `low_behavior_score` | `min_behavior_score` setting > visitor's score |
| `country_not_whitelisted` | Country whitelist mode, visitor not in list |
| `country_blacklisted` | Country blacklist mode, visitor in list |
| `device_not_whitelisted` | Device whitelist mode, visitor not in list |
| `device_blacklisted` | Device blacklist mode, visitor in list |

## Botection Configuration

Add to botection's `config.yaml`:

```yaml
panel_callback:
  enabled: true
  url: "http://127.0.0.1:3001/api/botection/should-block"
  timeout: "50ms"
  cache_ttl: "30s"
  host_patterns:
    - "*.redirect-domain.com"
    - "specific-link.example.com"
  fallback: "local_rules"
```

### Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable/disable callback. Disabled = standalone mode. |
| `url` | string | - | Botginx callback endpoint URL |
| `timeout` | duration | `50ms` | HTTP request timeout |
| `cache_ttl` | duration | `30s` | How long to cache decisions |
| `host_patterns` | []string | `[]` | Hosts to query panel for. Supports `*` wildcards. |
| `fallback` | string | `local_rules` | Behavior on timeout/error: `local_rules`, `allow`, or `block` |

### Fallback Behavior

| Value | On Timeout/Error |
|-------|------------------|
| `local_rules` | Apply botection's local rules from config |
| `allow` | Allow the request through |
| `block` | Block the request |

## Caching

Botection caches callback responses to minimize latency:

```
Cache key: sha256(host + ip)[:16]
Cache TTL: 30s (configurable)
Cache size: 10,000 entries (LRU eviction)
```

- Same visitor hitting same host: cached decision for 30s
- Settings changes take effect within cache TTL
- Cache miss: ~5ms latency (localhost call)

## Standalone VPS Compatibility

VPS deployments without botginx panel integration are unaffected:

1. **Default off:** `enabled: false` means callback is never called
2. **Host filtering:** Only hosts matching `host_patterns` trigger callbacks
3. **Fallback:** If panel is unreachable, `fallback` behavior applies
4. **No dependency:** Standalone VPS configs don't set `panel_callback`

## Relationship to Webhooks

| Callback | Webhook |
|----------|---------|
| **Before** action | **After** action |
| Synchronous (blocking) | Asynchronous |
| Returns block decision | Logs the outcome |
| `/api/botection/should-block` | `/webhooks/antibot/webhook` |

Both run for every request:
1. Callback asks "should I block this?"
2. Botection applies the decision
3. Webhook logs the visit with the action taken

## Example Usage

### curl Test

```bash
curl -X POST http://localhost:3001/api/botection/should-block \
  -H "Content-Type: application/json" \
  -d '{
    "host": "my-link.example.com",
    "ip": "203.0.113.42",
    "country": "US",
    "is_tor": false,
    "is_proxy": false,
    "is_datacenter": true,
    "is_headless": false,
    "user_agent": "Mozilla/5.0...",
    "score": 45
  }'
```

### Response (blocked due to datacenter IP)

```json
{
  "block": true,
  "reason": "datacenter_blocked",
  "redirect": "https://google.com"
}
```

### Response (unknown host)

```json
{
  "block": false,
  "defer_to_local": true
}
```

## Link Settings

Users configure per-link settings at `/user/analytics/link/{id}/settings`:

| Setting | Effect |
|---------|--------|
| Block known bots | Block if `score >= 70` |
| Block Tor exit nodes | Block if `is_tor` |
| Block proxy/VPN | Block if `is_proxy` |
| Block datacenter IPs | Block if `is_datacenter` |
| Block headless browsers | Block if `is_headless` |
| Min behavior score | Block if `score < threshold` |
| Country whitelist/blacklist | Block by country |
| Device whitelist/blacklist | Block by device type |
| Redirect on block | URL to send blocked visitors to |

## Files

| File | Description |
|------|-------------|
| `modules/analytics/handlers/handler.go` | `ShouldBlockCallback` handler |
| `modules/analytics/module.go` | `BotectionRoutes()` router |
| `cmd/server/main.go` | Mounts at `/api/botection` |
| `modules/analytics/services/analytics_service.go` | `ShouldBlock()` logic |
| `modules/analytics/models/link_settings.go` | Settings model |

## Related Documentation

- [ENFORCEMENT-PLAN.md](../ENFORCEMENT-PLAN.md) - Full enforcement architecture
- Botection config: `/home/this/dev/code/msf-tokens/antibot/config.yaml`
