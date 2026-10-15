# NameSilo API Integration Plan

## Overview

Allow users to connect their NameSilo account via API key to manage domains they own. This enables automatic DNS configuration for redirect links without manual intervention.

**Design Principle:** NameSilo integration lives in the existing Domains section alongside cPanel - not a separate "Integrations" page. Users never feel like they left the domain management area.

## Goals

1. Users can store their NameSilo API key securely
2. Users can list and import domains from their NameSilo account
3. Users can auto-configure DNS records to point to our servers
4. System can verify DNS propagation status
5. Integration sits alongside cPanel in unified Domains section

---

## UI Design: Unified Domains Section

### Dashboard Sidebar

```
┌──────────────────────────┐
│  📊 Dashboard            │
│                          │
│  REDIRECT LINKS          │
│  ├─ 🔗 Links             │
│  ├─ 📊 Analytics         │
│  └─ ⚙️ Bot Settings      │
│                          │
│  DOMAINS                 │  ◀── All domain providers here
│  ├─ 🌐 My Domains        │
│  ├─ 🔌 cPanel            │  ◀── Existing
│  └─ 🟠 NameSilo          │  ◀── NEW (same pattern)
│                          │
│  SETTINGS                │
│  └─ ⚙️ Account           │
│                          │
└──────────────────────────┘
```

### NameSilo Page (Same Pattern as cPanel)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  🟠 NameSilo                                                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │  Connection Status                                                  │   │
│   ├─────────────────────────────────────────────────────────────────────┤   │
│   │                                                                     │   │
│   │  Status:  ● Connected                         [Disconnect]          │   │
│   │  API Key: ••••••••••••abc123                                       │   │
│   │  Since:   Oct 15, 2026                                              │   │
│   │                                                                     │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │  Your NameSilo Domains                           [↻ Sync] [Import] │   │
│   ├─────────────────────────────────────────────────────────────────────┤   │
│   │                                                                     │   │
│   │  Domain            │ Expires    │ Status   │ Imported │ Actions    │   │
│   │  ─────────────────────────────────────────────────────────────────  │   │
│   │  example.com       │ 2025-12-01 │ Active   │ ✓ Yes    │ [Manage]   │   │
│   │  mysite.io         │ 2026-03-15 │ Active   │ ✓ Yes    │ [Manage]   │   │
│   │  newdomain.dev     │ 2026-08-10 │ Active   │ ○ No     │ [Import]   │   │
│   │  expired.net       │ 2024-01-01 │ Expired  │ —        │ —          │   │
│   │                                                                     │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │  ℹ Imported domains appear in "My Domains" with auto DNS setup     │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Not Connected State

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  🟠 NameSilo                                                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                                                                     │   │
│   │                         🟠                                          │   │
│   │                                                                     │   │
│   │              Connect your NameSilo account                         │   │
│   │                                                                     │   │
│   │     Import domains and auto-configure DNS with one click           │   │
│   │                                                                     │   │
│   │     API Key:  [                                        ]           │   │
│   │                                                                     │   │
│   │     ℹ Get your API key from namesilo.com → Account → API Manager  │   │
│   │                                                                     │   │
│   │                      [Connect NameSilo]                            │   │
│   │                                                                     │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Import Flow (Modal)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  Import Domain: example.com                                            [X] │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   This will:                                                               │
│   ✓ Add domain to your account                                            │
│   ✓ Configure DNS to point to our server (45.67.89.123)                   │
│   ✓ Auto-verify when DNS propagates                                       │
│                                                                             │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │  DNS Changes                                                        │   │
│   ├─────────────────────────────────────────────────────────────────────┤   │
│   │  ➕ A    │ @    │ 45.67.89.123  │ Root domain                       │   │
│   │  ➕ A    │ *    │ 45.67.89.123  │ Wildcard subdomains               │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│   ⚠ DNS changes take up to 15 minutes to propagate                        │
│                                                                             │
│   [Cancel]                                [Import & Configure DNS]          │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### My Domains (Shows All Sources)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  🌐 My Domains                                                  [+ Add]    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   Domain            │ DNS      │ SSL      │ Source       │ Actions         │
│   ──────────────────────────────────────────────────────────────────────── │
│   example.com       │ ✓ Ready  │ ✓ Active │ 🟠 NameSilo  │ [•••]           │
│   mysite.io         │ ✓ Ready  │ ✓ Active │ 🟠 NameSilo  │ [•••]           │
│   cpanel-site.com   │ ✓ Ready  │ ✓ Active │ 🔵 cPanel    │ [•••]           │
│   manual-domain.com │ ✓ Ready  │ ✓ Active │ ⚪ Manual    │ [•••]           │
│   pending.dev       │ ⏳ Pend. │ —        │ 🟠 NameSilo  │ [•••]           │
│                                                                             │
│   Source badges show where domain came from, all managed the same way      │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Phase 1: Database & Settings

