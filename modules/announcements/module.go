package announcements

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"time"

	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html
var templatesFS embed.FS

type Module struct {
	*module.BaseModule
	templates *module.TemplateEngine
}

func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"announcements",
			"Announcements",
			"News announcements for user dashboard",
		),
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)
	m.templates = deps.Templates

	tmplFS, _ := fs.Sub(templatesFS, "templates")
	deps.Templates.RegisterModule(m.ID(), tmplFS)

	return nil
}

func (m *Module) Migrate() error {
	return nil
}

func (m *Module) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", m.handleUserList)
	return r
}

func (m *Module) handleUserList(w http.ResponseWriter, r *http.Request) {
	var items []announcement
	m.DB().Select(&items, `
		SELECT id, title, body, badge, is_pinned, published_at
		FROM announcements
		ORDER BY is_pinned DESC, published_at DESC
		LIMIT 50
	`)

	module.RenderUserSection(w, r, m.templates, "announcements:user.html", map[string]interface{}{
		"Title":         "Announcements",
		"Announcements": items,
	})
}

func (m *Module) RoutesForSection(section module.MenuSection) chi.Router {
	if section == module.MenuSectionAdmin {
		r := chi.NewRouter()
		r.Get("/", m.handleList)
		r.Get("/api/list", m.apiList)
		r.Post("/api/create", m.apiCreate)
		r.Put("/api/{id}", m.apiUpdate)
		r.Delete("/api/{id}", m.apiDelete)
		return r
	}
	if section == module.MenuSectionUser {
		r := chi.NewRouter()
		r.Get("/", m.handleUserList)
		return r
	}
	return nil
}

func (m *Module) MenuItems() []module.MenuItem {
	return []module.MenuItem{
		{
			Title:   "Announcements",
			Icon:    "bi-megaphone",
			Path:    "/user/announcements",
			Order:   60,
			Section: module.MenuSectionUser,
			Group:   "Support",
		},
		{
			Title:   "Announcements",
			Icon:    "bi-megaphone",
			Path:    "/admin/announcements",
			Order:   10,
			Section: module.MenuSectionAdmin,
			Group:   "System",
		},
	}
}

func (m *Module) Templates() fs.FS {
	tmplFS, _ := fs.Sub(templatesFS, "templates")
	return tmplFS
}

func (m *Module) Widgets() []module.Widget {
	return nil
}

type announcement struct {
	ID          string    `db:"id" json:"id"`
	Title       string    `db:"title" json:"title"`
	Body        string    `db:"body" json:"body"`
	Badge       *string   `db:"badge" json:"badge"`
	IsPinned    bool      `db:"is_pinned" json:"is_pinned"`
	PublishedAt time.Time `db:"published_at" json:"published_at"`
}

func (m *Module) handleList(w http.ResponseWriter, r *http.Request) {
	module.Render(w, r, m.templates, "announcements:index.html", map[string]interface{}{
		"Title": "Announcements",
	})
}

func (m *Module) apiList(w http.ResponseWriter, r *http.Request) {
	var items []announcement
	m.DB().Select(&items, `
		SELECT id, title, body, badge, is_pinned, published_at
		FROM announcements
		ORDER BY is_pinned DESC, published_at DESC
	`)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

func (m *Module) apiCreate(w http.ResponseWriter, r *http.Request) {
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

func (m *Module) apiUpdate(w http.ResponseWriter, r *http.Request) {
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

func (m *Module) apiDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	_, err := m.DB().Exec(`DELETE FROM announcements WHERE id = $1`, id)
	if err != nil {
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}
