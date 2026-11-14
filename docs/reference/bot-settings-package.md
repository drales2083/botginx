# Bot Settings Package Reference

This document provides complete reference for the bot protection settings system. Use this to rebuild or port the package to another system.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        BOT SETTINGS SYSTEM                              │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  ┌─────────────────────┐    ┌─────────────────────┐                    │
│  │  pkg/protection/    │    │  pkg/settingspush/  │                    │
│  │  (Core Data Model)  │───▶│  (VPS Deployment)   │                    │
│  └─────────────────────┘    └─────────────────────┘                    │
│           │                          │                                  │
│           ▼                          ▼                                  │
│  ┌─────────────────────┐    ┌─────────────────────┐                    │
│  │ modules/antibot     │    │ /etc/botection/     │                    │
│  │ control/ (UI)       │    │ links/{id}.json     │                    │
│  └─────────────────────┘    └─────────────────────┘                    │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## File Reference Index

| Absolute Path | Purpose |
|---------------|---------|
| `/home/this/dev/code/botginx/pkg/protection/types.go` | Core Settings struct with JSON/SQL serialization |
| `/home/this/dev/code/botginx/pkg/protection/defaults.go` | Default settings factory function |
| `/home/this/dev/code/botginx/pkg/protection/countries.go` | ISO 3166-1 country codes for geo-filtering |
| `/home/this/dev/code/botginx/pkg/protection/embed.go` | Go embed directive for templates |
| `/home/this/dev/code/botginx/pkg/protection/templates/settings.html` | Reusable UI component (54KB) |
| `/home/this/dev/code/botginx/pkg/settingspush/pusher.go` | SSH-based settings deployment to VPS |
| `/home/this/dev/code/botginx/modules/antibotcontrol/services/service.go` | User defaults CRUD operations |
| `/home/this/dev/code/botginx/modules/antibotcontrol/handlers/handler.go` | HTTP API handlers |
| `/home/this/dev/code/botginx/modules/antibotcontrol/templates/index.html` | Main settings page |

---

## Core Data Model

### Settings Struct

**File:** `/home/this/dev/code/botginx/pkg/protection/types.go`

```go
package protection

import (
    "database/sql/driver"
    "encoding/json"
    "fmt"
)

// Settings holds all protection/filtering configuration
type Settings struct {
    // Challenge page appearance
    Template  string `json:"template"`  // "cloudflare", "humancheck", "humansecurity", "slidepuzzle", "smartpuzzle", "simple", "lobby"
    ThemeMode string `json:"themeMode"` // "auto", "light", "dark"

    // Geographic filtering
    CountryMode string   `json:"countryMode"` // "", "whitelist", "blacklist" (empty = disabled)
    CountryList []string `json:"countryList"` // ISO 3166-1 alpha-2 codes

    // ASN filtering
    ASNMode string   `json:"asnMode"` // "", "whitelist", "blacklist" (empty = disabled)
    ASNList []string `json:"asnList"` // ASN numbers as strings (e.g., "AS12345")

    // Device filtering
    DeviceMode string   `json:"deviceMode"` // "", "whitelist", "blacklist" (empty = disabled)
    DeviceList []string `json:"deviceList"` // "desktop", "mobile", "tablet"

    // Bot detection toggles
    BlockBots       bool `json:"blockBots"`
    BlockTor        bool `json:"blockTor"`
    BlockProxy      bool `json:"blockProxy"`
    BlockDatacenter bool `json:"blockDatacenter"`
    BlockHeadless   bool `json:"blockHeadless"`

    // Behavior scoring
    MinBehaviorScore int `json:"minBehaviorScore"` // 0-100, 0 = disabled

    // Block action
    RedirectOnBlock string `json:"redirectOnBlock"` // URL to redirect blocked visitors, empty = show block page
}
```

### Validation Method

```go
func (s *Settings) Validate() error {
    validModes := map[string]bool{"all": true, "whitelist": true, "blacklist": true, "": true}

    if !validModes[s.CountryMode] {
        return fmt.Errorf("invalid countryMode: %s", s.CountryMode)
    }
    if !validModes[s.ASNMode] {
        return fmt.Errorf("invalid asnMode: %s", s.ASNMode)
    }
    if !validModes[s.DeviceMode] {
        return fmt.Errorf("invalid deviceMode: %s", s.DeviceMode)
    }

    if s.MinBehaviorScore < 0 || s.MinBehaviorScore > 100 {
        return fmt.Errorf("minBehaviorScore must be 0-100, got %d", s.MinBehaviorScore)
    }

    for _, d := range s.DeviceList {
        if d != "desktop" && d != "mobile" && d != "tablet" {
            return fmt.Errorf("invalid device: %s", d)
        }
    }

    return nil
}
```

### SQL Serialization

