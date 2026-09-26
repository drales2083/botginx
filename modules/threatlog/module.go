package threatlog

import (
	"embed"
	"io/fs"

	"github.com/botginx/botginx/modules/threatlog/handlers"
	"github.com/botginx/botginx/modules/threatlog/services"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templatesFS embed.FS

type Module struct {
	*module.BaseModule
	service *services.ThreatLogService
	handler *handlers.Handler
}

func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"threatlog",
			"Threat Log",
			"Real-time security threat monitoring and analysis",
		),
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)

	m.service = services.NewThreatLogService(deps.DB)
	m.handler = handlers.NewHandler(m.service, deps.Templates)

	tmplFS, _ := fs.Sub(templatesFS, "templates")
	deps.Templates.RegisterModule(m.ID(), tmplFS)

	return nil
}

func (m *Module) Migrate() error {
	return nil
}

func (m *Module) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", m.handler.Index)

	r.Route("/api", func(r chi.Router) {
		r.Get("/threats", m.handler.APIThreats)
		r.Get("/stats", m.handler.APIStats)
		r.Get("/recent", m.handler.APIRecent)
		r.Get("/filters", m.handler.APIFilters)
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
			Title:   "Threat Log",
			Icon:    "bi-shield-exclamation",
			Path:    "/user/threatlog",
			Order:   28,
			Section: module.MenuSectionUser,
			Group:   "Metrics",
		},
	}
}

func (m *Module) Widgets() []module.Widget {
	return nil
}