### 1.1 User Settings Table Extension

Add NameSilo API key storage to users (same pattern as cPanel credentials):

```sql
-- Migration 004: Add NameSilo integration fields
ALTER TABLE users ADD COLUMN namesilo_api_key_encrypted TEXT;
ALTER TABLE users ADD COLUMN namesilo_connected_at TIMESTAMPTZ;

-- Add source tracking to domains table
ALTER TABLE domains ADD COLUMN IF NOT EXISTS source VARCHAR(50) DEFAULT 'manual';
ALTER TABLE domains ADD COLUMN IF NOT EXISTS provider_domain_id VARCHAR(255);
ALTER TABLE domains ADD COLUMN IF NOT EXISTS dns_status VARCHAR(50) DEFAULT 'not_configured';
ALTER TABLE domains ADD COLUMN IF NOT EXISTS dns_verified_at TIMESTAMPTZ;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS last_synced_at TIMESTAMPTZ;

-- source values: 'manual', 'cpanel', 'namesilo', 'cloudflare' (future)
-- dns_status values: 'not_configured', 'pending', 'verified', 'failed'
```

### 1.2 Encryption

- Use AES-256-GCM encryption for API key storage
- Encryption key from environment variable `NAMESILO_ENCRYPTION_KEY`
- Never log or expose decrypted API keys

### 1.3 Settings UI

Location: `/user/settings` or new `/user/integrations` page

Fields:
- API Key input (password type, masked after save)
- "Test Connection" button
- Connection status indicator
- "Disconnect" button to remove API key

---

## Phase 2: NameSilo API Client

### 2.1 Package Structure

```
pkg/namesilo/
├── client.go       # HTTP client, request handling
├── domains.go      # Domain operations
├── dns.go          # DNS record operations
├── types.go        # Request/response structs
└── errors.go       # Error handling
```

### 2.2 Client Configuration

```go
type Client struct {
    APIKey     string
    BaseURL    string // https://www.namesilo.com/api
    HTTPClient *http.Client
    RateLimit  *rate.Limiter // 1 req per 2 seconds
}

type Config struct {
    APIKey  string
    Timeout time.Duration // default 30s
    Sandbox bool          // use sandbox.namesilo.com
}
```

### 2.3 API Operations

#### Domain Operations

| Method | Endpoint | Description |
|--------|----------|-------------|
| `ListDomains()` | `listDomains` | Get all domains in account |
| `GetDomainInfo(domain)` | `getDomainInfo` | Get domain details |
| `CheckDomain(domain)` | `checkRegisterAvailability` | Check if available |

#### DNS Operations

| Method | Endpoint | Description |
|--------|----------|-------------|
| `ListDNSRecords(domain)` | `dnsListRecords` | List all DNS records |
| `AddDNSRecord(domain, record)` | `dnsAddRecord` | Create new record |
| `UpdateDNSRecord(domain, rrid, record)` | `dnsUpdateRecord` | Update existing |
| `DeleteDNSRecord(domain, rrid)` | `dnsDeleteRecord` | Remove record |

### 2.4 Types