Settings implements `driver.Valuer` and `sql.Scanner` for PostgreSQL JSONB storage:

```go
// Value implements driver.Valuer for database storage
func (s Settings) Value() (driver.Value, error) {
    return json.Marshal(s)
}

// Scan implements sql.Scanner for database retrieval
func (s *Settings) Scan(value interface{}) error {
    if value == nil {
        *s = Settings{}
        return nil
    }

    var data []byte
    switch v := value.(type) {
    case []byte:
        data = v
    case string:
        data = []byte(v)
    default:
        return fmt.Errorf("cannot scan %T into Settings", value)
    }

    return json.Unmarshal(data, s)
}
```

---

## Default Settings

**File:** `/home/this/dev/code/botginx/pkg/protection/defaults.go`

```go
package protection

// GetDefaultSettings returns sensible defaults for new links.
// Philosophy: challenge over block - Tor/VPN/datacenter users see challenge, not block.
func GetDefaultSettings() Settings {
    return Settings{
        Template:         "cloudflare",
        ThemeMode:        "light",
        CountryMode:      "",           // disabled
        CountryList:      []string{},
        ASNMode:          "",           // disabled
        ASNList:          []string{},
        DeviceMode:       "",           // disabled
        DeviceList:       []string{},
        BlockBots:        true,         // block known bots
        BlockTor:         false,        // allow Tor (challenge)
        BlockProxy:       false,        // allow proxies (challenge)
        BlockDatacenter:  false,        // allow datacenter IPs (challenge)
        BlockHeadless:    false,        // allow headless browsers (challenge)
        MinBehaviorScore: 0,            // disabled
        RedirectOnBlock:  "",           // show block page
    }
}
```

---

## Country Codes

**File:** `/home/this/dev/code/botginx/pkg/protection/countries.go`

```go
package protection

type Country struct {
    Code string `json:"code"` // ISO 3166-1 alpha-2
    Name string `json:"name"`
}

func GetCountries() []Country {
    return []Country{
        {Code: "US", Name: "United States"},
        {Code: "GB", Name: "United Kingdom"},
        {Code: "DE", Name: "Germany"},
        {Code: "FR", Name: "France"},
        // ... 150+ countries total
    }
}
```

---

## Settings Push (VPS Deployment)

**File:** `/home/this/dev/code/botginx/pkg/settingspush/pusher.go`

### LinkSettings (Wire Format)

This struct is what the antibot proxy (botection) expects:

```go
package settingspush

type LinkSettings struct {
    LinkID           string   `json:"link_id"`
    UserID           string   `json:"user_id"`
    Host             string   `json:"host"`
    Template         string   `json:"template"`         // cloudflare, humancheck, humansecurity, slidepuzzle, smartpuzzle, simple, lobby, foyer, latch, interstitial
    PuzzleMode       string   `json:"puzzle_mode"`      // auto, slider, drag, rotate, icon, text, strip, gobang, match3
    ThemeMode        string   `json:"theme_mode"`       // auto, light, dark
    BlockBots        bool     `json:"block_bots"`
    BlockTor         bool     `json:"block_tor"`
    BlockProxy       bool     `json:"block_proxy"`
    BlockDatacenter  bool     `json:"block_datacenter"`
    BlockHeadless    bool     `json:"block_headless"`
    CountryMode      string   `json:"country_mode"`
    CountryList      []string `json:"country_list"`
    ASNMode          string   `json:"asn_mode"`
    ASNList          []string `json:"asn_list"`
    DeviceMode       string   `json:"device_mode"`
    DeviceList       []string `json:"device_list"`
    MinBehaviorScore int      `json:"min_behavior_score"`
    RedirectOnBlock  string   `json:"redirect_on_block"`
    UpdatedAt        string   `json:"updated_at"`

    // Cloudflare Turnstile credentials (from domain settings)
    TurnstileSiteKey   string `json:"turnstile_site_key,omitempty"`
    TurnstileSecretKey string `json:"turnstile_secret_key,omitempty"`
}
```

### ServerInfo

```go
type ServerInfo struct {
    IP       string
    Port     int
    User     string
    Password string
}
```

### Pusher Service

