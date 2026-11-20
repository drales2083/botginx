# NameSilo Domain Integration - Complete Implementation Reference

This document provides everything needed to implement NameSilo domain import with automatic DNS configuration and wildcard SSL generation. All code is extracted from a production implementation.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           IMPORT FLOW                                    │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  User enters API key → Test connection → Store encrypted in DB          │
│                                                                         │
│  User selects domain → Import to local DB → Auto-configure DNS          │
│                           │                      │                      │
│                           │                      ├─→ Add @ A record     │
│                           │                      ├─→ Add * A record     │
│                           │                      └─→ Add verify TXT     │
│                           │                                             │
│                           └─→ Setup acme-dns CNAME                      │
│                                      │                                  │
│                                      └─→ Generate wildcard SSL          │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

## Database Schema

### Migration (011_namesilo.sql)

```sql
-- NameSilo integration
-- Store API key per user (single connection per user)

ALTER TABLE users ADD COLUMN IF NOT EXISTS namesilo_api_key_encrypted TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS namesilo_connected_at TIMESTAMPTZ;

-- Add source tracking to domains
ALTER TABLE domains ADD COLUMN IF NOT EXISTS source VARCHAR(50) DEFAULT 'manual';
ALTER TABLE domains ADD COLUMN IF NOT EXISTS provider_domain_id VARCHAR(255);

-- DNS status for tracking auto-configuration
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
CREATE INDEX IF NOT EXISTS idx_domains_provider_domain_id ON domains(provider_domain_id);
```

---

## Package 1: NameSilo API Client

### File: `pkg/providers/namesilo/client.go`

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
	defaultBaseURL = "https://www.namesilo.com/api"
	apiVersion     = "1"
	responseType   = "json"

	// Rate limiting: 1 request per 2 seconds (NameSilo enforces this)
	minRequestInterval = 2 * time.Second
)

// Client is the NameSilo API client
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client

	// Rate limiting
	mu          sync.Mutex
	lastRequest time.Time
}

// NewClient creates a new NameSilo API client
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// apiResponse is the wrapper for all NameSilo API responses
type apiResponse struct {
	Reply reply `json:"reply"`
}

type reply struct {
	Code   int    `json:"code"`
	Detail string `json:"detail"`
}

// doRequest makes a rate-limited request to the NameSilo API
func (c *Client) doRequest(ctx context.Context, operation string, params url.Values) ([]byte, error) {
	// Rate limiting - NameSilo requires 2s between requests
	c.mu.Lock()
	elapsed := time.Since(c.lastRequest)
	if elapsed < minRequestInterval {
		time.Sleep(minRequestInterval - elapsed)
	}
	c.lastRequest = time.Now()
	c.mu.Unlock()

	// Build URL
	if params == nil {
		params = url.Values{}
	}
	params.Set("version", apiVersion)
	params.Set("type", responseType)
	params.Set("key", c.apiKey)

	reqURL := fmt.Sprintf("%s/%s?%s", c.baseURL, operation, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	return body, nil
}

// checkResponse parses the response and checks for API errors
func checkResponse(body []byte) error {
	var resp apiResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}

	// NameSilo success codes: 300 = success, 301 = already exists (also success)
	if resp.Reply.Code != 300 && resp.Reply.Code != 301 {
		return &APIError{
			Code:    resp.Reply.Code,
			Message: resp.Reply.Detail,
		}
	}

	return nil
}

// APIError represents a NameSilo API error
type APIError struct {
	Code    int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("namesilo: %s (code %d)", e.Message, e.Code)
}

// IsInvalidKey returns true if the error indicates an invalid API key
func (e *APIError) IsInvalidKey() bool {
	return e.Code == 110
}

// IsDomainNotFound returns true if the domain doesn't exist in the account
func (e *APIError) IsDomainNotFound() bool {
	return e.Code == 200
}

// Common error codes:
// 110 = Invalid API key
// 200 = Domain not found in account
// 250 = Domain not active
// 261 = DNS record already exists
// 262 = Invalid IP address
// 280 = DNS record not found
```

### File: `pkg/providers/namesilo/domains.go`

```go
package namesilo

import (
	"context"
	"encoding/json"
	"net/url"
	"time"
)

// Domain represents a domain from the NameSilo account
type Domain struct {
	Domain    string     `json:"domain"`
	Created   *time.Time `json:"created,omitempty"`
	Expires   *time.Time `json:"expires,omitempty"`
	Status    string     `json:"status"`
	Locked    bool       `json:"locked"`
	Private   bool       `json:"private"`
	AutoRenew bool       `json:"auto_renew"`
}

