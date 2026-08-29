package domains

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"

	"github.com/botginx/botginx/modules/domains/handlers"
	"github.com/botginx/botginx/modules/domains/models"
	"github.com/botginx/botginx/modules/domains/services"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Module struct {
	*module.BaseModule
	service *services.DomainService
	handler *handlers.Handler
}

func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"domains",
			"Domains",
			"Manage your domains",
		),
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)

	m.service = services.NewDomainService(deps.DB)
	m.handler = handlers.NewHandler(m.service, deps.Templates)

	tmplFS, _ := fs.Sub(templatesFS, "templates")
	deps.Templates.RegisterModule(m.ID(), tmplFS)

	return nil
}

func (m *Module) Migrate() error {
	files, err := fs.Glob(migrationsFS, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(files)

	for _, f := range files {
		sql, err := fs.ReadFile(migrationsFS, f)
		if err != nil {
			return err
		}
		if _, err := m.DB().Exec(string(sql)); err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
	}
	return nil
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
		r.Get("/{id}", m.handler.APIGet)
		r.Put("/{id}", m.handler.APIUpdate)
		r.Delete("/{id}", m.handler.APIDelete)
		r.Post("/{id}/verify", m.handler.APIVerifyDNS)
		r.Get("/{id}/ssl", m.handler.APICheckSSL)
		r.Post("/{id}/setup", m.handler.APISetupDomain)
		r.Get("/{id}/wildcard-ssl", m.handler.APIGetWildcardSSLInstructions)
	})

	return r
}

// RoutesForSection serves the user's own domains under /user/domains and the
// shared platform pool under /admin/domains.
func (m *Module) RoutesForSection(section module.MenuSection) chi.Router {
	if section != module.MenuSectionAdmin {
		return m.Routes()
	}

	r := chi.NewRouter()

	// Pages
	r.Get("/", m.handler.SharedList)
	r.Get("/new", m.handler.SharedNew)

	// API
	r.Route("/api", func(r chi.Router) {
		r.Get("/", m.handler.APISharedList)
		r.Post("/", m.handler.APISharedCreate)
		r.Delete("/{id}", m.handler.APIDelete)
		r.Post("/{id}/verify", m.handler.APIVerifyDNS)
		r.Get("/{id}/ssl", m.handler.APICheckSSL)
		r.Post("/{id}/setup", m.handler.APISetupDomain)
		r.Get("/{id}/wildcard-ssl", m.handler.APIGetWildcardSSLInstructions)
		r.Put("/toggle", m.handler.APIToggleShared)
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
			Title:   "Domains",
			Icon:    "bi-globe",
			Path:    "/user/domains",
			Order:   15,
			Section: module.MenuSectionUser,
		},
		{
			Title:   "Shared Domains",
			Icon:    "bi-globe2",
			Path:    "/admin/domains",
			Order:   15,
			Section: module.MenuSectionAdmin,
		},
	}
}

// Service exposes the domain service to other modules (deploy target lookup).
func (m *Module) Service() *services.DomainService {
	return m.service
}

// ListAvailable returns every domain a user may deploy to. Delegated so other
// modules can depend on the module value at registration time, before Init has
// built the service.
func (m *Module) ListAvailable(userID string) ([]models.Domain, error) {
	return m.service.ListAvailable(userID)
}

func (m *Module) Widgets() []module.Widget {
	return []module.Widget{
		{
			ID:    "domains-count",
			Title: "Total Domains",
			Icon:  "bi-globe",
			Size:  "small",
			Order: 20,
			Data: func() (interface{}, error) {
				count, err := m.service.CountAll()
				return map[string]interface{}{
					"count": count,
					"label": "Domains",
				}, err
			},
		},
	}
}
