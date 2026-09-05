# Link Shortener Module

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create a new Link Shortener module for creating short URLs with bot protection, multiple destinations (rotation), QR codes, and full protection settings.

**Architecture:** Standard botginx module structure with handlers, services, models, and templates. Uses shared Protection Component for filtering settings.

**Tech Stack:** Go, PostgreSQL, html/template, JavaScript, QR code generation

**Spec:** New module at `/user/shortener` providing URL shortening with bot protection.

**Dependencies:** 
- `pkg/protection` - Protection Settings Component (must be implemented first)
- `modules/domains` - Domain provider for available domains

## Global Constraints

- Follow existing module patterns (see redirectlinks module)
- Use existing domain system (no new domain tables)
- QR codes generated server-side with go-qrcode library
- Deploy short links to VPS via existing deployment system
- All routes under `/user/shortener`

---

### Task 1: Create Module Directory Structure

**Files:**
- Create: `modules/shortener/` directory structure

**Interfaces:**
- Consumes: Nothing
- Produces: Empty module structure

- [ ] **Step 1: Create directories**

```bash
mkdir -p modules/shortener/{handlers,services,models,migrations,templates}
```

- [ ] **Step 2: Create placeholder files**

```bash
touch modules/shortener/module.go
touch modules/shortener/handlers/handler.go
touch modules/shortener/services/shortener_service.go
touch modules/shortener/models/short_link.go
touch modules/shortener/migrations/001_create_tables.sql
touch modules/shortener/templates/{list,new,show}.html
```

- [ ] **Step 3: Commit**

```bash
git add modules/shortener/
git commit -m "feat(shortener): create module directory structure"
```

---

### Task 2: Create Database Schema

**Files:**
- Create: `modules/shortener/migrations/001_create_tables.sql`

**Interfaces:**
- Consumes: Nothing
- Produces: Database tables for short links

- [ ] **Step 1: Write migration**

```sql
-- Short links table
CREATE TABLE IF NOT EXISTS short_links (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    domain_id TEXT NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    destinations JSONB NOT NULL DEFAULT '[]',
    rotation_mode TEXT NOT NULL DEFAULT 'random',
    bot_error INTEGER NOT NULL DEFAULT 403,
    qr_enabled BOOLEAN NOT NULL DEFAULT false,
    protection_settings JSONB,
    click_count INTEGER NOT NULL DEFAULT 0,
    human_count INTEGER NOT NULL DEFAULT 0,
    bot_count INTEGER NOT NULL DEFAULT 0,
    last_click_at TIMESTAMP,
    deploy_status TEXT NOT NULL DEFAULT 'pending',
    deployed_url TEXT,
    deploy_error TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    
    CONSTRAINT unique_short_link_domain_path UNIQUE(domain_id, path)
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_short_links_user_id ON short_links(user_id);
CREATE INDEX IF NOT EXISTS idx_short_links_domain_id ON short_links(domain_id);
CREATE INDEX IF NOT EXISTS idx_short_links_is_active ON short_links(is_active);

-- Click tracking for short links
CREATE TABLE IF NOT EXISTS short_link_clicks (
    id TEXT PRIMARY KEY,
    link_id TEXT NOT NULL REFERENCES short_links(id) ON DELETE CASCADE,
    visitor_ip TEXT,
    visitor_ip_hash TEXT,
    country TEXT,
    city TEXT,
    device TEXT,
    browser TEXT,
    os TEXT,
    is_bot BOOLEAN NOT NULL DEFAULT false,
    bot_type TEXT,
    bot_reason TEXT,
    destination_used TEXT,
    referer TEXT,
    user_agent TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_short_link_clicks_link_id ON short_link_clicks(link_id);
CREATE INDEX IF NOT EXISTS idx_short_link_clicks_created_at ON short_link_clicks(created_at);
CREATE INDEX IF NOT EXISTS idx_short_link_clicks_is_bot ON short_link_clicks(is_bot);
```

- [ ] **Step 2: Commit**

```bash
git add modules/shortener/migrations/001_create_tables.sql
git commit -m "feat(shortener): add database schema"
```

---

### Task 3: Create Models

**Files:**
- Create: `modules/shortener/models/short_link.go`

**Interfaces:**
- Consumes: `pkg/protection.Settings`
- Produces: `ShortLink`, `ShortLinkClick`, input structs

- [ ] **Step 1: Write models**

