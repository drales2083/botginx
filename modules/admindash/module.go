package admindash

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"

	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templatesFS embed.FS

type Module struct {
	*module.BaseModule
	templates *module.TemplateEngine
}

func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"admindash",
			"Admin Dashboard",
			"Platform-wide statistics and overview",
		),
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)
	m.templates = deps.Templates

	tmplFS, _ := fs.Sub(templatesFS, "templates")
	deps.Templates.RegisterModule(m.ID(), tmplFS)

	return nil
}

func (m *Module) Migrate() error {
	return nil
}

func (m *Module) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", m.handleDashboard)
	r.Get("/api/timeline", m.apiTimeline)
	r.Get("/api/countries", m.apiCountries)
	r.Get("/domains/all", m.handleAllDomains)
	return r
}

func (m *Module) handleDashboard(w http.ResponseWriter, r *http.Request) {
	module.Render(w, r, m.templates, "admindash:index.html", map[string]interface{}{
		"Title": "Admin Dashboard",
		"Stats": m.stats(),
	})
}

func (m *Module) stats() map[string]interface{} {
	var totalUsers, activeUsers, totalDomains, sharedDomains int
	var totalLinks, deployedLinks, totalVisits, todayVisits int
	var blockedVisits, uniqueVisits int

	// Users
	m.DB().Get(&totalUsers, `SELECT COUNT(*) FROM users`)
	m.DB().Get(&activeUsers, `SELECT COUNT(*) FROM users WHERE subscription_expires_at > NOW()`)

	// Domains
	m.DB().Get(&totalDomains, `SELECT COUNT(*) FROM domains`)
	m.DB().Get(&sharedDomains, `SELECT COUNT(*) FROM domains WHERE is_shared = true`)

	// Links
	m.DB().Get(&totalLinks, `SELECT COUNT(*) FROM redirect_links`)
	m.DB().Get(&deployedLinks, `SELECT COUNT(*) FROM redirect_links WHERE deploy_status = 'deployed'`)

	// Visits
	m.DB().Get(&totalVisits, `SELECT COUNT(*) FROM visits`)
	m.DB().Get(&todayVisits, `SELECT COUNT(*) FROM visits WHERE created_at > CURRENT_DATE`)
	m.DB().Get(&blockedVisits, `SELECT COUNT(*) FROM visits WHERE blocked = true`)
	m.DB().Get(&uniqueVisits, `SELECT COUNT(DISTINCT ip) FROM visits`)

	return map[string]interface{}{
		"totalUsers":    totalUsers,
		"activeUsers":   activeUsers,
		"totalDomains":  totalDomains,
		"sharedDomains": sharedDomains,
		"totalLinks":    totalLinks,
		"deployedLinks": deployedLinks,
		"totalVisits":   totalVisits,
		"todayVisits":   todayVisits,
		"blockedVisits": blockedVisits,
		"uniqueVisits":  uniqueVisits,
	}
}

type timelinePoint struct {
	Time    string `db:"time_bucket" json:"time"`
	Total   int    `db:"total" json:"total"`
	Blocked int    `db:"blocked" json:"blocked"`
	Unique  int    `db:"unique_count" json:"unique"`
}

func (m *Module) apiTimeline(w http.ResponseWriter, r *http.Request) {
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
			WHERE created_at > NOW() - INTERVAL '24 hours'
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
			WHERE created_at > NOW() - INTERVAL '7 days'
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
			WHERE created_at > NOW() - INTERVAL '12 months'
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
			WHERE created_at > NOW() - INTERVAL '24 hours'
			GROUP BY time_bucket
			ORDER BY time_bucket
		`
	}

	var timeline []timelinePoint
	m.DB().Select(&timeline, query)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(timeline)
}

type countryStats struct {
	Country string `db:"country" json:"country"`
	Count   int    `db:"count" json:"count"`
}

func (m *Module) apiCountries(w http.ResponseWriter, r *http.Request) {
	var countries []countryStats
	m.DB().Select(&countries, `
		SELECT country, COUNT(*) as count
		FROM visits
		WHERE country != ''
		GROUP BY country
		ORDER BY count DESC
		LIMIT 20
	`)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(countries)
}

type domainRow struct {
	ID               string  `db:"id"`
	Name             string  `db:"name"`
	UserEmail        string  `db:"user_email"`
	IsShared         bool    `db:"is_shared"`
	DNSVerified      bool    `db:"dns_verified"`
	SSLEnabled       bool    `db:"ssl_enabled"`
	LinkCount        int     `db:"link_count"`
	MarketplacePrice float64 `db:"marketplace_price"`
	CreatedAt        string  `db:"created_at"`
}

func (m *Module) handleAllDomains(w http.ResponseWriter, r *http.Request) {
	var domains []domainRow
	m.DB().Select(&domains, `
		SELECT d.id, d.name, COALESCE(u.email, '') as user_email, d.is_shared,
			d.dns_verified, d.ssl_enabled,
			(SELECT COUNT(*) FROM redirect_links WHERE domain_id = d.id) as link_count,
			COALESCE(d.marketplace_price, 0) as marketplace_price,
			TO_CHAR(d.created_at, 'YYYY-MM-DD') as created_at
		FROM domains d
		LEFT JOIN users u ON u.id = d.user_id
		ORDER BY d.created_at DESC
	`)

	module.Render(w, r, m.templates, "admindash:domains.html", map[string]interface{}{
		"Title":   "All Domains",
		"Domains": domains,
	})
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
			Path:    "/admin/admindash",
			Order:   0,
			Section: module.MenuSectionAdmin,
		},
		{
			Title:   "All Domains",
			Icon:    "bi-globe2",
			Path:    "/admin/admindash/domains/all",
			Order:   5,
			Section: module.MenuSectionAdmin,
		},
	}
}

func (m *Module) Widgets() []module.Widget {
	return nil
}
