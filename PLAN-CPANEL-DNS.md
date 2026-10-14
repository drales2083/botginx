# Plan: cPanel Auto-DNS Verification

## Overview

Add cPanel integration to automatically create DNS records (TXT for verification, ACME challenges) when users add domains. Users choose between manual DNS setup or connecting their cPanel account for automatic setup.

## Goals

1. Eliminate manual DNS configuration for cPanel users
2. Instant domain verification (no waiting for DNS propagation)
3. Automatic SSL certificate provisioning
4. Secure storage of cPanel credentials

---

## Phase 1: cPanel API Client Package

### 1.1 Create `pkg/cpanel/` package

```
pkg/cpanel/
├── client.go       # HTTP client with auth
├── dns.go          # DNS record operations
├── types.go        # Request/response types
└── errors.go       # Error handling
```

### 1.2 Client Features

```go
type Client struct {
    host     string  // e.g., "example.com:2083"
    username string
    apiToken string
    http     *http.Client
}

// Core methods
func NewClient(host, username, apiToken string) *Client
func (c *Client) TestConnection() error
func (c *Client) GetDomains() ([]string, error)

// DNS operations
func (c *Client) AddTXTRecord(domain, name, value string) error
func (c *Client) RemoveTXTRecord(domain, name string) error
func (c *Client) GetTXTRecords(domain string) ([]TXTRecord, error)
func (c *Client) AddARecord(domain, name, ip string) error
```

### 1.3 API Endpoints Used

| Operation | cPanel API |
|-----------|------------|
| Test connection | `UAPI DomainInfo::list_domains` |
| List DNS records | `API2 ZoneEdit::fetchzone` |
| Add TXT record | `API2 ZoneEdit::add_zone_record` |
| Remove TXT record | `API2 ZoneEdit::remove_zone_record` |
| Add A record | `API2 ZoneEdit::add_zone_record` |

### 1.4 Security Considerations

- API tokens stored encrypted (same as hosting passwords)
- HTTPS only for cPanel connections
- Allow self-signed certs (common on cPanel servers)
- Token validation on save (test connection before storing)

---

## Phase 2: Database Changes

### 2.1 New Table: `cpanel_connections`

Store cPanel credentials at USER level (one connection can manage multiple domains).

```sql
CREATE TABLE cpanel_connections (
    id VARCHAR(26) PRIMARY KEY,
    user_id VARCHAR(26) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,                    -- Display name (e.g., "My Hosting")
    host VARCHAR(255) NOT NULL,                    -- e.g., "example.com:2083"
    username VARCHAR(255) NOT NULL,
    api_token_encrypted TEXT NOT NULL,
    is_active BOOLEAN DEFAULT true,
    last_used_at TIMESTAMP,
    last_error TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    
    UNIQUE(user_id, host, username)
);

CREATE INDEX idx_cpanel_connections_user ON cpanel_connections(user_id);
```

### 2.2 Update `domains` Table

Add reference to cPanel connection used for DNS:

```sql
ALTER TABLE domains ADD COLUMN cpanel_connection_id VARCHAR(26) REFERENCES cpanel_connections(id);
ALTER TABLE domains ADD COLUMN cpanel_auto_dns BOOLEAN DEFAULT false;
```

### 2.3 Migration File

Create: `modules/domains/migrations/009_cpanel_connections.sql`

---

## Phase 3: Domain Add UI (Two Paths)

### 3.1 Add Domain Modal - Step 1: Choose Method

```
┌─────────────────────────────────────────────────────────┐
│  Add New Domain                                         │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  How would you like to set up DNS?                      │
│                                                         │
│  ┌─────────────────────┐  ┌─────────────────────────┐  │
│  │                     │  │                         │  │
│  │  📝 Manual Setup    │  │  🔗 Connect cPanel      │  │
│  │                     │  │                         │  │
│  │  I'll add DNS       │  │  Auto-configure DNS     │  │
│  │  records myself     │  │  via cPanel API         │  │
│  │                     │  │                         │  │
│  └─────────────────────┘  └─────────────────────────┘  │
│                                                         │
│  [ Cancel ]                                             │
└─────────────────────────────────────────────────────────┘
```

