# GoDaddy Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add GoDaddy as a DNS provider following the Cloudflare pattern, enabling users to connect their GoDaddy account, import domains, and auto-configure DNS/SSL/nginx.

**Architecture:** Users connect via API key/secret, list their domains, click Import which triggers background DNS setup (A records to deploy VPS IP), acme-dns CNAME registration, wildcard SSL generation, and nginx config creation. ReconfigureAllDomains updates DNS when deploy VPS changes.

**Tech Stack:** Go, GoDaddy REST API v1, AES-256-GCM encryption, SSH for deploy server operations

## Global Constraints

- DNS A records must point to deploy VPS IP from `domainService.GetDeployIP()`, not panel IP
- Must integrate with `ReconfigureAllDomainsWithIP()` for admin reconfigure button
- GoDaddy API: `Authorization: sso-key {API_KEY}:{API_SECRET}` header
- GoDaddy TTL range: 600-86400 seconds
- Rate limit: 60 requests/minute per credential
- UI must match existing Cloudflare/NameSilo templates exactly

---

## File Structure

### Files to Create

| File | Responsibility |
|------|----------------|
| `pkg/providers/godaddy/client.go` | HTTP client, auth, rate limiting, base request method |
| `pkg/providers/godaddy/domains.go` | ListDomains, GetDomain, VerifyCredentials |
| `pkg/providers/godaddy/dns.go` | ListDNSRecords, AddDNSRecord, UpdateDNSRecord, DeleteDNSRecord, SetupDNS, AddVerificationTXT, AddAcmeDNSCname |
| `modules/domains/models/godaddy.go` | GoDaddyConnection, GoDaddyDomain, GoDaddyImportInput |
| `modules/domains/services/godaddy_service.go` | Connect, Disconnect, ListDomains, Import, ReconfigureAllDomains, setupDNSInternal, setupProxyNginxConfig |
| `modules/domains/handlers/godaddy_handlers.go` | HTTP handlers for connect, disconnect, list, import, reconfigure |
| `modules/domains/templates/godaddy.html` | UI matching Cloudflare pattern |
| `modules/domains/migrations/009_godaddy.sql` | Add godaddy columns to users table |

### Files to Modify

| File | Change |
|------|--------|
| `modules/domains/module.go` | Register GoDaddyService, GoDaddyHandler, add routes, update reconfigureAll |
| `modules/domains/module.go` | Add GoDaddy menu item |
| `pkg/i18n/locales/en.json` | Add `menu.connect_godaddy` translation |
| `pkg/i18n/locales/ru.json` | Add `menu.connect_godaddy` translation |

---

### Task 1: GoDaddy API Client - Base

**Files:**
- Create: `pkg/providers/godaddy/client.go`

**Interfaces:**
- Consumes: None
- Produces: `NewClient(apiKey, apiSecret string) *Client`, `(c *Client) doRequest(ctx, method, path string, body interface{}) ([]byte, error)`

- [ ] **Step 1: Create the package directory**

```bash
mkdir -p pkg/providers/godaddy
```

- [ ] **Step 2: Create client.go with base HTTP client**

```go
package godaddy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	defaultBaseURL     = "https://api.godaddy.com"
	minRequestInterval = 1000 * time.Millisecond // 60 req/min = 1 req/sec
)

// Client is the GoDaddy API client
type Client struct {
	apiKey     string
	apiSecret  string
	baseURL    string
	httpClient *http.Client

	// Rate limiting
	mu          sync.Mutex
	lastRequest time.Time
}

// NewClient creates a new GoDaddy API client
func NewClient(apiKey, apiSecret string) *Client {
	return &Client{
		apiKey:    apiKey,
		apiSecret: apiSecret,
		baseURL:   defaultBaseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetBaseURL sets a custom base URL (for testing)
func (c *Client) SetBaseURL(url string) {
	c.baseURL = url
}

// APIError represents a GoDaddy API error
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Fields  []struct {
		Path    string `json:"path"`
		Message string `json:"message"`
	} `json:"fields,omitempty"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("godaddy: %s (%s)", e.Message, e.Code)
}