// listDomainsResponse handles NameSilo's variable response format
type listDomainsResponse struct {
	Reply struct {
		Code    int         `json:"code"`
		Detail  string      `json:"detail"`
		Domains interface{} `json:"domains"` // can be object, array, or string
	} `json:"reply"`
}

type getDomainInfoResponse struct {
	Reply struct {
		Code      int    `json:"code"`
		Detail    string `json:"detail"`
		Created   string `json:"created"`
		Expires   string `json:"expires"`
		Status    string `json:"status"`
		Locked    string `json:"locked"`    // "Yes" or "No"
		Private   string `json:"private"`   // "Yes" or "No"
		AutoRenew string `json:"auto_renew"` // "Yes" or "No"
	} `json:"reply"`
}

// ListDomains returns all domains in the NameSilo account
func (c *Client) ListDomains(ctx context.Context) ([]string, error) {
	body, err := c.doRequest(ctx, "listDomains", nil)
	if err != nil {
		return nil, err
	}

	var resp listDomainsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	if resp.Reply.Code != 300 {
		return nil, &APIError{Code: resp.Reply.Code, Message: resp.Reply.Detail}
	}

	// Handle various response formats from NameSilo API
	var domains []string
	switch v := resp.Reply.Domains.(type) {
	case map[string]interface{}:
		// Format: {"domains": {"domain": "example.com"}} or {"domain": ["a.com", "b.com"]}
		if domainField, ok := v["domain"]; ok {
			switch d := domainField.(type) {
			case string:
				if d != "" {
					domains = []string{d}
				}
			case []interface{}:
				for _, item := range d {
					if s, ok := item.(string); ok {
						domains = append(domains, s)
					}
				}
			}
		}
	case []interface{}:
		for _, d := range v {
			switch item := d.(type) {
			case string:
				domains = append(domains, item)
			case map[string]interface{}:
				if domainName, ok := item["domain"].(string); ok && domainName != "" {
					domains = append(domains, domainName)
				}
			}
		}
	case string:
		if v != "" {
			domains = []string{v}
		}
	}

	return domains, nil
}

// GetDomainInfo returns detailed information about a specific domain
func (c *Client) GetDomainInfo(ctx context.Context, domain string) (*Domain, error) {
	params := url.Values{}
	params.Set("domain", domain)

	body, err := c.doRequest(ctx, "getDomainInfo", params)
	if err != nil {
		return nil, err
	}

	var resp getDomainInfoResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	if resp.Reply.Code != 300 {
		return nil, &APIError{Code: resp.Reply.Code, Message: resp.Reply.Detail}
	}

	// Parse dates
	var created, expires *time.Time
	if resp.Reply.Created != "" {
		if t, err := time.Parse("2006-01-02", resp.Reply.Created); err == nil {
			created = &t
		}
	}
	if resp.Reply.Expires != "" {
		if t, err := time.Parse("2006-01-02", resp.Reply.Expires); err == nil {
			expires = &t
		}
	}

	return &Domain{
		Domain:    domain,
		Created:   created,
		Expires:   expires,
		Status:    resp.Reply.Status,
		Locked:    resp.Reply.Locked == "Yes",
		Private:   resp.Reply.Private == "Yes",
		AutoRenew: resp.Reply.AutoRenew == "Yes",
	}, nil
}

// TestConnection tests if the API key is valid
func (c *Client) TestConnection(ctx context.Context) error {
	_, err := c.ListDomains(ctx)
	return err
}
```

### File: `pkg/providers/namesilo/dns.go`

```go
package namesilo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// DNSRecord represents a DNS record
type DNSRecord struct {
	RecordID string `json:"record_id"`
	Type     string `json:"type"`
	Host     string `json:"host"`
	Value    string `json:"value"`
	TTL      int    `json:"ttl"`
	Distance int    `json:"distance,omitempty"` // MX priority
}

// DNSRecordInput is the input for adding/updating a DNS record
type DNSRecordInput struct {
	Type     string // A, AAAA, CNAME, MX, TXT
	Host     string // subdomain or empty string for root (@)
	Value    string // IP or target
	TTL      int    // seconds (default 3600)
	Distance int    // MX priority (optional)
}

type dnsListResponse struct {
	Reply struct {
		Code           int         `json:"code"`
		Detail         string      `json:"detail"`
		ResourceRecord interface{} `json:"resource_record"`
	} `json:"reply"`
}

