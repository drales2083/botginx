# Module Integration Guide

Internal reference for integrating third-party Go packages into {{PROJECT_NAME}}.

## Module Interface

Third-party packages must implement:

```go
type Module interface {
    ID() string              // URL path segment: /user/{id}
    Name() string            // Display name
    Description() string
    Init(deps *Dependencies) error
    Migrate() error
    Routes() chi.Router
    Templates() fs.FS        // Optional
    MenuItems() []MenuItem   // Optional
    Widgets() []Widget       // Optional
}
```

## Registration

In `cmd/server/main.go`:

```go
import "path/to/theirpackage"

// After other module registrations (~line 290-350):
registry.Register(theirpackage.New())
```

## Dependencies Available

Modules receive via `Init(deps *module.Dependencies)`:

| Field | Type | Description |
|-------|------|-------------|
| `deps.DB` | `*sqlx.DB` | PostgreSQL connection |
| `deps.Router` | `chi.Router` | HTTP router |
| `deps.Templates` | `*TemplateEngine` | Template renderer |
| `deps.Config` | `map[string]interface{}` | Config values |

## Template Registration

Module must register its templates in `Init()`:

```go
tmplFS, _ := fs.Sub(templatesFS, "templates")
deps.Templates.RegisterModule(m.ID(), tmplFS)
```

Templates render with:
```go
module.RenderUserSection(w, r, h.templates, "moduleid:template.html", data)
```

## Menu Items

```go
func (m *Module) MenuItems() []module.MenuItem {
    return []module.MenuItem{{
        Title:   "Menu Label",
        Icon:    "bi-icon-name",      // Bootstrap Icons
        Path:    "/user/moduleid",
        Section: module.MenuSectionUser,  // or MenuSectionAdmin
        Group:   "Links",             // Links, Infrastructure, Billing, Help
        Order:   100,
    }}
}
```

## Routes Mount Point

- User modules: `/user/{module.ID()}/...`
- Admin modules: `/admin/{module.ID()}/...`

## Database

- Use PostgreSQL
- Tables should have `user_id UUID REFERENCES users(id) ON DELETE CASCADE`
- Use `gen_random_uuid()` for primary keys
- Migrations run via `Migrate()` method

## Auth Context

```go
import "{{PROJECT_IMPORT}}/pkg/ctx"

user := ctx.GetUser(r)  // Returns *ctx.User or nil
user.ID                 // UUID string
user.Email
user.Name
user.Role               // "user" or "admin"
user.Balance            // float64
user.IsAdmin()          // bool
```

## Required Imports

```go
import (
    "{{PROJECT_IMPORT}}/pkg/module"
    "{{PROJECT_IMPORT}}/pkg/ctx"
    "github.com/go-chi/chi/v5"
    "github.com/jmoiron/sqlx"
)
```

## Checklist When Integrating

1. [ ] Import package in `cmd/server/main.go`
2. [ ] Register with `registry.Register(pkg.New())`
3. [ ] Verify `go mod tidy` resolves dependencies
4. [ ] Run migrations: module's `Migrate()` called on startup
5. [ ] Check menu appears in correct section/group
6. [ ] Test routes at `/user/{id}/` or `/admin/{id}/`
7. [ ] Verify templates render correctly
8. [ ] Test with both user and admin accounts

## File Structure Expected

```
theirpackage/
├── module.go           # New() + Module interface
├── handlers/
│   └── handler.go
├── services/
│   └── service.go
├── models/
│   └── models.go
├── migrations/
│   └── 001_*.sql
└── templates/
    └── *.html
```

## Common Issues

**Templates not found**: Check `RegisterModule()` called with correct ID

**Routes 404**: Verify `Routes()` returns valid chi.Router

**Menu not showing**: Check `MenuItems()` returns items with correct Section

**DB errors**: Ensure migrations ran, check table/column names match

**Auth issues**: Always check `ctx.GetUser(r) != nil` before accessing user data
