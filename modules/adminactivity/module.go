package adminactivity

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/botginx/botginx/pkg/adminlog"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Module struct {
	*module.BaseModule
	templates   *module.TemplateEngine
	adminLogger *adminlog.Logger
	db          *sqlx.DB
}

func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"adminactivity",
			"Admin Activity",
			"Admin activity audit logs",
		),
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)
	m.templates = deps.Templates
	m.db = deps.DB
	m.adminLogger = adminlog.NewLogger(deps.DB)

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

func (m *Module) RoutesForSection(section module.MenuSection) chi.Router {
	if section == module.MenuSectionAdmin {
		r := chi.NewRouter()
		r.Get("/", m.handleList)
		return r
	}
	return nil
}

func (m *Module) Routes() chi.Router {
	return chi.NewRouter()
}

func (m *Module) handleList(w http.ResponseWriter, r *http.Request) {
	filterAdmin := r.URL.Query().Get("admin")
	filterAction := r.URL.Query().Get("action")
	filterTarget := r.URL.Query().Get("target")

	logs, _ := m.adminLogger.GetLogs(100, filterAdmin, filterAction, filterTarget)

	// Get list of admins for filter dropdown
	var admins []struct {
		Email string `db:"email"`
	}
	m.db.Select(&admins, `SELECT email FROM users WHERE role = 'admin' OR role = 'superadmin' ORDER BY email`)

	module.Render(w, r, m.templates, "adminactivity:index.html", map[string]interface{}{
		"Title":        "Admin Activity Logs",
		"Logs":         logs,
		"Admins":       admins,
		"FilterAdmin":  filterAdmin,
		"FilterAction": filterAction,
		"FilterTarget": filterTarget,
	})
}

func (m *Module) Templates() fs.FS {
	tmplFS, _ := fs.Sub(templatesFS, "templates")
	return tmplFS
}

func (m *Module) MenuItems() []module.MenuItem {
	return []module.MenuItem{
		{
			Title:   "Activity Logs",
			Icon:    "bi-activity",
			Path:    "/admin/adminactivity",
			Order:   99,
			Section: module.MenuSectionAdmin,
			Group:   "System",
		},
	}
}

// Logger returns the admin activity logger for use by other modules
func (m *Module) Logger() *adminlog.Logger {
	return m.adminLogger
}
