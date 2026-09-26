package v1

import (
	"github.com/botginx/botginx/modules/analytics/services"
	authservices "github.com/botginx/botginx/modules/auth/services"
	domainservices "github.com/botginx/botginx/modules/domains/services"
	qrservices "github.com/botginx/botginx/modules/qrcodes/services"
	linkservices "github.com/botginx/botginx/modules/redirectlinks/services"
	shortenerservices "github.com/botginx/botginx/modules/shortener/services"
	"github.com/botginx/botginx/pkg/api/v1/handlers"
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

// APIDependencies holds all service dependencies for API handlers
type APIDependencies struct {
	DB                     *sqlx.DB
	LinksService           *linkservices.RedirectLinkService
	DomainsService         *domainservices.DomainService
	AnalyticsService       *services.AnalyticsService
	QRCodesService         *qrservices.QRCodeService
	ShortenerService       *shortenerservices.ShortenerService
	GlobalWhitelistService *authservices.GlobalWhitelistService
}

// NewRouter creates the API v1 router with all endpoints
func NewRouter(deps *APIDependencies) chi.Router {
	r := chi.NewRouter()

	// Initialize handlers
	linksHandler := handlers.NewLinksHandler(deps.LinksService, deps.DomainsService)
	domainsHandler := handlers.NewDomainsHandler(deps.DomainsService)
	analyticsHandler := handlers.NewAnalyticsHandler(deps.AnalyticsService)
	qrcodesHandler := handlers.NewQRCodesHandler(deps.QRCodesService)
	shortenerHandler := handlers.NewShortenerHandler(deps.ShortenerService)
	iplistsHandler := handlers.NewIPListsHandler(deps.GlobalWhitelistService)
	accountHandler := handlers.NewAccountHandler(deps.DB)

	// Links endpoints
	r.Route("/links", func(r chi.Router) {
		r.Get("/", linksHandler.List)
		r.Post("/", linksHandler.Create)
		r.Get("/{id}", linksHandler.Get)
		r.Put("/{id}", linksHandler.Update)
		r.Delete("/{id}", linksHandler.Delete)
	})

	// Domains endpoints
	r.Route("/domains", func(r chi.Router) {
		r.Get("/", domainsHandler.List)
		r.Post("/", domainsHandler.Create)
		r.Get("/{id}", domainsHandler.Get)
		r.Put("/{id}", domainsHandler.Update)
		r.Delete("/{id}", domainsHandler.Delete)
		r.Post("/{id}/verify", domainsHandler.Verify)
	})

	// Analytics endpoints
	r.Route("/analytics", func(r chi.Router) {
		r.Get("/overview", analyticsHandler.Overview)
		r.Get("/links/{id}", analyticsHandler.LinkStats)
		r.Get("/links/{id}/timeline", analyticsHandler.LinkTimeline)
		r.Get("/links/{id}/countries", analyticsHandler.LinkCountries)
		r.Get("/links/{id}/devices", analyticsHandler.LinkDevices)
		r.Get("/links/{id}/referrers", analyticsHandler.LinkReferrers)
	})

	// QR Codes endpoints
	r.Route("/qrcodes", func(r chi.Router) {
		r.Get("/", qrcodesHandler.List)
		r.Post("/", qrcodesHandler.Create)
		r.Get("/{id}", qrcodesHandler.Get)
		r.Get("/{id}/download", qrcodesHandler.Download)
		r.Delete("/{id}", qrcodesHandler.Delete)
	})

	// Shortener endpoints
	r.Route("/shortener", func(r chi.Router) {
		r.Get("/", shortenerHandler.List)
		r.Post("/", shortenerHandler.Create)
		r.Get("/{id}", shortenerHandler.Get)
		r.Get("/{id}/stats", shortenerHandler.Stats)
		r.Put("/{id}", shortenerHandler.Update)
		r.Delete("/{id}", shortenerHandler.Delete)
	})

	// IP Lists endpoints
	r.Route("/iplists", func(r chi.Router) {
		r.Get("/whitelist", iplistsHandler.ListWhitelist)
		r.Post("/whitelist", iplistsHandler.AddWhitelist)
		r.Delete("/whitelist/{id}", iplistsHandler.RemoveWhitelist)
	})

	// Account endpoints
	r.Route("/account", func(r chi.Router) {
		r.Get("/", accountHandler.Info)
		r.Get("/usage", accountHandler.Usage)
	})

	return r
}
