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

// CloudflareHandler handles Cloudflare integration endpoints
type CloudflareHandler struct {
	cloudflareService *services.CloudflareService
	templates         *module.TemplateEngine
}

// NewCloudflareHandler creates a new Cloudflare handler
func NewCloudflareHandler(cloudflareService *services.CloudflareService, templates *module.TemplateEngine) *CloudflareHandler {
	return &CloudflareHandler{
		cloudflareService: cloudflareService,
		templates:         templates,
	}
}

func (h *CloudflareHandler) jsonError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   msg,
	})
}

func (h *CloudflareHandler) jsonSuccess(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    data,
	})
}

// Page renders the Cloudflare connection page
func (h *CloudflareHandler) Page(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	conn, err := h.cloudflareService.GetConnection(userID)
	if err != nil {
		conn = &models.CloudflareConnection{IsConnected: false}
	}

	var zones []models.CloudflareZone
	if conn.IsConnected {
		zones, _ = h.cloudflareService.ListZones(r.Context(), userID)
	}

	module.RenderUserSection(w, r, h.templates, "domains:cloudflare.html", map[string]interface{}{
		"Title":      "Cloudflare",
		"Connection": conn,
		"Zones":      zones,
	})
}

// Connect handles API key connection
func (h *CloudflareHandler) Connect(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var input struct {
		Email  string `json:"email"`
		APIKey string `json:"apiKey"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if input.Email == "" || input.APIKey == "" {
		h.jsonError(w, "Email and API key are required", http.StatusBadRequest)
		return
	}

	if err := h.cloudflareService.Connect(r.Context(), userID, input.Email, input.APIKey); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.jsonSuccess(w, map[string]interface{}{
		"message": "Connected to Cloudflare",
	})
}

// Disconnect removes the API key
func (h *CloudflareHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	if err := h.cloudflareService.Disconnect(userID); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonSuccess(w, map[string]interface{}{
		"message": "Disconnected from Cloudflare",
	})
}

// ListZones returns all zones
func (h *CloudflareHandler) ListZones(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	zones, err := h.cloudflareService.ListZones(r.Context(), userID)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonSuccess(w, zones)
}

// GetDNSRecords returns DNS records for a zone
func (h *CloudflareHandler) GetDNSRecords(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	zoneID := chi.URLParam(r, "zoneID")

	records, err := h.cloudflareService.ListDNSRecords(r.Context(), userID, zoneID)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonSuccess(w, records)
}

// Import imports a domain from Cloudflare
func (h *CloudflareHandler) Import(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var input models.CloudflareImportInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if input.ZoneID == "" {
		h.jsonError(w, "Zone ID is required", http.StatusBadRequest)
		return
	}

	domain, err := h.cloudflareService.Import(r.Context(), userID, input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.jsonSuccess(w, domain)
}

// SetupDNS configures DNS records for a zone
func (h *CloudflareHandler) SetupDNS(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var input models.CloudflareDNSSetupInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if input.ZoneID == "" || input.Domain == "" {
		h.jsonError(w, "Zone ID and domain are required", http.StatusBadRequest)
		return
	}

	if err := h.cloudflareService.SetupDNS(r.Context(), userID, input.ZoneID, input.Domain); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonSuccess(w, map[string]interface{}{
		"message": "DNS configured successfully",
	})
}

// Reconfigure updates DNS for a single domain
func (h *CloudflareHandler) ReconfigureDomain(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	domainID := chi.URLParam(r, "id")

	if err := h.cloudflareService.ReconfigureDomain(r.Context(), userID, domainID); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonSuccess(w, map[string]interface{}{
		"message": "DNS reconfigured",
	})
}

// AdminReconfigureAll updates all Cloudflare domains (admin only)
func (h *CloudflareHandler) AdminReconfigureAll(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ServerIP string `json:"serverIp"`
	}
	json.NewDecoder(r.Body).Decode(&input)

	results, err := h.cloudflareService.ReconfigureAllDomains(r.Context(), input.ServerIP)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonSuccess(w, results)
}
