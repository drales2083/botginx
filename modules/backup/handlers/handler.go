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

func (h *Handler) Restore(w http.ResponseWriter, r *http.Request) {
	module.Render(w, r, h.templates, "backup:restore.html", map[string]interface{}{
		"Title": "Restore Backup",
	})
}

func (h *Handler) APIValidateManifest(w http.ResponseWriter, r *http.Request) {
	var manifest services.RestoreManifest
	if err := json.NewDecoder(r.Body).Decode(&manifest); err != nil {
		h.jsonError(w, "Invalid manifest JSON", http.StatusBadRequest)
		return
	}

	if err := h.service.ValidateManifest(&manifest); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Calculate total size
	var totalBytes int64
	var totalChunks int
	for _, f := range manifest.Files {
		totalBytes += f.Size
		if f.Chunked {
			totalChunks += len(f.Chunks)
		}
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"valid":       true,
		"backupId":    manifest.BackupID,
		"timestamp":   manifest.Timestamp,
		"host":        manifest.Host,
		"totalFiles":  len(manifest.Files),
		"totalBytes":  totalBytes,
		"totalChunks": totalChunks,
	})
}

func (h *Handler) APIRunRestore(w http.ResponseWriter, r *http.Request) {
	var manifest services.RestoreManifest
	if err := json.NewDecoder(r.Body).Decode(&manifest); err != nil {
		h.jsonError(w, "Invalid manifest JSON", http.StatusBadRequest)
		return
	}

	if err := h.service.ValidateManifest(&manifest); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Run restore synchronously (for now - could be background job)
	result, err := h.service.RunRestore(context.Background(), &manifest, nil)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, result)
}

func (h *Handler) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) jsonError(w http.ResponseWriter, message string, status int) {
	h.json(w, status, map[string]interface{}{"error": message})
}
