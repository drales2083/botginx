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

	"github.com/botginx/botginx/modules/analytics"
	"github.com/botginx/botginx/modules/auth"
	"github.com/botginx/botginx/modules/dashboard"
	"github.com/botginx/botginx/modules/domains"
	modulesmgmt "github.com/botginx/botginx/modules/modules"
	"github.com/botginx/botginx/modules/redirectlinks"
	"github.com/botginx/botginx/modules/servers"
	"github.com/botginx/botginx/modules/users"
	"github.com/botginx/botginx/pkg/buildinfo"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/database"
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
	})
	// Rebuilt per render so "t" resolves in the visitor's chosen language
	// rather than the configured default.
	templates.SetRequestFuncs(translator.RequestTemplateFuncs)

	// Error pages are registered like a module so they can use the same layouts.
	errorsSub, _ := fs.Sub(web.ErrorsFS, "templates/errors")
	templates.RegisterModule("errors", errorsSub)

	// Subscription gating, shared by the middleware and the admin UI.
	subscriptions := subscription.NewService(db.DB)

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

	registry.Register(authModule)
	registry.Register(dashboard.New())
	registry.Register(serversModule)             // Admin module
	registry.Register(domainsModule)
	registry.Register(redirectLinksModule)
	registry.Register(analyticsModule)           // Analytics module
	registry.Register(users.New())               // Admin module
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

	// Wire up analytics settings push dependencies (after init, to avoid circular deps)
	analyticsModule.SetLinkDetails(redirectLinksModule)
	analyticsModule.SetServerProvider(serversModule)

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

	// Static files
	staticSub, _ := fs.Sub(web.StaticFS, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))

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

	// User routes (/user/*) - require auth, and an active subscription for any
	// write. Reads stay open so a lapsed user keeps view-only access.
	r.Route("/user", func(r chi.Router) {
		r.Use(authModule.Handler.AuthMiddleware)
		// Attached to the whole section so every page can render an accurate
		// subscription banner, including the ones that stay writable below.
		r.Use(subscriptions.Attach)

		// Account settings live in the auth module but belong to this section.
		// Deliberately outside Enforce: changing your own password must not
		// require an active subscription.
		r.Mount("/settings", authModule.SettingsRoutes())

		// Product routes: writes require an active subscription.
		r.Group(func(r chi.Router) {
			r.Use(subscriptions.Enforce)
			registry.MountRoutesBySection(r, module.MenuSectionUser, "auth")
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
