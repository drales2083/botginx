# Turnstile Integration Plan

> **Status:** Planning  
> **Date:** 2026-09-15

## Overview

Add Cloudflare Turnstile support to botginx panel. Turnstile credentials are configured per-domain and automatically included when users select the "cloudflare" template for link bot protection.

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│  Domain Settings Page                                                   │
│  /user/domains/{id}/settings                                           │
│                                                                         │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │ Cloudflare Turnstile                                             │   │
│  │                                                                   │   │
│  │ Site Key:    [________________________]                          │   │
│  │ Secret Key:  [________________________]                          │   │
│  │                                                                   │   │
│  │ ℹ️ Get your keys from Cloudflare Dashboard → Turnstile           │   │
│  │    Same keys work for all subdomains of this domain              │   │
│  └─────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    │ Keys stored on domain
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  Link Bot Protection Settings                                           │
│  /user/analytics/link/{id}/settings                                    │
│                                                                         │
│  Template: [cloudflare ▼]  ← User selects cloudflare template          │
│                                                                         │
│  If domain has Turnstile keys:                                         │
│    ✅ "Turnstile enabled - using keys from domain settings"            │
│                                                                         │
│  If domain missing keys:                                               │
│    ⚠️ "Turnstile requires API keys. Configure them in Domain Settings" │
│    [Go to Domain Settings →]                                           │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    │ On save, auto-fetch domain keys
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  Settings Push to VPS                                                   │
│  /etc/botection/links/{linkId}.json                                    │
│                                                                         │
│  {                                                                      │
│    "link_id": "lnk-abc123",                                            │
│    "host": "promo.example.com",                                        │
│    "template": "cloudflare",                                           │
│    "turnstile_site_key": "0x4AAAAAAA...",    ← Auto-included           │
│    "turnstile_secret_key": "0x4AAAAAAA...",  ← Auto-included           │
│    ...                                                                  │
│  }                                                                      │
└─────────────────────────────────────────────────────────────────────────┘
```

## Database Changes

### Migration: Add Turnstile columns to domains table

```sql
ALTER TABLE domains ADD COLUMN turnstile_site_key TEXT;
ALTER TABLE domains ADD COLUMN turnstile_secret_key TEXT;
```

No changes to `link_settings` or `redirect_links` tables.

## File Changes

### 1. Domain Model

**File:** `modules/domains/models/domain.go`

Add fields:
```go
type Domain struct {
    // ... existing fields ...
    TurnstileSiteKey   *string `db:"turnstile_site_key" json:"turnstileSiteKey,omitempty"`
    TurnstileSecretKey *string `db:"turnstile_secret_key" json:"-"` // Never expose in JSON
}
```

### 2. Domain Settings Page (New)

**File:** `modules/domains/templates/settings.html` (new)

Create new template with:
- Turnstile Site Key input
- Turnstile Secret Key input (password field)
- Save button
- Cloudflare setup guide

### 3. Domain Settings Handler

**File:** `modules/domains/handlers/handler.go`

Add routes:
```go
r.Get("/{id}/settings", h.handleSettings)      // GET - show settings page
r.Post("/{id}/settings", h.handleSaveSettings) // POST - save settings
r.Get("/{id}/settings/turnstile", h.apiGetTurnstileStatus) // API - check if keys exist
```

### 4. Settings Pusher

**File:** `pkg/settingspush/pusher.go`

Add Turnstile fields to struct:
```go
type LinkSettings struct {
    // ... existing fields ...
    TurnstileSiteKey   string `json:"turnstile_site_key,omitempty"`
    TurnstileSecretKey string `json:"turnstile_secret_key,omitempty"`
}
```

### 5. Analytics Handler - Settings Push

**File:** `modules/analytics/handlers/handler.go`

Update `pushSettingsToVPS()`:
```go
func (h *Handler) pushSettingsToVPS(linkID, userID, domainID, host string, settings models.LinkSettings) {
    // ... existing code ...

    // If template is cloudflare, fetch domain's Turnstile keys
    var turnstileSiteKey, turnstileSecretKey string
    if settings.Template == "cloudflare" {
        h.db.Get(&turnstileSiteKey, 
            `SELECT COALESCE(turnstile_site_key, '') FROM domains WHERE id = $1`, domainID)
        h.db.Get(&turnstileSecretKey, 
            `SELECT COALESCE(turnstile_secret_key, '') FROM domains WHERE id = $1`, domainID)
    }

    pushSettings := settingspush.LinkSettings{
        // ... existing fields ...
        TurnstileSiteKey:   turnstileSiteKey,
        TurnstileSecretKey: turnstileSecretKey,
    }
    
    // ... rest of push logic ...
}
```

### 6. Link Settings UI - Validation

**File:** `modules/analytics/templates/settings.html`

Add JavaScript validation:
```javascript
// When template changes to 'cloudflare', check if domain has Turnstile keys
templateSelect.addEventListener('change', async function() {
    if (this.value === 'cloudflare') {
        const resp = await fetch(`/user/domains/${domainId}/settings/turnstile`);
        const data = await resp.json();
        
        if (!data.hasKeys) {
            showWarning('Turnstile requires API keys. Configure them in Domain Settings.');
            // Show link to domain settings
        } else {
            showSuccess('Turnstile enabled - using keys from domain settings');
        }
    }
});
```

### 7. Sidebar Menu Update

**File:** `modules/domains/module.go`

Add "Settings" link to domain detail page or as submenu.

## UI Components

### Domain Settings Page

```
┌─────────────────────────────────────────────────────────────────┐
│ Domain Settings                                                  │
│ example.com                                                      │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│ Cloudflare Turnstile                                            │
│ ─────────────────────────────────────────────────────────────── │
│                                                                  │
│ Use Cloudflare Turnstile for human verification on the          │
│ "cloudflare" challenge template. Same keys work for all         │
│ subdomains.                                                      │
│                                                                  │
│ Site Key                                                         │
│ ┌─────────────────────────────────────────────────────────────┐ │
│ │ 0x4AAAAAAA...                                                │ │
│ └─────────────────────────────────────────────────────────────┘ │
│                                                                  │
│ Secret Key                                                       │
│ ┌─────────────────────────────────────────────────────────────┐ │
│ │ ••••••••••••••••••••                                        │ │
│ └─────────────────────────────────────────────────────────────┘ │
│                                                                  │
│                                              [Save Settings]     │
│                                                                  │
├─────────────────────────────────────────────────────────────────┤
│ 📘 How to get Turnstile keys                                    │
│                                                                  │
│ 1. Go to Cloudflare Dashboard → Turnstile                       │
│    https://dash.cloudflare.com/?to=/:account/turnstile          │
│                                                                  │
│ 2. Click "Add site"                                             │
│                                                                  │
│ 3. Enter your domain name (e.g., example.com)                   │
│                                                                  │
│ 4. Select widget type: "Managed"                                │
│                                                                  │
│ 5. Copy the Site Key and Secret Key                             │
│                                                                  │
│ Note: Turnstile is free for unlimited use.                      │
└─────────────────────────────────────────────────────────────────┘
```

### Link Settings Warning (when cloudflare selected without keys)

```
┌─────────────────────────────────────────────────────────────────┐
│ ⚠️ Turnstile Not Configured                                     │
│                                                                  │
│ The "cloudflare" template requires Turnstile API keys.          │
│ Without keys, it will use proof-of-work instead of the          │
│ official Cloudflare widget.                                      │
│                                                                  │
│ [Configure Turnstile Keys →]                                    │
└─────────────────────────────────────────────────────────────────┘
```

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/user/domains/{id}/settings` | Domain settings page |
| POST | `/user/domains/{id}/settings` | Save domain settings |
| GET | `/user/domains/{id}/settings/turnstile` | Check if Turnstile keys exist (JSON) |

