# Server Auto-Provisioning Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** One-click VPS provisioning with real-time SSE progress streaming from `/admin/servers/new`

**Architecture:** When user clicks "Add & Provision", create server record, start background goroutine that runs deploy scripts via SSH, stream output to UI via Server-Sent Events. Each provisioning step updates database state for resume/retry.

**Tech Stack:** Go, SSE (text/event-stream), golang.org/x/crypto/ssh (existing pkg/ssh), AdminLTE templates

**Spec:** `docs/superpowers/specs/2026-11-20-server-auto-provisioning-design.md`

## Global Constraints

- Bootstrap Icons only (bi-*), never FontAwesome
- AdminLTE v4.9.1 components
- Existing pkg/ssh for SSH operations
- Scripts: `deploy-vps-setup.sh` (in repo), antibot `packaging/deploy.sh` (cloned to target)
- ACME_DOMAIN is only required user input (GitHub token already in antibot deploy.sh)
- SSH credentials never in logs or API responses

## Review Focus

1. **SSE connection drops mid-provision** — UI should reconnect and show current state from DB, not reset
2. **Script fails at step 3** — Provision marked "failed" with step number, retry button resumes from that step
3. **SSH timeout during long apt-get** — Command timeout must be long enough (10+ minutes for base setup)
4. **Browser navigates away during provision** — Provision continues in background, status visible on return
5. **Concurrent provisions on same server** — Should be blocked, only one active provision per server

---

### Task 1: Database Migration

**Files:**
- Create: `modules/servers/migrations/002_add_provisions.sql`

**Interfaces:**
- Consumes: Nothing
- Produces: `server_provisions` table, `provision_id` column on `servers`

- [ ] **Step 1: Create migration file**

```sql
-- modules/servers/migrations/002_add_provisions.sql

-- Provisioning jobs track multi-step server setup
CREATE TABLE IF NOT EXISTS server_provisions (
    id              VARCHAR(36) PRIMARY KEY,
    server_id       VARCHAR(36) NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    current_step    INT DEFAULT 0,
    total_steps     INT DEFAULT 5,
    step_name       VARCHAR(100),
    output          TEXT,
    error           TEXT,
    acme_domain     VARCHAR(255),
    started_at      TIMESTAMP,
    completed_at    TIMESTAMP,
    created_at      TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_server_provisions_server_id ON server_provisions(server_id);
CREATE INDEX IF NOT EXISTS idx_server_provisions_status ON server_provisions(status);

-- Add provision_id to servers for tracking current/last provision
ALTER TABLE servers ADD COLUMN IF NOT EXISTS provision_id VARCHAR(36);
```

- [ ] **Step 2: Update module.go Migrate() to run new migration**

Add after existing migration execution in `modules/servers/module.go`:

```go
// In Migrate() function, after first migration
sql2, err := fs.ReadFile(migrationsFS, "migrations/002_add_provisions.sql")
if err == nil {
    m.DB().Exec(string(sql2))
}
```

- [ ] **Step 3: Run migration locally to verify**

Run: `go run ./cmd/server` and check logs for migration errors
Expected: No errors, table created

- [ ] **Step 4: Commit**

```bash
git add modules/servers/migrations/002_add_provisions.sql modules/servers/module.go
git commit -m "feat(servers): add provisions table migration"
```

---

### Task 2: Provision Model

**Files:**
- Create: `modules/servers/models/provision.go`

**Interfaces:**
- Consumes: Nothing
- Produces: `Provision` struct, `ProvisionStatus` constants, `CreateProvisionInput` struct

- [ ] **Step 1: Create provision model**

