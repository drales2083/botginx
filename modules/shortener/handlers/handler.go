package handlers

import (
	"encoding/json"
	"net/http"

	domainmodels "github.com/botginx/botginx/modules/domains/models"
	"github.com/botginx/botginx/modules/shortener/models"
	"github.com/botginx/botginx/modules/shortener/services"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
	"github.com/botginx/botginx/pkg/protection"
	"github.com/go-chi/chi/v5"
	"github.com/skip2/go-qrcode"
)

type DomainProvider interface {
	ListAvailable(userID string) ([]domainmodels.Domain, error)
}

type Handler struct {
	service   *services.ShortenerService
	templates *module.TemplateEngine
	domains   DomainProvider
}

func NewHandler(service *services.ShortenerService, templates *module.TemplateEngine, domains DomainProvider) *Handler {
	return &Handler{
		service:   service,
		templates: templates,
		domains:   domains,
	}
}

func (h *Handler) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) jsonError(w http.ResponseWriter, message string, status int) {
	h.json(w, status, map[string]string{"error": message})
}

// Page Handlers

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	links, _ := h.service.List(userID)

	module.RenderUserSection(w, r, h.templates, "shortener:list.html", map[string]interface{}{
		"Title": "Short Links",
		"Links": links,
	})
}

func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	domains, _ := h.domains.ListAvailable(userID)

	module.RenderUserSection(w, r, h.templates, "shortener:new.html", map[string]interface{}{
		"Title":             "Create Short Link",
		"Domains":           domains,
		"DefaultProtection": protection.GetDefaultSettings(),
		"BotErrors":         getBotErrors(),
	})
}

func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	link, err := h.service.Get(id)
	if err != nil {
		http.Error(w, "Link not found", http.StatusNotFound)
		return
	}

	// Verify ownership
	userID := ctx.GetUserID(r)
	if link.UserID != userID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	stats, _ := h.service.GetStats(id, 30)

	module.RenderUserSection(w, r, h.templates, "shortener:show.html", map[string]interface{}{
		"Title":     "Short Link",
		"Link":      link,
		"Stats":     stats,
		"BotErrors": getBotErrors(),
		"Settings":  link.ProtectionSettings,
	})
}

func (h *Handler) QRCode(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	link, err := h.service.Get(id)
	if err != nil {
		http.Error(w, "Link not found", http.StatusNotFound)
		return
	}

	size := 256
	if s := r.URL.Query().Get("size"); s == "512" {
		size = 512
	}

	png, err := qrcode.Encode(link.FullURL(), qrcode.Medium, size)
	if err != nil {
		http.Error(w, "Failed to generate QR code", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(png)
}

// API Handlers

func (h *Handler) APIList(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	links, err := h.service.List(userID)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.json(w, http.StatusOK, links)
}

func (h *Handler) APIGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	link, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Link not found", http.StatusNotFound)
		return
	}

	userID := ctx.GetUserID(r)
	if link.UserID != userID {
		h.jsonError(w, "Forbidden", http.StatusForbidden)
		return
	}

	h.json(w, http.StatusOK, link)
}

func (h *Handler) APICreate(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var input models.CreateShortLinkInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate
	if input.DomainID == "" {
		h.jsonError(w, "Domain is required", http.StatusBadRequest)
		return
	}
	if input.Path == "" {
		h.jsonError(w, "Path is required", http.StatusBadRequest)
		return
	}
	if len(input.Destinations) == 0 {
		h.jsonError(w, "At least one destination URL is required", http.StatusBadRequest)
		return
	}

	// Check path availability
	available, _ := h.service.CheckPathAvailable(input.DomainID, input.Path)
	if !available {
		h.jsonError(w, "Path already in use", http.StatusConflict)
		return
	}

	link, err := h.service.Create(userID, input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusCreated, link)
}

func (h *Handler) APIUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Verify ownership
	link, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Link not found", http.StatusNotFound)
		return
	}
	userID := ctx.GetUserID(r)
	if link.UserID != userID {
		h.jsonError(w, "Forbidden", http.StatusForbidden)
		return
	}

	var input models.UpdateShortLinkInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	updated, err := h.service.Update(id, input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, updated)
}

func (h *Handler) APIDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Verify ownership
	link, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Link not found", http.StatusNotFound)
		return
	}
	userID := ctx.GetUserID(r)
	if link.UserID != userID {
		h.jsonError(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := h.service.Delete(id); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) APIRandomPath(w http.ResponseWriter, r *http.Request) {
	path := h.service.GenerateRandomPath(6)
	h.json(w, http.StatusOK, map[string]string{"path": path})
}

func (h *Handler) APIStats(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Verify ownership
	link, err := h.service.Get(id)
	if err != nil {
		h.jsonError(w, "Link not found", http.StatusNotFound)
		return
	}
	userID := ctx.GetUserID(r)
	if link.UserID != userID {
		h.jsonError(w, "Forbidden", http.StatusForbidden)
		return
	}

	stats, err := h.service.GetStats(id, 30)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, stats)
}

func (h *Handler) APICheckPath(w http.ResponseWriter, r *http.Request) {
	domainID := r.URL.Query().Get("domainId")
	path := r.URL.Query().Get("path")

	if domainID == "" || path == "" {
		h.jsonError(w, "domainId and path required", http.StatusBadRequest)
		return
	}

	available, _ := h.service.CheckPathAvailable(domainID, path)
	h.json(w, http.StatusOK, map[string]bool{"available": available})
}

// Helpers

func getBotErrors() []map[string]interface{} {
	return []map[string]interface{}{
		{"code": 400, "name": "Bad Request"},
		{"code": 401, "name": "Unauthorized"},
		{"code": 403, "name": "Forbidden"},
		{"code": 404, "name": "Not Found"},
		{"code": 405, "name": "Method Not Allowed"},
		{"code": 408, "name": "Request Timeout"},
		{"code": 410, "name": "Gone"},
		{"code": 429, "name": "Too Many Requests"},
		{"code": 500, "name": "Internal Server Error"},
		{"code": 502, "name": "Bad Gateway"},
		{"code": 503, "name": "Service Unavailable"},
	}
}
