package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/botginx/botginx/modules/domains/models"
	"github.com/botginx/botginx/modules/domains/services"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

// NameSiloHandler handles NameSilo integration endpoints
type NameSiloHandler struct {
	namesiloService *services.NameSiloService
	templates       *module.TemplateEngine
}

// NewNameSiloHandler creates a new NameSilo handler
func NewNameSiloHandler(namesiloService *services.NameSiloService, templates *module.TemplateEngine) *NameSiloHandler {
	return &NameSiloHandler{
		namesiloService: namesiloService,
		templates:       templates,
	}
}

func (h *NameSiloHandler) jsonError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   msg,
	})
}

func (h *NameSiloHandler) jsonSuccess(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    data,
	})
}

// Page renders the NameSilo connection page
func (h *NameSiloHandler) Page(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	conn, err := h.namesiloService.GetConnection(userID)
	if err != nil {
		conn = &models.NameSiloConnection{IsConnected: false}
	}

	var domains []models.NameSiloDomain
	if conn.IsConnected {
		domains, _ = h.namesiloService.ListDomains(r.Context(), userID)
	}

	module.RenderUserSection(w, r, h.templates, "domains:namesilo.html", map[string]interface{}{
		"Title":      "NameSilo",
		"Connection": conn,
		"Domains":    domains,
	})
}

// Connect handles API key connection
func (h *NameSiloHandler) Connect(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var input struct {
		APIKey string `json:"apiKey"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if input.APIKey == "" {
		h.jsonError(w, "API key is required", http.StatusBadRequest)
		return
	}

	if err := h.namesiloService.Connect(r.Context(), userID, input.APIKey); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.jsonSuccess(w, map[string]interface{}{
		"message": "Connected to NameSilo",
	})
}

// Disconnect removes the API key
func (h *NameSiloHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	if err := h.namesiloService.Disconnect(userID); err != nil {
		h.jsonError(w, "Failed to disconnect", http.StatusInternalServerError)
		return
	}

	h.jsonSuccess(w, map[string]interface{}{
		"message": "Disconnected from NameSilo",
	})
}

// ListDomains returns domains from NameSilo account
func (h *NameSiloHandler) ListDomains(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	domains, err := h.namesiloService.ListDomains(r.Context(), userID)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonSuccess(w, domains)
}

// Import imports a domain from NameSilo
func (h *NameSiloHandler) Import(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var input models.NameSiloImportInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if input.Domain == "" {
		h.jsonError(w, "Domain is required", http.StatusBadRequest)
		return
	}

	domainID, err := h.namesiloService.ImportDomain(r.Context(), userID, input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.jsonSuccess(w, map[string]interface{}{
		"message":  "Domain imported successfully",
		"domainId": domainID,
	})
}

// SetupDNS configures DNS for a domain
func (h *NameSiloHandler) SetupDNS(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	domain := chi.URLParam(r, "domain")

	if domain == "" {
		h.jsonError(w, "Domain is required", http.StatusBadRequest)
		return
	}

	if err := h.namesiloService.SetupDNS(r.Context(), userID, domain); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.jsonSuccess(w, map[string]interface{}{
		"message": "DNS configured successfully",
	})
}

// GetDNSRecords returns current DNS records for a domain
func (h *NameSiloHandler) GetDNSRecords(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	domain := chi.URLParam(r, "domain")

	if domain == "" {
		h.jsonError(w, "Domain is required", http.StatusBadRequest)
		return
	}

	records, err := h.namesiloService.GetDNSRecords(r.Context(), userID, domain)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonSuccess(w, records)
}

// Reconfigure updates DNS for all user's NameSilo domains with current server IP
func (h *NameSiloHandler) Reconfigure(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	results, err := h.namesiloService.ReconfigureUserDomains(r.Context(), userID)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Count successes and failures
	successCount := 0
	failCount := 0
	for _, res := range results {
		if res.Success {
			successCount++
		} else {
			failCount++
		}
	}

	h.jsonSuccess(w, map[string]interface{}{
		"results":      results,
		"total":        len(results),
		"successCount": successCount,
		"failCount":    failCount,
	})
}

// ReconfigureDomain updates DNS for a single domain
func (h *NameSiloHandler) ReconfigureDomain(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	domainID := chi.URLParam(r, "id")

	if domainID == "" {
		h.jsonError(w, "Domain ID is required", http.StatusBadRequest)
		return
	}

	result, err := h.namesiloService.ReconfigureDomain(r.Context(), userID, domainID)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonSuccess(w, result)
}

// AdminReconfigureAll updates DNS for ALL NameSilo domains across all users (admin only)
func (h *NameSiloHandler) AdminReconfigureAll(w http.ResponseWriter, r *http.Request) {
	// Get the new IP from request body, or auto-detect
	var input struct {
		IP string `json:"ip"`
	}
	json.NewDecoder(r.Body).Decode(&input)

	var serverIP string
	if input.IP != "" {
		serverIP = input.IP
	} else {
		// Auto-detect server IP
		resp, err := http.Get("https://api.ipify.org")
		if err != nil {
			h.jsonError(w, "Failed to detect server IP: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		serverIP = strings.TrimSpace(string(body))
	}

	if serverIP == "" {
		h.jsonError(w, "Could not determine server IP", http.StatusBadRequest)
		return
	}

	results, err := h.namesiloService.ReconfigureAllDomains(r.Context(), serverIP)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	successCount := 0
	failCount := 0
	for _, res := range results {
		if res.Success {
			successCount++
		} else {
			failCount++
		}
	}

	h.jsonSuccess(w, map[string]interface{}{
		"serverIP":     serverIP,
		"results":      results,
		"total":        len(results),
		"successCount": successCount,
		"failCount":    failCount,
	})
}

// GenerateSSL triggers wildcard SSL generation for a NameSilo domain
func (h *NameSiloHandler) GenerateSSL(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	domain := chi.URLParam(r, "domain")

	if domain == "" {
		h.jsonError(w, "Domain is required", http.StatusBadRequest)
		return
	}

	// Start SSL generation in background
	// Uses slow method (300s DNS propagation wait) as fallback
	go func() {
		bgCtx := context.Background()
		if err := h.namesiloService.GenerateWildcardSSLSlow(bgCtx, userID, domain); err != nil {
			// Log error - we can't return it since this is async
			println("[NameSilo] SSL generation failed for", domain, ":", err.Error())
		}
	}()

	h.jsonSuccess(w, map[string]interface{}{
		"message": "Wildcard SSL generation started. This may take up to 10 minutes due to DNS propagation.",
		"domain":  domain,
	})
}