type dnsAddResponse struct {
	Reply struct {
		Code     int    `json:"code"`
		Detail   string `json:"detail"`
		RecordID string `json:"record_id"`
	} `json:"reply"`
}

// ListDNSRecords returns all DNS records for a domain
func (c *Client) ListDNSRecords(ctx context.Context, domain string) ([]DNSRecord, error) {
	params := url.Values{}
	params.Set("domain", domain)

	body, err := c.doRequest(ctx, "dnsListRecords", params)
	if err != nil {
		return nil, err
	}

	var resp dnsListResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	if resp.Reply.Code != 300 {
		return nil, &APIError{Code: resp.Reply.Code, Message: resp.Reply.Detail}
	}

	var records []DNSRecord
	switch v := resp.Reply.ResourceRecord.(type) {
	case map[string]interface{}:
		record := parseRecordMap(v)
		if record != nil {
			records = append(records, *record)
		}
	case []interface{}:
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				record := parseRecordMap(m)
				if record != nil {
					records = append(records, *record)
				}
			}
		}
	}

	return records, nil
}

func parseRecordMap(m map[string]interface{}) *DNSRecord {
	record := &DNSRecord{}
	if v, ok := m["record_id"].(string); ok {
		record.RecordID = v
	}
	if v, ok := m["type"].(string); ok {
		record.Type = v
	}
	if v, ok := m["host"].(string); ok {
		record.Host = v
	}
	if v, ok := m["value"].(string); ok {
		record.Value = v
	}
	if v, ok := m["ttl"].(float64); ok {
		record.TTL = int(v)
	}
	if v, ok := m["distance"].(float64); ok {
		record.Distance = int(v)
	}
	return record
}

// AddDNSRecord adds a new DNS record
func (c *Client) AddDNSRecord(ctx context.Context, domain string, input DNSRecordInput) (string, error) {
	params := url.Values{}
	params.Set("domain", domain)
	params.Set("rrtype", input.Type)
	params.Set("rrhost", input.Host)
	params.Set("rrvalue", input.Value)

	ttl := input.TTL
	if ttl == 0 {
		ttl = 3600
	}
	params.Set("rrttl", fmt.Sprintf("%d", ttl))

	if input.Type == "MX" && input.Distance > 0 {
		params.Set("rrdistance", fmt.Sprintf("%d", input.Distance))
	}

	body, err := c.doRequest(ctx, "dnsAddRecord", params)
	if err != nil {
		return "", err
	}

	var resp dnsAddResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", err
	}

	if resp.Reply.Code != 300 && resp.Reply.Code != 301 {
		return "", &APIError{Code: resp.Reply.Code, Message: resp.Reply.Detail}
	}

	return resp.Reply.RecordID, nil
}

// DeleteDNSRecord deletes a DNS record
func (c *Client) DeleteDNSRecord(ctx context.Context, domain, recordID string) error {
	params := url.Values{}
	params.Set("domain", domain)
	params.Set("rrid", recordID)

	body, err := c.doRequest(ctx, "dnsDeleteRecord", params)
	if err != nil {
		return err
	}

	return checkResponse(body)
}

// SetupDNS configures A records for root (@) and wildcard (*)
// Deletes existing conflicting records first
func (c *Client) SetupDNS(ctx context.Context, domain, serverIP string) (rootID, wildcardID string, err error) {
	// List existing records to check for conflicts
	records, err := c.ListDNSRecords(ctx, domain)
	if err != nil {
		return "", "", fmt.Errorf("listing DNS records: %w", err)
	}

	// Delete existing A records for @ and *
	for _, r := range records {
		if r.Type == "A" {
			if r.Host == "" || r.Host == "@" {
				if err := c.DeleteDNSRecord(ctx, domain, r.RecordID); err != nil {
					return "", "", fmt.Errorf("deleting existing root A record: %w", err)
				}
			} else if r.Host == "*" {
				if err := c.DeleteDNSRecord(ctx, domain, r.RecordID); err != nil {
					return "", "", fmt.Errorf("deleting existing wildcard A record: %w", err)
				}
			}
		}
	}

	// Add root A record (@)
	rootID, err = c.AddDNSRecord(ctx, domain, DNSRecordInput{
		Type:  "A",
		Host:  "", // empty = root (@)
		Value: serverIP,
		TTL:   3600,
	})
	if err != nil {
		return "", "", fmt.Errorf("adding root A record: %w", err)
	}

	// Add wildcard A record (*)
	wildcardID, err = c.AddDNSRecord(ctx, domain, DNSRecordInput{
		Type:  "A",
		Host:  "*",
		Value: serverIP,
		TTL:   3600,
	})
	if err != nil {
		return rootID, "", fmt.Errorf("adding wildcard A record: %w", err)
	}

	return rootID, wildcardID, nil
}

