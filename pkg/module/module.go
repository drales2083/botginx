package module

import (
	"io/fs"
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

// MenuSection defines where menu items appear
type MenuSection string

const (
	MenuSectionUser  MenuSection = "user"  // User features (Redirect Links, Domains, etc.)
	MenuSectionAdmin MenuSection = "admin" // Admin features (Users, Settings, etc.)
)

// Module interface — every feature implements this
type Module interface {
	// Identity
	ID() string
	Name() string
	Description() string

	// Lifecycle
	Init(deps *Dependencies) error
	Migrate() error

	// HTTP
	Routes() chi.Router

	// Templates (optional)
	Templates() fs.FS

	// Sidebar menu items
	MenuItems() []MenuItem

	// Dashboard widgets (optional)
	Widgets() []Widget
}

// Dependencies injected into each module
type Dependencies struct {
	DB        *sqlx.DB
	Router    chi.Router
	Templates *TemplateEngine
	Config    map[string]interface{}
}

// MenuItem for sidebar navigation
type MenuItem struct {
	Title    string
	Icon     string      // Bootstrap icon class
	Path     string
	Badge    string      // Optional badge text
	Children []MenuItem  // Nested items
	Order    int         // Sort order
	Section  MenuSection // admin or user section
	Hidden   bool        // If true, routes mount but menu item doesn't show
	Group    string      // Group name for collapsible menus (Links, Infrastructure, Billing, Help)
}

// Widget for dashboard
type Widget struct {
	ID       string                 // Unique widget ID
	Title    string                 // Widget title
	Icon     string                 // Bootstrap icon
	Size     string                 // "small", "medium", "large"
	Order    int                    // Display order
	Template string                 // Template name (module:widget.html)
	Data     func() (interface{}, error) // Dynamic data loader
}

// Registry holds all registered modules
type Registry struct {
	modules map[string]Module
	order   []string
}

func NewRegistry() *Registry {
	return &Registry{
		modules: make(map[string]Module),
		order:   []string{},
	}
}

func (r *Registry) Register(m Module) {
	r.modules[m.ID()] = m
	r.order = append(r.order, m.ID())
}

func (r *Registry) Get(id string) (Module, bool) {
	m, ok := r.modules[id]
	return m, ok
}

func (r *Registry) All() []Module {
	result := make([]Module, 0, len(r.order))
	for _, id := range r.order {
		if m, ok := r.modules[id]; ok {
			result = append(result, m)
		}
	}
	return result
}

func (r *Registry) InitAll(deps *Dependencies) error {
	for _, m := range r.All() {
		if err := m.Init(deps); err != nil {
			return err
		}
	}
	return nil
}

func (r *Registry) MigrateAll() error {
	for _, m := range r.All() {
		if err := m.Migrate(); err != nil {
			return err
		}
	}
	return nil
}

// CollectMenuItems returns all menu items (legacy - returns all)
func (r *Registry) CollectMenuItems() []MenuItem {
	var items []MenuItem
	for _, m := range r.All() {
		items = append(items, m.MenuItems()...)
	}
	sortMenuItems(items)
	return items
}

// CollectMenuBySection returns menu items for a specific section, grouped by Group field
func (r *Registry) CollectMenuBySection(section MenuSection) []MenuItem {
	var items []MenuItem
	for _, m := range r.All() {
		for _, item := range m.MenuItems() {
			if item.Section == section || (item.Section == "" && section == MenuSectionUser) {
				items = append(items, item)
			}
		}
	}
	sortMenuItems(items)
	return groupMenuItems(items)
}

// groupMenuItems organizes items into collapsible groups
func groupMenuItems(items []MenuItem) []MenuItem {
	// Define group metadata (order, icon)
	groupMeta := map[string]struct {
		Icon  string
		Order int
	}{
		// User section groups
		"Links":          {Icon: "bi-link-45deg", Order: 10},
		"Infrastructure": {Icon: "bi-server", Order: 30},
		"Billing":        {Icon: "bi-wallet2", Order: 40},
		"Help":           {Icon: "bi-question-circle", Order: 50},
		// Admin section groups
		"Admin Infrastructure": {Icon: "bi-hdd-stack", Order: 20},
		"Revenue":              {Icon: "bi-currency-dollar", Order: 30},
		"Users & Support":      {Icon: "bi-people", Order: 40},
		"System":               {Icon: "bi-gear", Order: 50},
	}

	// Separate grouped and ungrouped items
	groups := make(map[string][]MenuItem)
	var ungrouped []MenuItem

	for _, item := range items {
		if item.Group != "" {
			groups[item.Group] = append(groups[item.Group], item)
		} else {
			ungrouped = append(ungrouped, item)
		}
	}

	// Build result: ungrouped items + group parents with children
	var result []MenuItem

	// Add ungrouped items first (they keep their original order)
	for _, item := range ungrouped {
		result = append(result, item)
	}

	// Add grouped items as collapsible parents
	for groupName, children := range groups {
		meta, ok := groupMeta[groupName]
		if !ok {
			meta = struct {
				Icon  string
				Order int
			}{Icon: "bi-folder", Order: 99}
		}

		// Sort children by their order
		sortMenuItems(children)

		parent := MenuItem{
			Title:    groupName,
			Icon:     meta.Icon,
			Path:     "#", // No direct path, just expands
			Order:    meta.Order,
			Children: children,
		}
		result = append(result, parent)
	}

	// Sort final result by order
	sortMenuItems(result)
	return result
}

// CollectWidgets returns all dashboard widgets
func (r *Registry) CollectWidgets() []Widget {
	var widgets []Widget
	for _, m := range r.All() {
		widgets = append(widgets, m.Widgets()...)
	}
	sort.Slice(widgets, func(i, j int) bool {
		return widgets[i].Order < widgets[j].Order
	})
	return widgets
}

func sortMenuItems(items []MenuItem) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].Order < items[j].Order
	})
}

