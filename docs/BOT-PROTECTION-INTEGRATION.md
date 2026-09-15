# Bot Protection System - Full Integration Guide

This document explains how the bot protection settings work, how they're deployed to websites, and all file paths needed for theme/system recreation.

---

## System Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        BOTGINX PANEL                                    │
│  (This project: /home/this/dev/code/botginx)                           │
│                                                                         │
│  User configures settings in:                                           │
│  /user/analytics/link/{linkId}/settings                                │
│                                                                         │
│  Settings saved to: link_settings table (PostgreSQL)                    │
│                                                                         │
│  On save → Push settings via SSH to VPS:                               │
│  /etc/botection/links/{linkId}.json                                    │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    │ SSH/SCP Push
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                        DEPLOY VPS (Botection)                           │
│                                                                         │
│  Reads settings from: /etc/botection/links/{linkId}.json               │
│                                                                         │
│  Visitor request flow:                                                  │
│  1. Visitor hits domain                                                │
│  2. Botection (reverse proxy) intercepts                               │
│  3. Loads {linkId}.json settings                                       │
│  4. Applies blocking rules (country, device, bot score, etc.)         │
│  5. Either: Allow → proxy to PHP | Block → show challenge/redirect    │
│  6. Logs visit via webhook to panel                                    │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Settings Structure

### Database Table: `link_settings`

Each redirect link has its own protection settings stored in `link_settings`:

```sql
CREATE TABLE link_settings (
    id              TEXT PRIMARY KEY,
    link_id         TEXT NOT NULL REFERENCES redirect_links(id),
    template        TEXT DEFAULT 'cloudflare',
    theme_mode      TEXT DEFAULT 'auto',
    country_mode    TEXT DEFAULT '',
    country_list    TEXT DEFAULT '[]',
    asn_mode        TEXT DEFAULT '',
    asn_list        TEXT DEFAULT '[]',
    device_mode     TEXT DEFAULT '',
    device_list     TEXT DEFAULT '[]',
    block_bots      BOOLEAN DEFAULT false,
    block_tor       BOOLEAN DEFAULT false,
    block_proxy     BOOLEAN DEFAULT false,
    block_datacenter BOOLEAN DEFAULT false,
    block_headless  BOOLEAN DEFAULT false,
    min_behavior_score INT DEFAULT 0,
    redirect_on_block TEXT DEFAULT '',
    updated_at      TIMESTAMP DEFAULT NOW()
);
```

### Go Model: `LinkSettings`

**File:** `/home/this/dev/code/botginx/modules/analytics/models/visit.go` (lines 102-124)

```go
type LinkSettings struct {
    ID                 string    `db:"id" json:"id"`
    LinkID             string    `db:"link_id" json:"linkId"`
    Template           string    `db:"template" json:"template"`
    ThemeMode          string    `db:"theme_mode" json:"themeMode"`
    CountryMode        string    `db:"country_mode" json:"countryMode"`
    CountryList        []string  `json:"countryList"`
    ASNMode            string    `db:"asn_mode" json:"asnMode"`
    ASNList            []string  `json:"asnList"`
    DeviceMode         string    `db:"device_mode" json:"deviceMode"`
    DeviceList         []string  `json:"deviceList"`
    BlockBots          bool      `db:"block_bots" json:"blockBots"`
    BlockTor           bool      `db:"block_tor" json:"blockTor"`
    BlockProxy         bool      `db:"block_proxy" json:"blockProxy"`
    BlockDatacenter    bool      `db:"block_datacenter" json:"blockDatacenter"`
    BlockHeadless      bool      `db:"block_headless" json:"blockHeadless"`
    MinBehaviorScore   int       `db:"min_behavior_score" json:"minBehaviorScore"`
    RedirectOnBlock    string    `db:"redirect_on_block" json:"redirectOnBlock"`
    UpdatedAt          time.Time `db:"updated_at" json:"updatedAt"`
}
```

---

## Settings Options

### Challenge Templates

Users select a challenge page style that blocked visitors see:

| Template | Description |
|----------|-------------|
| `cloudflare` | Cloudflare-style "Checking your browser" |
| `humancheck` | "Verify you are human" with checkbox |
| `humansecurity` | Human Security branded |
| `slidepuzzle` | Slide puzzle challenge |
| `smartpuzzle` | Multiple puzzle types (slider, drag, rotate, icon) |
| `simple` | Minimal text-based |
| `lobby` | "Please wait" lobby style |
| `foyer` | Elegant foyer entrance |
| `latch` | Security latch animation |
| `interstitial` | Full-page interstitial |

