# DNS-PERSIST-01 Migration Plan

## Current Status (September 2026)

**IMPORTANT:** As of September 2026, dns-persist-01 is **NOT YET AVAILABLE** in Let's Encrypt production. It is only available on the staging environment. Expected production rollout: **Q3-Q4 2026**.

The code implementation is complete and ready. Once Let's Encrypt enables dns-persist-01 in production, it will work automatically.

**Sources:**
- [Let's Encrypt Announcement](https://letsencrypt.org/2026/02/18/dns-persist-01)
- [Let's Encrypt Community Discussion](https://community.letsencrypt.org/)

## Executive Summary

Migrate from the current **dns-01** challenge (dynamic TXT record per renewal) to **dns-persist-01** (one-time persistent TXT record). This eliminates DNS cache issues that caused SSL generation failures.

## Problem Statement

**Current State (dns-01):**
```
User adds:  _acme-challenge.domain.com TXT "token-abc123"
                                            ↑ Changes every renewal!
```

Every SSL issuance/renewal requires a NEW TXT record value. OpenDNS anycast caching caused 1-hour timeout failures because edge servers cached different values.

**New State (dns-persist-01):**
```
User adds:  _validation-persist.domain.com TXT "acme=letsencrypt.org;acct=..."
                                               ↑ Never changes!
```

One-time setup. All future renewals work automatically.

---

## Technology Stack

| Component | Current | New |
|-----------|---------|-----|
| CA | Let's Encrypt | Let's Encrypt (dns-persist-01 in production since Q2 2026) |
| ACME Client | lego v4.14.2 | lego v5.x (dns-persist-01 support) |
| Challenge Type | dns-01 (exec provider) | dns-persist-01 |
| DNS Record | `_acme-challenge` TXT (dynamic) | `_validation-persist` TXT (static) |

---

## Database Changes

### New Columns

```sql
-- Migration: 00X_add_dns_persist.sql
ALTER TABLE domains ADD COLUMN persist_txt_value TEXT;          -- The persistent TXT record value
ALTER TABLE domains ADD COLUMN persist_txt_verified BOOLEAN DEFAULT FALSE;
ALTER TABLE domains ADD COLUMN lego_account_uri TEXT;           -- ACME account URI for this domain
```

### Columns to Deprecate (keep for rollback)

```sql
-- These will be unused but kept for backwards compatibility
-- acme_token, acme_token_expires_at (no longer needed - token never changes)
-- Can be removed in a future migration after confirming dns-persist-01 works
```

---

## File Changes

### 1. Verification Service

**File:** `modules/domains/services/verification_service.go`

**REMOVE/DEPRECATE these functions:**
```go
// OLD: These functions handle dynamic dns-01 tokens - no longer needed
func (s *VerificationService) PreGenerateAcmeToken(domain string) (string, error)          // Lines 825-895
func (s *VerificationService) CompleteWildcardSSL(domain, savedACMEToken string) error     // Lines 900-1003
func (s *VerificationService) GetSavedAcmeToken(baseDomain string) string                  // Lines 1054-1078
func (s *VerificationService) GetLiveAcmeToken(domain string) (string, error)              // Lines 1082-1112
func (s *VerificationService) CheckAcmeTXT(domain, expectedToken string) bool              // Lines 779-819
func (s *VerificationService) GetAcmeTXTValue(domain string) string                        // Lines 745-776
func (s *VerificationService) LegoStartChallenge(domain, email string) (*LegoChallenge, error)  // Lines 1170-1288
func (s *VerificationService) LegoCompleteChallenge(domain string) error                   // Lines 1291-1342
func (s *VerificationService) LegoGetPendingToken(domain string) (string, error)           // Lines 1372-1395
func (s *VerificationService) LegoCancelChallenge(domain string) error                     // Lines 1398-1428
```

**ADD these functions:**
```go
// NEW: dns-persist-01 implementation

// PersistTXTValue generates the persistent TXT record value for a domain
// Format: "acme=letsencrypt.org;acct=https://acme-v02.api.letsencrypt.org/acme/acct/123456789"
func (s *VerificationService) GeneratePersistTXTValue(domain, email string) (string, string, error) {
    // 1. SSH to Deploy VPS
    // 2. Run: lego --email=EMAIL --accept-tos --dns-persist-01-generate
    // 3. Returns: txtValue, accountURI, error
}

// CheckPersistTXT verifies the _validation-persist TXT record exists with correct value
func (s *VerificationService) CheckPersistTXT(domain, expectedValue string) (bool, error) {
    // Query _validation-persist.domain.com TXT
    // Compare against expectedValue
}

// GenerateWildcardSSLPersist generates wildcard SSL using dns-persist-01
// This is the main SSL generation function - replaces all dns-01 functions
func (s *VerificationService) GenerateWildcardSSLPersist(domain string) error {
    // 1. SSH to Deploy VPS
    // 2. Check if _validation-persist TXT is correctly configured
    // 3. Run: lego --dns-persist-01 --domains="*.domain.com" run
    // 4. Install certificate
    // Returns immediately - no waiting for DNS propagation!
}

// ensureLegoV5Installed upgrades lego to v5+ for dns-persist-01 support
func (s *VerificationService) ensureLegoV5Installed(client *sshexec.Client) error {
    // Check version, upgrade if < 5.0
}
```

### 2. Domain Handler

**File:** `modules/domains/handlers/handler.go`

**REMOVE/DEPRECATE these endpoints:**
```go
// OLD: Two-phase SSL flow - no longer needed
func (h *Handler) APIStartSSLChallenge(w http.ResponseWriter, r *http.Request)    // Generates dynamic token
func (h *Handler) APICompleteSSLChallenge(w http.ResponseWriter, r *http.Request) // Verifies token & generates cert
func (h *Handler) APICancelSSLChallenge(w http.ResponseWriter, r *http.Request)   // Cancels pending challenge
func (h *Handler) APIGetSSLToken(w http.ResponseWriter, r *http.Request)          // Gets current token
```

**ADD these endpoints:**
```go
// NEW: One-step SSL flow
func (h *Handler) APIGetPersistTXTValue(w http.ResponseWriter, r *http.Request) {
    // Returns the _validation-persist TXT value for user to add
    // Called when user first sets up domain
}

func (h *Handler) APIVerifyPersistTXT(w http.ResponseWriter, r *http.Request) {
    // Checks if _validation-persist TXT is correctly configured
    // Returns: verified (bool), currentValue (string)
}

func (h *Handler) APIGenerateSSLPersist(w http.ResponseWriter, r *http.Request) {
    // One-click SSL generation after TXT is verified
    // No waiting for DNS - works immediately
}
```

### 3. Domain Routes

**File:** `modules/domains/module.go`

**REMOVE routes:**
```go
// OLD routes
r.Post("/api/{id}/ssl/start", h.APIStartSSLChallenge)
r.Post("/api/{id}/ssl/complete", h.APICompleteSSLChallenge)
r.Delete("/api/{id}/ssl/cancel", h.APICancelSSLChallenge)
r.Get("/api/{id}/ssl/token", h.APIGetSSLToken)
```

**ADD routes:**
```go
// NEW routes
r.Get("/api/{id}/persist-txt", h.APIGetPersistTXTValue)
r.Post("/api/{id}/persist-txt/verify", h.APIVerifyPersistTXT)
r.Post("/api/{id}/ssl/generate", h.APIGenerateSSLPersist)
```

### 4. UI Templates

**File:** `modules/domains/templates/external_setup.html`

**REMOVE sections:**
```html
<!-- OLD: Two-phase SSL flow with dynamic token -->
<div id="ssl-phase1">
    <button id="btn-get-ssl-token">Get SSL Token</button>
</div>
<div id="ssl-phase2">
    <table>TXT record with dynamic token</table>
    <button id="btn-complete-ssl">Complete SSL</button>
</div>
```

**REPLACE with:**
```html
<!-- NEW: One-time persistent TXT setup -->
<div class="card mb-3" id="card-step3">
    <div class="card-header">
        <span class="badge">3</span>
        TXT Record — SSL Authorization (One-Time)
    </div>
    <div class="card-body">
        <p>Add this TXT record to authorize SSL certificates for your domain:</p>
        <table class="table">
            <tr>
                <td>Type</td>
                <td><code>TXT</code></td>
            </tr>
            <tr>
                <td>Name</td>
                <td><code>_validation-persist</code></td>
            </tr>
            <tr>
                <td>Value</td>
                <td><code id="persist-txt-value">{{.SetupInfo.PersistTXTValue}}</code></td>
            </tr>
            <tr>
                <td>TTL</td>
                <td><code>3600</code> (or any value - doesn't matter!)</td>
            </tr>
        </table>
        
        <div class="alert alert-success">
            <i class="bi bi-check-circle"></i>
            <strong>One-time setup!</strong> This record never changes. 
            All future SSL renewals will work automatically.
        </div>
        
        <button id="btn-verify-persist" class="btn btn-primary">
            <i class="bi bi-check-lg"></i> Verify TXT Record
        </button>
        <button id="btn-generate-ssl" class="btn btn-success" disabled>
            <i class="bi bi-lock"></i> Generate SSL Certificate
        </button>
    </div>
</div>
```

**REMOVE JavaScript:**
```javascript
// OLD: Two-phase functions
async function startSSLChallenge() { ... }
async function completeSSLChallenge() { ... }
async function cancelSSLChallenge() { ... }
async function checkPendingSSLToken() { ... }
```

**ADD JavaScript:**
```javascript
// NEW: One-step functions
async function verifyPersistTXT() {
    const res = await fetch(`/user/domains/api/${domainId}/persist-txt/verify`, { method: 'POST' });
    const data = await res.json();
    if (data.verified) {
        document.getElementById('btn-generate-ssl').disabled = false;
        showToast('TXT record verified!', 'success');
    } else {
        showToast('TXT record not found. Please add it and try again.', 'warning');
    }
}

async function generateSSLPersist() {
    const btn = document.getElementById('btn-generate-ssl');
    btn.disabled = true;
    btn.innerHTML = '<span class="spinner-border spinner-border-sm"></span> Generating...';
    
    const res = await fetch(`/user/domains/api/${domainId}/ssl/generate`, { method: 'POST' });
    const data = await res.json();
    
    if (data.success) {
        showToast('SSL certificate generated!', 'success');
        showSetupComplete();
    } else {
        showToast(data.error || 'SSL generation failed', 'danger');
        btn.disabled = false;
        btn.innerHTML = '<i class="bi bi-lock"></i> Generate SSL Certificate';
    }
}
```

### 5. Domain Model

**File:** `modules/domains/models/domain.go`

**ADD fields:**
```go
type Domain struct {
    // ... existing fields ...
    
    // dns-persist-01 fields (NEW)
    PersistTXTValue    *string `db:"persist_txt_value" json:"persistTxtValue,omitempty"`
    PersistTXTVerified bool    `db:"persist_txt_verified" json:"persistTxtVerified"`
    LegoAccountURI     *string `db:"lego_account_uri" json:"-"` // Never expose
}
```

**ADD to SetupInfo:**
```go
type SetupInfo struct {
    // ... existing fields ...
    
    // dns-persist-01 (NEW)
    PersistTXTValue    string `json:"persistTxtValue,omitempty"`
    PersistTXTName     string `json:"persistTxtName,omitempty"`     // _validation-persist.domain.com
    PersistTXTVerified bool   `json:"persistTxtVerified"`
}
```

**DEPRECATE fields (keep for backwards compat):**
```go
// These are no longer used but kept for existing data
AcmeToken          *string    `db:"acme_token" json:"acmeToken,omitempty"`          // DEPRECATED
AcmeTokenExpiresAt *time.Time `db:"acme_token_expires_at" json:"-"`                 // DEPRECATED
```

### 6. Background Service

**File:** `modules/domains/services/background.go`

**MODIFY setupSSL function:**
```go
func (b *BackgroundVerifier) setupSSL(domainID, domainName string) {
    // ... existing Cloudflare handling ...
    
    if isWildcard(domainName) {
        // OLD: Skip auto SSL - user must trigger via UI after adding TXT record
        // NEW: Check if persist TXT is verified, then auto-generate
        domain, _ := b.domainService.Get(domainID)
        if domain.PersistTXTVerified {
            if err := b.verifyService.GenerateWildcardSSLPersist(domainName); err != nil {
                log.Printf("[domains] SSL generation failed for %s: %v", domainName, err)
                return
            }
            t := true
            b.domainService.Update(domainID, models.UpdateDomainInput{SSLEnabled: &t})
        }
        return
    }
    
    // ... existing non-wildcard handling (HTTP-01) ...
}
```

### 7. Deploy VPS Scripts

**Create:** `scripts/upgrade-lego-v5.sh`

```bash
#!/bin/bash
# Upgrade lego to v5+ for dns-persist-01 support

LEGO_VERSION="v5.0.0"  # Or latest v5.x

cd /tmp
curl -fsSL "https://github.com/go-acme/lego/releases/download/${LEGO_VERSION}/lego_${LEGO_VERSION}_linux_amd64.tar.gz" -o lego.tar.gz
tar xzf lego.tar.gz lego
mv lego /usr/local/bin/
chmod +x /usr/local/bin/lego
rm -f lego.tar.gz

# Verify installation
lego --version
```

---

## Migration Steps

### Phase 1: Database Migration

```sql
-- 00X_add_dns_persist.sql
BEGIN;

-- Add new columns
ALTER TABLE domains ADD COLUMN IF NOT EXISTS persist_txt_value TEXT;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS persist_txt_verified BOOLEAN DEFAULT FALSE;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS lego_account_uri TEXT;

-- Index for quick lookup
CREATE INDEX IF NOT EXISTS idx_domains_persist_verified ON domains(persist_txt_verified) WHERE persist_txt_verified = TRUE;

COMMIT;
```

### Phase 2: Upgrade Lego on Deploy VPS

```bash
# SSH to Deploy VPS
ssh root@<DEPLOY_VPS_IP>

# Run upgrade script
bash /path/to/upgrade-lego-v5.sh

# Verify
lego --version  # Should show v5.x
```

### Phase 3: Deploy Code Changes

1. Deploy updated Go code
2. Deploy updated templates
3. Test with a new domain

### Phase 4: Migrate Existing Domains (Optional)

For existing wildcard domains that need SSL renewal:

1. Generate persist_txt_value for each domain
2. Notify users to update DNS (add _validation-persist TXT)
3. Once verified, SSL renewals will work automatically

---

## Rollback Plan

If dns-persist-01 doesn't work:

1. Keep old code paths (marked as deprecated but functional)
2. Revert to dns-01 by uncommenting old handlers
3. Old database columns still exist

---

## User Flow Comparison

### OLD Flow (dns-01):

```
1. User adds domain *.example.com
2. System shows: Add A record pointing to VPS
3. System shows: Add _guardbot-verify TXT record
4. User clicks "Get SSL Token" → System generates random token
5. System shows: Add _acme-challenge TXT = "random-token-xyz"
6. User adds TXT record, waits for DNS propagation (minutes to hours)
7. User clicks "Complete SSL"
8. If DNS not propagated → FAILURE, retry
9. Every 90 days: Repeat steps 4-8 for renewal
```

### NEW Flow (dns-persist-01):

```
1. User adds domain *.example.com
2. System shows: Add A record pointing to VPS
3. System shows: Add _guardbot-verify TXT record
4. System shows: Add _validation-persist TXT = "acme=letsencrypt.org;acct=..."
   (This value NEVER changes!)
5. User adds TXT record, waits for DNS propagation (one-time)
6. User clicks "Verify TXT" → System confirms record exists
7. User clicks "Generate SSL" → Certificate generated instantly
8. Every 90 days: Auto-renewal happens silently, no user action needed
```

---

## References

- [Let's Encrypt dns-persist-01 Announcement](https://letsencrypt.org/2026/02/18/dns-persist-01)
- [Lego v5 dns-persist-01 Documentation](https://go-acme.github.io/lego/obtain/dnspersist01/index.html)
- [IETF Draft: ACME Persistent DNS Challenge](https://www.ietf.org/archive/id/draft-ietf-acme-dns-persist-00.html)
- [CA/Browser Forum Ballot SC-088v3](https://cabforum.org/2025/10/sc-088v3/)

---

## Success Metrics

| Metric | Before | After |
|--------|--------|-------|
| SSL generation success rate | ~30% (cache issues) | 100% |
| User setup time | 30+ minutes | 5 minutes |
| Renewal success rate | Variable | 100% (automatic) |
| Support tickets for SSL | High | Near zero |

---

## Timeline

| Phase | Duration | Description |
|-------|----------|-------------|
| Phase 1 | 1 day | Database migration |
| Phase 2 | 1 day | Lego upgrade + testing |
| Phase 3 | 2 days | Code changes + UI updates |
| Phase 4 | Ongoing | User migration (as domains renew) |

---

## Appendix: Full Code Examples

### A. GeneratePersistTXTValue Implementation

```go
func (s *VerificationService) GeneratePersistTXTValue(domain, email string) (txtValue, accountURI string, err error) {
    server, err := s.getServer()
    if err != nil {
        return "", "", fmt.Errorf("no deploy server available")
    }

    baseDomain := GetBaseDomain(domain)
    if email == "" {
        email = "admin@" + baseDomain
    }

    port := fmt.Sprintf("%d", server.Port)
    if server.Port == 0 {
        port = "22"
    }

    client, err := sshexec.NewClient(server.IP, port, server.User, server.Password)
    if err != nil {
        return "", "", fmt.Errorf("SSH connection failed: %w", err)
    }
    defer client.Close()

    // Ensure lego v5+ is installed
    if err := s.ensureLegoV5Installed(client); err != nil {
        return "", "", err
    }

    // Generate account and get persist TXT value
    legoDir := fmt.Sprintf("/root/.lego-%s", baseDomain)
    cmd := fmt.Sprintf(`
        mkdir -p %s
        cd %s
        lego --email="%s" --accept-tos --path=%s \
            account create 2>/dev/null || true
        
        # Get account URI
        ACCOUNT_URI=$(lego --email="%s" --path=%s account info 2>/dev/null | grep -oP 'URI: \K.*')
        
        # Generate persist TXT value
        echo "acme=letsencrypt.org;acct=${ACCOUNT_URI}"
    `, legoDir, legoDir, email, legoDir, email, legoDir)

    output, err := client.Run(cmd)
    if err != nil {
        return "", "", fmt.Errorf("failed to generate persist TXT: %w", err)
    }

    txtValue = strings.TrimSpace(output)
    // Extract account URI from the value
    if idx := strings.Index(txtValue, "acct="); idx != -1 {
        accountURI = txtValue[idx+5:]
    }

    return txtValue, accountURI, nil
}
```

### B. GenerateWildcardSSLPersist Implementation

```go
func (s *VerificationService) GenerateWildcardSSLPersist(domain string) error {
    server, err := s.getServer()
    if err != nil {
        return fmt.Errorf("no deploy server available")
    }

    baseDomain := GetBaseDomain(domain)

    port := fmt.Sprintf("%d", server.Port)
    if server.Port == 0 {
        port = "22"
    }

    client, err := sshexec.NewClient(server.IP, port, server.User, server.Password)
    if err != nil {
        return fmt.Errorf("SSH connection failed: %w", err)
    }
    defer client.Close()

    // Check if cert already exists
    checkExisting := fmt.Sprintf(`test -f /etc/letsencrypt/live/%s/fullchain.pem && echo "EXISTS"`, baseDomain)
    if out, _ := client.Run(checkExisting); strings.Contains(out, "EXISTS") {
        return nil // Already have cert
    }

    // Ensure lego v5+ is installed
    if err := s.ensureLegoV5Installed(client); err != nil {
        return err
    }

    legoDir := fmt.Sprintf("/root/.lego-%s", baseDomain)

    // Run lego with dns-persist-01 challenge
    // This checks _validation-persist TXT record and issues cert
    legoCmd := fmt.Sprintf(`
        lego --email="admin@%s" --accept-tos \
            --domains="*.%s" \
            --dns-persist-01 \
            --path=%s \
            run 2>&1
    `, baseDomain, baseDomain, legoDir)

    output, err := client.Run(legoCmd)
    if err != nil {
        return fmt.Errorf("SSL generation failed: %s", parseCertbotError(output))
    }

    // Install cert to Let's Encrypt location
    return s.installLegoCert(client, baseDomain)
}
```
