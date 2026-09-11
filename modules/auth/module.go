package auth

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/botginx/botginx/modules/auth/handlers"
	"github.com/botginx/botginx/modules/auth/services"
	"github.com/botginx/botginx/pkg/module"
	"github.com/botginx/botginx/pkg/subscription"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Module struct {
	*module.BaseModule
	service          *services.AuthService
	globalWhitelist  *services.GlobalWhitelistService
	Handler          *handlers.Handler
}

func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"auth",
			"Authentication",
			"User authentication and sessions",
		),
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)

	m.service = services.NewAuthService(deps.DB)
	m.globalWhitelist = services.NewGlobalWhitelistService(deps.DB)
	m.Handler = handlers.NewHandler(m.service, deps.Templates)
	m.Handler.SetGlobalWhitelistService(m.globalWhitelist)
	m.Handler.SetDB(deps.DB)

	tmplFS, _ := fs.Sub(templatesFS, "templates")
	deps.Templates.RegisterModule(m.ID(), tmplFS)

	return nil
}

// SetSubscriptionService wires up the subscription service for self-service subscription purchase
func (m *Module) SetSubscriptionService(s *subscription.Service) {
	m.Handler.SetSubscriptionService(s)
}

// AuthService returns the auth service for use by other modules (e.g., impersonation)
func (m *Module) AuthService() *services.AuthService {
	return m.service
}

func (m *Module) Migrate() error {
	migrations := []string{
		"migrations/001_create_tables.sql",
		"migrations/002_global_whitelist.sql",
	}

	for _, mig := range migrations {
		sql, err := fs.ReadFile(migrationsFS, mig)
		if err != nil {
			return err
		}
		if _, err := m.DB().Exec(string(sql)); err != nil {
			return err
		}
	}
	return nil
}

// SettingsRoutes is the signed-in user's own account page. It is mounted under
// /user rather than /auth so it inherits the app chrome and the auth
// middleware, while the password logic stays here with the rest of auth.
func (m *Module) SettingsRoutes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", m.Handler.SettingsPage)
	r.Put("/api/profile", m.Handler.APIUpdateProfile)
	r.Put("/api/password", m.Handler.APIChangePassword)

	// Global whitelist API
	r.Get("/api/global-whitelist", m.Handler.APIGetGlobalWhitelist)
	r.Post("/api/global-whitelist", m.Handler.APIAddGlobalWhitelist)
	r.Delete("/api/global-whitelist/{id}", m.Handler.APIRemoveGlobalWhitelist)

	return r
}

// SubscriptionRoutes returns routes for the subscription page.
// Mounted at /user/subscription, outside EnforceAll so users can view
// their subscription status even when expired.
func (m *Module) SubscriptionRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", m.Handler.SubscriptionPage)
	r.Post("/api/subscribe", m.Handler.APISubscribe)
	return r
}

func (m *Module) Routes() chi.Router {
	r := chi.NewRouter()

	// Public pages
	r.Get("/login", m.Handler.LoginPage)
	r.Get("/signup", m.Handler.SignupPage)

	// API
	r.Post("/api/signup", m.Handler.APISignup)
	r.Post("/api/login", m.Handler.APILogin)
	r.Post("/api/logout", m.Handler.APILogout)
	r.Get("/api/me", m.Handler.APIMe)

	// Logout redirect
	r.Get("/logout", func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie("session"); err == nil {
			m.service.Logout(cookie.Value)
		}
		http.SetCookie(w, &http.Cookie{
			Name:   "session",
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		})
		http.Redirect(w, r, "/auth/login", http.StatusFound)
	})

	return r
}

func (m *Module) Templates() fs.FS {
	tmplFS, _ := fs.Sub(templatesFS, "templates")
	return tmplFS
}

func (m *Module) MenuItems() []module.MenuItem {
	// Login and signup stay out of the sidebar, but the account pages
	// this module serves under /user belong there. Ordered last so they
	// sit below the feature pages.
	return []module.MenuItem{
		{
			Title:   "Subscription",
			Icon:    "bi-credit-card",
			Path:    "/user/subscription",
			Order:   85,
			Section: module.MenuSectionUser,
		},
		{
			Title:   "Settings",
			Icon:    "bi-gear",
			Path:    "/user/settings",
			Order:   90,
			Section: module.MenuSectionUser,
		},
	}
}
