# Cloudflare Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add "Connect Cloudflare" functionality matching the existing Namesilo integration, allowing users to connect their Cloudflare account via Global API Key, list zones, import domains, and auto-configure DNS.

**Architecture:** Mirror the Namesilo pattern exactly. API client in `pkg/providers/cloudflare/`, service layer in `modules/domains/services/`, handlers in `modules/domains/handlers/`, template in `modules/domains/templates/`. User credentials stored encrypted in `users` table. Cloudflare uses X-Auth-Email + X-Auth-Key headers for authentication.

**Tech Stack:** Go 1.21+, Chi router, sqlx, AES-256-GCM encryption for API keys, Cloudflare API v4

## Global Constraints

- Follow existing Namesilo pattern exactly for consistency
- API keys encrypted at rest using AES-256-GCM with SESSION_SECRET
- Rate limiting: respect Cloudflare's 1200 requests/5 minutes limit
- All DNS records created with proxied=false (direct connection to GuardBot IP)
- TTL=1 (automatic) for Cloudflare-managed records

---

## File Structure

| File | Purpose |
|------|---------|
| `pkg/providers/cloudflare/client.go` | HTTP client with auth headers, rate limiting |
| `pkg/providers/cloudflare/zones.go` | List/get zones (domains) |
| `pkg/providers/cloudflare/dns.go` | DNS record CRUD operations |
| `modules/domains/models/cloudflare.go` | Data models for Cloudflare entities |
| `modules/domains/services/cloudflare_service.go` | Business logic, encryption, DB access |
| `modules/domains/handlers/cloudflare_handlers.go` | HTTP handlers for all endpoints |
| `modules/domains/templates/cloudflare.html` | UI template matching Namesilo style |
| `modules/domains/migrations/012_cloudflare.sql` | Database migration |
| `modules/domains/module.go` | Wire up routes and services (modify) |

---

### Task 1: Database Migration

**Files:**
- Create: `modules/domains/migrations/012_cloudflare.sql`

**Interfaces:**
- Produces: `users.cloudflare_email`, `users.cloudflare_api_key_encrypted`, `users.cloudflare_connected_at` columns

- [ ] **Step 1: Create migration file**

```sql
-- Cloudflare integration
-- Store email + API key per user (single connection per user)

ALTER TABLE users ADD COLUMN IF NOT EXISTS cloudflare_email VARCHAR(255);
ALTER TABLE users ADD COLUMN IF NOT EXISTS cloudflare_api_key_encrypted TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS cloudflare_connected_at TIMESTAMPTZ;

-- Index for finding connected users
CREATE INDEX IF NOT EXISTS idx_users_cloudflare_connected ON users(cloudflare_connected_at) WHERE cloudflare_connected_at IS NOT NULL;
```

- [ ] **Step 2: Verify migration syntax**

Run: `psql $DATABASE_URL -c "\d users" | grep -E "cloudflare|namesilo"`

Expected: Shows both namesilo and cloudflare columns after migration runs.

- [ ] **Step 3: Commit**

```bash
git add modules/domains/migrations/012_cloudflare.sql
git commit -m "feat(cloudflare): add database migration for user credentials"
```

---

### Task 2: Cloudflare API Client

**Files:**
- Create: `pkg/providers/cloudflare/client.go`

**Interfaces:**
- Produces: `cloudflare.Client` struct, `NewClient(email, apiKey string) *Client`, `Client.doRequest(ctx, method, path, body) ([]byte, error)`

- [ ] **Step 1: Create client.go with basic structure**

```go
package cloudflare

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
	defaultBaseURL     = "https://api.cloudflare.com/client/v4"
	minRequestInterval = 250 * time.Millisecond // 1200 req/5min = ~4 req/sec
)

// Client is the Cloudflare API client
type Client struct {
	email      string
	apiKey     string
	baseURL    string
	httpClient *http.Client

	// Rate limiting
	mu          sync.Mutex
	lastRequest time.Time
}

// NewClient creates a new Cloudflare API client
func NewClient(email, apiKey string) *Client {
	return &Client{
		email:   email,
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetBaseURL sets a custom base URL (for testing)
func (c *Client) SetBaseURL(url string) {
	c.baseURL = url
}

// APIResponse is the wrapper for all Cloudflare API responses
type APIResponse struct {
	Success  bool            `json:"success"`
	Errors   []APIError      `json:"errors"`
	Messages []string        `json:"messages"`
	Result   json.RawMessage `json:"result"`
}

// APIError represents a Cloudflare API error
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("cloudflare: %s (code %d)", e.Message, e.Code)
}

// IsInvalidAuth returns true if the error indicates invalid credentials
func (e *APIError) IsInvalidAuth() bool {
	return e.Code == 9103 || e.Code == 10000
}

// doRequest makes a rate-limited request to the Cloudflare API
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

	// Set headers
	req.Header.Set("X-Auth-Email", c.email)
	req.Header.Set("X-Auth-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

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

	return respBody, nil
}

// checkResponse parses the response and checks for API errors
func checkResponse(body []byte) (*APIResponse, error) {
	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	if !resp.Success && len(resp.Errors) > 0 {
		return &resp, &resp.Errors[0]
	}

	return &resp, nil
}
```

- [ ] **Step 2: Test compilation**

Run: `go build ./pkg/providers/cloudflare/...`

Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add pkg/providers/cloudflare/client.go
git commit -m "feat(cloudflare): add API client with auth and rate limiting"
```

---

### Task 3: Zones API (List Domains)

**Files:**
- Create: `pkg/providers/cloudflare/zones.go`

**Interfaces:**
- Consumes: `Client.doRequest`
- Produces: `Zone` struct, `Client.ListZones(ctx) ([]Zone, error)`, `Client.GetZone(ctx, zoneID) (*Zone, error)`

- [ ] **Step 1: Create zones.go**

```go
package cloudflare

import (
	"context"
	"encoding/json"
	"fmt"
)

// Zone represents a Cloudflare zone (domain)
type Zone struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Status          string   `json:"status"` // active, pending, initializing, moved, deleted
	Paused          bool     `json:"paused"`
	Type            string   `json:"type"` // full, partial
	NameServers     []string `json:"name_servers"`
	OriginalNS      []string `json:"original_name_servers"`
	ActivatedOn     string   `json:"activated_on"`
	ModifiedOn      string   `json:"modified_on"`
	DevelopmentMode int      `json:"development_mode"`
}

// zonesResponse is the response from listing zones
type zonesResponse struct {
	Result     []Zone `json:"result"`
	ResultInfo struct {
		Page       int `json:"page"`
		PerPage    int `json:"per_page"`
		TotalPages int `json:"total_pages"`
		Count      int `json:"count"`
		TotalCount int `json:"total_count"`
	} `json:"result_info"`
}

