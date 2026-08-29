# Bot Protection Enforcement Plan

## Status

**Current state:** Settings persist and `ShouldBlock()` logic runs, but no serving layer acts on the decision. Visits are labeled blocked/allowed in analytics; real visitors are never actually stopped.

**Goal:** Make every toggle on `/user/analytics/link/{id}/settings` actually block matching traffic.

---

## Key Constraint: Botection Serves Multiple Deployments

Botection (the antibot reverse proxy) is used in two modes:

| Mode | Example | Settings source |
|------|---------|-----------------|
| **Standalone** | VPS running a custom app | Botection's `config.yaml` (global) |
| **With botginx** | VPS serving redirect links | Per-link settings from botginx panel |

The enforcement solution must:
1. Support per-link settings when botginx is connected
2. Fall back to global botection rules when not connected (or unreachable)
3. Add minimal latency to the request path
4. Not break existing standalone deployments

---

## Architecture: Optional Callback API

Botection gains an **optional** callback to an external panel. If configured and reachable, botection asks the panel whether to block. If not configured or the call fails, botection uses its local rules.

```
                         ┌─────────────────────────────────────────┐
                         │         Botection (VPS)                 │
  visitor ──────────────▶│                                         │
                         │  1. Extract host from request           │
                         │  2. Check: is callback configured?      │
                         │     ├─ NO  → use local rules            │
                         │     └─ YES → call panel API             │
                         │             ├─ Success → use response   │
                         │             └─ Fail/timeout → local rules│
                         │  3. Block or allow based on decision    │
                         │  4. Fire webhook event (async)          │
                         └─────────────────────────────────────────┘
                                          │
                                          │ (2) POST /api/botection/should-block
                                          ▼
                         ┌─────────────────────────────────────────┐
                         │         Botginx Panel                   │
                         │                                         │
                         │  1. Resolve host → link_id              │
                         │  2. Load link_settings for link_id      │
                         │  3. Apply rules (block_tor, country,    │
                         │     block_datacenter, min_score, etc.)  │
                         │  4. Return {block: bool, reason: str}   │
                         └─────────────────────────────────────────┘
```

### Why Callback Over Config Push

| Concern | Callback API | Config push |
|---------|--------------|-------------|
| Settings take effect | Instantly | After redeploy |
| Botection complexity | Low (HTTP call) | High (per-host rule engine) |
| Panel dependency | Soft (falls back to local) | None |
| Latency | +5-20ms (same datacenter) | None |
| Backwards compatible | Yes (callback is optional) | Yes |

The callback adds latency, but:
- Call is to localhost or same datacenter (5-20ms, not 100ms+)
- Result can be cached briefly (5-60s) by botection to reduce calls
- Fallback to local rules means panel outage doesn't break serving

---

## Botection Configuration for Callback

Add to botection's `config.yaml`:

```yaml
# External panel integration (optional)
# When configured, botection calls the panel API before making block decisions
# for requests matching host_patterns. Falls back to local rules on failure.
panel_callback:
  enabled: false                    # disabled by default — standalone VPS unaffected
  url: "http://127.0.0.1:3001/api/botection/should-block"
  timeout: "50ms"                   # keep short — this is in the request path
  cache_ttl: "30s"                  # cache decisions by (host, ip)
  
  # Only call panel for these hosts. Others use local rules only.
  # Supports wildcards: "*.example.com" matches "foo.example.com"
  host_patterns:
    - "*.redirect-domain.com"
    - "specific-link.example.com"
  
  # Fallback behavior when panel is unreachable or returns error
  fallback: "local_rules"           # "local_rules" | "allow" | "block"
```

### How Standalone VPS Are Unaffected

1. **Default off:** `enabled: false` means existing deployments keep using local rules
2. **Host filtering:** Only hosts in `host_patterns` trigger callbacks — a VPS serving non-panel apps never calls the panel
3. **Fallback:** If panel is down/slow, botection uses `fallback` behavior (default: local rules)
4. **No botginx dependency:** Standalone VPS configs never set `panel_callback`, so botection works exactly as before

### Request Flow with Callback

```
Visitor → Botection
              │
              ├─ Is panel_callback.enabled?
              │     └─ NO → use local rules, done
              │
              ├─ Is host in host_patterns?
              │     └─ NO → use local rules, done
              │
              ├─ Is (host, ip) in cache?
              │     └─ YES → use cached decision
              │
              ├─ Call panel: POST /api/botection/should-block
              │     {host, ip, country, is_tor, is_proxy, is_datacenter,
              │      is_headless, user_agent, score}
              │
              ├─ Panel response (within timeout)?
              │     ├─ YES → use panel decision, cache it
              │     └─ NO/error → use fallback behavior
              │
              └─ Block or allow
```