// doRequest makes a rate-limited request to the GoDaddy API
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	// Rate limiting
	c.mu.Lock()
	elapsed := time.Since(c.lastRequest)
	if elapsed < minRequestInterval {
		time.Sleep(minRequestInterval - elapsed)
	}
	c.lastRequest = time.Now()
	c.mu.Unlock()

	// Build URL
	reqURL := c.baseURL + path

	// Create request body if provided
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, reqURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Set headers - GoDaddy uses sso-key auth
	req.Header.Set("Authorization", fmt.Sprintf("sso-key %s:%s", c.apiKey, c.apiSecret))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Execute
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	// Check for error status codes
	if resp.StatusCode >= 400 {
		var apiErr APIError
		if err := json.Unmarshal(respBody, &apiErr); err == nil && apiErr.Message != "" {
			return nil, &apiErr
		}
		return nil, fmt.Errorf("godaddy: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
```

- [ ] **Step 3: Verify file compiles**

Run: `go build ./pkg/providers/godaddy/...`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add pkg/providers/godaddy/client.go
git commit -m "feat(godaddy): add base API client with auth and rate limiting"
```

---

### Task 2: GoDaddy API Client - Domains

**Files:**
- Create: `pkg/providers/godaddy/domains.go`

**Interfaces:**
- Consumes: `(c *Client) doRequest(ctx, method, path string, body interface{}) ([]byte, error)`
- Produces: `(c *Client) ListDomains(ctx) ([]Domain, error)`, `(c *Client) GetDomain(ctx, domain string) (*Domain, error)`, `(c *Client) VerifyCredentials(ctx) error`

- [ ] **Step 1: Create domains.go**

```go
package godaddy

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Domain represents a GoDaddy domain
type Domain struct {
	Domain           string    `json:"domain"`
	DomainID         int       `json:"domainId"`
	Status           string    `json:"status"` // ACTIVE, CANCELLED, etc.
	Expires          time.Time `json:"expires"`
	ExpirationProtected bool   `json:"expirationProtected"`
	HoldRegistrar    bool      `json:"holdRegistrar"`
	Locked           bool      `json:"locked"`
	Privacy          bool      `json:"privacy"`
	RenewAuto        bool      `json:"renewAuto"`
	RenewDeadline    time.Time `json:"renewDeadline"`
	Renewable        bool      `json:"renewable"`
	TransferProtected bool     `json:"transferProtected"`
}

// ListDomains returns all domains in the account
func (c *Client) ListDomains(ctx context.Context) ([]Domain, error) {
	path := "/v1/domains"
	body, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var domains []Domain
	if err := json.Unmarshal(body, &domains); err != nil {
		return nil, fmt.Errorf("parsing domains response: %w", err)
	}

	return domains, nil
}

// GetDomain returns details for a specific domain
func (c *Client) GetDomain(ctx context.Context, domain string) (*Domain, error) {
	path := fmt.Sprintf("/v1/domains/%s", domain)
	body, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var d Domain
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, fmt.Errorf("parsing domain response: %w", err)
	}

	return &d, nil
}

// VerifyCredentials checks if the API credentials are valid
func (c *Client) VerifyCredentials(ctx context.Context) error {
	_, err := c.ListDomains(ctx)
	return err
}
```

- [ ] **Step 2: Verify file compiles**

Run: `go build ./pkg/providers/godaddy/...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add pkg/providers/godaddy/domains.go
git commit -m "feat(godaddy): add domain listing and verification"
```

---

### Task 3: GoDaddy API Client - DNS

**Files:**
- Create: `pkg/providers/godaddy/dns.go`

**Interfaces:**
- Consumes: `(c *Client) doRequest(ctx, method, path string, body interface{}) ([]byte, error)`
- Produces: `(c *Client) ListDNSRecords(ctx, domain string) ([]DNSRecord, error)`, `(c *Client) SetupDNS(ctx, domain, serverIP string) error`, `(c *Client) AddVerificationTXT(ctx, domain, token string) error`, `(c *Client) AddAcmeDNSCname(ctx, domain, fulldomain string) error`

- [ ] **Step 1: Create dns.go**

```go
package godaddy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// DNSRecord represents a DNS record
type DNSRecord struct {
	Type     string `json:"type"`              // A, AAAA, CNAME, TXT, MX, NS, SRV
	Name     string `json:"name"`              // subdomain or @ for root
	Data     string `json:"data"`              // value (IP, target, etc.)
	TTL      int    `json:"ttl"`               // 600-86400
	Priority int    `json:"priority,omitempty"` // MX priority
}

// ListDNSRecords returns all DNS records for a domain
func (c *Client) ListDNSRecords(ctx context.Context, domain string) ([]DNSRecord, error) {
	path := fmt.Sprintf("/v1/domains/%s/records", domain)
	body, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var records []DNSRecord
	if err := json.Unmarshal(body, &records); err != nil {
		return nil, fmt.Errorf("parsing DNS records response: %w", err)
	}

	return records, nil
}

// AddDNSRecords adds DNS records (PATCH appends to existing)
func (c *Client) AddDNSRecords(ctx context.Context, domain string, records []DNSRecord) error {
	path := fmt.Sprintf("/v1/domains/%s/records", domain)
	_, err := c.doRequest(ctx, "PATCH", path, records)
	return err
}

// ReplaceDNSRecords replaces all records of a given type and name
func (c *Client) ReplaceDNSRecords(ctx context.Context, domain, recordType, name string, records []DNSRecord) error {
	path := fmt.Sprintf("/v1/domains/%s/records/%s/%s", domain, recordType, name)
	_, err := c.doRequest(ctx, "PUT", path, records)
	return err
}

// DeleteDNSRecords deletes all records of a given type and name
func (c *Client) DeleteDNSRecords(ctx context.Context, domain, recordType, name string) error {
	path := fmt.Sprintf("/v1/domains/%s/records/%s/%s", domain, recordType, name)
	_, err := c.doRequest(ctx, "DELETE", path, nil)
	return err
}

// SetupDNS configures A records for root (@) and wildcard (*)
func (c *Client) SetupDNS(ctx context.Context, domain, serverIP string) error {
	// Replace root A record (@)
	rootRecords := []DNSRecord{{
		Type: "A",
		Name: "@",
		Data: serverIP,
		TTL:  600,
	}}
	if err := c.ReplaceDNSRecords(ctx, domain, "A", "@", rootRecords); err != nil {
		return fmt.Errorf("setting root A record: %w", err)
	}

	// Replace wildcard A record (*)
	wildcardRecords := []DNSRecord{{
		Type: "A",
		Name: "*",
		Data: serverIP,
		TTL:  600,
	}}
	if err := c.ReplaceDNSRecords(ctx, domain, "A", "*", wildcardRecords); err != nil {
		return fmt.Errorf("setting wildcard A record: %w", err)
	}

	return nil
}

// AddVerificationTXT adds the GuardBot verification TXT record
// Creates: _guardbot-verify → token
func (c *Client) AddVerificationTXT(ctx context.Context, domain, token string) error {
	// First try to delete existing
	c.DeleteDNSRecords(ctx, domain, "TXT", "_guardbot-verify")

	records := []DNSRecord{{
		Type: "TXT",
		Name: "_guardbot-verify",
		Data: token,
		TTL:  600,
	}}
	return c.AddDNSRecords(ctx, domain, records)
}

// AddAcmeDNSCname adds a CNAME record for acme-dns delegation
// Creates: _acme-challenge → fulldomain
func (c *Client) AddAcmeDNSCname(ctx context.Context, domain, fulldomain string) error {
	// Delete existing _acme-challenge records (TXT or CNAME)
	c.DeleteDNSRecords(ctx, domain, "CNAME", "_acme-challenge")
	c.DeleteDNSRecords(ctx, domain, "TXT", "_acme-challenge")

	// Remove trailing dot if present
	target := strings.TrimSuffix(fulldomain, ".")

	records := []DNSRecord{{
		Type: "CNAME",
		Name: "_acme-challenge",
		Data: target,
		TTL:  600,
	}}
	return c.AddDNSRecords(ctx, domain, records)
}

// AddTXTRecord adds a TXT record (for SSL challenges)
func (c *Client) AddTXTRecord(ctx context.Context, domain, name, value string) error {
	records := []DNSRecord{{
		Type: "TXT",
		Name: name,
		Data: value,
		TTL:  600,
	}}
	return c.AddDNSRecords(ctx, domain, records)
}
```

- [ ] **Step 2: Verify file compiles**

Run: `go build ./pkg/providers/godaddy/...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add pkg/providers/godaddy/dns.go
git commit -m "feat(godaddy): add DNS record management"
```

---

### Task 4: Database Migration

**Files:**
- Create: `modules/domains/migrations/009_godaddy.sql`

**Interfaces:**
- Consumes: None
- Produces: `godaddy_api_key_encrypted`, `godaddy_api_secret_encrypted`, `godaddy_connected_at` columns on users table

- [ ] **Step 1: Create migration file**

```sql
-- Add GoDaddy credential columns to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS godaddy_api_key_encrypted TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS godaddy_api_secret_encrypted TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS godaddy_connected_at TIMESTAMP;
```

- [ ] **Step 2: Verify migration filename is correct**

Run: `ls modules/domains/migrations/ | tail -3`
Expected: See existing migrations numbered 006-008

- [ ] **Step 3: Commit**

```bash
git add modules/domains/migrations/009_godaddy.sql
git commit -m "feat(godaddy): add database migration for credentials"
```

---

### Task 5: GoDaddy Models

**Files:**
- Create: `modules/domains/models/godaddy.go`

**Interfaces:**
- Consumes: None
- Produces: `GoDaddyConnection`, `GoDaddyDomain`, `GoDaddyImportInput` types

- [ ] **Step 1: Create models file**

```go
package models

import "time"

// GoDaddyConnection represents user's GoDaddy API connection
type GoDaddyConnection struct {
	UserID      string     `json:"userId"`
	APIKey      string     `json:"-"` // decrypted at runtime, never in JSON
	APISecret   string     `json:"-"` // decrypted at runtime, never in JSON
	ConnectedAt *time.Time `json:"connectedAt,omitempty"`
	IsConnected bool       `json:"isConnected"`
}

// GoDaddyDomain represents a domain from the GoDaddy API
type GoDaddyDomain struct {
	Domain     string     `json:"domain"`
	Status     string     `json:"status"` // ACTIVE, CANCELLED, etc.
	Expires    *time.Time `json:"expires,omitempty"`
	IsImported bool       `json:"isImported"`        // already in our system
	LocalID    string     `json:"localId,omitempty"` // our domain ID if imported
}

// GoDaddyImportInput is the input for importing a domain
type GoDaddyImportInput struct {
	Domain       string `json:"domain" validate:"required"`
	ConfigureDNS bool   `json:"configureDns"` // auto-configure DNS records
}
```

- [ ] **Step 2: Verify file compiles**

Run: `go build ./modules/domains/...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add modules/domains/models/godaddy.go
git commit -m "feat(godaddy): add models for connection and domains"
```

---

### Task 6: GoDaddy Service

**Files:**
- Create: `modules/domains/services/godaddy_service.go`

**Interfaces:**
- Consumes: `pkg/providers/godaddy.Client`, `DomainService.GetDeployIP()`, `VerificationService.GenerateWildcardSSLWithAcmeDNS()`
- Produces: `NewGoDaddyService()`, `GetConnection()`, `Connect()`, `Disconnect()`, `ListDomains()`, `Import()`, `ReconfigureAllDomains()`, `setupDNSInternal()`, `setupProxyNginxConfig()`

- [ ] **Step 1: Create service file**

```go
package services

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/botginx/botginx/modules/domains/models"
	"github.com/botginx/botginx/pkg/acmedns"
	"github.com/botginx/botginx/pkg/providers/godaddy"
	"github.com/botginx/botginx/pkg/sshexec"
	"github.com/jmoiron/sqlx"
)

// GoDaddyService handles GoDaddy integration
type GoDaddyService struct {
	db                  *sqlx.DB
	encryptionKey       []byte
	domainService       *DomainService
	verificationService *VerificationService
}

// NewGoDaddyService creates a new GoDaddy service
func NewGoDaddyService(db *sqlx.DB, domainService *DomainService, verificationService *VerificationService) *GoDaddyService {
	key := os.Getenv("GODADDY_ENCRYPTION_KEY")
	if key == "" {
		key = os.Getenv("SESSION_SECRET")
	}

	keyBytes := []byte(key)
	if len(keyBytes) < 32 {
		padded := make([]byte, 32)
		copy(padded, keyBytes)
		keyBytes = padded
	} else if len(keyBytes) > 32 {
		keyBytes = keyBytes[:32]
	}

	return &GoDaddyService{
		db:                  db,
		encryptionKey:       keyBytes,
		domainService:       domainService,
		verificationService: verificationService,
	}
}

// encrypt encrypts data using AES-256-GCM
func (s *GoDaddyService) encrypt(plaintext string) (string, error) {
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

// decrypt decrypts data using AES-256-GCM
func (s *GoDaddyService) decrypt(ciphertext string) (string, error) {
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

// GetConnection returns the user's GoDaddy connection status
func (s *GoDaddyService) GetConnection(userID string) (*models.GoDaddyConnection, error) {
	var encryptedKey sql.NullString
	var encryptedSecret sql.NullString
	var connectedAt sql.NullTime

	err := s.db.QueryRow(`
		SELECT godaddy_api_key_encrypted, godaddy_api_secret_encrypted, godaddy_connected_at
		FROM users WHERE id = $1
	`, userID).Scan(&encryptedKey, &encryptedSecret, &connectedAt)
	if err != nil {
		return nil, err
	}

	conn := &models.GoDaddyConnection{
		UserID:      userID,
		IsConnected: encryptedKey.Valid && encryptedKey.String != "" && encryptedSecret.Valid && encryptedSecret.String != "",
	}

	if connectedAt.Valid {
		conn.ConnectedAt = &connectedAt.Time
	}

	return conn, nil
}

// Connect validates and saves the GoDaddy credentials
func (s *GoDaddyService) Connect(ctx context.Context, userID, apiKey, apiSecret string) error {
	// Validate credentials by making a test request
	client := godaddy.NewClient(apiKey, apiSecret)
	if err := client.VerifyCredentials(ctx); err != nil {
		return fmt.Errorf("invalid credentials: %w", err)
	}

	// Encrypt credentials
	encryptedKey, err := s.encrypt(apiKey)
	if err != nil {
		return fmt.Errorf("encrypting API key: %w", err)
	}

	encryptedSecret, err := s.encrypt(apiSecret)
	if err != nil {
		return fmt.Errorf("encrypting API secret: %w", err)
	}

	// Save to database
	_, err = s.db.Exec(`
		UPDATE users 
		SET godaddy_api_key_encrypted = $1, 
		    godaddy_api_secret_encrypted = $2, 
		    godaddy_connected_at = NOW()
		WHERE id = $3
	`, encryptedKey, encryptedSecret, userID)
	if err != nil {
		return fmt.Errorf("saving credentials: %w", err)
	}

	return nil
}

// Disconnect removes the GoDaddy credentials
func (s *GoDaddyService) Disconnect(userID string) error {
	_, err := s.db.Exec(`
		UPDATE users 
		SET godaddy_api_key_encrypted = NULL, 
		    godaddy_api_secret_encrypted = NULL, 
		    godaddy_connected_at = NULL
		WHERE id = $1
	`, userID)
	return err
}

// getClient creates an authenticated GoDaddy client for the user
func (s *GoDaddyService) getClient(userID string) (*godaddy.Client, error) {
	var encryptedKey sql.NullString
	var encryptedSecret sql.NullString

	err := s.db.QueryRow(`
		SELECT godaddy_api_key_encrypted, godaddy_api_secret_encrypted
		FROM users WHERE id = $1
	`, userID).Scan(&encryptedKey, &encryptedSecret)
	if err != nil {
		return nil, err
	}

	if !encryptedKey.Valid || !encryptedSecret.Valid {
		return nil, errors.New("godaddy not connected")
	}

	apiKey, err := s.decrypt(encryptedKey.String)
	if err != nil {
		return nil, fmt.Errorf("decrypting API key: %w", err)
	}

	apiSecret, err := s.decrypt(encryptedSecret.String)
	if err != nil {
		return nil, fmt.Errorf("decrypting API secret: %w", err)
	}

	return godaddy.NewClient(apiKey, apiSecret), nil
}

// ListDomains returns all domains from the user's GoDaddy account
func (s *GoDaddyService) ListDomains(ctx context.Context, userID string) ([]models.GoDaddyDomain, error) {
	client, err := s.getClient(userID)
	if err != nil {
		return nil, err
	}

	gdDomains, err := client.ListDomains(ctx)
	if err != nil {
		return nil, err
	}

	// Get list of already-imported domains
	importedDomains := make(map[string]string)
	rows, err := s.db.Query(`
		SELECT name, id FROM domains 
		WHERE user_id = $1 AND source = 'godaddy'
	`, userID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var name, id string
			rows.Scan(&name, &id)
			importedDomains[name] = id
		}
	}

	domains := make([]models.GoDaddyDomain, len(gdDomains))
	for i, d := range gdDomains {
		domains[i] = models.GoDaddyDomain{
			Domain:  d.Domain,
			Status:  d.Status,
			Expires: &d.Expires,
		}
		if localID, ok := importedDomains[d.Domain]; ok {
			domains[i].IsImported = true
			domains[i].LocalID = localID
		}
	}

	return domains, nil
}

// Import imports a GoDaddy domain as a GuardBot domain
func (s *GoDaddyService) Import(ctx context.Context, userID string, input models.GoDaddyImportInput) (*models.Domain, error) {
	client, err := s.getClient(userID)
	if err != nil {
		return nil, err
	}

	// Verify domain exists
	gdDomain, err := client.GetDomain(ctx, input.Domain)
	if err != nil {
		return nil, fmt.Errorf("getting domain: %w", err)
	}

	// Check if domain already exists
	var existingID string
	err = s.db.QueryRow(`SELECT id FROM domains WHERE name = $1 AND user_id = $2`, gdDomain.Domain, userID).Scan(&existingID)
	if err == nil {
		return nil, fmt.Errorf("domain %s already exists", gdDomain.Domain)
	}

	// Create domain in GuardBot
	domain := &models.Domain{
		UserID:            userID,
		Name:              gdDomain.Domain,
		Source:            "godaddy",
		DNSAutoConfigured: input.ConfigureDNS,
		IsWildcard:        true,
		VerifyToken:       fmt.Sprintf("guardbot-%s", generateID(12)),
	}
	domain.ID = fmt.Sprintf("dom_%s", generateID(12))

	// Insert domain
	err = s.db.QueryRow(`
		INSERT INTO domains (id, user_id, name, verify_token, source, dns_auto_configured, is_wildcard, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, true, NOW(), NOW())
		RETURNING created_at
	`, domain.ID, domain.UserID, domain.Name, domain.VerifyToken, domain.Source, domain.DNSAutoConfigured).
		Scan(&domain.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating domain: %w", err)
	}

	// Auto-configure DNS and SSL if requested
	if input.ConfigureDNS {
		go func(domID, uid, domainName string) {
			setupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			// Step 1: Set up A records and acme-dns CNAME
			if err := s.setupDNSInternal(setupCtx, uid, domainName); err != nil {
				log.Printf("[GoDaddy] DNS setup failed for %s: %v", domainName, err)
				s.db.Exec(`UPDATE domains SET dns_config_error = $1 WHERE id = $2`, err.Error(), domID)
				return
			}

			s.db.Exec(`UPDATE domains SET dns_auto_configured = true, dns_verified = true WHERE id = $1`, domID)

			// Step 2: Get acme-dns credentials for SSL generation
			var acmeSubdomain, acmeUsername, acmePassword string
			err := s.db.QueryRow(`
				SELECT COALESCE(acme_subdomain, ''), COALESCE(acme_username, ''), COALESCE(acme_password, '')
				FROM domains WHERE id = $1
			`, domID).Scan(&acmeSubdomain, &acmeUsername, &acmePassword)

			if err != nil || acmeSubdomain == "" {
				// Fallback to slow method using direct GoDaddy TXT records
				log.Printf("[GoDaddy] No acme-dns credentials for %s, using slow TXT method", domainName)
				if err := s.GenerateWildcardSSLSlow(setupCtx, uid, domainName); err != nil {
					log.Printf("[GoDaddy] Slow SSL also failed for %s: %v", domainName, err)
					s.db.Exec(`UPDATE domains SET ssl_error = $1 WHERE id = $2`, err.Error(), domID)
				} else {
					log.Printf("[GoDaddy] Wildcard SSL completed for %s (slow method)", domainName)
					s.domainService.CompleteSSLSetup(domID)
				}
				return
			}

			// Step 3: Wait for CNAME propagation
			log.Printf("[GoDaddy] Waiting 90s for CNAME propagation for %s", domainName)
			time.Sleep(90 * time.Second)

			// Step 4: Generate wildcard SSL using acme-dns
			log.Printf("[GoDaddy] Generating wildcard SSL for %s", domainName)
			if err := s.verificationService.GenerateWildcardSSLWithAcmeDNS(domainName, acmeSubdomain, acmeUsername, acmePassword); err != nil {
				log.Printf("[GoDaddy] acme-dns SSL failed for %s: %v, trying slow method", domainName, err)
				if err := s.GenerateWildcardSSLSlow(setupCtx, uid, domainName); err != nil {
					log.Printf("[GoDaddy] Slow SSL also failed for %s: %v", domainName, err)
					s.db.Exec(`UPDATE domains SET ssl_error = $1 WHERE id = $2`, err.Error(), domID)
				} else {
					log.Printf("[GoDaddy] Wildcard SSL completed for %s (slow fallback)", domainName)
					s.domainService.CompleteSSLSetup(domID)
					if err := s.setupProxyNginxConfig(domainName); err != nil {
						log.Printf("[GoDaddy] Warning: nginx config failed for %s: %v", domainName, err)
					}
				}
				return
			}

			// Step 5: Complete setup
			log.Printf("[GoDaddy] Wildcard SSL completed for %s", domainName)
			s.domainService.CompleteSSLSetup(domID)
			s.db.Exec(`UPDATE domains SET acme_cname_verified = true WHERE id = $1`, domID)

			// Step 6: Setup nginx config
			if err := s.setupProxyNginxConfig(domainName); err != nil {
				log.Printf("[GoDaddy] Warning: nginx config failed for %s: %v", domainName, err)
			}
		}(domain.ID, userID, gdDomain.Domain)
	}

	return domain, nil
}

func (s *GoDaddyService) setupDNSInternal(ctx context.Context, userID, domain string) error {
	client, err := s.getClient(userID)
	if err != nil {
		return err
	}

	// Get deploy server IP (where SSL and traffic are handled)
	serverIP := s.domainService.GetDeployIP()
	if serverIP == "" || serverIP == "No server available" || net.ParseIP(serverIP) == nil {
		return errors.New("no deploy server available - add one in Admin > Servers")
	}

	// Setup A records
	if err := client.SetupDNS(ctx, domain, serverIP); err != nil {
		return fmt.Errorf("setting up A records: %w", err)
	}

	// Get domain verify token for TXT record
	var verifyToken string
	s.db.QueryRow(`SELECT verify_token FROM domains WHERE name = $1 AND user_id = $2`, domain, userID).Scan(&verifyToken)

	// Add verification TXT record
	if verifyToken != "" {
		if err := client.AddVerificationTXT(ctx, domain, verifyToken); err != nil {
			log.Printf("[GoDaddy] Warning: failed to add verification TXT for %s: %v", domain, err)
		}
	}

	// Setup acme-dns CNAME if configured
	acmeDomain := os.Getenv("ACME_DNS_DOMAIN")
	if acmeDomain != "" {
		// Check if domain already has acme-dns credentials
		var existingFulldomain string
		s.db.QueryRow(`SELECT acme_fulldomain FROM domains WHERE name = $1 AND user_id = $2 AND acme_fulldomain IS NOT NULL`,
			domain, userID).Scan(&existingFulldomain)

		var fulldomain string
		if existingFulldomain != "" {
			fulldomain = existingFulldomain
			log.Printf("[GoDaddy] Using existing acme-dns credentials for %s → %s", domain, fulldomain)
		} else {
			reg, err := s.registerWithAcmeDNS(userID, domain)
			if err != nil {
				log.Printf("[GoDaddy] Warning: acme-dns registration failed for %s: %v", domain, err)
			} else {
				fulldomain = reg.Fulldomain
			}
		}

		// Add CNAME if we have a fulldomain
		if fulldomain != "" {
			if err := client.AddAcmeDNSCname(ctx, domain, fulldomain); err != nil {
				log.Printf("[GoDaddy] Warning: failed to add ACME DNS CNAME for %s: %v", domain, err)
			}
		}
	}

	// Update domain status
	s.db.Exec(`
		UPDATE domains
		SET dns_auto_configured = true, dns_config_error = NULL, dns_verified = true
		WHERE name = $1 AND user_id = $2
	`, domain, userID)

	return nil
}

// ReconfigureDomain updates DNS for a single domain
func (s *GoDaddyService) ReconfigureDomain(ctx context.Context, userID, domainID string) error {
	type domainInfo struct {
		ID                string  `db:"id"`
		Name              string  `db:"name"`
		AcmeFulldomain    *string `db:"acme_fulldomain"`
		AcmeSubdomain     *string `db:"acme_subdomain"`
		AcmeUsername      *string `db:"acme_username"`
		AcmePassword      *string `db:"acme_password"`
		AcmeCnameVerified bool    `db:"acme_cname_verified"`
		SSLEnabled        bool    `db:"ssl_enabled"`
	}

	var d domainInfo
	err := s.db.Get(&d, `
		SELECT id, name, acme_fulldomain, acme_subdomain,
		       acme_username, acme_password, COALESCE(acme_cname_verified, false) as acme_cname_verified,
		       COALESCE(ssl_enabled, false) as ssl_enabled
		FROM domains
		WHERE id = $1 AND user_id = $2 AND source = 'godaddy'
	`, domainID, userID)
	if err != nil {
		return fmt.Errorf("domain not found: %w", err)
	}

	if err := s.setupDNSInternal(ctx, userID, d.Name); err != nil {
		s.db.Exec(`UPDATE domains SET dns_config_error = $1, updated_at = NOW() WHERE id = $2`, err.Error(), d.ID)
		return err
	}

	// Trigger SSL generation if needed
	if !d.SSLEnabled && d.AcmeSubdomain != nil && d.AcmeUsername != nil && d.AcmePassword != nil {
		go func(domID, domainName, subdomain, username, password string) {
			log.Printf("[GoDaddy] ReconfigureDomain: generating SSL for %s", domainName)
			time.Sleep(60 * time.Second)

			if err := s.verificationService.GenerateWildcardSSLWithAcmeDNS(domainName, subdomain, username, password); err != nil {
				log.Printf("[GoDaddy] ReconfigureDomain: SSL failed for %s: %v", domainName, err)
				s.db.Exec(`UPDATE domains SET ssl_error = $1 WHERE id = $2`, err.Error(), domID)
			} else {
				log.Printf("[GoDaddy] ReconfigureDomain: SSL completed for %s", domainName)
				s.domainService.CompleteSSLSetup(domID)
				s.db.Exec(`UPDATE domains SET acme_cname_verified = true WHERE id = $1`, domID)
			}
		}(d.ID, d.Name, *d.AcmeSubdomain, *d.AcmeUsername, *d.AcmePassword)
	}

	return nil
}

// ReconfigureAllDomains updates DNS for all GoDaddy domains
func (s *GoDaddyService) ReconfigureAllDomains(ctx context.Context, serverIP string) ([]ReconfigureResult, error) {
	var results []ReconfigureResult

	rows, err := s.db.Query(`
		SELECT d.id, d.name, d.user_id
		FROM domains d
		WHERE d.source = 'godaddy'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var domainID, domainName, userID string
		if err := rows.Scan(&domainID, &domainName, &userID); err != nil {
			continue
		}

		result := ReconfigureResult{
			Domain: domainName,
		}

		if err := s.setupDNSInternal(ctx, userID, domainName); err != nil {
			result.Success = false
			result.Error = err.Error()
		} else {
			result.Success = true
		}

		results = append(results, result)
	}

	return results, nil
}

