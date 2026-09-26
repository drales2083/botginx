package handlers

import (
	"encoding/json"
	"html/template"
	"net/http"

	"github.com/botginx/botginx/modules/antibotcontrol/services"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
	"github.com/botginx/botginx/pkg/protection"
)

type Handler struct {
	service   *services.AntibotControlService
	templates *module.TemplateEngine
}

func NewHandler(service *services.AntibotControlService, tmpl *module.TemplateEngine) *Handler {
	return &Handler{
		service:   service,
		templates: tmpl,
	}
}

func (h *Handler) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) jsonError(w http.ResponseWriter, msg string, status int) {
	h.json(w, status, map[string]string{"error": msg})
}

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	defaults, err := h.service.GetDefaults(userID)
	if err != nil {
		defaults = &protection.Settings{}
	}

	// Serialize defaults to JSON for JavaScript initialization
	defaultsJSON, _ := json.Marshal(defaults)

	module.RenderUserSection(w, r, h.templates, "antibotcontrol:index.html", map[string]interface{}{
		"Title":        "Antibot Control",
		"Defaults":     defaults,
		"DefaultsJSON": template.JS(defaultsJSON),
		"Countries":    protection.GetCountries(),
	})
}

func (h *Handler) APIGetDefaults(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	defaults, err := h.service.GetDefaults(userID)
	if err != nil {
		h.jsonError(w, "Failed to get defaults", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, defaults)
}

func (h *Handler) APISaveDefaults(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var settings protection.Settings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.service.SaveDefaults(userID, &settings); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Default settings saved",
	})
}

func (h *Handler) APIResetDefaults(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	if err := h.service.ResetDefaults(userID); err != nil {
		h.jsonError(w, "Failed to reset defaults", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Settings reset to system defaults",
	})
}

