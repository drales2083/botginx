# Domain Addition System Reference

This reference documents the domain addition system for agent integration. The system supports three modes: Manual, cPanel API, and NameSilo API.

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        Domain System                             │
├─────────────────────────────────────────────────────────────────┤
│  Manual Mode          cPanel Mode           NameSilo Mode        │
│  (/user/domains/new)  (/user/domains/cpanel) (/user/domains/namesilo)
│         │                   │                      │             │
│         ▼                   ▼                      ▼             │
│  ┌──────────────┐   ┌──────────────┐     ┌──────────────┐       │
│  │ DomainService│   │CpanelService │     │NameSiloService│       │
│  └──────┬───────┘   └──────┬───────┘     └──────┬───────┘       │
│         │                   │                    │               │
│         └───────────────────┼────────────────────┘               │
│                             ▼                                    │
│                    ┌──────────────┐                              │
│                    │   domains    │  (PostgreSQL table)          │
│                    └──────────────┘                              │
└─────────────────────────────────────────────────────────────────┘
```

---

## Data Models

### Domain

The core domain entity. All three modes create this record.

**File:** `/home/this/dev/code/botginx/modules/domains/models/domain.go:23-80`

```go
type Domain struct {
    ID          string    `db:"id" json:"id"`
    UserID      string    `db:"user_id" json:"userId"`
    Name        string    `db:"name" json:"name"` // e.g., example.com or *.example.com
    ServerID    *string   `db:"server_id" json:"serverId,omitempty"`
    VerifyToken string    `db:"verify_token" json:"verifyToken"`
    DNSVerified bool      `db:"dns_verified" json:"dnsVerified"`
    SSLEnabled  bool      `db:"ssl_enabled" json:"sslEnabled"`
    IsShared    bool      `db:"is_shared" json:"isShared"`
    CreatedAt   time.Time `db:"created_at" json:"createdAt"`
    UpdatedAt   time.Time `db:"updated_at" json:"updatedAt"`

    // External setup fields
    SetupType string `db:"setup_type" json:"setupType"` // "direct" or "external"
    SetupStep string `db:"setup_step" json:"setupStep"` // pending, dns_waiting, ssl_waiting, ssl_generating, complete

    // Source tracking
    Source            string  `db:"source" json:"source"`                 // manual, namesilo, cpanel
    ProviderDomainID  *string `db:"provider_domain_id" json:"providerDomainId,omitempty"`
    DNSAutoConfigured bool    `db:"dns_auto_configured" json:"dnsAutoConfigured"`

    // cPanel integration
    CpanelConnectionID *string `db:"cpanel_connection_id" json:"cpanelConnectionId,omitempty"`
    CpanelAutoDNS      bool    `db:"cpanel_auto_dns" json:"cpanelAutoDns"`
}
```

### CreateDomainInput

Input for creating a domain (all modes).

**File:** `/home/this/dev/code/botginx/modules/domains/models/domain.go:140-144`

```go
type CreateDomainInput struct {
    Name               string  `json:"name" validate:"required"`
    ServerID           string  `json:"serverId"`
    CpanelConnectionID *string `json:"cpanelConnectionId"` // Optional: use cPanel for auto-DNS
}
```

### CpanelConnection

Stores encrypted cPanel API credentials.

**File:** `/home/this/dev/code/botginx/modules/domains/models/cpanel_connection.go:6-24`

```go
type CpanelConnection struct {
    ID                string     `db:"id" json:"id"`
    UserID            string     `db:"user_id" json:"userId"`
    Name              string     `db:"name" json:"name"`
    Host              string     `db:"host" json:"host"` // hostname:port
    Username          string     `db:"username" json:"username"`
    APITokenEncrypted string     `db:"api_token_encrypted" json:"-"`
    IsActive          bool       `db:"is_active" json:"isActive"`
    LastUsedAt        *time.Time `db:"last_used_at" json:"lastUsedAt,omitempty"`
    LastError         *string    `db:"last_error" json:"lastError,omitempty"`
    CreatedAt         time.Time  `db:"created_at" json:"createdAt"`
    UpdatedAt         time.Time  `db:"updated_at" json:"updatedAt"`
    APIToken          string     `db:"-" json:"-"` // Decrypted at runtime
    DomainCount       int        `db:"domain_count" json:"domainCount,omitempty"`
}
```

### NameSiloDomain

Domain fetched from NameSilo API.

**File:** `/home/this/dev/code/botginx/modules/domains/models/namesilo.go:15-25`

```go
type NameSiloDomain struct {
    Domain     string     `json:"domain"`
    Created    *time.Time `json:"created,omitempty"`
    Expires    *time.Time `json:"expires,omitempty"`
    Status     string     `json:"status"` // Active, Expired, etc.
    Locked     bool       `json:"locked"`
    Private    bool       `json:"private"`  // WHOIS privacy
    AutoRenew  bool       `json:"autoRenew"`
    IsImported bool       `json:"isImported"` // already in our system
    LocalID    string     `json:"localId,omitempty"` // our domain ID if imported
}
```

---

## Mode 1: Manual Domain Addition

User adds domain manually, then configures DNS at their registrar.

### API Endpoints

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/user/domains/new` | `handler.New` | Render add domain form |
| POST | `/user/domains/api/` | `handler.APICreate` | Create domain |
| GET | `/user/domains/{id}/setup` | `handler.ExternalSetup` | DNS setup wizard |

