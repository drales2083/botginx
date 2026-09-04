package dashboard

import (
	"embed"
	"encoding/json"
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

	// API endpoints for charts
	r.Get("/api/timeline", m.apiTimeline)
	r.Get("/api/countries", m.apiCountries)
	r.Get("/api/visitors", m.apiVisitors)

	return r
}

func (m *Module) handleDashboard(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	module.RenderUserSection(w, r, m.templates, "dashboard:index.html", map[string]interface{}{
		"Title":   "Dashboard",
		"Stats":   m.stats(userID),
		"Domains": m.recentDomains(userID),
		"Links":   m.recentLinks(userID),
		"Hosting": m.hostingStats(userID),
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

type hostingAccount struct {
	ID          string  `db:"id"`
	PackageName string  `db:"package_name"`
	Status      string  `db:"status"`
	DomainCount int     `db:"domain_count"`
	Price       float64 `db:"price"`
}

func (m *Module) hostingStats(userID string) map[string]interface{} {
	var accountCount, activeCount, hostingDomains, serverCount int
	var balance float64

	// Check if hosting feature is available (any servers exist)
	m.DB().Get(&serverCount,
		`SELECT COUNT(*) FROM hosting_servers WHERE is_active = true`)

	// Count hosting accounts
	m.DB().Get(&accountCount,
		`SELECT COUNT(*) FROM hosting_accounts WHERE user_id = $1`, userID)

	// Count active accounts
	m.DB().Get(&activeCount,
		`SELECT COUNT(*) FROM hosting_accounts WHERE user_id = $1 AND status = 'active'`, userID)

	// Count hosting domains
	m.DB().Get(&hostingDomains,
		`SELECT COUNT(*) FROM hosting_domains hd
		 JOIN hosting_accounts ha ON ha.id = hd.account_id
		 WHERE ha.user_id = $1`, userID)

	// Get balance
	m.DB().Get(&balance,
		`SELECT COALESCE(balance, 0) FROM users WHERE id = $1`, userID)

	// Get recent accounts
	var accounts []hostingAccount
	m.DB().Select(&accounts, `
		SELECT ha.id, hp.name as package_name, ha.status,
		       (SELECT COUNT(*) FROM hosting_domains WHERE account_id = ha.id) as domain_count,
		       hp.price_monthly as price
		FROM hosting_accounts ha
		JOIN hosting_packages hp ON hp.id = ha.package_id
		WHERE ha.user_id = $1
		ORDER BY ha.created_at DESC
		LIMIT 3
	`, userID)

	return map[string]interface{}{
		"available":      serverCount > 0,
		"accountCount":   accountCount,
		"activeCount":    activeCount,
		"hostingDomains": hostingDomains,
		"balance":        balance,
		"accounts":       accounts,
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

// API handlers for dashboard charts

type timelinePoint struct {
	Time    string `db:"time_bucket" json:"time"`
	Total   int    `db:"total" json:"total"`
	Blocked int    `db:"blocked" json:"blocked"`
	Unique  int    `db:"unique_count" json:"unique"`
}

func (m *Module) apiTimeline(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "hourly"
	}

	var query string
	switch period {
	case "hourly":
		query = `
			SELECT TO_CHAR(created_at, 'YYYY-MM-DD HH24:00') as time_bucket,
				COUNT(*) as total,
				COUNT(*) FILTER (WHERE blocked = true) as blocked,
				COUNT(*) FILTER (WHERE is_unique = true) as unique_count
			FROM visits
			WHERE user_id = $1 AND created_at > NOW() - INTERVAL '24 hours'
			GROUP BY time_bucket
			ORDER BY time_bucket
		`
	case "daily":
		query = `
			SELECT TO_CHAR(created_at, 'YYYY-MM-DD') as time_bucket,
				COUNT(*) as total,
				COUNT(*) FILTER (WHERE blocked = true) as blocked,
				COUNT(*) FILTER (WHERE is_unique = true) as unique_count
			FROM visits
			WHERE user_id = $1 AND created_at > NOW() - INTERVAL '7 days'
			GROUP BY time_bucket
			ORDER BY time_bucket
		`
	case "monthly":
		query = `
			SELECT TO_CHAR(created_at, 'YYYY-MM') as time_bucket,
				COUNT(*) as total,
				COUNT(*) FILTER (WHERE blocked = true) as blocked,
				COUNT(*) FILTER (WHERE is_unique = true) as unique_count
			FROM visits
			WHERE user_id = $1 AND created_at > NOW() - INTERVAL '12 months'
			GROUP BY time_bucket
			ORDER BY time_bucket
		`
	default:
		query = `
			SELECT TO_CHAR(created_at, 'YYYY-MM-DD HH24:00') as time_bucket,
				COUNT(*) as total,
				COUNT(*) FILTER (WHERE blocked = true) as blocked,
				COUNT(*) FILTER (WHERE is_unique = true) as unique_count
			FROM visits
			WHERE user_id = $1 AND created_at > NOW() - INTERVAL '24 hours'
			GROUP BY time_bucket
			ORDER BY time_bucket
		`
	}

	var timeline []timelinePoint
	m.DB().Select(&timeline, query, userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(timeline)
}

type countryStats struct {
	Country string `db:"country" json:"country"`
	Count   int    `db:"count" json:"count"`
}

func (m *Module) apiCountries(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var countries []countryStats
	m.DB().Select(&countries, `
		SELECT country, COUNT(*) as count
		FROM visits
		WHERE user_id = $1 AND country != ''
		GROUP BY country
		ORDER BY count DESC
		LIMIT 20
	`, userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(countries)
}

type visitorPoint struct {
	Lat     float64 `db:"latitude" json:"lat"`
	Lng     float64 `db:"longitude" json:"lng"`
	Country string  `db:"country" json:"country"`
	City    string  `db:"city" json:"city"`
	Blocked bool    `db:"blocked" json:"blocked"`
}

func (m *Module) apiVisitors(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var points []visitorPoint
	m.DB().Select(&points, `
		SELECT latitude, longitude, country, city, blocked
		FROM visits
		WHERE user_id = $1 AND latitude != 0 AND longitude != 0
		ORDER BY created_at DESC
		LIMIT 500
	`, userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(points)
}