```go
// modules/shortener/models/short_link.go
package models

import (
    "database/sql/driver"
    "encoding/json"
    "time"

    "github.com/botginx/botginx/pkg/protection"
)

// JSONArray for storing string arrays in PostgreSQL JSONB
type JSONArray []string

func (j JSONArray) Value() (driver.Value, error) {
    return json.Marshal(j)
}

func (j *JSONArray) Scan(value interface{}) error {
    if value == nil {
        *j = []string{}
        return nil
    }
    var data []byte
    switch v := value.(type) {
    case []byte:
        data = v
    case string:
        data = []byte(v)
    }
    return json.Unmarshal(data, j)
}

// ShortLink represents a shortened URL
type ShortLink struct {
    ID                 string              `db:"id" json:"id"`
    UserID             string              `db:"user_id" json:"userId"`
    DomainID           string              `db:"domain_id" json:"domainId"`
    DomainName         string              `db:"domain_name" json:"domainName"` // joined from domains
    Path               string              `db:"path" json:"path"`
    Destinations       JSONArray           `db:"destinations" json:"destinations"`
    RotationMode       string              `db:"rotation_mode" json:"rotationMode"` // random, sequential
    BotError           int                 `db:"bot_error" json:"botError"`
    QREnabled          bool                `db:"qr_enabled" json:"qrEnabled"`
    ProtectionSettings protection.Settings `db:"protection_settings" json:"protectionSettings"`
    ClickCount         int                 `db:"click_count" json:"clickCount"`
    HumanCount         int                 `db:"human_count" json:"humanCount"`
    BotCount           int                 `db:"bot_count" json:"botCount"`
    LastClickAt        *time.Time          `db:"last_click_at" json:"lastClickAt"`
    DeployStatus       string              `db:"deploy_status" json:"deployStatus"`
    DeployedURL        *string             `db:"deployed_url" json:"deployedUrl"`
    DeployError        *string             `db:"deploy_error" json:"deployError"`
    IsActive           bool                `db:"is_active" json:"isActive"`
    CreatedAt          time.Time           `db:"created_at" json:"createdAt"`
    UpdatedAt          time.Time           `db:"updated_at" json:"updatedAt"`
}

// FullURL returns the complete short link URL
func (s *ShortLink) FullURL() string {
    return "https://" + s.DomainName + "/" + s.Path
}

// ShortLinkClick represents a single click/visit
type ShortLinkClick struct {
    ID              string    `db:"id" json:"id"`
    LinkID          string    `db:"link_id" json:"linkId"`
    VisitorIP       string    `db:"visitor_ip" json:"-"` // never expose
    VisitorIPHash   string    `db:"visitor_ip_hash" json:"visitorIpHash"`
    Country         string    `db:"country" json:"country"`
    City            string    `db:"city" json:"city"`
    Device          string    `db:"device" json:"device"`
    Browser         string    `db:"browser" json:"browser"`
    OS              string    `db:"os" json:"os"`
    IsBot           bool      `db:"is_bot" json:"isBot"`
    BotType         string    `db:"bot_type" json:"botType"`
    BotReason       string    `db:"bot_reason" json:"botReason"`
    DestinationUsed string    `db:"destination_used" json:"destinationUsed"`
    Referer         string    `db:"referer" json:"referer"`
    UserAgent       string    `db:"user_agent" json:"userAgent"`
    CreatedAt       time.Time `db:"created_at" json:"createdAt"`
}

// CreateShortLinkInput for creating new short links
type CreateShortLinkInput struct {
    DomainID           string              `json:"domainId"`
    Path               string              `json:"path"`
    Destinations       []string            `json:"destinations"`
    RotationMode       string              `json:"rotationMode"`
    BotError           int                 `json:"botError"`
    QREnabled          bool                `json:"qrEnabled"`
    ProtectionSettings protection.Settings `json:"protectionSettings"`
}

// UpdateShortLinkInput for updating short links
type UpdateShortLinkInput struct {
    Destinations       []string             `json:"destinations,omitempty"`
    RotationMode       *string              `json:"rotationMode,omitempty"`
    BotError           *int                 `json:"botError,omitempty"`
    QREnabled          *bool                `json:"qrEnabled,omitempty"`
    ProtectionSettings *protection.Settings `json:"protectionSettings,omitempty"`
}

// ShortLinkStats for analytics
type ShortLinkStats struct {
    TotalClicks  int            `json:"totalClicks"`
    HumanClicks  int            `json:"humanClicks"`
    BotClicks    int            `json:"botClicks"`
    ByCountry    map[string]int `json:"byCountry"`
    ByDevice     map[string]int `json:"byDevice"`
    ByDay        []DayStats     `json:"byDay"`
}

type DayStats struct {
    Date   string `json:"date"`
    Humans int    `json:"humans"`
    Bots   int    `json:"bots"`
}
```

- [ ] **Step 2: Verify build**

```bash
go build ./modules/shortener/models/
```

- [ ] **Step 3: Commit**

```bash
git add modules/shortener/models/short_link.go
git commit -m "feat(shortener): add data models"
```

---

### Task 4: Create Service Layer

**Files:**
- Create: `modules/shortener/services/shortener_service.go`

**Interfaces:**
- Consumes: Database, models
- Produces: CRUD operations, stats queries

- [ ] **Step 1: Write service**

