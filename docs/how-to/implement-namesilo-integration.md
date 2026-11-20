# How to Implement NameSilo Domain Integration

This guide walks through implementing NameSilo domain import with automatic DNS configuration and wildcard SSL generation. Follow these steps in order.

## Prerequisites

- Go application with `sqlx` database access
- SSH access to a deploy server (for SSL generation)
- PostgreSQL database
- `github.com/go-chi/chi/v5` for routing (or adapt to your router)

## Step 1: Create the Database Migration

Create migration file `migrations/XXX_namesilo.sql`:

```sql
-- Users table additions (store API key per user)
ALTER TABLE users ADD COLUMN IF NOT EXISTS namesilo_api_key_encrypted TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS namesilo_connected_at TIMESTAMPTZ;

-- Domains table additions
ALTER TABLE domains ADD COLUMN IF NOT EXISTS source VARCHAR(50) DEFAULT 'manual';
ALTER TABLE domains ADD COLUMN IF NOT EXISTS provider_domain_id VARCHAR(255);
ALTER TABLE domains ADD COLUMN IF NOT EXISTS dns_auto_configured BOOLEAN DEFAULT FALSE;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS dns_config_error TEXT;

-- acme-dns credentials for instant SSL
ALTER TABLE domains ADD COLUMN IF NOT EXISTS acme_subdomain VARCHAR(255);
ALTER TABLE domains ADD COLUMN IF NOT EXISTS acme_username VARCHAR(255);
ALTER TABLE domains ADD COLUMN IF NOT EXISTS acme_password VARCHAR(255);
ALTER TABLE domains ADD COLUMN IF NOT EXISTS acme_fulldomain VARCHAR(255);
ALTER TABLE domains ADD COLUMN IF NOT EXISTS acme_cname_verified BOOLEAN DEFAULT FALSE;

-- Indexes
CREATE INDEX IF NOT EXISTS idx_domains_source ON domains(source);
```

Run the migration.

## Step 2: Create the NameSilo API Client Package

Create `pkg/providers/namesilo/` with three files.

### 2.1 Create `client.go`

This handles authentication and rate limiting:

```go
package namesilo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const (
	defaultBaseURL     = "https://www.namesilo.com/api"
	apiVersion         = "1"
	responseType       = "json"
	minRequestInterval = 2 * time.Second // NameSilo rate limit
)

type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	mu         sync.Mutex
	lastRequest time.Time
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) doRequest(ctx context.Context, operation string, params url.Values) ([]byte, error) {
	// Rate limiting
	c.mu.Lock()
	if elapsed := time.Since(c.lastRequest); elapsed < minRequestInterval {
		time.Sleep(minRequestInterval - elapsed)
	}
	c.lastRequest = time.Now()
	c.mu.Unlock()

	if params == nil {
		params = url.Values{}
	}
	params.Set("version", apiVersion)
	params.Set("type", responseType)
	params.Set("key", c.apiKey)

	reqURL := fmt.Sprintf("%s/%s?%s", c.baseURL, operation, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

type APIError struct {
	Code    int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("namesilo: %s (code %d)", e.Message, e.Code)
}

func (e *APIError) IsInvalidKey() bool { return e.Code == 110 }
```

### 2.2 Create `domains.go`

Domain listing and info:

```go
package namesilo

import (
	"context"
	"encoding/json"
	"net/url"
	"time"
)

type Domain struct {
	Domain    string     `json:"domain"`
	Created   *time.Time `json:"created,omitempty"`
	Expires   *time.Time `json:"expires,omitempty"`
	Status    string     `json:"status"`
}

func (c *Client) ListDomains(ctx context.Context) ([]string, error) {
	body, err := c.doRequest(ctx, "listDomains", nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Reply struct {
			Code    int         `json:"code"`
			Detail  string      `json:"detail"`
			Domains interface{} `json:"domains"`
		} `json:"reply"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	if resp.Reply.Code != 300 {
		return nil, &APIError{Code: resp.Reply.Code, Message: resp.Reply.Detail}
	}

	// Handle multiple response formats (NameSilo API is inconsistent)
	var domains []string
	switch v := resp.Reply.Domains.(type) {
	case map[string]interface{}:
		if d, ok := v["domain"]; ok {
			switch dd := d.(type) {
			case string:
				domains = []string{dd}
			case []interface{}:
				for _, item := range dd {
					if s, ok := item.(string); ok {
						domains = append(domains, s)
					}
				}
			}
		}
	case []interface{}:
		for _, d := range v {
			if s, ok := d.(string); ok {
				domains = append(domains, s)
			}
		}
	case string:
		domains = []string{v}
	}
	return domains, nil
}