### Create Domain Request

**Endpoint:** `POST /user/domains/api/`

**File:** `/home/this/dev/code/botginx/modules/domains/handlers/handler.go:233-270`

```go
// Request body
type CreateRequest struct {
    Name string `json:"name"` // e.g., "example.com" or "*.example.com"
}

// Response (201 Created)
type CreateResponse struct {
    Domain Domain `json:"domain"`
}

// Error responses
// 400: "Domain name is required"
// 409: "Domain already registered"
```

**Example:**

```bash
curl -X POST https://guardbot.sbs/user/domains/api/ \
  -H "Content-Type: application/json" \
  -H "Cookie: session=<session_token>" \
  -d '{"name": "example.com"}'
```

**Response:**
```json
{
  "domain": {
    "id": "abc123def456",
    "userId": "user_789",
    "name": "example.com",
    "verifyToken": "guardbot-a1b2c3d4e5f6",
    "dnsVerified": false,
    "sslEnabled": false,
    "setupStep": "pending",
    "source": "manual"
  }
}
```

### Service Implementation

**File:** `/home/this/dev/code/botginx/modules/domains/services/domain_service.go:181-260`

```go
// Create adds a domain owned by one user.
func (s *DomainService) Create(userID string, input models.CreateDomainInput) (*models.Domain, error) {
    return s.create(userID, input, false)
}

func (s *DomainService) create(userID string, input models.CreateDomainInput, shared bool) (*models.Domain, error) {
    // Check if domain exists for this user
    existing, _ := s.GetByName(userID, input.Name)
    if existing != nil {
        return nil, ErrDomainExists
    }

    domain := &models.Domain{
        ID:          s.generateID(),
        UserID:      userID,
        Name:        input.Name,
        VerifyToken: "guardbot-" + s.generateID()[:12],
        DNSVerified: false,
        SSLEnabled:  false,
        IsShared:    shared,
        CreatedAt:   time.Now(),
        UpdatedAt:   time.Now(),
    }

    // Insert into database
    _, err := s.db.Exec(`
        INSERT INTO domains (id, user_id, name, verify_token, dns_verified, ssl_enabled, 
                            is_shared, created_at, updated_at, source)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'manual')
    `, domain.ID, domain.UserID, domain.Name, domain.VerifyToken, 
       domain.DNSVerified, domain.SSLEnabled, domain.IsShared, 
       domain.CreatedAt, domain.UpdatedAt)

    return domain, err
}
```

### DNS Setup Flow

After manual creation, user must add DNS records:

1. **TXT Record** (verification): `_guardbot-verify.example.com` → `guardbot-a1b2c3d4e5f6`
2. **A Record** (traffic): `*.example.com` → `<deploy_server_ip>`

**File:** `/home/this/dev/code/botginx/modules/domains/templates/external_setup.html`

---

## Mode 2: cPanel API Integration

User connects cPanel account, imports domains with auto-DNS.

