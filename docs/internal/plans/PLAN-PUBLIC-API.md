# Public API Plan

## Current State

### What Exists
- **API Keys module** (`/user/apikeys`) - Creates keys with scopes, rate limits, expiration
- **Key format**: `bg_live_` or `bg_test_` + 48 hex chars
- **Storage**: SHA256 hash in database (never plaintext)
- **Scopes defined**:
  ```
  links:read, links:write, links:delete
  domains:read, domains:manage
  analytics:read
  qrcodes:read, qrcodes:write
  webhooks:manage
  account:read
  ```

### What's Missing
- **No public API router** - Keys exist but can't be used
- **No API auth middleware** - Nothing validates `Authorization: Bearer bg_live_xxx`
- **No rate limiting** - Rate limit stored but not enforced
- **No API documentation** - Users don't know endpoints exist
- **No webhooks** - No way to push events to external systems

---

## Architecture

### Route Structure
```
/api/v1/                        # Public API (API key auth)
├── /links                      # Redirect links CRUD
├── /domains                    # Domain management
├── /analytics                  # Stats and metrics
├── /qrcodes                    # QR code generation
├── /shortener                  # URL shortener
├── /iplists                    # Whitelist/blocklist
├── /webhooks                   # Webhook subscriptions
└── /account                    # Account info, usage

/api/botection/                 # Internal (localhost only, existing)
/api/track/                     # Public tracking pixel (existing)
/api/cron/                      # Internal cron (existing)
```

### Authentication Flow
```
Request:
  Authorization: Bearer bg_live_abc123...

Middleware:
  1. Extract token from header
  2. Hash token with SHA256
  3. Lookup hash in api_keys table
  4. Check: is_active, expires_at, rate_limit
  5. Check: required scope for endpoint
  6. Attach user context to request
  7. Increment requests_count, update last_used_at
```

---

## Phase 1: Core Infrastructure

### 1.1 API Auth Middleware
**File**: `pkg/apiauth/middleware.go`

```go
type APIAuthMiddleware struct {
    db        *sqlx.DB
    rateStore *RateLimitStore
}

func (m *APIAuthMiddleware) Authenticate(next http.Handler) http.Handler
func (m *APIAuthMiddleware) RequireScope(scope string) func(http.Handler) http.Handler
func (m *APIAuthMiddleware) RateLimit(next http.Handler) http.Handler
```

**Features**:
- Extract `Authorization: Bearer <token>` or `X-API-Key: <token>`
- SHA256 hash lookup
- Scope validation per endpoint
- Rate limiting (requests per hour from api_keys.rate_limit)
- Request counting and last_used tracking

### 1.2 Rate Limiting
**File**: `pkg/apiauth/ratelimit.go`

```go
type RateLimitStore struct {
    // In-memory with sliding window
    // Key: api_key_id, Value: request timestamps
}

// Returns remaining requests, reset time
func (s *RateLimitStore) Check(keyID string, limit int) (remaining int, resetAt time.Time, allowed bool)
```

**Response headers**:
```
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 847
X-RateLimit-Reset: 1695686400
```

### 1.3 API Response Format
**File**: `pkg/apiauth/response.go`

Consistent JSON responses:
```json
// Success
{
  "success": true,
  "data": { ... }
}

// Error
{
  "success": false,
  "error": {
    "code": "invalid_scope",
    "message": "This API key does not have the 'links:write' scope"
  }
}

// Paginated
{
  "success": true,
  "data": [...],
  "pagination": {
    "page": 1,
    "perPage": 25,
    "total": 142,
    "totalPages": 6
  }
}
```

---

## Phase 2: API Endpoints

### 2.1 Links API
**Scope**: `links:read`, `links:write`, `links:delete`

| Method | Endpoint | Scope | Description |
|--------|----------|-------|-------------|
| GET | `/api/v1/links` | links:read | List all links (paginated) |
| POST | `/api/v1/links` | links:write | Create new link |
| GET | `/api/v1/links/{id}` | links:read | Get link details |
| PUT | `/api/v1/links/{id}` | links:write | Update link |
| DELETE | `/api/v1/links/{id}` | links:delete | Delete link |
| POST | `/api/v1/links/bulk` | links:write | Create multiple links |
| DELETE | `/api/v1/links/bulk` | links:delete | Delete multiple links |

**Create request**:
```json
{
  "name": "Black Friday Campaign",
  "domainId": "dom_abc123",
  "targetUrl": "https://example.com/offer",
  "path": "bf-offer",              // optional, auto-generated if empty
  "protection": {
    "templateStyle": "shield",
    "countryMode": "allow",
    "countryList": ["US", "CA"],
    "blockBots": true,
    "blockProxy": true
  }
}
```

