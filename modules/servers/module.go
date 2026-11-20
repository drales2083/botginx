package servers

import (
	"embed"
	"io/fs"

	"github.com/botginx/botginx/modules/servers/handlers"
	"github.com/botginx/botginx/modules/servers/models"
	"github.com/botginx/botginx/modules/servers/services"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Module implements the servers feature
type Module struct {
	*module.BaseModule
	service          *services.ServerService
	handler          *handlers.Handler
	provisionService *services.ProvisionService
	provisionHandler *handlers.ProvisionHandler

	onServerIPChange func(newIP string) error
}

// SetOnServerIPChange registers a callback fired when a server's IP changes.
// Safe to call before or after Init.
func (m *Module) SetOnServerIPChange(fn func(string) error) {
	m.onServerIPChange = fn
}

func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"servers",
			"VPS Servers",
			"Manage VPS servers via SSH",
		),
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)

	// Initialize service
	m.service = services.NewServerService(deps.DB)

	// Initialize handler
	m.handler = handlers.NewHandler(m.service, deps.Templates)

	// Initialize provision service and handler
	m.provisionService = services.NewProvisionService(deps.DB, m.service)
	m.provisionHandler = handlers.NewProvisionHandler(m.provisionService, m.service, deps.Templates)

	m.handler.SetOnServerIPChange(func(ip string) error {
		if m.onServerIPChange == nil {
			return nil
		}
		return m.onServerIPChange(ip)
	})

	// Register templates
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
	if err != nil {
		return err
	}

	// Run provisions table migration
	sql2, err := fs.ReadFile(migrationsFS, "migrations/002_add_provisions.sql")
	if err == nil {
		m.DB().Exec(string(sql2))
	}

	return nil
}

func (m *Module) Routes() chi.Router {
	r := chi.NewRouter()

	// Pages
	r.Get("/", m.handler.List)
	r.Get("/new", m.handler.New)
	r.Get("/{id}", m.handler.Show)
	r.Get("/{id}/logs", m.handler.Logs)
	r.Get("/{id}/terminal", m.handler.Terminal)
	r.Get("/provision/{id}", m.provisionHandler.ProvisionPage)

	// API
	r.Route("/api", func(r chi.Router) {
		r.Get("/", m.handler.APIList)
		r.Post("/", m.handler.APICreate)
		r.Get("/{id}", m.handler.APIGet)
		r.Put("/{id}", m.handler.APIUpdate)
		r.Delete("/{id}", m.handler.APIDelete)
		r.Post("/{id}/test", m.handler.APITestConnection)
		r.Post("/{id}/exec", m.handler.APIExec)
	})

	// Provision API
	r.Route("/api/provision", func(r chi.Router) {
		r.Post("/", m.provisionHandler.APIStartProvision)
		r.Get("/{id}", m.provisionHandler.APIGetProvision)
		r.Get("/{id}/stream", m.provisionHandler.APIStreamProvision)
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
			Title:   "Servers",
			Icon:    "bi-server",
			Path:    "/admin/servers",
			Order:   10,
			Section: module.MenuSectionAdmin,
			Group:   "Admin Infrastructure",
		},
	}
}

// Service exposes the server service for other modules (random server pick)
func (m *Module) Service() *services.ServerService {
	return m.service
}

// PickRandom returns a server to deploy to. Delegated so other modules can
// depend on the module value at registration time, before Init has built the
// service.
func (m *Module) PickRandom() (*models.Server, error) {
	return m.service.PickRandom()
}

// GetServerForDomain returns SSH connection details for the server a domain is deployed to.
// Used by analytics to push settings files to the VPS.
func (m *Module) GetServerForDomain(domainID string) (ip string, port int, user, password string, err error) {
	return m.service.GetServerForDomain(domainID)
}

// GetDeployIP returns the IP of an available deploy server for DNS instructions.
func (m *Module) GetDeployIP() string {
	return m.service.GetDeployIP()
}

// GetAllDeployServers returns all ready servers for IP list push.
func (m *Module) GetAllDeployServers() ([]services.DeployServerInfo, error) {
	return m.service.GetAllDeployServers()
}

func (m *Module) Widgets() []module.Widget {
	return nil
}
