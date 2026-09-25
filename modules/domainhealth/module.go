package domainhealth

import (
	"embed"
	"io/fs"

	"github.com/botginx/botginx/modules/domainhealth/handlers"
	"github.com/botginx/botginx/modules/domainhealth/services"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templatesFS embed.FS

type Module struct {
	*module.BaseModule
	service *services.HealthService
	handler *handlers.Handler
}

func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"domainhealth",
			"Domain Health",
			"Monitor domain DNS, SSL and connectivity status",
		),
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)

	m.service = services.NewHealthService(deps.DB)
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
	r.Post("/api/check/{id}", m.handler.CheckDomain)
	r.Post("/api/check-all", m.handler.CheckAll)

	return r
}

func (m *Module) Templates() fs.FS {
	tmplFS, _ := fs.Sub(templatesFS, "templates")
	return tmplFS
}

func (m *Module) MenuItems() []module.MenuItem {
	return nil
}

func (m *Module) Widgets() []module.Widget {
	return nil
}
