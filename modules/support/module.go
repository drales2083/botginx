package support

import (
	"embed"
	"io/fs"

	"github.com/botginx/botginx/modules/support/handlers"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Module struct {
	*module.BaseModule
	handler *handlers.Handler
}

func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"support",
			"Support",
			"Support ticket system",
		),
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)

	m.handler = handlers.NewHandler(deps.DB, deps.Templates)

	tmplFS, _ := fs.Sub(templatesFS, "templates")
	deps.Templates.RegisterModule(m.ID(), tmplFS)

	return nil
}

func (m *Module) Migrate() error {
	sql, err := fs.ReadFile(migrationsFS, "migrations/001_create_tickets.sql")
	if err != nil {
		return err
	}
	_, err = m.DB().Exec(string(sql))
	return err
}

func (m *Module) Routes() chi.Router {
	r := chi.NewRouter()

	// User pages
	r.Get("/", m.handler.List)
	r.Get("/new", m.handler.New)
	r.Get("/{id}", m.handler.Show)

	// API
	r.Post("/api", m.handler.APICreate)
	r.Post("/api/{id}/reply", m.handler.APIReply)
	r.Post("/api/{id}/close", m.handler.APIClose)

	return r
}

func (m *Module) RoutesForSection(section module.MenuSection) chi.Router {
	if section == module.MenuSectionAdmin {
		r := chi.NewRouter()
		r.Get("/", m.handler.AdminList)
		r.Get("/{id}", m.handler.AdminShow)
		r.Post("/api/{id}/reply", m.handler.APIAdminReply)
		r.Post("/api/{id}/close", m.handler.APIAdminClose)
		r.Post("/api/{id}/reopen", m.handler.APIAdminReopen)
		return r
	}
	return m.Routes()
}

// SetNotifier sets the notification handler for ticket events
func (m *Module) SetNotifier(n handlers.Notifier) {
	m.handler.SetNotifier(n)
}

// MenuItems returns sidebar navigation items for this module
func (m *Module) MenuItems() []module.MenuItem {
	return []module.MenuItem{
		{
			Title:   "Support",
			Icon:    "bi-headset",
			Path:    "/user/support",
			Order:   85,
			Section: module.MenuSectionUser,
			Group:   "Help",
		},
		{
			Title:   "Support",
			Icon:    "bi-headset",
			Path:    "/admin/support",
			Order:   20,
			Section: module.MenuSectionAdmin,
			Group:   "Users & Support",
		},
	}
}
