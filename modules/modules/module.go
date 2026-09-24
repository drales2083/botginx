package modules

import (
	"embed"
	"io/fs"

	"github.com/botginx/botginx/modules/modules/handlers"
	"github.com/botginx/botginx/modules/modules/services"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Module struct {
	*module.BaseModule
	service  *services.ModuleService
	handler  *handlers.Handler
	registry *module.Registry
}

func New(registry *module.Registry) *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"modules",
			"Modules",
			"Manage installed modules",
		),
		registry: registry,
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)

	m.service = services.NewModuleService(deps.DB, m.registry)
	m.handler = handlers.NewHandler(m.service, deps.Templates)

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
	if err != nil {
		return err
	}

	// Sync modules to database
	return m.service.SyncModules()
}

func (m *Module) Routes() chi.Router {
	r := chi.NewRouter()

	// Pages
	r.Get("/", m.handler.List)
	r.Get("/{id}", m.handler.Show)

	// API
	r.Route("/api", func(r chi.Router) {
		r.Get("/", m.handler.APIList)

		// Declared before /{id} routes so "restart" is not read as a module id.
		r.Post("/restart", m.handler.APIRestart)

		r.Get("/{id}", m.handler.APIGet)
		r.Post("/{id}/enable", m.handler.APIEnable)
		r.Post("/{id}/disable", m.handler.APIDisable)
		r.Put("/{id}/settings", m.handler.APIUpdateSettings)
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
			Title:   "Modules",
			Icon:    "bi-puzzle",
			Path:    "/admin/modules",
			Order:   40,
			Section: module.MenuSectionAdmin,
			Group:   "System",
		},
	}
}

func (m *Module) Widgets() []module.Widget {
	return []module.Widget{
		{
			ID:    "modules-count",
			Title: "Active Modules",
			Icon:  "bi-puzzle",
			Size:  "small",
			Order: 5,
			Data: func() (interface{}, error) {
				count, err := m.service.CountEnabled()
				return map[string]interface{}{
					"count": count,
					"label": "Modules",
				}, err
			},
		},
	}
}

// Service exposes the module service for external use
func (m *Module) Service() *services.ModuleService {
	return m.service
}