```go
// modules/servers/models/provision.go
package models

import "time"

type ProvisionStatus string

const (
    ProvisionStatusPending   ProvisionStatus = "pending"
    ProvisionStatusRunning   ProvisionStatus = "running"
    ProvisionStatusCompleted ProvisionStatus = "completed"
    ProvisionStatusFailed    ProvisionStatus = "failed"
)

type Provision struct {
    ID          string          `db:"id" json:"id"`
    ServerID    string          `db:"server_id" json:"serverId"`
    Status      ProvisionStatus `db:"status" json:"status"`
    CurrentStep int             `db:"current_step" json:"currentStep"`
    TotalSteps  int             `db:"total_steps" json:"totalSteps"`
    StepName    string          `db:"step_name" json:"stepName"`
    Output      string          `db:"output" json:"output,omitempty"`
    Error       string          `db:"error" json:"error,omitempty"`
    AcmeDomain  string          `db:"acme_domain" json:"acmeDomain"`
    StartedAt   *time.Time      `db:"started_at" json:"startedAt,omitempty"`
    CompletedAt *time.Time      `db:"completed_at" json:"completedAt,omitempty"`
    CreatedAt   time.Time       `db:"created_at" json:"createdAt"`
}

type CreateProvisionInput struct {
    ServerID   string `json:"serverId" validate:"required"`
    AcmeDomain string `json:"acmeDomain" validate:"required"`
}

// ProvisionStep defines a provisioning step
type ProvisionStep struct {
    Number int
    Name   string
}

var ProvisionSteps = []ProvisionStep{
    {1, "Connecting via SSH"},
    {2, "Installing base packages"},
    {3, "Cloning antibot repository"},
    {4, "Installing botection"},
    {5, "Verifying services"},
}
```

- [ ] **Step 2: Verify model compiles**

Run: `go build ./modules/servers/...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add modules/servers/models/provision.go
git commit -m "feat(servers): add provision model"
```

---

### Task 3: Provision Service

**Files:**
- Create: `modules/servers/services/provision_service.go`

**Interfaces:**
- Consumes: `models.Provision`, `models.CreateProvisionInput`, `ServerService.GetSSHClient`
- Produces: `ProvisionService` with methods: `Create`, `Get`, `GetByServerID`, `UpdateStep`, `SetStatus`, `AppendOutput`, `RunProvision`

- [ ] **Step 1: Create provision service with CRUD operations**

Create `modules/servers/services/provision_service.go` with:
- `NewProvisionService(db *sqlx.DB, serverService *ServerService) *ProvisionService`
- `Create(input models.CreateProvisionInput) (*models.Provision, error)` - creates provision record
- `Get(id string) (*models.Provision, error)` - fetches by ID
- `GetByServerID(serverID string) (*models.Provision, error)` - fetches active provision for server
- `UpdateStep(id string, step int, name string) error` - updates current_step and step_name
- `SetStatus(id string, status models.ProvisionStatus) error` - updates status
- `SetError(id string, errorMsg string) error` - sets error field
- `AppendOutput(id string, line string) error` - appends to output TEXT field
- Helper: `generateID() string` using crypto/rand

- [ ] **Step 2: Verify CRUD compiles**

Run: `go build ./modules/servers/...`
Expected: No errors

- [ ] **Step 3: Add RunProvision method with step execution**

Add to provision_service.go:
- `RunProvision(ctx context.Context, provisionID string, outputChan chan<- string) error`
- Fetches provision and server
- Gets SSH client from serverService
- Executes steps 1-5 sequentially:
  - Step 1: Test SSH connection
  - Step 2: Run `ACME_DOMAIN=<domain> bash -s < deploy-vps-setup.sh` (read from embedded or file)
  - Step 3: Clone antibot repo via SSH command
  - Step 4: Run `cd /var/www/antibot && SERVER_TYPE=templates bash packaging/deploy.sh`
  - Step 5: Verify services with `systemctl is-active nginx redis-server botection`
- Sends output lines to outputChan
- Updates step progress in DB
- Sets status to completed/failed

- [ ] **Step 4: Embed deploy-vps-setup.sh script**

Add to provision_service.go:
```go
//go:embed scripts/deploy-vps-setup.sh
var deployVPSSetupScript string
```

Create symlink or copy script to `modules/servers/services/scripts/deploy-vps-setup.sh`

- [ ] **Step 5: Verify full service compiles**

Run: `go build ./modules/servers/...`
Expected: No errors

- [ ] **Step 6: Commit**

```bash
git add modules/servers/services/provision_service.go modules/servers/services/scripts/
git commit -m "feat(servers): add provision service with step execution"
```

---

### Task 4: Provision Handler with SSE Streaming

**Files:**
- Create: `modules/servers/handlers/provision_handler.go`

**Interfaces:**
- Consumes: `ProvisionService`, `models.CreateProvisionInput`
- Produces: `ProvisionHandler` with methods: `APIStartProvision`, `APIStreamProvision`, `ProvisionPage`

- [ ] **Step 1: Create provision handler with start endpoint**

Create `modules/servers/handlers/provision_handler.go` with:
- `NewProvisionHandler(provisionService *services.ProvisionService, templates *module.TemplateEngine) *ProvisionHandler`
- `APIStartProvision(w http.ResponseWriter, r *http.Request)` - POST, creates provision, starts goroutine, returns provisionID

