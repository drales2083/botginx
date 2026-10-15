# cPanel Domain Integration Specification

## Overview

Enable users to add domains hosted on their own cPanel servers, in addition to domains deployed on the main (admin-controlled) VPS. This allows users with existing hosting to use their own infrastructure for redirect links.

## Architecture

### Two Domain Types

| Type | Description | Deployment Target | Credentials |
|------|-------------|-------------------|-------------|
| **Main** | Deployed to admin's VPS | Central Deploy VPS (139.28.37.226) | Admin SSH keys |
| **cPanel** | Deployed to user's hosting | User's cPanel server | User provides API token |

### Traffic Flow

**Main Domain:**
```
User → CDN/Cloudflare → Deploy VPS nginx → botection → site files
```

**cPanel Domain:**
```
User → User's hosting nginx/apache → site files (no botection)
```

> Note: cPanel domains do NOT go through botection. Bot protection is embedded in the PHP files themselves via the self-hosted antibot SDK.

---

## Database Schema

### New Table: `cpanel_servers`

Stores user's cPanel server credentials (encrypted).

```sql
CREATE TABLE cpanel_servers (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,                          -- User-friendly name ("My Hosting")
    hostname TEXT NOT NULL,                      -- e.g., "server.hostingprovider.com"
    port INT NOT NULL DEFAULT 2083,              -- cPanel SSL port
    username TEXT NOT NULL,                      -- cPanel username
    api_token_encrypted BYTEA NOT NULL,          -- Encrypted API token (NEVER store plain)
    document_root TEXT DEFAULT '/public_html',   -- Default document root
    verified BOOLEAN DEFAULT FALSE,              -- Connection verified
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    
    UNIQUE(user_id, hostname, username)          -- One entry per user/server combo
);

CREATE INDEX idx_cpanel_servers_user ON cpanel_servers(user_id);
```

### Modify: `domains` Table

Add fields to support cPanel-hosted domains.

```sql
-- Add new columns
ALTER TABLE domains ADD COLUMN hosting_type TEXT NOT NULL DEFAULT 'main';
-- Values: 'main' (deploy VPS), 'cpanel' (user's cPanel)

ALTER TABLE domains ADD COLUMN cpanel_server_id TEXT REFERENCES cpanel_servers(id) ON DELETE SET NULL;
-- Only set when hosting_type = 'cpanel'

ALTER TABLE domains ADD COLUMN cpanel_document_root TEXT;
-- Override server default, e.g., '/public_html/subdomain.domain.com'

-- Add check constraint
ALTER TABLE domains ADD CONSTRAINT chk_hosting_type 
    CHECK (hosting_type IN ('main', 'cpanel'));

ALTER TABLE domains ADD CONSTRAINT chk_cpanel_server 
    CHECK (
        (hosting_type = 'cpanel' AND cpanel_server_id IS NOT NULL) OR
        (hosting_type = 'main' AND cpanel_server_id IS NULL)
    );
```

---

## Models

### Go Structs

```go
// pkg/models/cpanel_server.go

type CpanelServer struct {
    ID                string     `db:"id" json:"id"`
    UserID            string     `db:"user_id" json:"userId"`
    Name              string     `db:"name" json:"name"`
    Hostname          string     `db:"hostname" json:"hostname"`
    Port              int        `db:"port" json:"port"`
    Username          string     `db:"username" json:"username"`
    APITokenEncrypted []byte     `db:"api_token_encrypted" json:"-"` // Never expose
    DocumentRoot      string     `db:"document_root" json:"documentRoot"`
    Verified          bool       `db:"verified" json:"verified"`
    VerifiedAt        *time.Time `db:"verified_at" json:"verifiedAt"`
    CreatedAt         time.Time  `db:"created_at" json:"createdAt"`
    UpdatedAt         time.Time  `db:"updated_at" json:"updatedAt"`
}

// CreateCpanelServerInput - for adding a new server
type CreateCpanelServerInput struct {
    Name         string `json:"name" validate:"required,min=1,max=100"`
    Hostname     string `json:"hostname" validate:"required,hostname"`
    Port         int    `json:"port" validate:"min=1,max=65535"`
    Username     string `json:"username" validate:"required,min=1,max=100"`
    APIToken     string `json:"apiToken" validate:"required,min=10"` // Plain token from user
    DocumentRoot string `json:"documentRoot"`
}
```