### 3.2 Manual Path (Existing Flow)

1. Enter domain name
2. Show TXT record instructions
3. User adds manually
4. Click "Verify DNS"

### 3.3 cPanel Path - Step 2a: Select/Add Connection

```
┌─────────────────────────────────────────────────────────┐
│  Connect cPanel                              [< Back]   │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  Select existing connection or add new:                 │
│                                                         │
│  ○ My Hosting (user@example.com:2083)                  │
│  ○ Client Server (admin@client.com:2083)               │
│  ● + Add new cPanel connection                          │
│                                                         │
│  ┌─────────────────────────────────────────────────┐   │
│  │ Connection Name: [My cPanel Account          ]  │   │
│  │ cPanel Host:     [example.com:2083           ]  │   │
│  │ Username:        [cpanel_username            ]  │   │
│  │ API Token:       [••••••••••••••••••••       ]  │   │
│  │                                                  │   │
│  │ [How to get an API token?]                      │   │
│  └─────────────────────────────────────────────────┘   │
│                                                         │
│  [ Cancel ]                    [ Test & Continue → ]   │
└─────────────────────────────────────────────────────────┘
```

### 3.4 cPanel Path - Step 2b: Select Domain

After connection verified, show domains available on that cPanel:

```
┌─────────────────────────────────────────────────────────┐
│  Select Domain                               [< Back]   │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  Domains found on your cPanel account:                  │
│                                                         │
│  ○ example.com (main domain)                           │
│  ○ client-site.com (addon domain)                      │
│  ○ promo.example.com (subdomain)                       │
│                                                         │
│  Or enter a subdomain:                                  │
│  [ offers.example.com                              ]    │
│                                                         │
│  [ Cancel ]                    [ Add & Verify → ]      │
└─────────────────────────────────────────────────────────┘
```

### 3.5 cPanel Path - Step 3: Auto-Setup

```
┌─────────────────────────────────────────────────────────┐
│  Setting Up Domain                                      │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  example.com                                            │
│                                                         │
│  ✓ Connected to cPanel                                 │
│  ✓ Added verification TXT record                       │
│  ✓ DNS verified                                        │
│  ◐ Generating SSL certificate...                       │
│  ○ Complete                                             │
│                                                         │
│  [━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━░░░░░░░] 75%        │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

---

## Phase 4: Backend Services

### 4.1 cPanel Connection Service

`modules/domains/services/cpanel_service.go`

```go
type CpanelService struct {
    db     *sqlx.DB
    crypto *crypto.Service
}

// Connection management
func (s *CpanelService) Create(userID string, input CreateCpanelConnectionInput) (*CpanelConnection, error)
func (s *CpanelService) List(userID string) ([]CpanelConnection, error)
func (s *CpanelService) Get(userID, connectionID string) (*CpanelConnection, error)
func (s *CpanelService) Delete(userID, connectionID string) error
func (s *CpanelService) TestConnection(userID, connectionID string) error

// DNS operations
func (s *CpanelService) AddVerificationTXT(connectionID, domain, token string) error
func (s *CpanelService) AddAcmeChallengeTXT(connectionID, domain, token string) error
func (s *CpanelService) RemoveVerificationTXT(connectionID, domain string) error
func (s *CpanelService) ListDomains(connectionID string) ([]string, error)
```

### 4.2 Updated Domain Service

Add cPanel-aware methods:

```go
// In domain_service.go
func (s *DomainService) CreateWithCpanel(userID string, input CreateDomainWithCpanelInput) (*Domain, error) {
    // 1. Validate cPanel connection belongs to user
    // 2. Create domain record with cpanel_connection_id
    // 3. Add verification TXT via cPanel API
    // 4. Immediately verify (should pass since we just added it)
    // 5. Start SSL provisioning
}
```

### 4.3 Auto-ACME Integration

When SSL certificate needs renewal:

```go
func (s *VerificationService) ProvisionSSLWithCpanel(domain *Domain) error {
    if domain.CpanelConnectionID != nil {
        // 1. Get ACME challenge token from certbot/lego
        // 2. Add _acme-challenge TXT via cPanel
        // 3. Complete ACME challenge
        // 4. Remove _acme-challenge TXT
    }
}
```

---

## Phase 5: API Handlers

### 5.1 New Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/user/cpanel/connections` | List user's cPanel connections |
| POST | `/user/cpanel/connections` | Add new connection |
| DELETE | `/user/cpanel/connections/{id}` | Remove connection |
| POST | `/user/cpanel/connections/{id}/test` | Test connection |
| GET | `/user/cpanel/connections/{id}/domains` | List domains on cPanel |