### API Endpoints

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/user/domains/cpanel/api` | `cpanelHandler.ListConnections` | List cPanel connections |
| POST | `/user/domains/cpanel/api` | `cpanelHandler.CreateConnection` | Add cPanel connection |
| POST | `/user/domains/cpanel/api/test` | `cpanelHandler.TestNewConnection` | Test credentials before saving |
| GET | `/user/domains/cpanel/api/{id}` | `cpanelHandler.GetConnection` | Get connection details |
| PUT | `/user/domains/cpanel/api/{id}` | `cpanelHandler.UpdateConnection` | Update connection |
| DELETE | `/user/domains/cpanel/api/{id}` | `handler.APIDeleteCpanelConnection` | Delete connection |
| POST | `/user/domains/cpanel/api/{id}/test` | `cpanelHandler.TestConnection` | Test existing connection |
| GET | `/user/domains/cpanel/api/{id}/domains` | `cpanelHandler.ListDomains` | List domains from cPanel |

### Connect cPanel Account

**Endpoint:** `POST /user/domains/cpanel/api`

**File:** `/home/this/dev/code/botginx/modules/domains/handlers/cpanel_handlers.go:56-87`

```go
// Request body
type CreateCpanelConnectionInput struct {
    Name     string `json:"name"`     // Display name (e.g., "My Hosting")
    Host     string `json:"host"`     // e.g., "server.host.com:2083"
    Username string `json:"username"` // cPanel username
    APIToken string `json:"apiToken"` // cPanel API token
}

// Response (201 Created)
type ConnectionResponse struct {
    Connection CpanelConnection `json:"connection"`
}

// Error responses
// 400: "Name, host, username, and API token are required"
// 401: "Invalid cPanel credentials"
// 409: "This cPanel account is already connected"
// 502: "Unable to connect to cPanel server"
```

**Example:**

```bash
curl -X POST https://guardbot.sbs/user/domains/cpanel/api \
  -H "Content-Type: application/json" \
  -H "Cookie: session=<session_token>" \
  -d '{
    "name": "My Hosting",
    "host": "server.myhost.com:2083",
    "username": "myuser",
    "apiToken": "ABCDEF123456"
  }'
```

### List Domains from cPanel

**Endpoint:** `GET /user/domains/cpanel/api/{connectionId}/domains`

**File:** `/home/this/dev/code/botginx/modules/domains/handlers/cpanel_handlers.go:192-205`

**Response:**
```json
{
  "mainDomain": "example.com",
  "addonDomains": ["addon1.com", "addon2.com"],
  "subDomains": ["blog.example.com"],
  "parkedDomains": ["alias.com"],
  "existingDomains": ["example.com"]
}
```

### Import Domain from cPanel

When creating a domain with `cpanelConnectionId`, the system auto-configures DNS.

**File:** `/home/this/dev/code/botginx/modules/domains/services/cpanel_service.go:422-460`

```go
// TriggerAutoDNS adds all required DNS records for a domain using its cPanel connection:
// - Verification TXT record: _guardbot-verify.domain.com → guardbot-<token>
// - Wildcard A record: *.domain.com → <deploy_server_ip>
func (s *CpanelService) TriggerAutoDNS(connectionID, domainName, verifyToken, deployIP string) error {
    client, err := s.getClientForConnection(connectionID)
    if err != nil {
        return err
    }

    baseDomain := getBaseDomain(domainName)

    // Add verification TXT record
    err = client.AddDNSRecord(baseDomain, cpanel.DNSRecord{
        Type:  "TXT",
        Name:  "_guardbot-verify",
        Value: verifyToken,
        TTL:   14400,
    })
    if err != nil {
        return fmt.Errorf("failed to add verification TXT: %w", err)
    }

    // Add wildcard A record
    err = client.AddDNSRecord(baseDomain, cpanel.DNSRecord{
        Type:  "A",
        Name:  "*",
        Value: deployIP,
        TTL:   14400,
    })
    if err != nil {
        return fmt.Errorf("failed to add wildcard A: %w", err)
    }

    return nil
}
```

### cPanel Client

**File:** `/home/this/dev/code/botginx/pkg/cpanel/client.go`

```go
// NewClientSimple creates a client with minimal config
func NewClientSimple(host, username, apiToken string) *Client {
    return NewClient(Config{
        Host:     host,
        Username: username,
        APIToken: apiToken,
        SkipTLS:  true, // cPanel often uses self-signed certs
    })
}

// TestConnection verifies the connection and credentials
func (c *Client) TestConnection() error {
    _, err := c.GetDomains()
    return err
}

// GetDomains returns all domains on this cPanel account
func (c *Client) GetDomains() (*DomainInfo, error) {
    body, err := c.callUAPI("DomainInfo", "list_domains", nil)
    // ... parse response
}

