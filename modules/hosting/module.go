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
	service      *services.HostingService
	billing      *services.BillingService
	provisioning *services.ProvisioningService
	handler      *handlers.Handler
	bgVerifier   *services.BackgroundVerifier
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
	m.billing = services.NewBillingService(deps.DB, m.service)
	m.provisioning = services.NewProvisioningService(deps.DB)
	m.handler = handlers.NewHandler(m.service, m.billing, m.provisioning, deps.Templates)

	// Start background verifier for DNS/SSL auto-setup
	m.bgVerifier = services.NewBackgroundVerifier(m.service)
	m.bgVerifier.Start()

	tmplFS, _ := fs.Sub(templatesFS, "templates")
	deps.Templates.RegisterModule(m.ID(), tmplFS)

	return nil
}

// Migrate runs database migrations
func (m *Module) Migrate() error {
	// Run initial schema
	sql1, err := fs.ReadFile(migrationsFS, "migrations/001_create_tables.sql")
	if err != nil {
		return err
	}
	if _, err = m.DB().Exec(string(sql1)); err != nil {
		return err
	}

	// Run simplification migration
	sql2, err := fs.ReadFile(migrationsFS, "migrations/002_simplify_hosting.sql")
	if err != nil {
		return err
	}
	if _, err = m.DB().Exec(string(sql2)); err != nil {
		return err
	}

	// Add server type column
	sql3, err := fs.ReadFile(migrationsFS, "migrations/003_add_server_type.sql")
	if err != nil {
		return err
	}
	if _, err = m.DB().Exec(string(sql3)); err != nil {
		return err
	}

	// Fix user_id column types for UUID support
	sql4, err := fs.ReadFile(migrationsFS, "migrations/004_fix_user_id_types.sql")
	if err != nil {
		return err
	}
	if _, err = m.DB().Exec(string(sql4)); err != nil {
		return err
	}

	// Add provisioning_error column
	sql5, err := fs.ReadFile(migrationsFS, "migrations/005_add_provisioning_error.sql")
	if err != nil {
		return err
	}
	if _, err = m.DB().Exec(string(sql5)); err != nil {
		return err
	}

	// Add domain status tracking
	sql6, err := fs.ReadFile(migrationsFS, "migrations/006_add_domain_status.sql")
	if err != nil {
		return err
	}
	if _, err = m.DB().Exec(string(sql6)); err != nil {
		return err
	}

	// Add hosting visits table for domain analytics
	sql7, err := fs.ReadFile(migrationsFS, "migrations/007_hosting_visits.sql")
	if err != nil {
		return err
	}
	if _, err = m.DB().Exec(string(sql7)); err != nil {
		return err
	}

	// Enable link_settings on all servers (background, non-blocking)
	go m.enableLinkSettingsOnAllServers()

	return nil
}

// enableLinkSettingsOnAllServers enables local file settings on all hosting servers
func (m *Module) enableLinkSettingsOnAllServers() {
	servers, err := m.service.ListServers()
	if err != nil || len(servers) == 0 {
		return
	}
	for _, server := range servers {
		if server.IsActive {
			m.service.EnableLinkSettingsOnServer(server.ID)
		}
	}

	// Push settings for all existing domains
	m.service.PushAllDomainSettings()
}

