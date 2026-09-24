package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/botginx/botginx/modules/backup/models"
	"github.com/botginx/botginx/modules/backup/services"
	"github.com/botginx/botginx/pkg/module"
)

type Handler struct {
	service   *services.BackupService
	templates *module.TemplateEngine
}

func NewHandler(service *services.BackupService, templates *module.TemplateEngine) *Handler {
	return &Handler{
		service:   service,
		templates: templates,
	}
}

func (h *Handler) Settings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.service.GetSettings()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	module.Render(w, r, h.templates, "backup:settings.html", map[string]interface{}{
		"Title":    "Backup Settings",
		"Settings": settings,
	})
}

func (h *Handler) History(w http.ResponseWriter, r *http.Request) {
	history, err := h.service.GetHistory(50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	module.Render(w, r, h.templates, "backup:history.html", map[string]interface{}{
		"Title":   "Backup History",
		"History": history,
	})
}

func (h *Handler) APIUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var input models.UpdateSettingsInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := h.service.UpdateSettings(input); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

func (h *Handler) APIRunBackup(w http.ResponseWriter, r *http.Request) {
	history, err := h.service.RunBackup(context.Background())
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"backup":  history,
	})
}

func (h *Handler) APIGetHistory(w http.ResponseWriter, r *http.Request) {
	history, err := h.service.GetHistory(50)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, history)
}

func (h *Handler) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) jsonError(w http.ResponseWriter, message string, status int) {
	h.json(w, status, map[string]interface{}{"error": message})
}