// ListZones returns all zones in the account
func (c *Client) ListZones(ctx context.Context) ([]Zone, error) {
	var allZones []Zone
	page := 1
	perPage := 50

	for {
		path := fmt.Sprintf("/zones?page=%d&per_page=%d", page, perPage)
		body, err := c.doRequest(ctx, "GET", path, nil)
		if err != nil {
			return nil, err
		}

		_, err = checkResponse(body)
		if err != nil {
			return nil, err
		}

		var resp zonesResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("parsing zones response: %w", err)
		}

		allZones = append(allZones, resp.Result...)

		// Check if more pages
		if page >= resp.ResultInfo.TotalPages {
			break
		}
		page++
	}

	return allZones, nil
}

// GetZone returns details for a specific zone
func (c *Client) GetZone(ctx context.Context, zoneID string) (*Zone, error) {
	path := fmt.Sprintf("/zones/%s", zoneID)
	body, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	_, err = checkResponse(body)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Result Zone `json:"result"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing zone response: %w", err)
	}

	return &resp.Result, nil
}

// VerifyCredentials checks if the API credentials are valid by listing zones
func (c *Client) VerifyCredentials(ctx context.Context) error {
	path := "/zones?page=1&per_page=1"
	body, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return err
	}

	_, err = checkResponse(body)
	return err
}
```

- [ ] **Step 2: Test compilation**

Run: `go build ./pkg/providers/cloudflare/...`

Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add pkg/providers/cloudflare/zones.go
git commit -m "feat(cloudflare): add zones API for listing domains"
```

---

### Task 4: DNS Records API

**Files:**
- Create: `pkg/providers/cloudflare/dns.go`

**Interfaces:**
- Consumes: `Client.doRequest`
- Produces: `DNSRecord`, `DNSRecordInput`, `Client.ListDNSRecords`, `Client.AddDNSRecord`, `Client.UpdateDNSRecord`, `Client.DeleteDNSRecord`, `Client.SetupDNS`, `Client.AddVerificationTXT`, `Client.AddAcmeDNSCname`

- [ ] **Step 1: Create dns.go**

```go
package cloudflare

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// DNSRecord represents a DNS record
type DNSRecord struct {
	ID       string `json:"id"`
	ZoneID   string `json:"zone_id"`
	ZoneName string `json:"zone_name"`
	Name     string `json:"name"`    // full name: subdomain.domain.com
	Type     string `json:"type"`    // A, AAAA, CNAME, TXT, MX
	Content  string `json:"content"` // value (IP, target, etc.)
	TTL      int    `json:"ttl"`     // 1 = automatic
	Proxied  bool   `json:"proxied"` // orange cloud on/off
	Priority int    `json:"priority,omitempty"` // MX priority
}

// DNSRecordInput is the input for adding/updating a DNS record
type DNSRecordInput struct {
	Type     string `json:"type"`
	Name     string `json:"name"`    // subdomain or @ or full name
	Content  string `json:"content"` // IP or target
	TTL      int    `json:"ttl"`     // 1 = automatic
	Proxied  bool   `json:"proxied"`
	Priority int    `json:"priority,omitempty"`
}

// ListDNSRecords returns all DNS records for a zone
func (c *Client) ListDNSRecords(ctx context.Context, zoneID string) ([]DNSRecord, error) {
	var allRecords []DNSRecord
	page := 1
	perPage := 100

	for {
		path := fmt.Sprintf("/zones/%s/dns_records?page=%d&per_page=%d", zoneID, page, perPage)
		body, err := c.doRequest(ctx, "GET", path, nil)
		if err != nil {
			return nil, err
		}

		_, err = checkResponse(body)
		if err != nil {
			return nil, err
		}

		var resp struct {
			Result     []DNSRecord `json:"result"`
			ResultInfo struct {
				Page       int `json:"page"`
				TotalPages int `json:"total_pages"`
			} `json:"result_info"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("parsing DNS records response: %w", err)
		}

		allRecords = append(allRecords, resp.Result...)

		if page >= resp.ResultInfo.TotalPages {
			break
		}
		page++
	}

	return allRecords, nil
}

// AddDNSRecord creates a new DNS record
func (c *Client) AddDNSRecord(ctx context.Context, zoneID string, input DNSRecordInput) (string, error) {
	path := fmt.Sprintf("/zones/%s/dns_records", zoneID)
	
	body, err := c.doRequest(ctx, "POST", path, input)
	if err != nil {
		return "", err
	}

	resp, err := checkResponse(body)
	if err != nil {
		return "", err
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("parsing create response: %w", err)
	}

	return result.ID, nil
}

// UpdateDNSRecord updates an existing DNS record
func (c *Client) UpdateDNSRecord(ctx context.Context, zoneID, recordID string, input DNSRecordInput) error {
	path := fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, recordID)
	
	body, err := c.doRequest(ctx, "PATCH", path, input)
	if err != nil {
		return err
	}

	_, err = checkResponse(body)
	return err
}

// DeleteDNSRecord deletes a DNS record
func (c *Client) DeleteDNSRecord(ctx context.Context, zoneID, recordID string) error {
	path := fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, recordID)
	
	body, err := c.doRequest(ctx, "DELETE", path, nil)
	if err != nil {
		return err
	}

	_, err = checkResponse(body)
	return err
}

// SetupDNS configures A records for root (@) and wildcard (*)
// Returns the record IDs created
func (c *Client) SetupDNS(ctx context.Context, zoneID, zoneName, serverIP string) (rootID, wildcardID string, err error) {
	// First, list existing records to check for conflicts
	records, err := c.ListDNSRecords(ctx, zoneID)
	if err != nil {
		return "", "", fmt.Errorf("listing DNS records: %w", err)
	}

	// Find and delete existing A records for @ and *
	for _, r := range records {
		if r.Type == "A" {
			// Root record: name equals zone name
			if r.Name == zoneName {
				if err := c.DeleteDNSRecord(ctx, zoneID, r.ID); err != nil {
					return "", "", fmt.Errorf("deleting existing root A record: %w", err)
				}
			}
			// Wildcard record: name is *.zonename
			if r.Name == "*."+zoneName {
				if err := c.DeleteDNSRecord(ctx, zoneID, r.ID); err != nil {
					return "", "", fmt.Errorf("deleting existing wildcard A record: %w", err)
				}
			}
		}
	}

	// Add root A record (@)
	rootID, err = c.AddDNSRecord(ctx, zoneID, DNSRecordInput{
		Type:    "A",
		Name:    "@",
		Content: serverIP,
		TTL:     1, // automatic
		Proxied: false,
	})
	if err != nil {
		return "", "", fmt.Errorf("adding root A record: %w", err)
	}

	// Add wildcard A record (*)
	wildcardID, err = c.AddDNSRecord(ctx, zoneID, DNSRecordInput{
		Type:    "A",
		Name:    "*",
		Content: serverIP,
		TTL:     1,
		Proxied: false,
	})
	if err != nil {
		return rootID, "", fmt.Errorf("adding wildcard A record: %w", err)
	}

	return rootID, wildcardID, nil
}

// AddVerificationTXT adds the GuardBot verification TXT record
// Creates: _guardbot-verify.domain.com → token
func (c *Client) AddVerificationTXT(ctx context.Context, zoneID, zoneName, token string) (string, error) {
	// First check if it already exists and delete it
	records, err := c.ListDNSRecords(ctx, zoneID)
	if err == nil {
		for _, r := range records {
			if r.Type == "TXT" && r.Name == "_guardbot-verify."+zoneName {
				c.DeleteDNSRecord(ctx, zoneID, r.ID)
			}
		}
	}

	return c.AddDNSRecord(ctx, zoneID, DNSRecordInput{
		Type:    "TXT",
		Name:    "_guardbot-verify",
		Content: token,
		TTL:     1,
		Proxied: false,
	})
}

// RemoveVerificationTXT removes the GuardBot verification TXT record
func (c *Client) RemoveVerificationTXT(ctx context.Context, zoneID, zoneName string) error {
	records, err := c.ListDNSRecords(ctx, zoneID)
	if err != nil {
		return err
	}

	for _, r := range records {
		if r.Type == "TXT" && r.Name == "_guardbot-verify."+zoneName {
			return c.DeleteDNSRecord(ctx, zoneID, r.ID)
		}
	}

	return nil // Not found, nothing to delete
}

// AddAcmeDNSCname adds a CNAME record for acme-dns delegation
// Creates: _acme-challenge.domain.com → fulldomain (e.g., xyz.acme.pamach.xyz)
func (c *Client) AddAcmeDNSCname(ctx context.Context, zoneID, zoneName, fulldomain string) (string, error) {
	// First check if it already exists and delete it
	records, err := c.ListDNSRecords(ctx, zoneID)
	if err == nil {
		for _, r := range records {
			if r.Name == "_acme-challenge."+zoneName {
				c.DeleteDNSRecord(ctx, zoneID, r.ID)
			}
		}
	}

	// Remove trailing dot if present
	target := strings.TrimSuffix(fulldomain, ".")

	return c.AddDNSRecord(ctx, zoneID, DNSRecordInput{
		Type:    "CNAME",
		Name:    "_acme-challenge",
		Content: target,
		TTL:     1,
		Proxied: false,
	})
}
```

- [ ] **Step 2: Test compilation**

Run: `go build ./pkg/providers/cloudflare/...`

Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add pkg/providers/cloudflare/dns.go
git commit -m "feat(cloudflare): add DNS records API with SetupDNS helper"
```

---

### Task 5: Cloudflare Models

**Files:**
- Create: `modules/domains/models/cloudflare.go`

**Interfaces:**
- Produces: `CloudflareConnection`, `CloudflareZone`, `CloudflareDNSRecord`, `CloudflareImportInput`, `CloudflareDNSSetupInput`

- [ ] **Step 1: Create cloudflare.go**

```go
package models

import "time"

// CloudflareConnection represents user's Cloudflare API connection
type CloudflareConnection struct {
	UserID      string     `json:"userId"`
	Email       string     `json:"email"`
	APIKey      string     `json:"-"` // decrypted at runtime, never in JSON
	ConnectedAt *time.Time `json:"connectedAt,omitempty"`
	IsConnected bool       `json:"isConnected"`
}

// CloudflareZone represents a zone from the Cloudflare API
type CloudflareZone struct {
	ZoneID      string   `json:"zoneId"`
	Domain      string   `json:"domain"`
	Status      string   `json:"status"` // active, pending, etc.
	NameServers []string `json:"nameServers"`
	IsImported  bool     `json:"isImported"` // already in our system
	LocalID     string   `json:"localId,omitempty"` // our domain ID if imported
}

// CloudflareDNSRecord represents a DNS record from Cloudflare
type CloudflareDNSRecord struct {
	RecordID string `json:"recordId"`
	Type     string `json:"type"`    // A, AAAA, CNAME, MX, TXT
	Name     string `json:"name"`    // full name (subdomain.domain.com)
	Content  string `json:"content"` // IP address or target
	TTL      int    `json:"ttl"`     // 1 = automatic
	Proxied  bool   `json:"proxied"` // orange cloud
}

// CloudflareImportInput is the input for importing a domain
type CloudflareImportInput struct {
	ZoneID       string `json:"zoneId" validate:"required"`
	Domain       string `json:"domain" validate:"required"`
	ConfigureDNS bool   `json:"configureDns"` // auto-configure DNS records
}

// CloudflareDNSSetupInput is the input for setting up DNS
type CloudflareDNSSetupInput struct {
	ZoneID   string `json:"zoneId" validate:"required"`
	Domain   string `json:"domain" validate:"required"`
	ServerIP string `json:"serverIp,omitempty"` // optional, auto-detect if empty
}
```

- [ ] **Step 2: Test compilation**

Run: `go build ./modules/domains/...`

Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add modules/domains/models/cloudflare.go
git commit -m "feat(cloudflare): add data models"
```

---

### Task 6: Cloudflare Service

**Files:**
- Create: `modules/domains/services/cloudflare_service.go`

**Interfaces:**
- Consumes: `cloudflare.Client`, `models.CloudflareConnection`, `models.CloudflareZone`
- Produces: `CloudflareService`, `NewCloudflareService`, all service methods

- [ ] **Step 1: Create cloudflare_service.go**

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
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/botginx/botginx/modules/domains/models"
	"github.com/botginx/botginx/pkg/acmedns"
	"github.com/botginx/botginx/pkg/providers/cloudflare"
	"github.com/botginx/botginx/pkg/sshexec"
	"github.com/jmoiron/sqlx"
)

// CloudflareService handles Cloudflare integration
type CloudflareService struct {
	db            *sqlx.DB
	encryptionKey []byte
	domainService *DomainService
}

// NewCloudflareService creates a new Cloudflare service
func NewCloudflareService(db *sqlx.DB, domainService *DomainService) *CloudflareService {
	key := os.Getenv("CLOUDFLARE_ENCRYPTION_KEY")
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

	return &CloudflareService{
		db:            db,
		encryptionKey: keyBytes,
		domainService: domainService,
	}
}

// encrypt encrypts data using AES-256-GCM
func (s *CloudflareService) encrypt(plaintext string) (string, error) {
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
func (s *CloudflareService) decrypt(ciphertext string) (string, error) {
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

// GetConnection returns the user's Cloudflare connection status
func (s *CloudflareService) GetConnection(userID string) (*models.CloudflareConnection, error) {
	var email sql.NullString
	var encryptedKey sql.NullString
	var connectedAt sql.NullTime

	err := s.db.QueryRow(`
		SELECT cloudflare_email, cloudflare_api_key_encrypted, cloudflare_connected_at
		FROM users WHERE id = $1
	`, userID).Scan(&email, &encryptedKey, &connectedAt)
	if err != nil {
		return nil, err
	}

	conn := &models.CloudflareConnection{
		UserID:      userID,
		IsConnected: email.Valid && email.String != "" && encryptedKey.Valid && encryptedKey.String != "",
	}

	if email.Valid {
		conn.Email = email.String
	}
	if connectedAt.Valid {
		conn.ConnectedAt = &connectedAt.Time
	}

	return conn, nil
}

// Connect validates and saves the Cloudflare credentials
func (s *CloudflareService) Connect(ctx context.Context, userID, email, apiKey string) error {
	// Validate credentials by making a test request
	client := cloudflare.NewClient(email, apiKey)
	if err := client.VerifyCredentials(ctx); err != nil {
		return fmt.Errorf("invalid credentials: %w", err)
	}

	// Encrypt API key
	encryptedKey, err := s.encrypt(apiKey)
	if err != nil {
		return fmt.Errorf("encrypting API key: %w", err)
	}

	// Save to database
	_, err = s.db.Exec(`
		UPDATE users 
		SET cloudflare_email = $1, 
		    cloudflare_api_key_encrypted = $2, 
		    cloudflare_connected_at = NOW()
		WHERE id = $3
	`, email, encryptedKey, userID)
	if err != nil {
		return fmt.Errorf("saving credentials: %w", err)
	}

	return nil
}

// Disconnect removes the Cloudflare credentials
func (s *CloudflareService) Disconnect(userID string) error {
	_, err := s.db.Exec(`
		UPDATE users 
		SET cloudflare_email = NULL, 
		    cloudflare_api_key_encrypted = NULL, 
		    cloudflare_connected_at = NULL
		WHERE id = $1
	`, userID)
	return err
}

// getClient creates an authenticated Cloudflare client for the user
func (s *CloudflareService) getClient(userID string) (*cloudflare.Client, error) {
	var email sql.NullString
	var encryptedKey sql.NullString

	err := s.db.QueryRow(`
		SELECT cloudflare_email, cloudflare_api_key_encrypted
		FROM users WHERE id = $1
	`, userID).Scan(&email, &encryptedKey)
	if err != nil {
		return nil, err
	}

	if !email.Valid || !encryptedKey.Valid {
		return nil, errors.New("cloudflare not connected")
	}

	apiKey, err := s.decrypt(encryptedKey.String)
	if err != nil {
		return nil, fmt.Errorf("decrypting API key: %w", err)
	}

	return cloudflare.NewClient(email.String, apiKey), nil
}

// ListZones returns all zones from the user's Cloudflare account
func (s *CloudflareService) ListZones(ctx context.Context, userID string) ([]models.CloudflareZone, error) {
	client, err := s.getClient(userID)
	if err != nil {
		return nil, err
	}

	cfZones, err := client.ListZones(ctx)
	if err != nil {
		return nil, err
	}

	// Get list of already-imported domains
	importedDomains := make(map[string]string)
	rows, err := s.db.Query(`
		SELECT name, id FROM domains 
		WHERE user_id = $1 AND source = 'cloudflare'
	`, userID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var name, id string
			rows.Scan(&name, &id)
			importedDomains[name] = id
		}
	}

	zones := make([]models.CloudflareZone, len(cfZones))
	for i, z := range cfZones {
		zones[i] = models.CloudflareZone{
			ZoneID:      z.ID,
			Domain:      z.Name,
			Status:      z.Status,
			NameServers: z.NameServers,
		}
		if localID, ok := importedDomains[z.Name]; ok {
			zones[i].IsImported = true
			zones[i].LocalID = localID
		}
	}

	return zones, nil
}

// ListDNSRecords returns DNS records for a zone
func (s *CloudflareService) ListDNSRecords(ctx context.Context, userID, zoneID string) ([]models.CloudflareDNSRecord, error) {
	client, err := s.getClient(userID)
	if err != nil {
		return nil, err
	}

	cfRecords, err := client.ListDNSRecords(ctx, zoneID)
	if err != nil {
		return nil, err
	}

	records := make([]models.CloudflareDNSRecord, len(cfRecords))
	for i, r := range cfRecords {
		records[i] = models.CloudflareDNSRecord{
			RecordID: r.ID,
			Type:     r.Type,
			Name:     r.Name,
			Content:  r.Content,
			TTL:      r.TTL,
			Proxied:  r.Proxied,
		}
	}

	return records, nil
}

// Import imports a Cloudflare zone as a GuardBot domain
func (s *CloudflareService) Import(ctx context.Context, userID string, input models.CloudflareImportInput) (*models.Domain, error) {
	client, err := s.getClient(userID)
	if err != nil {
		return nil, err
	}

	// Verify zone exists and get details
	zone, err := client.GetZone(ctx, input.ZoneID)
	if err != nil {
		return nil, fmt.Errorf("getting zone: %w", err)
	}

	// Check if domain already exists
	var existingID string
	err = s.db.QueryRow(`SELECT id FROM domains WHERE name = $1 AND user_id = $2`, zone.Name, userID).Scan(&existingID)
	if err == nil {
		return nil, fmt.Errorf("domain %s already exists", zone.Name)
	}

	// Create domain in GuardBot
	domain := &models.Domain{
		UserID:            userID,
		Name:              zone.Name,
		Source:            "cloudflare",
		ProviderDomainID:  input.ZoneID,
		Status:            "pending",
		DNSAutoConfigured: input.ConfigureDNS,
	}

	// Insert domain
	err = s.db.QueryRow(`
		INSERT INTO domains (user_id, name, source, provider_domain_id, status, dns_auto_configured, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		RETURNING id, created_at
	`, domain.UserID, domain.Name, domain.Source, domain.ProviderDomainID, domain.Status, domain.DNSAutoConfigured).
		Scan(&domain.ID, &domain.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating domain: %w", err)
	}

	// Auto-configure DNS if requested
	if input.ConfigureDNS {
		go func() {
			setupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			if err := s.setupDNSInternal(setupCtx, userID, input.ZoneID, zone.Name); err != nil {
				log.Printf("Cloudflare DNS setup failed for %s: %v", zone.Name, err)
				s.db.Exec(`UPDATE domains SET dns_config_error = $1 WHERE id = $2`, err.Error(), domain.ID)
			} else {
				s.db.Exec(`UPDATE domains SET status = 'active', verified_at = NOW() WHERE id = $1`, domain.ID)
			}
		}()
	}

	return domain, nil
}

// SetupDNS configures DNS records for a domain
func (s *CloudflareService) SetupDNS(ctx context.Context, userID, zoneID, domain string) error {
	return s.setupDNSInternal(ctx, userID, zoneID, domain)
}

func (s *CloudflareService) setupDNSInternal(ctx context.Context, userID, zoneID, domain string) error {
	client, err := s.getClient(userID)
	if err != nil {
		return err
	}

	// Get server IP
	serverIP := os.Getenv("GUARDBOT_SERVER_IP")
	if serverIP == "" {
		// Try to get external IP
		resp, err := http.Get("https://api.ipify.org")
		if err == nil {
			defer resp.Body.Close()
			ipBytes, _ := io.ReadAll(resp.Body)
			serverIP = strings.TrimSpace(string(ipBytes))
		}
	}

	if serverIP == "" || net.ParseIP(serverIP) == nil {
		return errors.New("could not determine server IP")
	}

	// Setup A records
	_, _, err = client.SetupDNS(ctx, zoneID, domain, serverIP)
	if err != nil {
		return fmt.Errorf("setting up A records: %w", err)
	}

	// Setup acme-dns CNAME if configured
	acmeDomain := os.Getenv("ACME_DNS_DOMAIN")
	if acmeDomain != "" {
		acmeClient := acmedns.NewClient(acmeDomain)
		reg, err := acmeClient.Register()
		if err == nil {
			_, err = client.AddAcmeDNSCname(ctx, zoneID, domain, reg.Fulldomain)
			if err != nil {
				log.Printf("Warning: failed to add ACME DNS CNAME for %s: %v", domain, err)
			}
		}
	}

	// Update domain status
	s.db.Exec(`
		UPDATE domains 
		SET dns_auto_configured = true, dns_config_error = NULL, status = 'active', verified_at = NOW()
		WHERE name = $1 AND user_id = $2
	`, domain, userID)

	return nil
}

// ReconfigureDomain updates DNS for a single domain
func (s *CloudflareService) ReconfigureDomain(ctx context.Context, userID, domainID string) error {
	var zoneID, domainName string
	err := s.db.QueryRow(`
		SELECT provider_domain_id, name FROM domains 
		WHERE id = $1 AND user_id = $2 AND source = 'cloudflare'
	`, domainID, userID).Scan(&zoneID, &domainName)
	if err != nil {
		return fmt.Errorf("domain not found: %w", err)
	}

	return s.setupDNSInternal(ctx, userID, zoneID, domainName)
}

// ReconfigureAllDomains updates DNS for all Cloudflare domains
func (s *CloudflareService) ReconfigureAllDomains(ctx context.Context, serverIP string) ([]ReconfigureResult, error) {
	var results []ReconfigureResult

	rows, err := s.db.Query(`
		SELECT d.id, d.name, d.provider_domain_id, d.user_id
		FROM domains d
		WHERE d.source = 'cloudflare' AND d.provider_domain_id IS NOT NULL
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var domainID, domainName, zoneID, userID string
		if err := rows.Scan(&domainID, &domainName, &zoneID, &userID); err != nil {
			continue
		}

		result := ReconfigureResult{
			DomainID:   domainID,
			DomainName: domainName,
		}

		if err := s.setupDNSInternal(ctx, userID, zoneID, domainName); err != nil {
			result.Success = false
			result.Error = err.Error()
		} else {
			result.Success = true
		}

		results = append(results, result)
	}

	return results, nil
}
```

- [ ] **Step 2: Test compilation**

Run: `go build ./modules/domains/...`

Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add modules/domains/services/cloudflare_service.go
git commit -m "feat(cloudflare): add service layer with encryption and business logic"
```

---

### Task 7: Cloudflare Handlers

**Files:**
- Create: `modules/domains/handlers/cloudflare_handlers.go`

**Interfaces:**
- Consumes: `CloudflareService`, all its methods
- Produces: HTTP handlers for all Cloudflare routes

- [ ] **Step 1: Create cloudflare_handlers.go**

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

// CloudflareHandler handles Cloudflare integration endpoints
type CloudflareHandler struct {
	cloudflareService *services.CloudflareService
	templates         *module.TemplateEngine
}

// NewCloudflareHandler creates a new Cloudflare handler
func NewCloudflareHandler(cloudflareService *services.CloudflareService, templates *module.TemplateEngine) *CloudflareHandler {
	return &CloudflareHandler{
		cloudflareService: cloudflareService,
		templates:         templates,
	}
}

func (h *CloudflareHandler) jsonError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   msg,
	})
}

func (h *CloudflareHandler) jsonSuccess(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    data,
	})
}

// Page renders the Cloudflare connection page
func (h *CloudflareHandler) Page(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	conn, err := h.cloudflareService.GetConnection(userID)
	if err != nil {
		conn = &models.CloudflareConnection{IsConnected: false}
	}

	var zones []models.CloudflareZone
	if conn.IsConnected {
		zones, _ = h.cloudflareService.ListZones(r.Context(), userID)
	}

	module.RenderUserSection(w, r, h.templates, "domains:cloudflare.html", map[string]interface{}{
		"Title":      "Cloudflare",
		"Connection": conn,
		"Zones":      zones,
	})
}

// Connect handles API key connection
func (h *CloudflareHandler) Connect(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var input struct {
		Email  string `json:"email"`
		APIKey string `json:"apiKey"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if input.Email == "" || input.APIKey == "" {
		h.jsonError(w, "Email and API key are required", http.StatusBadRequest)
		return
	}

	if err := h.cloudflareService.Connect(r.Context(), userID, input.Email, input.APIKey); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.jsonSuccess(w, map[string]interface{}{
		"message": "Connected to Cloudflare",
	})
}

// Disconnect removes the API key
func (h *CloudflareHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	if err := h.cloudflareService.Disconnect(userID); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonSuccess(w, map[string]interface{}{
		"message": "Disconnected from Cloudflare",
	})
}

// ListZones returns all zones
func (h *CloudflareHandler) ListZones(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	zones, err := h.cloudflareService.ListZones(r.Context(), userID)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonSuccess(w, zones)
}

// GetDNSRecords returns DNS records for a zone
func (h *CloudflareHandler) GetDNSRecords(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	zoneID := chi.URLParam(r, "zoneID")

	records, err := h.cloudflareService.ListDNSRecords(r.Context(), userID, zoneID)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonSuccess(w, records)
}

// Import imports a domain from Cloudflare
func (h *CloudflareHandler) Import(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var input models.CloudflareImportInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	domain, err := h.cloudflareService.Import(r.Context(), userID, input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.jsonSuccess(w, domain)
}

// SetupDNS configures DNS records for a zone
func (h *CloudflareHandler) SetupDNS(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var input models.CloudflareDNSSetupInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := h.cloudflareService.SetupDNS(r.Context(), userID, input.ZoneID, input.Domain); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonSuccess(w, map[string]interface{}{
		"message": "DNS configured successfully",
	})
}

// Reconfigure updates DNS for a single domain
func (h *CloudflareHandler) ReconfigureDomain(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	domainID := chi.URLParam(r, "id")

	if err := h.cloudflareService.ReconfigureDomain(r.Context(), userID, domainID); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonSuccess(w, map[string]interface{}{
		"message": "DNS reconfigured",
	})
}

// AdminReconfigureAll updates all Cloudflare domains (admin only)
func (h *CloudflareHandler) AdminReconfigureAll(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ServerIP string `json:"serverIp"`
	}
	json.NewDecoder(r.Body).Decode(&input)

	results, err := h.cloudflareService.ReconfigureAllDomains(r.Context(), input.ServerIP)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonSuccess(w, results)
}
```

- [ ] **Step 2: Test compilation**

Run: `go build ./modules/domains/...`

Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add modules/domains/handlers/cloudflare_handlers.go
git commit -m "feat(cloudflare): add HTTP handlers"
```

---

### Task 8: Cloudflare UI Template

**Files:**
- Create: `modules/domains/templates/cloudflare.html`

**Interfaces:**
- Consumes: Template data from `CloudflareHandler.Page`
- Produces: HTML page matching Namesilo UI style

- [ ] **Step 1: Create cloudflare.html**

```html
{{define "title"}}Cloudflare{{end}}

{{define "content"}}
<div class="container-fluid py-4">
    <div class="row">
        <div class="col-12">
            <div class="card">
                <div class="card-header pb-0">
                    <div class="d-flex justify-content-between align-items-center">
                        <div>
                            <h5 class="mb-0">
                                <i class="bi bi-cloud me-2"></i>Cloudflare Integration
                            </h5>
                            <p class="text-sm text-muted mb-0">Connect your Cloudflare account to import domains and auto-configure DNS</p>
                        </div>
                        {{if .Connection.IsConnected}}
                        <span class="badge bg-success">
                            <i class="bi bi-check-circle me-1"></i>Connected
                        </span>
                        {{end}}
                    </div>
                </div>
                <div class="card-body">
                    {{if not .Connection.IsConnected}}
                    <!-- Connection Form -->
                    <div class="row justify-content-center">
                        <div class="col-md-6">
                            <div class="alert alert-info">
                                <h6 class="alert-heading"><i class="bi bi-info-circle me-2"></i>How to get your API Key</h6>
                                <ol class="mb-0 small">
                                    <li>Go to <a href="https://dash.cloudflare.com/profile/api-tokens" target="_blank">Cloudflare Dashboard → My Profile → API Tokens</a></li>
                                    <li>Scroll to "API Keys" section</li>
                                    <li>Click "View" next to "Global API Key"</li>
                                    <li>Enter your password and copy the key</li>
                                </ol>
                            </div>
                            <form id="connectForm">
                                <div class="mb-3">
                                    <label class="form-label">Cloudflare Email</label>
                                    <input type="email" class="form-control" id="email" placeholder="your@email.com" required>
                                </div>
                                <div class="mb-3">
                                    <label class="form-label">Global API Key</label>
                                    <input type="password" class="form-control" id="apiKey" placeholder="Enter your Global API Key" required>
                                </div>
                                <button type="submit" class="btn btn-primary w-100">
                                    <i class="bi bi-plug me-2"></i>Connect Cloudflare
                                </button>
                            </form>
                        </div>
                    </div>
                    {{else}}
                    <!-- Connected State -->
                    <div class="d-flex justify-content-between align-items-center mb-4">
                        <div>
                            <span class="text-muted">Connected as:</span>
                            <strong>{{.Connection.Email}}</strong>
                            {{if .Connection.ConnectedAt}}
                            <small class="text-muted ms-2">since {{.Connection.ConnectedAt.Format "Jan 2, 2006"}}</small>
                            {{end}}
                        </div>
                        <button class="btn btn-outline-danger btn-sm" onclick="disconnect()">
                            <i class="bi bi-x-circle me-1"></i>Disconnect
                        </button>
                    </div>

                    <!-- Zones List -->
                    <h6 class="mb-3">Your Domains ({{len .Zones}})</h6>
                    {{if .Zones}}
                    <div class="table-responsive">
                        <table class="table table-hover align-middle">
                            <thead>
                                <tr>
                                    <th>Domain</th>
                                    <th>Status</th>
                                    <th>Name Servers</th>
                                    <th class="text-end">Actions</th>
                                </tr>
                            </thead>
                            <tbody>
                                {{range .Zones}}
                                <tr>
                                    <td>
                                        <strong>{{.Domain}}</strong>
                                        {{if .IsImported}}
                                        <span class="badge bg-success ms-2">Imported</span>
                                        {{end}}
                                    </td>
                                    <td>
                                        {{if eq .Status "active"}}
                                        <span class="badge bg-success">Active</span>
                                        {{else}}
                                        <span class="badge bg-warning">{{.Status}}</span>
                                        {{end}}
                                    </td>
                                    <td>
                                        <small class="text-muted">
                                            {{range $i, $ns := .NameServers}}{{if $i}}, {{end}}{{$ns}}{{end}}
                                        </small>
                                    </td>
                                    <td class="text-end">
                                        {{if .IsImported}}
                                        <a href="/user/domains/{{.LocalID}}" class="btn btn-sm btn-outline-primary">
                                            <i class="bi bi-eye me-1"></i>View
                                        </a>
                                        <button class="btn btn-sm btn-outline-secondary" onclick="setupDNS('{{.ZoneID}}', '{{.Domain}}')">
                                            <i class="bi bi-arrow-repeat me-1"></i>Reconfigure DNS
                                        </button>
                                        {{else}}
                                        <button class="btn btn-sm btn-primary" onclick="importDomain('{{.ZoneID}}', '{{.Domain}}')">
                                            <i class="bi bi-download me-1"></i>Import
                                        </button>
                                        {{end}}
                                        <button class="btn btn-sm btn-outline-secondary" onclick="viewDNS('{{.ZoneID}}', '{{.Domain}}')">
                                            <i class="bi bi-list me-1"></i>DNS
                                        </button>
                                    </td>
                                </tr>
                                {{end}}
                            </tbody>
                        </table>
                    </div>
                    {{else}}
                    <div class="text-center py-4 text-muted">
                        <i class="bi bi-inbox display-4"></i>
                        <p class="mt-2">No domains found in your Cloudflare account</p>
                    </div>
                    {{end}}
                    {{end}}
                </div>
            </div>
        </div>
    </div>
</div>

<!-- DNS Records Modal -->
<div class="modal fade" id="dnsModal" tabindex="-1">
    <div class="modal-dialog modal-lg">
        <div class="modal-content">
            <div class="modal-header">
                <h5 class="modal-title">DNS Records: <span id="dnsModalDomain"></span></h5>
                <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
            </div>
            <div class="modal-body">
                <div id="dnsRecords"></div>
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
                <p>Import <strong id="importDomain"></strong> to GuardBot?</p>
                <div class="form-check mb-3">
                    <input class="form-check-input" type="checkbox" id="configureDns" checked>
                    <label class="form-check-label" for="configureDns">
                        Auto-configure DNS (recommended)
                        <small class="d-block text-muted">Creates A records pointing to GuardBot server</small>
                    </label>
                </div>
            </div>
            <div class="modal-footer">
                <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">Cancel</button>
                <button type="button" class="btn btn-primary" id="confirmImport">
                    <i class="bi bi-download me-1"></i>Import
                </button>
            </div>
        </div>
    </div>
</div>
{{end}}

{{define "scripts"}}
<script>
let currentZoneID = '';
let currentDomain = '';

document.getElementById('connectForm')?.addEventListener('submit', async (e) => {
    e.preventDefault();
    const btn = e.target.querySelector('button[type="submit"]');
    btn.disabled = true;
    btn.innerHTML = '<span class="spinner-border spinner-border-sm me-2"></span>Connecting...';

    try {
        const resp = await fetch('/user/domains/cloudflare/api/connect', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                email: document.getElementById('email').value,
                apiKey: document.getElementById('apiKey').value
            })
        });
        const data = await resp.json();
        if (data.success) {
            showToast('success', 'Connected to Cloudflare');
            location.reload();
        } else {
            showToast('error', data.error || 'Connection failed');
        }
    } catch (err) {
        showToast('error', 'Connection failed');
    } finally {
        btn.disabled = false;
        btn.innerHTML = '<i class="bi bi-plug me-2"></i>Connect Cloudflare';
    }
});

async function disconnect() {
    if (!confirm('Disconnect from Cloudflare? Your imported domains will remain.')) return;
    
    const resp = await fetch('/user/domains/cloudflare/api/disconnect', {method: 'POST'});
    const data = await resp.json();
    if (data.success) {
        showToast('success', 'Disconnected');
        location.reload();
    } else {
        showToast('error', data.error);
    }
}

function importDomain(zoneID, domain) {
    currentZoneID = zoneID;
    currentDomain = domain;
    document.getElementById('importDomain').textContent = domain;
    new bootstrap.Modal(document.getElementById('importModal')).show();
}

document.getElementById('confirmImport')?.addEventListener('click', async () => {
    const btn = document.getElementById('confirmImport');
    btn.disabled = true;
    btn.innerHTML = '<span class="spinner-border spinner-border-sm me-2"></span>Importing...';

    try {
        const resp = await fetch('/user/domains/cloudflare/api/import', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                zoneId: currentZoneID,
                domain: currentDomain,
                configureDns: document.getElementById('configureDns').checked
            })
        });
        const data = await resp.json();
        if (data.success) {
            showToast('success', 'Domain imported successfully');
            bootstrap.Modal.getInstance(document.getElementById('importModal')).hide();
            location.reload();
        } else {
            showToast('error', data.error);
        }
    } catch (err) {
        showToast('error', 'Import failed');
    } finally {
        btn.disabled = false;
        btn.innerHTML = '<i class="bi bi-download me-1"></i>Import';
    }
});

async function viewDNS(zoneID, domain) {
    document.getElementById('dnsModalDomain').textContent = domain;
    document.getElementById('dnsRecords').innerHTML = '<div class="text-center py-4"><span class="spinner-border"></span></div>';
    new bootstrap.Modal(document.getElementById('dnsModal')).show();

    const resp = await fetch(`/user/domains/cloudflare/api/dns/${zoneID}`);
    const data = await resp.json();

    if (data.success) {
        let html = '<table class="table table-sm"><thead><tr><th>Type</th><th>Name</th><th>Content</th><th>TTL</th><th>Proxied</th></tr></thead><tbody>';
        for (const r of data.data) {
            html += `<tr>
                <td><span class="badge bg-secondary">${r.type}</span></td>
                <td><code>${r.name}</code></td>
                <td><code>${r.content}</code></td>
                <td>${r.ttl === 1 ? 'Auto' : r.ttl}</td>
                <td>${r.proxied ? '<i class="bi bi-cloud-fill text-warning"></i>' : '<i class="bi bi-cloud text-muted"></i>'}</td>
            </tr>`;
        }
        html += '</tbody></table>';
        document.getElementById('dnsRecords').innerHTML = html;
    } else {
        document.getElementById('dnsRecords').innerHTML = `<div class="alert alert-danger">${data.error}</div>`;
    }
}

async function setupDNS(zoneID, domain) {
    if (!confirm(`Reconfigure DNS for ${domain}? This will update A records to point to GuardBot.`)) return;

    showToast('info', 'Configuring DNS...');
    const resp = await fetch('/user/domains/cloudflare/api/dns/setup', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({zoneId: zoneID, domain: domain})
    });
    const data = await resp.json();
    if (data.success) {
        showToast('success', 'DNS configured successfully');
    } else {
        showToast('error', data.error);
    }
}
</script>
{{end}}
```

- [ ] **Step 2: Verify template syntax**

Run: Template will be validated when module loads.

- [ ] **Step 3: Commit**

```bash
git add modules/domains/templates/cloudflare.html
git commit -m "feat(cloudflare): add UI template"
```

---

### Task 9: Wire Up Routes in Module

**Files:**
- Modify: `modules/domains/module.go`

**Interfaces:**
- Consumes: `CloudflareService`, `CloudflareHandler`
- Produces: Registered routes and sidebar menu item

- [ ] **Step 1: Add imports and fields to Module struct**

Add to imports:
```go
// In the import block, these should already exist from namesilo
```

Add fields to Module struct (after namesiloService and namesiloHandler):
```go
cloudflareService *services.CloudflareService
cloudflareHandler *handlers.CloudflareHandler
```

- [ ] **Step 2: Initialize service and handler in Init()**

Add after `m.namesiloHandler = ...`:
```go
m.cloudflareService = services.NewCloudflareService(deps.DB, m.domainService)
m.cloudflareHandler = handlers.NewCloudflareHandler(m.cloudflareService, deps.Templates)
```

- [ ] **Step 3: Add routes in Routes()**

Add after the `/namesilo` route block:
```go
r.Route("/cloudflare", func(r chi.Router) {
	r.Use(userRequiredMiddleware)
	r.Get("/", m.cloudflareHandler.Page)

	// API endpoints
	r.Post("/api/connect", m.cloudflareHandler.Connect)
	r.Post("/api/disconnect", m.cloudflareHandler.Disconnect)
	r.Get("/api/zones", m.cloudflareHandler.ListZones)
	r.Post("/api/import", m.cloudflareHandler.Import)
	r.Get("/api/dns/{zoneID}", m.cloudflareHandler.GetDNSRecords)
	r.Post("/api/dns/setup", m.cloudflareHandler.SetupDNS)
	r.Post("/api/reconfigure/{id}", m.cloudflareHandler.ReconfigureDomain)
})
```

- [ ] **Step 4: Add admin route**

Add in the admin routes section after namesilo:
```go
r.Post("/cloudflare/reconfigure-all", m.cloudflareHandler.AdminReconfigureAll)
```

- [ ] **Step 5: Add menu item in MenuItems()**

Add after the NameSilo menu item:
```go
{
	Path:    "/user/domains/cloudflare",
	Label:   "Cloudflare",
	Icon:    "cloud",
	Section: "domains",
},
```

- [ ] **Step 6: Update AdminReconfigureAllDomains to include Cloudflare**

In the `AdminReconfigureAllDomains` function (around line 330, after the cPanel block), add Cloudflare:
```go
// Reconfigure Cloudflare domains
cloudflareResults, err := m.cloudflareService.ReconfigureAllDomains(context.Background(), serverIP)
if err != nil {
	response["cloudflare"] = map[string]interface{}{
		"error": err.Error(),
	}
} else {
	cfSuccess, cfFail := 0, 0
	for _, r := range cloudflareResults {
		if r.Success {
			cfSuccess++
		} else {
			cfFail++
		}
	}
	response["cloudflare"] = sourceResults{
		Total:   len(cloudflareResults),
		Success: cfSuccess,
		Fail:    cfFail,
		Results: cloudflareResults,
	}
}
```

This hooks into the existing "Reconfigure All DNS" button that admins use when the deploy VPS IP changes. One button updates Namesilo + cPanel + Cloudflare domains all at once.

- [ ] **Step 7: Update servers template toast to show Cloudflare counts**

In `modules/servers/templates/show.html`, update the reconfigure button's success toast to include Cloudflare:

Find this section:
```javascript
if (nsTotal === 0 && cpTotal === 0) {
    flashToast('No domains to update', 'info');
} else {
    flashToast(`Updated ${nsSuccess + cpSuccess} domains (NameSilo: ${nsSuccess}/${nsTotal}, cPanel: ${cpSuccess}/${cpTotal})`, 'success');
}
```

Replace with:
```javascript
const cf = data.data.cloudflare || {};
const cfTotal = cf.total || 0;
const cfSuccess = cf.success || 0;

if (nsTotal === 0 && cpTotal === 0 && cfTotal === 0) {
    flashToast('No domains to update', 'info');
} else {
    const parts = [];
    if (nsTotal > 0) parts.push(`NameSilo: ${nsSuccess}/${nsTotal}`);
    if (cpTotal > 0) parts.push(`cPanel: ${cpSuccess}/${cpTotal}`);
    if (cfTotal > 0) parts.push(`Cloudflare: ${cfSuccess}/${cfTotal}`);
    flashToast(`Updated ${nsSuccess + cpSuccess + cfSuccess} domains (${parts.join(', ')})`, 'success');
}
```

- [ ] **Step 8: Test compilation**

Run: `go build ./cmd/server/...`

Expected: No errors

- [ ] **Step 9: Commit**

```bash
git add modules/domains/module.go modules/servers/templates/show.html
git commit -m "feat(cloudflare): wire up routes and unified reconfigure button"
```

---

### Task 10: Auto-Trigger DNS Reconfigure on Server IP Change

**Files:**
- Modify: `modules/servers/handlers/handler.go`
- Modify: `modules/servers/module.go`

**Interfaces:**
- Consumes: `domains.Module.AdminReconfigureAllDomains` or a service-level equivalent
- Produces: Auto-trigger when deploy server IP changes

- [ ] **Step 1: Add domains module dependency to servers module**

In `modules/servers/module.go`, add a reference to the domains module's reconfigure function. This requires either:
- A callback function passed during init
- Or making `ReconfigureAllDomains` available as a service method

Option A (callback approach) - in servers Module struct:
```go
onServerIPChange func(serverIP string) error
```

And a setter method:
```go
func (m *Module) SetOnServerIPChange(fn func(string) error) {
    m.onServerIPChange = fn
}
```

- [ ] **Step 2: Update APIUpdate handler to detect IP change and trigger reconfigure**

In `modules/servers/handlers/handler.go`, modify `APIUpdate`:
```go
func (h *Handler) APIUpdate(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")

    // Get current server to detect IP change
    oldServer, err := h.service.Get(id)
    if err != nil {
        h.jsonError(w, err.Error(), http.StatusNotFound)
        return
    }

    var input models.UpdateServerInput
    if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
        h.jsonError(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    server, err := h.service.Update(id, input)
    if err != nil {
        h.jsonError(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // If IP changed and callback is set, trigger DNS reconfigure
    if input.IP != "" && input.IP != oldServer.IP && h.onServerIPChange != nil {
        go func() {
            if err := h.onServerIPChange(server.IP); err != nil {
                log.Printf("[servers] Auto-reconfigure DNS failed: %v", err)
            } else {
                log.Printf("[servers] Auto-reconfigured DNS for new IP: %s", server.IP)
            }
        }()
    }

    h.json(w, http.StatusOK, map[string]interface{}{"server": server})
}
```

- [ ] **Step 3: Wire up the callback in main or module init**

In `cmd/server/main.go` or during module registration, after both modules are initialized:
```go
serversModule.SetOnServerIPChange(func(ip string) error {
    return domainsModule.ReconfigureAllDomainsWithIP(ip)
})
```

Or create a service-level function in domains that servers can call directly.

- [ ] **Step 4: Test auto-trigger**

1. Update a deploy server's IP via the API
2. Check logs for "Auto-reconfigured DNS for new IP"
3. Verify DNS records were updated in Namesilo/cPanel/Cloudflare

- [ ] **Step 5: Commit**

```bash
git add modules/servers/handlers/handler.go modules/servers/module.go
git commit -m "feat: auto-trigger DNS reconfigure when deploy server IP changes"
```

---

### Task 11: Integration Testing

**Files:**
- None (manual testing)

**Interfaces:**
- Consumes: All previous tasks

- [ ] **Step 1: Run migrations**

```bash
go run ./cmd/server migrate
```

Expected: Migration 012_cloudflare.sql runs successfully

- [ ] **Step 2: Start server**

```bash
go run ./cmd/server
```

Expected: Server starts without errors

- [ ] **Step 3: Test connection flow**

1. Go to `/user/domains/cloudflare`
2. Enter Cloudflare email and Global API Key
3. Click Connect
4. Expected: Shows list of zones from Cloudflare account

- [ ] **Step 4: Test import flow**

1. Click "Import" on a domain
2. Check "Auto-configure DNS"
3. Click Import
4. Expected: Domain appears in GuardBot domains list

- [ ] **Step 5: Test DNS viewing**

1. Click "DNS" on any zone
2. Expected: Modal shows DNS records from Cloudflare

- [ ] **Step 6: Test unified reconfigure button**

1. Go to `/admin/servers/{id}` for the deploy server
2. Click "Update All Domain DNS"
3. Expected: Toast shows counts for NameSilo, cPanel, AND Cloudflare

- [ ] **Step 7: Test auto-trigger on IP change**

1. Update the deploy server IP via API
2. Check logs for auto-reconfigure message
3. Verify DNS records updated across all providers

- [ ] **Step 8: Commit final version tag**

```bash
git tag -a v1.0.62 -m "feat: add Cloudflare integration with auto-reconfigure"
git push origin v1.0.62
```

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Database migration | 1 new |
| 2 | API client | 1 new |
| 3 | Zones API | 1 new |
| 4 | DNS API | 1 new |
| 5 | Models | 1 new |
| 6 | Service | 1 new |
| 7 | Handlers | 1 new |
| 8 | Template | 1 new |
| 9 | Module wiring + unified button | 2 modify |
| 10 | Auto-trigger on IP change | 2 modify |
| 11 | Testing | 0 |

**Total: 8 new files, 4 modified files, 11 tasks**
