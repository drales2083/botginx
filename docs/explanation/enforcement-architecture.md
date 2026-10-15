# Bot Protection Enforcement Plan

## Status

**Current state:** Settings persist and `ShouldBlock()` logic runs, but no serving layer acts on the decision. Visits are labeled blocked/allowed in analytics; real visitors are never actually stopped.

**Goal:** Make every toggle on `/user/analytics/link/{id}/settings` actually block matching traffic.

---

## Architecture: Push Settings Files

When a user saves link settings in the botginx panel (VPS A), we generate a `settings.json` file and push it to the deploy VPS (VPS B) where the redirect link lives. Botection reads from local files — no network call at request time.

```
VPS A (Main Panel)                          VPS B (Deploy VPS)
┌─────────────────────┐                     ┌─────────────────────┐
│  Botginx Panel      │                     │  Botection          │
│  ├─ User dashboard  │                     │  ├─ Reads settings  │
│  ├─ Settings UI     │     SSH/SCP         │  │   from local file │
│  └─ Database        │ ──────────────────► │  └─ Blocks/allows   │
│                     │  settings.json      │                     │
└─────────────────────┘                     └─────────────────────┘
```

### Why Push Over Callback

| Concern | Push files | Callback API |
|---------|------------|--------------|
| Request latency | Zero (local file) | +50-200ms (network) |
| Panel dependency | None at runtime | Hard dependency |
| Offline operation | Works if VPS A down | Fails if VPS A down |
| Settings delay | ~2-5 seconds (SCP) | Instant |
| Complexity | SSH already exists | Auth, HTTPS, timeouts |

Push wins because:
- Deploy already uses SSH — same mechanism for settings
- Zero latency at request time
- No network dependency during traffic serving
- VPS B works even if VPS A goes offline

---

## Settings File Format

Each deployed link gets a settings file at:
```
/etc/botection/links/{linkId}.json
```

Example `/etc/botection/links/lnk-abc123.json`:
```json
{
  "link_id": "lnk-abc123",
  "host": "amber-canyon.example.com",
  "block_bots": true,
  "block_tor": true,
  "block_proxy": false,
  "block_datacenter": true,
  "block_headless": true,
  "country_mode": "whitelist",
  "country_list": ["US", "CA", "GB"],
  "device_mode": "allow",
  "device_list": [],
  "min_behavior_score": 0,
  "redirect_on_block": "https://google.com",
  "updated_at": "2026-08-29T18:30:00Z"
}
```

---

## Botginx: Push Settings on Save

When user saves settings in the panel:

1. Save to database (existing)
2. Generate `settings.json` for that link
3. Look up which VPS the link is deployed to
4. SCP the file to that VPS

### Implementation in `modules/analytics/handlers/handler.go`

```go
func (h *Handler) APIUpdateSettings(w http.ResponseWriter, r *http.Request) {
    linkID := chi.URLParam(r, "linkId")
    
    // ... existing validation and save to database ...
    
    if err := h.service.SaveLinkSettings(settings); err != nil {
        h.jsonError(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    // Push settings to deploy VPS
    if err := h.pushSettingsToVPS(linkID, settings); err != nil {
        // Log but don't fail — user can retry
        log.Printf("Failed to push settings for %s: %v", linkID, err)
    }
    
    h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

func (h *Handler) pushSettingsToVPS(linkID string, settings *models.LinkSettings) error {
    // 1. Get the link to find which VPS it's deployed to
    link, err := h.links.GetByID(linkID)
    if err != nil || link.ServerID == "" {
        return nil // Not deployed yet, nothing to push
    }
    
    // 2. Get server SSH details
    server, err := h.servers.GetByID(link.ServerID)
    if err != nil {
        return err
    }
    
    // 3. Generate settings JSON
    settingsJSON := map[string]interface{}{
        "link_id":            settings.LinkID,
        "host":               link.Host(),
        "block_bots":         settings.BlockBots,
        "block_tor":          settings.BlockTor,
        "block_proxy":        settings.BlockProxy,
        "block_datacenter":   settings.BlockDatacenter,
        "block_headless":     settings.BlockHeadless,
        "country_mode":       settings.CountryMode,
        "country_list":       settings.CountryList,
        "device_mode":        settings.DeviceMode,
        "device_list":        settings.DeviceList,
        "min_behavior_score": settings.MinBehaviorScore,
        "redirect_on_block":  settings.RedirectOnBlock,
        "updated_at":         time.Now().UTC().Format(time.RFC3339),
    }
    
    data, _ := json.MarshalIndent(settingsJSON, "", "  ")
    
    // 4. SCP to VPS
    remotePath := fmt.Sprintf("/etc/botection/links/%s.json", linkID)
    return h.ssh.WriteFile(server, remotePath, data)
}
```

---

## Botection: Read Local Settings Files

Botection reads settings from `/etc/botection/links/` directory.

### Configuration in `config.yaml`

```yaml
# Per-link settings from external panel
link_settings:
  enabled: true
  directory: "/etc/botection/links"
  watch: true  # Watch for file changes, reload automatically
```

### Implementation

