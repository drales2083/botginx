package apikeys

import (
	"embed"
	"io/fs"

	"github.com/botginx/botginx/modules/apikeys/handlers"
	"github.com/botginx/botginx/modules/apikeys/services"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templatesFS embed.FS

type Module struct {
	*module.BaseModule
	service *services.APIKeysService
	handler *handlers.Handler
}

func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"apikeys",
			"API Keys",
			"Manage API keys for external integrations",
		),
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)

	m.service = services.NewAPIKeysService(deps.DB)
	m.handler = handlers.NewHandler(m.service, deps.Templates)

	tmplFS, _ := fs.Sub(templatesFS, "templates")
	deps.Templates.RegisterModule(m.ID(), tmplFS)

	return nil
}

func (m *Module) Migrate() error {
	_, err := m.DB().Exec(`
		CREATE TABLE IF NOT EXISTS api_keys (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			key_hash TEXT NOT NULL UNIQUE,
			key_prefix TEXT NOT NULL,
			scopes TEXT[] NOT NULL DEFAULT '{}',
			expires_at TIMESTAMP,
			rate_limit INT DEFAULT 1000,
			is_test BOOLEAN DEFAULT false,
			is_active BOOLEAN DEFAULT true,
			last_used_at TIMESTAMP,
			requests_count BIGINT DEFAULT 0,
			created_at TIMESTAMP DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id);
		CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys(key_hash);
	`)
	return err
}

func (m *Module) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", m.handler.Index)

	r.Route("/api", func(r chi.Router) {
		r.Get("/keys", m.handler.APIList)
		r.Post("/keys", m.handler.APICreate)
		r.Get("/keys/{id}", m.handler.APIGet)
		r.Delete("/keys/{id}", m.handler.APIDelete)
		r.Post("/keys/{id}/revoke", m.handler.APIRevoke)
		r.Post("/keys/{id}/rotate", m.handler.APIRotate)
		r.Get("/scopes", m.handler.APIScopes)
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
			Title:   "API Keys",
			Icon:    "bi-key",
			Path:    "/user/apikeys",
			Order:   50,
			Section: module.MenuSectionUser,
			Group:   "Account",
		},
	}
}

func (m *Module) Widgets() []module.Widget {
	return nil
}

// Service returns the service for external API authentication
func (m *Module) Service() *services.APIKeysService {
	return m.service
}