```go
// modules/shortener/services/shortener_service.go
package services

import (
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "time"

    "github.com/botginx/botginx/modules/shortener/models"
    "github.com/jmoiron/sqlx"
)

type ShortenerService struct {
    db *sqlx.DB
}

func NewShortenerService(db *sqlx.DB) *ShortenerService {
    return &ShortenerService{db: db}
}

func (s *ShortenerService) generateID() string {
    b := make([]byte, 12)
    rand.Read(b)
    return hex.EncodeToString(b)
}

// List returns all short links for a user
func (s *ShortenerService) List(userID string) ([]models.ShortLink, error) {
    var links []models.ShortLink
    err := s.db.Select(&links, `
        SELECT sl.*, d.name as domain_name
        FROM short_links sl
        JOIN domains d ON d.id = sl.domain_id
        WHERE sl.user_id = $1 AND sl.is_active = true
        ORDER BY sl.created_at DESC
    `, userID)
    return links, err
}

// Get returns a single short link
func (s *ShortenerService) Get(id string) (*models.ShortLink, error) {
    var link models.ShortLink
    err := s.db.Get(&link, `
        SELECT sl.*, d.name as domain_name
        FROM short_links sl
        JOIN domains d ON d.id = sl.domain_id
        WHERE sl.id = $1
    `, id)
    if err != nil {
        return nil, err
    }
    return &link, nil
}

// Create creates a new short link
func (s *ShortenerService) Create(userID string, input models.CreateShortLinkInput) (*models.ShortLink, error) {
    id := s.generateID()
    
    if input.RotationMode == "" {
        input.RotationMode = "random"
    }
    if input.BotError == 0 {
        input.BotError = 403
    }

    link := &models.ShortLink{
        ID:                 id,
        UserID:             userID,
        DomainID:           input.DomainID,
        Path:               input.Path,
        Destinations:       input.Destinations,
        RotationMode:       input.RotationMode,
        BotError:           input.BotError,
        QREnabled:          input.QREnabled,
        ProtectionSettings: input.ProtectionSettings,
        DeployStatus:       "pending",
        IsActive:           true,
        CreatedAt:          time.Now(),
        UpdatedAt:          time.Now(),
    }

    _, err := s.db.NamedExec(`
        INSERT INTO short_links (
            id, user_id, domain_id, path, destinations, rotation_mode,
            bot_error, qr_enabled, protection_settings, deploy_status,
            is_active, created_at, updated_at
        ) VALUES (
            :id, :user_id, :domain_id, :path, :destinations, :rotation_mode,
            :bot_error, :qr_enabled, :protection_settings, :deploy_status,
            :is_active, :created_at, :updated_at
        )
    `, link)
    if err != nil {
        return nil, err
    }

    return s.Get(id)
}

// Update updates a short link
func (s *ShortenerService) Update(id string, input models.UpdateShortLinkInput) (*models.ShortLink, error) {
    updates := []string{}
    args := []interface{}{id}
    argNum := 2

    if input.Destinations != nil {
        updates = append(updates, fmt.Sprintf("destinations = $%d", argNum))
        args = append(args, models.JSONArray(input.Destinations))
        argNum++
    }
    if input.RotationMode != nil {
        updates = append(updates, fmt.Sprintf("rotation_mode = $%d", argNum))
        args = append(args, *input.RotationMode)
        argNum++
    }
    if input.BotError != nil {
        updates = append(updates, fmt.Sprintf("bot_error = $%d", argNum))
        args = append(args, *input.BotError)
        argNum++
    }
    if input.QREnabled != nil {
        updates = append(updates, fmt.Sprintf("qr_enabled = $%d", argNum))
        args = append(args, *input.QREnabled)
        argNum++
    }
    if input.ProtectionSettings != nil {
        updates = append(updates, fmt.Sprintf("protection_settings = $%d", argNum))
        args = append(args, *input.ProtectionSettings)
        argNum++
    }

    if len(updates) == 0 {
        return s.Get(id)
    }

    query := fmt.Sprintf(`UPDATE short_links SET %s, updated_at = NOW() WHERE id = $1`,
        stringJoin(updates, ", "))
    _, err := s.db.Exec(query, args...)
    if err != nil {
        return nil, err
    }

    return s.Get(id)
}

// Delete soft-deletes a short link
func (s *ShortenerService) Delete(id string) error {
    _, err := s.db.Exec(`UPDATE short_links SET is_active = false, updated_at = NOW() WHERE id = $1`, id)
    return err
}

// GetStats returns analytics for a short link
func (s *ShortenerService) GetStats(id string, days int) (*models.ShortLinkStats, error) {
    stats := &models.ShortLinkStats{
        ByCountry: make(map[string]int),
        ByDevice:  make(map[string]int),
        ByDay:     []models.DayStats{},
    }

    // Get totals
    var link models.ShortLink
    err := s.db.Get(&link, `SELECT click_count, human_count, bot_count FROM short_links WHERE id = $1`, id)
    if err != nil {
        return nil, err
    }
    stats.TotalClicks = link.ClickCount
    stats.HumanClicks = link.HumanCount
    stats.BotClicks = link.BotCount

    // By country
    rows, err := s.db.Query(`
        SELECT country, COUNT(*) as count
        FROM short_link_clicks
        WHERE link_id = $1 AND created_at > NOW() - INTERVAL '1 day' * $2
        GROUP BY country
        ORDER BY count DESC
        LIMIT 10
    `, id, days)
    if err == nil {
        defer rows.Close()
        for rows.Next() {
            var country string
            var count int
            rows.Scan(&country, &count)
            if country == "" {
                country = "Unknown"
            }
            stats.ByCountry[country] = count
        }
    }

    // By device
    rows, err = s.db.Query(`
        SELECT device, COUNT(*) as count
        FROM short_link_clicks
        WHERE link_id = $1 AND created_at > NOW() - INTERVAL '1 day' * $2
        GROUP BY device
    `, id, days)
    if err == nil {
        defer rows.Close()
        for rows.Next() {
            var device string
            var count int
            rows.Scan(&device, &count)
            if device == "" {
                device = "Unknown"
            }
            stats.ByDevice[device] = count
        }
    }

    // By day
    rows, err = s.db.Query(`
        SELECT DATE(created_at) as date,
               SUM(CASE WHEN is_bot = false THEN 1 ELSE 0 END) as humans,
               SUM(CASE WHEN is_bot = true THEN 1 ELSE 0 END) as bots
        FROM short_link_clicks
        WHERE link_id = $1 AND created_at > NOW() - INTERVAL '1 day' * $2
        GROUP BY DATE(created_at)
        ORDER BY date
    `, id, days)
    if err == nil {
        defer rows.Close()
        for rows.Next() {
            var ds models.DayStats
            rows.Scan(&ds.Date, &ds.Humans, &ds.Bots)
            stats.ByDay = append(stats.ByDay, ds)
        }
    }

    return stats, nil
}

// CheckPathAvailable checks if a path is available for a domain
func (s *ShortenerService) CheckPathAvailable(domainID, path string) (bool, error) {
    var count int
    err := s.db.Get(&count, `
        SELECT COUNT(*) FROM short_links 
        WHERE domain_id = $1 AND path = $2 AND is_active = true
    `, domainID, path)
    return count == 0, err
}

// SetDeployStatus updates deployment status
func (s *ShortenerService) SetDeployStatus(id string, status string, url, errorMsg *string) error {
    _, err := s.db.Exec(`
        UPDATE short_links SET
            deploy_status = $2,
            deployed_url = COALESCE($3, deployed_url),
            deploy_error = $4,
            updated_at = NOW()
        WHERE id = $1
    `, id, status, url, errorMsg)
    return err
}

func stringJoin(strs []string, sep string) string {
    if len(strs) == 0 {
        return ""
    }
    result := strs[0]
    for _, s := range strs[1:] {
        result += sep + s
    }
    return result
}
```

- [ ] **Step 2: Verify build**

```bash
go build ./modules/shortener/services/
```

- [ ] **Step 3: Commit**

```bash
git add modules/shortener/services/shortener_service.go
git commit -m "feat(shortener): add service layer"
```

---

### Task 5: Create Handlers

**Files:**
- Create: `modules/shortener/handlers/handler.go`

**Interfaces:**
- Consumes: Service, TemplateEngine, DomainProvider
- Produces: HTTP handlers for pages and API

- [ ] **Step 1: Write handlers**

