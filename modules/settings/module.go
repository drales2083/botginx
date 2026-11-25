package settings

import (
	"embed"
	"io/fs"

	"github.com/botginx/botginx/modules/settings/handlers"
	"github.com/botginx/botginx/modules/telegram"
	"github.com/botginx/botginx/pkg/banner"
	"github.com/botginx/botginx/pkg/module"
	"github.com/botginx/botginx/pkg/proxy"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templatesFS embed.FS

type Module struct {
	*module.BaseModule
	handler        *handlers.Handler
	telegramModule *telegram.Module
	ProxyService   *proxy.Service
	BannerService  *banner.Service
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

	// Initialize services
	m.ProxyService = proxy.NewService(deps.DB)
	// BannerService is set externally via SetBannerService (created early for template function)
	if m.BannerService == nil {
		m.BannerService = banner.NewService(deps.DB)
	}

	m.handler = handlers.NewHandler(deps.DB, deps.Templates, m.telegramModule, m.ProxyService, m.BannerService)

	tmplFS, _ := fs.Sub(templatesFS, "templates")
	deps.Templates.RegisterModule(m.ID(), tmplFS)

	return nil
}

// SetBannerService sets the banner service (must be called before Init if sharing with templates)
func (m *Module) SetBannerService(svc *banner.Service) {
	m.BannerService = svc
}

func (m *Module) Migrate() error {
	// Ensure tables exist
	if err := m.ProxyService.EnsureTable(); err != nil {
		return err
	}
	return m.BannerService.EnsureTable()
}

func (m *Module) RoutesForSection(section module.MenuSection) chi.Router {
	if section == module.MenuSectionAdmin {
		r := chi.NewRouter()

		// Main settings page
		r.Get("/", m.handler.Index)

		// System banner settings
		r.Route("/banner", func(r chi.Router) {
			r.Get("/", m.handler.Banner)
			r.Get("/api/config", m.handler.APIGetBanner)
			r.Put("/api/config", m.handler.APIUpdateBanner)
		})

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

		// Proxy settings section
		r.Route("/proxy", func(r chi.Router) {
			r.Get("/", m.handler.Proxy)
			r.Get("/api/config", m.handler.APIGetProxy)
			r.Put("/api/config", m.handler.APIUpdateProxy)
			r.Post("/api/test", m.handler.APITestProxy)
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
