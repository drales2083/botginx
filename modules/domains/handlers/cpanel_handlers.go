package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/botginx/botginx/modules/domains/models"
	"github.com/botginx/botginx/modules/domains/services"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/go-chi/chi/v5"
)

// CpanelHandler handles cPanel connection API endpoints
type CpanelHandler struct {
	cpanelService *services.CpanelService
}

// NewCpanelHandler creates a new cPanel handler
func NewCpanelHandler(cpanelService *services.CpanelService) *CpanelHandler {
	return &CpanelHandler{
		cpanelService: cpanelService,
	}
}

func (h *CpanelHandler) jsonError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   msg,
	})
}

func (h *CpanelHandler) jsonSuccess(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    data,
	})
}

// ListConnections returns all cPanel connections for the user
func (h *CpanelHandler) ListConnections(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	connections, err := h.cpanelService.List(userID)
	if err != nil {
		h.jsonError(w, "Failed to list connections", http.StatusInternalServerError)
		return
	}

	h.jsonSuccess(w, connections)
}

// CreateConnection creates a new cPanel connection
func (h *CpanelHandler) CreateConnection(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var input models.CreateCpanelConnectionInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if input.Name == "" || input.Host == "" || input.Username == "" || input.APIToken == "" {
		h.jsonError(w, "Name, host, username, and API token are required", http.StatusBadRequest)
		return
	}

	conn, err := h.cpanelService.Create(userID, input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.jsonSuccess(w, conn)
}

// GetConnection returns a single cPanel connection
func (h *CpanelHandler) GetConnection(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	connID := chi.URLParam(r, "id")

	conn, err := h.cpanelService.Get(userID, connID)
	if err != nil {
		h.jsonError(w, "Connection not found", http.StatusNotFound)
		return
	}

	// Don't return the decrypted token
	conn.APIToken = ""

	h.jsonSuccess(w, conn)
}

// UpdateConnection updates a cPanel connection
func (h *CpanelHandler) UpdateConnection(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	connID := chi.URLParam(r, "id")

	var input models.UpdateCpanelConnectionInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	conn, err := h.cpanelService.Update(userID, connID, input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Don't return the decrypted token
	conn.APIToken = ""

	h.jsonSuccess(w, conn)
}

// DeleteConnection deletes a cPanel connection
func (h *CpanelHandler) DeleteConnection(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	connID := chi.URLParam(r, "id")

	if err := h.cpanelService.Delete(userID, connID); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.jsonSuccess(w, map[string]string{"message": "Connection deleted"})
}

// TestConnection tests a cPanel connection
func (h *CpanelHandler) TestConnection(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	connID := chi.URLParam(r, "id")

	if err := h.cpanelService.TestConnection(userID, connID); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.jsonSuccess(w, map[string]string{"message": "Connection successful"})
}

// TestNewConnection tests a cPanel connection before saving
func (h *CpanelHandler) TestNewConnection(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Host     string `json:"host"`
		Username string `json:"username"`
		APIToken string `json:"apiToken"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if input.Host == "" || input.Username == "" || input.APIToken == "" {
		h.jsonError(w, "Host, username, and API token are required", http.StatusBadRequest)
		return
	}

	// Test connection (uses proxy if configured)
	domains, err := h.cpanelService.TestNewConnection(input.Host, input.Username, input.APIToken)
	if err != nil {
		h.jsonError(w, "Connection failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	h.jsonSuccess(w, map[string]interface{}{
		"message": "Connection successful",
		"domains": domains,
	})
}

// ListDomains returns all domains available on a cPanel connection
func (h *CpanelHandler) ListDomains(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	connID := chi.URLParam(r, "id")

	domains, err := h.cpanelService.ListDomains(userID, connID)
	if err != nil {
		// User-friendly error messages
		errMsg := err.Error()
		if errMsg == "sql: no rows in result set" {
			h.jsonError(w, "Connection not found or was deleted", http.StatusNotFound)
		} else if strings.Contains(errMsg, "connection refused") || strings.Contains(errMsg, "timeout") {
			h.jsonError(w, "Unable to connect to cPanel server", http.StatusBadGateway)
		} else if strings.Contains(errMsg, "unauthorized") || strings.Contains(errMsg, "forbidden") {
			h.jsonError(w, "Invalid cPanel credentials", http.StatusUnauthorized)
		} else {
			h.jsonError(w, "Failed to load domains from cPanel", http.StatusBadRequest)
		}
		return
	}

	h.jsonSuccess(w, domains)
}