```go
type Domain struct {
    Domain      string    `json:"domain"`
    Created     time.Time `json:"created"`
    Expires     time.Time `json:"expires"`
    Status      string    `json:"status"`
    Locked      bool      `json:"locked"`
    Private     bool      `json:"private"`
    AutoRenew   bool      `json:"auto_renew"`
}

type DNSRecord struct {
    RecordID string `json:"record_id"` // rrid for updates
    Type     string `json:"type"`      // A, AAAA, CNAME, MX, TXT
    Host     string `json:"host"`      // subdomain or @
    Value    string `json:"value"`     // IP or target
    TTL      int    `json:"ttl"`       // seconds
    Distance int    `json:"distance"`  // MX priority
}

type DNSRecordInput struct {
    Type     string `json:"rrtype"`
    Host     string `json:"rrhost"`
    Value    string `json:"rrvalue"`
    TTL      int    `json:"rrttl"`
    Distance int    `json:"rrdistance"` // MX only
}
```

### 2.5 Response Handling

NameSilo returns XML or JSON based on `type` parameter. Use JSON:

```go
type APIResponse struct {
    Reply struct {
        Code   int    `json:"code"`
        Detail string `json:"detail"`
        // Operation-specific fields embedded
    } `json:"reply"`
}

// Success codes: 300 = success, 301 = success (already exists)
// Error codes: 110 = invalid API key, 200 = domain not found, etc.
```

### 2.6 Rate Limiting

```go
// Enforce 1 request per 2 seconds per domain
// Use golang.org/x/time/rate
limiter := rate.NewLimiter(rate.Every(2*time.Second), 1)

func (c *Client) doRequest(ctx context.Context, operation string, params url.Values) (*APIResponse, error) {
    if err := c.limiter.Wait(ctx); err != nil {
        return nil, err
    }
    // ... make request
}
```

---

## Phase 3: Domain Import Feature

### 3.1 Import Flow

1. User clicks "Import from NameSilo" button
2. System calls `ListDomains()` with user's API key
3. Display domains with checkboxes
4. User selects domains to import
5. For each selected domain:
   - Create domain record in our database
   - Mark as `source = "namesilo"`
   - Store NameSilo domain reference

### 3.2 Database Changes

```sql
-- Add source tracking to domains table
ALTER TABLE domains ADD COLUMN source VARCHAR(50) DEFAULT 'manual';
ALTER TABLE domains ADD COLUMN external_id VARCHAR(255);
ALTER TABLE domains ADD COLUMN last_synced_at TIMESTAMPTZ;

-- source: 'manual', 'namesilo', 'cloudflare' (future)
-- external_id: domain name in external system
```

### 3.3 Import UI

Location: `/user/domains` with "Import" button

Modal content:
- Loading spinner while fetching
- List of domains with:
  - Checkbox
  - Domain name
  - Expiry date
  - Status (active, expired, locked)
  - "Already imported" badge if exists
- "Import Selected" button

---

## Phase 4: Auto DNS Configuration

### 4.1 Configuration Options

When user selects a domain, offer:

1. **Full Setup** - Configure root and wildcard
   - `@` A record → server IP
   - `*` A record → server IP (for subdomains)

2. **Subdomain Only** - Single subdomain
   - User specifies subdomain (e.g., `links`)
   - Create A record for that subdomain

3. **Custom** - Manual record selection

### 4.2 Server IP Detection

```go
// Get server's public IP for DNS configuration
func GetServerIP() (string, error) {
    // Option 1: From environment
    if ip := os.Getenv("SERVER_PUBLIC_IP"); ip != "" {
        return ip, nil
    }
    
    // Option 2: Query external service
    resp, _ := http.Get("https://api.ipify.org")
    // parse response
}
```

### 4.3 DNS Setup Flow

```go
func SetupDNSForDomain(ctx context.Context, client *Client, domain string, serverIP string) error {
    // 1. List existing records
    records, err := client.ListDNSRecords(domain)
    
    // 2. Check for conflicts
    for _, r := range records {
        if r.Type == "A" && (r.Host == "@" || r.Host == "*") {
            // Record exists - offer to update or skip
        }
    }
    
    // 3. Add/update records
    // Root record
    err = client.AddDNSRecord(domain, DNSRecordInput{
        Type:  "A",
        Host:  "@",
        Value: serverIP,
        TTL:   3600,
    })
    
    // Wildcard record
    err = client.AddDNSRecord(domain, DNSRecordInput{
        Type:  "A",
        Host:  "*",
        Value: serverIP,
        TTL:   3600,
    })
    
    return nil
}
```

### 4.4 UI Flow