### Caching Strategy

To minimize latency, botection caches decisions:

```
Cache key: sha256(host + ip)[:16]
Cache TTL: 30s (configurable)
Cache size: 10,000 entries (LRU eviction)
```

A visitor hitting the same link from the same IP gets a cached decision for 30s.
Settings changes in botginx take effect within 30 seconds.

---

## Botginx Callback Endpoint

**Endpoint:** `POST /api/botection/should-block`

This is an internal API (not user-facing). It receives visitor context from botection and returns a block/allow decision.

### Request Body (from botection)

```json
{
  "host": "amber-canyon.example.com",
  "ip": "203.0.113.42",
  "country": "US",
  "is_tor": false,
  "is_proxy": false,
  "is_datacenter": true,
  "is_headless": false,
  "user_agent": "Mozilla/5.0 ...",
  "score": 72
}
```

### Response (from botginx)

```json
{
  "block": true,
  "reason": "datacenter_ip",
  "redirect": "https://google.com"
}
```

Or if allowed:

```json
{
  "block": false
}
```

### Endpoint Implementation

```go
func (h *Handler) ShouldBlock(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Host         string `json:"host"`
        IP           string `json:"ip"`
        Country      string `json:"country"`
        IsTor        bool   `json:"is_tor"`
        IsProxy      bool   `json:"is_proxy"`
        IsDatacenter bool   `json:"is_datacenter"`
        IsHeadless   bool   `json:"is_headless"`
        UserAgent    string `json:"user_agent"`
        Score        int    `json:"score"`
    }
    json.NewDecoder(r.Body).Decode(&req)
    
    // 1. Resolve host → link_id, user_id
    linkID, _, err := h.links.ResolveByHost(req.Host)
    if err != nil {
        // Unknown host — can't make a per-link decision
        // Tell botection to use its local rules
        json.NewEncoder(w).Encode(map[string]any{
            "block": false,
            "defer_to_local": true,
        })
        return
    }
    
    // 2. Load link_settings for this link
    settings, err := h.service.GetSettings(linkID)
    if err != nil {
        json.NewEncoder(w).Encode(map[string]any{"block": false, "defer_to_local": true})
        return
    }
    
    // 3. Apply rules
    block, reason := settings.ShouldBlock(req.Country, req.IsTor, req.IsProxy,
        req.IsDatacenter, req.IsHeadless, req.Score)
    
    resp := map[string]any{"block": block}
    if block {
        resp["reason"] = reason
        if settings.RedirectOnBlock != "" {
            resp["redirect"] = settings.RedirectOnBlock
        }
    }
    json.NewEncoder(w).Encode(resp)
}
```

### Response Semantics

| Field | Meaning |
|-------|---------|
| `block: true` | Botection should block this request |
| `block: false` | Botection should allow this request |
| `defer_to_local: true` | No per-link settings found — use botection's local rules |
| `reason` | Why blocked (for logging) |
| `redirect` | Where to send blocked visitors (optional) |

When `defer_to_local: true` is returned, botection proceeds with its normal rule evaluation — this handles the case where a host pattern matches but the link isn't in botginx (e.g., a wildcard *.example.com catches a non-panel hostname).

---

## Open Questions

### 1. IP Intelligence Source

`is_tor`, `is_proxy`, `is_datacenter` need data. Options:

| Source | Accuracy | Latency | Cost |
|--------|----------|---------|------|
| **MaxMind GeoIP2 + GeoLite2 ASN** | Good for datacenter/ASN | Local DB lookup, ~1ms | Free (GeoLite2) or paid |
| **IPHub** | Tor/proxy/VPN detection | HTTP call, ~100ms | Freemium |
| **ip-api.com** | Country + proxy + hosting | HTTP call, ~50ms | Free tier, rate-limited |
| **Local Tor exit list** | Tor only | Local, instant | Free (dan.me.uk list, updated hourly) |

**My recommendation:** Bundle these at deploy:

- **GeoLite2-Country.mmdb** — country lookup (free, updated weekly)
- **GeoLite2-ASN.mmdb** — ASN lookup to flag known datacenter ranges
- **Tor exit node list** — plaintext, fetched hourly by cron on VPS
- **Known datacenter ASN list** — static list (AWS, GCP, Azure, Hetzner, OVH, DigitalOcean, Vultr, Linode)