### 5.2 Updated Domain Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/user/domains` | Add domain (updated to accept cpanel_connection_id) |

---

## Phase 6: UI Components

### 6.1 Files to Create/Modify

```
modules/domains/templates/
├── index.html           # Update "Add Domain" button/modal
├── add-modal.html       # New: Two-path add modal (partial)
├── cpanel-connect.html  # New: cPanel connection form (partial)
└── cpanel-setup.html    # New: Auto-setup progress (partial)

modules/domains/handlers/
├── handler.go           # Add cPanel connection handlers
└── cpanel_handlers.go   # New: cPanel-specific handlers
```

### 6.2 JavaScript

- Form validation for cPanel fields
- Connection test with loading state
- Auto-setup progress polling
- Domain selector from cPanel list

---

## Phase 7: Error Handling

### 7.1 cPanel Errors

| Error | User Message | Action |
|-------|--------------|--------|
| Connection refused | "Cannot connect to cPanel server" | Check host/port |
| 401 Unauthorized | "Invalid username or API token" | Re-enter credentials |
| Domain not found | "Domain not found on this cPanel" | Select different domain |
| Permission denied | "API token lacks DNS permission" | Generate new token with ZoneEdit |
| Record exists | "TXT record already exists" | Offer to update |

### 7.2 Fallback to Manual

If cPanel auto-setup fails at any point:
1. Show error message
2. Offer "Continue with manual setup" option
3. Pre-fill domain name in manual form

---

## Phase 8: Testing

### 8.1 Unit Tests

- `pkg/cpanel/client_test.go` - Mock HTTP responses
- `services/cpanel_service_test.go` - Connection CRUD

### 8.2 Integration Tests

- Real cPanel server connection (staging)
- Add/remove TXT record cycle
- Full domain setup flow

### 8.3 Manual Test Cases

1. Add domain via cPanel - happy path
2. Add domain via cPanel - invalid credentials
3. Add domain via cPanel - domain not on account
4. Switch from manual to cPanel mid-setup
5. Delete domain - verify TXT cleanup
6. SSL renewal via cPanel ACME

---

## Implementation Order

| Step | Task | Est. Time |
|------|------|-----------|
| 1 | Create `pkg/cpanel/` client package | 2h |
| 2 | Database migration for cpanel_connections | 30m |
| 3 | CpanelService with CRUD operations | 1h |
| 4 | API handlers for connections | 1h |
| 5 | Update domain add UI (two paths) | 2h |
| 6 | cPanel connection form + test button | 1h |
| 7 | Domain selector from cPanel | 1h |
| 8 | Auto-setup flow with progress | 2h |
| 9 | ACME integration for SSL | 2h |
| 10 | Error handling + fallback | 1h |
| 11 | Testing | 2h |

**Total: ~15 hours**

---

## Future Enhancements

1. **Cloudflare API** - Third option alongside Manual and cPanel
2. **Auto-detect cPanel** - Check if domain:2083 responds to cPanel
3. **Bulk domain import** - Import all domains from cPanel at once
4. **A record management** - Auto-point domain to our server
5. **Connection health monitoring** - Alert if cPanel token expires

---

## Questions to Resolve

1. ~~Store credentials per domain or per user?~~ → **Per user** (one connection, many domains)
2. Should we auto-remove TXT records when domain is deleted? → **Yes, cleanup**
3. Support for cPanel on non-standard ports? → **Yes, user enters host:port**
4. Allow password auth or API token only? → **API token only** (more secure)