```go
// modules/shortener/handlers/handler.go
package handlers

import (
    "crypto/rand"
    "encoding/hex"
    "encoding/json"
    "net/http"

    domainmodels "github.com/botginx/botginx/modules/domains/models"
    "github.com/botginx/botginx/modules/shortener/models"
    "github.com/botginx/botginx/modules/shortener/services"
    "github.com/botginx/botginx/pkg/ctx"
    "github.com/botginx/botginx/pkg/module"
    "github.com/botginx/botginx/pkg/protection"
    "github.com/go-chi/chi/v5"
    "github.com/skip2/go-qrcode"
)

type DomainProvider interface {
    ListAvailable(userID string) ([]domainmodels.Domain, error)
}

type Handler struct {
    service   *services.ShortenerService
    templates *module.TemplateEngine
    domains   DomainProvider
}

func NewHandler(service *services.ShortenerService, templates *module.TemplateEngine, domains DomainProvider) *Handler {
    return &Handler{
        service:   service,
        templates: templates,
        domains:   domains,
    }
}

// Page Handlers

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
    userID := ctx.GetUserID(r)
    links, _ := h.service.List(userID)

    module.RenderUserSection(w, r, h.templates, "shortener:list.html", map[string]interface{}{
        "Title": "My Short Links",
        "Links": links,
    })
}

func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
    userID := ctx.GetUserID(r)
    domains, _ := h.domains.ListAvailable(userID)

    module.RenderUserSection(w, r, h.templates, "shortener:new.html", map[string]interface{}{
        "Title":             "Create Short Link",
        "Domains":           domains,
        "DefaultProtection": protection.GetDefaultSettings(),
        "BotErrors":         getBotErrors(),
        "Countries":         protection.GetCountries(),
    })
}

func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    link, err := h.service.Get(id)
    if err != nil {
        http.Error(w, "Link not found", http.StatusNotFound)
        return
    }

    // Verify ownership
    userID := ctx.GetUserID(r)
    if link.UserID != userID {
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }

    stats, _ := h.service.GetStats(id, 30)

    module.RenderUserSection(w, r, h.templates, "shortener:show.html", map[string]interface{}{
        "Title":     "Short Link Details",
        "Link":      link,
        "Stats":     stats,
        "BotErrors": getBotErrors(),
        "Countries": protection.GetCountries(),
    })
}

func (h *Handler) QRCode(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    link, err := h.service.Get(id)
    if err != nil {
        http.Error(w, "Link not found", http.StatusNotFound)
        return
    }

    size := 256
    if s := r.URL.Query().Get("size"); s == "512" {
        size = 512
    }

    png, err := qrcode.Encode(link.FullURL(), qrcode.Medium, size)
    if err != nil {
        http.Error(w, "Failed to generate QR code", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "image/png")
    w.Header().Set("Cache-Control", "public, max-age=86400")
    w.Write(png)
}

// API Handlers

func (h *Handler) APIList(w http.ResponseWriter, r *http.Request) {
    userID := ctx.GetUserID(r)
    links, err := h.service.List(userID)
    if err != nil {
        module.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
        return
    }
    json.NewEncoder(w).Encode(links)
}

func (h *Handler) APIGet(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    link, err := h.service.Get(id)
    if err != nil {
        module.JSON(w, http.StatusNotFound, map[string]string{"error": "Link not found"})
        return
    }

    userID := ctx.GetUserID(r)
    if link.UserID != userID {
        module.JSON(w, http.StatusForbidden, map[string]string{"error": "Forbidden"})
        return
    }

    json.NewEncoder(w).Encode(link)
}

func (h *Handler) APICreate(w http.ResponseWriter, r *http.Request) {
    userID := ctx.GetUserID(r)

    var input models.CreateShortLinkInput
    if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
        module.JSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
        return
    }

    // Validate
    if input.DomainID == "" {
        module.JSON(w, http.StatusBadRequest, map[string]string{"error": "Domain is required"})
        return
    }
    if input.Path == "" {
        module.JSON(w, http.StatusBadRequest, map[string]string{"error": "Path is required"})
        return
    }
    if len(input.Destinations) == 0 {
        module.JSON(w, http.StatusBadRequest, map[string]string{"error": "At least one destination URL is required"})
        return
    }

    // Check path availability
    available, _ := h.service.CheckPathAvailable(input.DomainID, input.Path)
    if !available {
        module.JSON(w, http.StatusConflict, map[string]string{"error": "Path already in use"})
        return
    }

    link, err := h.service.Create(userID, input)
    if err != nil {
        module.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
        return
    }

    // TODO: Deploy to VPS
    // go h.deploy(link.ID)

    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(link)
}

func (h *Handler) APIUpdate(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    
    // Verify ownership
    link, err := h.service.Get(id)
    if err != nil {
        module.JSON(w, http.StatusNotFound, map[string]string{"error": "Link not found"})
        return
    }
    userID := ctx.GetUserID(r)
    if link.UserID != userID {
        module.JSON(w, http.StatusForbidden, map[string]string{"error": "Forbidden"})
        return
    }

    var input models.UpdateShortLinkInput
    if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
        module.JSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
        return
    }

    updated, err := h.service.Update(id, input)
    if err != nil {
        module.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
        return
    }

    // TODO: Redeploy
    // go h.deploy(id)

    json.NewEncoder(w).Encode(updated)
}

func (h *Handler) APIDelete(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")

    // Verify ownership
    link, err := h.service.Get(id)
    if err != nil {
        module.JSON(w, http.StatusNotFound, map[string]string{"error": "Link not found"})
        return
    }
    userID := ctx.GetUserID(r)
    if link.UserID != userID {
        module.JSON(w, http.StatusForbidden, map[string]string{"error": "Forbidden"})
        return
    }

    if err := h.service.Delete(id); err != nil {
        module.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
        return
    }

    // TODO: Remove from VPS

    module.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) APIRandomPath(w http.ResponseWriter, r *http.Request) {
    path := generateRandomPath(6)
    module.JSON(w, http.StatusOK, map[string]string{"path": path})
}

func (h *Handler) APIStats(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")

    // Verify ownership
    link, err := h.service.Get(id)
    if err != nil {
        module.JSON(w, http.StatusNotFound, map[string]string{"error": "Link not found"})
        return
    }
    userID := ctx.GetUserID(r)
    if link.UserID != userID {
        module.JSON(w, http.StatusForbidden, map[string]string{"error": "Forbidden"})
        return
    }

    stats, err := h.service.GetStats(id, 30)
    if err != nil {
        module.JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
        return
    }

    json.NewEncoder(w).Encode(stats)
}

func (h *Handler) APICheckPath(w http.ResponseWriter, r *http.Request) {
    domainID := r.URL.Query().Get("domainId")
    path := r.URL.Query().Get("path")

    if domainID == "" || path == "" {
        module.JSON(w, http.StatusBadRequest, map[string]string{"error": "domainId and path required"})
        return
    }

    available, _ := h.service.CheckPathAvailable(domainID, path)
    module.JSON(w, http.StatusOK, map[string]bool{"available": available})
}

// Helpers

func generateRandomPath(length int) string {
    const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
    b := make([]byte, length)
    rand.Read(b)
    for i := range b {
        b[i] = charset[int(b[i])%len(charset)]
    }
    return string(b)
}

func getBotErrors() []map[string]interface{} {
    return []map[string]interface{}{
        {"code": 400, "name": "Bad Request"},
        {"code": 401, "name": "Unauthorized"},
        {"code": 403, "name": "Forbidden"},
        {"code": 404, "name": "Not Found"},
        {"code": 405, "name": "Method Not Allowed"},
        {"code": 408, "name": "Request Timeout"},
        {"code": 410, "name": "Gone"},
        {"code": 429, "name": "Too Many Requests"},
        {"code": 500, "name": "Internal Server Error"},
        {"code": 502, "name": "Bad Gateway"},
        {"code": 503, "name": "Service Unavailable"},
    }
}
```

