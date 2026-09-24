package marketplace

import (
	"embed"
	"io/fs"

	"github.com/botginx/botginx/modules/marketplace/handlers"
	"github.com/botginx/botginx/modules/marketplace/services"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Module struct {
	*module.BaseModule
	service *services.MarketplaceService
	handler *handlers.Handler
}

func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"marketplace",
			"Marketplace",
			"Domain marketplace - buy and sell domains",
		),
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)

	m.service = services.NewMarketplaceService(deps.DB)
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

// Routes returns user-facing routes mounted at /user/marketplace
func (m *Module) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", m.handler.UserBrowse)

	r.Route("/api", func(r chi.Router) {
		r.Get("/domains", m.handler.APIListAvailable)
		r.Post("/purchase/{domainID}", m.handler.APIPurchase)
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

// AdminRoutes returns admin-facing routes mounted at /admin/marketplace
func (m *Module) AdminRoutes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", m.handler.AdminList)

	r.Route("/api", func(r chi.Router) {
		r.Get("/domains", m.handler.APIListForSale)
		r.Post("/domains", m.handler.APIMarkForSale)
		r.Put("/domains/{domainID}", m.handler.APIUpdateListing)
		r.Delete("/domains/{domainID}", m.handler.APIRemoveFromSale)
		r.Get("/sales", m.handler.APIGetSales)
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
			Title:   "Buy Domain",
			Icon:    "bi-bag",
			Path:    "/user/marketplace",
			Order:   25,
			Section: module.MenuSectionUser,
			Hidden:  true, // Accessed via button on /user/domains
		},
		{
			Title:   "Sell Domain",
			Icon:    "bi-tag",
			Path:    "/admin/marketplace",
			Order:   20,
			Section: module.MenuSectionAdmin,
			Group:   "Revenue",
		},
	}
}
