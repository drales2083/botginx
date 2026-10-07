package pdfgenerator

import (
	"embed"
	"io/fs"

	"github.com/botginx/botginx/modules/pdfgenerator/handlers"
	"github.com/botginx/botginx/modules/pdfgenerator/services"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html templates/partials/*.html
var templatesFS embed.FS

//go:embed static/backgrounds/*
var backgroundsFS embed.FS

// Module implements the PDF generator feature
type Module struct {
	*module.BaseModule
	service *services.PDFService
	handler *handlers.Handler
}

func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"pdfgenerator",
			"Generate PDF",
			"Create PDFs with blurred backgrounds and CTA buttons",
		),
	}
}

func (m *Module) Migrate() error {
	// No database tables needed for this module
	return nil
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)

	// Get backgrounds filesystem
	bgFS, _ := fs.Sub(backgroundsFS, "static/backgrounds")

	// Initialize service
	m.service = services.NewPDFService(bgFS)

	// Initialize handler with link provider
	m.handler = handlers.NewHandler(m.service, deps.Templates, deps.DB, bgFS)

	// Register templates
	tmplFS, _ := fs.Sub(templatesFS, "templates")
	deps.Templates.RegisterModule(m.ID(), tmplFS)

	return nil
}

func (m *Module) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", m.handler.Index)
	r.Get("/api/links", m.handler.GetLinks)
	r.Get("/api/backgrounds", m.handler.GetBackgrounds)
	r.Get("/api/backgrounds/{name}", m.handler.GetBackgroundImage)
	r.Post("/api/preview", m.handler.Preview)
	r.Post("/api/generate", m.handler.Generate)

	return r
}

func (m *Module) Templates() fs.FS {
	tmplFS, _ := fs.Sub(templatesFS, "templates")
	return tmplFS
}

func (m *Module) MenuItems() []module.MenuItem {
	return []module.MenuItem{{
		Title:   "Generate PDF",
		Icon:    "bi-file-earmark-pdf",
		Path:    "/user/generate-pdf",
		Section: module.MenuSectionUser,
		Group:   "Links",
		Order:   115,
	}}
}

func (m *Module) Widgets() []module.Widget {
	return nil
}