// AddDNSRecord adds a DNS record via cPanel Zone Editor
func (c *Client) AddDNSRecord(domain string, record DNSRecord) error {
    params := url.Values{}
    params.Set("domain", domain)
    params.Set("type", record.Type)
    params.Set("name", record.Name)
    params.Set("value", record.Value)
    params.Set("ttl", strconv.Itoa(record.TTL))

    _, err := c.callUAPI("DNS", "add_record", params)
    return err
}
```

---

## Mode 3: NameSilo API Integration

User connects NameSilo API key, imports domains with full DNS automation.

### API Endpoints

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/user/domains/namesilo/` | `namesiloHandler.Page` | NameSilo management page |
| POST | `/user/domains/namesilo/api/connect` | `namesiloHandler.Connect` | Connect API key |
| POST | `/user/domains/namesilo/api/disconnect` | `namesiloHandler.Disconnect` | Remove API key |
| GET | `/user/domains/namesilo/api/domains` | `namesiloHandler.ListDomains` | List domains from NameSilo |
| POST | `/user/domains/namesilo/api/import` | `namesiloHandler.Import` | Import domain |
| POST | `/user/domains/namesilo/api/dns/{domain}` | `namesiloHandler.SetupDNS` | Configure DNS |
| GET | `/user/domains/namesilo/api/dns/{domain}` | `namesiloHandler.GetDNSRecords` | Get DNS records |
| POST | `/user/domains/namesilo/api/reconfigure` | `namesiloHandler.Reconfigure` | Update all domains |
| POST | `/user/domains/namesilo/api/ssl/{domain}` | `namesiloHandler.GenerateSSL` | Generate SSL |

### Connect NameSilo API

**Endpoint:** `POST /user/domains/namesilo/api/connect`

**File:** `/home/this/dev/code/botginx/modules/domains/handlers/namesilo_handlers.go:70-94`

```go
// Request body
type ConnectRequest struct {
    APIKey string `json:"apiKey"`
}

// Response (200 OK)
{
    "message": "Connected to NameSilo"
}

// Error responses
// 400: "API key is required"
// 400: "Invalid API key" (if connection test fails)
```

**Example:**

```bash
curl -X POST https://guardbot.sbs/user/domains/namesilo/api/connect \
  -H "Content-Type: application/json" \
  -H "Cookie: session=<session_token>" \
  -d '{"apiKey": "your-namesilo-api-key"}'
```

### List Domains from NameSilo

**Endpoint:** `GET /user/domains/namesilo/api/domains`

**File:** `/home/this/dev/code/botginx/modules/domains/handlers/namesilo_handlers.go:110-121`

**Response:**
```json
[
  {
    "domain": "example.com",
    "status": "Active",
    "expires": "2027-01-15T00:00:00Z",
    "locked": true,
    "private": true,
    "autoRenew": true,
    "isImported": false,
    "localId": ""
  },
  {
    "domain": "another.com",
    "status": "Active",
    "isImported": true,
    "localId": "abc123def456"
  }
]
```

### Import Domain

**Endpoint:** `POST /user/domains/namesilo/api/import`

**File:** `/home/this/dev/code/botginx/modules/domains/handlers/namesilo_handlers.go:123-148`

```go
// Request body
type NameSiloImportInput struct {
    Domain       string `json:"domain"`       // e.g., "example.com"
    ConfigureDNS bool   `json:"configureDns"` // true = auto-setup DNS
}

// Response (200 OK)
{
    "message": "Domain imported successfully",
    "domainId": "abc123def456"
}
```

**Example:**

```bash
curl -X POST https://guardbot.sbs/user/domains/namesilo/api/import \
  -H "Content-Type: application/json" \
  -H "Cookie: session=<session_token>" \
  -d '{"domain": "example.com", "configureDns": true}'
```

### Service Implementation

**File:** `/home/this/dev/code/botginx/modules/domains/services/namesilo_service.go`