// AddVerificationTXT adds a verification TXT record
func (c *Client) AddVerificationTXT(ctx context.Context, domain, token string) (string, error) {
	// Delete existing verification record if present
	records, err := c.ListDNSRecords(ctx, domain)
	if err == nil {
		for _, r := range records {
			if r.Type == "TXT" && r.Host == "_guardbot-verify" {
				c.DeleteDNSRecord(ctx, domain, r.RecordID)
			}
		}
	}

	return c.AddDNSRecord(ctx, domain, DNSRecordInput{
		Type:  "TXT",
		Host:  "_guardbot-verify",
		Value: token,
		TTL:   3600,
	})
}

// AddAcmeDNSCname adds a CNAME for acme-dns delegation (instant SSL)
// Creates: _acme-challenge.domain.com → fulldomain (e.g., xyz.acme.example.com)
func (c *Client) AddAcmeDNSCname(ctx context.Context, domain, fulldomain string) (string, error) {
	// Delete existing _acme-challenge records
	records, err := c.ListDNSRecords(ctx, domain)
	if err == nil {
		for _, r := range records {
			if r.Host == "_acme-challenge" {
				c.DeleteDNSRecord(ctx, domain, r.RecordID)
			}
		}
	}

	// NameSilo requires CNAME value without trailing dot
	target := strings.TrimSuffix(fulldomain, ".")

	return c.AddDNSRecord(ctx, domain, DNSRecordInput{
		Type:  "CNAME",
		Host:  "_acme-challenge",
		Value: target,
		TTL:   3600,
	})
}
```

---

## Package 2: acme-dns Client (Instant SSL)

### File: `pkg/acmedns/client.go`

```go
package acmedns

import (
	"encoding/json"
	"fmt"
	"strings"

	"yourapp/pkg/sshexec"
)

// Registration contains the acme-dns registration response
type Registration struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	Subdomain  string `json:"subdomain"`
	Fulldomain string `json:"fulldomain"`
}

// Client manages acme-dns operations via SSH to Deploy VPS
type Client struct {
	sshClient *sshexec.Client
	apiURL    string
}

// NewClient creates a new acme-dns client connected via SSH
func NewClient(ip string, port int, user, password string) (*Client, error) {
	portStr := fmt.Sprintf("%d", port)
	if port == 0 {
		portStr = "22"
	}

	client, err := sshexec.NewClient(ip, portStr, user, password)
	if err != nil {
		return nil, fmt.Errorf("SSH connection failed: %w", err)
	}

	return &Client{
		sshClient: client,
		apiURL:    "http://127.0.0.1:8053",
	}, nil
}

func (c *Client) Close() {
	if c.sshClient != nil {
		c.sshClient.Close()
	}
}

// IsInstalled checks if acme-dns is running on the server
func (c *Client) IsInstalled() bool {
	output, err := c.sshClient.Run("systemctl is-active acme-dns 2>/dev/null || echo inactive")
	if err != nil {
		return false
	}
	return strings.TrimSpace(output) == "active"
}

// Register creates a new subdomain on acme-dns
func (c *Client) Register() (*Registration, error) {
	cmd := fmt.Sprintf(`curl -s -X POST %s/register`, c.apiURL)

	output, err := c.sshClient.Run(cmd)
	if err != nil {
		return nil, fmt.Errorf("registration failed: %w", err)
	}

	output = strings.TrimSpace(output)
	if output == "" {
		return nil, fmt.Errorf("empty response from acme-dns")
	}

	var reg Registration
	if err := json.Unmarshal([]byte(output), &reg); err != nil {
		return nil, fmt.Errorf("failed to parse registration: %w (response: %s)", err, output)
	}

	if reg.Subdomain == "" {
		return nil, fmt.Errorf("invalid registration response: %s", output)
	}

	return &reg, nil
}

