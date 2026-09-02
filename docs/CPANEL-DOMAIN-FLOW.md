# cPanel/External Domain Setup Flow

## Overview

Smooth onboarding for users who have domains on cPanel or other external DNS providers where they can't change nameservers but CAN add DNS records.

---

## User Flow (Proposed)

### Step 1: User Adds Domain

User enters: `*.mynutradvice.com` or `mynutradvice.com`

Panel detects:
- Domain doesn't point to our server → External/cPanel hosting
- Shows "External Domain Setup" wizard

### Step 2: Unified Instructions Page

Show ALL required DNS records at once:

```
┌─────────────────────────────────────────────────────────────────┐
│ 🌐 External Domain Setup: *.mynutradvice.com                    │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│ Add these DNS records in your cPanel Zone Editor:               │
│                                                                 │
│ ┌─────────────────────────────────────────────────────────────┐ │
│ │ STEP 1: Point domain to our server                         │ │
│ │                                                             │ │
│ │ Type: A                                                     │ │
│ │ Name: *                                                     │ │
│ │ Value: 194.147.217.45                          [📋 Copy]   │ │
│ └─────────────────────────────────────────────────────────────┘ │
│                                                                 │
│ ┌─────────────────────────────────────────────────────────────┐ │
│ │ STEP 2: Verify domain ownership                             │ │
│ │                                                             │ │
│ │ Type: TXT                                                   │ │
│ │ Name: _guardbot-verify                                      │ │
│ │ Value: guardbot-a1b2c3d4e5f6                   [📋 Copy]   │ │
│ └─────────────────────────────────────────────────────────────┘ │
│                                                                 │
│ ┌─────────────────────────────────────────────────────────────┐ │
│ │ STEP 3: SSL Certificate (for HTTPS)                         │ │
│ │                                                             │ │
│ │ Type: TXT                                                   │ │
│ │ Name: _acme-challenge                                       │ │
│ │ Value: [Generating...]                         [📋 Copy]   │ │
│ │                                                             │ │
│ │ ⏳ Waiting for ACME token... (auto-refreshes)              │ │
│ └─────────────────────────────────────────────────────────────┘ │
│                                                                 │
│ ┌─────────────────────────────────────────────────────────────┐ │
│ │ 📖 cPanel Instructions:                                     │ │
│ │ 1. Login to cPanel                                          │ │
│ │ 2. Go to "Zone Editor"                                      │ │
│ │ 3. Click "Manage" next to your domain                       │ │
│ │ 4. Add each record above using "+ Add Record"               │ │
│ └─────────────────────────────────────────────────────────────┘ │
│                                                                 │
│              [I've Added All Records - Verify Now]              │
│                                                                 │
│ Status: ⏳ Waiting for DNS records...                          │
│ • A Record: ❌ Not found                                        │
│ • Verify TXT: ❌ Not found                                      │
│ • ACME TXT: ❌ Not found                                        │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Step 3: Auto-Polling

Panel polls DNS every 30 seconds:
- Check A record → ✅
- Check verification TXT → ✅  
- Check ACME challenge TXT → ✅

When all found → Automatically proceed to SSL generation.

### Step 4: Success

```
┌─────────────────────────────────────────────────────────────────┐
│ ✅ Domain Setup Complete!                                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│ *.mynutradvice.com is now active with SSL                      │
│                                                                 │
│ • DNS Verified: ✅                                              │
│ • SSL Certificate: ✅ Expires 2026-11-28                        │
│ • Nginx Configured: ✅                                          │
│                                                                 │
│                    [Go to Dashboard]                            │
└─────────────────────────────────────────────────────────────────┘
```

---

## Technical Implementation

### 1. Domain Detection

```go
// In domain creation handler
func (h *Handler) detectDomainType(domain string) string {
    // Check if domain A record points to our server
    ips, err := net.LookupHost(domain)
    if err != nil || !containsOurIP(ips) {
        return "external" // cPanel, Cloudflare, etc.
    }
    return "direct" // Points to us, can use HTTP-01
}
```

### 2. Pre-generate ACME Token

Before showing the setup page, start certbot in "dry-run" or token-generation mode:

```go
// Generate ACME challenge token ahead of time
func (s *Service) PreGenerateACMEToken(domain string) (string, error) {
    // Run certbot with a hook that captures and returns the token
    // Store token in database with expiry (valid for ~15 minutes)
    // Return token for display to user
}
```

### 3. Database Schema Addition

```sql
-- Track external domain setup progress
ALTER TABLE domains ADD COLUMN setup_type TEXT DEFAULT 'direct';
ALTER TABLE domains ADD COLUMN acme_token TEXT;
ALTER TABLE domains ADD COLUMN acme_token_expires_at TIMESTAMP;
ALTER TABLE domains ADD COLUMN setup_step TEXT DEFAULT 'pending';
-- setup_step: pending, dns_waiting, verifying, ssl_generating, complete
```

### 4. Polling Endpoint

```go
// GET /api/domain/{id}/setup-status
func (h *Handler) APIGetSetupStatus(w http.ResponseWriter, r *http.Request) {
    domain := getDomain(r)
    
    status := SetupStatus{
        ARecord:    checkARecord(domain.Name),
        VerifyTXT:  checkVerifyTXT(domain.Name, domain.VerifyToken),
        AcmeTXT:    checkAcmeTXT(domain.Name, domain.AcmeToken),
    }
    
    // If all pass, trigger SSL generation
    if status.AllPassed() && domain.SetupStep == "dns_waiting" {
        go h.completeSetup(domain)
    }
    
    json.NewEncoder(w).Encode(status)
}
```

### 5. Frontend Auto-Refresh

```javascript
// Poll every 30 seconds
const pollInterval = setInterval(async () => {
    const res = await fetch(`/api/domain/${domainId}/setup-status`);
    const status = await res.json();
    
    updateStatusUI(status);
    
    if (status.complete) {
        clearInterval(pollInterval);
        showSuccess();
    }
}, 30000);
```

---

## ACME Token Pre-Generation Strategy

### Option A: On-Demand Token (Current)
- Start certbot when user clicks "Generate SSL"
- User waits for token, then adds TXT record
- **Problem:** User has to wait, two-step process

### Option B: Pre-Generated Token Pool
- Background job keeps pool of 10 tokens ready
- When user adds domain, assign one from pool
- **Problem:** Tokens expire (~15 min), wasteful

### Option C: Combined Flow (Recommended)
1. When user submits domain, immediately start certbot in background
2. Show setup page with placeholder for ACME token
3. Auto-refresh until token is generated (~5 seconds)
4. User adds all 3 records at once
5. Poll until verified, complete SSL

```
Timeline:
0s   - User submits domain
0-5s - Certbot generates ACME token (background)
5s   - Page shows all 3 records to add
5s+  - User adds records in cPanel (~2-5 min)
     - Page polls every 30s
     - All records found → SSL completes