1. User clicks "Configure DNS" on imported domain
2. Modal shows:
   - Current DNS records (read from NameSilo)
   - Proposed changes (highlighted)
   - Server IP that will be used
3. User confirms
4. System makes API calls
5. Show success/error status
6. Note: "DNS changes may take up to 15 minutes to propagate"

---

## Phase 5: DNS Verification & Monitoring

### 5.1 Verification Job

```go
// Run after DNS setup, check if records propagated
func VerifyDNSPropagation(domain, expectedIP string) (bool, error) {
    ips, err := net.LookupHost(domain)
    if err != nil {
        return false, err
    }
    
    for _, ip := range ips {
        if ip == expectedIP {
            return true, nil
        }
    }
    return false, nil
}
```

### 5.2 Background Verification

After DNS setup:
1. Mark domain as `dns_status = "pending"`
2. Schedule verification job (run every 5 minutes)
3. After successful verification, mark `dns_status = "verified"`
4. After 1 hour of failures, mark `dns_status = "failed"`
5. Notify user of final status

### 5.3 Domain Health Dashboard

Show on domain detail page:
- DNS Status: Pending / Verified / Failed
- Last checked: timestamp
- Current DNS records (live query)
- "Refresh DNS" button
- "Re-configure DNS" button if failed

---

## Phase 6: Module Structure

### 6.1 Extend Existing `domains` Module

NameSilo lives inside the domains module as a provider, same pattern as cPanel.

```
modules/domains/
├── module.go                  # Add NameSilo routes
├── handlers/
│   ├── handler.go             # Existing
│   ├── cpanel.go              # Existing cPanel handlers
│   ├── namesilo.go            # NEW: NameSilo handlers
│   └── dns.go                 # NEW: Shared DNS configuration
├── services/
│   ├── domain.go              # Existing
│   ├── cpanel.go              # Existing
│   └── namesilo.go            # NEW: NameSilo business logic
├── templates/
│   ├── list.html              # Existing - add source badge
│   ├── cpanel_*.html          # Existing cPanel templates
│   ├── namesilo.html          # NEW: NameSilo connection page
│   ├── namesilo_import.html   # NEW: Import modal
│   └── dns_setup.html         # NEW: DNS config modal
└── migrations/
    └── 004_add_namesilo.sql   # Add provider fields
```

### 6.2 Shared Provider Package

```
pkg/providers/
├── namesilo/
│   ├── client.go       # HTTP client, rate limiting
│   ├── domains.go      # listDomains, getDomainInfo
│   ├── dns.go          # DNS record operations
│   ├── types.go        # Request/response structs
│   └── errors.go       # Error handling
└── cloudflare/         # Future: same pattern
    └── ...
```

### 6.3 Routes (Under /user/domains)

```go
// NameSilo provider routes at /user/domains/namesilo
r.Route("/namesilo", func(r chi.Router) {
    r.Get("/", h.NameSiloPage)              // Connection page
    r.Post("/connect", h.NameSiloConnect)    // Save API key
    r.Post("/disconnect", h.NameSiloDisconnect)
    r.Get("/sync", h.NameSiloSync)           // Refresh domain list
    
    r.Route("/api", func(r chi.Router) {
        r.Get("/domains", h.NameSiloListDomains)
        r.Post("/import", h.NameSiloImport)
        r.Post("/dns/setup/{domain}", h.NameSiloDNSSetup)
    })
})
```

### 6.4 Menu Items (Same Group as cPanel)

```go
[]MenuItem{
    {
        Title:   "My Domains",
        Icon:    "bi-globe",
        Path:    "/user/domains",
        Order:   10,
        Section: MenuSectionUser,
        Group:   "Domains",
    },
    {
        Title:   "cPanel",
        Icon:    "bi-hdd-rack",
        Path:    "/user/domains/cpanel",
        Order:   11,
        Section: MenuSectionUser,
        Group:   "Domains",
    },
    {
        Title:   "NameSilo",           // NEW
        Icon:    "bi-box",             // Or custom icon
        Path:    "/user/domains/namesilo",
        Order:   12,
        Section: MenuSectionUser,
        Group:   "Domains",
    },
}
```

---

## Phase 7: Security Considerations

### 7.1 API Key Storage