- [ ] **Step 2: Add go-qrcode dependency**

```bash
go get github.com/skip2/go-qrcode
```

- [ ] **Step 3: Verify build**

```bash
go build ./modules/shortener/handlers/
```

- [ ] **Step 4: Commit**

```bash
git add modules/shortener/handlers/handler.go go.mod go.sum
git commit -m "feat(shortener): add HTTP handlers"
```

---

### Task 6: Create Module Registration

**Files:**
- Create: `modules/shortener/module.go`

**Interfaces:**
- Consumes: Dependencies
- Produces: Registered module

- [ ] **Step 1: Write module.go**

```go
// modules/shortener/module.go
package shortener

import (
    "embed"
    "io/fs"

    domainmodels "github.com/botginx/botginx/modules/domains/models"
    "github.com/botginx/botginx/modules/shortener/handlers"
    "github.com/botginx/botginx/modules/shortener/services"
    "github.com/botginx/botginx/pkg/module"
    "github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed migrations/*.sql
var migrationsFS embed.FS

type DomainProvider interface {
    ListAvailable(userID string) ([]domainmodels.Domain, error)
}

type Module struct {
    *module.BaseModule
    service *services.ShortenerService
    handler *handlers.Handler
    domains DomainProvider
}

func New(domains DomainProvider) *Module {
    return &Module{
        BaseModule: module.NewBaseModule(
            "shortener",
            "Link Shortener",
            "Create short links with bot protection",
        ),
        domains: domains,
    }
}

func (m *Module) Init(deps *module.Dependencies) error {
    m.SetDeps(deps)

    m.service = services.NewShortenerService(deps.DB)
    m.handler = handlers.NewHandler(m.service, deps.Templates, m.domains)

    tmplFS, _ := fs.Sub(templatesFS, "templates")
    deps.Templates.RegisterModule(m.ID(), tmplFS)

    return nil
}

func (m *Module) Migrate() error {
    sql, err := fs.ReadFile(migrationsFS, "migrations/001_create_tables.sql")
    if err != nil {
        return err
    }
    _, err = m.DB().Exec(string(sql))
    return err
}

func (m *Module) Routes() chi.Router {
    r := chi.NewRouter()

    // Pages
    r.Get("/", m.handler.List)
    r.Get("/new", m.handler.New)
    r.Get("/{id}", m.handler.Show)
    r.Get("/{id}/qr", m.handler.QRCode)

    // API
    r.Route("/api", func(r chi.Router) {
        r.Get("/", m.handler.APIList)
        r.Post("/", m.handler.APICreate)
        r.Get("/random-path", m.handler.APIRandomPath)
        r.Get("/check-path", m.handler.APICheckPath)
        r.Get("/{id}", m.handler.APIGet)
        r.Put("/{id}", m.handler.APIUpdate)
        r.Delete("/{id}", m.handler.APIDelete)
        r.Get("/{id}/stats", m.handler.APIStats)
    })

    return r
}

func (m *Module) Templates() fs.FS {
    tmplFS, _ := fs.Sub(templatesFS, "templates")
    return tmplFS
}

func (m *Module) MenuItems() []module.MenuItem {
    return []module.MenuItem{
        {
            Title:   "Link Shortener",
            Icon:    "bi-link-45deg",
            Path:    "/user/shortener",
            Order:   15,
            Section: module.MenuSectionUser,
        },
    }
}

func (m *Module) Widgets() []module.Widget {
    return nil
}
```

- [ ] **Step 2: Commit**

```bash
git add modules/shortener/module.go
git commit -m "feat(shortener): add module registration"
```

---

### Task 7: Create List Template

**Files:**
- Create: `modules/shortener/templates/list.html`

**Interfaces:**
- Consumes: `.Links` array
- Produces: Short links list page

- [ ] **Step 1: Write list.html**