### Domain Model Updates

```go
// Extend existing Domain model
type Domain struct {
    // ... existing fields ...
    
    HostingType      string  `db:"hosting_type" json:"hostingType"`       // "main" or "cpanel"
    CpanelServerID   *string `db:"cpanel_server_id" json:"cpanelServerId"`
    CpanelDocRoot    *string `db:"cpanel_document_root" json:"cpanelDocumentRoot"`
}
```

---

## API Endpoints

### cPanel Servers Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/user/cpanel/servers` | List user's cPanel servers |
| POST | `/user/cpanel/servers` | Add new cPanel server |
| GET | `/user/cpanel/servers/:id` | Get server details |
| PUT | `/user/cpanel/servers/:id` | Update server |
| DELETE | `/user/cpanel/servers/:id` | Remove server |
| POST | `/user/cpanel/servers/:id/verify` | Test connection |
| GET | `/user/cpanel/servers/:id/domains` | List domains on this server |

### API Response Examples

**List Servers:**
```json
{
  "servers": [
    {
      "id": "abc123",
      "name": "My Hosting",
      "hostname": "server.example.com",
      "port": 2083,
      "username": "myuser",
      "documentRoot": "/public_html",
      "verified": true,
      "verifiedAt": "2026-09-20T10:00:00Z",
      "domainCount": 3
    }
  ]
}
```

**Add Server:**
```json
// Request
{
  "name": "My Hosting",
  "hostname": "server.example.com",
  "port": 2083,
  "username": "myuser",
  "apiToken": "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX",
  "documentRoot": "/public_html"
}

// Response
{
  "server": { ... },
  "verified": true,
  "message": "Connection successful"
}
```

---

## cPanel API Integration

### Authentication

cPanel uses API tokens (preferred) or username/password. **Always use API tokens.**

```
Authorization: cpanel USERNAME:APITOKEN
```

### Key API Endpoints

All cPanel API calls go to:
```
https://{hostname}:{port}/execute/{module}/{function}
```

#### 1. Verify Connection
```
GET /execute/Ftp/list_ftp
```
Returns FTP accounts. If successful, connection is valid.

#### 2. List Addon Domains
```
GET /execute/DomainInfo/list_domains
```
Returns all domains (main, addon, subdomain, parked).

#### 3. Create Subdomain
```
POST /execute/SubDomain/addsubdomain
Parameters:
  - domain: "subdomain"
  - rootdomain: "example.com"
  - dir: "/public_html/subdomain.example.com" (optional)
```

#### 4. File Operations (File Manager API)
```
POST /execute/Fileman/upload_files
POST /execute/Fileman/save_file_content
GET /execute/Fileman/list_files
DELETE /execute/Fileman/delete
```

#### 5. SSL Certificate (AutoSSL)
```
POST /execute/SSL/start_autossl_check
GET /execute/SSL/get_autossl_problems
```

### Go cPanel Client