```go
// ImportDomain imports a domain from NameSilo with optional DNS auto-config
func (s *NameSiloService) ImportDomain(ctx context.Context, userID string, input models.NameSiloImportInput) (string, error) {
    // Create domain in our system
    domain, err := s.domainService.Create(userID, models.CreateDomainInput{
        Name: input.Domain,
    })
    if err != nil {
        return "", err
    }

    // Mark source as namesilo
    s.db.Exec(`UPDATE domains SET source = 'namesilo' WHERE id = $1`, domain.ID)

    // Auto-configure DNS if requested
    if input.ConfigureDNS {
        if err := s.SetupDNS(ctx, userID, input.Domain); err != nil {
            // Log but don't fail - domain was created
            log.Printf("DNS setup failed for %s: %v", input.Domain, err)
        }
    }

    return domain.ID, nil
}

// SetupDNS configures DNS records via NameSilo API
func (s *NameSiloService) SetupDNS(ctx context.Context, userID, domain string) error {
    client, err := s.getClient(userID)
    if err != nil {
        return err
    }

    deployIP := s.getDeployIP()
    verifyToken := s.getVerifyToken(domain)

    // Add verification TXT record
    err = client.AddDNSRecord(domain, namesilo.DNSRecord{
        Type:  "TXT",
        Host:  "_guardbot-verify",
        Value: verifyToken,
    })
    if err != nil {
        return fmt.Errorf("TXT record failed: %w", err)
    }

    // Add wildcard A record for *.domain.com
    err = client.AddDNSRecord(domain, namesilo.DNSRecord{
        Type:  "A",
        Host:  "*",
        Value: deployIP,
    })
    if err != nil {
        return fmt.Errorf("A record failed: %w", err)
    }

    // Mark as auto-configured
    s.db.Exec(`
        UPDATE domains 
        SET dns_auto_configured = true, dns_verified = true 
        WHERE name = $1 AND user_id = $2
    `, domain, userID)

    return nil
}
```

### Reconfigure All Domains

Updates DNS for all NameSilo domains (e.g., when server IP changes).

**Endpoint:** `POST /user/domains/namesilo/api/reconfigure`

**File:** `/home/this/dev/code/botginx/modules/domains/services/namesilo_service.go:580-650`

---

## Database Schema

### domains table

```sql
CREATE TABLE domains (
    id VARCHAR(24) PRIMARY KEY,
    user_id VARCHAR(24) NOT NULL REFERENCES users(id),
    name VARCHAR(255) NOT NULL,
    server_id VARCHAR(24) REFERENCES servers(id),
    verify_token VARCHAR(50) NOT NULL,
    dns_verified BOOLEAN DEFAULT FALSE,
    ssl_enabled BOOLEAN DEFAULT FALSE,
    is_shared BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    
    -- Setup tracking
    setup_type VARCHAR(20) DEFAULT 'external',
    setup_step VARCHAR(30) DEFAULT 'pending',
    
    -- Source tracking
    source VARCHAR(20) DEFAULT 'manual',  -- manual, namesilo, cpanel
    provider_domain_id VARCHAR(100),
    dns_auto_configured BOOLEAN DEFAULT FALSE,
    
    -- cPanel integration
    cpanel_connection_id VARCHAR(30) REFERENCES cpanel_connections(id),
    cpanel_auto_dns BOOLEAN DEFAULT FALSE,
    
    UNIQUE(user_id, name)
);
```

### cpanel_connections table

```sql
CREATE TABLE cpanel_connections (
    id VARCHAR(30) PRIMARY KEY,
    user_id VARCHAR(24) NOT NULL REFERENCES users(id),
    name VARCHAR(100) NOT NULL,
    host VARCHAR(255) NOT NULL,
    username VARCHAR(100) NOT NULL,
    api_token_encrypted TEXT NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    last_used_at TIMESTAMP,
    last_error TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

### users table (NameSilo fields)

```sql
ALTER TABLE users ADD COLUMN namesilo_api_key_encrypted TEXT;
ALTER TABLE users ADD COLUMN namesilo_connected_at TIMESTAMP;
```

---

## Integration Example

To integrate domain addition into another system:

### 1. Manual Mode Integration

```go
import (
    "github.com/botginx/botginx/modules/domains/models"
    "github.com/botginx/botginx/modules/domains/services"
)

// Initialize service
domainService := services.NewDomainService(db)

// Create domain
domain, err := domainService.Create(userID, models.CreateDomainInput{
    Name: "example.com",
})
if err != nil {
    if err == services.ErrDomainExists {
        // Handle duplicate
    }
    return err
}

// domain.VerifyToken contains the TXT record value
// User must add: _guardbot-verify.example.com TXT <verifyToken>
```

### 2. cPanel Mode Integration

```go
import (
    "github.com/botginx/botginx/modules/domains/services"
    "github.com/botginx/botginx/pkg/cpanel"
)

// Initialize services
cpanelService := services.NewCpanelService(db, encryptionKey)

// Create connection
conn, err := cpanelService.Create(userID, models.CreateCpanelConnectionInput{
    Name:     "My Hosting",
    Host:     "server.host.com:2083",
    Username: "cpuser",
    APIToken: "TOKEN123",
})
if err != nil {
    return err
}

// List domains from cPanel
domains, err := cpanelService.ListDomains(userID, conn.ID)