// registerWithAcmeDNS registers a domain with acme-dns on the Deploy VPS
func (s *GoDaddyService) registerWithAcmeDNS(userID, domainName string) (*acmedns.Registration, error) {
	var server struct {
		IP       string `db:"ip"`
		Port     int    `db:"port"`
		User     string `db:"ssh_user"`
		Password string `db:"ssh_password"`
	}
	err := s.db.Get(&server, `
		SELECT ip, COALESCE(port, 22) as port, ssh_user, ssh_password
		FROM servers WHERE status = 'ready' ORDER BY created_at LIMIT 1
	`)
	if err != nil {
		return nil, fmt.Errorf("no deploy server found: %w", err)
	}

	client, err := acmedns.NewClient(server.IP, server.Port, server.User, server.Password)
	if err != nil {
		return nil, fmt.Errorf("SSH connection failed: %w", err)
	}
	defer client.Close()

	if !client.IsInstalled() {
		acmeDomain := os.Getenv("ACME_DNS_DOMAIN")
		if acmeDomain == "" {
			return nil, errors.New("ACME_DNS_DOMAIN not set and acme-dns not installed")
		}
	}

	reg, err := client.Register()
	if err != nil {
		return nil, fmt.Errorf("acme-dns registration failed: %w", err)
	}

	_, err = s.db.Exec(`
		UPDATE domains SET
			acme_subdomain = $1, acme_username = $2, acme_password = $3,
			acme_fulldomain = $4, updated_at = NOW()
		WHERE name = $5 AND user_id = $6
	`, reg.Subdomain, reg.Username, reg.Password, reg.Fulldomain, domainName, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to store acme-dns credentials: %w", err)
	}
	return reg, nil
}