```go
// pkg/cpanel/client.go

type Client struct {
    hostname string
    port     int
    username string
    apiToken string
    http     *http.Client
}

func NewClient(hostname string, port int, username, apiToken string) *Client {
    return &Client{
        hostname: hostname,
        port:     port,
        username: username,
        apiToken: apiToken,
        http: &http.Client{
            Timeout: 30 * time.Second,
            Transport: &http.Transport{
                TLSClientConfig: &tls.Config{
                    // cPanel often uses self-signed certs
                    InsecureSkipVerify: true, // Or verify against known CAs
                },
            },
        },
    }
}

func (c *Client) baseURL() string {
    return fmt.Sprintf("https://%s:%d", c.hostname, c.port)
}

func (c *Client) request(method, endpoint string, params url.Values) ([]byte, error) {
    reqURL := fmt.Sprintf("%s/execute/%s", c.baseURL(), endpoint)
    
    var req *http.Request
    var err error
    
    if method == "GET" {
        reqURL += "?" + params.Encode()
        req, err = http.NewRequest("GET", reqURL, nil)
    } else {
        req, err = http.NewRequest("POST", reqURL, strings.NewReader(params.Encode()))
        req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    }
    if err != nil {
        return nil, err
    }
    
    // cPanel authentication header
    req.Header.Set("Authorization", fmt.Sprintf("cpanel %s:%s", c.username, c.apiToken))
    
    resp, err := c.http.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    return io.ReadAll(resp.Body)
}

// VerifyConnection tests if credentials work
func (c *Client) VerifyConnection() error {
    _, err := c.request("GET", "Ftp/list_ftp", nil)
    return err
}

// CreateSubdomain creates a subdomain
func (c *Client) CreateSubdomain(subdomain, rootDomain, docRoot string) error {
    params := url.Values{
        "domain":     {subdomain},
        "rootdomain": {rootDomain},
    }
    if docRoot != "" {
        params.Set("dir", docRoot)
    }
    
    resp, err := c.request("POST", "SubDomain/addsubdomain", params)
    if err != nil {
        return err
    }
    
    // Parse response for errors
    var result struct {
        Status int    `json:"status"`
        Errors []string `json:"errors"`
    }
    if err := json.Unmarshal(resp, &result); err != nil {
        return err
    }
    if result.Status == 0 && len(result.Errors) > 0 {
        return fmt.Errorf("cPanel error: %s", result.Errors[0])
    }
    
    return nil
}

// UploadFile uploads content to a path
func (c *Client) UploadFile(remotePath string, content []byte) error {
    // cPanel File Manager API for saving file content
    params := url.Values{
        "dir":      {filepath.Dir(remotePath)},
        "file":     {filepath.Base(remotePath)},
        "from_charset": {"utf-8"},
        "to_charset":   {"utf-8"},
    }
    
    // For file content, use save_file_content
    // This requires multipart form for binary, or base64 encoding
    // Implementation depends on file type
    
    // ... implementation details ...
    return nil
}

// DeleteFile removes a file
func (c *Client) DeleteFile(remotePath string) error {
    params := url.Values{
        "dir":  {filepath.Dir(remotePath)},
        "file": {filepath.Base(remotePath)},
    }
    _, err := c.request("POST", "Fileman/delete_files", params)
    return err
}

// ListSubdomains returns all subdomains for a domain
func (c *Client) ListSubdomains(rootDomain string) ([]string, error) {
    resp, err := c.request("GET", "DomainInfo/list_domains", nil)
    if err != nil {
        return nil, err
    }
    
    var result struct {
        Data struct {
            SubDomains []string `json:"sub_domains"`
        } `json:"data"`
    }
    if err := json.Unmarshal(resp, &result); err != nil {
        return nil, err
    }
    
    return result.Data.SubDomains, nil
}
```

---

## Deployment Flow

### Main Domain (existing)

```
1. User creates redirect link
2. Panel generates PHP/HTML
3. SSH to Deploy VPS
4. Write to /var/www/sites/{domain}/{subdomain}/index.php
5. nginx already configured (wildcard)
6. Done
```

### cPanel Domain (new)

```
1. User creates redirect link
2. Panel generates PHP/HTML (with embedded antibot SDK)
3. Get cPanel credentials from cpanel_servers (decrypt token)
4. Create cPanel client
5. Check if subdomain exists, create if not
6. Upload index.php to document_root
7. Verify file exists
8. Done
```

### Code Flow