// Import a domain with auto-DNS
domainService.Create(userID, models.CreateDomainInput{
    Name:               "example.com",
    CpanelConnectionID: &conn.ID,
})

// Auto-DNS is triggered automatically when CpanelConnectionID is set
```

### 3. NameSilo Mode Integration

```go
import (
    "github.com/botginx/botginx/modules/domains/services"
)

// Initialize service
namesiloService := services.NewNameSiloService(db, domainService, encryptionKey)

// Connect API key
err := namesiloService.Connect(ctx, userID, "namesilo-api-key")
if err != nil {
    return err
}

// List available domains
domains, err := namesiloService.ListDomains(ctx, userID)

// Import with auto-DNS
domainID, err := namesiloService.ImportDomain(ctx, userID, models.NameSiloImportInput{
    Domain:       "example.com",
    ConfigureDNS: true,
})

// SSL can be generated after DNS propagates
err = namesiloService.GenerateSSL(ctx, userID, "example.com")
```

---

## File Reference Index

| Component | File Path |
|-----------|-----------|
| **Models** | |
| Domain model | `/home/this/dev/code/botginx/modules/domains/models/domain.go` |
| cPanel models | `/home/this/dev/code/botginx/modules/domains/models/cpanel_connection.go` |
| NameSilo models | `/home/this/dev/code/botginx/modules/domains/models/namesilo.go` |
| **Services** | |
| Domain service | `/home/this/dev/code/botginx/modules/domains/services/domain_service.go` |
| cPanel service | `/home/this/dev/code/botginx/modules/domains/services/cpanel_service.go` |
| NameSilo service | `/home/this/dev/code/botginx/modules/domains/services/namesilo_service.go` |
| **Handlers** | |
| Domain handlers | `/home/this/dev/code/botginx/modules/domains/handlers/handler.go` |
| cPanel handlers | `/home/this/dev/code/botginx/modules/domains/handlers/cpanel_handlers.go` |
| NameSilo handlers | `/home/this/dev/code/botginx/modules/domains/handlers/namesilo_handlers.go` |
| **External Clients** | |
| cPanel client | `/home/this/dev/code/botginx/pkg/cpanel/client.go` |
| cPanel DNS | `/home/this/dev/code/botginx/pkg/cpanel/dns.go` |
| **Templates** | |
| Manual add form | `/home/this/dev/code/botginx/modules/domains/templates/new.html` |
| DNS setup wizard | `/home/this/dev/code/botginx/modules/domains/templates/external_setup.html` |
| cPanel list | `/home/this/dev/code/botginx/modules/domains/templates/cpanel_list.html` |
| cPanel connect | `/home/this/dev/code/botginx/modules/domains/templates/cpanel_connect.html` |
| cPanel domains | `/home/this/dev/code/botginx/modules/domains/templates/cpanel_domains.html` |
| NameSilo page | `/home/this/dev/code/botginx/modules/domains/templates/namesilo.html` |
| **Routes** | |
| Module routes | `/home/this/dev/code/botginx/modules/domains/module.go:85-160` |

---

## Status Constants

**File:** `/home/this/dev/code/botginx/pkg/status/status.go`

```go
// Setup types
const (
    SetupTypeDirect   = "direct"   // Direct server integration
    SetupTypeExternal = "external" // User configures DNS externally
)

// Setup steps
const (
    SetupStepPending       = "pending"        // Initial state
    SetupStepDNSWaiting    = "dns_waiting"    // Waiting for DNS verification
    SetupStepSSLWaiting    = "ssl_waiting"    // DNS verified, waiting for SSL
    SetupStepSSLGenerating = "ssl_generating" // SSL generation in progress
    SetupStepComplete      = "complete"       // Fully configured
)

// Domain sources
const (
    SourceManual   = "manual"
    SourceNameSilo = "namesilo"
    SourceCpanel   = "cpanel"
)
```

---

## Error Handling

All handlers return JSON errors in this format:

```json
{
    "error": "Error message here"
}
```

**Common errors:**

| Code | Error | Cause |
|------|-------|-------|
| 400 | "Domain name is required" | Empty name in request |
| 400 | "Invalid request body" | Malformed JSON |
| 401 | "Invalid cPanel credentials" | Wrong username/token |
| 404 | "Domain not found" | Invalid domain ID |
| 409 | "Domain already registered" | Duplicate domain name |
| 502 | "Unable to connect to cPanel server" | Network/firewall issue |
