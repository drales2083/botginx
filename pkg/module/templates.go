package module

import (
	"bytes"
	"encoding/json"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/domainstats"
	"github.com/botginx/botginx/pkg/protection"
	"github.com/botginx/botginx/pkg/subscription"
)

// RequestFuncs produces template functions scoped to a single request, such as
// translation bound to the visitor's chosen language.
type RequestFuncs func(r *http.Request) template.FuncMap

// TemplateEngine manages templates from multiple modules
type TemplateEngine struct {
	mu           sync.RWMutex
	templates    map[string]*template.Template
	layouts      fs.FS
	funcs        template.FuncMap
	requestFuncs RequestFuncs
	userMenu     []MenuItem
	adminMenu    []MenuItem
	widgets      []Widget
}

func NewTemplateEngine(layouts fs.FS) *TemplateEngine {
	return &TemplateEngine{
		templates: make(map[string]*template.Template),
		layouts:   layouts,
		funcs:     defaultFuncs(),
	}
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case float64:
		return n
	case float32:
		return float64(n)
	default:
		return 0
	}
}

func defaultFuncs() template.FuncMap {
	return template.FuncMap{
		"safeHTML":  func(s string) template.HTML { return template.HTML(s) },
		"safeJS":    func(s string) template.JS { return template.JS(s) },
		"safeURL":   func(s string) template.URL { return template.URL(s) },
		"hasPrefix": strings.HasPrefix,
		"json": func(v interface{}) template.JS {
			b, err := json.Marshal(v)
			if err != nil {
				return template.JS("{}")
			}
			return template.JS(b)
		},
		// String helpers
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		// Math helpers for templates
		"sub": func(a, b int) int {
			return a - b
		},
		"add": func(a, b int) int {
			return a + b
		},
		"divf": func(a, b any) float64 {
			af := toFloat(a)
			bf := toFloat(b)
			if bf == 0 {
				return 0
			}
			return af / bf
		},
		"mulf": func(a, b any) float64 {
			af := toFloat(a)
			bf := toFloat(b)
			return af * bf
		},
		// Pointer dereference helpers
		"deref": func(p *float64) float64 {
			if p == nil {
				return 0
			}
			return *p
		},
	}
}

// SetMenuItems sets menu items (legacy - puts all in user section)
func (te *TemplateEngine) SetMenuItems(items []MenuItem) {
	te.mu.Lock()
	defer te.mu.Unlock()
	te.userMenu = items
}

// SetMenuSections sets user and admin menu items separately
func (te *TemplateEngine) SetMenuSections(userItems, adminItems []MenuItem) {
	te.mu.Lock()
	defer te.mu.Unlock()
	te.userMenu = userItems
	te.adminMenu = adminItems
}

// SetWidgets sets dashboard widgets
func (te *TemplateEngine) SetWidgets(widgets []Widget) {
	te.mu.Lock()
	defer te.mu.Unlock()
	te.widgets = widgets
}

// SetRequestFuncs registers functions rebuilt for every render. Their names
// must already exist in the parse-time func map.
func (te *TemplateEngine) SetRequestFuncs(fn RequestFuncs) {
	te.mu.Lock()
	defer te.mu.Unlock()
	te.requestFuncs = fn
}

func (te *TemplateEngine) AddFuncs(funcs template.FuncMap) {
	te.mu.Lock()
	defer te.mu.Unlock()
	for k, v := range funcs {
		te.funcs[k] = v
	}
}

