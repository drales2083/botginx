package analogstats

import (
	"embed"
	"io/fs"

	"github.com/botginx/botginx/modules/analogstats/handlers"
	"github.com/botginx/botginx/modules/analogstats/services"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templatesFS embed.FS

type Module struct {
	*module.BaseModule
	service *services.StatsService
	handler *handlers.Handler
}

func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"analogstats",
			"Analog Stats",
			"Classic web server statistics and traffic reports",
		),
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)

	m.service = services.NewStatsService(deps.DB)
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

	// API
	r.Route("/api", func(r chi.Router) {
		r.Get("/summary", m.handler.APISummary)
		r.Get("/hourly", m.handler.APIHourly)
		r.Get("/daily", m.handler.APIDaily)
		r.Get("/browsers", m.handler.APIBrowsers)
		r.Get("/os", m.handler.APIOS)
		r.Get("/countries", m.handler.APICountries)
		r.Get("/pages", m.handler.APIPages)
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
			Title:   "Analog Stats",
			Icon:    "bi-bar-chart-steps",
			Path:    "/user/analogstats",
			Order:   26,
			Section: module.MenuSectionUser,
			Group:   "Metrics",
		},
	}
}

func (m *Module) Widgets() []module.Widget {
	return nil
}
