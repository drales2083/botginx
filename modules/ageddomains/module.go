package ageddomains

import (
	"context"
	"embed"
	"io/fs"
	"log"

	"github.com/botginx/botginx/modules/ageddomains/handlers"
	"github.com/botginx/botginx/modules/ageddomains/services"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Module struct {
	*module.BaseModule
	balanceService   *services.BalanceService
	inventoryService *services.InventoryService
	purchaseService  *services.PurchaseService
	providerRegistry *services.ProviderRegistry
	favoritesService *services.FavoritesService
	handler          *handlers.Handler
	adminHandler     *handlers.AdminHandler
	background       *services.BackgroundJobs
}

func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"ageddomains",
			"Aged Domains",
			"Purchase aged domains from the marketplace",
		),
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)

	// Initialize services
	m.balanceService = services.NewBalanceService(deps.DB)
	m.providerRegistry = services.NewProviderRegistry(deps.DB)
	m.providerRegistry.InitializeClients(context.Background())
	m.inventoryService = services.NewInventoryService(deps.DB, m.providerRegistry)
	m.purchaseService = services.NewPurchaseService(deps.DB, m.balanceService, m.inventoryService, m.providerRegistry)
	m.favoritesService = services.NewFavoritesService(deps.DB)

	// Initialize handlers
	m.handler = handlers.NewHandler(m.balanceService, m.inventoryService, m.purchaseService, m.favoritesService, deps.Templates)
	m.adminHandler = handlers.NewAdminHandler(m.providerRegistry, m.inventoryService, m.purchaseService, deps.Templates)

	// Register templates
	tmplFS, _ := fs.Sub(templatesFS, "templates")
	deps.Templates.RegisterModule(m.ID(), tmplFS)

	// Start background jobs
	m.background = services.NewBackgroundJobs(deps.DB, m.inventoryService, m.balanceService, m.providerRegistry)
	m.background.Start()

	// Wire background jobs to admin handler for manual refresh
	m.adminHandler.SetBackgroundJobs(m.background)

	log.Printf("[ageddomains] Module initialized")
	return nil
}

// DomainCreator interface for creating domain records
type DomainCreator interface {
	CreateDomainFromPurchase(ctx context.Context, userID, domainName, verifyToken, purchaseID string) (string, error)
}

// SetDomainCreator sets the domain creator (from domains module)
func (m *Module) SetDomainCreator(dc DomainCreator) {
	m.purchaseService.SetDomainCreator(dc)
}

func (m *Module) Migrate() error {
	// Run main tables migration
	sql1, err := fs.ReadFile(migrationsFS, "migrations/001_create_tables.sql")
	if err != nil {
		return err
	}
	if _, err := m.DB().Exec(string(sql1)); err != nil {
		return err
	}

	// Run balance_transactions extension (may fail if table doesn't exist yet)
	sql2, err := fs.ReadFile(migrationsFS, "migrations/002_extend_balance_transactions.sql")
	if err != nil {
		return err
	}
	// Ignore errors for the extension migration (table may not exist)
	m.DB().Exec(string(sql2))

	// Run favorites table migration
	sql3, err := fs.ReadFile(migrationsFS, "migrations/003_add_favorites.sql")
	if err != nil {
		return err
	}
	if _, err := m.DB().Exec(string(sql3)); err != nil {
		return err
	}

	return nil
}

// Routes returns user-facing routes mounted at /user/ageddomains
func (m *Module) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", m.handler.Browse)

	r.Route("/api", func(r chi.Router) {
		r.Get("/domains", m.handler.APIListDomains)
		r.Get("/domains/{id}/check", m.handler.APICheckSafety)
		r.Post("/domains/{id}/favorite", m.handler.APIToggleFavorite)
		r.Get("/tlds", m.handler.APIGetTLDs)
		r.Post("/purchase/{id}", m.handler.APIPurchase)
		r.Get("/purchase/{id}/status", m.handler.APIGetStatus)
		r.Get("/balance", m.handler.APIGetBalance)
	})

	return r
}

// RoutesForSection returns routes for the requested section
func (m *Module) RoutesForSection(section module.MenuSection) chi.Router {
	if section == module.MenuSectionAdmin {
		return m.AdminRoutes()
	}
	return m.Routes()
}

// AdminRoutes returns admin-facing routes mounted at /admin/ageddomains
func (m *Module) AdminRoutes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", m.adminHandler.Dashboard)
	r.Get("/providers", m.adminHandler.Providers)
	r.Get("/inventory", m.adminHandler.Inventory)
	r.Get("/purchases", m.adminHandler.Purchases)

	r.Route("/api", func(r chi.Router) {
		// Provider management
		r.Get("/providers", m.adminHandler.APIListProviders)
		r.Post("/providers", m.adminHandler.APICreateProvider)
		r.Put("/providers/{id}", m.adminHandler.APIUpdateProvider)
		r.Delete("/providers/{id}", m.adminHandler.APIDeleteProvider)
		r.Post("/providers/{id}/test", m.adminHandler.APITestProvider)

		// Inventory management
		r.Get("/inventory", m.adminHandler.APIListInventory)
		r.Post("/inventory/refresh", m.adminHandler.APIRefreshInventory)

		// Purchases
		r.Get("/purchases", m.adminHandler.APIListPurchases)
	})

	return r
}

// MenuItems returns menu items for navigation
func (m *Module) MenuItems() []module.MenuItem {
	return []module.MenuItem{
		{
			Section: module.MenuSectionUser,
			Title:   "Aged Domains",
			Icon:    "bi-clock-history",
			Path:    "/user/ageddomains",
			Order:   26,
		},
		{
			Section: module.MenuSectionAdmin,
			Title:   "Aged Domains",
			Icon:    "bi-clock-history",
			Path:    "/admin/ageddomains",
			Order:   45,
		},
	}
}

func (m *Module) Templates() fs.FS {
	tmplFS, _ := fs.Sub(templatesFS, "templates")
	return tmplFS
}

func (m *Module) Shutdown() error {
	if m.background != nil {
		m.background.Stop()
	}
	return nil
}
