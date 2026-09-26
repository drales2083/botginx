package antibotcontrol

import (
	"embed"
	"io/fs"

	"github.com/botginx/botginx/modules/antibotcontrol/handlers"
	"github.com/botginx/botginx/modules/antibotcontrol/services"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templatesFS embed.FS

type Module struct {
	*module.BaseModule
	service *services.AntibotControlService
	handler *handlers.Handler
}

func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"antibotcontrol",
			"Antibot Control",
			"Configure default bot protection settings for new links",
		),
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)

	m.service = services.NewAntibotControlService(deps.DB)
	m.handler = handlers.NewHandler(m.service, deps.Templates)

	tmplFS, _ := fs.Sub(templatesFS, "templates")
	deps.Templates.RegisterModule(m.ID(), tmplFS)

	return nil
}

func (m *Module) Migrate() error {
	_, err := m.DB().Exec(`
		CREATE TABLE IF NOT EXISTS user_antibot_defaults (
			user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
			settings JSONB NOT NULL DEFAULT '{}',
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		)
	`)
	return err
}

func (m *Module) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", m.handler.Index)

	r.Route("/api", func(r chi.Router) {
		r.Get("/defaults", m.handler.APIGetDefaults)
		r.Post("/defaults", m.handler.APISaveDefaults)
		r.Delete("/defaults", m.handler.APIResetDefaults)
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
			Title:   "Antibot Control",
			Icon:    "bi-shield-check",
			Path:    "/user/antibotcontrol",
			Order:   15,
			Section: module.MenuSectionUser,
			Group:   "Settings",
		},
	}
}

func (m *Module) Widgets() []module.Widget {
	return nil
}

// Service returns the service for use by other modules (e.g., redirect links)
func (m *Module) Service() *services.AntibotControlService {
	return m.service
}