// RoutesForSection returns routes for the requested section (user or admin)
func (m *Module) RoutesForSection(section module.MenuSection) chi.Router {
	if section == module.MenuSectionAdmin {
		return m.AdminRoutes()
	}
	return m.Routes()
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
		r.Get("/domains/{domainID}/setup", m.handler.UserDomainSetup)
		r.Get("/domains/{domainID}/analytics", m.handler.UserDomainAnalytics)
		r.Get("/domains/{domainID}/settings", m.handler.UserDomainSettings)
	})

	// API
	r.Route("/api", func(r chi.Router) {
		r.Route("/{accountID}", func(r chi.Router) {
			r.Get("/credentials", m.handler.APIGetPanelCredentials)
			r.Get("/status", m.handler.APIGetProvisioningStatus)
			r.Post("/retry", m.handler.APIRetryProvisioning)
			r.Put("/domains/{domainID}/settings", m.handler.APIUpdateDomainSettings)
			r.Get("/domains/{domainID}/stats", m.handler.APIGetDomainStats)
			r.Post("/reactivate", m.handler.APIReactivate)
			// Domain management
			r.Post("/domains", m.handler.APIUserAddDomain)
			r.Delete("/domains/{domainID}", m.handler.APIUserDeleteDomain)
			// Domain DNS/SSL status
			r.Get("/domains/{domainID}/status", m.handler.APIGetDomainStatus)
			r.Post("/domains/{domainID}/check-dns", m.handler.APICheckDNS)
			r.Post("/domains/{domainID}/ssl", m.handler.APIEnableSSL)
			// Analytics for antibot dashboard
			r.Get("/analytics/summary", m.handler.APIAnalyticsSummary)
			r.Get("/analytics", m.handler.APIAnalytics)
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
		r.Put("/servers/{id}", m.handler.APIUpdateServer)
		r.Put("/servers/{id}/toggle", m.handler.APIToggleServer)
		r.Delete("/servers/{id}", m.handler.APIDeleteServer)
		r.Post("/servers/{id}/enable-link-settings", m.handler.APIEnableLinkSettings)

		// Packages
		r.Post("/packages", m.handler.APICreatePackage)
		r.Put("/packages/{id}", m.handler.APIUpdatePackage)
		r.Put("/packages/{id}/toggle", m.handler.APITogglePackage)

		// Accounts
		r.Get("/accounts/{id}/credentials", m.handler.APIGetAccountCredentials)
		r.Post("/accounts/{id}/link", m.handler.APILinkAccount)
		r.Post("/accounts/{id}/domains", m.handler.APIAddDomain)
		r.Delete("/accounts/{id}/domains/{domainID}", m.handler.APIDeleteDomain)
		r.Put("/accounts/{id}/suspend", m.handler.APISuspendAccount)
		r.Put("/accounts/{id}/unsuspend", m.handler.APIUnsuspendAccount)
		r.Delete("/accounts/{id}", m.handler.APIDeleteAccount)

		// Balance
		r.Post("/balance/topup", m.handler.APITopUpBalance)

		// Debug / diagnostics
		r.Get("/domains/{domainID}/verify", m.handler.APIVerifyDomainSettings)
		r.Post("/domains/{domainID}/push", m.handler.APIPushDomainSettings)
	})

	return r
}

// Templates returns the module's template filesystem
func (m *Module) Templates() fs.FS {
	tmplFS, _ := fs.Sub(templatesFS, "templates")
	return tmplFS
}

// MenuItems returns menu items for this module (both user and admin sections)
func (m *Module) MenuItems() []module.MenuItem {
	return []module.MenuItem{
		{
			Title:   "Hosting",
			Icon:    "bi-shield-check",
			Path:    "/user/hosting",
			Order:   50,
			Section: module.MenuSectionUser,
		},
		{
			Title:   "Hosting",
			Icon:    "bi-shield-check",
			Path:    "/admin/hosting",
			Order:   50,
			Section: module.MenuSectionAdmin,
		},
	}
}

// CronRoutes returns internal routes for cron jobs (localhost only)
func (m *Module) CronRoutes() chi.Router {
	r := chi.NewRouter()
	r.Post("/billing", m.handler.RunBilling)
	return r
}

// Service returns the hosting service for use by other modules
func (m *Module) Service() *services.HostingService {
	return m.service
}

// SetPaymentProcessor sets the callback for referral commission processing
func (m *Module) SetPaymentProcessor(p services.PaymentProcessor) {
	m.billing.SetPaymentProcessor(p)
}
