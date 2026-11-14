# Cloudflare Integration Reference

Technical specification for the Cloudflare integration in GuardBot.

## Authentication

GuardBot uses Cloudflare's Global API Key authentication method.

| Header | Value |
|--------|-------|
| `X-Auth-Email` | Cloudflare account email |
| `X-Auth-Key` | Global API Key |

Credentials are encrypted at rest using AES-256-GCM with the `SESSION_SECRET` environment variable as the key.

## Rate Limiting

Cloudflare allows 1200 requests per 5 minutes. GuardBot enforces a minimum 250ms interval between API calls to stay within limits.

## API Endpoints

### User Endpoints

All endpoints require authentication. Base path: `/user/domains/cloudflare`

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Render Cloudflare connection page |
| POST | `/api/connect` | Connect Cloudflare account |
| POST | `/api/disconnect` | Remove Cloudflare connection |
| GET | `/api/zones` | List all zones (domains) |
| POST | `/api/import` | Import a domain to GuardBot |
| GET | `/api/dns/{zoneID}` | List DNS records for a zone |
| POST | `/api/dns/setup` | Configure DNS for a domain |
| POST | `/api/reconfigure/{id}` | Reconfigure DNS for imported domain |

### Admin Endpoints

Base path: `/admin/domains/api/cloudflare`

| Method | Path | Description |
|--------|------|-------------|
| POST | `/reconfigure-all` | Update DNS for all Cloudflare domains |

## Request/Response Formats

### Connect

```json
// POST /api/connect
{
  "email": "user@example.com",
  "apiKey": "your-global-api-key"
}

// Response
{
  "success": true,
  "data": {
    "message": "Connected to Cloudflare"
  }
}
```

### Import Domain

```json
// POST /api/import
{
  "zoneId": "cloudflare-zone-id",
  "domain": "example.com",
  "configureDns": true
}

// Response
{
  "success": true,
  "data": {
    "id": "dom_abc123",
    "name": "example.com",
    "source": "cloudflare",
    "status": "pending"
  }
}
```

### Setup DNS

```json
// POST /api/dns/setup
{
  "zoneId": "cloudflare-zone-id",
  "domain": "example.com"
}

// Response
{
  "success": true,
  "data": {
    "message": "DNS configured successfully"
  }
}
```

## DNS Records Created

When auto-configure DNS is enabled, these records are created:

| Type | Name | Content | TTL | Proxied |
|------|------|---------|-----|---------|
| A | @ | Server IP | Auto (1) | No |
| A | * | Server IP | Auto (1) | No |
| CNAME | _acme-challenge | acme-dns fulldomain | Auto (1) | No |

Existing A records for @ and * are deleted before creating new ones.

**Proxied is always `false`** — traffic must reach GuardBot directly for bot protection to work. Cloudflare's proxy would interfere with visitor fingerprinting.

## Database Schema

Cloudflare credentials are stored in the `users` table:

| Column | Type | Description |
|--------|------|-------------|
| `cloudflare_email` | VARCHAR(255) | Cloudflare account email |
| `cloudflare_api_key_encrypted` | TEXT | AES-256-GCM encrypted API key |
| `cloudflare_connected_at` | TIMESTAMPTZ | Connection timestamp |

Imported domains use the standard `domains` table with:
- `source`: `"cloudflare"`
- `provider_domain_id`: Cloudflare zone ID

## Auto-Trigger on IP Change

When a deploy server's IP address changes, DNS is automatically reconfigured for all providers (NameSilo, cPanel, Cloudflare). This is triggered by the servers module callback.

The reconfigure operation:
1. Queries all domains with `source = 'cloudflare'`
2. For each domain, calls `SetupDNS` with the new server IP
3. Returns success/failure count per provider

## Environment Variables

| Variable | Description |
|----------|-------------|
| `CLOUDFLARE_ENCRYPTION_KEY` | Optional. Key for encrypting API keys. Falls back to `SESSION_SECRET`. |
| `GUARDBOT_SERVER_IP` | Server IP for DNS records. Auto-detected if not set. |
| `ACME_DNS_DOMAIN` | Domain for acme-dns CNAME delegation. |

## Error Codes

Cloudflare API errors are passed through with their original codes:

| Code | Meaning |
|------|---------|
| 9103 | Invalid credentials |
| 10000 | Authentication error |
| 81057 | Record already exists |
| 81058 | Record does not exist |

## Related

- [How to Connect Cloudflare](../how-to/connect-cloudflare.md) — Step-by-step guide
- [Botection Callback API](botection-callback-api.md) — Per-link bot blocking decisions
