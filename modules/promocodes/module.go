package promocodes

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"

	"github.com/botginx/botginx/modules/promocodes/handlers"
	"github.com/botginx/botginx/modules/promocodes/services"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Module is the promo codes module
type Module struct {
	*module.BaseModule
	service *services.PromoService
	handler *handlers.Handler
}

// New creates a new promo codes module
func New(db interface{}, templates interface{}) *Module {
	// This constructor is for backwards compatibility but won't be used
	// The real initialization happens in Init()
	return &Module{
		BaseModule: module.NewBaseModule(
			"promocodes",
			"Promo Codes",
			"Discount codes for subscriptions",
		),
	}
}

// Init initializes the module with dependencies
func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)

	m.service = services.NewPromoService(deps.DB)
	m.handler = handlers.NewHandler(m.service, deps.Templates)

	tmplFS, _ := fs.Sub(templatesFS, "templates")
	deps.Templates.RegisterModule(m.ID(), tmplFS)

	return nil
}

// Migrate runs database migrations
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

// Routes returns the module routes
func (m *Module) Routes() chi.Router {
	r := chi.NewRouter()

	// Admin pages
	r.Get("/", m.handler.AdminList)
	r.Get("/new", m.handler.AdminNew)
	r.Post("/", m.handler.AdminCreate)
	r.Get("/{id}", m.handler.AdminEdit)
	r.Post("/{id}", m.handler.AdminUpdate)
	r.Delete("/{id}", m.handler.AdminDelete)
	r.Post("/{id}/toggle", m.handler.AdminToggle)
	r.Get("/{id}/usage", m.handler.AdminUsage)

	return r
}

// APIRoutes returns API routes for user-facing promo validation
func (m *Module) APIRoutes() chi.Router {
	r := chi.NewRouter()
	r.Post("/validate", m.handler.APIValidate)
	return r
}

// Templates returns the embedded templates
func (m *Module) Templates() fs.FS {
	tmplFS, _ := fs.Sub(templatesFS, "templates")
	return tmplFS
}

// MenuItems returns admin menu items
func (m *Module) MenuItems() []module.MenuItem {
	return []module.MenuItem{
		{
			Title:   "Promo Codes",
			Icon:    "bi-ticket-perforated",
			Path:    "/admin/promocodes",
			Order:   55,
			Section: module.MenuSectionAdmin,
			Group:   "Settings",
		},
	}
}

// Handler returns the handler for external access
func (m *Module) Handler() *handlers.Handler {
	return m.handler
}

// Service returns the service for external access
func (m *Module) Service() *services.PromoService {
	return m.service
}