- Encrypt with AES-256-GCM before storing
- Use unique IV per encryption
- Store IV with ciphertext
- Encryption key from environment only

```go
func EncryptAPIKey(plaintext, encryptionKey string) (string, error) {
    block, _ := aes.NewCipher([]byte(encryptionKey))
    gcm, _ := cipher.NewGCM(block)
    nonce := make([]byte, gcm.NonceSize())
    io.ReadFull(rand.Reader, nonce)
    ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}
```

### 7.2 API Key Validation

- Test connection immediately on save
- Mask key in UI after save (show only last 4 chars)
- Log API key usage (not the key itself)
- Rate limit connection attempts

### 7.3 Permission Checks

- Only allow users to manage their own integrations
- Verify domain ownership before DNS changes
- Audit log all DNS modifications

---

## Phase 8: Error Handling

### 8.1 NameSilo Error Codes

| Code | Meaning | User Message |
|------|---------|--------------|
| 110 | Invalid API key | "API key is invalid. Please check and try again." |
| 200 | Domain not in account | "This domain is not in your NameSilo account." |
| 250 | Domain not active | "This domain is not active." |
| 261 | DNS record exists | "DNS record already exists." |
| 262 | Invalid IP address | "Invalid IP address provided." |
| 263 | Invalid record type | "Invalid DNS record type." |
| 280 | DNS record not found | "DNS record not found." |

### 8.2 Retry Logic

```go
func (c *Client) doRequestWithRetry(ctx context.Context, op string, params url.Values) (*APIResponse, error) {
    var lastErr error
    for attempt := 0; attempt < 3; attempt++ {
        resp, err := c.doRequest(ctx, op, params)
        if err == nil {
            return resp, nil
        }
        if !isRetryable(err) {
            return nil, err
        }
        lastErr = err
        time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
    }
    return nil, lastErr
}
```

---

## Phase 9: Testing

### 9.1 Unit Tests

```go
// pkg/namesilo/client_test.go
func TestListDomains(t *testing.T) { ... }
func TestAddDNSRecord(t *testing.T) { ... }
func TestRateLimiting(t *testing.T) { ... }
```

### 9.2 Integration Tests

- Use NameSilo sandbox environment
- Test full flow: connect → import → setup DNS → verify
- Test error scenarios

### 9.3 Mock Server

For CI/CD without sandbox access:

```go
// pkg/namesilo/mock_server_test.go
func NewMockNameSiloServer() *httptest.Server { ... }
```

---

## Phase 10: Environment Variables

```bash
# Required
NAMESILO_ENCRYPTION_KEY=32-byte-key-for-aes256

# Optional
SERVER_PUBLIC_IP=1.2.3.4              # Auto-detected if not set
NAMESILO_SANDBOX=false                # Use sandbox API
NAMESILO_RATE_LIMIT_MS=2000           # Rate limit in milliseconds
```

---

## Implementation Order

| Phase | Description | Effort | Dependencies |
|-------|-------------|--------|--------------|
| 1 | Database & Settings | 2h | None |
| 2 | API Client | 4h | None |
| 3 | Domain Import | 3h | Phase 1, 2 |
| 4 | Auto DNS Config | 4h | Phase 2, 3 |
| 5 | DNS Verification | 2h | Phase 4 |
| 6 | Module Structure | 2h | Phase 1-5 |
| 7 | Security | 1h | Phase 1 |
| 8 | Error Handling | 1h | Phase 2 |
| 9 | Testing | 3h | All |
| 10 | Documentation | 1h | All |

**Total estimated effort: ~23 hours**

---

## Future Enhancements

1. **Cloudflare Integration** - Same pattern, different API
2. **GoDaddy Integration** - Same pattern, different API  
3. **Auto-renewal alerts** - Notify when domain expiring
4. **Bulk DNS updates** - Update multiple domains at once
5. **DNS templates** - Save and reuse DNS configurations
6. **Subdomain management** - Create subdomains for users

---

## References

- [NameSilo API Reference](https://www.namesilo.com/api-reference)
- [NameSilo API Blog Guide](https://www.namesilo.com/blog/en/dns/updating-dns-records-programmatically-via-the-namesilo-api)
- [Go NameSilo Package](https://pkg.go.dev/github.com/r6c/namesilo)
