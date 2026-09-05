package shortener

import (
	"embed"
	"io/fs"

	domainmodels "github.com/botginx/botginx/modules/domains/models"
	"github.com/botginx/botginx/modules/shortener/handlers"
	"github.com/botginx/botginx/modules/shortener/services"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed migrations/*.sql
var migrationsFS embed.FS

type DomainProvider interface {
	ListAvailable(userID string) ([]domainmodels.Domain, error)
}

type Module struct {
	*module.BaseModule
	service *services.ShortenerService
	handler *handlers.Handler
	domains DomainProvider
}

func New(domains DomainProvider) *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"shortener",
			"Link Shortener",
			"Create short links with protection",
		),
		domains: domains,
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)

	m.service = services.NewShortenerService(deps.DB)
	m.handler = handlers.NewHandler(m.service, deps.Templates, m.domains)

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
	r.Get("/{id}/qr", m.handler.QRCode)

	// API
	r.Route("/api", func(r chi.Router) {
		r.Get("/", m.handler.APIList)
		r.Post("/", m.handler.APICreate)
		r.Get("/random-path", m.handler.APIRandomPath)
		r.Get("/check-path", m.handler.APICheckPath)
		r.Get("/{id}", m.handler.APIGet)
		r.Put("/{id}", m.handler.APIUpdate)
		r.Delete("/{id}", m.handler.APIDelete)
		r.Get("/{id}/stats", m.handler.APIStats)
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
			Title:   "Link Shortener",
			Icon:    "bi-link-45deg",
			Path:    "/user/shortener",
			Order:   15,
			Section: module.MenuSectionUser,
		},
	}
}

func (m *Module) Widgets() []module.Widget {
	return nil
}