```go
// modules/redirectlinks/handlers/handler.go

func (h *Handler) deployLink(link *models.RedirectLink, domain *models.Domain) error {
    content, fileType := getDeployContent(link)
    
    switch domain.HostingType {
    case "main":
        return h.deployToMainVPS(link, domain, content, fileType)
    case "cpanel":
        return h.deployToCpanel(link, domain, content, fileType)
    default:
        return fmt.Errorf("unknown hosting type: %s", domain.HostingType)
    }
}

func (h *Handler) deployToCpanel(link *models.RedirectLink, domain *models.Domain, content string, fileType deployFileType) error {
    // 1. Get cPanel server credentials
    server, err := h.cpanelService.GetServer(*domain.CpanelServerID)
    if err != nil {
        return fmt.Errorf("cPanel server not found: %w", err)
    }
    
    // 2. Decrypt API token
    apiToken, err := h.crypto.Decrypt(server.APITokenEncrypted)
    if err != nil {
        return fmt.Errorf("failed to decrypt API token: %w", err)
    }
    
    // 3. Create cPanel client
    client := cpanel.NewClient(server.Hostname, server.Port, server.Username, string(apiToken))
    
    // 4. Determine paths
    docRoot := server.DocumentRoot
    if domain.CpanelDocRoot != nil && *domain.CpanelDocRoot != "" {
        docRoot = *domain.CpanelDocRoot
    }
    
    // For subdomain: /public_html/subdomain.domain.com/
    subdomainDir := fmt.Sprintf("%s/%s.%s", docRoot, link.Subdomain, domain.BaseDomain())
    
    // 5. Create subdomain if needed
    if err := client.CreateSubdomain(link.Subdomain, domain.BaseDomain(), subdomainDir); err != nil {
        // Ignore "already exists" error
        if !strings.Contains(err.Error(), "already exists") {
            return fmt.Errorf("failed to create subdomain: %w", err)
        }
    }
    
    // 6. Upload file
    filename := "index.php"
    if fileType == deployHTML {
        filename = "index.html"
    }
    remotePath := fmt.Sprintf("%s/%s", subdomainDir, filename)
    
    if err := client.UploadFile(remotePath, []byte(content)); err != nil {
        return fmt.Errorf("failed to upload file: %w", err)
    }
    
    // 7. Clean up old file type if switching
    if fileType == deployHTML {
        _ = client.DeleteFile(fmt.Sprintf("%s/index.php", subdomainDir))
    } else {
        _ = client.DeleteFile(fmt.Sprintf("%s/index.html", subdomainDir))
    }
    
    return nil
}
```

---

## Security Considerations

### 1. API Token Encryption

**NEVER store plain API tokens.** Use AES-256-GCM encryption.

```go
// pkg/crypto/encryption.go

type Encryptor struct {
    key []byte // 32 bytes for AES-256
}

func NewEncryptor(keyHex string) (*Encryptor, error) {
    key, err := hex.DecodeString(keyHex)
    if err != nil || len(key) != 32 {
        return nil, errors.New("invalid encryption key")
    }
    return &Encryptor{key: key}, nil
}

func (e *Encryptor) Encrypt(plaintext []byte) ([]byte, error) {
    block, err := aes.NewCipher(e.key)
    if err != nil {
        return nil, err
    }
    
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }
    
    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return nil, err
    }
    
    return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func (e *Encryptor) Decrypt(ciphertext []byte) ([]byte, error) {
    block, err := aes.NewCipher(e.key)
    if err != nil {
        return nil, err
    }
    
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }
    
    nonceSize := gcm.NonceSize()
    if len(ciphertext) < nonceSize {
        return nil, errors.New("ciphertext too short")
    }
    
    nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
    return gcm.Open(nil, nonce, ciphertext, nil)
}
```

**Environment variable for key:**
```bash
# .env
CPANEL_ENCRYPTION_KEY=your-64-char-hex-string-for-32-bytes
```

### 2. Input Validation

- Validate hostname format (no URL schemes, no paths)
- Validate port range (1-65535, typically 2083 for cPanel SSL)
- Sanitize username (alphanumeric + underscore only)
- API token: minimum length, no whitespace

### 3. Rate Limiting

- Limit connection verification attempts (prevent brute force)
- Limit API calls per user per hour
- Log failed connection attempts

### 4. Audit Logging

```go
// Log all cPanel operations
type CpanelAuditLog struct {
    ID          string    `db:"id"`
    UserID      string    `db:"user_id"`
    ServerID    string    `db:"server_id"`
    Action      string    `db:"action"` // "verify", "create_subdomain", "upload", "delete"
    Target      string    `db:"target"` // subdomain or file path
    Success     bool      `db:"success"`
    ErrorMsg    string    `db:"error_msg"`
    IPAddress   string    `db:"ip_address"`
    CreatedAt   time.Time `db:"created_at"`
}
```

---

## UI/UX Flow

### Adding a cPanel Server

1. User goes to Settings > Hosting Servers
2. Click "Add cPanel Server"
3. Fill form:
   - Name (friendly label)
   - Hostname (server.example.com)
   - Port (default 2083)
   - cPanel Username
   - API Token (with link to "How to get API token")
4. Click "Verify & Save"
5. System tests connection
6. If successful, server is saved
7. If failed, show specific error (auth failed, connection timeout, etc.)

### Adding a cPanel Domain

1. User goes to Domains > Add Domain
2. Select "Domain Type":
   - **Managed** (deployed to main VPS) - default
   - **Self-Hosted** (your cPanel server)
