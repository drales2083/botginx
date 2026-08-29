package redirectlinks

import (
	"embed"
	"io/fs"

	domainmodels "github.com/botginx/botginx/modules/domains/models"
	"github.com/botginx/botginx/modules/redirectlinks/handlers"
	"github.com/botginx/botginx/modules/redirectlinks/services"
	servermodels "github.com/botginx/botginx/modules/servers/models"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DomainProvider supplies the domains a user may deploy to: their own, plus the
// shared pool when an admin has enabled it.
type DomainProvider interface {
	ListAvailable(userID string) ([]domainmodels.Domain, error)
}

// ServerPool hands out a deploy target. Users never pick a server -- the
// platform picks one at random from the admin's pool.
type ServerPool interface {
	PickRandom() (*servermodels.Server, error)
}

// Module implements the redirect links feature
type Module struct {
	*module.BaseModule
	service *services.RedirectLinkService
	handler *handlers.Handler
	domains DomainProvider
	servers ServerPool
}

func New(domains DomainProvider, servers ServerPool) *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"redirectlinks",
			"Redirect Links",
			"Custom redirect links deployed to VPS",
		),
		domains: domains,
		servers: servers,
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)

	// Initialize service
	m.service = services.NewRedirectLinkService(deps.DB)

	// Initialize handler
	m.handler = handlers.NewHandler(m.service, deps.Templates, m.domains, m.servers)

	// Register templates
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
	r.Get("/{id}/customize", m.handler.Customize)

	// API
	r.Route("/api", func(r chi.Router) {
		r.Get("/", m.handler.APIList)
		r.Post("/", m.handler.APICreate)
		r.Get("/random-subdomain", m.handler.APIRandomSubdomain)
		r.Get("/{id}", m.handler.APIGet)
		r.Put("/{id}", m.handler.APIUpdate)
		r.Delete("/{id}", m.handler.APIDelete)
		r.Put("/{id}/customization", m.handler.APIUpdateCustomization)
		r.Post("/{id}/deploy", m.handler.APIDeploy)
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
			Title:   "Redirect Links",
			Icon:    "bi-link-45deg",
			Path:    "/user/redirectlinks",
			Order:   20,
			Section: module.MenuSectionUser,
		},
	}
}

func (m *Module) Widgets() []module.Widget {
	return nil
}

// ResolveByHost maps an inbound hostname to the link serving it and its owner.
// Delegated so analytics can depend on the module value at registration time,
// before Init has built the service.
func (m *Module) ResolveByHost(host string) (linkID, userID string, err error) {
	return m.service.ResolveByHost(host)
}

// OwnerOf returns the account a link belongs to.
func (m *Module) OwnerOf(linkID string) (string, error) {
	return m.service.OwnerOf(linkID)
}