func (c *Client) GetDomainInfo(ctx context.Context, domain string) (*Domain, error) {
	params := url.Values{}
	params.Set("domain", domain)

	body, err := c.doRequest(ctx, "getDomainInfo", params)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Reply struct {
			Code    int    `json:"code"`
			Detail  string `json:"detail"`
			Created string `json:"created"`
			Expires string `json:"expires"`
			Status  string `json:"status"`
		} `json:"reply"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	if resp.Reply.Code != 300 {
		return nil, &APIError{Code: resp.Reply.Code, Message: resp.Reply.Detail}
	}

	return &Domain{Domain: domain, Status: resp.Reply.Status}, nil
}

func (c *Client) TestConnection(ctx context.Context) error {
	_, err := c.ListDomains(ctx)
	return err
}
```

### 2.3 Create `dns.go`

DNS record management:

```go
package namesilo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type DNSRecord struct {
	RecordID string `json:"record_id"`
	Type     string `json:"type"`
	Host     string `json:"host"`
	Value    string `json:"value"`
	TTL      int    `json:"ttl"`
}

func (c *Client) ListDNSRecords(ctx context.Context, domain string) ([]DNSRecord, error) {
	params := url.Values{}
	params.Set("domain", domain)

	body, err := c.doRequest(ctx, "dnsListRecords", params)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Reply struct {
			Code           int         `json:"code"`
			Detail         string      `json:"detail"`
			ResourceRecord interface{} `json:"resource_record"`
		} `json:"reply"`
	}
	json.Unmarshal(body, &resp)

	if resp.Reply.Code != 300 {
		return nil, &APIError{Code: resp.Reply.Code, Message: resp.Reply.Detail}
	}

	var records []DNSRecord
	switch v := resp.Reply.ResourceRecord.(type) {
	case []interface{}:
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				records = append(records, parseRecord(m))
			}
		}
	case map[string]interface{}:
		records = append(records, parseRecord(v))
	}
	return records, nil
}

func parseRecord(m map[string]interface{}) DNSRecord {
	r := DNSRecord{}
	if v, ok := m["record_id"].(string); ok { r.RecordID = v }
	if v, ok := m["type"].(string); ok { r.Type = v }
	if v, ok := m["host"].(string); ok { r.Host = v }
	if v, ok := m["value"].(string); ok { r.Value = v }
	if v, ok := m["ttl"].(float64); ok { r.TTL = int(v) }
	return r
}

func (c *Client) AddDNSRecord(ctx context.Context, domain, rrtype, host, value string, ttl int) (string, error) {
	params := url.Values{}
	params.Set("domain", domain)
	params.Set("rrtype", rrtype)
	params.Set("rrhost", host)
	params.Set("rrvalue", value)
	params.Set("rrttl", fmt.Sprintf("%d", ttl))

	body, err := c.doRequest(ctx, "dnsAddRecord", params)
	if err != nil {
		return "", err
	}

	var resp struct {
		Reply struct {
			Code     int    `json:"code"`
			Detail   string `json:"detail"`
			RecordID string `json:"record_id"`
		} `json:"reply"`
	}
	json.Unmarshal(body, &resp)

	if resp.Reply.Code != 300 && resp.Reply.Code != 301 {
		return "", &APIError{Code: resp.Reply.Code, Message: resp.Reply.Detail}
	}
	return resp.Reply.RecordID, nil
}

func (c *Client) DeleteDNSRecord(ctx context.Context, domain, recordID string) error {
	params := url.Values{}
	params.Set("domain", domain)
	params.Set("rrid", recordID)

	body, err := c.doRequest(ctx, "dnsDeleteRecord", params)
	if err != nil {
		return err
	}

	var resp struct {
		Reply struct {
			Code   int    `json:"code"`
			Detail string `json:"detail"`
		} `json:"reply"`
	}
	json.Unmarshal(body, &resp)

	if resp.Reply.Code != 300 {
		return &APIError{Code: resp.Reply.Code, Message: resp.Reply.Detail}
	}
	return nil
}

// SetupDNS adds @ and * A records pointing to serverIP
func (c *Client) SetupDNS(ctx context.Context, domain, serverIP string) error {
	// Delete existing A records first
	records, _ := c.ListDNSRecords(ctx, domain)
	for _, r := range records {
		if r.Type == "A" && (r.Host == "" || r.Host == "@" || r.Host == "*") {
			c.DeleteDNSRecord(ctx, domain, r.RecordID)
		}
	}

	// Add root A record
	if _, err := c.AddDNSRecord(ctx, domain, "A", "", serverIP, 3600); err != nil {
		return fmt.Errorf("adding root A record: %w", err)
	}

	// Add wildcard A record
	if _, err := c.AddDNSRecord(ctx, domain, "A", "*", serverIP, 3600); err != nil {
		return fmt.Errorf("adding wildcard A record: %w", err)
	}

	return nil
}

// AddAcmeDNSCname adds _acme-challenge CNAME for instant SSL
func (c *Client) AddAcmeDNSCname(ctx context.Context, domain, target string) (string, error) {
	// Delete existing _acme-challenge records
	records, _ := c.ListDNSRecords(ctx, domain)
	for _, r := range records {
		if r.Host == "_acme-challenge" {
			c.DeleteDNSRecord(ctx, domain, r.RecordID)
		}
	}

	target = strings.TrimSuffix(target, ".")
	return c.AddDNSRecord(ctx, domain, "CNAME", "_acme-challenge", target, 3600)
}
```

## Step 3: Create the Service Layer

Create `services/namesilo_service.go`:

```go
package services

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"yourapp/pkg/providers/namesilo"
	"github.com/jmoiron/sqlx"
)

type NameSiloService struct {
	db            *sqlx.DB
	encryptionKey []byte
}

func NewNameSiloService(db *sqlx.DB) *NameSiloService {
	key := os.Getenv("SESSION_SECRET")
	keyBytes := make([]byte, 32)
	copy(keyBytes, []byte(key))
	return &NameSiloService{db: db, encryptionKey: keyBytes}
}

func (s *NameSiloService) Connect(ctx context.Context, userID, apiKey string) error {
	client := namesilo.NewClient(apiKey)
	if err := client.TestConnection(ctx); err != nil {
		return errors.New("invalid API key")
	}

	encrypted, _ := s.encrypt(apiKey)
	_, err := s.db.Exec(`
		UPDATE users SET namesilo_api_key_encrypted = $1, namesilo_connected_at = $2 WHERE id = $3
	`, encrypted, time.Now(), userID)
	return err
}

func (s *NameSiloService) GetClient(userID string) (*namesilo.Client, error) {
	var encryptedKey string
	err := s.db.QueryRow(`SELECT namesilo_api_key_encrypted FROM users WHERE id = $1`, userID).Scan(&encryptedKey)
	if err != nil || encryptedKey == "" {
		return nil, errors.New("not connected to NameSilo")
	}

	apiKey, _ := s.decrypt(encryptedKey)
	return namesilo.NewClient(apiKey), nil
}

func (s *NameSiloService) ImportDomain(ctx context.Context, userID string, domain string, configureDNS bool) (string, error) {
	domain = strings.ToLower(domain)

	client, err := s.GetClient(userID)
	if err != nil {
		return "", err
	}

	// Verify domain exists
	if _, err := client.GetDomainInfo(ctx, domain); err != nil {
		return "", fmt.Errorf("domain not found: %w", err)
	}

	// Check if already imported
	var existing string
	if s.db.QueryRow(`SELECT id FROM domains WHERE user_id = $1 AND LOWER(name) = $2`, userID, domain).Scan(&existing) == nil {
		return "", errors.New("domain already imported")
	}

	// Generate IDs
	domainID := fmt.Sprintf("dom_%s", randomString(12))

	_, err = s.db.Exec(`
		INSERT INTO domains (id, user_id, name, source, is_wildcard, created_at, updated_at)
		VALUES ($1, $2, $3, 'namesilo', true, $4, $4)
	`, domainID, userID, domain, time.Now())
	if err != nil {
		return "", err
	}

	// Auto-configure DNS if requested
	if configureDNS {
		serverIP := os.Getenv("DEPLOY_SERVER_IP")
		if serverIP != "" {
			if err := client.SetupDNS(ctx, domain, serverIP); err != nil {
				s.db.Exec(`UPDATE domains SET dns_config_error = $1 WHERE id = $2`, err.Error(), domainID)
			} else {
				s.db.Exec(`UPDATE domains SET dns_auto_configured = true WHERE id = $1`, domainID)
			}
		}
	}

	return domainID, nil
}

func (s *NameSiloService) ReconfigureAllDomains(ctx context.Context, newIP string) (int, int, error) {
	type domainInfo struct {
		ID     string `db:"id"`
		Name   string `db:"name"`
		UserID string `db:"user_id"`
	}

	var domains []domainInfo
	s.db.Select(&domains, `SELECT id, name, user_id FROM domains WHERE source = 'namesilo' AND dns_auto_configured = true`)

	success, fail := 0, 0
	userClients := make(map[string]*namesilo.Client)

	for _, d := range domains {
		client, ok := userClients[d.UserID]
		if !ok {
			client, _ = s.GetClient(d.UserID)
			userClients[d.UserID] = client
		}
		if client == nil {
			fail++
			continue
		}

		if err := client.SetupDNS(ctx, d.Name, newIP); err != nil {
			fail++
			s.db.Exec(`UPDATE domains SET dns_config_error = $1 WHERE id = $2`, err.Error(), d.ID)
		} else {
			success++
			s.db.Exec(`UPDATE domains SET dns_config_error = NULL WHERE id = $1`, d.ID)
		}
	}

	return success, fail, nil
}

// Encryption helpers
func (s *NameSiloService) encrypt(plaintext string) (string, error) {
	block, _ := aes.NewCipher(s.encryptionKey)
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, gcm.NonceSize())
	io.ReadFull(rand.Reader, nonce)
	return base64.StdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(plaintext), nil)), nil
}

func (s *NameSiloService) decrypt(encrypted string) (string, error) {
	ciphertext, _ := base64.StdEncoding.DecodeString(encrypted)
	block, _ := aes.NewCipher(s.encryptionKey)
	gcm, _ := cipher.NewGCM(block)
	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, _ := gcm.Open(nil, nonce, ciphertext, nil)
	return string(plaintext), nil
}

func randomString(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	rand.Read(b)
	for i := range b {
		b[i] = chars[b[i]%byte(len(chars))]
	}
	return string(b)
}
```

## Step 4: Create HTTP Handlers

Create `handlers/namesilo_handlers.go`:

```go
package handlers

import (
	"encoding/json"
	"net/http"

	"yourapp/services"
	"yourapp/pkg/ctx"
)

type NameSiloHandler struct {
	svc *services.NameSiloService
}

func NewNameSiloHandler(svc *services.NameSiloService) *NameSiloHandler {
	return &NameSiloHandler{svc: svc}
}

func (h *NameSiloHandler) Connect(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var input struct {
		APIKey string `json:"apiKey"`
	}
	json.NewDecoder(r.Body).Decode(&input)

	if err := h.svc.Connect(r.Context(), userID, input.APIKey); err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

func (h *NameSiloHandler) Import(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var input struct {
		Domain       string `json:"domain"`
		ConfigureDNS bool   `json:"configureDns"`
	}
	json.NewDecoder(r.Body).Decode(&input)

	domainID, err := h.svc.ImportDomain(r.Context(), userID, input.Domain, input.ConfigureDNS)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "domainId": domainID})
}

func (h *NameSiloHandler) AdminReconfigureAll(w http.ResponseWriter, r *http.Request) {
	var input struct {
		IP string `json:"ip"`
	}
	json.NewDecoder(r.Body).Decode(&input)

	success, fail, err := h.svc.ReconfigureAllDomains(r.Context(), input.IP)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"successCount": success,
		"failCount":    fail,
	})
}
```

## Step 5: Register Routes

In your router setup:

```go
r.Route("/namesilo", func(r chi.Router) {
	r.Use(authMiddleware)
	r.Post("/connect", namesiloHandler.Connect)
	r.Post("/import", namesiloHandler.Import)
})

r.Route("/admin", func(r chi.Router) {
	r.Use(adminMiddleware)
	r.Post("/namesilo/reconfigure-all", namesiloHandler.AdminReconfigureAll)
})
```

## Step 6: Set Environment Variables

```bash
# Required
SESSION_SECRET=your-32-char-secret-key-here

# Deploy server IP (where DNS should point)
DEPLOY_SERVER_IP=1.2.3.4
```

## Step 7: Test the Integration

1. **Connect API key**:
   ```bash
   curl -X POST /namesilo/connect \
     -H "Content-Type: application/json" \
     -d '{"apiKey": "your-namesilo-api-key"}'
   ```

2. **Import domain with DNS auto-config**:
   ```bash
   curl -X POST /namesilo/import \
     -H "Content-Type: application/json" \
     -d '{"domain": "example.com", "configureDns": true}'
   ```

3. **Reconfigure all domains** (when server IP changes):
   ```bash
   curl -X POST /admin/namesilo/reconfigure-all \
     -H "Content-Type: application/json" \
     -d '{"ip": "5.6.7.8"}'
   ```

## Key Implementation Notes

1. **Rate limiting is mandatory**: NameSilo enforces 2 seconds between requests. The client handles this automatically.

2. **API response formats vary**: NameSilo returns domains/records as strings, arrays, or objects depending on count. Handle all formats.

3. **Domain names should be lowercase**: DNS is case-insensitive, normalize to lowercase on import.

4. **Delete before create**: When updating DNS records, delete existing ones first to avoid conflicts.

5. **Store encrypted API keys**: Use AES-256-GCM encryption for API keys in the database.

6. **Background SSL generation**: Wildcard SSL via acme-dns takes ~90 seconds for CNAME propagation. Run in a goroutine.
