package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/botginx/botginx/modules/telegram"
	"github.com/botginx/botginx/pkg/module"
	"github.com/botginx/botginx/pkg/proxy"
	"github.com/jmoiron/sqlx"
)

type Handler struct {
	db             *sqlx.DB
	templates      *module.TemplateEngine
	telegramModule *telegram.Module
	proxyService   *proxy.Service
}

func NewHandler(db *sqlx.DB, templates *module.TemplateEngine, telegramModule *telegram.Module, proxyService *proxy.Service) *Handler {
	return &Handler{
		db:             db,
		templates:      templates,
		telegramModule: telegramModule,
		proxyService:   proxyService,
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
		{
			ID:          "proxy",
			Title:       "Proxy",
			Description: "Configure proxy for cPanel API connections when server IP is blocked",
			Icon:        "bi-shield-lock",
			Path:        "/admin/settings/proxy",
		},
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

// Proxy shows the proxy settings page
func (h *Handler) Proxy(w http.ResponseWriter, r *http.Request) {
	config, err := h.proxyService.Get()
	if err != nil || config == nil {
		// Return empty config if not found
		config = &proxy.Config{
			Host:           "global.rotgb.711proxy.com",
			Port:           10000,
			Zone:           "custom",
			SessionMinutes: 10,
		}
	}

	module.Render(w, r, h.templates, "settings:proxy.html", map[string]interface{}{
		"Title":  "Proxy Settings",
		"Config": config,
	})
}

// APIGetProxy returns the proxy configuration
func (h *Handler) APIGetProxy(w http.ResponseWriter, r *http.Request) {
	config, err := h.proxyService.Get()
	if err != nil {
		h.jsonError(w, "Failed to get proxy config", http.StatusInternalServerError)
		return
	}
	h.jsonSuccess(w, config)
}

// APIUpdateProxy updates the proxy configuration
func (h *Handler) APIUpdateProxy(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Enabled        bool   `json:"enabled"`
		Username       string `json:"username"`
		Password       string `json:"password"`
		Host           string `json:"host"`
		Port           int    `json:"port"`
		Zone           string `json:"zone"`
		SessionMinutes int    `json:"session_minutes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Set defaults
	if input.Host == "" {
		input.Host = "global.rotgb.711proxy.com"
	}
	if input.Port == 0 {
		input.Port = 10000
	}
	if input.Zone == "" {
		input.Zone = "custom"
	}
	if input.SessionMinutes == 0 {
		input.SessionMinutes = 10
	}

	config := &proxy.Config{
		Enabled:        input.Enabled,
		Username:       input.Username,
		Password:       input.Password,
		Host:           input.Host,
		Port:           input.Port,
		Zone:           input.Zone,
		SessionMinutes: input.SessionMinutes,
	}

	if err := h.proxyService.Update(config); err != nil {
		h.jsonError(w, "Failed to update proxy config", http.StatusInternalServerError)
		return
	}

	h.jsonSuccess(w, map[string]string{"message": "Proxy settings updated"})
}

// APITestProxy tests the proxy connection
func (h *Handler) APITestProxy(w http.ResponseWriter, r *http.Request) {
	if err := h.proxyService.TestConnection(); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.jsonSuccess(w, map[string]string{"message": "Proxy connection successful"})
}

func (h *Handler) jsonError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   msg,
	})
}

func (h *Handler) jsonSuccess(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    data,
	})
}

// parseInt helper
func parseInt(s string, def int) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}
