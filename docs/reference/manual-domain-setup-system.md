# Manual Domain Setup System — Complete Reference

This document provides 100% complete documentation for the manual domain setup process, where users add DNS records manually in their DNS provider instead of using the cPanel API automation. Every code example is copied verbatim from the working implementation with absolute file paths.

**Target Audience:** AI agents or developers porting this system to a new platform.

---

## Table of Contents

1. [Overview](#overview)
2. [Reference Files](#reference-files)
3. [Setup Flow](#setup-flow)
4. [Data Models](#data-models)
5. [Backend Handlers](#backend-handlers)
6. [DNS Verification Service](#dns-verification-service)
7. [Acme-DNS Integration](#acme-dns-integration)
8. [Frontend Template](#frontend-template)
9. [API Endpoints](#api-endpoints)
10. [Porting Checklist](#porting-checklist)

---

## Overview

The manual setup flow is used when:
1. A user adds a domain from cPanel but doesn't use the cPanel API automation
2. A user adds an external domain (not hosted on cPanel)
3. The cPanel API automation fails and the user falls back to manual

The user is shown a setup wizard with DNS records to add manually. The system polls for DNS propagation and automatically generates SSL once verified.

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Manual Setup Flow                            │
├─────────────────────────────────────────────────────────────────────┤
│  1. User adds domain → system generates verify_token                │
│  2. System registers with acme-dns → gets CNAME target              │
│  3. User adds DNS records manually in their DNS provider:           │
│     - A record: * → Deploy server IP                                │
│     - TXT record: _guardbot-verify → verify_token                   │
│     - CNAME record: _acme-challenge → acme-dns subdomain            │
│  4. Frontend polls /api/{id}/dns-status every 5 seconds             │
│  5. Once all records verified → user clicks "Generate SSL"         │
│  6. System generates wildcard SSL via acme-dns CNAME delegation     │
│  7. Setup complete                                                   │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Reference Files

| Absolute Path | Purpose |
|--------------|---------|
| `/home/this/dev/code/botginx/modules/domains/handlers/handler.go` | ExternalSetup handler (lines 932-998), DNS status API (lines 643-669) |
| `/home/this/dev/code/botginx/modules/domains/templates/external_setup.html` | Setup wizard UI with JavaScript polling |
| `/home/this/dev/code/botginx/modules/domains/services/verification_service.go` | DNS checking, SSL generation |
| `/home/this/dev/code/botginx/modules/domains/services/domain_service.go` | Acme-DNS registration (lines 514-580) |
| `/home/this/dev/code/botginx/modules/domains/models/domain.go` | ExternalSetupInfo model (lines 121-138) |
| `/home/this/dev/code/botginx/modules/domains/module.go` | Route definitions |
| `/home/this/dev/code/botginx/pkg/acmedns/client.go` | Acme-DNS SSH client |

---

## Setup Flow

### Entry Point: ExternalSetup Handler

**File:** `/home/this/dev/code/botginx/modules/domains/handlers/handler.go` (lines 932-998)

```go
// ExternalSetup shows the setup wizard for external/cPanel domains
func (h *Handler) ExternalSetup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	domain, err := h.service.Get(id)
	if err != nil {
		http.Error(w, "Domain not found", http.StatusNotFound)
		return
	}

	// Check ownership
	if !h.canAccessDomain(r, domain) {
		http.Error(w, "Not authorized", http.StatusForbidden)
		return
	}

	// Get deploy server IP
	deployIP := h.service.GetDeployIP()

	// Build setup info
	baseDomain := services.GetBaseDomain(domain.Name)
	// cPanel domains are treated as wildcard - we only care about *.domain.com, not the root @
	isCpanel := domain.CpanelConnectionID != nil && *domain.CpanelConnectionID != ""
	isWildcard := domain.IsWildcard || len(domain.Name) > 2 && domain.Name[:2] == "*." || isCpanel

	setupInfo := models.ExternalSetupInfo{
		Domain:        domain.Name,
		BaseDomain:    baseDomain,
		IsWildcard:    isWildcard,
		ServerIP:      deployIP,
		VerifyToken:   domain.VerifyToken,
		SetupStep:     domain.SetupStep,
		VerifyTXTName: "_guardbot-verify." + baseDomain,
		AcmeTXTName:   "_acme-challenge." + baseDomain,
	}

	// Check for acme-dns registration (all domains use wildcard SSL)
	if domain.AcmeFulldomain != nil && *domain.AcmeFulldomain != "" {
		setupInfo.AcmeCnameTarget = *domain.AcmeFulldomain
		setupInfo.AcmeCnameVerified = domain.AcmeCnameVerified
		setupInfo.UseAcmeDns = true

		// Verify CNAME if not yet verified
		if !domain.AcmeCnameVerified {
			if h.verification.CheckAcmeCnameRecord(domain.Name, *domain.AcmeFulldomain) {
				// CNAME verified - update database
				verified := true
				h.service.Update(domain.ID, models.UpdateDomainInput{AcmeCnameVerified: &verified})
				setupInfo.AcmeCnameVerified = true
			}
		}
	} else {
		// All domains need acme-dns for wildcard SSL support
		go h.registerAcmeDnsBackground(domain.ID)
	}

	// Legacy ACME token (fallback if acme-dns not available)
	if domain.AcmeToken != nil && *domain.AcmeToken != "" {
		setupInfo.AcmeToken = *domain.AcmeToken
		setupInfo.AcmeTokenReady = true
	}

	module.RenderUserSection(w, r, h.templates, "domains:external_setup.html", map[string]interface{}{
		"Title":     "Setup " + domain.Name,
		"Domain":    domain,
		"SetupInfo": setupInfo,
	})
}
```

### Key Difference: cPanel vs External Domains

**File:** `/home/this/dev/code/botginx/modules/domains/templates/external_setup.html` (lines 56-99)

```html
{{if .SetupInfo.IsWildcard}}
<!-- cPanel user: only * record needed -->
<tr>
    <td><span class="badge bg-primary">A</span></td>
    <td><code>*</code></td>
    <td>
        <div class="input-group input-group-sm" style="max-width:300px">
            <input type="text" class="form-control form-control-sm" value="{{.SetupInfo.ServerIP}}" readonly id="copy-ip">
            <button class="btn btn-outline-secondary" type="button" onclick="copyField('copy-ip')">
                <i class="bi bi-clipboard"></i>
            </button>
        </div>
    </td>
    <td></td>
</tr>
{{else}}
<!-- External DNS user: both @ and * records needed for subdomain support -->
<tr>
    <td><span class="badge bg-primary">A</span></td>
    <td><code>@</code></td>
    <td>
        <div class="input-group input-group-sm" style="max-width:300px">
            <input type="text" class="form-control form-control-sm" value="{{.SetupInfo.ServerIP}}" readonly id="copy-ip-base">
            <button class="btn btn-outline-secondary" type="button" onclick="copyField('copy-ip-base')">
                <i class="bi bi-clipboard"></i>
            </button>
        </div>
    </td>
    <td><span id="badge-a-base" class="badge bg-secondary"><i class="bi bi-hourglass-split"></i></span></td>
</tr>
<tr>
    <td><span class="badge bg-primary">A</span></td>
    <td><code>*</code></td>
    <td>
        <div class="input-group input-group-sm" style="max-width:300px">
            <input type="text" class="form-control form-control-sm" value="{{.SetupInfo.ServerIP}}" readonly id="copy-ip">
            <button class="btn btn-outline-secondary" type="button" onclick="copyField('copy-ip')">
                <i class="bi bi-clipboard"></i>
            </button>
        </div>
    </td>
    <td><span id="badge-a-wild" class="badge bg-secondary"><i class="bi bi-hourglass-split"></i></span></td>
</tr>
{{end}}
```

**Why the difference:**
- **cPanel domains:** The root domain (`@`) stays on cPanel hosting, only subdomains route to antibot. Only `*` A record needed.
- **External domains:** Both root and subdomains route to antibot. Both `@` and `*` A records needed.

---

## Data Models

### ExternalSetupInfo

**File:** `/home/this/dev/code/botginx/modules/domains/models/domain.go` (lines 121-138)

```go
// ExternalSetupInfo contains all info needed for the setup wizard
type ExternalSetupInfo struct {
	Domain          string `json:"domain"`
	BaseDomain      string `json:"baseDomain"` // For wildcard, the root domain
	IsWildcard      bool   `json:"isWildcard"`
	ServerIP        string `json:"serverIp"`
	VerifyToken     string `json:"verifyToken"`
	AcmeToken       string `json:"acmeToken"`
	AcmeTokenReady  bool   `json:"acmeTokenReady"`
	SetupStep       string `json:"setupStep"`
	VerifyTXTName   string `json:"verifyTxtName"` // _guardbot-verify.domain.com
	AcmeTXTName     string `json:"acmeTxtName"`   // _acme-challenge.domain.com (legacy)

	// acme-dns CNAME delegation (new - 100% reliable)
	AcmeCnameTarget   string `json:"acmeCnameTarget,omitempty"`   // abc123.<ACME_DNS_DOMAIN>
	AcmeCnameVerified bool   `json:"acmeCnameVerified"`
	UseAcmeDns        bool   `json:"useAcmeDns"` // true = use CNAME delegation
}
```

### Domain Model (Acme-DNS Fields)

**File:** `/home/this/dev/code/botginx/modules/domains/models/domain.go` (lines 43-49)

```go
// acme-dns delegation fields (for 100% reliable wildcard SSL)
AcmeSubdomain     *string `db:"acme_subdomain" json:"acmeSubdomain,omitempty"`
AcmeUsername      *string `db:"acme_username" json:"-"`  // For API auth (X-Api-User)
AcmePassword      *string `db:"acme_password" json:"-"`  // Never expose in JSON
AcmeFulldomain    *string `db:"acme_fulldomain" json:"acmeFulldomain,omitempty"`
AcmeCnameVerified bool    `db:"acme_cname_verified" json:"acmeCnameVerified"`
```

---

## Backend Handlers

### DNS Status Check

**File:** `/home/this/dev/code/botginx/modules/domains/handlers/handler.go` (lines 643-669)

Called by frontend polling every 5 seconds:

```go
// APIGetDNSStatus returns individual DNS record verification status
func (h *Handler) APIGetDNSStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	domain, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Domain not found", http.StatusNotFound)
		return
	}
	if !h.canAccessDomain(r, domain) {
		h.jsonError(w, "Not authorized", http.StatusForbidden)
		return
	}

	// Trust database if already verified (DNS cache can return stale negatives)
	// Only do live check if not yet verified in database
	txtVerified := domain.DNSVerified || h.verification.CheckTXTRecord(domain.Name, domain.VerifyToken)
	aRecordVerified := domain.DNSVerified || h.verification.CheckWildcardARecord(domain.Name)
	cnameVerified := domain.AcmeCnameVerified || h.verification.CheckAcmeCNAME(domain.Name)

	h.json(w, http.StatusOK, map[string]interface{}{
		"txtVerified":     txtVerified,
		"aRecordVerified": aRecordVerified,
		"cnameVerified":   cnameVerified,
		"allVerified":     txtVerified && aRecordVerified && cnameVerified,
	})
}
```

### Acme-DNS Registration

**File:** `/home/this/dev/code/botginx/modules/domains/handlers/handler.go` (lines 1032-1070)

```go
// APIRegisterAcmeDNS manually triggers acme-dns registration
func (h *Handler) APIRegisterAcmeDNS(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	domain, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Domain not found", http.StatusNotFound)
		return
	}

	// Check ownership
	if !h.canAccessDomain(r, domain) {
		h.jsonError(w, "Not authorized", http.StatusForbidden)
		return
	}

	// Already registered?
	if domain.AcmeSubdomain != nil && *domain.AcmeSubdomain != "" {
		h.json(w, http.StatusOK, map[string]interface{}{
			"registered":  true,
			"fulldomain":  domain.AcmeFulldomain,
			"cnameTarget": domain.AcmeFulldomain,
		})
		return
	}

	// Register
	reg, err := h.service.RegisterWithAcmeDNS(id)
	if err != nil {
		h.jsonError(w, "Registration failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"registered":  true,
		"subdomain":   reg.Subdomain,
		"fulldomain":  reg.Fulldomain,
		"cnameTarget": reg.Fulldomain,
	})
}
```

### CNAME Verification

**File:** `/home/this/dev/code/botginx/modules/domains/handlers/handler.go` (lines 1072-1105)

```go
// APICheckAcmeCname checks if the CNAME record is correctly configured
func (h *Handler) APICheckAcmeCname(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	domain, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Domain not found", http.StatusNotFound)
		return
	}

	// Check ownership
	if !h.canAccessDomain(r, domain) {
		h.jsonError(w, "Not authorized", http.StatusForbidden)
		return
	}

	if domain.AcmeFulldomain == nil || *domain.AcmeFulldomain == "" {
		h.jsonError(w, "Domain not registered with acme-dns", http.StatusBadRequest)
		return
	}

	verified := h.verification.CheckAcmeCnameRecord(domain.Name, *domain.AcmeFulldomain)

	if verified && !domain.AcmeCnameVerified {
		// Update database
		v := true
		h.service.Update(id, models.UpdateDomainInput{AcmeCnameVerified: &v})
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"verified":     verified,
		"cnameTarget":  domain.AcmeFulldomain,
		"expectedName": "_acme-challenge." + services.GetBaseDomain(domain.Name),
	})
}
```

---

## DNS Verification Service

### TXT Record Verification

**File:** `/home/this/dev/code/botginx/modules/domains/services/verification_service.go`

```go
// CheckTXTRecord verifies the _guardbot-verify TXT record exists with correct value
func (s *VerificationService) CheckTXTRecord(domain, expectedToken string) bool {
	baseDomain := GetBaseDomain(domain)
	txtHost := "_guardbot-verify." + baseDomain

	records, err := net.LookupTXT(txtHost)
	if err != nil {
		return false
	}

	for _, record := range records {
		if record == expectedToken {
			return true
		}
	}
	return false
}
```

### Wildcard A Record Verification

```go
// CheckWildcardARecord verifies the wildcard A record points to deploy server
func (s *VerificationService) CheckWildcardARecord(domain string) bool {
	baseDomain := GetBaseDomain(domain)
	wildcardHost := "*." + baseDomain

	// Resolve the wildcard - use a random subdomain to test
	testHost := "test-" + randomString(8) + "." + baseDomain

	ips, err := net.LookupIP(testHost)
	if err != nil {
		return false
	}

	deployIP := s.getDeployIP()
	for _, ip := range ips {
		if ip.String() == deployIP {
			return true
		}
	}
	return false
}
```

### CNAME Verification

**File:** `/home/this/dev/code/botginx/modules/domains/services/verification_service.go` (lines 492-523)

```go
// CheckAcmeCNAME checks if _acme-challenge CNAME exists (pointing to any acme-dns)
func (s *VerificationService) CheckAcmeCNAME(domain string) bool {
	baseDomain := GetBaseDomain(domain)
	cnameHost := "_acme-challenge." + baseDomain

	cname, err := net.LookupCNAME(cnameHost)
	if err != nil {
		return false
	}

	// Just check that a CNAME exists (not self-referential)
	cname = strings.TrimSuffix(cname, ".")
	return cname != "" && cname != cnameHost
}

// CheckAcmeCnameRecord verifies if CNAME record is correctly pointing to acme-dns
func (s *VerificationService) CheckAcmeCnameRecord(domain, expectedTarget string) bool {
	baseDomain := GetBaseDomain(domain)
	cnameHost := "_acme-challenge." + baseDomain

	// Query CNAME record
	cname, err := net.LookupCNAME(cnameHost)
	if err != nil {
		return false
	}

	// Normalize (remove trailing dot)
	cname = strings.TrimSuffix(cname, ".")
	expectedTarget = strings.TrimSuffix(expectedTarget, ".")

	return strings.EqualFold(cname, expectedTarget)
}
```

---

## Acme-DNS Integration

### Registration with Acme-DNS

**File:** `/home/this/dev/code/botginx/modules/domains/services/domain_service.go` (lines 514-563)

```go
// RegisterWithAcmeDNS registers a domain with acme-dns on the Deploy VPS
// Returns the registration info to be stored in the domain record
func (s *DomainService) RegisterWithAcmeDNS(domainID string) (*acmedns.Registration, error) {
	// Get Deploy VPS credentials
	ip, port, user, password, err := s.GetDeployServer()
	if err != nil {
		return nil, fmt.Errorf("no deploy server available: %w", err)
	}

	// Connect to Deploy VPS
	client, err := acmedns.NewClient(ip, port, user, password)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to deploy server: %w", err)
	}
	defer client.Close()

	// Check if acme-dns is installed
	if !client.IsInstalled() {
		// Try to set it up
		acmeDomain := os.Getenv("ACME_DNS_DOMAIN")
		if acmeDomain == "" {
			return nil, fmt.Errorf("ACME_DNS_DOMAIN environment variable not set")
		}
		if err := client.Setup(acmeDomain); err != nil {
			return nil, fmt.Errorf("acme-dns not installed and setup failed: %w", err)
		}
	}

	// Register new subdomain
	reg, err := client.Register()
	if err != nil {
		return nil, fmt.Errorf("acme-dns registration failed: %w", err)
	}

	// Store registration in database (including username for API auth)
	_, err = s.db.Exec(`
		UPDATE domains SET
			acme_subdomain = $1,
			acme_username = $2,
			acme_password = $3,
			acme_fulldomain = $4,
			updated_at = NOW()
		WHERE id = $5
	`, reg.Subdomain, reg.Username, reg.Password, reg.Fulldomain, domainID)
	if err != nil {
		return nil, fmt.Errorf("failed to store acme-dns credentials: %w", err)
	}

	return reg, nil
}
```

### SSL Generation via Acme-DNS

**File:** `/home/this/dev/code/botginx/modules/domains/services/verification_service.go` (lines 383-490)

```go
// GenerateWildcardSSLWithAcmeDNS generates wildcard SSL using acme-dns CNAME delegation
// This is 100% reliable - we control the TXT record via acme-dns API
// acmeUsername is used for X-Api-User (different from subdomain in acme-dns)
func (s *VerificationService) GenerateWildcardSSLWithAcmeDNS(domain, acmeSubdomain, acmeUsername, acmePassword string) error {
	server, err := s.getServer()
	if err != nil {
		return err
	}

	port := fmt.Sprintf("%d", server.Port)
	if server.Port == 0 {
		port = "22"
	}

	client, err := sshexec.NewClient(server.IP, port, server.User, server.Password)
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer client.Close()

	baseDomain := GetBaseDomain(domain)

	// Verify acme-dns is running before attempting SSL generation
	checkCmd := "curl -s -o /dev/null -w '%{http_code}' http://localhost:8053/health || echo '000'"
	output, _ := client.Run(checkCmd)
	if strings.TrimSpace(output) != "200" {
		return fmt.Errorf("acme-dns service is not running on this server. Please contact support")
	}

	// Create auth hook that updates acme-dns TXT record
	// X-Api-User must be the username from acme-dns registration
	authHookScript := fmt.Sprintf(`cat > /tmp/acmedns-auth-hook.sh << 'HOOKEOF'
#!/bin/bash
# Update acme-dns TXT record via API
BODY='{"subdomain":"%s","txt":"'$CERTBOT_VALIDATION'"}'
HTTP_CODE=$(curl -s -o /tmp/acmedns-response.txt -w '%%{http_code}' \
    -X POST \
    -H "X-Api-User: %s" \
    -H "X-Api-Key: %s" \
    -H "Content-Type: application/json" \
    -d "$BODY" \
    http://localhost:8053/update)
BODY=$(cat /tmp/acmedns-response.txt 2>/dev/null)
if [ "$HTTP_CODE" != "200" ]; then
    echo "acme-dns update failed: HTTP $HTTP_CODE - $BODY" >&2
    exit 1
fi
# Wait for DNS propagation (acme-dns is instant, but give it a moment)
sleep 2
HOOKEOF
chmod +x /tmp/acmedns-auth-hook.sh`, acmeSubdomain, acmeUsername, acmePassword)

	// Run auth hook setup
	if _, err := client.Run(authHookScript); err != nil {
		return fmt.Errorf("failed to create auth hook: %w", err)
	}

	// Generate wildcard certificate using CNAME delegation
	certbotCmd := fmt.Sprintf(`certbot certonly --manual --preferred-challenges dns \
  -d "*.%s" -d "%s" \
  --manual-auth-hook /tmp/acmedns-auth-hook.sh \
  --manual-cleanup-hook "rm -f /tmp/acmedns-auth-hook.sh /tmp/acmedns-response.txt" \
  --agree-tos --email admin@%s --no-eff-email \
  --non-interactive --force-renewal 2>&1`, baseDomain, baseDomain, baseDomain)

	output, err = client.Run(certbotCmd)
	if err != nil {
		return fmt.Errorf("certbot failed: %s", output)
	}

	// Deploy certificate to nginx
	deployCmd := fmt.Sprintf(`
mkdir -p /etc/nginx/ssl
cp /etc/letsencrypt/live/%s/fullchain.pem /etc/nginx/ssl/%s.pem
cp /etc/letsencrypt/live/%s/privkey.pem /etc/nginx/ssl/%s.key
nginx -t && systemctl reload nginx
`, baseDomain, baseDomain, baseDomain, baseDomain)

	if _, err := client.Run(deployCmd); err != nil {
		return fmt.Errorf("certificate deployment failed: %w", err)
	}

	return nil
}
```

---

## Frontend Template

### JavaScript Polling Logic

**File:** `/home/this/dev/code/botginx/modules/domains/templates/external_setup.html` (lines 459-600)

```javascript
const domainId = '{{.Domain.ID}}';
let acmeToken = '{{.SetupInfo.AcmeToken}}';
const initialSetupStep = '{{.Domain.SetupStep}}';
const initialSSLEnabled = {{if .Domain.SSLEnabled}}true{{else}}false{{end}};
const isWildcard = {{if .SetupInfo.IsWildcard}}true{{else}}false{{end}};
let sslToken = '';
let pollInterval;
let dnsVerified = { aRecord: false, verifyTxt: false };

// Update SSL button state based on DNS verification
function updateSSLButtonState() {
    const btn = document.getElementById('btn-get-ssl-token');
    const hint = document.getElementById('ssl-prereq-hint');
    if (!btn) return;

    if (dnsVerified.aRecord && dnsVerified.verifyTxt) {
        btn.disabled = false;
        if (hint) hint.classList.add('d-none');
    } else {
        btn.disabled = true;
        if (hint) {
            hint.classList.remove('d-none');
            const missing = [];
            if (!dnsVerified.aRecord) missing.push('A record');
            if (!dnsVerified.verifyTxt) missing.push('verify TXT');
            hint.textContent = `Waiting for ${missing.join(' and ')}...`;
        }
    }
}

// CNAME delegation: Verify CNAME record
async function verifyCnameRecord() {
    const btn = document.getElementById('btn-verify-cname');
    btn.disabled = true;
    btn.innerHTML = '<span class="spinner-border spinner-border-sm me-1"></span>Verifying...';

    try {
        const res = await fetch(`/user/domains/api/${domainId}/acme-dns/verify`, { method: 'POST' });
        const data = await res.json();

        if (data.verified) {
            showToast('CNAME verified! You can now generate SSL.', 'success');
            document.getElementById('badge-ssl').className = 'badge bg-success';
            document.getElementById('badge-ssl').innerHTML = '<i class="bi bi-check-circle"></i> Verified';
            // Show generate button
            btn.classList.add('d-none');
            const genBtn = document.getElementById('btn-generate-ssl-cname');
            if (genBtn) genBtn.classList.remove('d-none');
        } else {
            showToast('CNAME not found yet. Please check your DNS settings and wait for propagation.', 'warning');
            btn.disabled = false;
            btn.innerHTML = '<i class="bi bi-arrow-repeat me-1"></i>Verify CNAME Record';
        }
    } catch (e) {
        showToast('Failed to verify CNAME', 'danger');
        btn.disabled = false;
        btn.innerHTML = '<i class="bi bi-arrow-repeat me-1"></i>Verify CNAME Record';
    }
}

// Generate SSL using acme-dns CNAME delegation
async function generateSSLWithCname() {
    const btn = document.getElementById('btn-generate-ssl-cname');
    btn.disabled = true;
    btn.innerHTML = '<span class="spinner-border spinner-border-sm me-1"></span>Generating...';

    document.getElementById('ssl-cname-generating').classList.remove('d-none');

    try {
        const res = await fetch(`/user/domains/api/${domainId}/ssl/acme-dns`, { method: 'POST' });
        const data = await res.json();

        if (data.error) {
            showToast(data.error, 'danger');
            btn.disabled = false;
            btn.innerHTML = '<i class="bi bi-lock me-1"></i>Generate SSL Certificate';
            document.getElementById('ssl-cname-generating').classList.add('d-none');
            return;
        }

        if (data.success) {
            showToast('SSL certificate generated successfully!', 'success');
            showSetupComplete();
        }
    } catch (e) {
        showToast('SSL generation failed', 'danger');
        btn.disabled = false;
        btn.innerHTML = '<i class="bi bi-lock me-1"></i>Generate SSL Certificate';
        document.getElementById('ssl-cname-generating').classList.add('d-none');
    }
}

// Poll DNS status every 5 seconds
function startPolling() {
    pollInterval = setInterval(async () => {
        try {
            const res = await fetch(`/user/domains/${domainId}/dns-status`);
            const data = await res.json();

            // Update badges
            updateBadge('badge-a', data.aRecordVerified);
            updateBadge('badge-verify', data.txtVerified);

            dnsVerified.aRecord = data.aRecordVerified;
            dnsVerified.verifyTxt = data.txtVerified;

            updateSSLButtonState();

            if (data.allVerified) {
                clearInterval(pollInterval);
            }
        } catch (e) {
            console.error('DNS poll failed:', e);
        }
    }, 5000);
}

// Start polling on page load
document.addEventListener('DOMContentLoaded', startPolling);
```

---

## API Endpoints

### Routes Definition

**File:** `/home/this/dev/code/botginx/modules/domains/module.go`

```go
// Manual setup routes
r.Get("/{id}/setup", m.handler.ExternalSetup)      // Setup wizard page
r.Get("/{id}/dns-status", m.handler.APIGetDNSStatus) // Poll DNS verification

// Acme-DNS routes
r.Post("/api/{id}/acme-dns/register", m.handler.APIRegisterAcmeDNS)  // Register with acme-dns
r.Post("/api/{id}/acme-dns/verify", m.handler.APICheckAcmeCname)     // Verify CNAME
r.Post("/api/{id}/ssl/acme-dns", m.handler.APIGenerateSSLAcmeDNS)    // Generate SSL

// Legacy SSL routes (fallback)
r.Post("/api/{id}/ssl/start", m.handler.APIStartSSLChallenge)        // Get ACME token
r.Post("/api/{id}/ssl/complete", m.handler.APICompleteSSLChallenge)  // Complete challenge
r.Delete("/api/{id}/ssl/cancel", m.handler.APICancelSSLChallenge)    // Cancel challenge
```

### API Summary

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/user/domains/{id}/setup` | Show setup wizard page |
| `GET` | `/user/domains/{id}/dns-status` | Poll DNS record verification |
| `POST` | `/user/domains/api/{id}/acme-dns/register` | Register domain with acme-dns |
| `POST` | `/user/domains/api/{id}/acme-dns/verify` | Verify CNAME points to acme-dns |
| `POST` | `/user/domains/api/{id}/ssl/acme-dns` | Generate SSL via acme-dns |
| `POST` | `/user/domains/api/{id}/ssl/start` | Legacy: get ACME TXT token |
| `POST` | `/user/domains/api/{id}/ssl/complete` | Legacy: complete ACME challenge |

---

## DNS Records Summary

### For cPanel Domains (IsWildcard = true)

| Step | Type | Name | Value |
|------|------|------|-------|
| 1 | **A** | `*` | `{deploy_server_ip}` |
| 2 | **TXT** | `_guardbot-verify` | `{domain.verify_token}` |
| 3 | **CNAME** | `_acme-challenge` | `{domain.acme_fulldomain}` |

### For External Domains (IsWildcard = false)

| Step | Type | Name | Value |
|------|------|------|-------|
| 1a | **A** | `@` | `{deploy_server_ip}` |
| 1b | **A** | `*` | `{deploy_server_ip}` |
| 2 | **TXT** | `_guardbot-verify` | `{domain.verify_token}` |
| 3 | **CNAME** | `_acme-challenge` | `{domain.acme_fulldomain}` |

---

## Porting Checklist

1. **Database Schema**
   - [ ] `verify_token` column for domain ownership verification
   - [ ] `acme_subdomain`, `acme_username`, `acme_password`, `acme_fulldomain` for acme-dns
   - [ ] `acme_cname_verified` boolean flag

2. **Backend Handlers**
   - [ ] Setup wizard page that builds `ExternalSetupInfo`
   - [ ] DNS status polling endpoint (checks TXT, A, CNAME)
   - [ ] Acme-DNS registration endpoint
   - [ ] CNAME verification endpoint
   - [ ] SSL generation via acme-dns endpoint

3. **DNS Verification**
   - [ ] `CheckTXTRecord(domain, token)` - verify `_guardbot-verify` TXT
   - [ ] `CheckWildcardARecord(domain)` - verify `*` A record points to deploy IP
   - [ ] `CheckAcmeCnameRecord(domain, target)` - verify `_acme-challenge` CNAME

4. **Acme-DNS Integration**
   - [ ] SSH client to connect to deploy VPS
   - [ ] Acme-DNS registration (POST `/register`)
   - [ ] Acme-DNS TXT update (POST `/update`)
   - [ ] Certbot with `--manual-auth-hook` that calls acme-dns API

5. **Frontend**
   - [ ] Setup wizard showing DNS records to add
   - [ ] Copy-to-clipboard buttons for values
   - [ ] 5-second polling for DNS status
   - [ ] Badge updates for verified records
   - [ ] SSL generation button (enabled when all records verified)

6. **Environment Variables**
   - [ ] `ACME_DNS_DOMAIN` - domain for acme-dns CNAME targets

---

## Related Documentation

- `/home/this/dev/code/botginx/docs/reference/cpanel-integration-system.md` — cPanel API automation
- `/home/this/dev/code/botginx/docs/explanation/enforcement-architecture.md` — Bot protection architecture
