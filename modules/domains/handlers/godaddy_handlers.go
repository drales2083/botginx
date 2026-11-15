package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/botginx/botginx/modules/domains/models"
	"github.com/botginx/botginx/modules/domains/services"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

// GoDaddyHandler handles GoDaddy integration endpoints
type GoDaddyHandler struct {
	godaddyService *services.GoDaddyService
	templates      *module.TemplateEngine
}

// NewGoDaddyHandler creates a new GoDaddy handler
func NewGoDaddyHandler(godaddyService *services.GoDaddyService, templates *module.TemplateEngine) *GoDaddyHandler {
	return &GoDaddyHandler{
		godaddyService: godaddyService,
		templates:      templates,
	}
}

func (h *GoDaddyHandler) jsonError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   msg,
	})
}

func (h *GoDaddyHandler) jsonSuccess(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    data,
	})
}

// Page renders the GoDaddy connection page
func (h *GoDaddyHandler) Page(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	conn, err := h.godaddyService.GetConnection(userID)
	if err != nil {
		conn = &models.GoDaddyConnection{IsConnected: false}
	}

	var domains []models.GoDaddyDomain
	if conn.IsConnected {
		domains, _ = h.godaddyService.ListDomains(r.Context(), userID)
	}

	module.RenderUserSection(w, r, h.templates, "domains:godaddy.html", map[string]interface{}{
		"Title":      "GoDaddy",
		"Connection": conn,
		"Domains":    domains,
	})
}

// Connect handles API key connection
func (h *GoDaddyHandler) Connect(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var input struct {
		APIKey    string `json:"apiKey"`
		APISecret string `json:"apiSecret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if input.APIKey == "" || input.APISecret == "" {
		h.jsonError(w, "API Key and Secret are required", http.StatusBadRequest)
		return
	}

	if err := h.godaddyService.Connect(r.Context(), userID, input.APIKey, input.APISecret); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.jsonSuccess(w, map[string]interface{}{
		"message": "Connected to GoDaddy",
	})
}

// Disconnect removes the API credentials
func (h *GoDaddyHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	if err := h.godaddyService.Disconnect(userID); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonSuccess(w, map[string]interface{}{
		"message": "Disconnected from GoDaddy",
	})
}

// ListDomains returns all domains
func (h *GoDaddyHandler) ListDomains(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	domains, err := h.godaddyService.ListDomains(r.Context(), userID)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonSuccess(w, domains)
}

// Import imports a domain from GoDaddy
func (h *GoDaddyHandler) Import(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var input models.GoDaddyImportInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if input.Domain == "" {
		h.jsonError(w, "Domain is required", http.StatusBadRequest)
		return
	}

	domain, err := h.godaddyService.Import(r.Context(), userID, input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.jsonSuccess(w, domain)
}

// ReconfigureDomain updates DNS for a single domain
func (h *GoDaddyHandler) ReconfigureDomain(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	domainID := chi.URLParam(r, "id")

	if err := h.godaddyService.ReconfigureDomain(r.Context(), userID, domainID); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonSuccess(w, map[string]interface{}{
		"message": "DNS reconfigured",
	})
}

// AdminReconfigureAll updates all GoDaddy domains (admin only)
func (h *GoDaddyHandler) AdminReconfigureAll(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ServerIP string `json:"serverIp"`
	}
	json.NewDecoder(r.Body).Decode(&input)

	results, err := h.godaddyService.ReconfigureAllDomains(r.Context(), input.ServerIP)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonSuccess(w, results)
}