### API Response: Turnstile Status

```json
{
  "hasKeys": true,
  "siteKeySet": true,
  "secretKeySet": true
}
```

## Implementation Steps

### Phase 1: Database & Model
1. [ ] Create migration for `turnstile_site_key` and `turnstile_secret_key` on `domains`
2. [ ] Update Domain model with new fields
3. [ ] Add domain service methods for getting/saving Turnstile keys

### Phase 2: Domain Settings Page
4. [ ] Create domain settings route handler
5. [ ] Create domain settings template with Turnstile inputs
6. [ ] Add Cloudflare setup guide to template
7. [ ] Add settings link to domain detail page

### Phase 3: Settings Push Integration
8. [ ] Add Turnstile fields to pusher struct
9. [ ] Update `pushSettingsToVPS()` to fetch domain keys when template is cloudflare
10. [ ] Test settings push includes Turnstile keys

### Phase 4: Link Settings Validation
11. [ ] Add API endpoint to check Turnstile key status
12. [ ] Add JavaScript validation on template select
13. [ ] Show warning/link when keys not configured

### Phase 5: Testing
14. [ ] Test domain settings save/load
15. [ ] Test settings push with cloudflare template
16. [ ] Test warning appears when keys missing
17. [ ] Verify botection receives keys correctly

## Security Considerations

1. **Secret key never exposed in JSON** — Use `json:"-"` tag
2. **Secret key shown as password field** — Masked in UI
3. **Keys stored encrypted?** — Consider if needed (currently plain text like other credentials)
4. **API only returns boolean** — Never returns actual key values

## Notes

- Turnstile is free for unlimited use
- Same keys work for all subdomains (per Cloudflare docs)
- Without keys, botection falls back to proof-of-work (still works, just not official Turnstile widget)
- Keys are optional — cloudflare template works without them, just uses PoW