For proxy/VPN beyond datacenters, either accept the gap or integrate IPHub. I'd start without it and add later if users complain.

---

### 2. Country Detection

**Recommendation:** GeoLite2-Country bundled on VPS. The Go sidecar reads it with `oschwald/maxminddb-golang`. No external call, ~1ms lookup.

The panel deploy step:
1. Checks if `/var/lib/botginx/GeoLite2-Country.mmdb` exists on VPS
2. If missing or stale (>7 days), SCP a fresh copy from panel or download from MaxMind

Country whitelist/blacklist in settings becomes a simple `if country not in whitelist { block }` in the sidecar.

---

### 3. Headless Browser Detection

Server-side UA sniffing catches:
- `HeadlessChrome`, `PhantomJS`, `Puppeteer`, `Selenium`, `MSIE 5.0` (ancient bots)
- Missing `Accept-Language` or `Sec-Fetch-*` headers

It does **not** catch:
- Puppeteer with `puppeteer-extra-plugin-stealth`
- Real Chrome controlled by Playwright with proper fingerprints

**Options:**

| Level | What it does | Complexity |
|-------|--------------|------------|
| **Basic (server-side)** | UA + header sniff | Low — Go sidecar only |
| **Medium (JS challenge)** | Inject a script that probes `navigator.webdriver`, canvas, WebGL, runs a timing challenge, sets a signed cookie | Medium — need splash page JS |
| **Heavy (Turnstile)** | Cloudflare's managed challenge widget | Low code, but Turnstile is already a separate toggle |

**Recommendation:** Implement Basic now. The splash page already has an `animation_duration` delay; during that time we can run a lightweight JS probe and POST results back. That's the Medium path, but it's a separate phase after the sidecar is working.

---

### 4. Behavior Score

This requires client-side telemetry: mouse movement, scroll, click timing, keystroke cadence. The splash page would need to:

1. Run a JS snippet during the animation delay
2. Compute a score (0–100)
3. POST to `/webhooks/antibot/event` with `event: "behavior"` and the score
4. The sidecar waits for this before allowing the redirect (or times out and blocks)

**Problem:** This turns a simple redirect into a multi-round-trip flow and requires the sidecar to hold state (waiting for the behavior event).

**Recommendation:** Defer behavior scoring to Phase 2. For Phase 1, `min_behavior_score` is ignored (or we document it as "requires JS challenge mode"). The setting stays in the UI but doesn't block anyone yet.

---

## Implementation Phases

### Phase 1: Callback API (this plan)

**In botginx (panel):**

1. **Add callback endpoint** (`/api/botection/should-block`)
   - Accept POST with visitor context from botection
   - Resolve host → link_id via `ResolveByHost()`
   - Load `link_settings` from database
   - Apply `ShouldBlock()` logic (already exists)
   - Return `{block, reason, redirect}` or `{defer_to_local: true}`

2. **Mount without auth** (like webhooks)
   - This is machine-to-machine from botection, not user-facing
   - Authenticate via shared secret in header (optional, same-host only)

**In botection (VPS):**

3. **Add `panel_callback` config section**
   - `enabled`, `url`, `timeout`, `cache_ttl`, `host_patterns`, `fallback`
   - Default: disabled (standalone VPS unaffected)

4. **Implement callback client**
   - Before applying local rules, check if host matches `host_patterns`
   - Call panel API with visitor context
   - Cache response by (host, ip) for `cache_ttl`
   - On timeout/error, use `fallback` behavior

5. **Wire callback into decision flow**
   - Existing botection modules (rate_limiter, ip_reputation, etc.) produce signals
   - New: if callback enabled, ask panel before final decision
   - Panel decision overrides local rules (for matched hosts)

**Settings take effect instantly** — no redeploy needed. User changes a toggle, next request sees it (after cache expires).

### Phase 2: JS Challenge

- Splash page runs a fingerprint + timing probe
- Probe POSTs results to botection
- Botection includes score in callback to panel
- If score >= min_behavior_score, allow; else block

### Phase 3: Turnstile Integration

- If `turnstile_enabled`, splash page embeds the widget
- Botection verifies the Turnstile token via Cloudflare API
- Already have `turnstile_site_key` and `turnstile_secret_key` fields in link_settings

---