- [ ] **Step 2: Add SSE streaming endpoint**

Add `APIStreamProvision(w http.ResponseWriter, r *http.Request)`:
- GET endpoint for `/api/provision/{id}/stream`
- Sets headers: `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`
- Creates output channel
- Calls `provisionService.RunProvision()` in goroutine if status is pending
- Loops reading from channel, writes SSE events:
  - `event: step\ndata: {"step": N, "total": 5, "name": "..."}\n\n`
  - `event: output\ndata: {"line": "..."}\n\n`
  - `event: error\ndata: {"message": "..."}\n\n`
  - `event: complete\ndata: {"success": true/false}\n\n`
- Flushes after each event
- Handles client disconnect via `r.Context().Done()`

- [ ] **Step 3: Add provision page handler**

Add `ProvisionPage(w http.ResponseWriter, r *http.Request)`:
- GET endpoint for `/provision/{id}`
- Fetches provision and server
- Renders `provision.html` template

- [ ] **Step 4: Verify handler compiles**

Run: `go build ./modules/servers/...`
Expected: No errors

- [ ] **Step 5: Commit**

```bash
git add modules/servers/handlers/provision_handler.go
git commit -m "feat(servers): add provision handler with SSE streaming"
```

---

### Task 5: Update Module Routes

**Files:**
- Modify: `modules/servers/module.go`

**Interfaces:**
- Consumes: `ProvisionService`, `ProvisionHandler`
- Produces: New routes registered

- [ ] **Step 1: Add provision service and handler to module**

In `module.go`:
- Add `provisionService *services.ProvisionService` field to Module struct
- Add `provisionHandler *handlers.ProvisionHandler` field
- In `Init()`, create `ProvisionService` and `ProvisionHandler`

- [ ] **Step 2: Register provision routes**

In `Routes()` function, add:
```go
// Provision pages
r.Get("/provision/{id}", m.provisionHandler.ProvisionPage)

// Provision API
r.Route("/api/provision", func(r chi.Router) {
    r.Post("/", m.provisionHandler.APIStartProvision)
    r.Get("/{id}", m.provisionHandler.APIGetProvision)
    r.Get("/{id}/stream", m.provisionHandler.APIStreamProvision)
})
```

- [ ] **Step 3: Verify routes compile**

Run: `go build ./cmd/server`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add modules/servers/module.go
git commit -m "feat(servers): register provision routes"
```

---

### Task 6: Update new.html Template

**Files:**
- Modify: `modules/servers/templates/new.html`

**Interfaces:**
- Consumes: Existing form structure
- Produces: ACME domain field, "Add & Provision" button

- [ ] **Step 1: Add ACME Domain field after provider select**

Add before the submit buttons div:
```html
<div class="mb-3" id="acme-section">
    <label for="acmeDomain" class="form-label">ACME Domain (for wildcard SSL)</label>
    <input type="text" class="form-control" id="acmeDomain" name="acmeDomain" 
           placeholder="acme.pamach.xyz" value="{{.DefaultAcmeDomain}}">
    <small class="text-muted">Required for auto-provisioning. Leave empty to skip provisioning.</small>
</div>
```

- [ ] **Step 2: Add "Add & Provision" button**

Update buttons section:
```html
<div class="d-flex gap-2">
    <button type="submit" class="btn btn-primary" id="submit-btn" data-action="create">
        <span class="spinner-border spinner-border-sm d-none me-1"></span>
        <i class="bi bi-plus-lg me-1 btn-icon"></i>
        <span class="btn-text">Add Server</span>
    </button>
    <button type="button" class="btn btn-success" id="provision-btn" data-action="provision">
        <span class="spinner-border spinner-border-sm d-none me-1"></span>
        <i class="bi bi-rocket-takeoff me-1 btn-icon"></i>
        <span class="btn-text">Add & Provision</span>
    </button>
    <a href="/admin/servers" class="btn btn-secondary">Cancel</a>
