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
	userID := ctx.GetUserID(r)
	module.RenderUserSection(w, r, m.templates, "dashboard:index.html", map[string]interface{}{
		"Title":   "Dashboard",
		"Stats":   m.stats(userID),
		"Domains": m.recentDomains(userID),
		"Links":   m.recentLinks(userID),
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

type recentDomain struct {
	ID        string `db:"id"`
	Name      string `db:"name"`
	SSLStatus string `db:"ssl_status"`
}

func (m *Module) recentDomains(userID string) []recentDomain {
	var domains []recentDomain
	m.DB().Select(&domains, `
		SELECT id, name, ssl_status
		FROM domains
		WHERE user_id = $1 AND is_shared = FALSE
		ORDER BY created_at DESC
		LIMIT 5
	`, userID)
	return domains
}

type recentLink struct {
	ID           string `db:"id"`
	Subdomain    string `db:"subdomain"`
	DomainName   string `db:"domain_name"`
	Path         string `db:"path"`
	DeployStatus string `db:"deploy_status"`
}

func (m *Module) recentLinks(userID string) []recentLink {
	var links []recentLink
	m.DB().Select(&links, `
		SELECT r.id, r.subdomain, REGEXP_REPLACE(d.name, '^\*\.', '') as domain_name, r.path, r.deploy_status
		FROM redirect_links r
		JOIN domains d ON d.id = r.domain_id
		WHERE r.user_id = $1
		ORDER BY r.created_at DESC
		LIMIT 5
	`, userID)
	return links
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
