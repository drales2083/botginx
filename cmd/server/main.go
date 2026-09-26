package main

import (
	"context"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/botginx/botginx/modules/admindash"
	"github.com/botginx/botginx/modules/analogstats"
	"github.com/botginx/botginx/modules/analytics"
	"github.com/botginx/botginx/modules/awstats"
	"github.com/botginx/botginx/modules/announcements"
	"github.com/botginx/botginx/modules/antibotcontrol"
	"github.com/botginx/botginx/modules/apikeys"
	"github.com/botginx/botginx/modules/backup"
	"github.com/botginx/botginx/modules/referrals"
	"github.com/botginx/botginx/modules/webhooks"
	"github.com/botginx/botginx/pkg/api/v1"
	"github.com/botginx/botginx/pkg/apiauth"
	analyticshandlers "github.com/botginx/botginx/modules/analytics/handlers"
	"github.com/botginx/botginx/modules/auth"
	"github.com/botginx/botginx/modules/dashboard"
	"github.com/botginx/botginx/modules/domainhealth"
	"github.com/botginx/botginx/modules/domains"
	"github.com/botginx/botginx/modules/help"
	"github.com/botginx/botginx/modules/hosting"
	hostingmodels "github.com/botginx/botginx/modules/hosting/models"
	"github.com/botginx/botginx/modules/iplists"
	iplistsvc "github.com/botginx/botginx/modules/iplists/services"
	"github.com/botginx/botginx/modules/marketplace"
	modulesmgmt "github.com/botginx/botginx/modules/modules"
	"github.com/botginx/botginx/modules/payments"
	"github.com/botginx/botginx/modules/qrcodes"
	qrcodeshandlers "github.com/botginx/botginx/modules/qrcodes/handlers"
	"github.com/botginx/botginx/modules/redirectlinks"
	"github.com/botginx/botginx/modules/reports"
	"github.com/botginx/botginx/modules/servers"
	"github.com/botginx/botginx/modules/shortener"
	"github.com/botginx/botginx/modules/settings"
	"github.com/botginx/botginx/modules/support"
	"github.com/botginx/botginx/modules/telegram"
	"github.com/botginx/botginx/modules/threatlog"
	"github.com/botginx/botginx/modules/twofactor"
	"github.com/botginx/botginx/modules/users"
	"github.com/botginx/botginx/pkg/buildinfo"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/database"
	"github.com/botginx/botginx/pkg/domainstats"
	"github.com/botginx/botginx/pkg/domainsync"
	"github.com/botginx/botginx/pkg/i18n"
	"github.com/botginx/botginx/pkg/lifecycle"
	"github.com/botginx/botginx/pkg/module"
	"github.com/botginx/botginx/pkg/subscription"
	"github.com/botginx/botginx/web"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// iplistServerAdapter bridges the servers module to the iplists ServerProvider interface.
type iplistServerAdapter struct {
	servers *servers.Module
}

func (a *iplistServerAdapter) GetAllDeployServers() ([]iplistsvc.ServerInfo, error) {
	srvs, err := a.servers.GetAllDeployServers()
	if err != nil {
		return nil, err
	}
	result := make([]iplistsvc.ServerInfo, len(srvs))
	for i, s := range srvs {
		result[i] = iplistsvc.ServerInfo{
			IP:       s.IP,
			Port:     s.Port,
			User:     s.User,
			Password: s.Password,
		}
	}
	return result, nil
}

// hostingSettingsAdapter bridges the hosting module to the analytics HostingSettingsProvider interface.
type hostingSettingsAdapter struct {
	hosting *hosting.Module
}

func (a *hostingSettingsAdapter) GetDomainSettingsByHost(host string) (*analyticshandlers.HostingSettings, error) {
	settings, err := a.hosting.Service().GetDomainSettingsByHost(host)
	if err != nil {
		return nil, err
	}
	return &analyticshandlers.HostingSettings{
		CountryMode:      settings.CountryMode,
		CountryList:      settings.CountryList,
		DeviceMode:       settings.DeviceMode,
		DeviceList:       settings.DeviceList,
		BlockBots:        settings.BlockBots,
		BlockTor:         settings.BlockTor,
		BlockProxy:       settings.BlockProxy,
		BlockDatacenter:  settings.BlockDatacenter,
		BlockHeadless:    settings.BlockHeadless,
		MinBehaviorScore: settings.MinBehaviorScore,
		RedirectOnBlock:  settings.RedirectOnBlock,
	}, nil
}

// hostingVisitRecorderAdapter bridges the hosting module to the analytics HostingVisitRecorder interface.
type hostingVisitRecorderAdapter struct {
	hosting *hosting.Module
}

func (a *hostingVisitRecorderAdapter) GetDomainByHost(host string) (*analyticshandlers.HostingDomainInfo, error) {
	info, err := a.hosting.Service().GetDomainByHost(host)
	if err != nil {
		return nil, err
	}
	return &analyticshandlers.HostingDomainInfo{
		DomainID:  info.DomainID,
		AccountID: info.AccountID,
		UserID:    info.UserID,
	}, nil
}

func (a *hostingVisitRecorderAdapter) RecordHostingVisit(visit *analyticshandlers.HostingVisit) error {
	// Convert to hosting model
	hostingVisit := &hostingmodels.HostingVisit{
		DomainID:         visit.DomainID,
		AccountID:        visit.AccountID,
		IP:               visit.IP,
		Path:             visit.Path,
		Method:           visit.Method,
		Country:          visit.Country,
		City:             visit.City,
		ASN:              visit.ASN,
		ASNOrg:           visit.ASNOrg,
		Device:           visit.Device,
		Browser:          visit.Browser,
		OS:               visit.OS,
		UserAgent:        visit.UserAgent,
		Language:         visit.Language,
		Timezone:         visit.Timezone,
		ScreenResolution: visit.ScreenResolution,
		Referrer:         visit.Referrer,
		ReferrerDomain:   visit.ReferrerDomain,
		UTMSource:        visit.UTMSource,
		UTMMedium:        visit.UTMMedium,
		UTMCampaign:      visit.UTMCampaign,
		UTMTerm:          visit.UTMTerm,
		UTMContent:       visit.UTMContent,
		IsBot:            visit.IsBot,
		BotScore:         visit.BotScore,
		BehaviorScore:    visit.BehaviorScore,
		AutomationTool:   visit.AutomationTool,
		IsHeadless:       visit.IsHeadless,
		IsTor:            visit.IsTor,
		IsProxy:          visit.IsProxy,
		IsDatacenter:     visit.IsDatacenter,
		Fingerprint:      visit.Fingerprint,
		Action:           visit.Action,
		Blocked:          visit.Blocked,
		BlockReason:      visit.BlockReason,
		SessionID:        visit.SessionID,
		CreatedAt:        visit.CreatedAt,
	}
	return a.hosting.Service().RecordHostingVisit(hostingVisit)
}

// Config structure
type Config struct {
	Server struct {
		Listen string `yaml:"listen"`
	} `yaml:"server"`
	Database struct {
		URL string `yaml:"url"`
	} `yaml:"database"`
	I18n struct {
		DefaultLang string   `yaml:"default_lang"`
		Available   []string `yaml:"available"`
	} `yaml:"i18n"`
}

func main() {
	configPath := flag.String("config", "config.yaml", "config file path")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("%s (build %s)\n", buildinfo.Version(), buildinfo.Build())
		os.Exit(0)
	}

	// Logger
	log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}).
		With().Timestamp().Logger()

	// Load config
	cfg := loadConfig(*configPath)

	// Database
	db, err := database.Connect(cfg.Database.URL)
	if err != nil {
		log.Fatal().Err(err).Msg("database connection failed")
	}
	defer db.Close()

	// i18n
	translator := i18n.New(cfg.I18n.DefaultLang, cfg.I18n.Available)
	if err := translator.LoadFromFS(); err != nil {
		log.Warn().Err(err).Msg("failed to load translations")
	}

	// Template engine
	layoutsSub, _ := fs.Sub(web.LayoutsFS, "templates/layouts")
	templates := module.NewTemplateEngine(layoutsSub)
	templates.AddFuncs(translator.TemplateFuncs())
	templates.AddFuncs(template.FuncMap{
		"appVersion": buildinfo.Version,
		"appBuild":   buildinfo.Build,
		"appName":    getAppName,
		"hasPrefix":  strings.HasPrefix,
		"formatBytes": func(b int64) string {
			const unit = 1024
			if b < unit {
				return fmt.Sprintf("%d B", b)
			}
			div, exp := int64(unit), 0
			for n := b / unit; n >= unit; n /= unit {
				div *= unit
				exp++
			}
			return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
		},
		"duration": func(start, end time.Time) string {
			if end.IsZero() {
				return "-"
			}
			d := end.Sub(start)
			if d < time.Second {
				return fmt.Sprintf("%dms", d.Milliseconds())
			}
			return fmt.Sprintf("%.1fs", d.Seconds())
		},
	})
	// Rebuilt per render so "t" resolves in the visitor's chosen language
	// rather than the configured default.
	templates.SetRequestFuncs(translator.RequestTemplateFuncs)

	// Error pages are registered like a module so they can use the same layouts.
	errorsSub, _ := fs.Sub(web.ErrorsFS, "templates/errors")
	templates.RegisterModule("errors", errorsSub)

	// Subscription gating, shared by the middleware and the admin UI.
	subscriptions := subscription.NewService(db.DB)

	// Start auto-renewal background loop (checks hourly, renews from balance)
	subscriptions.StartAutoRenewalLoop()

	// Start domain sync service (checks every 5 min, syncs domains to servers)
	domainSyncSvc := domainsync.NewService(db.DB, 5*time.Minute)
	domainSyncSvc.SetLinkRedeployer(domainsync.NewDefaultRedeployer(db.DB))
	go domainSyncSvc.Start(context.Background())

	// Module registry
	registry := module.NewRegistry()

	// Register modules (order matters for menu).
	// Servers and domains are registered before redirectlinks, which borrows
	// their services to resolve deploy targets.
	authModule := auth.New()
	serversModule := servers.New()
	domainsModule := domains.New()
	redirectLinksModule := redirectlinks.New(domainsModule, serversModule)

	// Analytics resolves inbound hostnames to links through redirectlinks.
	analyticsModule := analytics.New(redirectLinksModule)

	iplistsModule := iplists.New()

	hostingModule := hosting.New()
	shortenerModule := shortener.New(domainsModule, serversModule)
	marketplaceModule := marketplace.New()
	paymentsModule := payments.New()

	registry.Register(authModule)
	registry.Register(dashboard.New())
	registry.Register(serversModule)             // Admin module
	registry.Register(domainsModule)
	registry.Register(redirectLinksModule)
	registry.Register(analyticsModule)           // Analytics module
	registry.Register(iplistsModule)             // IP Lists module
	registry.Register(shortenerModule)           // Link Shortener module
	qrcodesModule := qrcodes.New()
	registry.Register(qrcodesModule)             // QR Codes module
	reportsModule := reports.New()
	registry.Register(reportsModule)             // Reports module (CSV export)
	domainHealthModule := domainhealth.New()
	registry.Register(domainHealthModule)        // Domain health checker
	registry.Register(analogstats.New())         // Analog Stats (classic web stats)
	registry.Register(awstats.New())             // AWStats (advanced web statistics)
	registry.Register(threatlog.New())           // Threat Log (security monitoring)
	registry.Register(antibotcontrol.New())      // Antibot Control (default settings)
	registry.Register(apikeys.New())             // API Keys management
	registry.Register(webhooks.New())            // Webhooks module
	registry.Register(hostingModule)             // Bullet Proof Hosting module
	registry.Register(marketplaceModule)         // Domain marketplace
	registry.Register(paymentsModule)            // Crypto payments
	registry.Register(help.New())                // Help/FAQ module
	supportModule := support.New()
	registry.Register(supportModule)             // Support tickets
	telegramModule := telegram.New()
	registry.Register(telegramModule)            // Telegram bot integration
	settingsModule := settings.New(telegramModule)
	registry.Register(settingsModule)            // Admin settings (telegram, etc.)
	backupModule := backup.New()
	registry.Register(backupModule)              // Telegram backup system
	usersModule := users.New()
	registry.Register(usersModule)               // Admin module
	referralsModule := referrals.New()

	registry.Register(admindash.New())           // Admin dashboard (stats)
	registry.Register(announcements.New())       // News announcements (admin)
	registry.Register(referralsModule)           // Referral commission system
	registry.Register(twofactor.New())           // Two-factor authentication
	registry.Register(modulesmgmt.New(registry)) // Module management (admin)

	// Initialize all modules
	deps := &module.Dependencies{
		DB:        db.DB,
		Templates: templates,
		Config:    map[string]interface{}{},
	}

	if err := registry.InitAll(deps); err != nil {
		log.Fatal().Err(err).Msg("module init failed")
	}

	// Wire up dependencies after init to avoid circular imports
	analyticsModule.SetLinkDetails(redirectLinksModule)
	analyticsModule.SetServerProvider(serversModule)
	analyticsModule.SetHostingSettingsProvider(&hostingSettingsAdapter{hosting: hostingModule})
	analyticsModule.SetHostingVisitRecorder(&hostingVisitRecorderAdapter{hosting: hostingModule})
	analyticsModule.SetShortLinkResolver(shortenerModule)
	redirectLinksModule.SetServerProvider(serversModule)
	shortenerModule.SetServerProvider(serversModule)
	iplistsModule.SetServerProvider(&iplistServerAdapter{servers: serversModule})
	hostingModule.SetPaymentProcessor(referralsModule)  // Referral commissions on hosting payments
	authModule.SetSubscriptionService(subscriptions)    // Self-service subscription purchase
	usersModule.SetAuthService(authModule.AuthService()) // Admin impersonation
	supportModule.SetNotifier(telegramModule.Handler)    // Telegram notifications for tickets

	// QR codes needs redirect links list
	qrcodesModule.SetLinkLister(func(userID string) ([]qrcodeshandlers.RedirectLink, error) {
		links, err := redirectLinksModule.ListForUser(userID)
		if err != nil {
			return nil, err
		}
		result := make([]qrcodeshandlers.RedirectLink, len(links))
		for i, l := range links {
			result[i] = qrcodeshandlers.RedirectLink{
				ID:        l.ID,
				Subdomain: l.Subdomain,
				Domain:    l.Domain,
				Path:      l.Path,
			}
		}
		return result, nil
	})

	// Run migrations
	if err := registry.MigrateAll(); err != nil {
		log.Fatal().Err(err).Msg("migration failed")
	}

	// Collect menu items by section
	userMenu := registry.CollectMenuBySection(module.MenuSectionUser)
	adminMenu := registry.CollectMenuBySection(module.MenuSectionAdmin)
	templates.SetMenuSections(userMenu, adminMenu)

	// Collect widgets
	templates.SetWidgets(registry.CollectWidgets())

	// Router
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))
	r.Use(translator.Middleware) // i18n middleware

	// Static files (no directory listing)
	staticSub, _ := fs.Sub(web.StaticFS, "static")
	staticHandler := http.StripPrefix("/static/", noDirectoryListing(http.FileServer(http.FS(staticSub))))
	r.Handle("/static", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }))
	r.Handle("/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }))
	r.Handle("/static/*", staticHandler)

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	// 404 page. Signed-in visitors get it inside the app chrome; everyone else
	// gets the standalone version, so the menu is never rendered for them.
	notFound := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)

		user := ctx.GetUser(r)
		if user == nil {
			templates.RenderAuth(w, r, "errors:404_public.html", map[string]interface{}{
				"Title": "Page Not Found",
			})
			return
		}

		data := map[string]interface{}{
			"Title": "Page Not Found",
			"User":  user,
		}
		if user.IsAdmin() && strings.HasPrefix(r.URL.Path, "/admin") {
			templates.Render(w, r, "errors:404.html", data)
			return
		}
		templates.RenderUserSection(w, r, "errors:404.html", data)
	}

	// Public routes (auth)
	r.Mount("/auth", authModule.Routes())

	// Webhook routes (no auth, signature verified in handler)
	r.Mount("/webhooks", analyticsModule.WebhookRoutes())

	// Botection callback API (no auth, localhost only)
	// Used by botection to get per-link blocking decisions before taking action
	r.Mount("/api/botection", analyticsModule.BotectionRoutes())

	// Internal cron routes (no auth, localhost only)
	// POST /api/cron/hosting/billing - process monthly billing
	r.Mount("/api/cron/hosting", hostingModule.CronRoutes())

	// Public tracking API (no auth, CORS enabled)
	// Called by redirect pages to record visits
	r.Mount("/api/track", analyticsModule.TrackingRoutes())

	// Payments webhook (no auth, signature verified in handler)
	// POST /api/payments/webhook/bitgo - BitGo transaction callback
	r.Mount("/api/payments", paymentsModule.PublicRoutes())

	// Public API v1 (API key auth)
	// External integrations use API keys created in /user/apikeys
	apiAuthMiddleware := apiauth.NewMiddleware(db.DB)
	apiRateLimiter := apiauth.NewRateLimiter()
	apiDeps := &v1.APIDependencies{
		DB:                     db.DB,
		LinksService:           redirectLinksModule.Service(),
		DomainsService:         domainsModule.Service(),
		AnalyticsService:       analyticsModule.Service(),
		QRCodesService:         qrcodesModule.Service(),
		ShortenerService:       shortenerModule.Service(),
		GlobalWhitelistService: authModule.GlobalWhitelistService(),
	}
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(apiAuthMiddleware.Authenticate)
		r.Use(apiRateLimiter.RateLimitMiddleware())
		r.Mount("/", v1.NewRouter(apiDeps))
	})

	// Telegram webhook (no auth, secret verified in handler)
	// POST /telegram/webhook/{botID}/{secret} - Telegram bot callback
	r.Mount("/telegram", telegramModule.Routes())

	// User routes (/user/*) - require auth and active subscription for all
	// product features. Unsubscribed users are redirected to settings.
	r.Route("/user", func(r chi.Router) {
		r.Use(authModule.Handler.AuthMiddleware)
		// Attached to the whole section so every page can render an accurate
		// subscription banner, including the ones that stay writable below.
		r.Use(subscriptions.Attach)
		// Domain stats for promotional banners
		r.Use(domainstats.Middleware(db.DB))

		// Account settings and subscription pages live in the auth module.
		// Deliberately outside EnforceAll: users must be able to view their
		// subscription status and change passwords even when expired.
		r.Mount("/settings", authModule.SettingsRoutes())
		r.Mount("/subscription", authModule.SubscriptionRoutes())

		// Payments routes: users must be able to deposit even without subscription.
		// They need money to buy a subscription in the first place.
		r.Mount("/payments", paymentsModule.Routes())

		// Impersonation exit: must be accessible by impersonated user (not admin-only)
		r.Post("/impersonate/exit", usersModule.Handler().APIExitImpersonation)

		// Two-factor auth: available to all users regardless of subscription.
		// Security features should never be paywalled.
		if tfMod, ok := registry.Get("twofactor"); ok {
			r.Mount("/twofactor", tfMod.Routes())
		}

		// QR Codes: available to all users regardless of subscription.
		if qrMod, ok := registry.Get("qrcodes"); ok {
			r.Mount("/qrcodes", qrMod.Routes())
		}

		// Reports: CSV export of analytics data.
		if repMod, ok := registry.Get("reports"); ok {
			r.Mount("/reports", repMod.Routes())
		}

		// Domain Health: check DNS, SSL, HTTP status.
		if dhMod, ok := registry.Get("domainhealth"); ok {
			r.Mount("/domains/health", dhMod.Routes())
		}

		// Analog Stats: classic web server statistics.
		if asMod, ok := registry.Get("analogstats"); ok {
			r.Mount("/analogstats", asMod.Routes())
		}

		// AWStats: advanced web statistics.
		if awMod, ok := registry.Get("awstats"); ok {
			r.Mount("/awstats", awMod.Routes())
		}

		// Threat Log: security monitoring.
		if tlMod, ok := registry.Get("threatlog"); ok {
			r.Mount("/threatlog", tlMod.Routes())
		}

		// Antibot Control: default protection settings.
		if abMod, ok := registry.Get("antibotcontrol"); ok {
			r.Mount("/antibotcontrol", abMod.Routes())
		}

		// API Keys: manage API keys for external integrations.
		if akMod, ok := registry.Get("apikeys"); ok {
			r.Mount("/apikeys", akMod.Routes())
		}

		// Webhooks: event subscriptions for external integrations.
		if whMod, ok := registry.Get("webhooks"); ok {
			r.Mount("/webhooks", whMod.Routes())
		}

		// Product routes: writes require an active subscription.
		// Unsubscribed users can browse but cannot create, edit, or delete.
		r.Group(func(r chi.Router) {
			r.Use(subscriptions.Enforce)
			registry.MountRoutesBySection(r, module.MenuSectionUser, "auth", "payments", "twofactor", "qrcodes", "reports", "domainhealth", "analogstats", "awstats", "threatlog", "antibotcontrol", "apikeys", "webhooks")
		})

		// Registered on the section rather than inside the group above: chi
		// resolves NotFound on the router it is set on, and a 404 is a read
		// that never needed the subscription gate anyway.
		r.NotFound(notFound)
	})

	// Admin routes (/admin/*) - require auth + admin role
	r.Route("/admin", func(r chi.Router) {
		r.Use(authModule.Handler.AuthMiddleware)
		r.Use(authModule.Handler.AdminMiddleware)
		registry.MountRoutesBySection(r, module.MenuSectionAdmin, "auth")
		r.NotFound(notFound)
	})

	// Unknown paths outside the app sections. Optional auth so a signed-in
	// visitor still gets the full page rather than the anonymous one.
	// Subscription state is attached too, or a subscribed user landing on a bad
	// root URL would be told they need to subscribe.
	r.NotFound(authModule.Handler.OptionalAuthMiddleware(
		subscriptions.Attach(http.HandlerFunc(notFound)),
	).ServeHTTP)

	// Root redirect to user dashboard
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/user/dashboard", http.StatusFound)
	})

	// Start server
	srv := &http.Server{
		Addr:    cfg.Server.Listen,
		Handler: r,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Info().Str("addr", cfg.Server.Listen).Msg("starting server")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Restart is requested from the admin panel; SIGTERM/SIGINT come from the
	// supervisor or the terminal. Both drain in-flight requests first.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		log.Fatal().Err(err).Msg("server failed")

	case sig := <-signals:
		log.Info().Str("signal", sig.String()).Msg("shutting down")
		drain(srv)

	case <-lifecycle.Restart():
		log.Info().Msg("restart requested from admin panel")
		drain(srv)

		// Replaces this process image, so nothing below runs on success.
		if err := lifecycle.Exec(); err != nil {
			// Exiting non-zero lets a supervisor bring us back; without one,
			// staying down is still better than a half-shut-down process.
			log.Fatal().Err(err).Msg("restart failed")
		}
	}
}