</div>
```

- [ ] **Step 3: Update JavaScript for provision flow**

Update form submit handler to:
- Check which button was clicked (create vs provision)
- If provision: validate ACME domain is filled, create server, then call provision API, redirect to provision page
- Use separate click handlers for each button

- [ ] **Step 4: Test UI renders**

Run: `go run ./cmd/server` and navigate to `/admin/servers/new`
Expected: Form shows ACME domain field and both buttons

- [ ] **Step 5: Commit**

```bash
git add modules/servers/templates/new.html
git commit -m "feat(servers): add provision option to new server form"
```

---

### Task 7: Create provision.html Template

**Files:**
- Create: `modules/servers/templates/provision.html`

**Interfaces:**
- Consumes: `Provision` and `Server` from handler
- Produces: Real-time progress UI with SSE connection

- [ ] **Step 1: Create provision template with progress steps**

Create `modules/servers/templates/provision.html`:
- Card header with server name
- Step list showing 5 steps with status icons (✓ done, ⟳ running, pending)
- Output panel (pre/code block) for streaming logs
- Uses EventSource to connect to `/admin/servers/api/provision/{{.Provision.ID}}/stream`
- Handles step, output, error, complete events
- Auto-scrolls output panel
- Shows toast on completion/failure
- "Back to Server" button when complete

- [ ] **Step 2: Add CSS for step states**

Inline styles or classes for:
- `.step-done` - green checkmark
- `.step-running` - spinning indicator
- `.step-pending` - gray circle
- Output panel: dark background, monospace font, max-height with scroll

- [ ] **Step 3: Add JavaScript for SSE handling**

```javascript
const evtSource = new EventSource('/admin/servers/api/provision/{{.Provision.ID}}/stream');

evtSource.addEventListener('step', (e) => {
    const data = JSON.parse(e.data);
    updateStepUI(data.step, data.name);
});

evtSource.addEventListener('output', (e) => {
    const data = JSON.parse(e.data);
    appendOutput(data.line);
});

evtSource.addEventListener('complete', (e) => {
    const data = JSON.parse(e.data);
    evtSource.close();
    showComplete(data.success);
});

evtSource.addEventListener('error', (e) => {
    // Handle connection errors vs provision errors
});
```

- [ ] **Step 4: Test template renders**

Run: `go run ./cmd/server`
Expected: Template renders without errors (SSE won't connect yet without provision)

- [ ] **Step 5: Commit**

```bash
git add modules/servers/templates/provision.html
git commit -m "feat(servers): add provision progress template"
```

---

### Task 8: Integration and End-to-End Test

**Files:**
- Modify: `modules/servers/handlers/handler.go` (add DefaultAcmeDomain to New page data)

**Interfaces:**
- Consumes: All previous tasks
- Produces: Working end-to-end flow

- [ ] **Step 1: Pass DefaultAcmeDomain to new.html**

In `handler.go` `New()` function, add `DefaultAcmeDomain` from env:
```go
module.Render(w, r, h.templates, "servers:new.html", map[string]interface{}{
    "Title": "Add Server",
    "DefaultAcmeDomain": os.Getenv("DEFAULT_ACME_DOMAIN"),
})
```

- [ ] **Step 2: Test full flow locally**

1. Start server: `go run ./cmd/server`
2. Navigate to `/admin/servers/new`
3. Fill in test server details (use a real test VPS if available)
4. Enter ACME domain
5. Click "Add & Provision"
6. Verify redirect to provision page
7. Verify SSE stream connects
8. Verify steps update in real-time
9. Verify output appears in log panel

- [ ] **Step 3: Handle edge cases**

Test and fix:
- Provision page loaded after completion (show final state)
- SSE reconnection on network hiccup
- Very long output lines (truncate/wrap)
- Credential sanitization in output

- [ ] **Step 4: Commit**

```bash
git add modules/servers/handlers/handler.go
git commit -m "feat(servers): complete auto-provisioning integration"
```

- [ ] **Step 5: Tag release**

```bash
git tag -a v1.11.0 -m "feat: server auto-provisioning with SSE streaming"
git push origin main --tags
```

---

## File Summary

| File | Action | Purpose |
|------|--------|---------|
| `migrations/002_add_provisions.sql` | Create | Provision tracking table |
| `models/provision.go` | Create | Provision struct and constants |
| `services/provision_service.go` | Create | CRUD and step execution |
| `services/scripts/deploy-vps-setup.sh` | Create | Embedded setup script |
| `handlers/provision_handler.go` | Create | SSE streaming endpoint |
| `handlers/handler.go` | Modify | Add DefaultAcmeDomain |
| `templates/new.html` | Modify | Add provision option |
| `templates/provision.html` | Create | Progress UI |
| `module.go` | Modify | Register routes, init services |