// GenerateWildcardSSLSlow generates wildcard SSL using GoDaddy DNS for DNS-01 challenge
func (s *GoDaddyService) GenerateWildcardSSLSlow(ctx context.Context, userID, domainName string) error {
	// Get the user's GoDaddy credentials
	var encryptedKey, encryptedSecret string
	err := s.db.QueryRow(`SELECT godaddy_api_key_encrypted, godaddy_api_secret_encrypted FROM users WHERE id = $1`, userID).Scan(&encryptedKey, &encryptedSecret)
	if err != nil || encryptedKey == "" {
		return fmt.Errorf("user not connected to GoDaddy")
	}

	apiKey, err := s.decrypt(encryptedKey)
	if err != nil {
		return fmt.Errorf("decrypting API key: %w", err)
	}

	apiSecret, err := s.decrypt(encryptedSecret)
	if err != nil {
		return fmt.Errorf("decrypting API secret: %w", err)
	}

	// Get deploy server credentials
	var server struct {
		IP       string `db:"ip"`
		Port     int    `db:"port"`
		User     string `db:"ssh_user"`
		Password string `db:"ssh_password"`
	}
	err = s.db.Get(&server, `
		SELECT ip, COALESCE(port, 22) as port, ssh_user, ssh_password
		FROM servers WHERE status = 'ready' ORDER BY created_at LIMIT 1
	`)
	if err != nil {
		return fmt.Errorf("no deploy server available: %w", err)
	}

	port := fmt.Sprintf("%d", server.Port)
	sshClient, err := sshexec.NewClient(server.IP, port, server.User, server.Password)
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer sshClient.Close()

	log.Printf("[GoDaddy] Generating wildcard SSL (slow method) for *.%s", domainName)

	// Create auth hook that adds TXT record via GoDaddy API
	authHook := fmt.Sprintf(`#!/bin/bash
API_KEY="%s"
API_SECRET="%s"
DOMAIN="%s"

# Delete existing _acme-challenge TXT records
curl -s -X DELETE "https://api.godaddy.com/v1/domains/${DOMAIN}/records/TXT/_acme-challenge" \
    -H "Authorization: sso-key ${API_KEY}:${API_SECRET}" > /dev/null 2>&1

sleep 2

# Add new TXT record
curl -s -X PATCH "https://api.godaddy.com/v1/domains/${DOMAIN}/records" \
    -H "Authorization: sso-key ${API_KEY}:${API_SECRET}" \
    -H "Content-Type: application/json" \
    --data "[{\"type\":\"TXT\",\"name\":\"_acme-challenge\",\"data\":\"${CERTBOT_VALIDATION}\",\"ttl\":600}]" > /dev/null

# Wait for GoDaddy DNS propagation (can be slow, 180s to be safe)
sleep 180
`, apiKey, apiSecret, domainName)

	// Create cleanup hook
	cleanupHook := fmt.Sprintf(`#!/bin/bash
API_KEY="%s"
API_SECRET="%s"
DOMAIN="%s"

curl -s -X DELETE "https://api.godaddy.com/v1/domains/${DOMAIN}/records/TXT/_acme-challenge" \
    -H "Authorization: sso-key ${API_KEY}:${API_SECRET}" > /dev/null 2>&1
`, apiKey, apiSecret, domainName)

	// Write hooks to deploy server
	authPath := fmt.Sprintf("/tmp/godaddy_auth_%s.sh", domainName)
	cleanupPath := fmt.Sprintf("/tmp/godaddy_cleanup_%s.sh", domainName)

	sshClient.Run(fmt.Sprintf("cat > %s << 'AUTHEOF'\n%sAUTHEOF\nchmod +x %s", authPath, authHook, authPath))
	sshClient.Run(fmt.Sprintf("cat > %s << 'CLEANEOF'\n%sCLEANEOF\nchmod +x %s", cleanupPath, cleanupHook, cleanupPath))

	// Run certbot for wildcard cert
	certbotCmd := fmt.Sprintf(`certbot certonly --manual --preferred-challenges dns \
		-d "*.%s" -d "%s" \
		--manual-auth-hook %s \
		--manual-cleanup-hook %s \
		--agree-tos --email admin@%s \
		--non-interactive --force-renewal 2>&1`,
		domainName, domainName, authPath, cleanupPath, domainName)

	output, err := sshClient.Run(certbotCmd)
	log.Printf("[GoDaddy] Certbot output for %s: %s", domainName, output)

	// Cleanup hook scripts
	sshClient.Run(fmt.Sprintf("rm -f %s %s", authPath, cleanupPath))

	if err != nil {
		return fmt.Errorf("certbot failed: %w\nOutput: %s", err, output)
	}

	log.Printf("[GoDaddy] Wildcard SSL generated for *.%s (slow method)", domainName)
	return nil
}

