---
title: Creating Modules
description: Build custom modules for GuardBot
---

This guide explains how to create a Go module that integrates with GuardBot. Your module will be a separate Go package that implements the required interfaces.

## Module Interface

Every module must implement the `Module` interface:

```go
type Module interface {
    // Identity
    ID() string          // Unique identifier (e.g., "mymodule")
    Name() string        // Display name (e.g., "My Module")
    Description() string // Short description

    // Lifecycle
    Init(deps *Dependencies) error  // Called on startup
    Migrate() error                 // Run database migrations

    // HTTP
    Routes() chi.Router  // Return your HTTP routes

    // Optional
    Templates() fs.FS    // Embedded HTML templates
    MenuItems() []MenuItem  // Sidebar menu entries
    Widgets() []Widget      // Dashboard widgets
}
```

## Project Structure

```
mymodule/
├── module.go           # Main module file
├── handlers/
│   └── handler.go      # HTTP handlers
├── services/
│   └── service.go      # Business logic
├── models/
│   └── models.go       # Data structures
├── migrations/
│   └── 001_create_tables.sql
└── templates/
    ├── index.html
    └── show.html
```

## Basic Module Example

```go
package mymodule

import (
    "embed"
    "io/fs"

    "github.com/botginx/botginx/pkg/module"
    "github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Module struct {
    *module.BaseModule
    service *MyService
    handler *Handler
}

func New() *Module {
    return &Module{
        BaseModule: module.NewBaseModule(
            "mymodule",        // ID - used in URLs: /user/mymodule
            "My Module",       // Display name
            "Does something",  // Description
        ),
    }
}

func (m *Module) Init(deps *module.Dependencies) error {
    m.SetDeps(deps)

    // Initialize your services
    m.service = NewMyService(deps.DB)
    m.handler = NewHandler(m.service, deps.Templates)

    // Register templates
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

    // Page routes
    r.Get("/", m.handler.List)
    r.Get("/new", m.handler.New)
    r.Get("/{id}", m.handler.Show)

    // API routes
    r.Route("/api", func(r chi.Router) {
        r.Get("/", m.handler.APIList)
        r.Post("/", m.handler.APICreate)
        r.Delete("/{id}", m.handler.APIDelete)
    })

    return r
}

func (m *Module) MenuItems() []module.MenuItem {
    return []module.MenuItem{
        {
            Title:   "My Module",
            Icon:    "bi-box",
            Path:    "/user/mymodule",
            Section: module.MenuSectionUser,
            Group:   "Links",  // Groups: Links, Infrastructure, Billing, Help
            Order:   100,
        },
    }
}
```

## Dependencies

Your module receives these dependencies via `Init()`:

| Dependency | Type | Description |
|------------|------|-------------|
| `DB` | `*sqlx.DB` | PostgreSQL database connection |
| `Router` | `chi.Router` | HTTP router |
| `Templates` | `*TemplateEngine` | Template rendering engine |
| `Config` | `map[string]interface{}` | Configuration values |

## Database Migrations

Create SQL files in `migrations/` folder:

```sql
-- migrations/001_create_tables.sql
CREATE TABLE IF NOT EXISTS my_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    data JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_my_items_user ON my_items(user_id);
```

## Templates

Templates use Go's `html/template` with AdminLTE v4 styling:

```html
{{define "content"}}
<div class="my-module">
    <div class="d-flex justify-content-between mb-4">
        <h4>{{t "mymodule.title"}}</h4>
        <a href="/user/mymodule/new" class="btn btn-primary">
            <i class="bi bi-plus-lg"></i> Create
        </a>
    </div>

    <div class="card">
        <div class="card-body">
            {{range .Items}}
            <div class="item">{{.Name}}</div>
            {{else}}
            <p class="text-muted">No items yet</p>
            {{end}}
        </div>
    </div>
</div>
{{end}}
```

### Available Template Functions

| Function | Description |
|----------|-------------|
| `{{t "key"}}` | Translate a string |
| `{{appName}}` | Application name |
| `{{.User}}` | Current logged-in user |
| `{{.IsAdmin}}` | Check if user is admin |

## Handler Pattern

```go
package handlers

import (
    "net/http"
    "github.com/botginx/botginx/pkg/module"
    "github.com/botginx/botginx/pkg/ctx"
)

type Handler struct {
    service   *MyService
    templates *module.TemplateEngine
}

func NewHandler(svc *MyService, te *module.TemplateEngine) *Handler {
    return &Handler{service: svc, templates: te}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
    user := ctx.GetUser(r)
    if user == nil {
        http.Redirect(w, r, "/auth/login", http.StatusFound)
        return
    }

    items, err := h.service.ListByUser(user.ID)
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }

    module.RenderUserSection(w, r, h.templates, "mymodule:index.html", map[string]interface{}{
        "Title": "My Module",
        "Items": items,
    })
}
```

## Menu Sections & Groups

Modules appear in sidebar based on `Section` and `Group`:

**Sections:**
- `MenuSectionUser` - User dashboard (`/user/*`)
- `MenuSectionAdmin` - Admin panel (`/admin/*`)

**Groups (for user section):**
- `Links` - Link management features
- `Infrastructure` - Domains, servers
- `Billing` - Payments, subscriptions
- `Help` - Support, docs

## Registering Your Module

In `cmd/server/main.go`:

```go
import "github.com/yourname/mymodule"

// In main():
registry.Register(mymodule.New())
```

## Context Helpers

```go
import "github.com/botginx/botginx/pkg/ctx"

// Get current user
user := ctx.GetUser(r)  // Returns *ctx.User or nil

// User struct
type User struct {
    ID      string
    Email   string
    Name    string
    Role    string  // "user" or "admin"
    Balance float64
}

// Check admin
if user.IsAdmin() { ... }
```

## Best Practices

1. **Use UUIDs** for primary keys
2. **Always check user ownership** before returning data
3. **Use transactions** for multi-step operations
4. **Return JSON** from `/api/*` routes
5. **Use `module.RenderUserSection`** for user pages
6. **Add proper indexes** on foreign keys and query columns
