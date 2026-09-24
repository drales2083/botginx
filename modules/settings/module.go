package settings

import (
	"embed"
	"io/fs"

	"github.com/botginx/botginx/modules/settings/handlers"
	"github.com/botginx/botginx/modules/telegram"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templatesFS embed.FS

type Module struct {
	*module.BaseModule
	handler        *handlers.Handler
	telegramModule *telegram.Module
}

func New(telegramModule *telegram.Module) *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"settings",
			"Settings",
			"System settings and configuration",
		),
		telegramModule: telegramModule,
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)

	m.handler = handlers.NewHandler(deps.DB, deps.Templates, m.telegramModule)

	tmplFS, _ := fs.Sub(templatesFS, "templates")
	deps.Templates.RegisterModule(m.ID(), tmplFS)

	return nil
}

func (m *Module) Migrate() error {
	// No migrations - settings module uses other modules' tables
	return nil
}

func (m *Module) RoutesForSection(section module.MenuSection) chi.Router {
	if section == module.MenuSectionAdmin {
		r := chi.NewRouter()

		// Main settings page
		r.Get("/", m.handler.Index)

		// Telegram settings section
		r.Route("/telegram", func(r chi.Router) {
			r.Get("/", m.handler.Telegram)
			r.Get("/api/bots", m.telegramModule.Handler.APIListBots)
			r.Post("/api/bots", m.telegramModule.Handler.APICreateBot)
			r.Put("/api/bots/{id}", m.telegramModule.Handler.APIUpdateBot)
			r.Delete("/api/bots/{id}", m.telegramModule.Handler.APIDeleteBot)
			r.Post("/api/bots/{id}/test", m.telegramModule.Handler.APITestBot)
			r.Post("/api/bots/{id}/webhook", m.telegramModule.Handler.APISetupWebhook)
			r.Get("/api/bots/{id}/chats", m.telegramModule.Handler.APIGetChats)
		})

		return r
	}
	return chi.NewRouter()
}

func (m *Module) Routes() chi.Router {
	return chi.NewRouter()
}

func (m *Module) MenuItems() []module.MenuItem {
	return []module.MenuItem{
		{
			Title:   "Settings",
			Icon:    "bi-gear",
			Path:    "/admin/settings",
			Order:   30,
			Section: module.MenuSectionAdmin,
			Group:   "System",
		},
	}
}
