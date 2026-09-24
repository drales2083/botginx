package help

import (
	"embed"
	"io/fs"

	"github.com/botginx/botginx/modules/help/handlers"
	"github.com/botginx/botginx/modules/help/services"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Module struct {
	*module.BaseModule
	service *services.HelpService
	handler *handlers.Handler
}

func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"help",
			"Help",
			"FAQ and help documentation",
		),
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)

	m.service = services.NewHelpService(deps.DB)
	m.handler = handlers.NewHandler(m.service, deps.Templates)

	tmplFS, _ := fs.Sub(templatesFS, "templates")
	deps.Templates.RegisterModule(m.ID(), tmplFS)

	return nil
}

func (m *Module) Migrate() error {
	// Run all migration files in order
	files := []string{
		"migrations/001_create_tables.sql",
		"migrations/002_add_template_faqs.sql",
	}
	for _, file := range files {
		sql, err := fs.ReadFile(migrationsFS, file)
		if err != nil {
			return err
		}
		if _, err = m.DB().Exec(string(sql)); err != nil {
			return err
		}
	}
	return nil
}

func (m *Module) Routes() chi.Router {
	r := chi.NewRouter()

	// User pages
	r.Get("/", m.handler.Index)

	return r
}

func (m *Module) AdminRoutes() chi.Router {
	r := chi.NewRouter()

	// Admin pages
	r.Get("/", m.handler.AdminList)

	// API
	r.Route("/api", func(r chi.Router) {
		// Categories
		r.Get("/categories", m.handler.APIListCategories)
		r.Post("/categories", m.handler.APICreateCategory)
		r.Put("/categories/{id}", m.handler.APIUpdateCategory)
		r.Delete("/categories/{id}", m.handler.APIDeleteCategory)

		// Items
		r.Get("/items", m.handler.APIListItems)
		r.Post("/items", m.handler.APICreateItem)
		r.Get("/items/{id}", m.handler.APIGetItem)
		r.Put("/items/{id}", m.handler.APIUpdateItem)
		r.Delete("/items/{id}", m.handler.APIDeleteItem)
	})

	return r
}

// MenuItems returns sidebar navigation items for this module
func (m *Module) MenuItems() []module.MenuItem {
	return []module.MenuItem{
		{
			Title:   "FAQ",
			Icon:    "bi-question-circle",
			Path:    "/user/help",
			Order:   86,
			Section: module.MenuSectionUser,
			Group:   "Help",
		},
	}
}

// AdminMenuItems returns admin sidebar navigation items
func (m *Module) AdminMenuItems() []module.MenuItem {
	return []module.MenuItem{
		{
			Title:   "FAQs",
			Icon:    "bi-question-circle",
			Path:    "/admin/help",
			Section: module.MenuSectionAdmin,
		},
	}
}
