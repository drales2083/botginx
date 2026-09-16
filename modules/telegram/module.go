package telegram

import (
	"embed"
	"io/fs"
	"os"

	"github.com/botginx/botginx/modules/telegram/handlers"
	"github.com/botginx/botginx/modules/telegram/service"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Module struct {
	*module.BaseModule
	Handler *handlers.Handler
	Service *service.Service
}

func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"telegram",
			"Telegram",
			"Telegram bot integration for notifications and replies",
		),
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)

	m.Service = service.NewService(deps.DB)
	m.Handler = handlers.NewHandler(deps.DB, deps.Templates, m.Service)

	tmplFS, _ := fs.Sub(templatesFS, "templates")
	deps.Templates.RegisterModule(m.ID(), tmplFS)

	return nil
}

func (m *Module) PostInit() error {
	// Migrate existing env vars to database on first run
	m.migrateEnvVars()
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

func (m *Module) Routes() chi.Router {
	r := chi.NewRouter()
	// Webhook endpoint (no auth - uses secret in URL)
	r.Post("/webhook/{botID}/{secret}", m.Handler.Webhook)
	return r
}

func (m *Module) RoutesForSection(section module.MenuSection) chi.Router {
	if section == module.MenuSectionAdmin {
		r := chi.NewRouter()
		// Admin settings pages
		r.Get("/", m.Handler.Settings)
		// Admin API
		r.Get("/api/bots", m.Handler.APIListBots)
		r.Post("/api/bots", m.Handler.APICreateBot)
		r.Put("/api/bots/{id}", m.Handler.APIUpdateBot)
		r.Delete("/api/bots/{id}", m.Handler.APIDeleteBot)
		r.Post("/api/bots/{id}/test", m.Handler.APITestBot)
		r.Post("/api/bots/{id}/webhook", m.Handler.APISetupWebhook)
		r.Get("/api/bots/{id}/chats", m.Handler.APIGetChats)
		return r
	}
	return m.Routes()
}

func (m *Module) MenuItems() []module.MenuItem {
	return []module.MenuItem{
		{
			Title:   "Telegram",
			Icon:    "bi-telegram",
			Path:    "/admin/telegram",
			Order:   95,
			Section: module.MenuSectionAdmin,
		},
	}
}

// migrateEnvVars migrates existing environment variables to database
func (m *Module) migrateEnvVars() {
	// Check if we already have bots configured
	var count int
	m.DB().Get(&count, "SELECT COUNT(*) FROM telegram_bots")
	if count > 0 {
		return // Already migrated
	}

	// Migrate support bot
	supportToken := os.Getenv("TG_SUPPORT_BOT_TOKEN")
	supportChat := os.Getenv("TG_SUPPORT_CHAT_ID")
	if supportToken != "" && supportChat != "" {
		_, err := m.DB().Exec(`
			INSERT INTO telegram_bots (name, bot_token, chat_id, use_for_support, enabled)
			VALUES ('Support', $1, $2, TRUE, TRUE)
		`, supportToken, supportChat)
		if err != nil {
			log.Error().Err(err).Msg("Failed to migrate support bot")
		} else {
			log.Info().Msg("Migrated TG_SUPPORT_BOT to database")
		}
	}

	// Migrate deploy bot
	deployToken := os.Getenv("TG_DEPLOY_BOT_TOKEN")
	deployChat := os.Getenv("TG_DEPLOY_CHAT_ID")
	if deployToken != "" && deployChat != "" {
		_, err := m.DB().Exec(`
			INSERT INTO telegram_bots (name, bot_token, chat_id, use_for_support, enabled)
			VALUES ('Deploy', $1, $2, FALSE, TRUE)
		`, deployToken, deployChat)
		if err != nil {
			log.Error().Err(err).Msg("Failed to migrate deploy bot")
		} else {
			log.Info().Msg("Migrated TG_DEPLOY_BOT to database")
		}
	}
}