```go
type Pusher struct{}

func New() *Pusher {
    return &Pusher{}
}

// Push sends a settings file to the VPS
func (p *Pusher) Push(server ServerInfo, linkID string, settings LinkSettings) error {
    // 1. Set updated timestamp
    settings.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

    // 2. Generate JSON
    data, err := json.MarshalIndent(settings, "", "  ")
    if err != nil {
        return fmt.Errorf("failed to marshal settings: %w", err)
    }

    // 3. Create SSH client and connect
    client := ssh.NewClient(ssh.Config{
        Host:         server.IP,
        Port:         server.Port,
        User:         server.User,
        Password:     server.Password,
        TrustOnFirst: true,
    })
    defer client.Close()

    if err := client.Connect(); err != nil {
        return fmt.Errorf("failed to connect to %s: %w", server.IP, err)
    }

    // 4. Ensure directory exists
    remotePath := fmt.Sprintf("/etc/botection/links/%s.json", linkID)
    if _, err := client.Exec("mkdir -p /etc/botection/links"); err != nil {
        return fmt.Errorf("failed to create directory: %w", err)
    }

    // 5. Write file
    if err := client.WriteFile(remotePath, data, 0644); err != nil {
        return fmt.Errorf("failed to write settings file: %w", err)
    }

    return nil
}

// Delete removes a settings file from the VPS
func (p *Pusher) Delete(server ServerInfo, linkID string) error {
    // Connect via SSH and run: rm -f /etc/botection/links/{linkID}.json
}
```

---

## Database Schema

### User Antibot Defaults Table

```sql
CREATE TABLE user_antibot_defaults (
    user_id    VARCHAR(36) PRIMARY KEY,
    settings   JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_user_antibot_defaults_user ON user_antibot_defaults(user_id);
```

### Link Settings (stored per-link)

Settings are stored directly on the link/redirect record as a JSONB column:

```sql
-- Example: redirect_links table
CREATE TABLE redirect_links (
    id              VARCHAR(36) PRIMARY KEY,
    user_id         VARCHAR(36) NOT NULL,
    name            VARCHAR(255),
    destination_url TEXT NOT NULL,
    bot_settings    JSONB DEFAULT '{}',  -- protection.Settings
    created_at      TIMESTAMP DEFAULT NOW(),
    updated_at      TIMESTAMP DEFAULT NOW()
);
```

---

## Service Layer

**File:** `/home/this/dev/code/botginx/modules/antibotcontrol/services/service.go`

```go
package services

import (
    "github.com/botginx/botginx/pkg/protection"
    "github.com/jmoiron/sqlx"
)

type AntibotControlService struct {
    db *sqlx.DB
}

func NewAntibotControlService(db *sqlx.DB) *AntibotControlService {
    return &AntibotControlService{db: db}
}

type UserDefaults struct {
    UserID   string              `db:"user_id" json:"userId"`
    Settings protection.Settings `db:"settings" json:"settings"`
}

// GetDefaults retrieves user's default settings (or system defaults if none set)
func (s *AntibotControlService) GetDefaults(userID string) (*protection.Settings, error) {
    var settings protection.Settings

    err := s.db.Get(&settings, `
        SELECT settings FROM user_antibot_defaults WHERE user_id = $1
    `, userID)

    if err != nil {
        // Return system defaults if none set
        return &protection.Settings{
            Template:         "cloudflare",
            ThemeMode:        "auto",
            CountryMode:      "",
            CountryList:      []string{},
            ASNMode:          "",
            ASNList:          []string{},
            DeviceMode:       "",
            DeviceList:       []string{},
            BlockBots:        true,
            BlockTor:         false,
            BlockProxy:       false,
            BlockDatacenter:  false,
            BlockHeadless:    true,
            MinBehaviorScore: 0,
            RedirectOnBlock:  "",
        }, nil
    }

    return &settings, nil
}

// SaveDefaults saves user's default settings
func (s *AntibotControlService) SaveDefaults(userID string, settings *protection.Settings) error {
    if err := settings.Validate(); err != nil {
        return err
    }

    _, err := s.db.Exec(`
        INSERT INTO user_antibot_defaults (user_id, settings)
        VALUES ($1, $2)
        ON CONFLICT (user_id) DO UPDATE SET
            settings = EXCLUDED.settings,
            updated_at = NOW()
    `, userID, settings)

    return err
}

// ResetDefaults removes user's custom defaults (falls back to system defaults)
func (s *AntibotControlService) ResetDefaults(userID string) error {
    _, err := s.db.Exec(`
        DELETE FROM user_antibot_defaults WHERE user_id = $1
    `, userID)
    return err
}
```

---

## Template Embedding

**File:** `/home/this/dev/code/botginx/pkg/protection/embed.go`

```go
package protection

import "embed"

//go:embed templates/*.html
var TemplatesFS embed.FS
```

The embedded template at `/home/this/dev/code/botginx/pkg/protection/templates/settings.html` (54KB) contains the reusable UI component with:
- Challenge template selector
- Theme mode toggle
- Country whitelist/blacklist with multi-select
- ASN filtering
- Device type filtering
- Bot detection toggles (Tor, Proxy, Datacenter, Headless)
- Behavior score slider
- Redirect URL input

---

## Configuration Constants

### Challenge Templates