// UpdateTXT sets the TXT record value for a subdomain
func (c *Client) UpdateTXT(subdomain, password, txtValue string) error {
	payload := fmt.Sprintf(`{"subdomain":"%s","txt":"%s"}`, subdomain, txtValue)

	cmd := fmt.Sprintf(`curl -s -X POST %s/update \
		-H "X-Api-User: %s" \
		-H "X-Api-Key: %s" \
		-H "Content-Type: application/json" \
		-d '%s'`, c.apiURL, subdomain, password, payload)

	output, err := c.sshClient.Run(cmd)
	if err != nil {
		return fmt.Errorf("TXT update failed: %w", err)
	}

	if strings.Contains(output, "error") {
		return fmt.Errorf("acme-dns error: %s", output)
	}

	return nil
}
```

---

## Package 3: Service Layer

### File: `modules/domains/services/namesilo_service.go`

The service layer ties everything together. Key methods:

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
	"os"
	"strings"
	"time"

	"yourapp/modules/domains/models"
	"yourapp/pkg/acmedns"
	"yourapp/pkg/providers/namesilo"
	"yourapp/pkg/sshexec"
	"github.com/jmoiron/sqlx"
)

type NameSiloService struct {
	db            *sqlx.DB
	encryptionKey []byte
}

func NewNameSiloService(db *sqlx.DB) *NameSiloService {
	key := os.Getenv("NAMESILO_ENCRYPTION_KEY")
	if key == "" {
		key = os.Getenv("SESSION_SECRET") // fallback
	}

	// Ensure key is 32 bytes for AES-256
	keyBytes := []byte(key)
	if len(keyBytes) < 32 {
		padded := make([]byte, 32)
		copy(padded, keyBytes)
		keyBytes = padded
	} else if len(keyBytes) > 32 {
		keyBytes = keyBytes[:32]
	}

	return &NameSiloService{
		db:            db,
		encryptionKey: keyBytes,
	}
}

// Connect stores API key and tests connection
func (s *NameSiloService) Connect(ctx context.Context, userID, apiKey string) error {
	// Test the API key first
	client := namesilo.NewClient(apiKey)
	if err := client.TestConnection(ctx); err != nil {
		var apiErr *namesilo.APIError
		if errors.As(err, &apiErr) && apiErr.IsInvalidKey() {
			return errors.New("invalid API key")
		}
		return fmt.Errorf("connection test failed: %w", err)
	}

	// Encrypt the API key
	encrypted, err := s.encrypt(apiKey)
	if err != nil {
		return fmt.Errorf("encrypting API key: %w", err)
	}

	// Store in database
	_, err = s.db.Exec(`
		UPDATE users
		SET namesilo_api_key_encrypted = $1, namesilo_connected_at = $2
		WHERE id = $3
	`, encrypted, time.Now(), userID)

	return err
}

// ImportDomain imports a domain from NameSilo with optional DNS auto-config
func (s *NameSiloService) ImportDomain(ctx context.Context, userID string, input models.NameSiloImportInput) (string, error) {
	// Normalize domain name to lowercase
	input.Domain = strings.ToLower(input.Domain)

	client, err := s.GetClient(userID)
	if err != nil {
		return "", err
	}

	// Verify domain exists in user's NameSilo account
	_, err = client.GetDomainInfo(ctx, input.Domain)
	if err != nil {
		return "", fmt.Errorf("domain not found in NameSilo account: %w", err)
	}

	// Check if already imported
	var existingID string
	err = s.db.QueryRow(`
		SELECT id FROM domains WHERE user_id = $1 AND LOWER(name) = LOWER($2)
	`, userID, input.Domain).Scan(&existingID)
	if err == nil {
		return existingID, errors.New("domain already imported")
	}

	// Generate IDs
	domainID := fmt.Sprintf("dom_%s", generateID(12))
	verifyToken := fmt.Sprintf("guardbot-%s", generateID(12))
	serverID := s.getDeployServerID()

	// Insert domain (wildcard for redirect links)
	_, err = s.db.Exec(`
		INSERT INTO domains (id, user_id, name, verify_token, server_id, source, provider_domain_id, is_wildcard, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), 'namesilo', $6, true, $7, $7)
	`, domainID, userID, input.Domain, verifyToken, serverID, input.Domain, time.Now())
	if err != nil {
		return "", fmt.Errorf("inserting domain: %w", err)
	}

	// Auto-configure DNS if requested
	if input.ConfigureDNS {
		serverIP, err := s.getDeployServerIP()
		if err != nil {
			s.db.Exec(`UPDATE domains SET dns_config_error = $1 WHERE id = $2`, err.Error(), domainID)
		} else {
			// Add A records (root + wildcard)
			_, _, err = client.SetupDNS(ctx, input.Domain, serverIP)
			if err != nil {
				s.db.Exec(`UPDATE domains SET dns_config_error = $1 WHERE id = $2`, err.Error(), domainID)
			} else {
				// Add verification TXT record
				client.AddVerificationTXT(ctx, input.Domain, verifyToken)

				s.db.Exec(`UPDATE domains SET dns_auto_configured = true, dns_verified = true WHERE id = $1`, domainID)

				// Setup acme-dns for instant wildcard SSL (background)
				go s.setupAcmeDNSAndSSL(userID, input.Domain, domainID, client)
			}
		}
	}

	return domainID, nil
}

// setupAcmeDNSAndSSL sets up acme-dns CNAME and generates wildcard SSL
func (s *NameSiloService) setupAcmeDNSAndSSL(userID, domain, domainID string, nsClient *namesilo.Client) {
	log.Printf("[NameSilo] Setting up acme-dns for %s", domain)
	ctx := context.Background()

	// Step 1: Register with acme-dns on Deploy VPS
	acmeReg, err := s.registerWithAcmeDNS(domainID)
	if err != nil {
		log.Printf("[NameSilo] acme-dns registration failed for %s: %v", domain, err)
		s.db.Exec(`UPDATE domains SET ssl_error = $1 WHERE id = $2`, err.Error(), domainID)
		return
	}

	// Step 2: Add CNAME via NameSilo API
	_, err = nsClient.AddAcmeDNSCname(ctx, domain, acmeReg.Fulldomain)
	if err != nil {
		log.Printf("[NameSilo] CNAME setup failed for %s: %v", domain, err)
		s.db.Exec(`UPDATE domains SET ssl_error = $1 WHERE id = $2`, "acme-dns CNAME failed: "+err.Error(), domainID)
		return
	}

	// Step 3: Wait for CNAME propagation (usually 60-120s)
	log.Printf("[NameSilo] Waiting 90s for CNAME propagation for %s", domain)
	time.Sleep(90 * time.Second)

	// Step 4: Generate SSL using acme-dns (instant TXT updates)
	log.Printf("[NameSilo] Generating wildcard SSL via acme-dns for %s", domain)
	if err := s.GenerateWildcardSSLWithAcmeDNS(domain, acmeReg.Subdomain, acmeReg.Username, acmeReg.Password); err != nil {
		log.Printf("[NameSilo] acme-dns SSL failed for %s: %v", domain, err)
		s.db.Exec(`UPDATE domains SET ssl_error = $1 WHERE id = $2`, err.Error(), domainID)
	} else {
		log.Printf("[NameSilo] Wildcard SSL completed for %s", domain)
		s.db.Exec(`UPDATE domains SET ssl_enabled = true, ssl_error = NULL, acme_cname_verified = true WHERE id = $1`, domainID)
	}
}

// registerWithAcmeDNS registers a domain with acme-dns on the Deploy VPS
func (s *NameSiloService) registerWithAcmeDNS(domainID string) (*acmedns.Registration, error) {
	// Get Deploy VPS credentials
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

	// Connect to acme-dns
	client, err := acmedns.NewClient(server.IP, server.Port, server.User, server.Password)
	if err != nil {
		return nil, fmt.Errorf("SSH connection failed: %w", err)
	}
	defer client.Close()

	// Check if acme-dns is installed
	if !client.IsInstalled() {
		acmeDomain := os.Getenv("ACME_DNS_DOMAIN")
		if acmeDomain == "" {
			return nil, fmt.Errorf("ACME_DNS_DOMAIN not set and acme-dns not installed")
		}
		if err := client.Setup(acmeDomain); err != nil {
			return nil, fmt.Errorf("acme-dns setup failed: %w", err)
		}
	}

	// Register new subdomain
	reg, err := client.Register()
	if err != nil {
		return nil, fmt.Errorf("acme-dns registration failed: %w", err)
	}

	// Store registration in database
	_, err = s.db.Exec(`
		UPDATE domains SET
			acme_subdomain = $1, acme_username = $2, acme_password = $3, acme_fulldomain = $4, updated_at = NOW()
		WHERE id = $5
	`, reg.Subdomain, reg.Username, reg.Password, reg.Fulldomain, domainID)
	if err != nil {
		return nil, fmt.Errorf("failed to store acme-dns credentials: %w", err)
	}

	return reg, nil
}

// GenerateWildcardSSLWithAcmeDNS generates wildcard SSL using acme-dns
func (s *NameSiloService) GenerateWildcardSSLWithAcmeDNS(domain, acmeSubdomain, acmeUsername, acmePassword string) error {
	// Get Deploy VPS credentials
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
		return fmt.Errorf("no deploy server found: %w", err)
	}

	baseDomain := strings.TrimPrefix(domain, "*.")

	sshClient, err := sshexec.NewClient(server.IP, fmt.Sprintf("%d", server.Port), server.User, server.Password)
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer sshClient.Close()

	// Create auth hook that updates acme-dns TXT record (instant)
	authHookScript := fmt.Sprintf(`cat > /tmp/acmedns-auth-hook.sh << 'HOOKEOF'