// RegisterModule templates from a module's fs.FS
func (te *TemplateEngine) RegisterModule(moduleID string, tmplFS fs.FS) error {
	if tmplFS == nil {
		return nil
	}

	te.mu.Lock()
	defer te.mu.Unlock()

	// Collect partials first (files in partials/ directory)
	var partials [][]byte
	err := fs.WalkDir(tmplFS, "partials", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".html" {
			return nil
		}
		content, err := fs.ReadFile(tmplFS, path)
		if err != nil {
			return nil
		}
		partials = append(partials, content)
		return nil
	})
	// If partials directory doesn't exist, that's fine - not all modules have partials
	if err != nil {
		// Ignore the error - partials are optional
	}

	// Load global protection partial (shared across all modules)
	protPartial, protErr := fs.ReadFile(protection.TemplatesFS, "templates/settings.html")
	if protErr == nil {
		partials = append(partials, protPartial)
	}

	return fs.WalkDir(tmplFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if filepath.Ext(path) != ".html" {
			return nil
		}
		// Skip partials directory - they're included in other templates
		if filepath.Dir(path) == "partials" {
			return nil
		}

		content, err := fs.ReadFile(tmplFS, path)
		if err != nil {
			return err
		}

		// Parse with layouts
		// Create the root template - all associated templates will be accessible from it
		root := template.New(path).Funcs(te.funcs)

		// Parse layout files first
		if te.layouts != nil {
			layoutFiles, _ := fs.Glob(te.layouts, "*.html")
			for _, lf := range layoutFiles {
				lContent, err := fs.ReadFile(te.layouts, lf)
				if err != nil {
					continue
				}
				// Parse returns a different template if content has {{define}},
				// but all templates stay associated with root
				root.Parse(string(lContent))
			}
		}

		// Parse module partials - they define named templates that become associated
		for i, partial := range partials {
			_, parseErr := root.Parse(string(partial))
			if parseErr != nil {
				log.Printf("[templates] Module %s template %s: error parsing partial %d: %v", moduleID, path, i, parseErr)
			}
		}

		// Parse module template content
		_, err = root.Parse(string(content))
		if err != nil {
			return err
		}

		// Store the root template - it has access to all associated templates
		key := moduleID + ":" + path
		te.templates[key] = root
		return nil
	})
}

// menuMode selects which navigation the layout renders. The two sections are
// kept apart: an admin page shows admin navigation only, and vice versa.
type menuMode int

const (
	menuUser menuMode = iota
	menuAdmin
	menuNone
)

// Render renders an admin page: admin navigation only.
func (te *TemplateEngine) Render(w io.Writer, r *http.Request, name string, data map[string]interface{}) error {
	return te.renderWithLayout(w, r, name, "base", data, menuAdmin)
}

// RenderUserSection renders a user page: user navigation only.
func (te *TemplateEngine) RenderUserSection(w io.Writer, r *http.Request, name string, data map[string]interface{}) error {
	return te.renderWithLayout(w, r, name, "base", data, menuUser)
}

// RenderAuth renders a template with the auth layout (no sidebar)
func (te *TemplateEngine) RenderAuth(w io.Writer, r *http.Request, name string, data map[string]interface{}) error {
	return te.renderWithLayout(w, r, name, "auth", data, menuNone)
}

func (te *TemplateEngine) renderWithLayout(w io.Writer, r *http.Request, name, layout string, data map[string]interface{}, menu menuMode) error {
	te.mu.RLock()
	tmpl, ok := te.templates[name]
	requestFuncs := te.requestFuncs
	userMenu := te.userMenu
	adminMenu := te.adminMenu
	widgets := te.widgets
	te.mu.RUnlock()

	if !ok {
		log.Printf("[templates] Template not found: %s", name)
		return nil // Template not found
	}

	// Templates are shared across requests, so per-request functions go onto a
	// clone -- calling Funcs on the stored template would race.
	if requestFuncs != nil {
		clone, err := tmpl.Clone()
		if err != nil {
			return err
		}
		tmpl = clone.Funcs(requestFuncs(r))
	}

	// Inject common data
	if data == nil {
		data = make(map[string]interface{})
	}
	switch menu {
	case menuAdmin:
		data["UserMenu"] = nil
		data["AdminMenu"] = adminMenu
	case menuUser:
		data["UserMenu"] = userMenu
		data["AdminMenu"] = nil
	default:
		data["UserMenu"] = nil
		data["AdminMenu"] = nil
	}
	data["MenuItems"] = data["UserMenu"] // Legacy compatibility
	data["IsAdminSection"] = menu == menuAdmin
	data["Widgets"] = widgets

	// The signed-in user, so the layout can name the account and offer the
	// admin section to those who have it. Handlers may override it.
	if _, set := data["User"]; !set && r != nil {
		data["User"] = ctx.GetUser(r)
	}

	// Subscription state, so the layout can warn a user whose access has
	// lapsed. This drives presentation only -- writes are refused by the
	// middleware regardless of what the page shows.
	if _, set := data["Subscription"]; !set && r != nil {
		data["Subscription"] = subscription.FromRequest(r)
	}

	// Domain stats, so the layout can show promotional banners to users
	// who have not added any domains yet.
	if _, set := data["DomainStats"]; !set && r != nil {
		data["DomainStats"] = domainstats.FromRequest(r)
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, layout, data); err != nil {
		log.Printf("[templates] Execution error for %s: %v", name, err)
		return err
	}

	_, err := buf.WriteTo(w)
	return err
}
