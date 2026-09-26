package webhooks

import (
	"embed"
	"io/fs"

	"github.com/botginx/botginx/modules/webhooks/handlers"
	"github.com/botginx/botginx/modules/webhooks/services"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templatesFS embed.FS

type Module struct {
	*module.BaseModule
	service    *services.WebhooksService
	dispatcher *services.Dispatcher
	handler    *handlers.Handler
}

func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"webhooks",
			"Webhooks",
			"Webhook subscriptions for event notifications",
		),
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)

	m.service = services.NewWebhooksService(deps.DB)
	m.dispatcher = services.NewDispatcher(deps.DB, m.service)
	m.handler = handlers.NewHandler(m.service, m.dispatcher, deps.Templates)

	tmplFS, _ := fs.Sub(templatesFS, "templates")
	deps.Templates.RegisterModule(m.ID(), tmplFS)

	return nil
}

func (m *Module) Migrate() error {
	_, err := m.DB().Exec(`
		CREATE TABLE IF NOT EXISTS webhooks (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			url TEXT NOT NULL,
			events TEXT[] NOT NULL DEFAULT '{}',
			secret_hash TEXT,
			is_active BOOLEAN DEFAULT true,
			last_triggered_at TIMESTAMP,
			failure_count INT DEFAULT 0,
			created_at TIMESTAMP DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS webhook_deliveries (
			id TEXT PRIMARY KEY,
			webhook_id TEXT NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
			event_type TEXT NOT NULL,
			payload JSONB NOT NULL,
			response_status INT,
			response_body TEXT,
			delivered_at TIMESTAMP DEFAULT NOW(),
			success BOOLEAN DEFAULT false
		);

		CREATE INDEX IF NOT EXISTS idx_webhooks_user_id ON webhooks(user_id);
		CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_webhook_id ON webhook_deliveries(webhook_id);
	`)
	return err
}

func (m *Module) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", m.handler.Index)

	r.Route("/api", func(r chi.Router) {
		r.Get("/webhooks", m.handler.APIList)
		r.Post("/webhooks", m.handler.APICreate)
		r.Get("/webhooks/{id}", m.handler.APIGet)
		r.Put("/webhooks/{id}", m.handler.APIUpdate)
		r.Delete("/webhooks/{id}", m.handler.APIDelete)
		r.Get("/webhooks/{id}/deliveries", m.handler.APIDeliveries)
		r.Post("/webhooks/{id}/test", m.handler.APITest)
		r.Get("/events", m.handler.APIEvents)
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
			Title:   "Webhooks",
			Icon:    "bi-broadcast",
			Path:    "/user/webhooks",
			Order:   55,
			Section: module.MenuSectionUser,
			Group:   "Security",
		},
	}
}

func (m *Module) Widgets() []module.Widget {
	return nil
}

func (m *Module) Dispatcher() *services.Dispatcher {
	return m.dispatcher
}

func (m *Module) Service() *services.WebhooksService {
	return m.service
}
