package dashboard

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"time"

	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed migrations/*.sql
var migrationsFS embed.FS

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
	files := []string{
		"migrations/001_dashboard_stats.sql",
		"migrations/002_seed_announcements.sql",
	}
	for _, file := range files {
		sql, err := migrationsFS.ReadFile(file)
		if err != nil {
			return err
		}
		if _, err = m.DB().Exec(string(sql)); err != nil {
			return err
		}
	}
	return nil
}

func (m *Module) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", m.handleDashboard)

	// API endpoints
	r.Get("/api/timeline", m.apiTimeline)
	r.Get("/api/countries", m.apiCountries)
	r.Get("/api/visitors", m.apiVisitors)
	r.Post("/api/reset-stat", m.apiResetStat)

	return r
}

func (m *Module) AdminRoutes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", m.handleAdminAnnouncements)

	// API endpoints
	r.Get("/api/list", m.apiListAnnouncements)
	r.Post("/api/create", m.apiCreateAnnouncement)
	r.Put("/api/{id}", m.apiUpdateAnnouncement)
	r.Delete("/api/{id}", m.apiDeleteAnnouncement)

	return r
}

func (m *Module) handleDashboard(w http.ResponseWriter, r *http.Request) {
	user := ctx.GetUser(r)
	userID := user.ID

	// Subscription info
	var subDaysLeft int
	var subStatus string
	m.DB().Get(&subDaysLeft, `
		SELECT GREATEST(0, EXTRACT(DAY FROM (expires_at - NOW()))::INT)
		FROM subscriptions WHERE user_id = $1 AND status = 'active'
		ORDER BY expires_at DESC LIMIT 1
	`, userID)
	m.DB().Get(&subStatus, `
		SELECT COALESCE(status, 'none') FROM subscriptions
		WHERE user_id = $1 ORDER BY expires_at DESC LIMIT 1
	`, userID)

	// KPI stats
	var domains, botsDetected, humansVerified int64
	m.DB().Get(&domains, `SELECT COUNT(*) FROM domains WHERE user_id = $1 AND is_shared = FALSE`, userID)
	m.DB().Get(&botsDetected, `SELECT COALESCE(bots_detected, 0) FROM users WHERE id = $1`, userID)
	m.DB().Get(&humansVerified, `SELECT COALESCE(humans_verified, 0) FROM users WHERE id = $1`, userID)

	// Account overview
	var balance, referralEarnings float64
	var planName string
	m.DB().Get(&balance, `SELECT COALESCE(balance, 0) FROM users WHERE id = $1`, userID)
	m.DB().Get(&referralEarnings, `
		SELECT COALESCE(SUM(amount), 0) FROM balance_transactions
		WHERE user_id = $1 AND type = 'referral_commission'
	`, userID)
	m.DB().Get(&planName, `
		SELECT COALESCE(p.name, 'Free') FROM subscriptions s
		LEFT JOIN subscription_plans p ON p.id = s.plan_id
		WHERE s.user_id = $1 AND s.status = 'active'
		ORDER BY s.expires_at DESC LIMIT 1
	`, userID)
	if planName == "" {
		planName = "Free"
	}

	// Referral link
	var referralCode string
	m.DB().Get(&referralCode, `SELECT COALESCE(referral_code, '') FROM users WHERE id = $1`, userID)
	if referralCode == "" {
		referralCode = userID[:8]
	}

	// News feed
	announcements := m.getAnnouncements()

	module.RenderUserSection(w, r, m.templates, "dashboard:index.html", map[string]interface{}{
		"Title": "Dashboard",
		"KPI": map[string]interface{}{
			"domains":        domains,
			"botsDetected":   botsDetected,
			"humansVerified": humansVerified,
			"subDaysLeft":    subDaysLeft,
		},
		"Account": map[string]interface{}{
			"plan":             planName,
			"balance":          balance,
			"referralEarnings": referralEarnings,
			"referralCode":     referralCode,
		},
		"SubStatus":     subStatus,
		"Announcements": announcements,
	})
}

type announcement struct {
	ID          string    `db:"id"`
	Title       string    `db:"title"`
	Body        string    `db:"body"`
	Badge       *string   `db:"badge"`
	IsPinned    bool      `db:"is_pinned"`
	PublishedAt time.Time `db:"published_at"`
}

func (m *Module) getAnnouncements() []announcement {
	var items []announcement
	m.DB().Select(&items, `
		SELECT id, title, body, badge, is_pinned, published_at
		FROM announcements
		ORDER BY is_pinned DESC, published_at DESC
		LIMIT 10
	`)
	return items
}

func (m *Module) apiResetStat(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var req struct {
		Metric string `json:"metric"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	var err error
	switch req.Metric {
	case "bots":
		_, err = m.DB().Exec(`UPDATE users SET bots_detected = 0 WHERE id = $1`, userID)
	case "humans":
		_, err = m.DB().Exec(`UPDATE users SET humans_verified = 0 WHERE id = $1`, userID)
	default:
		http.Error(w, `{"error":"invalid metric"}`, http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
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

func (m *Module) AdminMenuItems() []module.MenuItem {
	return []module.MenuItem{
		{
			Title:   "Announcements",
			Icon:    "bi-megaphone",
			Path:    "/admin/announcements",
			Order:   90,
			Section: module.MenuSectionAdmin,
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
			SELECT time_bucket, SUM(total) as total, SUM(blocked) as blocked, SUM(unique_count) as unique_count FROM (
				SELECT TO_CHAR(created_at, 'YYYY-MM-DD HH24:00') as time_bucket,
					COUNT(*) as total,
					COUNT(*) FILTER (WHERE blocked = true) as blocked,
					COUNT(*) FILTER (WHERE is_unique = true) as unique_count
				FROM visits
				WHERE user_id = $1 AND created_at > NOW() - INTERVAL '24 hours'
				GROUP BY time_bucket
				UNION ALL
				SELECT TO_CHAR(created_at, 'YYYY-MM-DD HH24:00') as time_bucket,
					COUNT(*) as total,
					COUNT(*) FILTER (WHERE blocked = true) as blocked,
					0 as unique_count
				FROM short_link_clicks
				WHERE user_id = $1 AND created_at > NOW() - INTERVAL '24 hours'
				GROUP BY time_bucket
			) combined GROUP BY time_bucket ORDER BY time_bucket
		`
	case "daily":
		query = `
			SELECT time_bucket, SUM(total) as total, SUM(blocked) as blocked, SUM(unique_count) as unique_count FROM (
				SELECT TO_CHAR(created_at, 'YYYY-MM-DD') as time_bucket,
					COUNT(*) as total,
					COUNT(*) FILTER (WHERE blocked = true) as blocked,
					COUNT(*) FILTER (WHERE is_unique = true) as unique_count
				FROM visits
				WHERE user_id = $1 AND created_at > NOW() - INTERVAL '7 days'
				GROUP BY time_bucket
				UNION ALL
				SELECT TO_CHAR(created_at, 'YYYY-MM-DD') as time_bucket,
					COUNT(*) as total,
					COUNT(*) FILTER (WHERE blocked = true) as blocked,
					0 as unique_count
				FROM short_link_clicks
				WHERE user_id = $1 AND created_at > NOW() - INTERVAL '7 days'
				GROUP BY time_bucket
			) combined GROUP BY time_bucket ORDER BY time_bucket
		`
	case "monthly":
		query = `
			SELECT time_bucket, SUM(total) as total, SUM(blocked) as blocked, SUM(unique_count) as unique_count FROM (
				SELECT TO_CHAR(created_at, 'YYYY-MM') as time_bucket,
					COUNT(*) as total,
					COUNT(*) FILTER (WHERE blocked = true) as blocked,
					COUNT(*) FILTER (WHERE is_unique = true) as unique_count
				FROM visits
				WHERE user_id = $1 AND created_at > NOW() - INTERVAL '12 months'
				GROUP BY time_bucket
				UNION ALL
				SELECT TO_CHAR(created_at, 'YYYY-MM') as time_bucket,
					COUNT(*) as total,
					COUNT(*) FILTER (WHERE blocked = true) as blocked,
					0 as unique_count
				FROM short_link_clicks
				WHERE user_id = $1 AND created_at > NOW() - INTERVAL '12 months'
				GROUP BY time_bucket
			) combined GROUP BY time_bucket ORDER BY time_bucket
		`
	default:
		query = `
			SELECT time_bucket, SUM(total) as total, SUM(blocked) as blocked, SUM(unique_count) as unique_count FROM (
				SELECT TO_CHAR(created_at, 'YYYY-MM-DD HH24:00') as time_bucket,
					COUNT(*) as total,
					COUNT(*) FILTER (WHERE blocked = true) as blocked,
					COUNT(*) FILTER (WHERE is_unique = true) as unique_count
				FROM visits
				WHERE user_id = $1 AND created_at > NOW() - INTERVAL '24 hours'
				GROUP BY time_bucket
				UNION ALL
				SELECT TO_CHAR(created_at, 'YYYY-MM-DD HH24:00') as time_bucket,
					COUNT(*) as total,
					COUNT(*) FILTER (WHERE blocked = true) as blocked,
					0 as unique_count
				FROM short_link_clicks
				WHERE user_id = $1 AND created_at > NOW() - INTERVAL '24 hours'
				GROUP BY time_bucket
			) combined GROUP BY time_bucket ORDER BY time_bucket
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
		SELECT country, SUM(count) as count FROM (
			SELECT CASE
				WHEN country = 'United States' THEN 'US'
				WHEN country = 'Nigeria' THEN 'NG'
				WHEN country = 'United Kingdom' THEN 'GB'
				WHEN country = 'South Africa' THEN 'ZA'
				WHEN country = 'Australia' THEN 'AU'
				WHEN country = 'Germany' THEN 'DE'
				WHEN country = 'France' THEN 'FR'
				WHEN country = 'Canada' THEN 'CA'
				WHEN country = 'Brazil' THEN 'BR'
				WHEN country = 'India' THEN 'IN'
				WHEN country = 'China' THEN 'CN'
				WHEN country = 'Japan' THEN 'JP'
				WHEN country = 'Mexico' THEN 'MX'
				WHEN country = 'Russia' THEN 'RU'
				WHEN country = 'Spain' THEN 'ES'
				WHEN country = 'Italy' THEN 'IT'
				WHEN country = 'Netherlands' THEN 'NL'
				WHEN country = 'Ukraine' THEN 'UA'
				WHEN country = 'Poland' THEN 'PL'
				WHEN country = 'Indonesia' THEN 'ID'
				WHEN country = 'Turkey' THEN 'TR'
				WHEN country = 'Philippines' THEN 'PH'
				WHEN country = 'Vietnam' THEN 'VN'
				WHEN country = 'Thailand' THEN 'TH'
				WHEN country = 'Egypt' THEN 'EG'
				WHEN country = 'Pakistan' THEN 'PK'
				WHEN country = 'Bangladesh' THEN 'BD'
				WHEN country = 'Argentina' THEN 'AR'
				WHEN country = 'Colombia' THEN 'CO'
				WHEN country = 'Kenya' THEN 'KE'
				WHEN country = 'Ghana' THEN 'GH'
				WHEN country = 'Costa Rica' THEN 'CR'
				ELSE country
			END as country, COUNT(*) as count
			FROM visits
			WHERE user_id = $1 AND country != ''
			GROUP BY 1
			UNION ALL
			SELECT country, COUNT(*) as count
			FROM short_link_clicks
			WHERE user_id = $1 AND country != '' AND country IS NOT NULL
			GROUP BY country
		) combined
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
		SELECT latitude, longitude, country, city, blocked FROM (
			SELECT latitude, longitude, country, city, blocked, created_at
			FROM visits
			WHERE user_id = $1 AND latitude != 0 AND longitude != 0
			UNION ALL
			SELECT latitude, longitude, COALESCE(country, '') as country, COALESCE(city, '') as city, blocked, created_at
			FROM short_link_clicks
			WHERE user_id = $1 AND latitude != 0 AND longitude != 0
		) combined
		ORDER BY created_at DESC
		LIMIT 500
	`, userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(points)
}

// Admin handlers for announcements

func (m *Module) handleAdminAnnouncements(w http.ResponseWriter, r *http.Request) {
	module.Render(w, r, m.templates, "dashboard:admin_announcements.html", map[string]interface{}{
		"Title": "Announcements",
	})
}

func (m *Module) apiListAnnouncements(w http.ResponseWriter, r *http.Request) {
	var items []announcement
	m.DB().Select(&items, `
		SELECT id, title, body, badge, is_pinned, published_at
		FROM announcements
		ORDER BY is_pinned DESC, published_at DESC
	`)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

func (m *Module) apiCreateAnnouncement(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title    string  `json:"title"`
		Body     string  `json:"body"`
		Badge    *string `json:"badge"`
		IsPinned bool    `json:"is_pinned"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	id := time.Now().Format("20060102150405")
	_, err := m.DB().Exec(`
		INSERT INTO announcements (id, title, body, badge, is_pinned, published_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
	`, id, req.Title, req.Body, req.Badge, req.IsPinned)

	if err != nil {
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "id": id})
}

func (m *Module) apiUpdateAnnouncement(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req struct {
		Title    string  `json:"title"`
		Body     string  `json:"body"`
		Badge    *string `json:"badge"`
		IsPinned bool    `json:"is_pinned"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	_, err := m.DB().Exec(`
		UPDATE announcements SET title = $2, body = $3, badge = $4, is_pinned = $5
		WHERE id = $1
	`, id, req.Title, req.Body, req.Badge, req.IsPinned)

	if err != nil {
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

func (m *Module) apiDeleteAnnouncement(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	_, err := m.DB().Exec(`DELETE FROM announcements WHERE id = $1`, id)
	if err != nil {
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}