// setupProxyNginxConfig creates nginx config with proper proxy to botection
func (s *GoDaddyService) setupProxyNginxConfig(domainName string) error {
	var server struct {
		IP       string `db:"ip"`
		Port     int    `db:"port"`
		User     string `db:"ssh_user"`
		Password string `db:"ssh_password"`
	}
	err := s.db.Get(&server, `
		SELECT ip, COALESCE(port, 22) as port, ssh_user, ssh_password
		FROM servers WHERE status = 'ready' ORDER BY created_at LIMIT 1
	`)
	if err != nil {
		return fmt.Errorf("no deploy server available: %w", err)
	}

	port := fmt.Sprintf("%d", server.Port)
	sshClient, err := sshexec.NewClient(server.IP, port, server.User, server.Password)
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer sshClient.Close()

	baseDomain := GetBaseDomain(domainName)

	nginxConf := fmt.Sprintf(`server {
    listen 80;
    server_name %s *.%s;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name %s *.%s;

    ssl_certificate /etc/letsencrypt/live/%s/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/%s/privkey.pem;

    location /.well-known/acme-challenge/ {
        root /var/www/sites/%s;
        allow all;
    }

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}`, baseDomain, baseDomain, baseDomain, baseDomain, baseDomain, baseDomain, baseDomain)

	configPath := fmt.Sprintf("/etc/nginx/sites-available/*.%s.conf", baseDomain)
	writeCmd := fmt.Sprintf("cat > %s << 'NGINXEOF'\n%s\nNGINXEOF", configPath, nginxConf)
	if _, err := sshClient.Run(writeCmd); err != nil {
		return fmt.Errorf("failed to write nginx config: %w", err)
	}

	// Enable site
	linkCmd := fmt.Sprintf("ln -sf %s /etc/nginx/sites-enabled/", configPath)
	sshClient.Run(linkCmd)

	// Test and reload nginx
	testOut, err := sshClient.Run("nginx -t 2>&1")
	if err != nil || strings.Contains(testOut, "failed") {
		sshClient.Run(fmt.Sprintf("rm -f /etc/nginx/sites-enabled/*.%s.conf", baseDomain))
		return fmt.Errorf("nginx config invalid: %s", testOut)
	}

	sshClient.Run("systemctl reload nginx")
	log.Printf("[GoDaddy] Created proxy nginx config for %s", baseDomain)
	return nil
}
```

- [ ] **Step 2: Verify file compiles**

Run: `go build ./modules/domains/...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add modules/domains/services/godaddy_service.go
git commit -m "feat(godaddy): add service with connect, import, reconfigure"
```

---

### Task 7: GoDaddy Handlers

**Files:**
- Create: `modules/domains/handlers/godaddy_handlers.go`

**Interfaces:**
- Consumes: `services.GoDaddyService`, `module.TemplateEngine`
- Produces: HTTP handlers for Page, Connect, Disconnect, Import, ReconfigureDomain, AdminReconfigureAll

- [ ] **Step 1: Create handlers file**

```go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/botginx/botginx/modules/domains/models"
	"github.com/botginx/botginx/modules/domains/services"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