## Data Files on VPS (botection)

| File | Source | Update frequency |
|------|--------|------------------|
| `/var/lib/botection/GeoLite2-Country.mmdb` | MaxMind | Weekly |
| `/var/lib/botection/GeoLite2-ASN.mmdb` | MaxMind | Weekly |
| `/var/lib/botection/tor-exit-nodes.txt` | dan.me.uk/torlist | Hourly (cron) |
| `/var/lib/botection/datacenter-asns.txt` | Static | On deploy |

These are used by botection for the **input signals** it sends to the callback (is_tor, is_datacenter, country, etc.). The panel just receives these values and applies per-link rules — no IP intelligence needed on the panel side.

---

## Authentication

**Decision:** Same-host only (localhost). Botection and botginx always run on the same VPS, so:
- Callback URL is `http://127.0.0.1:3001/...`
- No auth needed — localhost isn't routable from outside
- ~5ms latency per call (before caching)

## Webhooks Still Fire

The callback is for **decision-making** (before action). The existing webhook is for **logging** (after action). Both run:

```
Request → Botection
            │
            ├─ Callback: POST /api/botection/should-block
            │            → get block/allow decision
            │
            ├─ Apply decision (block or allow)
            │
            └─ Webhook: POST /webhooks/antibot/webhook (async)
                        → log the visit with action taken
```

This means analytics continues to work exactly as before — webhooks fire for every visit, recording the outcome.

---

## Next Steps

**In botginx (this repo):**

1. Add `POST /api/botection/should-block` endpoint
2. Wire it into analytics module routes (unauthenticated, like webhooks)
3. Test with curl / mock requests

**In botection (separate repo):** See detailed spec below.

---

## Botection Changes Spec

> **For the agent working on `/home/this/dev/code/msf-tokens/antibot/`**
> 
> This section specifies exactly what to add to botection to support the panel callback.

### 1. Config Structure

Add to `internal/config/config.go`:

```go
type PanelCallback struct {
    Enabled      bool     `yaml:"enabled"`
    URL          string   `yaml:"url"`
    Timeout      string   `yaml:"timeout"`       // e.g. "50ms"
    CacheTTL     string   `yaml:"cache_ttl"`     // e.g. "30s"
    HostPatterns []string `yaml:"host_patterns"` // e.g. ["*.example.com"]
    Fallback     string   `yaml:"fallback"`      // "local_rules" | "allow" | "block"
}
```

Add field to main Config struct:
```go
PanelCallback PanelCallback `yaml:"panel_callback"`
```

### 2. Panel Client

Create `internal/panel/callback.go`:

```go
package panel

import (
    "bytes"
    "context"
    "encoding/json"
    "net/http"
    "sync"
    "time"
)

type CallbackRequest struct {
    Host         string `json:"host"`
    IP           string `json:"ip"`
    Country      string `json:"country"`
    IsTor        bool   `json:"is_tor"`
    IsProxy      bool   `json:"is_proxy"`
    IsDatacenter bool   `json:"is_datacenter"`
    IsHeadless   bool   `json:"is_headless"`
    UserAgent    string `json:"user_agent"`
    Score        int    `json:"score"` // 0-100, from engine's totalScore * 100
}

type CallbackResponse struct {
    Block        bool   `json:"block"`
    DeferToLocal bool   `json:"defer_to_local"`
    Reason       string `json:"reason"`
    Redirect     string `json:"redirect"`
}

type CallbackClient struct {
    url      string
    timeout  time.Duration
    cacheTTL time.Duration
    patterns []string // compiled glob patterns
    fallback string
    client   *http.Client
    
    cache   map[string]cacheEntry
    cacheMu sync.RWMutex
}

type cacheEntry struct {
    response  CallbackResponse
    expiresAt time.Time
}

func NewCallbackClient(cfg PanelCallbackConfig) *CallbackClient {
    // Parse timeout/cacheTTL durations
    // Compile host patterns (support * wildcard)
    // Return client with http.Client{Timeout: timeout}
}

func (c *CallbackClient) ShouldBlock(ctx context.Context, req CallbackRequest) (CallbackResponse, error) {
    // 1. Check if host matches any pattern
    //    if !c.matchesPattern(req.Host) { return CallbackResponse{DeferToLocal: true}, nil }
    
    // 2. Check cache
    //    cacheKey := sha256(req.Host + req.IP)[:16]
    //    if cached, ok := c.getCache(cacheKey); ok { return cached, nil }
    
    // 3. Call panel
    //    POST to c.url with JSON body
    //    On timeout/error: return fallback response
    
    // 4. Cache response
    //    c.setCache(cacheKey, response)
    
    // 5. Return response
}

func (c *CallbackClient) matchesPattern(host string) bool {
    // Match against c.patterns
    // Support "*.example.com" matching "foo.example.com"
}

func (c *CallbackClient) fallbackResponse() CallbackResponse {
    switch c.fallback {
    case "allow":
        return CallbackResponse{Block: false}
    case "block":
        return CallbackResponse{Block: true, Reason: "panel_unreachable"}
    default: // "local_rules"
        return CallbackResponse{DeferToLocal: true}
    }
}
```