### Theme Mode

| Value | Description |
|-------|-------------|
| `auto` | Match visitor's system preference |
| `light` | Force light theme |
| `dark` | Force dark theme |

### Geographic Filtering (Country)

| Mode | Behavior |
|------|----------|
| `` (empty) | Disabled - allow all countries |
| `whitelist` | Only allow countries in list |
| `blacklist` | Block countries in list |

**Country List:** Array of ISO 3166-1 alpha-2 codes (e.g., `["US", "CA", "GB"]`)

### ASN Filtering

| Mode | Behavior |
|------|----------|
| `` (empty) | Disabled |
| `whitelist` | Only allow ASNs in list |
| `blacklist` | Block ASNs in list |

**ASN List:** Array of ASN strings (e.g., `["AS12345", "AS67890"]`)

### Device Filtering

| Mode | Behavior |
|------|----------|
| `` (empty) | Disabled |
| `whitelist` | Only allow devices in list |
| `blacklist` | Block devices in list |

**Device List:** `["desktop", "mobile", "tablet"]`

### Bot Detection Toggles

| Setting | When Triggered |
|---------|----------------|
| `blockBots` | Visitor bot score ≥ 70 |
| `blockTor` | IP is Tor exit node |
| `blockProxy` | IP is known proxy/VPN |
| `blockDatacenter` | IP belongs to datacenter |
| `blockHeadless` | Browser appears headless |

### Behavior Score

- **Range:** 0-100
- **Default:** 0 (disabled)
- **Action:** Block visitors with score below threshold

### Redirect on Block

- **Empty:** Show challenge page
- **URL:** Redirect blocked visitors to this URL

---

## Deployed JSON Format

When settings are saved, they're pushed to the VPS as JSON:

**Remote Path:** `/etc/botection/links/{linkId}.json`

```json
{
  "link_id": "abc123def456",
  "user_id": "user789",
  "host": "promo.example.com",
  "template": "cloudflare",
  "puzzle_mode": "auto",
  "theme_mode": "dark",
  "block_bots": true,
  "block_tor": true,
  "block_proxy": false,
  "block_datacenter": true,
  "block_headless": true,
  "country_mode": "blacklist",
  "country_list": ["RU", "CN", "IR"],
  "asn_mode": "",
  "asn_list": [],
  "device_mode": "whitelist",
  "device_list": ["desktop", "mobile"],
  "min_behavior_score": 30,
  "redirect_on_block": "https://google.com",
  "updated_at": "2026-09-15T10:30:00Z"
}
```

---

## Settings Push Mechanism

**File:** `/home/this/dev/code/botginx/pkg/settingspush/pusher.go`

### How It Works

1. User saves settings in panel
2. Panel calls `Pusher.Push(server, linkId, settings)`
3. Pusher connects via SSH to VPS
4. Creates directory: `mkdir -p /etc/botection/links`
5. Writes JSON file: `/etc/botection/links/{linkId}.json`
6. Botection auto-reloads settings (watches directory)

### VPS Connection

Settings are pushed to configured VPS servers via SSH:

**Environment Variables:**
```
GUARD_VPS_IP=xxx.xxx.xxx.xxx
GUARD_VPS_USER=root
GUARD_VPS_PASSWORD=xxxxx
GUARD_VPS_PORT=22
```

---

## File Paths Reference

### Panel (This Project)

| Path | Description |
|------|-------------|
| `/home/this/dev/code/botginx/` | Project root |
| `pkg/protection/types.go` | Settings struct definition |
| `pkg/protection/defaults.go` | Default settings values |
| `pkg/protection/templates/settings.html` | Settings UI template |
| `pkg/settingspush/pusher.go` | SSH push logic |
| `modules/analytics/models/visit.go` | LinkSettings model |
| `modules/analytics/handlers/handler.go` | Settings API handlers |
| `modules/analytics/services/analytics_service.go` | Business logic |
| `docs/BOTECTION-CALLBACK-API.md` | Full API documentation |

### Static Assets

| Path | Description |
|------|-------------|
| `web/static/img/templates/` | Challenge template preview GIFs |
| `web/static/flags/` | Country flag PNGs (e.g., `us.png`) |

