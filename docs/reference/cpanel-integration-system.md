# cPanel Integration System — Complete Reference

This document provides 100% complete documentation for porting the cPanel API integration system to another platform. Every code example is copied verbatim from the working implementation with absolute file paths.

**Target Audience:** AI agents or developers recreating this system.

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Reference Files](#reference-files)
3. [Database Schema](#database-schema)
4. [Data Models](#data-models)
5. [cPanel API Client](#cpanel-api-client)
6. [DNS Operations (API2)](#dns-operations-api2)
7. [High-Level Operations](#high-level-operations)
8. [Service Layer](#service-layer)
9. [HTTP API Routes](#http-api-routes)
10. [Error Handling](#error-handling)
11. [Proxy Integration](#proxy-integration)
12. [Porting Checklist](#porting-checklist)

---

## Architecture Overview

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│   botginx       │────▶│   cPanel API     │────▶│   DNS Zone      │
│   (this system) │     │   (UAPI/API2)    │     │   (example.com) │
└─────────────────┘     └──────────────────┘     └─────────────────┘
        │                        │
        │  Uses proxy if        │
        │  configured           │
        ▼                        │
┌─────────────────┐              │
│  Rotating Proxy │──────────────┘
│  (optional)     │
└─────────────────┘
```

### Key Design Decisions

1. **Two API versions**: UAPI (newer) for domain listing, API2 (legacy) for DNS zone editing
2. **Credential encryption**: API tokens encrypted with AES-256-GCM before storage
3. **Proxy support**: Optional residential proxy for geo-blocked cPanel servers
4. **Idempotent operations**: All DNS operations check existing records before adding/updating
5. **Line-based deletion**: cPanel DNS records are identified by line number in the zone file

---

## Reference Files

| Absolute Path | Purpose |
|--------------|---------|
| `/home/this/dev/code/botginx/pkg/cpanel/client.go` | HTTP client, authentication, domain listing |
| `/home/this/dev/code/botginx/pkg/cpanel/dns.go` | All DNS record operations (A, TXT, CNAME) |
| `/home/this/dev/code/botginx/pkg/cpanel/types.go` | Data structures (ZoneRecord, DomainInfo, etc.) |
| `/home/this/dev/code/botginx/pkg/cpanel/errors.go` | Error types and sentinel errors |
| `/home/this/dev/code/botginx/modules/domains/services/cpanel_service.go` | Business logic, encryption, auto-DNS |
| `/home/this/dev/code/botginx/modules/domains/handlers/cpanel_handlers.go` | HTTP API handlers |
| `/home/this/dev/code/botginx/modules/domains/models/cpanel_connection.go` | Database models |
| `/home/this/dev/code/botginx/modules/domains/models/domain.go` | Domain model with cPanel fields |
| `/home/this/dev/code/botginx/modules/domains/migrations/010_cpanel_connections.sql` | Database schema |
| `/home/this/dev/code/botginx/modules/domains/module.go` | Route definitions, module setup |
| `/home/this/dev/code/botginx/pkg/proxy/proxy.go` | Rotating proxy for geo-blocked cPanel servers |

---

## Database Schema

**File:** `/home/this/dev/code/botginx/modules/domains/migrations/010_cpanel_connections.sql`

```sql
-- cPanel connections for auto-DNS management
-- Each user can have multiple cPanel connections (different hosting accounts)

CREATE TABLE IF NOT EXISTS cpanel_connections (
    id VARCHAR(50) PRIMARY KEY,
    user_id VARCHAR(26) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,                     -- Display name (e.g., "My Hosting Account")
    host VARCHAR(255) NOT NULL,                     -- cPanel host:port (e.g., "example.com:2083")
    username VARCHAR(255) NOT NULL,                 -- cPanel username
    api_token_encrypted TEXT NOT NULL,              -- API token (encrypted)
    is_active BOOLEAN DEFAULT true,
    last_used_at TIMESTAMP,
    last_error TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    UNIQUE(user_id, host, username)
);

CREATE INDEX IF NOT EXISTS idx_cpanel_connections_user ON cpanel_connections(user_id);
CREATE INDEX IF NOT EXISTS idx_cpanel_connections_active ON cpanel_connections(user_id, is_active);

-- Add cpanel_connection_id to domains table
ALTER TABLE domains ADD COLUMN IF NOT EXISTS cpanel_connection_id VARCHAR(50) REFERENCES cpanel_connections(id) ON DELETE SET NULL;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS cpanel_auto_dns BOOLEAN DEFAULT false;

-- Index for finding domains by cpanel connection
CREATE INDEX IF NOT EXISTS idx_domains_cpanel ON domains(cpanel_connection_id) WHERE cpanel_connection_id IS NOT NULL;
```

**Key design decisions:**
- `api_token_encrypted`: API tokens are encrypted with AES-256-GCM before storage
- `ON DELETE SET NULL`: Deleting a cPanel connection doesn't delete domains, just unlinks them
- `UNIQUE(user_id, host, username)`: Prevents duplicate connections to the same cPanel account

---

## Data Models

### CpanelConnection

**File:** `/home/this/dev/code/botginx/modules/domains/models/cpanel_connection.go`

```go
// CpanelConnection represents a user's cPanel account credentials
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

	// Decrypted token (not stored, populated at runtime)
	APIToken string `db:"-" json:"-"`

	// Stats (populated by joins)
	DomainCount int `db:"domain_count" json:"domainCount,omitempty"`
}

// CreateCpanelConnectionInput is the input for creating a cPanel connection
type CreateCpanelConnectionInput struct {
	Name     string `json:"name" validate:"required"`
	Host     string `json:"host" validate:"required"`
	Username string `json:"username" validate:"required"`
	APIToken string `json:"apiToken" validate:"required"`
}

// UpdateCpanelConnectionInput is the input for updating a cPanel connection
type UpdateCpanelConnectionInput struct {
	Name     *string `json:"name"`
	Host     *string `json:"host"`
	Username *string `json:"username"`
	APIToken *string `json:"apiToken"` // empty = keep current
	IsActive *bool   `json:"isActive"`
}
```

### DNS Record Types

**File:** `/home/this/dev/code/botginx/pkg/cpanel/types.go`

```go
// ZoneRecord represents a DNS record in a zone
type ZoneRecord struct {
	Line     int    `json:"line,omitempty"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	TTL      int    `json:"ttl,omitempty"`
	Class    string `json:"class,omitempty"`
	Address  string `json:"address,omitempty"`  // A record
	CName    string `json:"cname,omitempty"`    // CNAME record
	TXTData  string `json:"txtdata,omitempty"`  // TXT record
	Exchange string `json:"exchange,omitempty"` // MX record
	Priority int    `json:"preference,omitempty"`
}

// TXTRecord is a simplified TXT record
type TXTRecord struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Line  int    `json:"line,omitempty"`
}

// ARecord is a simplified A record
type ARecord struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Line    int    `json:"line,omitempty"`
}

// CNAMERecord is a simplified CNAME record
type CNAMERecord struct {
	Name   string `json:"name"`
	Target string `json:"target"`
	Line   int    `json:"line,omitempty"`
}
```

**Critical:** The `Line` field is required for deletion. cPanel's API deletes records by line number, not by name/value.

### Domain Model (cPanel Fields)

**File:** `/home/this/dev/code/botginx/modules/domains/models/domain.go` (lines 71-74)

```go
// cPanel auto-DNS fields
CpanelConnectionID *string `db:"cpanel_connection_id" json:"cpanelConnectionId,omitempty"`
CpanelAutoDNS      bool    `db:"cpanel_auto_dns" json:"cpanelAutoDns"`
```

---

## cPanel API Client

### Client Configuration

**File:** `/home/this/dev/code/botginx/pkg/cpanel/types.go` (lines 78-93)

```go
// Config holds cPanel connection settings
type Config struct {
	Host         string // hostname:port (e.g., "example.com:2083")
	Username     string
	APIToken     string
	Timeout      int  // seconds, default 30
	SkipTLS      bool // allow self-signed certs
	UseProxy     bool // use proxy if configured
	ProxyService ProxyTransportProvider
}

// ProxyTransportProvider gets an HTTP transport with optional proxy
type ProxyTransportProvider interface {
	GetTransport(skipTLS bool) (*http.Transport, error)
	IsEnabled() bool
}
```

### Client Initialization

**File:** `/home/this/dev/code/botginx/pkg/cpanel/client.go` (lines 20-79)

```go
// NewClient creates a new cPanel API client
func NewClient(config Config) *Client {
	if config.Timeout == 0 {
		config.Timeout = 30
	}

	var transport *http.Transport

	// Try to get proxy transport if proxy is enabled
	if config.UseProxy && config.ProxyService != nil && config.ProxyService.IsEnabled() {
		var err error
		transport, err = config.ProxyService.GetTransport(config.SkipTLS)
		if err != nil {
			// Fallback to direct connection if proxy fails
			transport = &http.Transport{}
			if config.SkipTLS {
				transport.TLSClientConfig = &tls.Config{
					InsecureSkipVerify: true,
				}
			}
		}
	} else {
		transport = &http.Transport{}
		if config.SkipTLS {
			transport.TLSClientConfig = &tls.Config{
				InsecureSkipVerify: true,
			}
		}
	}

	return &Client{
		config: config,
		http: &http.Client{
			Timeout:   time.Duration(config.Timeout) * time.Second,
			Transport: transport,
		},
	}
}

// NewClientSimple creates a client with minimal config (no proxy)
func NewClientSimple(host, username, apiToken string) *Client {
	return NewClient(Config{
		Host:     host,
		Username: username,
		APIToken: apiToken,
		SkipTLS:  true, // cPanel often uses self-signed certs
	})
}

// NewClientWithProxy creates a client that uses proxy if enabled
func NewClientWithProxy(host, username, apiToken string, proxyService ProxyTransportProvider) *Client {
	return NewClient(Config{
		Host:         host,
		Username:     username,
		APIToken:     apiToken,
		SkipTLS:      true,
		UseProxy:     true,
		ProxyService: proxyService,
	})
}
```

### Authentication

**File:** `/home/this/dev/code/botginx/pkg/cpanel/client.go` (lines 82-96)

cPanel uses token-based authentication via the `Authorization` header:

```go
// baseURL returns the cPanel API base URL
func (c *Client) baseURL() string {
	host := c.config.Host
	if !strings.Contains(host, ":") {
		host = host + ":2083"
	}
	if !strings.HasPrefix(host, "https://") {
		host = "https://" + host
	}
	return host
}

// authHeader returns the Authorization header value
func (c *Client) authHeader() string {
	return fmt.Sprintf("cpanel %s:%s", c.config.Username, c.config.APIToken)
}
```

**Important:** The auth header format is `cpanel USERNAME:TOKEN` (not Basic auth).

### HTTP Request Handling

**File:** `/home/this/dev/code/botginx/pkg/cpanel/client.go` (lines 98-142)

```go
// doRequest performs an authenticated HTTP request
func (c *Client) doRequest(method, endpoint string, params url.Values) ([]byte, error) {
	var reqURL string
	var body io.Reader

	if method == http.MethodGet {
		reqURL = fmt.Sprintf("%s%s?%s", c.baseURL(), endpoint, params.Encode())
	} else {
		reqURL = fmt.Sprintf("%s%s", c.baseURL(), endpoint)
		body = strings.NewReader(params.Encode())
	}

	req, err := http.NewRequest(method, reqURL, body)
	if err != nil {
		return nil, NewAPIError("request", "failed to create request", err)
	}

	req.Header.Set("Authorization", c.authHeader())
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, NewAPIError("connection", "failed to connect to cPanel", ErrConnectionFailed)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, NewAPIError("response", "failed to read response", err)
	}

	// Check HTTP status
	switch resp.StatusCode {
	case http.StatusOK:
		return respBody, nil
	case http.StatusUnauthorized:
		return nil, NewAPIError("auth", "invalid credentials", ErrUnauthorized)
	case http.StatusForbidden:
		return nil, NewAPIError("auth", "insufficient permissions", ErrForbidden)
	default:
		return nil, NewAPIError("http", fmt.Sprintf("unexpected status %d: %s", resp.StatusCode, string(respBody)), nil)
	}
}
```

### Domain Listing (UAPI)

**File:** `/home/this/dev/code/botginx/pkg/cpanel/client.go` (lines 144-196)

```go
// callUAPI calls a UAPI endpoint
func (c *Client) callUAPI(module, function string, params url.Values) ([]byte, error) {
	endpoint := fmt.Sprintf("/execute/%s/%s", module, function)
	return c.doRequest(http.MethodGet, endpoint, params)
}

// GetDomains returns all domains on this cPanel account
func (c *Client) GetDomains() (*DomainInfo, error) {
	body, err := c.callUAPI("DomainInfo", "list_domains", nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Status   int      `json:"status"`
		Errors   []string `json:"errors,omitempty"`
		Messages []string `json:"messages,omitempty"`
		Data     struct {
			MainDomain    string   `json:"main_domain"`
			AddonDomains  []string `json:"addon_domains"`
			SubDomains    []string `json:"sub_domains"`
			ParkedDomains []string `json:"parked_domains"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, NewAPIError("parse", "failed to parse domains response", err)
	}

	if resp.Status != 1 {
		errMsg := "unknown error"
		if len(resp.Errors) > 0 {
			errMsg = strings.Join(resp.Errors, "; ")
		}
		return nil, NewAPIError("list_domains", errMsg, nil)
	}

	return &DomainInfo{
		MainDomain:    resp.Data.MainDomain,
		AddonDomains:  resp.Data.AddonDomains,
		SubDomains:    resp.Data.SubDomains,
		ParkedDomains: resp.Data.ParkedDomains,
	}, nil
}
```

### Base Domain Resolution

**File:** `/home/this/dev/code/botginx/pkg/cpanel/client.go` (lines 240-265)

When adding DNS records, you need the base domain (DNS zone), not the subdomain:

```go
// GetBaseDomain finds the base domain for a given hostname
// e.g., "promo.example.com" -> "example.com"
func (c *Client) GetBaseDomain(hostname string) (string, error) {
	domains, err := c.AllDomains()
	if err != nil {
		return "", err
	}

	hostname = strings.ToLower(strings.TrimSpace(hostname))

	// Check if hostname is exactly a known domain
	for _, d := range domains {
		if strings.ToLower(d) == hostname {
			return d, nil
		}
	}

	// Check if hostname is a subdomain of a known domain
	for _, d := range domains {
		if strings.HasSuffix(hostname, "."+strings.ToLower(d)) {
			return d, nil
		}
	}

	return "", ErrDomainNotFound
}
```

---

## DNS Operations (API2)

### Fetching Zone Records

**File:** `/home/this/dev/code/botginx/pkg/cpanel/dns.go` (lines 12-101)

```go
// GetZone fetches all DNS records for a domain
func (c *Client) GetZone(domain string) ([]ZoneRecord, error) {
	params := url.Values{}
	params.Set("domain", domain)

	body, err := c.doRequest(http.MethodGet, "/json-api/cpanel", params)
	if err != nil {
		return nil, err
	}

	// Parse the nested response structure
	var resp struct {
		CPanelResult struct {
			Data []struct {
				StatusMsg string                   `json:"statusmsg"`
				Record    []map[string]interface{} `json:"record"`
			} `json:"data"`
			Error string `json:"error,omitempty"`
		} `json:"cpanelresult"`
	}

	// Add API2 params for this specific call
	params.Set("cpanel_jsonapi_apiversion", "2")
	params.Set("cpanel_jsonapi_module", "ZoneEdit")
	params.Set("cpanel_jsonapi_func", "fetchzone")

	body, err = c.doRequest(http.MethodGet, "/json-api/cpanel", params)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, NewAPIError("parse", "failed to parse zone response", err)
	}

	if resp.CPanelResult.Error != "" {
		return nil, NewAPIError("fetchzone", resp.CPanelResult.Error, nil)
	}

	if len(resp.CPanelResult.Data) == 0 || len(resp.CPanelResult.Data[0].Record) == 0 {
		return nil, NewAPIError("fetchzone", "no DNS records found", nil)
	}

	// Records are in data[0].record
	rawRecords := resp.CPanelResult.Data[0].Record
	records := make([]ZoneRecord, 0, len(rawRecords))

	for _, item := range rawRecords {
		record := ZoneRecord{}

		if v, ok := item["line"].(float64); ok {
			record.Line = int(v)
		}
		if v, ok := item["name"].(string); ok {
			record.Name = strings.TrimSuffix(v, ".")
		}
		if v, ok := item["type"].(string); ok {
			record.Type = v
		}
		if v, ok := item["ttl"].(float64); ok {
			record.TTL = int(v)
		}
		if v, ok := item["class"].(string); ok {
			record.Class = v
		}
		if v, ok := item["address"].(string); ok {
			record.Address = v
		}
		if v, ok := item["cname"].(string); ok {
			record.CName = strings.TrimSuffix(v, ".")
		}
		if v, ok := item["txtdata"].(string); ok {
			record.TXTData = v
		}
		if v, ok := item["exchange"].(string); ok {
			record.Exchange = v
		}
		if v, ok := item["preference"].(float64); ok {
			record.Priority = int(v)
		}

		// Skip raw/comment lines
		if record.Type == ":RAW" || record.Type == "$TTL" || record.Type == "" {
			continue
		}

		records = append(records, record)
	}

	return records, nil
}
```

**Important:** Record names from cPanel end with a trailing dot (e.g., `example.com.`). Strip it with `strings.TrimSuffix(v, ".")`.

### Finding Records

**File:** `/home/this/dev/code/botginx/pkg/cpanel/dns.go` (lines 145-229)

```go
// FindARecord finds an A record by name
// e.g., FindARecord("example.com", "*") finds "*.example.com"
func (c *Client) FindARecord(domain, name string) (*ARecord, error) {
	records, err := c.GetARecords(domain)
	if err != nil {
		return nil, err
	}

	// Build the full name we're looking for
	var fullName string
	if name == "*" {
		fullName = "*." + strings.ToLower(domain)
	} else if name == "" || name == "@" {
		fullName = strings.ToLower(domain)
	} else {
		fullName = strings.ToLower(name + "." + domain)
	}

	for _, r := range records {
		if strings.ToLower(r.Name) == fullName {
			return &r, nil
		}
	}

	return nil, nil // Not found, but not an error
}

// FindTXTRecord finds a TXT record by name prefix
// e.g., FindTXTRecord("example.com", "_guardbot-verify") finds "_guardbot-verify.example.com"
func (c *Client) FindTXTRecord(domain, namePrefix string) (*TXTRecord, error) {
	records, err := c.GetTXTRecords(domain)
	if err != nil {
		return nil, err
	}

	// Build the full name we're looking for
	fullName := strings.ToLower(namePrefix + "." + domain)

	for _, r := range records {
		if strings.ToLower(r.Name) == fullName {
			return &r, nil
		}
	}

	return nil, nil // Not found, but not an error
}

// FindCNAMERecord finds a CNAME record by name
func (c *Client) FindCNAMERecord(domain, name string) (*CNAMERecord, error) {
	records, err := c.GetCNAMERecords(domain)
	if err != nil {
		return nil, err
	}

	fullName := strings.ToLower(name + "." + domain)

	for _, r := range records {
		if strings.ToLower(r.Name) == fullName {
			return &r, nil
		}
	}

	return nil, nil
}
```

### Adding Records

**File:** `/home/this/dev/code/botginx/pkg/cpanel/dns.go` (lines 231-297, 358-404, 491-537)

```go
// AddTXTRecord adds a TXT record to the domain's DNS zone
func (c *Client) AddTXTRecord(domain, name, value string) error {
	// First check if record already exists
	existing, err := c.FindTXTRecord(domain, name)
	if err != nil {
		return err
	}
	if existing != nil {
		if existing.Value == value {
			return nil // Already exists with same value
		}
		return fmt.Errorf("%w: %s.%s already exists with value %q", ErrRecordExists, name, domain, existing.Value)
	}

	params := url.Values{}
	params.Set("cpanel_jsonapi_apiversion", "2")
	params.Set("cpanel_jsonapi_module", "ZoneEdit")
	params.Set("cpanel_jsonapi_func", "add_zone_record")
	params.Set("domain", domain)
	params.Set("name", name)
	params.Set("type", "TXT")
	params.Set("txtdata", value)
	params.Set("ttl", "300") // 5 minute TTL for quick propagation

	body, err := c.doRequest(http.MethodGet, "/json-api/cpanel", params)
	if err != nil {
		return err
	}

	// Parse response
	var resp struct {
		CPanelResult struct {
			Data []struct {
				Result struct {
					Status    int         `json:"status"`
					StatusMsg string      `json:"statusmsg"`
					NewSerial interface{} `json:"newserial"`
				} `json:"result"`
			} `json:"data"`
			Error string `json:"error,omitempty"`
		} `json:"cpanelresult"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return NewAPIError("parse", "failed to parse add_zone_record response", err)
	}

	if resp.CPanelResult.Error != "" {
		return NewAPIError("add_zone_record", resp.CPanelResult.Error, ErrZoneEditFailed)
	}

	if len(resp.CPanelResult.Data) == 0 {
		return NewAPIError("add_zone_record", "no response data", ErrZoneEditFailed)
	}

	result := resp.CPanelResult.Data[0].Result
	if result.Status != 1 {
		errMsg := result.StatusMsg
		if errMsg == "" {
			errMsg = "failed to add TXT record"
		}
		return NewAPIError("add_zone_record", errMsg, ErrZoneEditFailed)
	}

	return nil
}

// AddARecord adds an A record to the domain's DNS zone
func (c *Client) AddARecord(domain, name, ip string) error {
	params := url.Values{}
	params.Set("cpanel_jsonapi_apiversion", "2")
	params.Set("cpanel_jsonapi_module", "ZoneEdit")
	params.Set("cpanel_jsonapi_func", "add_zone_record")
	params.Set("domain", domain)
	params.Set("name", name)
	params.Set("type", "A")
	params.Set("address", ip)
	params.Set("ttl", "300")

	body, err := c.doRequest(http.MethodGet, "/json-api/cpanel", params)
	if err != nil {
		return err
	}

	var resp struct {
		CPanelResult struct {
			Data []struct {
				Result struct {
					Status    int    `json:"status"`
					StatusMsg string `json:"statusmsg"`
				} `json:"result"`
			} `json:"data"`
			Error string `json:"error,omitempty"`
		} `json:"cpanelresult"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return NewAPIError("parse", "failed to parse add_zone_record response", err)
	}

	if resp.CPanelResult.Error != "" {
		return NewAPIError("add_zone_record", resp.CPanelResult.Error, ErrZoneEditFailed)
	}

	if len(resp.CPanelResult.Data) > 0 && resp.CPanelResult.Data[0].Result.Status != 1 {
		errMsg := resp.CPanelResult.Data[0].Result.StatusMsg
		if errMsg == "" {
			errMsg = "failed to add A record"
		}
		return NewAPIError("add_zone_record", errMsg, ErrZoneEditFailed)
	}

	return nil
}

// AddCNAMERecord adds a CNAME record to the domain's DNS zone
func (c *Client) AddCNAMERecord(domain, name, target string) error {
	params := url.Values{}
	params.Set("cpanel_jsonapi_apiversion", "2")
	params.Set("cpanel_jsonapi_module", "ZoneEdit")
	params.Set("cpanel_jsonapi_func", "add_zone_record")
	params.Set("domain", domain)
	params.Set("name", name)
	params.Set("type", "CNAME")
	params.Set("cname", target)
	params.Set("ttl", "300")

	body, err := c.doRequest(http.MethodGet, "/json-api/cpanel", params)
	if err != nil {
		return err
	}

	var resp struct {
		CPanelResult struct {
			Data []struct {
				Result struct {
					Status    int    `json:"status"`
					StatusMsg string `json:"statusmsg"`
				} `json:"result"`
			} `json:"data"`
			Error string `json:"error,omitempty"`
		} `json:"cpanelresult"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return NewAPIError("parse", "failed to parse add_zone_record response", err)
	}

	if resp.CPanelResult.Error != "" {
		return NewAPIError("add_zone_record", resp.CPanelResult.Error, ErrZoneEditFailed)
	}

	if len(resp.CPanelResult.Data) > 0 && resp.CPanelResult.Data[0].Result.Status != 1 {
		errMsg := resp.CPanelResult.Data[0].Result.StatusMsg
		if errMsg == "" {
			errMsg = "failed to add CNAME record"
		}
		return NewAPIError("add_zone_record", errMsg, ErrZoneEditFailed)
	}

	return nil
}
```

### Removing Records by Line Number

**File:** `/home/this/dev/code/botginx/pkg/cpanel/dns.go` (lines 299-356)

**Critical:** cPanel deletes records by line number, not by name/value. You must first find the record to get its line number.

```go
// RemoveTXTRecord removes a TXT record from the domain's DNS zone
func (c *Client) RemoveTXTRecord(domain, name string) error {
	// Find the record to get its line number
	record, err := c.FindTXTRecord(domain, name)
	if err != nil {
		return err
	}
	if record == nil {
		return nil // Already doesn't exist
	}

	return c.removeRecordByLine(domain, record.Line)
}

// removeRecordByLine removes a DNS record by its line number
func (c *Client) removeRecordByLine(domain string, line int) error {
	params := url.Values{}
	params.Set("cpanel_jsonapi_apiversion", "2")
	params.Set("cpanel_jsonapi_module", "ZoneEdit")
	params.Set("cpanel_jsonapi_func", "remove_zone_record")
	params.Set("domain", domain)
	params.Set("line", fmt.Sprintf("%d", line))

	body, err := c.doRequest(http.MethodGet, "/json-api/cpanel", params)
	if err != nil {
		return err
	}

	var resp struct {
		CPanelResult struct {
			Data []struct {
				Result struct {
					Status    int    `json:"status"`
					StatusMsg string `json:"statusmsg"`
				} `json:"result"`
			} `json:"data"`
			Error string `json:"error,omitempty"`
		} `json:"cpanelresult"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return NewAPIError("parse", "failed to parse remove_zone_record response", err)
	}

	if resp.CPanelResult.Error != "" {
		return NewAPIError("remove_zone_record", resp.CPanelResult.Error, ErrZoneEditFailed)
	}

	if len(resp.CPanelResult.Data) > 0 && resp.CPanelResult.Data[0].Result.Status != 1 {
		errMsg := resp.CPanelResult.Data[0].Result.StatusMsg
		if errMsg == "" {
			errMsg = "failed to remove DNS record"
		}
		return NewAPIError("remove_zone_record", errMsg, ErrZoneEditFailed)
	}

	return nil
}
```

### Update-or-Add Pattern

**File:** `/home/this/dev/code/botginx/pkg/cpanel/dns.go` (lines 406-446, 539-573)

The update-or-add pattern: find existing → delete if different → add new.

```go
// UpdateOrAddARecord updates an A record if it exists with different IP, or adds it if not
func (c *Client) UpdateOrAddARecord(domain, name, ip string) error {
	existing, err := c.FindARecord(domain, name)
	if err != nil {
		return err
	}

	if existing != nil {
		if existing.Address == ip {
			return nil // Already correct
		}
		// Remove old record first
		if err := c.removeRecordByLine(domain, existing.Line); err != nil {
			return fmt.Errorf("failed to remove old A record: %w", err)
		}
	}

	// Add new record
	return c.AddARecord(domain, name, ip)
}

// UpdateOrAddCNAMERecord updates a CNAME record if it exists with different target, or adds it if not
// IMPORTANT: CNAME cannot coexist with any other record type at the same hostname.
// This function removes conflicting TXT/A records before adding the CNAME.
func (c *Client) UpdateOrAddCNAMERecord(domain, name, target string) error {
	// First, remove any conflicting TXT record at this hostname
	// (DNS rule: CNAME cannot coexist with other record types)
	existingTXT, err := c.FindTXTRecord(domain, name)
	if err != nil {
		return err
	}
	if existingTXT != nil {
		if err := c.removeRecordByLine(domain, existingTXT.Line); err != nil {
			return fmt.Errorf("failed to remove conflicting TXT record: %w", err)
		}
	}

	// Check for existing CNAME
	existing, err := c.FindCNAMERecord(domain, name)
	if err != nil {
		return err
	}

	if existing != nil {
		if existing.Target == target {
			return nil // Already correct
		}
		// Remove old record first
		if err := c.removeRecordByLine(domain, existing.Line); err != nil {
			return fmt.Errorf("failed to remove old CNAME record: %w", err)
		}
	}

	// Add new record
	return c.AddCNAMERecord(domain, name, target)
}
```

**Critical DNS Rule:** A CNAME record cannot coexist with any other record type at the same hostname. The implementation removes conflicting TXT records before adding a CNAME.

---

## High-Level Operations

### GuardBot-Specific DNS Methods

**File:** `/home/this/dev/code/botginx/pkg/cpanel/dns.go` (lines 448-489)

```go
// AddVerificationTXT adds the GuardBot verification TXT record
// Record name: _guardbot-verify.domain.com
func (c *Client) AddVerificationTXT(domain, token string) error {
	// Get base domain for DNS zone
	baseDomain, err := c.GetBaseDomain(domain)
	if err != nil {
		return err
	}

	return c.UpdateOrAddTXTRecord(baseDomain, "_guardbot-verify", token)
}

// RemoveVerificationTXT removes the GuardBot verification TXT record
func (c *Client) RemoveVerificationTXT(domain string) error {
	baseDomain, err := c.GetBaseDomain(domain)
	if err != nil {
		return err
	}

	return c.RemoveTXTRecord(baseDomain, "_guardbot-verify")
}

// AddAcmeChallengeTXT adds an ACME challenge TXT record for SSL
// Record name: _acme-challenge.domain.com
func (c *Client) AddAcmeChallengeTXT(domain, token string) error {
	baseDomain, err := c.GetBaseDomain(domain)
	if err != nil {
		return err
	}

	return c.UpdateOrAddTXTRecord(baseDomain, "_acme-challenge", token)
}

// RemoveAcmeChallengeTXT removes the ACME challenge TXT record
func (c *Client) RemoveAcmeChallengeTXT(domain string) error {
	baseDomain, err := c.GetBaseDomain(domain)
	if err != nil {
		return err
	}

	return c.RemoveTXTRecord(baseDomain, "_acme-challenge")
}
```

---

## Service Layer

### Service Initialization

**File:** `/home/this/dev/code/botginx/modules/domains/services/cpanel_service.go` (lines 30-46)

```go
// NewCpanelService creates a new cPanel service
func NewCpanelService(db *sqlx.DB) *CpanelService {
	// Get encryption key from env (same as session secret)
	key := os.Getenv("SESSION_SECRET")
	if len(key) < 32 {
		key = key + "00000000000000000000000000000000" // Pad if too short
	}

	return &CpanelService{
		db:            db,
		encryptionKey: []byte(key[:32]),
	}
}

// SetProxyService sets the proxy service for cPanel API calls
func (s *CpanelService) SetProxyService(ps *proxy.Service) {
	s.proxyService = ps
}
```

### Token Encryption (AES-256-GCM)

**File:** `/home/this/dev/code/botginx/modules/domains/services/cpanel_service.go` (lines 67-115)

```go
// encrypt encrypts a string using AES-GCM
func (s *CpanelService) encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt decrypts a string using AES-GCM
func (s *CpanelService) decrypt(ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	if len(data) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := data[:gcm.NonceSize()], string(data[gcm.NonceSize():])
	plaintext, err := gcm.Open(nil, nonce, []byte(ciphertext), nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
```

### Auto-DNS Trigger

**File:** `/home/this/dev/code/botginx/modules/domains/services/cpanel_service.go` (lines 423-458)

This is called when a domain is added with a cPanel connection:

```go
// TriggerAutoDNS adds all required DNS records for a domain using its cPanel connection:
// 1. Verification TXT record (_guardbot-verify)
// 2. Wildcard A record (*.domain.com → Deploy VPS IP)
// Returns true if all records were added, false if no cPanel connection or error
func (s *CpanelService) TriggerAutoDNS(domainID, domainName, verifyToken, cpanelConnectionID string) bool {
	if cpanelConnectionID == "" {
		return false
	}

	log.Printf("[cpanel] Auto-DNS triggered for domain %s", domainName)

	// 1. Add verification TXT record
	err := s.AddVerificationTXT(cpanelConnectionID, domainName, verifyToken)
	if err != nil {
		log.Printf("[cpanel] Auto-DNS failed for %s (TXT): %v", domainName, err)
		return false
	}
	log.Printf("[cpanel] TXT record added for %s", domainName)

	// 2. Add wildcard A record pointing to Deploy VPS (from admin/servers)
	deployIP := s.getDeployIP()
	if deployIP == "" {
		log.Printf("[cpanel] No deploy server found in admin/servers, skipping wildcard A record for %s", domainName)
		return true // TXT was added, partial success
	}

	err = s.AddWildcardARecord(cpanelConnectionID, domainName, deployIP)
	if err != nil {
		log.Printf("[cpanel] Auto-DNS wildcard A record failed for %s: %v", domainName, err)
		// Continue - TXT was added successfully
	} else {
		log.Printf("[cpanel] Wildcard A record added for *.%s → %s", domainName, deployIP)
	}

	log.Printf("[cpanel] Auto-DNS complete for %s", domainName)
	return true
}
```

### Adding Wildcard A Record

**File:** `/home/this/dev/code/botginx/modules/domains/services/cpanel_service.go` (lines 460-492)

```go
// AddWildcardARecord adds or updates a wildcard A record (*.domain.com) via cPanel
// If wildcard already exists with different IP, it will be updated
func (s *CpanelService) AddWildcardARecord(connectionID, domain, ip string) error {
	conn, err := s.GetByID(connectionID)
	if err != nil {
		return fmt.Errorf("failed to get connection: %w", err)
	}

	client := s.newClient(conn.Host, conn.Username, conn.APIToken)

	log.Printf("[cpanel] Setting wildcard A record *.%s → %s via %s", domain, ip, conn.Host)

	// Get base domain for DNS zone
	baseDomain, err := client.GetBaseDomain(domain)
	if err != nil {
		return fmt.Errorf("failed to get base domain: %w", err)
	}

	// Update or add wildcard A record: name="*", domain=baseDomain
	// This will update existing record if it has different IP
	if err := client.UpdateOrAddARecord(baseDomain, "*", ip); err != nil {
		// Update last_error
		errStr := err.Error()
		s.db.Exec(`UPDATE cpanel_connections SET last_error = $1 WHERE id = $2`, errStr, connectionID)
		return err
	}

	// Update last_used
	now := time.Now()
	s.db.Exec(`UPDATE cpanel_connections SET last_error = NULL, last_used_at = $1 WHERE id = $2`, now, connectionID)

	return nil
}
```

### Adding ACME CNAME for Wildcard SSL

**File:** `/home/this/dev/code/botginx/modules/domains/services/cpanel_service.go` (lines 533-561)

For wildcard SSL certificates, we use acme-dns delegation via CNAME:

```go
// AddAcmeCNAME adds or updates a CNAME record for acme-dns SSL verification
// If CNAME already exists with different target, it will be updated
func (s *CpanelService) AddAcmeCNAME(connectionID, domain, target string) error {
	conn, err := s.GetByID(connectionID)
	if err != nil {
		return fmt.Errorf("failed to get connection: %w", err)
	}

	client := s.newClient(conn.Host, conn.Username, conn.APIToken)

	log.Printf("[cpanel] Setting ACME CNAME _acme-challenge.%s → %s", domain, target)

	baseDomain, err := client.GetBaseDomain(domain)
	if err != nil {
		return fmt.Errorf("failed to get base domain: %w", err)
	}

	// Update or add CNAME for _acme-challenge
	if err := client.UpdateOrAddCNAMERecord(baseDomain, "_acme-challenge", target); err != nil {
		errStr := err.Error()
		s.db.Exec(`UPDATE cpanel_connections SET last_error = $1 WHERE id = $2`, errStr, connectionID)
		return err
	}

	now := time.Now()
	s.db.Exec(`UPDATE cpanel_connections SET last_error = NULL, last_used_at = $1 WHERE id = $2`, now, connectionID)

	return nil
}
```

**ACME CNAME target format:** `{subdomain}.{ACME_DNS_DOMAIN}` (e.g., `abc123.acme.pamach.xyz`)

### Reconfigure All Domains (Server IP Change)

**File:** `/home/this/dev/code/botginx/modules/domains/services/cpanel_service.go` (lines 563-609)

When the deploy server IP changes, update all cPanel domains:

```go
// ReconfigureAllDomains updates wildcard A records for all cPanel domains
// This is called when the deploy server IP changes
func (s *CpanelService) ReconfigureAllDomains(newIP string) ([]ReconfigureResult, error) {
	// Find all domains with cpanel_connection_id set
	type domainInfo struct {
		ID                 string `db:"id"`
		Name               string `db:"name"`
		CpanelConnectionID string `db:"cpanel_connection_id"`
	}

	var domains []domainInfo
	err := s.db.Select(&domains, `
		SELECT id, name, cpanel_connection_id
		FROM domains
		WHERE cpanel_connection_id IS NOT NULL AND cpanel_connection_id != ''
	`)
	if err != nil {
		return nil, fmt.Errorf("querying cPanel domains: %w", err)
	}

	if len(domains) == 0 {
		return []ReconfigureResult{}, nil
	}

	var results []ReconfigureResult

	// Process each domain
	for _, d := range domains {
		err := s.AddWildcardARecord(d.CpanelConnectionID, d.Name, newIP)
		if err != nil {
			results = append(results, ReconfigureResult{
				Domain:  d.Name,
				Success: false,
				Error:   err.Error(),
			})
			log.Printf("[cpanel] ReconfigureAll: failed to update %s: %v", d.Name, err)
		} else {
			results = append(results, ReconfigureResult{
				Domain:  d.Name,
				Success: true,
			})
			log.Printf("[cpanel] ReconfigureAll: updated *.%s → %s", d.Name, newIP)
		}
	}

	return results, nil
}
```

### DNS Cleanup on Domain Deletion

**File:** `/home/this/dev/code/botginx/modules/domains/services/cpanel_service.go` (lines 494-531)

```go
// CleanupDomainDNS removes DNS records we added for a domain via cPanel
// Called when a domain is deleted to clean up our records
func (s *CpanelService) CleanupDomainDNS(connectionID, domain string) {
	conn, err := s.GetByID(connectionID)
	if err != nil {
		log.Printf("[cpanel] cleanup: failed to get connection for %s: %v", domain, err)
		return
	}

	client := s.newClient(conn.Host, conn.Username, conn.APIToken)

	baseDomain, err := client.GetBaseDomain(domain)
	if err != nil {
		log.Printf("[cpanel] cleanup: failed to get base domain for %s: %v", domain, err)
		return
	}

	// Remove verification TXT record
	if err := client.RemoveTXTRecord(baseDomain, "_guardbot-verify"); err != nil {
		log.Printf("[cpanel] cleanup: failed to remove TXT for %s: %v", domain, err)
	} else {
		log.Printf("[cpanel] cleanup: removed _guardbot-verify TXT for %s", domain)
	}

	// Remove ACME CNAME record
	if record, _ := client.FindCNAMERecord(baseDomain, "_acme-challenge"); record != nil {
		if err := client.RemoveCNAMERecord(baseDomain, "_acme-challenge"); err != nil {
			log.Printf("[cpanel] cleanup: failed to remove CNAME for %s: %v", domain, err)
		} else {
			log.Printf("[cpanel] cleanup: removed _acme-challenge CNAME for %s", domain)
		}
	}

	// Note: We don't remove the wildcard A record because:
	// 1. The user might have other domains/subdomains using it
	// 2. They might re-add the domain and want the same setup
	// If needed, user can manually remove it from cPanel
}
```

---

## HTTP API Routes

**File:** `/home/this/dev/code/botginx/modules/domains/module.go` (lines 96-110)

```go
r.Route("/cpanel", func(r chi.Router) {
	// HTML pages
	r.Get("/", m.handler.CpanelList)            // Page: list all connections
	r.Get("/connect", m.handler.CpanelConnect)  // Page: add new connection form
	r.Get("/{id}/domains", m.handler.CpanelDomains) // Page: select domain from connection

	// JSON API
	r.Get("/api", m.cpanelHandler.ListConnections)
	r.Post("/api", m.cpanelHandler.CreateConnection)
	r.Post("/api/test", m.cpanelHandler.TestNewConnection)
	r.Get("/api/{id}", m.cpanelHandler.GetConnection)
	r.Put("/api/{id}", m.cpanelHandler.UpdateConnection)
	r.Delete("/api/{id}", m.handler.APIDeleteCpanelConnection)
	r.Post("/api/{id}/test", m.cpanelHandler.TestConnection)
	r.Get("/api/{id}/domains", m.cpanelHandler.ListDomains)
})
```

### API Endpoints Summary

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/user/domains/cpanel/api` | List user's cPanel connections |
| `POST` | `/user/domains/cpanel/api` | Create new connection (tests before saving) |
| `POST` | `/user/domains/cpanel/api/test` | Test credentials without saving |
| `GET` | `/user/domains/cpanel/api/{id}` | Get single connection |
| `PUT` | `/user/domains/cpanel/api/{id}` | Update connection |
| `DELETE` | `/user/domains/cpanel/api/{id}` | Delete connection |
| `POST` | `/user/domains/cpanel/api/{id}/test` | Test existing connection |
| `GET` | `/user/domains/cpanel/api/{id}/domains` | List domains on cPanel account |

---

## Error Handling

**File:** `/home/this/dev/code/botginx/pkg/cpanel/errors.go`

```go
// Sentinel errors
var (
	ErrConnectionFailed  = errors.New("cannot connect to cPanel server")
	ErrUnauthorized      = errors.New("invalid username or API token")
	ErrForbidden         = errors.New("API token lacks required permissions")
	ErrDomainNotFound    = errors.New("domain not found on this cPanel account")
	ErrRecordExists      = errors.New("DNS record already exists")
	ErrRecordNotFound    = errors.New("DNS record not found")
	ErrInvalidResponse   = errors.New("invalid response from cPanel API")
	ErrZoneEditFailed    = errors.New("DNS zone edit operation failed")
)

// APIError wraps cPanel API errors with context
type APIError struct {
	Operation string
	Message   string
	Code      int
	Err       error
}

func (e *APIError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("cpanel %s: %s (%v)", e.Operation, e.Message, e.Err)
	}
	return fmt.Sprintf("cpanel %s: %s", e.Operation, e.Message)
}

func (e *APIError) Unwrap() error {
	return e.Err
}

// NewAPIError creates a new API error
func NewAPIError(operation, message string, err error) *APIError {
	return &APIError{
		Operation: operation,
		Message:   message,
		Err:       err,
	}
}

// Error classification helpers
func IsConnectionError(err error) bool {
	return errors.Is(err, ErrConnectionFailed)
}

func IsAuthError(err error) bool {
	return errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrForbidden)
}

func IsDomainError(err error) bool {
	return errors.Is(err, ErrDomainNotFound)
}

func IsRecordError(err error) bool {
	return errors.Is(err, ErrRecordExists) || errors.Is(err, ErrRecordNotFound)
}
```

---

## Proxy Integration

**File:** `/home/this/dev/code/botginx/pkg/proxy/proxy.go`

Some cPanel servers are geo-restricted. The proxy service provides rotating residential IPs:

```go
// BuildProxyURL builds the full proxy URL with session
// Format: http://USER-zone-custom-region-US-session-{random}-sessTime-10:password@host:port
func (c *Config) BuildProxyURL() string {
	if c.Username == "" || c.Password == "" || c.Host == "" {
		return ""
	}

	sessionID := generateSessionID()
	// Region must be uppercase (US not us)
	user := fmt.Sprintf("%s-zone-%s-region-US-session-%s-sessTime-%d",
		c.Username, c.Zone, sessionID, c.SessionMinutes)

	return fmt.Sprintf("http://%s:%s@%s:%d", user, c.Password, c.Host, c.Port)
}

// GetTransport returns an HTTP transport configured with proxy if enabled
func (s *Service) GetTransport(skipTLS bool) (*http.Transport, error) {
	config, err := s.Get()
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{}

	if skipTLS {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	if config.Enabled && config.Username != "" {
		proxyURL := config.BuildProxyURL()
		if proxyURL != "" {
			parsedURL, err := url.Parse(proxyURL)
			if err != nil {
				return nil, fmt.Errorf("invalid proxy URL: %w", err)
			}
			transport.Proxy = http.ProxyURL(parsedURL)
		}
	}

	return transport, nil
}
```

Default proxy host (as of 2026-11-01): `global.rotgb.trustsource.live:10000`

---

## DNS Records Reference

The system manages three types of DNS records:

### 1. Verification TXT Record

- **Name:** `_guardbot-verify.{domain}`
- **Value:** Random verification token (stored in `domains.verify_token`)
- **Purpose:** Proves domain ownership before activation
- **TTL:** 300 seconds (5 minutes)

### 2. Wildcard A Record

- **Name:** `*.{domain}`
- **Value:** Deploy server IP address
- **Purpose:** Routes all subdomains to the antibot proxy
- **TTL:** 300 seconds

### 3. ACME Challenge CNAME (for wildcard SSL)

- **Name:** `_acme-challenge.{domain}`
- **Value:** `{subdomain}.{ACME_DNS_DOMAIN}` (e.g., `abc123.acme.pamach.xyz`)
- **Purpose:** Delegates ACME DNS-01 challenges to acme-dns server
- **TTL:** 300 seconds

**Why CNAME not TXT for ACME?** ACME DNS-01 challenges require updating TXT records dynamically. With CNAME delegation, the acme-dns server handles updates automatically, enabling reliable wildcard SSL without per-challenge API calls.

---

## Edge Cases and Gotchas

### 1. Zone File Line Numbers Change After Edits

After adding/removing a record, line numbers shift. Always re-fetch the zone before another deletion.

### 2. cPanel Self-Signed Certificates

Most cPanel installations use self-signed SSL. Always set `SkipTLS: true`.

### 3. Record Name Trailing Dots

cPanel returns record names with trailing dots (`example.com.`). Strip them:
```go
record.Name = strings.TrimSuffix(v, ".")
```

### 4. Wildcard Record Name

Use `*` as the name, cPanel automatically expands to `*.domain.com`.

### 5. CNAME Cannot Coexist with Other Records

Before adding CNAME, remove any TXT/A records at the same hostname.

### 6. TTL for Fast Propagation

Use TTL 300 (5 minutes) for records that need fast updates:
```go
params.Set("ttl", "300")
```

### 7. API Token vs Password

cPanel API tokens are NOT passwords. They're generated in cPanel → Manage API Tokens. They have specific permissions and can be revoked.

### 8. newserial Can Be String or Int

Some cPanel versions return `newserial` as string, others as int. Use `interface{}`:
```go
NewSerial interface{} `json:"newserial,omitempty"`
```

### 9. Base Domain Resolution

For `promo.example.com`, the base domain (DNS zone) is `example.com`. Use `GetBaseDomain()`.

---

## Porting Checklist

To port this system to another platform:

1. **Database**
   - [ ] Create `cpanel_connections` table with schema above
   - [ ] Add `cpanel_connection_id` and `cpanel_auto_dns` columns to domains table
   - [ ] Implement AES-256-GCM encryption for API tokens

2. **cPanel Client**
   - [ ] Implement HTTP client with `cpanel USERNAME:TOKEN` auth header
   - [ ] Implement UAPI call for `DomainInfo::list_domains`
   - [ ] Implement API2 calls for `ZoneEdit::fetchzone`, `add_zone_record`, `remove_zone_record`
   - [ ] Handle trailing dots in DNS record names
   - [ ] Implement line-number-based record deletion

3. **DNS Operations**
   - [ ] Implement GetBaseDomain to find DNS zone for any hostname
   - [ ] Implement Find{A,TXT,CNAME}Record with case-insensitive matching
   - [ ] Implement UpdateOrAdd pattern (find → delete if different → add)
   - [ ] Handle CNAME conflict rule (remove TXT before adding CNAME)

4. **High-Level Features**
   - [ ] TriggerAutoDNS: Add verification TXT + wildcard A on domain creation
   - [ ] AddAcmeCNAME: Add CNAME for wildcard SSL
   - [ ] ReconfigureAllDomains: Update all wildcard A records on server IP change
   - [ ] CleanupDomainDNS: Remove records on domain deletion

5. **API Endpoints**
   - [ ] CRUD for cPanel connections with test-before-save
   - [ ] List domains from cPanel account
   - [ ] User-friendly error messages for common failures

6. **Optional: Proxy Support**
   - [ ] Rotating proxy for geo-restricted cPanel servers
   - [ ] Session-based sticky IPs

---

## Related Documentation

- `/home/this/dev/code/botginx/docs/reference/botection-callback-api.md` — How antibot proxy uses domain settings
- `/home/this/dev/code/botginx/docs/explanation/enforcement-architecture.md` — How bot protection works