### 3. Wire into Engine

Modify `internal/core/engine.go`:

The callback should run AFTER modules compute signals but BEFORE the final action. Two options:

**Option A: Add to Engine struct**
```go
type Engine struct {
    registry       *Registry
    panelCallback  *panel.CallbackClient // nil if disabled
    // ...
}

func (e *Engine) Process(ctx *RequestContext) Decision {
    // ... existing module processing ...
    
    // After modules run, before final decision:
    if e.panelCallback != nil {
        resp, err := e.panelCallback.ShouldBlock(ctx.Context(), panel.CallbackRequest{
            Host:         ctx.Host,
            IP:           ctx.IP,
            Country:      ctx.Country,
            IsTor:        ctx.HasTag("tor"),
            IsProxy:      ctx.HasTag("proxy"),
            IsDatacenter: ctx.HasTag("datacenter"),
            IsHeadless:   ctx.HasTag("headless"),
            UserAgent:    ctx.UserAgent,
            Score:        int(totalScore * 100),
        })
        if err == nil && !resp.DeferToLocal {
            if resp.Block {
                return Decision{
                    Action:   Block,
                    Score:    totalScore,
                    Reason:   resp.Reason,
                    Module:   "panel_callback",
                    Redirect: resp.Redirect,
                    Results:  results,
                }
            }
            return Decision{
                Action:  Continue,
                Score:   totalScore,
                Reason:  "panel_allowed",
                Module:  "panel_callback",
                Results: results,
            }
        }
        // On error or DeferToLocal, continue to local rules
    }
    
    // ... existing threshold logic ...
}
```

**Option B: Separate middleware** (if cleaner for existing architecture)

### 4. Add Redirect field to Decision

The panel can return a custom redirect URL for blocked visitors:

```go
type Decision struct {
    Action   Action
    Score    float64
    Reason   string
    Module   string
    Redirect string // NEW: optional redirect URL for blocked requests
    Results  map[string]ModuleResult
    Duration int64
}
```

The proxy handler should check `decision.Redirect` and use it instead of default block behavior.

### 5. RequestContext helpers

Ensure `RequestContext` has these fields/methods available:
- `Host` - the hostname from the request
- `IP` - visitor IP
- `Country` - from GeoIP lookup
- `UserAgent` - raw UA string
- `HasTag(tag string) bool` - check if a module added a tag
- `Context()` - for timeout propagation

### 6. Config Example

```yaml
# In config.yaml
panel_callback:
  enabled: true
  url: "http://127.0.0.1:3001/api/botection/should-block"
  timeout: "50ms"
  cache_ttl: "30s"
  host_patterns:
    - "*.redirect-domain.com"
  fallback: "local_rules"
```

### 7. Testing

Test cases:
1. `enabled: false` → callback never called, local rules apply
2. Host not in patterns → callback not called, local rules apply  
3. Host in patterns, panel returns `{block: true}` → request blocked
4. Host in patterns, panel returns `{block: false}` → request allowed
5. Host in patterns, panel returns `{defer_to_local: true}` → local rules apply
6. Host in patterns, panel timeout → fallback behavior applies
7. Cache hit → no HTTP call made

---

## Questions Summary

| # | Question | Recommendation |
|---|----------|----------------|
| 1 | IP intelligence source? | Botection already has this — GeoLite2 + Tor list + datacenter ASNs |
| 2 | Country detection? | Botection already does this — sends country in callback |
| 3 | Headless detection level? | Botection does UA sniff; sends `is_headless` in callback |
| 4 | Behavior score? | Defer to Phase 2; ignore `min_behavior_score` for now |

The callback approach means **botginx doesn't need any IP intelligence** — botection does all the detection and sends the results. Botginx just applies per-link rules to those results.