```go
// internal/settings/loader.go
package settings

type LinkSettings struct {
    LinkID           string   `json:"link_id"`
    Host             string   `json:"host"`
    BlockBots        bool     `json:"block_bots"`
    BlockTor         bool     `json:"block_tor"`
    BlockProxy       bool     `json:"block_proxy"`
    BlockDatacenter  bool     `json:"block_datacenter"`
    BlockHeadless    bool     `json:"block_headless"`
    CountryMode      string   `json:"country_mode"`
    CountryList      []string `json:"country_list"`
    DeviceMode       string   `json:"device_mode"`
    DeviceList       []string `json:"device_list"`
    MinBehaviorScore int      `json:"min_behavior_score"`
    RedirectOnBlock  string   `json:"redirect_on_block"`
}

type Loader struct {
    dir      string
    settings map[string]*LinkSettings // keyed by host
    mu       sync.RWMutex
}

func NewLoader(dir string) *Loader {
    l := &Loader{
        dir:      dir,
        settings: make(map[string]*LinkSettings),
    }
    l.loadAll()
    return l
}

func (l *Loader) loadAll() {
    files, _ := filepath.Glob(filepath.Join(l.dir, "*.json"))
    for _, f := range files {
        l.loadFile(f)
    }
}

func (l *Loader) loadFile(path string) {
    data, err := os.ReadFile(path)
    if err != nil {
        return
    }
    var s LinkSettings
    if json.Unmarshal(data, &s) == nil {
        l.mu.Lock()
        l.settings[s.Host] = &s
        l.mu.Unlock()
    }
}

func (l *Loader) GetByHost(host string) *LinkSettings {
    l.mu.RLock()
    defer l.mu.RUnlock()
    return l.settings[host]
}

func (l *Loader) ShouldBlock(host string, ctx *RequestContext) (bool, string, string) {
    s := l.GetByHost(host)
    if s == nil {
        return false, "", "" // No settings, use global rules
    }
    
    if s.BlockBots && ctx.Score >= 70 {
        return true, "bot_blocked", s.RedirectOnBlock
    }
    if s.BlockTor && ctx.IsTor {
        return true, "tor_blocked", s.RedirectOnBlock
    }
    if s.BlockProxy && ctx.IsProxy {
        return true, "proxy_blocked", s.RedirectOnBlock
    }
    if s.BlockDatacenter && ctx.IsDatacenter {
        return true, "datacenter_blocked", s.RedirectOnBlock
    }
    if s.BlockHeadless && ctx.IsHeadless {
        return true, "headless_blocked", s.RedirectOnBlock
    }
    if s.MinBehaviorScore > 0 && ctx.BehaviorScore < s.MinBehaviorScore {
        return true, "low_behavior_score", s.RedirectOnBlock
    }
    
    // Country filtering
    if s.CountryMode == "whitelist" && len(s.CountryList) > 0 {
        if !contains(s.CountryList, ctx.Country) {
            return true, "country_not_whitelisted", s.RedirectOnBlock
        }
    }
    if s.CountryMode == "blacklist" && len(s.CountryList) > 0 {
        if contains(s.CountryList, ctx.Country) {
            return true, "country_blacklisted", s.RedirectOnBlock
        }
    }
    
    return false, "", ""
}
```

### Integration in Engine

```go
// In engine.go Process() method, after modules run:

// Check per-link settings if enabled
if e.linkSettings != nil {
    block, reason, redirect := e.linkSettings.ShouldBlock(ctx.Host, ctx)
    if block {
        return Decision{
            Action:   Block,
            Score:    totalScore,
            Reason:   reason,
            Module:   "link_settings",
            Redirect: redirect,
            Results:  results,
        }
    }
}

// Continue with global threshold rules...
```

---

## File Watcher (Optional)

For instant updates without restarting botection:

```go
func (l *Loader) Watch() {
    watcher, _ := fsnotify.NewWatcher()
    watcher.Add(l.dir)
    
    go func() {
        for event := range watcher.Events {
            if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
                l.loadFile(event.Name)
            }
            if event.Op&fsnotify.Remove != 0 {
                l.removeByFile(event.Name)
            }
        }
    }()
}
```

---

## Deploy Flow

### Initial Deploy

When a link is first deployed:

1. Botginx generates nginx config, splash page, etc.
2. Botginx also generates `settings.json` from current settings
3. All files SCP'd to VPS B
4. Botection picks up the new settings file

### Settings Update

When user changes settings:

1. Save to database
2. Generate new `settings.json`
3. SCP just the settings file (fast, ~1KB)
4. Botection reloads automatically (file watcher)

### Link Deletion

When a link is deleted:

1. Remove from database
2. SSH to VPS B, delete nginx config and settings file
3. Botection removes from memory (file watcher)

---

## Directory Structure on VPS B

```
/etc/botection/
├── config.yaml           # Main botection config
└── links/
    ├── lnk-abc123.json   # Settings for link abc123
    ├── lnk-def456.json   # Settings for link def456
    └── lnk-ghi789.json   # Settings for link ghi789
```

---

## Implementation Phases

### Phase 1: Settings Push (this plan)

**In botginx:**
1. Add `pushSettingsToVPS()` method
2. Call it from `APIUpdateSettings` handler
3. Call it during initial deploy
4. Call delete on link removal

**In botection:**
1. Add `internal/settings/loader.go`
2. Add `link_settings` config section
3. Wire into engine after module processing
4. Add file watcher for live reloads

### Phase 2: JS Challenge

- Splash page runs fingerprint + timing probe
- Results included in webhook
- Settings can gate on behavior score

### Phase 3: Turnstile Integration

- If enabled, splash page embeds Turnstile widget
- Botection verifies token
- Already have site_key/secret_key fields

---

## Fallback Behavior

If no settings file exists for a host:
- Botection uses its global threshold rules
- Standalone VPS (no botginx) work exactly as before
- New links work immediately after deploy

---

## Summary

| Component | Location | What it does |
|-----------|----------|--------------|
| Settings UI | Botginx `/user/analytics/link/{id}/settings` | User configures rules |
| Database | Botginx PostgreSQL | Stores settings |
| Push | Botginx → VPS B via SSH | Syncs settings.json |
| Loader | Botection `/etc/botection/links/*.json` | Reads local files |
| Enforcer | Botection engine | Applies rules at request time |
