package optimizer

import (
	"crypto/rand"
	"embed"
	"io/fs"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/botginx/botginx/modules/optimizer/core"
	"github.com/botginx/botginx/modules/optimizer/handlers"
	"github.com/botginx/botginx/pkg/module"
)

//go:embed templates
var templatesFS embed.FS

// Module is the Attachment Optimizer host module.
type Module struct {
	*module.BaseModule
	deps    *module.Dependencies
	signKey []byte
	handler *handlers.Handler
}

// New constructs the module.
func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"optimizer",
			"Attachment Optimizer",
			"Clean and optimize HTML and PDF attachments: strip trackers, scripts and embedded code, and neutralize or wrap links.",
		),
	}
}

// Init wires dependencies, derives the signing key, and registers templates.
func (m *Module) Init(deps *module.Dependencies) error {
	m.deps = deps
	m.SetDeps(deps)

	key := signKey(deps)
	m.signKey = key
	var wrapBase string
	if deps != nil && deps.Config != nil {
		wrapBase, _ = deps.Config["optimizer_public_base_url"].(string)
	}
	if !core.IsHTTPURL(wrapBase) {
		log.Printf("optimizer: optimizer_public_base_url is unset or not an absolute http(s) URL; wrap mode is disabled and links will be neutralized.")
	}
	m.handler = handlers.New(deps, key, wrapBase)

	sub, err := fs.Sub(templatesFS, "templates")
	if err != nil {
		return err
	}
	return deps.Templates.RegisterModule(m.ID(), sub)
}

// PublicRedirectHandler returns the unauthenticated redirect handler. It is a
// direct handler (no inner mux): it must be registered on a route that has a
// {token} URL param, e.g. r.Get("/optimizer/r/{token}", m.PublicRedirectHandler()),
// at the public path matching optimizer_public_base_url, OUTSIDE auth
// middleware. Before Init it responds 500 instead of panicking.
func (m *Module) PublicRedirectHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if len(m.signKey) == 0 {
			http.Error(w, "optimizer not initialized", http.StatusInternalServerError)
			return
		}
		handlers.Redirect(m.signKey, w, r)
	}
}

// PublicRoutes returns a router serving GET /{token}, for hosts that prefer
// r.Mount("/optimizer/r", m.PublicRoutes()). Mount outside auth middleware.
func (m *Module) PublicRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/{token}", m.PublicRedirectHandler())
	return r
}

// Migrate is a no-op: the module is stateless.
func (m *Module) Migrate() error { return nil }

// Routes returns the module's chi router (mounted at /user/optimizer).
func (m *Module) Routes() chi.Router { return m.handler.Routes() }

// Templates exposes the embedded template FS.
func (m *Module) Templates() fs.FS {
	sub, _ := fs.Sub(templatesFS, "templates")
	return sub
}

// MenuItems adds a single user-section entry.
func (m *Module) MenuItems() []module.MenuItem {
	return []module.MenuItem{{
		Title:   "Attachment Optimizer",
		Icon:    "bi-file-earmark-zip",
		Path:    "/user/optimizer",
		Section: module.MenuSectionUser,
		Group:   "Links",
		Order:   100,
	}}
}

func signKey(deps *module.Dependencies) []byte {
	if deps != nil && deps.Config != nil {
		if v, ok := deps.Config["optimizer_sign_key"].(string); ok && v != "" {
			return []byte(v)
		}
	}
	// Fallback: random key generated at startup. Tokens are short-lived, so a
	// restart simply invalidates outstanding redirector links.
	log.Printf("optimizer: optimizer_sign_key not set; using a random key. Wrapped links will not survive a restart. Set optimizer_sign_key in production.")
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return b
}
