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
	service       *services.DomainService
	cpanelService *services.CpanelService
	handler       *handlers.Handler
	cpanelHandler *handlers.CpanelHandler
	background    *services.BackgroundVerifier
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
	m.cpanelService = services.NewCpanelService(deps.DB)
	m.handler = handlers.NewHandler(m.service, m.cpanelService, deps.Templates)
	m.cpanelHandler = handlers.NewCpanelHandler(m.cpanelService)

	tmplFS, _ := fs.Sub(templatesFS, "templates")
	deps.Templates.RegisterModule(m.ID(), tmplFS)

	// Start background verifier for auto DNS/SSL checks
	verifyService := services.NewVerificationService()
	m.background = services.NewBackgroundVerifier(m.service, verifyService)
	m.background.Start()

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
	r.Get("/{id}/settings", m.handler.Settings)   // Domain settings (Turnstile, etc.)
	r.Get("/{id}/setup", m.handler.ExternalSetup) // External domain setup wizard

	// API
	r.Route("/api", func(r chi.Router) {
		r.Get("/", m.handler.APIList)
		r.Post("/", m.handler.APICreate)
		r.Get("/{id}", m.handler.APIGet)
		r.Put("/{id}", m.handler.APIUpdate)
		r.Delete("/{id}", m.handler.APIDelete)
		r.Post("/{id}/verify", m.handler.APIVerifyDNS)
		r.Post("/{id}/transfer", m.handler.APITransferOwnership) // Admin: transfer to user
		r.Get("/{id}/ssl", m.handler.APICheckSSL)
		r.Post("/{id}/setup", m.handler.APISetupDomain)
		r.Get("/{id}/setup-status", m.handler.APIGetSetupStatus) // Polling endpoint
		r.Post("/{id}/retry-ssl", m.handler.APIRetrySSL)          // Force retry SSL generation

		// Two-phase wildcard SSL (using lego) - legacy
		r.Post("/{id}/ssl/start", m.handler.APIStartSSLChallenge)       // Phase 1: Get token
		r.Post("/{id}/ssl/complete", m.handler.APICompleteSSLChallenge) // Phase 2: Complete after DNS
		r.Get("/{id}/ssl/token", m.handler.APIGetSSLToken)              // Get pending token
		r.Delete("/{id}/ssl/cancel", m.handler.APICancelSSLChallenge)   // Cancel pending challenge

		// acme-dns CNAME delegation (recommended for wildcard)
		r.Post("/{id}/acme-dns/register", m.handler.APIRegisterAcmeDNS) // Register with acme-dns
		r.Post("/{id}/acme-dns/verify", m.handler.APICheckAcmeCname)    // Verify CNAME record
		r.Post("/{id}/ssl/acme-dns", m.handler.APIGenerateSSLAcmeDNS)   // Generate SSL via acme-dns

		// Domain settings (Turnstile, etc.)
		r.Put("/{id}/settings", m.handler.APIUpdateSettings)
		r.Get("/{id}/turnstile", m.handler.APIGetTurnstileStatus)
	})

	// cPanel connections API
	r.Route("/cpanel", func(r chi.Router) {
		r.Get("/", m.cpanelHandler.ListConnections)
		r.Post("/", m.cpanelHandler.CreateConnection)
		r.Post("/test", m.cpanelHandler.TestNewConnection) // Test before saving
		r.Get("/{id}", m.cpanelHandler.GetConnection)
		r.Put("/{id}", m.cpanelHandler.UpdateConnection)
		r.Delete("/{id}", m.cpanelHandler.DeleteConnection)
		r.Post("/{id}/test", m.cpanelHandler.TestConnection)
		r.Get("/{id}/domains", m.cpanelHandler.ListDomains)
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
	r.Get("/{id}/setup", m.handler.SharedSetup)

	// API
	r.Route("/api", func(r chi.Router) {
		r.Get("/", m.handler.APISharedList)
		r.Post("/", m.handler.APISharedCreate)
		r.Delete("/{id}", m.handler.APIDelete)
		r.Post("/{id}/verify", m.handler.APIVerifyDNS)
		r.Get("/{id}/ssl", m.handler.APICheckSSL)
		r.Post("/{id}/setup", m.handler.APISetupDomain)
		r.Put("/toggle", m.handler.APIToggleShared)

		// SSL setup wizard endpoints (same as user but for shared domains)
		r.Get("/{id}/setup-status", m.handler.APIGetSetupStatus)
		r.Post("/{id}/ssl/start", m.handler.APIStartSSLChallenge)
		r.Post("/{id}/ssl/complete", m.handler.APICompleteSSLChallenge)
		r.Get("/{id}/ssl/token", m.handler.APIGetSSLToken)
		r.Delete("/{id}/ssl/cancel", m.handler.APICancelSSLChallenge)
		r.Post("/{id}/retry-ssl", m.handler.APIRetrySSL)

		// acme-dns CNAME delegation (same as user)
		r.Post("/{id}/acme-dns/register", m.handler.APIRegisterAcmeDNS)
		r.Post("/{id}/acme-dns/verify", m.handler.APICheckAcmeCname)
		r.Post("/{id}/ssl/acme-dns", m.handler.APIGenerateSSLAcmeDNS)
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
			Order:   35,
			Section: module.MenuSectionUser,
			Group:   "Infrastructure",
		},
		{
			Title:   "Shared Domains",
			Icon:    "bi-globe2",
			Path:    "/admin/domains",
			Order:   15,
			Section: module.MenuSectionAdmin,
			Group:   "Admin Infrastructure",
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
