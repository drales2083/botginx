# Coming Soon Features

Features marked "Coming Soon" on the cPanel dashboard. Each section describes what the feature should do.

## Traffic

### Link Wizard
**Route:** `/user/redirectlinks/wizard`

Two-step wizard to create redirect links with bot protection configured upfront:

1. **Step 1: Link Details**
   - Same fields as current create form
   - Target URL
   - Domain selection
   - Subdomain (auto-generate or custom)
   - Path style (random, invoice-style, custom)
   - "Next" button to continue

2. **Step 2: Bot Protection**
   - Reuse existing bot protection settings component
   - Includes: antibot toggle, challenge type, interstitial template, sensitivity
   - Includes: geo rules (allowed/blocked countries)
   - Includes: device rules (mobile, desktop, tablet, OS)
   - "Create Link" button

**Implementation:**
- Single page with two sections (tabs or accordion)
- Step indicator at top (1. Link Details → 2. Bot Protection)
- Back button to return to step 1
- Reuse `protection/templates/settings.html` partial for step 2

**Difference from "Create Link":** Regular create uses defaults for bot protection. Wizard lets user configure everything before creating.

---

### QR Codes
**Route:** `/user/qrcodes`

Generate QR codes for redirect links:

- List all QR codes created
- Generate QR for any existing redirect link
- Customization: size, color, logo embed
- Download formats: PNG, SVG, PDF
- Track scans separately from link clicks

---

### Bulk Import
**Route:** `/user/redirectlinks/import`

Import multiple redirect links at once:

- CSV upload with columns: target_url, subdomain, path, domain_id
- Preview before import
- Validation errors shown per row
- Apply same bot protection settings to all imported links
- Progress indicator for large imports

---

### Link Templates
**Route:** `/user/redirectlinks/templates`

Save and reuse link configurations:

- Save current link settings as template
- Name and describe templates
- Apply template when creating new links
- Default template option
- Share templates (coming later)

---

## Domains

### Domain Store
**Route:** `/user/marketplace`

**DONE** - Already implemented and enabled on dashboard.

---

### Domain Health
**Route:** `/user/domains/health`

Monitor domain status and issues:

- SSL certificate expiry warnings
- DNS propagation status
- Domain reputation scores
- Blacklist checks
- Uptime monitoring
- Suggested fixes for issues

---

## Metrics

### Click Logs
**Route:** `/user/analytics/logs`

Detailed log of every click:

- Timestamp, IP, country, city
- Device, browser, OS
- Referrer, UTM params
- Bot detection result
- Link clicked
- Filterable and searchable
- Export to CSV

---

### Real-time
**Route:** `/user/analytics/realtime`

Live dashboard showing:

- Current active visitors
- Clicks in last 60 seconds
- Live map with visitor locations
- Auto-refresh every 5 seconds
- Sound alert option for new clicks

---

### Export Data
**Route:** `/user/analytics/export`

Export analytics data:

- Date range selection
- Choose metrics to include
- Formats: CSV, JSON, PDF report
- Schedule recurring exports
- Email delivery option

---

## Security

### Threat Log
**Route:** `/user/iplists/threats`

Log of blocked threats:

- All blocked requests
- Block reason (bot, country, IP, etc.)
- IP addresses blocked
- One-click add to permanent blocklist
- Patterns and trends
- Export blocked IPs

---

## Account

### Referrals
**Route:** `/user/referrals`

**DONE** - Already implemented and enabled on dashboard.

---

### API Keys
**Route:** `/user/settings/api-keys`

Manage API access:

- Generate API keys
- Set permissions per key
- Rate limit configuration
- Usage statistics
- Revoke keys
- Webhook configuration

---

### Two-Factor Authentication
**Route:** `/user/settings/2fa`

Secure account with 2FA:

- TOTP setup (Google Authenticator, Authy)
- QR code for app scanning
- Backup codes
- Recovery options
- Require 2FA for sensitive actions

---

## Support

### Knowledge Base
**Route:** `/user/help`

**DONE** - Already implemented and enabled on dashboard.

---

### API Docs
**Route:** `/user/help/api`

API documentation:

- Endpoint reference
- Authentication guide
- Code examples (curl, Python, Node, Go)
- Rate limits
- Webhook events
- Interactive API explorer

---

## Implementation Priority

### Phase 1 (High Value)
1. Link Wizard - Better onboarding
2. Click Logs - Users want detailed data
3. Referrals - Growth driver

### Phase 2 (User Requested)
4. Two-Factor - Security requirement
5. API Keys - Power users
6. Bulk Import - Efficiency

### Phase 3 (Nice to Have)
7. QR Codes
8. Domain Health
9. Real-time Analytics
10. Export Data
11. Link Templates
12. Threat Log
13. Knowledge Base
14. API Docs
