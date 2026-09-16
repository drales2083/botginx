package support

import (
	"embed"
	"io/fs"

	"github.com/botginx/botginx/modules/support/handlers"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Module struct {
	*module.BaseModule
	handler *handlers.Handler
}

func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"support",
			"Support",
			"Support ticket system",
		),
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)

	m.handler = handlers.NewHandler(deps.DB, deps.Templates)

	tmplFS, _ := fs.Sub(templatesFS, "templates")
	deps.Templates.RegisterModule(m.ID(), tmplFS)

	return nil
}

func (m *Module) Migrate() error {
	sql, err := fs.ReadFile(migrationsFS, "migrations/001_create_tickets.sql")
	if err != nil {
		return err
	}
	_, err = m.DB().Exec(string(sql))
	return err
}

func (m *Module) Routes() chi.Router {
	r := chi.NewRouter()

	// User pages
	r.Get("/", m.handler.List)
	r.Get("/new", m.handler.New)
	r.Get("/{id}", m.handler.Show)

	// API
	r.Post("/api", m.handler.APICreate)
	r.Post("/api/{id}/reply", m.handler.APIReply)
	r.Post("/api/{id}/close", m.handler.APIClose)

	return r
}

func (m *Module) AdminRoutes() chi.Router {
	r := chi.NewRouter()

	// Admin pages
	r.Get("/", m.handler.AdminList)
	r.Get("/{id}", m.handler.AdminShow)

	// API
	r.Post("/api/{id}/reply", m.handler.APIAdminReply)
	r.Post("/api/{id}/close", m.handler.APIAdminClose)
	r.Post("/api/{id}/reopen", m.handler.APIAdminReopen)

	return r
}

// MenuItems returns sidebar navigation items for this module
func (m *Module) MenuItems() []module.MenuItem {
	return []module.MenuItem{
		{
			Title:   "Support",
			Icon:    "bi-headset",
			Path:    "/user/support",
			Order:   85,
			Section: module.MenuSectionUser,
		},
	}
}

// AdminMenuItems returns admin sidebar navigation items
func (m *Module) AdminMenuItems() []module.MenuItem {
	return []module.MenuItem{
		{
			Title:   "Support",
			Icon:    "bi-headset",
			Path:    "/admin/support",
			Section: module.MenuSectionAdmin,
		},
	}
}