### Challenge Template Preview GIFs

**Directory:** `/home/this/dev/code/botginx/web/static/img/templates/`

| File | Template |
|------|----------|
| `cloudflare.gif` | Cloudflare style |
| `humancheck.gif` | Human verification |
| `humansecurity.gif` | Human Security branded |
| `slidepuzzle.gif` | Slide puzzle |
| `smartpuzzle.gif` | Smart puzzle (multiple types) |
| `simple.gif` | Minimal style |
| `lobby.gif` | Lobby waiting |
| `foyer.gif` | Foyer entrance |
| `latch.gif` | Latch security |
| `interstitial.gif` | Interstitial page |

### VPS Deployment Paths

| Path | Description |
|------|-------------|
| `/etc/botection/links/` | Per-link settings JSON files |
| `/etc/botection/links/{linkId}.json` | Individual link settings |
| `/etc/botection/config.yaml` | Botection main config |

---

## Settings UI Components

**Template:** `/home/this/dev/code/botginx/pkg/protection/templates/settings.html`

### UI Sections

1. **Challenge Template Selector**
   - Dropdown with template options
   - Live GIF preview on selection
   - Theme mode toggle (auto/light/dark)

2. **Bot Detection Toggles**
   - Block bots (checkbox)
   - Block Tor (checkbox)
   - Block proxy/VPN (checkbox)
   - Block datacenter IPs (checkbox)
   - Block headless browsers (checkbox)

3. **Behavior Score Slider**
   - Range: 0-100
   - 0 = disabled
   - Shows threshold value

4. **Geographic Filtering**
   - Mode selector (off/whitelist/blacklist)
   - Country multi-select with flags
   - Searchable dropdown

5. **Device Filtering**
   - Mode selector (off/whitelist/blacklist)
   - Checkboxes: Desktop, Mobile, Tablet

6. **Redirect on Block**
   - URL input field
   - Empty = show challenge page

### JavaScript Functions (in settings.html)

```javascript
// Load settings from API
loadSettings(linkId)

// Save settings to API
saveSettings(linkId, settings)

// Template preview handling
updateTemplatePreview(templateName)

// Country selector
initCountrySelector()

// Form validation
validateSettings()
```

---

## API Endpoints

### Settings Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/user/analytics/link/{id}/settings` | Settings page (HTML) |
| GET | `/api/analytics/link/{id}/settings` | Get settings (JSON) |
| POST | `/api/analytics/link/{id}/settings` | Save settings |

### Botection Callback (Server-to-Server)

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/botection/should-block` | Check if visitor should be blocked |

---

## Sample Data

### Settings Object (JavaScript)

```javascript
const settings = {
  template: 'cloudflare',
  themeMode: 'auto',
  blockBots: true,
  blockTor: true,
  blockProxy: false,
  blockDatacenter: true,
  blockHeadless: true,
  countryMode: 'blacklist',
  countryList: ['RU', 'CN', 'IR', 'KP'],
  asnMode: '',
  asnList: [],
  deviceMode: 'whitelist',
  deviceList: ['desktop', 'mobile'],
  minBehaviorScore: 30,
  redirectOnBlock: 'https://google.com'
};
```

### Available Countries (Sample)

```javascript
const countries = [
  { code: 'US', name: 'United States' },
  { code: 'GB', name: 'United Kingdom' },
  { code: 'CA', name: 'Canada' },
  { code: 'AU', name: 'Australia' },
  { code: 'DE', name: 'Germany' },
  // ... 200+ countries
];
```

**Full list:** `/home/this/dev/code/botginx/pkg/protection/countries.go`

---

## Integration Notes for Theme Recreation

1. **Settings UI is reusable** - The `settings.html` partial can be embedded in any page that manages a link/domain

2. **GIF previews are essential** - Users rely on visual preview to choose challenge templates

3. **Real-time save** - Settings should auto-save or show clear save button with loading state

4. **Country flags** - Display flags next to country names in selector (PNGs in `/web/static/flags/`)

5. **Mobile-friendly** - Settings form must work on mobile viewports

6. **Toast feedback** - Show success/error toasts on save

7. **Loading states** - Show spinners while settings load/save

8. **Validation** - Validate settings before save (behavior score 0-100, valid modes, etc.)
