package handlers

import (
	"net/http"

	"github.com/botginx/botginx/modules/telegram"
	"github.com/botginx/botginx/pkg/module"
	"github.com/jmoiron/sqlx"
)

type Handler struct {
	db             *sqlx.DB
	templates      *module.TemplateEngine
	telegramModule *telegram.Module
}

func NewHandler(db *sqlx.DB, templates *module.TemplateEngine, telegramModule *telegram.Module) *Handler {
	return &Handler{
		db:             db,
		templates:      templates,
		telegramModule: telegramModule,
	}
}

// SettingsSection represents a settings subsection
type SettingsSection struct {
	ID          string
	Title       string
	Description string
	Icon        string
	Path        string
}

// Index shows the main settings page with all sections
func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	sections := []SettingsSection{
		{
			ID:          "telegram",
			Title:       "Telegram",
			Description: "Configure Telegram bots for notifications and ticket replies",
			Icon:        "bi-telegram",
			Path:        "/admin/settings/telegram",
		},
		// Future sections can be added here:
		// {ID: "email", Title: "Email", Description: "SMTP and email notification settings", Icon: "bi-envelope", Path: "/admin/settings/email"},
		// {ID: "general", Title: "General", Description: "General application settings", Icon: "bi-sliders", Path: "/admin/settings/general"},
	}

	module.Render(w, r, h.templates, "settings:index.html", map[string]interface{}{
		"Title":    "Settings",
		"Sections": sections,
	})
}

// Telegram shows the Telegram settings page
func (h *Handler) Telegram(w http.ResponseWriter, r *http.Request) {
	bots, _ := h.telegramModule.Service.ListBots()

	module.Render(w, r, h.templates, "settings:telegram.html", map[string]interface{}{
		"Title": "Telegram Settings",
		"Bots":  bots,
	})
}