```

---

## Files to Create/Modify

| File | Changes |
|------|---------|
| `modules/domains/templates/external_setup.html` | New wizard page |
| `modules/domains/handlers/handler.go` | Add setup status endpoint |
| `modules/domains/services/domain_service.go` | Add setup tracking |
| `modules/domains/services/verification_service.go` | Pre-generate ACME token |
| `modules/domains/models/domain.go` | Add setup fields |
| `migrations/xxx_add_domain_setup_fields.sql` | New columns |

---

## Edge Cases

1. **Token Expiry:** If ACME token expires before user adds TXT, regenerate and update UI
2. **Multiple Attempts:** Track failed attempts, show helpful error messages
3. **Partial Setup:** Allow user to resume setup later
4. **Root vs Wildcard:** Detect `*.domain.com` vs `domain.com` and adjust flow

---

## Future Enhancements

1. **cPanel API Integration:** User provides cPanel credentials, we add records automatically
2. **Cloudflare Integration:** If user has Cloudflare, use API for automatic DNS
3. **DNS Provider Detection:** Auto-detect if domain uses known providers with APIs
4. **QR Code:** Show QR linking to cPanel login for mobile users

---

## Summary

The key improvements:
1. **Show all records upfront** - User adds everything in one cPanel session
2. **Auto-polling** - No manual "verify" button clicking
3. **Pre-generate ACME token** - Minimal wait time
4. **Clear progress indicators** - User knows exactly what's happening
5. **One unified page** - Not jumping between different screens
