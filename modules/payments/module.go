package payments

import (
	"embed"
	"io/fs"

	"github.com/botginx/botginx/modules/payments/handlers"
	"github.com/botginx/botginx/modules/payments/services"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Module struct {
	*module.BaseModule
	service *services.PaymentService
	handler *handlers.Handler
}

func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"payments",
			"Payments",
			"Crypto payment gateway for deposits",
		),
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)

	m.service = services.NewPaymentService(deps.DB)
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

// Routes returns user-facing routes mounted at /user/payments
func (m *Module) Routes() chi.Router {
	r := chi.NewRouter()

	// Pages
	r.Get("/", m.handler.Deposit)
	r.Get("/transactions", m.handler.Transactions)

	// API
	r.Route("/api", func(r chi.Router) {
		r.Post("/generate-address", m.handler.APIGenerateAddress)
		r.Get("/transactions", m.handler.APIGetTransactions)
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

// AdminRoutes returns admin-facing routes mounted at /admin/payments
func (m *Module) AdminRoutes() chi.Router {
	r := chi.NewRouter()

	// Pages
	r.Get("/", m.handler.AdminDashboard)
	r.Get("/transactions", m.handler.AdminTransactions)

	// API
	r.Route("/api", func(r chi.Router) {
		r.Post("/resync/{txid}", m.handler.APIAdminResync)
		r.Get("/diagnostic/{txid}", m.handler.APIAdminDiagnostic)
		r.Post("/webhook", m.handler.APIAdminAddWebhook)
		r.Get("/webhooks", m.handler.APIAdminListWebhooks)
		r.Post("/sync-transfers", m.handler.APIAdminSyncTransfers)
	})

	return r
}

// PublicRoutes returns public routes (webhook endpoint)
func (m *Module) PublicRoutes() chi.Router {
	r := chi.NewRouter()

	// BitGo webhook - no auth required
	r.Post("/webhook/bitgo", m.handler.BitGoWebhook)

	return r
}

func (m *Module) Templates() fs.FS {
	tmplFS, _ := fs.Sub(templatesFS, "templates")
	return tmplFS
}

func (m *Module) MenuItems() []module.MenuItem {
	return []module.MenuItem{
		{
			Title:   "Deposit",
			Icon:    "bi-wallet2",
			Path:    "/user/payments",
			Order:   25,
			Section: module.MenuSectionUser,
			Group:   "Billing",
		},
		{
			Title:   "Payments",
			Icon:    "bi-credit-card",
			Path:    "/admin/payments",
			Order:   10,
			Section: module.MenuSectionAdmin,
			Group:   "Revenue",
		},
	}
}

// Service exposes the payment service to other modules
func (m *Module) Service() *services.PaymentService {
	return m.service
}
