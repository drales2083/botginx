package handlers

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/botginx/botginx/modules/apikeys/services"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service   *services.APIKeysService
	templates *module.TemplateEngine
}

func NewHandler(service *services.APIKeysService, tmpl *module.TemplateEngine) *Handler {
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

	keys, err := h.service.List(userID)
	if err != nil {
		keys = []services.APIKey{}
	}

	module.RenderUserSection(w, r, h.templates, "apikeys:index.html", map[string]interface{}{
		"Title":        "API Keys",
		"Keys":         keys,
		"Scopes":       services.ValidScopes,
		"ScopePresets": services.ScopePresets,
	})
}

func (h *Handler) Docs(w http.ResponseWriter, r *http.Request) {
	// Use PANEL_URL env var, fallback to request host
	baseURL := os.Getenv("PANEL_URL")
	if baseURL == "" {
		scheme := "https"
		if r.TLS == nil {
			scheme = "http"
		}
		baseURL = scheme + "://" + r.Host
	} else if baseURL[0] != 'h' {
		// Add https:// if just a hostname
		baseURL = "https://" + baseURL
	}

	appName := os.Getenv("UI_APP_NAME")
	if appName == "" {
		appName = "GuardBot"
	}

	module.RenderUserSection(w, r, h.templates, "apikeys:docs.html", map[string]interface{}{
		"Title":   "API Documentation",
		"Scopes":  services.ValidScopes,
		"BaseURL": baseURL,
		"AppName": appName,
	})
}

func (h *Handler) APIList(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	keys, err := h.service.List(userID)
	if err != nil {
		h.jsonError(w, "Failed to list keys", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, keys)
}

func (h *Handler) APICreate(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var req services.CreateKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	key, err := h.service.Create(userID, &req)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, key)
}

func (h *Handler) APIGet(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	keyID := chi.URLParam(r, "id")

	key, err := h.service.Get(userID, keyID)
	if err != nil {
		h.jsonError(w, "Key not found", http.StatusNotFound)
		return
	}

	h.json(w, http.StatusOK, key)
}

func (h *Handler) APIRevoke(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	keyID := chi.URLParam(r, "id")

	if err := h.service.Revoke(userID, keyID); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Key revoked",
	})
}

func (h *Handler) APIDelete(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	keyID := chi.URLParam(r, "id")

	if err := h.service.Delete(userID, keyID); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Key deleted",
	})
}

func (h *Handler) APIRotate(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	keyID := chi.URLParam(r, "id")

	key, err := h.service.Rotate(userID, keyID)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusOK, key)
}

func (h *Handler) APIScopes(w http.ResponseWriter, r *http.Request) {
	h.json(w, http.StatusOK, map[string]interface{}{
		"scopes":  services.ValidScopes,
		"presets": services.ScopePresets,
	})
}