**Response**:
```json
{
  "success": true,
  "data": {
    "id": "lnk_xyz789",
    "url": "https://yourdomain.com/bf-offer",
    "targetUrl": "https://example.com/offer",
    "clicks": 0,
    "createdAt": "2026-09-25T12:00:00Z"
  }
}
```

### 2.2 Domains API
**Scope**: `domains:read`, `domains:manage`

| Method | Endpoint | Scope | Description |
|--------|----------|-------|-------------|
| GET | `/api/v1/domains` | domains:read | List domains |
| POST | `/api/v1/domains` | domains:manage | Add domain |
| GET | `/api/v1/domains/{id}` | domains:read | Get domain details |
| PUT | `/api/v1/domains/{id}` | domains:manage | Update domain settings |
| DELETE | `/api/v1/domains/{id}` | domains:manage | Remove domain |
| POST | `/api/v1/domains/{id}/verify` | domains:manage | Trigger DNS verification |
| GET | `/api/v1/domains/{id}/ssl` | domains:read | Check SSL status |

### 2.3 Analytics API
**Scope**: `analytics:read`

| Method | Endpoint | Scope | Description |
|--------|----------|-------|-------------|
| GET | `/api/v1/analytics/overview` | analytics:read | Account-wide stats |
| GET | `/api/v1/analytics/links/{id}` | analytics:read | Per-link analytics |
| GET | `/api/v1/analytics/links/{id}/timeline` | analytics:read | Clicks over time |
| GET | `/api/v1/analytics/links/{id}/countries` | analytics:read | Geographic breakdown |
| GET | `/api/v1/analytics/links/{id}/devices` | analytics:read | Device breakdown |
| GET | `/api/v1/analytics/links/{id}/referrers` | analytics:read | Traffic sources |
| GET | `/api/v1/analytics/realtime` | analytics:read | Live visitor stream |

**Query params**:
```
?from=2026-09-01&to=2026-09-25    # Date range
?interval=day|hour                 # Granularity
?limit=100                         # Max results
```

### 2.4 QR Codes API
**Scope**: `qrcodes:read`, `qrcodes:write`

| Method | Endpoint | Scope | Description |
|--------|----------|-------|-------------|
| GET | `/api/v1/qrcodes` | qrcodes:read | List QR codes |
| POST | `/api/v1/qrcodes` | qrcodes:write | Create QR code |
| GET | `/api/v1/qrcodes/{id}` | qrcodes:read | Get QR code details |
| GET | `/api/v1/qrcodes/{id}/download` | qrcodes:read | Download QR image |
| DELETE | `/api/v1/qrcodes/{id}` | qrcodes:write | Delete QR code |

**Create request**:
```json
{
  "linkId": "lnk_xyz789",
  "format": "png",           // png, svg, pdf
  "size": 512,               // pixels
  "color": "#000000",
  "backgroundColor": "#ffffff",
  "logo": "https://..."      // optional center logo
}
```

### 2.5 IP Lists API
**Scope**: `iplists:read`, `iplists:write`

| Method | Endpoint | Scope | Description |
|--------|----------|-------|-------------|
| GET | `/api/v1/iplists/whitelist` | iplists:read | List whitelisted IPs |
| POST | `/api/v1/iplists/whitelist` | iplists:write | Add to whitelist |
| DELETE | `/api/v1/iplists/whitelist/{id}` | iplists:write | Remove from whitelist |
| GET | `/api/v1/iplists/blocklist` | iplists:read | List blocked IPs |
| POST | `/api/v1/iplists/blocklist` | iplists:write | Add to blocklist |
| DELETE | `/api/v1/iplists/blocklist/{id}` | iplists:write | Remove from blocklist |
| GET | `/api/v1/iplists/check/{ip}` | iplists:read | Check IP status |

### 2.6 Shortener API
**Scope**: `shortener:read`, `shortener:write`

| Method | Endpoint | Scope | Description |
|--------|----------|-------|-------------|
| GET | `/api/v1/shortener` | shortener:read | List short links |
| POST | `/api/v1/shortener` | shortener:write | Create short link |
| GET | `/api/v1/shortener/{id}` | shortener:read | Get short link |
| GET | `/api/v1/shortener/{id}/stats` | shortener:read | Get click stats |
| DELETE | `/api/v1/shortener/{id}` | shortener:write | Delete short link |

### 2.7 Account API
**Scope**: `account:read`

| Method | Endpoint | Scope | Description |
|--------|----------|-------|-------------|
| GET | `/api/v1/account` | account:read | Account info |
| GET | `/api/v1/account/usage` | account:read | API usage stats |
| GET | `/api/v1/account/subscription` | account:read | Subscription status |

---

## Phase 3: Webhooks

