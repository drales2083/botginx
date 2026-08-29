package dashboard

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templatesFS embed.FS

// Module implements the dashboard feature
type Module struct {
	*module.BaseModule
	templates *module.TemplateEngine
}

func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"dashboard",
			"Dashboard",
			"Main dashboard with stats and overview",
		),
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)
	m.templates = deps.Templates

	// Register templates
	tmplFS, _ := fs.Sub(templatesFS, "templates")
	deps.Templates.RegisterModule(m.ID(), tmplFS)

	return nil
}

func (m *Module) Migrate() error {
	// No migrations for dashboard
	return nil
}

func (m *Module) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", m.handleDashboard)

	return r
}

func (m *Module) handleDashboard(w http.ResponseWriter, r *http.Request) {
	module.RenderUserSection(w, r, m.templates, "dashboard:index.html", map[string]interface{}{
		"Title": "Dashboard",
		"Stats": m.stats(ctx.GetUserID(r)),
	})
}

// stats counts what the dashboard boxes show. A failed query leaves its counter
// at zero rather than failing the page -- the dashboard is a summary, and one
// bad count is not worth an error screen.
func (m *Module) stats(userID string) map[string]interface{} {
	var redirectLinks, domains, live int

	m.DB().Get(&redirectLinks,
		`SELECT COUNT(*) FROM redirect_links WHERE user_id = $1`, userID)

	// Shared platform domains belong to the admin pool, not to this user.
	m.DB().Get(&domains,
		`SELECT COUNT(*) FROM domains WHERE user_id = $1 AND is_shared = FALSE`, userID)

	m.DB().Get(&live,
		`SELECT COUNT(*) FROM redirect_links WHERE user_id = $1 AND deploy_status = 'deployed'`, userID)

	return map[string]interface{}{
		"redirectLinks": redirectLinks,
		"domains":       domains,
		"live":          live,
	}
}

func (m *Module) Templates() fs.FS {
	tmplFS, _ := fs.Sub(templatesFS, "templates")
	return tmplFS
}

func (m *Module) MenuItems() []module.MenuItem {
	return []module.MenuItem{
		{
			Title:   "Dashboard",
			Icon:    "bi-speedometer2",
			Path:    "/user/dashboard",
			Order:   0,
			Section: module.MenuSectionUser,
		},
	}
}

func (m *Module) Widgets() []module.Widget {
	return nil
}