#!/bin/bash
RESPONSE=$(curl -s -w "\n%%{http_code}" -X POST http://127.0.0.1:8053/update \
    -H "X-Api-User: %s" \
    -H "X-Api-Key: %s" \
    -H "Content-Type: application/json" \
    -d "{\"subdomain\":\"%s\",\"txt\":\"$CERTBOT_VALIDATION\"}")

HTTP_CODE=$(echo "$RESPONSE" | tail -1)
if [ "$HTTP_CODE" != "200" ]; then
    echo "acme-dns update failed: HTTP $HTTP_CODE" >&2
    exit 1
fi
sleep 5
HOOKEOF
chmod +x /tmp/acmedns-auth-hook.sh`, acmeUsername, acmePassword, acmeSubdomain)
	sshClient.Run(authHookScript)

	// Run certbot with DNS-01 challenge
	certbotCmd := fmt.Sprintf(`
certbot certonly --manual --preferred-challenges dns \
  -d "*.%s" -d "%s" \
  --agree-tos --email admin@%s \
  --manual-auth-hook /tmp/acmedns-auth-hook.sh \
  --manual-cleanup-hook "echo cleanup" \
  --non-interactive --force-renewal 2>&1
`, baseDomain, baseDomain, baseDomain)

	output, err := sshClient.Run(certbotCmd)
	if err != nil {
		if strings.Contains(output, "Certificate not yet due for renewal") {
			return nil
		}
		return fmt.Errorf("SSL generation failed: %s", output)
	}

	// Check certificate was created
	checkCmd := fmt.Sprintf(`test -f /etc/letsencrypt/live/%s/fullchain.pem && echo "SUCCESS"`, baseDomain)
	if out, _ := sshClient.Run(checkCmd); !strings.Contains(out, "SUCCESS") {
		return fmt.Errorf("certificate not created")
	}

	// Setup nginx config with proxy to botection
	s.setupProxyNginxConfig(sshClient, baseDomain)

	return nil
}

// ReconfigureAllDomains updates DNS for all NameSilo domains when server IP changes
func (s *NameSiloService) ReconfigureAllDomains(ctx context.Context, newIP string) ([]ReconfigureResult, error) {
	type domainInfo struct {
		ID     string `db:"id"`
		Name   string `db:"name"`
		UserID string `db:"user_id"`
	}

	var domains []domainInfo
	err := s.db.Select(&domains, `
		SELECT id, name, user_id FROM domains
		WHERE source = 'namesilo' AND dns_auto_configured = true
	`)
	if err != nil {
		return nil, fmt.Errorf("querying domains: %w", err)
	}

	// Group by user (each user has their own API key)
	userDomains := make(map[string][]domainInfo)
	for _, d := range domains {
		userDomains[d.UserID] = append(userDomains[d.UserID], d)
	}

	var results []ReconfigureResult

	for userID, userDoms := range userDomains {
		client, err := s.GetClient(userID)
		if err != nil {
			for _, d := range userDoms {
				results = append(results, ReconfigureResult{Domain: d.Name, Success: false, Error: err.Error()})
			}
			continue
		}

		for _, d := range userDoms {
			_, _, err := client.SetupDNS(ctx, d.Name, newIP)
			if err != nil {
				results = append(results, ReconfigureResult{Domain: d.Name, Success: false, Error: err.Error()})
				s.db.Exec(`UPDATE domains SET dns_config_error = $1 WHERE id = $2`, err.Error(), d.ID)
			} else {
				results = append(results, ReconfigureResult{Domain: d.Name, Success: true})
				s.db.Exec(`UPDATE domains SET dns_config_error = NULL, dns_verified = true WHERE id = $1`, d.ID)
			}
		}
	}

	return results, nil
}

// Encryption helpers
func (s *NameSiloService) encrypt(plaintext string) (string, error) {
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

func (s *NameSiloService) decrypt(encrypted string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
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

	if len(ciphertext) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

func generateID(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	rand.Read(b)
	for i := range b {
		b[i] = chars[b[i]%byte(len(chars))]
	}
	return string(b)
}
```

