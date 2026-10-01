package trackingpixel

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"io/fs"
	"log"
	"time"

	"github.com/go-chi/chi/v5"

	domainmodels "github.com/botginx/botginx/modules/domains/models"
	"github.com/botginx/botginx/modules/trackingpixel/handlers"
	"github.com/botginx/botginx/modules/trackingpixel/models"
	"github.com/botginx/botginx/pkg/module"
)

//go:embed templates
var templatesFS embed.FS

//go:embed migrations
var migrationsFS embed.FS

// DomainProvider is satisfied by the host's domains module, injected in main.go.
type DomainProvider interface {
	ListAvailable(userID string) ([]domainmodels.Domain, error)
}

// Module is the Tracking Pixel host module.
type Module struct {
	*module.BaseModule
	deps    *module.Dependencies
	domains DomainProvider
	handler *handlers.Handler
}

// New constructs the module with an injected domain provider.
func New(domains DomainProvider) *Module {
	return &Module{
		BaseModule: module.NewBaseModule("tracking-pixel", "Tracking Pixel",
			"Create 1x1 email open-tracking pixels on your verified domains and see who opens your mail."),
		domains: domains,
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.deps = deps
	m.SetDeps(deps)
	store := models.NewStore(deps.DB)
	m.handler = handlers.New(deps, store, domainLister(m.domains), ipSalt(deps), retentionDays(deps))
	sub, err := fs.Sub(templatesFS, "templates")
	if err != nil {
		return err
	}
	if err := deps.Templates.RegisterModule(m.ID(), sub); err != nil {
		return err
	}
	go purgeLoop(store, retentionDays(deps))
	return nil
}

// purgeLoop enforces retention once at startup and then every 6 hours.
func purgeLoop(store *models.Store, retention time.Duration) {
	purge := func() {
		if err := store.PurgeOlderThan(retention); err != nil {
			log.Printf("tracking-pixel: retention purge: %v", err)
		}
	}
	purge()
	t := time.NewTicker(6 * time.Hour)
	defer t.Stop()
	for range t.C {
		purge()
	}
}

// Migrate creates the module's tables.
func (m *Module) Migrate() error {
	b, err := migrationsFS.ReadFile("migrations/001_tracking_pixel.sql")
	if err != nil {
		return err
	}
	_, err = m.deps.DB.Exec(string(b))
	return err
}

// Routes are the authed routes, mounted by the host at /user/tracking-pixel.
func (m *Module) Routes() chi.Router { return m.handler.Routes() }

// PublicRoutes serve the pixel; the host mounts them OUTSIDE auth, e.g.
// r.Mount("/px", module.PublicRoutes()).
func (m *Module) PublicRoutes() chi.Router { return m.handler.PublicRoutes() }

func (m *Module) Templates() fs.FS { sub, _ := fs.Sub(templatesFS, "templates"); return sub }

func (m *Module) MenuItems() []module.MenuItem {
	return []module.MenuItem{{
		Title: "Tracking Pixel", Icon: "bi-bullseye", Path: "/user/tracking-pixel",
		Section: module.MenuSectionUser, Group: "Links", Order: 110,
	}}
}

// domainLister adapts DomainProvider to the plain closure handlers need.
func domainLister(dp DomainProvider) func(string) ([]handlers.Domain, error) {
	return func(userID string) ([]handlers.Domain, error) {
		if dp == nil {
			return nil, nil
		}
		ds, err := dp.ListAvailable(userID)
		if err != nil {
			return nil, err
		}
		out := make([]handlers.Domain, 0, len(ds))
		for _, d := range ds {
			out = append(out, handlers.Domain{Name: d.Name, Verified: d.DNSVerified})
		}
		return out, nil
	}
}

func ipSalt(deps *module.Dependencies) []byte {
	if deps.Config != nil {
		if v, ok := deps.Config["tracking_pixel_ip_salt"].(string); ok && v != "" {
			return []byte(v)
		}
	}
	log.Printf("tracking-pixel: WARNING tracking_pixel_ip_salt not set; using a random salt, so IP hashes are not stable across restarts or instances. Set the key.")
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return []byte(hex.EncodeToString(b))
}

func retentionDays(deps *module.Dependencies) time.Duration {
	days := 90
	if deps.Config != nil {
		switch v := deps.Config["tracking_pixel_retention_days"].(type) {
		case int:
			if v > 0 {
				days = v
			}
		case float64: // JSON-decoded config
			if v > 0 {
				days = int(v)
			}
		}
	}
	return time.Duration(days) * 24 * time.Hour
}