### 3.1 Webhook Subscriptions
**Scope**: `webhooks:manage`

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/webhooks` | List webhook subscriptions |
| POST | `/api/v1/webhooks` | Create webhook |
| GET | `/api/v1/webhooks/{id}` | Get webhook details |
| PUT | `/api/v1/webhooks/{id}` | Update webhook |
| DELETE | `/api/v1/webhooks/{id}` | Delete webhook |
| POST | `/api/v1/webhooks/{id}/test` | Send test event |

**Create request**:
```json
{
  "url": "https://your-server.com/webhook",
  "events": ["link.visited", "bot.blocked", "link.created"],
  "secret": "whsec_..."    // optional, for signature verification
}
```

### 3.2 Webhook Events

| Event | Trigger | Payload |
|-------|---------|---------|
| `link.created` | New link created | link object |
| `link.updated` | Link modified | link object, changes |
| `link.deleted` | Link removed | link id |
| `link.visited` | Click recorded | visit details |
| `bot.blocked` | Bot detected and blocked | threat details |
| `domain.verified` | DNS verified | domain object |
| `domain.ssl_ready` | SSL certificate issued | domain object |

**Webhook payload**:
```json
{
  "id": "evt_abc123",
  "type": "link.visited",
  "createdAt": "2026-09-25T12:00:00Z",
  "data": {
    "linkId": "lnk_xyz789",
    "ip": "203.0.113.42",
    "country": "US",
    "device": "mobile",
    "isBot": false
  }
}
```

**Signature header**:
```
X-Webhook-Signature: sha256=abc123...
```

### 3.3 Webhook Database Table

```sql
CREATE TABLE webhooks (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    events TEXT[] NOT NULL,
    secret_hash TEXT,
    is_active BOOLEAN DEFAULT true,
    last_triggered_at TIMESTAMP,
    failure_count INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE webhook_deliveries (
    id TEXT PRIMARY KEY,
    webhook_id TEXT NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    response_status INT,
    response_body TEXT,
    delivered_at TIMESTAMP DEFAULT NOW(),
    success BOOLEAN
);
```

---

## Phase 4: Documentation

### 4.1 API Documentation Page
**Route**: `/user/apikeys/docs` or `/docs/api`

Interactive documentation showing:
- All endpoints with request/response examples
- Authentication guide
- Rate limiting explanation
- Webhook setup guide
- Code examples (curl, Python, Node.js, PHP)

### 4.2 OpenAPI/Swagger Spec
**File**: `docs/openapi.yaml`

Auto-generated spec for:
- API client generation
- Postman import
- Integration testing

---

## Phase 5: Implementation Order

### Week 1: Core
1. `pkg/apiauth/middleware.go` - Auth middleware
2. `pkg/apiauth/ratelimit.go` - Rate limiting
3. `pkg/apiauth/response.go` - Response helpers
4. Mount `/api/v1` router in main.go
5. Add missing scopes: `iplists:read`, `iplists:write`, `shortener:read`, `shortener:write`

### Week 2: Endpoints
1. Links API (reuse existing handlers, wrap with API auth)
2. Domains API
3. Analytics API
4. QR Codes API
5. IP Lists API
6. Shortener API
7. Account API

### Week 3: Webhooks
1. Webhooks database migration
2. Webhook CRUD handlers
3. Event dispatcher service
4. Delivery queue and retry logic
5. Webhook management UI

### Week 4: Documentation
1. API docs page with examples
2. OpenAPI spec generation
3. Postman collection
4. Usage dashboard in API Keys page

---

## File Structure

```
pkg/
└── apiauth/
    ├── middleware.go      # Auth, scope checking
    ├── ratelimit.go       # Rate limiting
    ├── response.go        # JSON response helpers
    └── context.go         # Request context helpers

modules/
├── apikeys/
│   └── services/
│       └── service.go     # Add ValidateKey(), RecordUsage()
└── webhooks/              # New module
    ├── module.go
    ├── handlers/
    │   └── handler.go
    ├── services/
    │   ├── service.go     # CRUD
    │   └── dispatcher.go  # Event dispatch
    └── templates/
        └── index.html

cmd/server/
└── main.go                # Mount /api/v1 router
```

---

## Security Considerations

1. **Rate limiting** - Per-key limits enforced in middleware
2. **Scope enforcement** - Every endpoint checks required scope
3. **Key rotation** - Existing rotate endpoint creates new key, revokes old
4. **Test mode** - `bg_test_` keys work only in sandbox (no real writes)
5. **IP restrictions** - Optional: limit key usage to specific IPs
6. **Audit logging** - Log all API requests for security review
7. **Webhook signatures** - HMAC-SHA256 signature on all deliveries

---

## Success Metrics

- API adoption: keys created, requests per day
- Error rate: 4xx and 5xx responses
- Latency: p50, p95, p99 response times
- Webhook reliability: delivery success rate
- Documentation: page views, time on page
