# API Keys System Plan

Industry-standard API key management for botginx.

## Database Schema

```sql
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    key_hash VARCHAR(255) NOT NULL,
    key_prefix VARCHAR(8) NOT NULL,  -- "bg_live_" or "bg_test_" + last 4 chars for display
    scopes TEXT[] NOT NULL DEFAULT '{}',
    expires_at TIMESTAMP,
    rate_limit INT DEFAULT 1000,  -- requests per hour
    is_test BOOLEAN DEFAULT false,
    is_active BOOLEAN DEFAULT true,
    last_used_at TIMESTAMP,
    requests_count BIGINT DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    rotated_from_id UUID REFERENCES api_keys(id)
);

CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX idx_api_keys_key_hash ON api_keys(key_hash);
```

## Key Format

- Production: `bg_live_` + 32 random chars (e.g., `bg_live_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6`)
- Test/Sandbox: `bg_test_` + 32 random chars

Display in UI: `bg_live_****p5p6` (prefix + last 4 only)

## Scopes (Permissions)

| Scope | Description |
|-------|-------------|
| `links:read` | Read redirect links |
| `links:write` | Create/update redirect links |
| `links:delete` | Delete redirect links |
| `domains:read` | Read domains |
| `domains:manage` | Add/remove/configure domains |
| `analytics:read` | Read analytics data |
| `qrcodes:read` | Read QR codes |
| `qrcodes:write` | Create/update QR codes |
| `webhooks:manage` | Manage webhook endpoints |
| `account:read` | Read account info |

Preset bundles:
- **Read Only**: `links:read`, `domains:read`, `analytics:read`, `qrcodes:read`, `account:read`
- **Standard**: All read + `links:write`, `qrcodes:write`
- **Full Access**: All scopes

## API Endpoints

### Key Management (UI)
- `GET /user/apikeys` - List keys page
- `POST /user/apikeys/api/create` - Create new key
- `DELETE /user/apikeys/api/{id}` - Revoke key
- `POST /user/apikeys/api/{id}/rotate` - Rotate key (new key, old expires in 24h)
- `GET /user/apikeys/api/{id}/usage` - Usage stats

### External API (using keys)
- `POST /api/v1/links` - Create link
- `GET /api/v1/links` - List links
- `GET /api/v1/links/{id}` - Get link
- `PUT /api/v1/links/{id}` - Update link
- `DELETE /api/v1/links/{id}` - Delete link
- `GET /api/v1/analytics/{link_id}` - Get analytics
- `GET /api/v1/domains` - List domains
- etc.

## Authentication

Header: `Authorization: Bearer bg_live_xxxxx`

Or query param (not recommended): `?api_key=bg_live_xxxxx`

## Rate Limiting

- Default: 1000 requests/hour per key
- Configurable per key
- Headers returned:
  - `X-RateLimit-Limit: 1000`
  - `X-RateLimit-Remaining: 950`
  - `X-RateLimit-Reset: 1699999999`

## Security

- Keys hashed with bcrypt before storage
- Shown only once at creation (modal with copy button)
- Key rotation: new key issued, old key works for 24h grace period
- Automatic expiration support
- Usage logging for audit

## UI Design

### List Page
- Table: Name, Key (masked), Scopes, Last Used, Requests, Status, Actions
- Create button opens modal
- Actions: View usage, Rotate, Revoke

### Create Modal
1. Name (required)
2. Environment: Production / Test
3. Scopes: Checkboxes or preset dropdown
4. Expiration: Never / Custom date
5. Rate limit: Default or custom

### After Creation
- Modal shows full key ONCE
- Copy button
- Warning: "This key won't be shown again"

## Files to Create

```
modules/apikeys/
├── module.go
├── handlers/
│   └── handler.go
├── services/
│   └── service.go
├── models/
│   └── models.go
└── templates/
    └── index.html
```

## Future Enhancements

- Webhook delivery with key auth
- API usage dashboard with charts
- Key-specific analytics
- Team keys (shared across org)
- OAuth2 client credentials flow
