package qrcodes

import (
	"embed"
	"io/fs"

	"github.com/botginx/botginx/modules/qrcodes/handlers"
	"github.com/botginx/botginx/modules/qrcodes/services"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Module struct {
	*module.BaseModule
	service *services.QRCodeService
	handler *handlers.Handler
}

func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"qrcodes",
			"QR Codes",
			"Generate and manage QR codes",
		),
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)

	m.service = services.NewQRCodeService(deps.DB)
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
	return err
}

func (m *Module) Routes() chi.Router {
	r := chi.NewRouter()

	// Pages
	r.Get("/", m.handler.List)
	r.Get("/new", m.handler.New)
	r.Get("/{id}", m.handler.Show)

	// API
	r.Route("/api", func(r chi.Router) {
		r.Get("/", m.handler.APIList)
		r.Post("/", m.handler.APICreate)
		r.Post("/preview", m.handler.APIPreview)
		r.Get("/{id}", m.handler.APIGet)
		r.Put("/{id}", m.handler.APIUpdate)
		r.Delete("/{id}", m.handler.APIDelete)
		r.Get("/{id}/download", m.handler.APIDownload)
	})

	return r
}

func (m *Module) Templates() fs.FS {
	tmplFS, _ := fs.Sub(templatesFS, "templates")
	return tmplFS
}

// Service returns the QR code service for external use
func (m *Module) Service() *services.QRCodeService {
	return m.service
}

func (m *Module) MenuItems() []module.MenuItem {
	return []module.MenuItem{
		{
			Title:   "QR Codes",
			Icon:    "bi-qr-code",
			Path:    "/user/qrcodes",
			Order:   7,
			Section: module.MenuSectionUser,
			Group:   "Links",
		},
	}
}

func (m *Module) Widgets() []module.Widget {
	return nil
}

func (m *Module) SetLinkLister(fn handlers.RedirectLinkLister) {
	m.handler.SetLinkLister(fn)
}
