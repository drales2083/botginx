package hosting

import (
	"embed"
	"io/fs"

	"github.com/botginx/botginx/modules/hosting/handlers"
	"github.com/botginx/botginx/modules/hosting/services"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html templates/partials/*.html
var templatesFS embed.FS

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Module implements the Bullet Proof Hosting feature
type Module struct {
	*module.BaseModule
	service *services.HostingService
	handler *handlers.Handler
}

// New creates a new hosting module instance
func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"hosting",
			"Hosting",
			"Bullet Proof Hosting management",
		),
	}
}

// Init initializes the module with dependencies
func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)

	m.service = services.NewHostingService(deps.DB)
	m.handler = handlers.NewHandler(m.service, deps.Templates)

	tmplFS, _ := fs.Sub(templatesFS, "templates")
	deps.Templates.RegisterModule(m.ID(), tmplFS)

	return nil
}

// Migrate runs database migrations
func (m *Module) Migrate() error {
	sql, err := fs.ReadFile(migrationsFS, "migrations/001_create_tables.sql")
	if err != nil {
		return err
	}
	_, err = m.DB().Exec(string(sql))
	return err
}

// Routes returns user-facing routes mounted at /user/hosting
func (m *Module) Routes() chi.Router {
	r := chi.NewRouter()

	// Account list / redirect
	r.Get("/", m.handler.UserIndex)
	r.Get("/buy", m.handler.UserPurchase)
	r.Post("/buy", m.handler.UserDoPurchase)

	// Account-specific routes
	r.Route("/{accountID}", func(r chi.Router) {
		r.Get("/", m.handler.UserOverview)
		r.Get("/domains", m.handler.UserDomains)
		r.Get("/domains/{domainID}/settings", m.handler.UserDomainSettings)
		r.Get("/emails", m.handler.UserEmails)
		r.Get("/databases", m.handler.UserDatabases)
		r.Get("/ftp", m.handler.UserFTP)
	})

	// API
	r.Route("/api", func(r chi.Router) {
		r.Route("/{accountID}", func(r chi.Router) {
			r.Post("/domains", m.handler.APIAddDomain)
			r.Delete("/domains/{domainID}", m.handler.APIDeleteDomain)
			r.Post("/domains/{domainID}/ssl", m.handler.APIEnableSSL)
			r.Put("/domains/{domainID}/settings", m.handler.APIUpdateDomainSettings)
			r.Post("/emails", m.handler.APIAddEmail)
			r.Delete("/emails/{emailID}", m.handler.APIDeleteEmail)
			r.Post("/databases", m.handler.APIAddDatabase)
			r.Delete("/databases/{dbID}", m.handler.APIDeleteDatabase)
			r.Post("/ftp", m.handler.APIAddFTP)
			r.Delete("/ftp/{ftpID}", m.handler.APIDeleteFTP)
			r.Post("/reactivate", m.handler.APIReactivate)
		})
	})

	return r
}

// AdminRoutes returns admin-facing routes mounted at /admin/hosting
func (m *Module) AdminRoutes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", m.handler.AdminServers)
	r.Get("/packages", m.handler.AdminPackages)
	r.Get("/accounts", m.handler.AdminAccounts)

	// API
	r.Route("/api", func(r chi.Router) {
		// Servers
		r.Post("/servers", m.handler.APICreateServer)
		r.Post("/servers/{id}/test", m.handler.APITestServer)
		r.Put("/servers/{id}/toggle", m.handler.APIToggleServer)
		r.Delete("/servers/{id}", m.handler.APIDeleteServer)

		// Packages
		r.Post("/packages", m.handler.APICreatePackage)
		r.Put("/packages/{id}", m.handler.APIUpdatePackage)
		r.Put("/packages/{id}/toggle", m.handler.APITogglePackage)

		// Accounts
		r.Put("/accounts/{id}/suspend", m.handler.APISuspendAccount)
		r.Put("/accounts/{id}/unsuspend", m.handler.APIUnsuspendAccount)

		// Balance
		r.Post("/balance/topup", m.handler.APITopUpBalance)
	})

	return r
}

// Templates returns the module's template filesystem
func (m *Module) Templates() fs.FS {
	tmplFS, _ := fs.Sub(templatesFS, "templates")
	return tmplFS
}

// MenuItems returns the user menu items for this module
func (m *Module) MenuItems() []module.MenuItem {
	return []module.MenuItem{
		{
			Title:   "Hosting",
			Icon:    "bi-shield-check",
			Path:    "/user/hosting",
			Order:   50,
			Section: module.MenuSectionUser,
		},
	}
}

// AdminMenuItems returns the admin menu items for this module
func (m *Module) AdminMenuItems() []module.MenuItem {
	return []module.MenuItem{
		{
			Title:   "Hosting",
			Icon:    "bi-shield-check",
			Path:    "/admin/hosting",
			Section: module.MenuSectionAdmin,
		},
	}
}

// Service returns the hosting service for use by other modules
func (m *Module) Service() *services.HostingService {
	return m.service
}