// GoDaddyHandler handles GoDaddy integration endpoints
type GoDaddyHandler struct {
	godaddyService *services.GoDaddyService
	templates      *module.TemplateEngine
}

// NewGoDaddyHandler creates a new GoDaddy handler
func NewGoDaddyHandler(godaddyService *services.GoDaddyService, templates *module.TemplateEngine) *GoDaddyHandler {
	return &GoDaddyHandler{
		godaddyService: godaddyService,
		templates:      templates,
	}
}

func (h *GoDaddyHandler) jsonError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   msg,
	})
}

func (h *GoDaddyHandler) jsonSuccess(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    data,
	})
}

// Page renders the GoDaddy connection page
func (h *GoDaddyHandler) Page(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	conn, err := h.godaddyService.GetConnection(userID)
	if err != nil {
		conn = &models.GoDaddyConnection{IsConnected: false}
	}

	var domains []models.GoDaddyDomain
	if conn.IsConnected {
		domains, _ = h.godaddyService.ListDomains(r.Context(), userID)
	}

	module.RenderUserSection(w, r, h.templates, "domains:godaddy.html", map[string]interface{}{
		"Title":      "GoDaddy",
		"Connection": conn,
		"Domains":    domains,
	})
}

// Connect handles API key connection
func (h *GoDaddyHandler) Connect(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var input struct {
		APIKey    string `json:"apiKey"`
		APISecret string `json:"apiSecret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if input.APIKey == "" || input.APISecret == "" {
		h.jsonError(w, "API Key and Secret are required", http.StatusBadRequest)
		return
	}

	if err := h.godaddyService.Connect(r.Context(), userID, input.APIKey, input.APISecret); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.jsonSuccess(w, map[string]interface{}{
		"message": "Connected to GoDaddy",
	})
}

// Disconnect removes the API credentials
func (h *GoDaddyHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	if err := h.godaddyService.Disconnect(userID); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonSuccess(w, map[string]interface{}{
		"message": "Disconnected from GoDaddy",
	})
}

// ListDomains returns all domains
func (h *GoDaddyHandler) ListDomains(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	domains, err := h.godaddyService.ListDomains(r.Context(), userID)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonSuccess(w, domains)
}

// Import imports a domain from GoDaddy
func (h *GoDaddyHandler) Import(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var input models.GoDaddyImportInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if input.Domain == "" {
		h.jsonError(w, "Domain is required", http.StatusBadRequest)
		return
	}

	domain, err := h.godaddyService.Import(r.Context(), userID, input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.jsonSuccess(w, domain)
}

// ReconfigureDomain updates DNS for a single domain
func (h *GoDaddyHandler) ReconfigureDomain(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	domainID := chi.URLParam(r, "id")

	if err := h.godaddyService.ReconfigureDomain(r.Context(), userID, domainID); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonSuccess(w, map[string]interface{}{
		"message": "DNS reconfigured",
	})
}

// AdminReconfigureAll updates all GoDaddy domains (admin only)
func (h *GoDaddyHandler) AdminReconfigureAll(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ServerIP string `json:"serverIp"`
	}
	json.NewDecoder(r.Body).Decode(&input)

	results, err := h.godaddyService.ReconfigureAllDomains(r.Context(), input.ServerIP)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonSuccess(w, results)
}
```

- [ ] **Step 2: Verify file compiles**

Run: `go build ./modules/domains/...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add modules/domains/handlers/godaddy_handlers.go
git commit -m "feat(godaddy): add HTTP handlers"
```

---

### Task 8: GoDaddy Template

**Files:**
- Create: `modules/domains/templates/godaddy.html`

**Interfaces:**
- Consumes: `.Connection`, `.Domains` from handler
- Produces: UI matching Cloudflare pattern

- [ ] **Step 1: Create template**

```html
{{define "content"}}
<div class="row">
    <div class="col-12">
        <div class="card">
            <div class="card-header d-flex justify-content-between align-items-center">
                <h3 class="card-title mb-0"><i class="bi bi-globe2 me-2"></i>GoDaddy</h3>
                {{if .Connection.IsConnected}}
                <button type="button" class="btn btn-outline-danger btn-sm" onclick="disconnect()">
                    <i class="bi bi-x-lg me-1"></i>Disconnect
                </button>
                {{end}}
            </div>
            <div class="card-body">
                {{if not .Connection.IsConnected}}
                <!-- Not Connected State -->
                <div class="text-center py-5" id="connect-form">
                    <div class="mb-4">
                        <i class="bi bi-globe2 text-primary" style="font-size: 4rem;"></i>
                    </div>
                    <h4 class="mb-3">Connect your GoDaddy account</h4>
                    <p class="text-muted mb-4">Import domains and auto-configure DNS with one click</p>

                    <div class="row justify-content-center">
                        <div class="col-md-6 col-lg-4">
                            <div class="mb-3">
                                <input type="text" class="form-control" id="apiKey" placeholder="API Key">
                            </div>
                            <div class="input-group mb-3">
                                <input type="password" class="form-control" id="apiSecret" placeholder="API Secret">
                                <button class="btn btn-primary" type="button" onclick="connect()">
                                    <i class="bi bi-plug me-1"></i>Connect
                                </button>
                            </div>
                            <p class="text-muted small">
                                <i class="bi bi-info-circle me-1"></i>
                                Get your API credentials from <a href="https://developer.godaddy.com/keys" target="_blank">GoDaddy Developer Portal</a>
                            </p>
                        </div>
                    </div>
                </div>
                {{else}}
                <!-- Connected State -->
                <div class="mb-4">
                    <div class="d-flex align-items-center">
                        <span class="badge bg-success me-2"><i class="bi bi-check-circle me-1"></i>Connected</span>
                        <span class="text-muted small">
                            Since {{.Connection.ConnectedAt.Format "Jan 02, 2006"}}
                        </span>
                    </div>
                </div>

                <!-- Domains List -->
                {{if .Domains}}
                <div class="d-flex justify-content-between align-items-center mb-3">
                    <h5 class="mb-0">Your GoDaddy domains</h5>
                    <button type="button" class="btn btn-outline-secondary btn-sm" onclick="location.reload()">
                        <i class="bi bi-arrow-clockwise me-1"></i>Refresh
                    </button>
                </div>
                <div class="table-responsive">
                    <table class="table table-hover mb-0">
                        <thead>
                            <tr>
                                <th>Domain</th>
                                <th>Status</th>
                                <th>Imported</th>
                                <th class="text-end">Actions</th>
                            </tr>
                        </thead>
                        <tbody>
                            {{range .Domains}}
                            <tr>
                                <td><code class="fs-6">{{.Domain}}</code></td>
                                <td>
                                    {{if eq .Status "ACTIVE"}}
                                    <span class="badge bg-success">Active</span>
                                    {{else}}
                                    <span class="badge bg-secondary">{{.Status}}</span>
                                    {{end}}
                                </td>
                                <td>
                                    {{if .IsImported}}
                                    <span class="badge bg-primary"><i class="bi bi-check me-1"></i>Imported</span>
                                    {{else}}
                                    <span class="text-muted">—</span>
                                    {{end}}
                                </td>
                                <td class="text-end">
                                    {{if .IsImported}}
                                    <div class="btn-group btn-group-sm">
                                        <button type="button" class="btn btn-outline-primary" onclick="reconfigureDomain('{{.LocalID}}', '{{.Domain}}')">
                                            <i class="bi bi-arrow-repeat me-1"></i>Reconfigure
                                        </button>
                                        <a href="/user/domains/{{.LocalID}}" class="btn btn-outline-secondary">
                                            <i class="bi bi-gear me-1"></i>Manage
                                        </a>
                                    </div>
                                    {{else}}
                                    <button type="button" class="btn btn-sm btn-primary" onclick="showImportModal('{{.Domain}}')">
                                        <i class="bi bi-download me-1"></i>Import
                                    </button>
                                    {{end}}
                                </td>
                            </tr>
                            {{end}}
                        </tbody>
                    </table>
                </div>
                {{else}}
                <div class="text-center py-4 text-muted">
                    <i class="bi bi-inbox fs-1 d-block mb-2"></i>
                    No domains found in your GoDaddy account
                </div>
                {{end}}
                {{end}}
            </div>
        </div>
    </div>
</div>

<!-- Import Modal -->
<div class="modal fade" id="importModal" tabindex="-1">
    <div class="modal-dialog">
        <div class="modal-content">
            <div class="modal-header">
                <h5 class="modal-title">Import Domain</h5>
                <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
            </div>
            <div class="modal-body">
                <p>Import <strong id="importDomainName"></strong> to GuardBot?</p>
                <div class="form-check mb-3">
                    <input type="checkbox" class="form-check-input" id="configureDns" checked>
                    <label class="form-check-label" for="configureDns">
                        Auto-configure DNS and SSL
                    </label>
                    <div class="form-text">
                        Sets up A records, SSL certificate, and bot protection automatically
                    </div>
                </div>
            </div>
            <div class="modal-footer">
                <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">Cancel</button>
                <button type="button" class="btn btn-primary" onclick="importDomain()">
                    <i class="bi bi-download me-1"></i>Import
                </button>
            </div>
        </div>
    </div>
</div>

<script>
let importModal;
let currentDomain = '';

document.addEventListener('DOMContentLoaded', () => {
    importModal = new bootstrap.Modal(document.getElementById('importModal'));
});

async function connect() {
    const apiKey = document.getElementById('apiKey').value;
    const apiSecret = document.getElementById('apiSecret').value;
    
    if (!apiKey || !apiSecret) {
        showToast('Please enter both API Key and Secret', 'warning');
        return;
    }

    try {
        const resp = await fetch('/user/domains/godaddy/api/connect', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({apiKey, apiSecret})
        });
        const data = await resp.json();
        
        if (data.success) {
            showToast('Connected to GoDaddy', 'success');
            location.reload();
        } else {
            showToast(data.error || 'Connection failed', 'danger');
        }
    } catch (err) {
        showToast('Connection failed', 'danger');
    }
}

async function disconnect() {
    if (!confirm('Disconnect from GoDaddy?')) return;
    
    try {
        const resp = await fetch('/user/domains/godaddy/api/disconnect', {method: 'POST'});
        const data = await resp.json();
        
        if (data.success) {
            showToast('Disconnected', 'success');
            location.reload();
        } else {
            showToast(data.error || 'Failed to disconnect', 'danger');
        }
    } catch (err) {
        showToast('Failed to disconnect', 'danger');
    }
}

function showImportModal(domain) {
    currentDomain = domain;
    document.getElementById('importDomainName').textContent = domain;
    importModal.show();
}

async function importDomain() {
    const configureDns = document.getElementById('configureDns').checked;
    
    try {
        const resp = await fetch('/user/domains/godaddy/api/import', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                domain: currentDomain,
                configureDns: configureDns
            })
        });
        const data = await resp.json();
        
        if (data.success) {
            importModal.hide();
            showToast('Domain imported! DNS and SSL setup started.', 'success');
            location.reload();
        } else {
            showToast(data.error || 'Import failed', 'danger');
        }
    } catch (err) {
        showToast('Import failed', 'danger');
    }
}

async function reconfigureDomain(domainId, domainName) {
    if (!confirm(`Reconfigure DNS for ${domainName}?`)) return;
    
    try {
        const resp = await fetch(`/user/domains/godaddy/api/reconfigure/${domainId}`, {method: 'POST'});
        const data = await resp.json();
        
        if (data.success) {
            showToast('DNS reconfigured', 'success');
        } else {
            showToast(data.error || 'Reconfiguration failed', 'danger');
        }
    } catch (err) {
        showToast('Reconfiguration failed', 'danger');
    }
}
</script>
{{end}}
```

- [ ] **Step 2: Verify template syntax**

Run: `go build ./modules/domains/...`
Expected: No errors (templates are embedded at build time)

- [ ] **Step 3: Commit**

```bash
git add modules/domains/templates/godaddy.html
git commit -m "feat(godaddy): add UI template"
```

---

### Task 9: Module Integration

**Files:**
- Modify: `modules/domains/module.go:27-39` (add service/handler fields)
- Modify: `modules/domains/module.go:51-75` (Init, register service/handler)
- Modify: `modules/domains/module.go:96-130` (Routes, add GoDaddy routes)
- Modify: `modules/domains/module.go:244-287` (MenuItems, add GoDaddy menu)
- Modify: `modules/domains/module.go:337-422` (reconfigureAll, add GoDaddy)

**Interfaces:**
- Consumes: `GoDaddyService`, `GoDaddyHandler`
- Produces: `/user/domains/godaddy` routes, menu item, reconfigureAll integration

- [ ] **Step 1: Add service and handler fields to Module struct**

In `module.go`, add after line 37 (`namesiloHandler *handlers.NameSiloHandler`):

```go
	godaddyService *services.GoDaddyService
	godaddyHandler *handlers.GoDaddyHandler
```

- [ ] **Step 2: Initialize service and handler in Init function**

In `module.go`, add after line 65 (`m.cloudflareHandler = handlers.NewCloudflareHandler(...)`):

```go
	m.godaddyService = services.NewGoDaddyService(deps.DB, m.service, verifyService)
	m.godaddyHandler = handlers.NewGoDaddyHandler(m.godaddyService, deps.Templates)
```

- [ ] **Step 3: Add GoDaddy routes**

In `module.go`, in the `Routes()` function, add in the user routes section (around line 115, after the cloudflare route):

```go
	// GoDaddy
	r.Get("/godaddy", m.godaddyHandler.Page)
	r.Route("/godaddy/api", func(r chi.Router) {
		r.Post("/connect", m.godaddyHandler.Connect)
		r.Post("/disconnect", m.godaddyHandler.Disconnect)
		r.Get("/domains", m.godaddyHandler.ListDomains)
		r.Post("/import", m.godaddyHandler.Import)
		r.Post("/reconfigure/{id}", m.godaddyHandler.ReconfigureDomain)
	})
```

And in the admin API routes section (around line 230):

```go
			// GoDaddy admin operations
			r.Post("/godaddy/reconfigure-all", m.godaddyHandler.AdminReconfigureAll)
```

- [ ] **Step 4: Add GoDaddy menu item**

In `module.go`, in the `MenuItems()` function (around line 277), add after the Cloudflare menu item:

```go
		{
			Title:   "GoDaddy",
			Icon:    "bi-globe2",
			Path:    "/user/domains/godaddy",
			Order:   39,
			Section: module.MenuSectionUser,
			Group:   "Infrastructure",
		},
```

- [ ] **Step 5: Update reconfigureAll to include GoDaddy**

In `module.go`, in the `reconfigureAll()` function, add after the Cloudflare section (around line 419):

```go
	// Reconfigure GoDaddy domains
	godaddyResults, err := m.godaddyService.ReconfigureAllDomains(context.Background(), serverIP)
	if err != nil {
		response["godaddy"] = map[string]interface{}{
			"error": err.Error(),
		}
	} else {
		gdSuccess, gdFail := 0, 0
		for _, r := range godaddyResults {
			if r.Success {
				gdSuccess++
			} else {
				gdFail++
			}
		}
		response["godaddy"] = sourceResults{
			Total:   len(godaddyResults),
			Success: gdSuccess,
			Fail:    gdFail,
			Results: godaddyResults,
		}
	}
```

Also update `ReconfigureAllDomainsWithIP` (around line 430) to include godaddy in the check:

```go
	for _, k := range []string{"namesilo", "cpanel", "cloudflare", "godaddy"} {
```

- [ ] **Step 6: Verify everything compiles**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 7: Commit**

```bash
git add modules/domains/module.go
git commit -m "feat(godaddy): register service, handler, routes, menu, reconfigure"
```

---

### Task 10: Translations

**Files:**
- Modify: `pkg/i18n/locales/en.json`
- Modify: `pkg/i18n/locales/ru.json`

**Interfaces:**
- Consumes: None
- Produces: `menu.connect_godaddy` translation key

- [ ] **Step 1: Add English translation**

In `en.json`, find the menu section and add:

```json
"menu.connect_godaddy": "GoDaddy",
```

- [ ] **Step 2: Add Russian translation**

In `ru.json`, find the menu section and add:

```json
"menu.connect_godaddy": "GoDaddy",
```

- [ ] **Step 3: Commit**

```bash
git add pkg/i18n/locales/en.json pkg/i18n/locales/ru.json
git commit -m "feat(godaddy): add translation keys"
```

---

### Task 11: End-to-End Testing

**Files:**
- None (manual testing)

- [ ] **Step 1: Run the application**

```bash
go run ./cmd/server
```

- [ ] **Step 2: Test connection flow**

1. Navigate to `/user/domains/godaddy`
2. Enter test API key and secret
3. Click Connect
4. Verify connection success or error message

- [ ] **Step 3: Test domain listing**

1. After connecting, verify domains appear in the list
2. Check domain status badges display correctly

- [ ] **Step 4: Test import flow**

1. Click Import on an unimported domain
2. Verify modal appears with checkbox
3. Click Import
4. Verify domain imports and DNS setup starts

- [ ] **Step 5: Test reconfigure**

1. Click Reconfigure on an imported domain
2. Verify success message

- [ ] **Step 6: Test admin reconfigure-all**

Navigate to admin area and trigger reconfigure-all, verify GoDaddy domains are included in results.

- [ ] **Step 7: Final commit**

```bash
git add -A
git commit -m "feat(godaddy): complete integration with DNS, SSL, nginx"
```

---

## Self-Review Checklist

- [x] All spec requirements covered
- [x] No placeholder text
- [x] Type consistency across tasks (GoDaddyService, GoDaddyHandler, GoDaddyConnection, GoDaddyDomain)
- [x] Deploy IP from `domainService.GetDeployIP()` ✓
- [x] ReconfigureAllDomains integration ✓
- [x] Wildcard SSL generation ✓
- [x] nginx config creation ✓
- [x] UI matches Cloudflare pattern ✓