| Value | Description |
|-------|-------------|
| `cloudflare` | Cloudflare-style challenge page |
| `humancheck` | Human verification page |
| `humansecurity` | Human Security branded page |
| `slidepuzzle` | Slide puzzle CAPTCHA |
| `smartpuzzle` | AI-powered puzzle |
| `simple` | Simple click challenge |
| `lobby` | Waiting room style |
| `transit` | Transit/loading style |
| `foyer` | Foyer/entrance style |
| `latch` | Latch/gate style |
| `interstitial` | Interstitial page |

### Puzzle Modes

| Value | Description |
|-------|-------------|
| `auto` | Auto-select based on risk |
| `slider` | Slide to complete |
| `drag` | Drag and drop |
| `rotate` | Rotate image |
| `icon` | Click matching icons |
| `text` | Type shown text |
| `strip` | Strip puzzle |
| `gobang` | Gobang/five-in-row game |
| `match3` | Match-3 game |

### Filter Modes

| Value | Description |
|-------|-------------|
| `""` (empty) | Filtering disabled |
| `whitelist` | Allow only listed values |
| `blacklist` | Block listed values |

### Device Types

| Value | Description |
|-------|-------------|
| `desktop` | Desktop/laptop browsers |
| `mobile` | Mobile phone browsers |
| `tablet` | Tablet browsers |

---

## Integration Example

### Using the Package in Another Module

```go
package mymodule

import (
    "github.com/botginx/botginx/pkg/protection"
    "github.com/botginx/botginx/pkg/settingspush"
)

func SaveLinkWithBotSettings(linkID, userID string, settings protection.Settings) error {
    // 1. Validate settings
    if err := settings.Validate(); err != nil {
        return err
    }

    // 2. Save to database (settings auto-serializes to JSONB)
    _, err := db.Exec(`
        UPDATE redirect_links 
        SET bot_settings = $1, updated_at = NOW() 
        WHERE id = $2
    `, settings, linkID)
    if err != nil {
        return err
    }

    // 3. Push to antibot VPS
    pusher := settingspush.New()
    return pusher.Push(
        settingspush.ServerInfo{
            IP:       os.Getenv("GUARD_VPS_IP"),
            Port:     22,
            User:     os.Getenv("GUARD_VPS_USER"),
            Password: os.Getenv("GUARD_VPS_PASSWORD"),
        },
        linkID,
        settingspush.LinkSettings{
            LinkID:          linkID,
            UserID:          userID,
            Template:        settings.Template,
            ThemeMode:       settings.ThemeMode,
            BlockBots:       settings.BlockBots,
            BlockTor:        settings.BlockTor,
            BlockProxy:      settings.BlockProxy,
            BlockDatacenter: settings.BlockDatacenter,
            BlockHeadless:   settings.BlockHeadless,
            CountryMode:     settings.CountryMode,
            CountryList:     settings.CountryList,
            ASNMode:         settings.ASNMode,
            ASNList:         settings.ASNList,
            DeviceMode:      settings.DeviceMode,
            DeviceList:      settings.DeviceList,
            MinBehaviorScore: settings.MinBehaviorScore,
            RedirectOnBlock: settings.RedirectOnBlock,
        },
    )
}
```

---

## Environment Variables

| Variable | Description |
|----------|-------------|
| `GUARD_VPS_IP` | Antibot VPS IP address |
| `GUARD_VPS_PORT` | SSH port (default: 22) |
| `GUARD_VPS_USER` | SSH username |
| `GUARD_VPS_PASSWORD` | SSH password |

---

## Remote File Structure

Settings are pushed to the antibot VPS at:

```
/etc/botection/
└── links/
    ├── {link_id_1}.json
    ├── {link_id_2}.json
    └── ...
```

Each JSON file contains the full `LinkSettings` struct.

---

## Dependencies

| Package | Import Path | Purpose |
|---------|-------------|---------|
| sqlx | `github.com/jmoiron/sqlx` | Database operations |
| ssh | `github.com/botginx/botginx/pkg/ssh` | SSH client for VPS push |

---

## Porting Checklist

To port this package to another system:

1. [ ] Copy `/home/this/dev/code/botginx/pkg/protection/types.go` - core Settings struct
2. [ ] Copy `/home/this/dev/code/botginx/pkg/protection/defaults.go` - default values
3. [ ] Copy `/home/this/dev/code/botginx/pkg/protection/countries.go` - country list
4. [ ] Create database table `user_antibot_defaults` with JSONB column
5. [ ] Add `bot_settings JSONB` column to your links/entities table
6. [ ] Implement service layer for CRUD operations
7. [ ] Copy `/home/this/dev/code/botginx/pkg/settingspush/pusher.go` if you need VPS deployment
8. [ ] Adapt the HTML template from `/home/this/dev/code/botginx/pkg/protection/templates/settings.html`