func (r *Registry) MountRoutes(router chi.Router) {
	for _, m := range r.All() {
		routes := m.Routes()
		if routes != nil {
			router.Mount("/"+m.ID(), routes)
		}
	}
}

func (r *Registry) MountRoutesExcept(router chi.Router, excludeIDs ...string) {
	exclude := make(map[string]bool)
	for _, id := range excludeIDs {
		exclude[id] = true
	}

	for _, m := range r.All() {
		if exclude[m.ID()] {
			continue
		}
		routes := m.Routes()
		if routes != nil {
			router.Mount("/"+m.ID(), routes)
		}
	}
}

// MountRoutesBySection mounts only modules that belong to a specific section
func (r *Registry) MountRoutesBySection(router chi.Router, section MenuSection, excludeIDs ...string) {
	exclude := make(map[string]bool)
	for _, id := range excludeIDs {
		exclude[id] = true
	}

	for _, m := range r.All() {
		if exclude[m.ID()] {
			continue
		}
		// Check if this module's menu items belong to the requested section
		menuItems := m.MenuItems()
		belongsToSection := false
		for _, item := range menuItems {
			if item.Section == section || (item.Section == "" && section == MenuSectionUser) {
				belongsToSection = true
				break
			}
		}
		// Dashboard is special - it exists in both sections
		if m.ID() == "dashboard" {
			belongsToSection = true
		}
		if !belongsToSection {
			continue
		}
		// A module serving both sections (domains: own vs shared) picks its
		// routes per section; everything else mounts the same router in the
		// one section it belongs to.
		var routes chi.Router
		if sr, ok := m.(SectionRouter); ok {
			routes = sr.RoutesForSection(section)
		} else {
			routes = m.Routes()
		}
		if routes != nil {
			router.Mount("/"+m.ID(), routes)
		}
	}
}

// SectionRouter is an optional interface for modules that expose different
// routes in the user and admin sections.
type SectionRouter interface {
	RoutesForSection(section MenuSection) chi.Router
}

// BaseModule provides common functionality
type BaseModule struct {
	id          string
	name        string
	description string
	deps        *Dependencies
}

func NewBaseModule(id, name, description string) *BaseModule {
	return &BaseModule{
		id:          id,
		name:        name,
		description: description,
	}
}

func (b *BaseModule) ID() string             { return b.id }
func (b *BaseModule) Name() string           { return b.name }
func (b *BaseModule) Description() string    { return b.description }
func (b *BaseModule) Templates() fs.FS       { return nil }
func (b *BaseModule) MenuItems() []MenuItem  { return nil }
func (b *BaseModule) Widgets() []Widget      { return nil }

func (b *BaseModule) SetDeps(deps *Dependencies) {
	b.deps = deps
}

func (b *BaseModule) DB() *sqlx.DB {
	return b.deps.DB
}

// Render helper for handlers (shows admin menu if user is admin).
// The request is needed so translations follow the visitor's language.
func Render(w http.ResponseWriter, r *http.Request, te *TemplateEngine, name string, data map[string]interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := te.Render(w, r, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// RenderUserSection renders without admin menu (for /user/* routes)
func RenderUserSection(w http.ResponseWriter, r *http.Request, te *TemplateEngine, name string, data map[string]interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := te.RenderUserSection(w, r, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// RenderAuth renders with auth layout (no sidebar, centered form)
func RenderAuth(w http.ResponseWriter, r *http.Request, te *TemplateEngine, name string, data map[string]interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := te.RenderAuth(w, r, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// JSON helper
func JSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// encoding handled by caller
}