---

## HTTP Handlers

### File: `modules/domains/handlers/namesilo_handlers.go`

```go
package handlers

import (
	"encoding/json"
	"net/http"

	"yourapp/modules/domains/models"
	"yourapp/modules/domains/services"
	"yourapp/pkg/ctx"
	"github.com/go-chi/chi/v5"
)

type NameSiloHandler struct {
	namesiloService *services.NameSiloService
}

func NewNameSiloHandler(namesiloService *services.NameSiloService) *NameSiloHandler {
	return &NameSiloHandler{namesiloService: namesiloService}
}

func (h *NameSiloHandler) jsonError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": msg})
}

func (h *NameSiloHandler) jsonSuccess(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "data": data})
}

// Connect handles API key connection
func (h *NameSiloHandler) Connect(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var input struct {
		APIKey string `json:"apiKey"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := h.namesiloService.Connect(r.Context(), userID, input.APIKey); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.jsonSuccess(w, map[string]interface{}{"message": "Connected to NameSilo"})
}

// Import imports a domain from NameSilo
func (h *NameSiloHandler) Import(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var input models.NameSiloImportInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	domainID, err := h.namesiloService.ImportDomain(r.Context(), userID, input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.jsonSuccess(w, map[string]interface{}{
		"message":  "Domain imported successfully",
		"domainId": domainID,
	})
}

// Reconfigure updates DNS for all user's NameSilo domains
func (h *NameSiloHandler) Reconfigure(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	results, err := h.namesiloService.ReconfigureUserDomains(r.Context(), userID)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	successCount := 0
	for _, res := range results {
		if res.Success {
			successCount++
		}
	}

	h.jsonSuccess(w, map[string]interface{}{
		"results":      results,
		"total":        len(results),
		"successCount": successCount,
	})
}
```

---

## Routes Registration

```go
// In module.go or routes.go
r.Route("/namesilo", func(r chi.Router) {
	r.Use(authMiddleware)
	
	r.Get("/", namesiloHandler.Page)                    // Main page
	r.Post("/api/connect", namesiloHandler.Connect)     // Connect API key
	r.Post("/api/disconnect", namesiloHandler.Disconnect)
	r.Get("/api/domains", namesiloHandler.ListDomains)
	r.Post("/api/import", namesiloHandler.Import)       // Import domain
	r.Post("/api/dns/{domain}", namesiloHandler.SetupDNS)
	r.Get("/api/dns/{domain}", namesiloHandler.GetDNSRecords)
	r.Post("/api/reconfigure", namesiloHandler.Reconfigure)        // Update all domains
	r.Post("/api/reconfigure/{id}", namesiloHandler.ReconfigureDomain) // Update single domain
})

// Admin routes
r.Route("/admin", func(r chi.Router) {
	r.Use(adminMiddleware)
	r.Post("/namesilo/reconfigure-all", namesiloHandler.AdminReconfigureAll)
})
```

---

## Environment Variables

```bash
# Required
NAMESILO_ENCRYPTION_KEY=your-32-char-key-here  # or falls back to SESSION_SECRET

# For acme-dns (instant SSL)
ACME_DNS_DOMAIN=acme.yourdomain.com  # Your acme-dns server domain

# Deploy server (where SSL certs go)
# Either set these OR have a "servers" table with deploy server info
DEPLOY_SERVER_IP=1.2.3.4
```

---

## Flow Summary

1. **User connects API key** → Encrypted, stored in `users.namesilo_api_key_encrypted`
2. **User imports domain** → Verified via API, inserted into `domains` table with `source='namesilo'`
3. **DNS auto-config (optional)**:
   - Add `@` A record pointing to deploy server
   - Add `*` A record (wildcard) pointing to deploy server
   - Add verification TXT record
4. **acme-dns setup (background)**:
   - Register subdomain on acme-dns server
   - Add `_acme-challenge` CNAME pointing to acme-dns subdomain
   - Wait 90s for CNAME propagation
   - Run certbot with custom auth hook that uses acme-dns for instant TXT updates
5. **Reconfigure** (when deploy server IP changes):
   - Updates all A records for all auto-configured domains via their users' API keys