3. If Self-Hosted:
   - Select cPanel server from dropdown
   - Enter domain name
   - Optional: Custom document root
4. Domain verification:
   - For managed: DNS verification
   - For self-hosted: File verification (upload .well-known/domain-verify)
5. Done

### Visual Indicators

- Managed domains: Show server icon
- cPanel domains: Show cPanel logo + server name
- Clear labeling of which server hosts each domain

---

## Error Handling

### Connection Errors

| Error | User Message | Action |
|-------|--------------|--------|
| Connection refused | "Cannot connect to server. Check hostname and port." | Verify firewall, port |
| SSL error | "SSL certificate error. The server may use a self-signed certificate." | Option to allow |
| Auth failed | "Invalid username or API token." | Re-enter credentials |
| Timeout | "Server did not respond in time." | Retry later |

### Deployment Errors

| Error | User Message | Action |
|-------|--------------|--------|
| Subdomain exists | Proceed (not an error) | Use existing |
| Permission denied | "Cannot create files. Check cPanel permissions." | User fixes in cPanel |
| Disk quota | "Server disk quota exceeded." | User clears space |
| Domain not on server | "Domain not found on this cPanel server." | User adds domain to cPanel first |

### Recovery

- All operations should be idempotent (safe to retry)
- Store operation state for rollback if needed
- Provide manual retry button for failed deployments

---

## Testing Checklist

### Unit Tests

- [ ] Encryption/decryption of API tokens
- [ ] cPanel client URL building
- [ ] Response parsing (success and error cases)
- [ ] Hostname validation
- [ ] Port validation

### Integration Tests (with test cPanel server)

- [ ] Connection verification
- [ ] Create subdomain
- [ ] Upload file
- [ ] Delete file
- [ ] List subdomains
- [ ] Handle already-exists errors gracefully

### E2E Tests

- [ ] Full flow: Add server → Add domain → Create link → Deploy
- [ ] Edit link → Redeploy
- [ ] Delete link → Cleanup files
- [ ] Remove server → Domains become orphaned (handle gracefully)

### Security Tests

- [ ] API tokens never logged
- [ ] API tokens never in API responses
- [ ] Encrypted tokens cannot be decrypted without key
- [ ] Rate limiting works
- [ ] Invalid hostnames rejected

---

## Migration Plan

1. **Phase 1: Schema** - Add tables, columns (backwards compatible)
2. **Phase 2: Backend** - Implement services, handlers
3. **Phase 3: UI** - Add server management, domain type selection
4. **Phase 4: Deployment** - Implement cPanel deployment path
5. **Phase 5: Testing** - Full test suite
6. **Phase 6: Rollout** - Feature flag, gradual enablement

---

## File Structure

```
modules/cpanel/
├── module.go                 # Module init, routes
├── models/
│   ├── server.go            # CpanelServer model
│   └── audit_log.go         # Audit logging model
├── services/
│   └── cpanel_service.go    # Business logic
├── handlers/
│   └── handler.go           # HTTP handlers
├── templates/
│   ├── servers.html         # List servers
│   ├── server_form.html     # Add/edit server
│   └── _server_card.html    # Server card partial
└── migrations/
    ├── 001_create_servers.sql
    └── 002_add_domain_columns.sql

pkg/cpanel/
├── client.go                # cPanel API client
├── client_test.go           # Client tests
└── errors.go                # Error types

pkg/crypto/
├── encryption.go            # AES-256-GCM encryption
└── encryption_test.go
```

---

## Environment Variables

```bash
# Required for cPanel integration
CPANEL_ENCRYPTION_KEY=<64-char-hex>  # Generate: openssl rand -hex 32

# Optional
CPANEL_DEFAULT_PORT=2083
CPANEL_VERIFY_SSL=false              # Set true for production with valid certs
CPANEL_TIMEOUT_SECONDS=30
```

---

## Summary

This integration adds user-managed hosting alongside the main VPS. Key points:

1. **Two hosting types**: Main (admin VPS) and cPanel (user hosting)
2. **Secure credential storage**: AES-256-GCM encrypted API tokens
3. **cPanel API**: Use UAPI for all operations
4. **Embedded antibot**: cPanel sites include self-hosted SDK (no botection proxy)
5. **Graceful errors**: Clear messages, retry capability
6. **Audit trail**: Log all operations for security

The implementation is modular and doesn't affect existing main-domain functionality.
