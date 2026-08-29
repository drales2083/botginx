package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/botginx/botginx/modules/modules/services"
	"github.com/botginx/botginx/pkg/lifecycle"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service   *services.ModuleService
	templates *module.TemplateEngine
}

func NewHandler(service *services.ModuleService, templates *module.TemplateEngine) *Handler {
	return &Handler{
		service:   service,
		templates: templates,
	}
}

// Page handlers

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	modules, err := h.service.List()
	if err != nil {
		// Surfaced rather than swallowed: an empty list and a failed query look
		// identical on the page, which hid a scan error here before.
		http.Error(w, "Failed to load modules: "+err.Error(), http.StatusInternalServerError)
		return
	}

	module.Render(w, r, h.templates, "modules:list.html", map[string]interface{}{
		"Title":   "Modules",
		"Modules": modules,
	})
}

func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	state, err := h.service.Get(id)
	if err != nil {
		http.Error(w, "Module not found", http.StatusNotFound)
		return
	}

	module.Render(w, r, h.templates, "modules:show.html", map[string]interface{}{
		"Title":  state.Name,
		"Module": state,
	})
}

// APIRestart restarts the process so pending module changes take effect.
//
// The response is written and flushed before the restart is requested, so the
// caller gets an answer rather than a dropped connection.
func (h *Handler) APIRestart(w http.ResponseWriter, r *http.Request) {
	h.json(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Restarting. The panel will be back in a few seconds.",
	})

	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	// Briefly delayed so this response reaches the browser before the listener
	// stops accepting connections.
	go func() {
		time.Sleep(500 * time.Millisecond)
		lifecycle.RequestRestart()
	}()
}

// API handlers

func (h *Handler) APIList(w http.ResponseWriter, r *http.Request) {
	modules, err := h.service.List()
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.json(w, http.StatusOK, map[string]interface{}{"modules": modules})
}

func (h *Handler) APIGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	state, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Module not found", http.StatusNotFound)
		return
	}
	h.json(w, http.StatusOK, map[string]interface{}{"module": state})
}

func (h *Handler) APIEnable(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.Enable(id); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.json(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Module enabled. Restart server to apply changes.",
	})
}

func (h *Handler) APIDisable(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Check if core
	state, _ := h.service.Get(id)
	if state != nil && state.IsCore {
		h.jsonError(w, "Cannot disable core module", http.StatusForbidden)
		return
	}

	if err := h.service.Disable(id); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.json(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Module disabled. Restart server to apply changes.",
	})
}

func (h *Handler) APIUpdateSettings(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var input struct {
		Settings string `json:"settings"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.service.UpdateSettings(id, input.Settings); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

// Helpers

func (h *Handler) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) jsonError(w http.ResponseWriter, message string, status int) {
	h.json(w, status, map[string]interface{}{"error": message})
}
