package users

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"

	"github.com/botginx/botginx/modules/users/handlers"
	"github.com/botginx/botginx/modules/users/services"
	"github.com/botginx/botginx/pkg/module"
	"github.com/botginx/botginx/pkg/subscription"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Module struct {
	*module.BaseModule
	service       *services.UserService
	subscriptions *subscription.Service
	handler       *handlers.Handler
}

func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"users",
			"Users",
			"User management (admin only)",
		),
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)

	m.service = services.NewUserService(deps.DB)
	m.subscriptions = subscription.NewService(deps.DB)
	m.handler = handlers.NewHandler(m.service, m.subscriptions, deps.Templates)

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

		// Subscriptions
		r.Post("/{id}/subscription", m.handler.APIGrantSubscription)
		r.Delete("/{id}/subscription", m.handler.APIRevokeSubscription)
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
			Title:   "Users",
			Icon:    "bi-people",
			Path:    "/admin/users",
			Order:   100,
			Section: module.MenuSectionAdmin,
		},
	}
}

func (m *Module) Widgets() []module.Widget {
	return []module.Widget{
		{
			ID:    "users-count",
			Title: "Total Users",
			Icon:  "bi-people",
			Size:  "small",
			Order: 10,
			Data: func() (interface{}, error) {
				count, err := m.service.Count()
				return map[string]interface{}{
					"count": count,
					"label": "Users",
				}, err
			},
		},
	}
}