```html
{{define "content"}}
<div class="d-flex justify-content-between align-items-center mb-3">
    <h4 class="mb-0"><i class="bi bi-link-45deg me-2"></i>My Short Links</h4>
    <a href="/user/shortener/new" class="btn btn-primary">
        <i class="bi bi-plus-lg me-1"></i>Create Short Link
    </a>
</div>

<div class="card">
    <div class="card-body">
        <table class="table table-hover" id="links-table">
            <thead>
                <tr>
                    <th>Short Link</th>
                    <th>Destination(s)</th>
                    <th>Clicks</th>
                    <th>Bot Error</th>
                    <th>QR</th>
                    <th>Status</th>
                    <th>Created</th>
                    <th></th>
                </tr>
            </thead>
            <tbody>
                {{range .Links}}
                <tr>
                    <td>
                        <div class="d-flex align-items-center gap-2">
                            <a href="https://{{.DomainName}}/{{.Path}}" target="_blank" class="text-primary text-decoration-none">
                                {{.DomainName}}/{{.Path}}
                            </a>
                            <button class="btn btn-sm btn-link p-0" onclick="copyToClipboard('https://{{.DomainName}}/{{.Path}}')">
                                <i class="bi bi-clipboard"></i>
                            </button>
                        </div>
                    </td>
                    <td>
                        {{if gt (len .Destinations) 1}}
                        <span class="badge bg-info">{{len .Destinations}} URLs</span>
                        {{else if gt (len .Destinations) 0}}
                        <span class="text-truncate d-inline-block" style="max-width: 180px;" title="{{index .Destinations 0}}">
                            {{index .Destinations 0}}
                        </span>
                        {{else}}
                        <span class="text-muted">-</span>
                        {{end}}
                    </td>
                    <td>
                        <span class="text-success">{{.HumanCount}}</span>
                        <span class="text-muted">/</span>
                        <span class="text-danger">{{.BotCount}}</span>
                    </td>
                    <td><span class="badge bg-secondary">{{.BotError}}</span></td>
                    <td>
                        {{if .QREnabled}}
                        <a href="/user/shortener/{{.ID}}/qr" target="_blank" title="View QR Code">
                            <i class="bi bi-qr-code"></i>
                        </a>
                        {{else}}
                        <span class="text-muted">-</span>
                        {{end}}
                    </td>
                    <td>
                        {{if eq .DeployStatus "deployed"}}
                        <span class="badge bg-success">Live</span>
                        {{else if eq .DeployStatus "pending"}}
                        <span class="badge bg-warning">Pending</span>
                        {{else if eq .DeployStatus "error"}}
                        <span class="badge bg-danger" title="{{.DeployError}}">Error</span>
                        {{else}}
                        <span class="badge bg-secondary">{{.DeployStatus}}</span>
                        {{end}}
                    </td>
                    <td>{{.CreatedAt.Format "Jan 02, 2006"}}</td>
                    <td>
                        <div class="btn-group btn-group-sm">
                            <a href="/user/shortener/{{.ID}}" class="btn btn-outline-primary" title="View Details">
                                <i class="bi bi-eye"></i>
                            </a>
                            <button class="btn btn-outline-danger" onclick="deleteLink('{{.ID}}')" title="Delete">
                                <i class="bi bi-trash"></i>
                            </button>
                        </div>
                    </td>
                </tr>
                {{else}}
                <tr>
                    <td colspan="8" class="text-center text-muted py-5">
                        <i class="bi bi-link-45deg" style="font-size: 3rem; opacity: 0.3;"></i>
                        <div class="mt-2">No short links yet</div>
                        <a href="/user/shortener/new" class="btn btn-primary btn-sm mt-3">
                            <i class="bi bi-plus-lg me-1"></i>Create your first short link
                        </a>
                    </td>
                </tr>
                {{end}}
            </tbody>
        </table>
    </div>
</div>

<script>
function copyToClipboard(text) {
    navigator.clipboard.writeText(text).then(() => {
        showToast('Copied to clipboard', 'success');
    });
}

async function deleteLink(id) {
    if (!confirm('Delete this short link? This cannot be undone.')) return;
    
    const res = await fetch('/user/shortener/api/' + id, { method: 'DELETE' });
    if (res.ok) {
        showToast('Link deleted', 'success');
        location.reload();
    } else {
        const data = await res.json();
        showToast(data.error || 'Failed to delete', 'danger');
    }
}

// Initialize DataTable
document.addEventListener('DOMContentLoaded', () => {
    if (typeof simpleDatatables !== 'undefined' && document.querySelector('#links-table tbody tr td:not(.text-center)')) {
        new simpleDatatables.DataTable('#links-table', {
            searchable: true,
            perPage: 25
        });
    }
});
</script>
{{end}}
```

- [ ] **Step 2: Commit**

```bash
git add modules/shortener/templates/list.html
git commit -m "feat(shortener): add list template"
```

---

### Task 8: Create New Template

**Files:**
- Create: `modules/shortener/templates/new.html`

**Interfaces:**
- Consumes: `.Domains`, `.BotErrors`, `.DefaultProtection`, `.Countries`
- Produces: Create short link form

- [ ] **Step 1: Write new.html**

```html
{{define "content"}}
<div class="mb-3">
    <a href="/user/shortener" class="text-muted text-decoration-none">
        <i class="bi bi-arrow-left me-1"></i>Back to Short Links
    </a>
</div>

<div class="card">
    <div class="card-header">
        <h5 class="mb-0"><i class="bi bi-link-45deg me-2"></i>Create Short Link</h5>
    </div>
    <div class="card-body">
        <form id="create-form">
            <div class="row g-3">
                <!-- Domain -->
                <div class="col-md-4">
                    <label class="form-label">Domain</label>
                    <select class="form-select" id="domainId" required>
                        <option value="">Select domain...</option>
                        {{range .Domains}}
                        <option value="{{.ID}}">{{.Name}}</option>
                        {{end}}
                    </select>
                </div>

                <!-- Path -->
                <div class="col-md-3">
                    <label class="form-label">Path</label>
                    <div class="input-group">
                        <span class="input-group-text">/</span>
                        <input type="text" class="form-control" id="path" pattern="[a-zA-Z0-9\-_]+" required>
                        <button type="button" class="btn btn-outline-secondary" onclick="randomizePath()" title="Generate random">
                            <i class="bi bi-shuffle"></i>
                        </button>
                    </div>
                </div>

                <!-- Bot Error -->
                <div class="col-md-3">
                    <label class="form-label">Bot Response</label>
                    <select class="form-select" id="botError">
                        {{range .BotErrors}}
                        <option value="{{.code}}" {{if eq .code 403}}selected{{end}}>{{.code}} - {{.name}}</option>
                        {{end}}
                    </select>
                </div>

                <!-- QR -->
                <div class="col-md-2">
                    <label class="form-label">QR Code</label>
                    <div class="form-check form-switch mt-2">
                        <input class="form-check-input" type="checkbox" id="qrEnabled">
                        <label class="form-check-label" for="qrEnabled">Enable</label>
                    </div>
                </div>
            </div>

            <!-- Destinations -->
            <div class="mt-4">
                <label class="form-label">Destination URL(s)</label>
                <div id="destinations-container">
                    <div class="input-group mb-2">
                        <input type="url" class="form-control destination-input" placeholder="https://example.com" required>
                        <button type="button" class="btn btn-outline-success" onclick="addDestination()">
                            <i class="bi bi-plus-lg"></i>
                        </button>
                    </div>
                </div>
                <small class="text-muted">Add multiple URLs for random rotation</small>
            </div>

            <!-- Protection Settings (collapsible) -->
            <div class="mt-4">
                <div class="card border">
                    <div class="card-header bg-transparent d-flex justify-content-between align-items-center"
                         style="cursor: pointer;"
                         data-bs-toggle="collapse"
                         data-bs-target="#protection-collapse">
                        <span><i class="bi bi-shield-check me-2"></i>Protection Settings</span>
                        <i class="bi bi-chevron-down" id="protection-chevron"></i>
                    </div>
                    <div class="collapse" id="protection-collapse">
                        <div class="card-body">
                            {{template "protection-settings" .}}
                        </div>
                    </div>
                </div>
            </div>

            <div class="mt-4">
                <button type="submit" class="btn btn-primary" id="submit-btn">
                    <i class="bi bi-check-lg me-1"></i>Create Short Link
                </button>
            </div>
        </form>
    </div>
</div>

<script>
// Generate random path on load
document.addEventListener('DOMContentLoaded', () => {
    randomizePath();
    
    // Initialize protection with defaults
    const defaults = {{.DefaultProtection | json}};
    if (window.protectionSettings) {
        window.protectionSettings.init(defaults);
    }
    
    // Toggle chevron
    document.getElementById('protection-collapse').addEventListener('show.bs.collapse', () => {
        document.getElementById('protection-chevron').classList.replace('bi-chevron-down', 'bi-chevron-up');
    });
    document.getElementById('protection-collapse').addEventListener('hide.bs.collapse', () => {
        document.getElementById('protection-chevron').classList.replace('bi-chevron-up', 'bi-chevron-down');
    });
});

async function randomizePath() {
    const res = await fetch('/user/shortener/api/random-path');
    const data = await res.json();
    document.getElementById('path').value = data.path;
}

function addDestination() {
    const container = document.getElementById('destinations-container');
    const div = document.createElement('div');
    div.className = 'input-group mb-2';
    div.innerHTML = `
        <input type="url" class="form-control destination-input" placeholder="https://example.com">
        <button type="button" class="btn btn-outline-danger" onclick="this.parentElement.remove()">
            <i class="bi bi-trash"></i>
        </button>
    `;
    container.appendChild(div);
}

document.getElementById('create-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    
    const btn = document.getElementById('submit-btn');
    const originalHtml = btn.innerHTML;
    btn.disabled = true;
    btn.innerHTML = '<span class="spinner-border spinner-border-sm me-1"></span>Creating...';
    
    try {
        const destinations = Array.from(document.querySelectorAll('.destination-input'))
            .map(i => i.value.trim())
            .filter(v => v);
        
        if (destinations.length === 0) {
            showToast('At least one destination URL is required', 'warning');
            return;
        }
        
        const data = {
            domainId: document.getElementById('domainId').value,
            path: document.getElementById('path').value,
            destinations: destinations,
            rotationMode: 'random',
            botError: parseInt(document.getElementById('botError').value),
            qrEnabled: document.getElementById('qrEnabled').checked,
            protectionSettings: window.protectionSettings ? window.protectionSettings.getSettings() : {}
        };
        
        const res = await fetch('/user/shortener/api/', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data)
        });
        
        if (res.ok) {
            const link = await res.json();
            showToast('Short link created!', 'success');
            window.location.href = '/user/shortener/' + link.id;
        } else {
            const err = await res.json();
            showToast(err.error || 'Failed to create', 'danger');
        }
    } finally {
        btn.disabled = false;
        btn.innerHTML = originalHtml;
    }
});
</script>
{{end}}
```

