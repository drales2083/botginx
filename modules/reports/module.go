package reports

import (
	"embed"
	"io/fs"

	"github.com/botginx/botginx/modules/reports/handlers"
	"github.com/botginx/botginx/modules/reports/services"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templatesFS embed.FS

type Module struct {
	*module.BaseModule
	service *services.ReportService
	handler *handlers.Handler
}

func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"reports",
			"Reports",
			"Download analytics reports as CSV",
		),
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)

	m.service = services.NewReportService(deps.DB)
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
	r.Get("/download", m.handler.Download)

	return r
}

func (m *Module) Templates() fs.FS {
	tmplFS, _ := fs.Sub(templatesFS, "templates")
	return tmplFS
}

func (m *Module) MenuItems() []module.MenuItem {
	return []module.MenuItem{
		{
			Title:   "Reports",
			Icon:    "bi-file-earmark-spreadsheet",
			Path:    "/user/reports",
			Order:   15,
			Section: module.MenuSectionUser,
			Group:   "Analytics",
		},
	}
}

func (m *Module) Widgets() []module.Widget {
	return nil
}
