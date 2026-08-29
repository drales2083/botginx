package analytics

import (
	"embed"
	"io/fs"

	"github.com/botginx/botginx/modules/analytics/handlers"
	"github.com/botginx/botginx/modules/analytics/services"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed migrations/*.sql
var migrationsFS embed.FS

// LinkResolver attributes inbound traffic to a redirect link and its owner.
// Antibot events identify a request by hostname, not by link id.
type LinkResolver interface {
	ResolveByHost(host string) (linkID, userID string, err error)
	OwnerOf(linkID string) (userID string, err error)
}

type Module struct {
	*module.BaseModule
	service *services.AnalyticsService
	handler *handlers.Handler
	links   LinkResolver
}

func New(links LinkResolver) *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"analytics",
			"Analytics",
			"Track and analyze redirect link traffic",
		),
		links: links,
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)

	m.service = services.NewAnalyticsService(deps.DB)
	m.handler = handlers.NewHandler(m.service, deps.Templates, m.links)

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
	r.Get("/", m.handler.Overview)
	r.Get("/link/{linkId}", m.handler.LinkAnalytics)
	r.Get("/link/{linkId}/settings", m.handler.LinkSettings)

	// API
	r.Route("/api", func(r chi.Router) {
		r.Get("/overview", m.handler.APIGetOverview)
		r.Post("/record", m.handler.APIRecordVisit)

		r.Route("/link/{linkId}", func(r chi.Router) {
			r.Get("/stats", m.handler.APIGetStats)
			r.Get("/timeline", m.handler.APIGetTimeline)
			r.Get("/countries", m.handler.APIGetCountries)
			r.Get("/devices", m.handler.APIGetDevices)
			r.Get("/browsers", m.handler.APIGetBrowsers)
			r.Get("/referrers", m.handler.APIGetReferrers)
			r.Get("/visits", m.handler.APIGetRecentVisits)
			r.Get("/settings", m.handler.APIGetSettings)
			r.Put("/settings", m.handler.APIUpdateSettings)
		})
	})

	return r
}

// WebhookRoutes returns routes that should be mounted without auth middleware
func (m *Module) WebhookRoutes() chi.Router {
	r := chi.NewRouter()
	r.Post("/antibot/webhook", m.handler.WebhookHandler)
	return r
}

// BotectionRoutes returns internal API routes for botection callbacks
// These are machine-to-machine (localhost only), not user-facing
func (m *Module) BotectionRoutes() chi.Router {
	r := chi.NewRouter()
	r.Post("/should-block", m.handler.ShouldBlockCallback)
	return r
}

func (m *Module) Templates() fs.FS {
	tmplFS, _ := fs.Sub(templatesFS, "templates")
	return tmplFS
}

func (m *Module) MenuItems() []module.MenuItem {
	return []module.MenuItem{
		{
			Title:   "Analytics",
			Icon:    "bi-graph-up",
			Path:    "/user/analytics",
			Order:   25,
			Section: module.MenuSectionUser,
		},
	}
}

func (m *Module) Widgets() []module.Widget {
	return []module.Widget{
		{
			ID:    "analytics-summary",
			Title: "Traffic Today",
			Icon:  "bi-graph-up",
			Size:  "small",
			Order: 15,
			Data: func() (interface{}, error) {
				return map[string]interface{}{
					"count": 0,
					"label": "Visits Today",
				}, nil
			},
		},
	}
}

// Service exposes the analytics service for other modules
func (m *Module) Service() *services.AnalyticsService {
	return m.service
}