- [ ] **Step 2: Commit**

```bash
git add modules/shortener/templates/new.html
git commit -m "feat(shortener): add new template with protection component"
```

---

### Task 9: Create Show Template

**Files:**
- Create: `modules/shortener/templates/show.html`

**Interfaces:**
- Consumes: `.Link`, `.Stats`, `.BotErrors`, `.Countries`
- Produces: Link details page with stats and settings

- [ ] **Step 1: Write show.html**

```html
{{define "content"}}
<div class="d-flex justify-content-between align-items-center mb-3">
    <div>
        <a href="/user/shortener" class="text-muted text-decoration-none">
            <i class="bi bi-arrow-left me-1"></i>Back to Short Links
        </a>
    </div>
    <div class="d-flex gap-2">
        {{if .Link.QREnabled}}
        <a href="/user/shortener/{{.Link.ID}}/qr" target="_blank" class="btn btn-outline-secondary btn-sm">
            <i class="bi bi-qr-code me-1"></i>QR Code
        </a>
        {{end}}
        <button class="btn btn-outline-primary btn-sm" onclick="copyToClipboard('{{.Link.FullURL}}')">
            <i class="bi bi-clipboard me-1"></i>Copy Link
        </button>
    </div>
</div>

<!-- Link Info Card -->
<div class="card mb-3">
    <div class="card-body">
        <div class="row align-items-center">
            <div class="col-md-8">
                <h4 class="mb-1">
                    <a href="{{.Link.FullURL}}" target="_blank" class="text-decoration-none">
                        {{.Link.FullURL}}
                        <i class="bi bi-box-arrow-up-right ms-1" style="font-size: 0.8em;"></i>
                    </a>
                </h4>
                <div class="text-muted">
                    Created {{.Link.CreatedAt.Format "January 02, 2006 at 3:04 PM"}}
                </div>
            </div>
            <div class="col-md-4 text-md-end mt-3 mt-md-0">
                {{if eq .Link.DeployStatus "deployed"}}
                <span class="badge bg-success"><i class="bi bi-check-circle me-1"></i>Live</span>
                {{else if eq .Link.DeployStatus "pending"}}
                <span class="badge bg-warning"><i class="bi bi-clock me-1"></i>Deploying...</span>
                {{else if eq .Link.DeployStatus "error"}}
                <span class="badge bg-danger" title="{{.Link.DeployError}}"><i class="bi bi-x-circle me-1"></i>Error</span>
                {{end}}
            </div>
        </div>
    </div>
</div>

<!-- Stats Cards -->
<div class="row g-3 mb-3">
    <div class="col-md-4">
        <div class="card h-100">
            <div class="card-body text-center">
                <div class="text-muted small text-uppercase mb-1">Total Clicks</div>
                <div class="fs-2 fw-bold">{{.Link.ClickCount}}</div>
            </div>
        </div>
    </div>
    <div class="col-md-4">
        <div class="card h-100">
            <div class="card-body text-center">
                <div class="text-muted small text-uppercase mb-1">Human Visits</div>
                <div class="fs-2 fw-bold text-success">{{.Link.HumanCount}}</div>
            </div>
        </div>
    </div>
    <div class="col-md-4">
        <div class="card h-100">
            <div class="card-body text-center">
                <div class="text-muted small text-uppercase mb-1">Blocked Bots</div>
                <div class="fs-2 fw-bold text-danger">{{.Link.BotCount}}</div>
            </div>
        </div>
    </div>
</div>

<!-- Destinations -->
<div class="card mb-3">
    <div class="card-header">
        <h6 class="mb-0"><i class="bi bi-signpost-split me-2"></i>Destinations</h6>
    </div>
    <div class="card-body">
        {{if gt (len .Link.Destinations) 1}}
        <div class="alert alert-info py-2 mb-3">
            <i class="bi bi-shuffle me-1"></i>
            <strong>Rotation enabled:</strong> Traffic is distributed randomly across {{len .Link.Destinations}} URLs
        </div>
        {{end}}
        <ul class="list-group list-group-flush">
            {{range .Link.Destinations}}
            <li class="list-group-item d-flex justify-content-between align-items-center">
                <a href="{{.}}" target="_blank" class="text-truncate" style="max-width: 80%;">{{.}}</a>
                <i class="bi bi-box-arrow-up-right text-muted"></i>
            </li>
            {{end}}
        </ul>
    </div>
</div>

<!-- Settings -->
<div class="card mb-3">
    <div class="card-header d-flex justify-content-between align-items-center">
        <h6 class="mb-0"><i class="bi bi-gear me-2"></i>Settings</h6>
        <button class="btn btn-sm btn-outline-primary" onclick="toggleEditMode()">
            <i class="bi bi-pencil me-1"></i>Edit
        </button>
    </div>
    <div class="card-body">
        <div id="settings-view">
            <div class="row g-3">
                <div class="col-md-4">
                    <label class="text-muted small text-uppercase">Bot Response</label>
                    <div>{{.Link.BotError}} Error</div>
                </div>
                <div class="col-md-4">
                    <label class="text-muted small text-uppercase">QR Code</label>
                    <div>{{if .Link.QREnabled}}Enabled{{else}}Disabled{{end}}</div>
                </div>
                <div class="col-md-4">
                    <label class="text-muted small text-uppercase">Rotation</label>
                    <div>{{.Link.RotationMode}}</div>
                </div>
            </div>
        </div>
        
        <div id="settings-edit" style="display: none;">
            <form id="settings-form">
                <div class="row g-3">
                    <div class="col-md-4">
                        <label class="form-label">Bot Response</label>
                        <select class="form-select" id="edit-botError">
                            {{range $.BotErrors}}
                            <option value="{{.code}}" {{if eq .code $.Link.BotError}}selected{{end}}>{{.code}} - {{.name}}</option>
                            {{end}}
                        </select>
                    </div>
                    <div class="col-md-4">
                        <label class="form-label">QR Code</label>
                        <div class="form-check form-switch mt-2">
                            <input class="form-check-input" type="checkbox" id="edit-qrEnabled" {{if .Link.QREnabled}}checked{{end}}>
                            <label class="form-check-label" for="edit-qrEnabled">Enable</label>
                        </div>
                    </div>
                </div>
                
                <div class="mt-4">
                    <h6><i class="bi bi-shield-check me-2"></i>Protection Settings</h6>
                    {{template "protection-settings" .}}
                </div>
                
                <div class="mt-4">
                    <button type="submit" class="btn btn-primary" id="save-btn">
                        <i class="bi bi-check-lg me-1"></i>Save Changes
                    </button>
                    <button type="button" class="btn btn-outline-secondary" onclick="toggleEditMode()">Cancel</button>
                </div>
            </form>
        </div>
    </div>
</div>

<script>
function copyToClipboard(text) {
    navigator.clipboard.writeText(text).then(() => {
        showToast('Copied to clipboard', 'success');
    });
}

let editMode = false;
function toggleEditMode() {
    editMode = !editMode;
    document.getElementById('settings-view').style.display = editMode ? 'none' : 'block';
    document.getElementById('settings-edit').style.display = editMode ? 'block' : 'none';
}

// Initialize protection settings
document.addEventListener('DOMContentLoaded', () => {
    const settings = {{.Link.ProtectionSettings | json}};
    if (window.protectionSettings) {
        window.protectionSettings.init(settings);
    }
});

document.getElementById('settings-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    
    const btn = document.getElementById('save-btn');
    const originalHtml = btn.innerHTML;
    btn.disabled = true;
    btn.innerHTML = '<span class="spinner-border spinner-border-sm me-1"></span>Saving...';
    
    try {
        const data = {
            botError: parseInt(document.getElementById('edit-botError').value),
            qrEnabled: document.getElementById('edit-qrEnabled').checked,
            protectionSettings: window.protectionSettings ? window.protectionSettings.getSettings() : {}
        };
        
        const res = await fetch('/user/shortener/api/{{.Link.ID}}', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data)
        });
        
        if (res.ok) {
            showToast('Settings saved', 'success');
            location.reload();
        } else {
            const err = await res.json();
            showToast(err.error || 'Failed to save', 'danger');
        }
    } finally {
        btn.disabled = false;
        btn.innerHTML = originalHtml;
    }
});
</script>
{{end}}
```