// drain stops accepting connections and waits for in-flight requests, so a
// restart does not cut off a response mid-write.
func drain(srv *http.Server) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Warn().Err(err).Msg("shutdown timed out, closing anyway")
		srv.Close()
	}
}

func loadConfig(path string) *Config {
	cfg := &Config{}
	cfg.Server.Listen = ":3001"
	cfg.Database.URL = "postgres://localhost/botginx?sslmode=disable"
	cfg.I18n.DefaultLang = "en"
	cfg.I18n.Available = []string{"en", "ru"}

	data, err := os.ReadFile(path)
	if err != nil {
		log.Warn().Msg("using default config")
		return cfg
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		log.Warn().Err(err).Msg("config parse error, using defaults")
	}

	// Env overrides
	if url := os.Getenv("DATABASE_URL"); url != "" {
		cfg.Database.URL = url
	}
	if lang := os.Getenv("DEFAULT_LANG"); lang != "" {
		cfg.I18n.DefaultLang = lang
	}

	return cfg
}

func getAppName() string {
	if name := os.Getenv("UI_APP_NAME"); name != "" {
		return name
	}
	return "GuardBot"
}

// noDirectoryListing wraps a file server to return 404 for directory requests
func noDirectoryListing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block empty path (root directory) and paths ending with /
		if r.URL.Path == "" || r.URL.Path == "/" || strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
