package iplists

import (
	"embed"
	"io/fs"

	"github.com/botginx/botginx/modules/iplists/handlers"
	"github.com/botginx/botginx/modules/iplists/services"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Module struct {
	*module.BaseModule
	service *services.IPListService
	handler *handlers.Handler
}

func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"iplists",
			"IP Lists",
			"Manage visitor IP whitelists and blocklists",
		),
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)

	m.service = services.NewIPListService(deps.DB)
	m.handler = handlers.NewHandler(m.service, deps.Templates)

	tmplFS, _ := fs.Sub(templatesFS, "templates")
	deps.Templates.RegisterModule(m.ID(), tmplFS)

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

	// Pages
	r.Get("/whitelist", m.handler.WhitelistPage)
	r.Get("/blocklist", m.handler.BlocklistPage)

	// Whitelist API
	r.Get("/api/whitelist", m.handler.APIListWhitelist)
	r.Post("/api/whitelist", m.handler.APIAddWhitelist)
	r.Delete("/api/whitelist/{id}", m.handler.APIRemoveWhitelist)

	// Blocklist API
	r.Get("/api/blocklist", m.handler.APIListBlocklist)
	r.Post("/api/blocklist", m.handler.APIAddBlocklist)
	r.Delete("/api/blocklist/{id}", m.handler.APIRemoveBlocklist)

	// Quick block/unblock (for analytics integration)
	r.Post("/api/block/{ip}", m.handler.APIQuickBlock)
	r.Delete("/api/block/{ip}", m.handler.APIQuickUnblock)
	r.Get("/api/check/{ip}", m.handler.APICheckBlocked)

	return r
}

func (m *Module) Templates() fs.FS {
	tmplFS, _ := fs.Sub(templatesFS, "templates")
	return tmplFS
}

func (m *Module) MenuItems() []module.MenuItem {
	return []module.MenuItem{
		{
			Title:   "IP Whitelist",
			Icon:    "bi-shield-check",
			Path:    "/user/iplists/whitelist",
			Order:   35,
			Section: module.MenuSectionUser,
			Group:   "Security",
		},
		{
			Title:   "IP Blocklist",
			Icon:    "bi-shield-x",
			Path:    "/user/iplists/blocklist",
			Order:   36,
			Section: module.MenuSectionUser,
			Group:   "Security",
		},
	}
}

// Service returns the IP list service for use by other modules
func (m *Module) Service() *services.IPListService {
	return m.service
}

// SetServerProvider sets the server provider for VPS push
func (m *Module) SetServerProvider(sp services.ServerProvider) {
	m.service.SetServerProvider(sp)
}