- [ ] **Step 2: Commit**

```bash
git add modules/shortener/templates/show.html
git commit -m "feat(shortener): add show template with stats and edit"
```

---

### Task 10: Register Module in Main

**Files:**
- Modify: `cmd/server/main.go`

**Interfaces:**
- Consumes: Module
- Produces: Registered routes

- [ ] **Step 1: Import shortener module**

```go
import "github.com/botginx/botginx/modules/shortener"
```

- [ ] **Step 2: Create and register module**

Find where other modules are registered and add:

```go
shortenerMod := shortener.New(domainsMod)
modulesMgr.Register(shortenerMod)
```

- [ ] **Step 3: Add user routes**

```go
r.Route("/user/shortener", func(r chi.Router) {
    r.Use(authMiddleware)
    r.Mount("/", shortenerMod.Routes())
})
```

- [ ] **Step 4: Run migration**

The module's Migrate() will be called automatically on startup.

- [ ] **Step 5: Verify build and run**

```bash
go build ./cmd/server
./botginx
```

- [ ] **Step 6: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat(shortener): register module in main"
```

---

### Task 11: Integration Testing

**Files:**
- None (manual testing)

- [ ] **Step 1: Start server**

```bash
go run ./cmd/server
```

- [ ] **Step 2: Test list page**

Navigate to `/user/shortener` - should show empty state

- [ ] **Step 3: Test create flow**

Navigate to `/user/shortener/new`:
- Select domain
- Verify random path generates
- Add destination URL
- Toggle QR code
- Expand protection settings
- Submit form

- [ ] **Step 4: Test show page**

After creating, verify:
- Stats cards show zeros
- Destinations list correct
- Edit mode toggles
- Settings save correctly

- [ ] **Step 5: Test QR code**

If QR enabled, click QR icon and verify image renders

- [ ] **Step 6: Test delete**

Delete a link and verify it disappears

- [ ] **Step 7: Final commit**

```bash
git add -A
git commit -m "feat(shortener): complete link shortener module"
```
