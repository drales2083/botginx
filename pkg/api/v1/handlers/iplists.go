package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/botginx/botginx/modules/auth/services"
	"github.com/botginx/botginx/pkg/apiauth"
	"github.com/go-chi/chi/v5"
)

type IPListsHandler struct {
	whitelistService *services.GlobalWhitelistService
}

func NewIPListsHandler(whitelistService *services.GlobalWhitelistService) *IPListsHandler {
	return &IPListsHandler{
		whitelistService: whitelistService,
	}
}

// WhitelistEntryResponse represents a whitelist entry in API responses
type WhitelistEntryResponse struct {
	ID        string `json:"id"`
	IP        string `json:"ip"`
	CreatedAt string `json:"createdAt"`
}

// AddIPRequest is the request body for adding an IP
type AddIPRequest struct {
	IP string `json:"ip"`
}

func (h *IPListsHandler) ListWhitelist(w http.ResponseWriter, r *http.Request) {
	key := apiauth.GetAPIKey(r)
	if key == nil {
		apiauth.ErrorResp(w, "unauthorized", "API key required", http.StatusUnauthorized)
		return
	}

	if !key.HasScope("iplists:read") {
		apiauth.ErrorResp(w, "forbidden", "Missing scope: iplists:read", http.StatusForbidden)
		return
	}

	entries, err := h.whitelistService.List(key.UserID)
	if err != nil {
		apiauth.ErrorResp(w, "fetch_failed", "Failed to fetch whitelist", http.StatusInternalServerError)
		return
	}

	result := make([]*WhitelistEntryResponse, 0, len(entries))
	for _, e := range entries {
		result = append(result, &WhitelistEntryResponse{
			ID:        e.ID,
			IP:        e.IP,
			CreatedAt: e.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	apiauth.Success(w, result)
}

func (h *IPListsHandler) AddWhitelist(w http.ResponseWriter, r *http.Request) {
	key := apiauth.GetAPIKey(r)
	if key == nil {
		apiauth.ErrorResp(w, "unauthorized", "API key required", http.StatusUnauthorized)
		return
	}

	if !key.HasScope("iplists:write") {
		apiauth.ErrorResp(w, "forbidden", "Missing scope: iplists:write", http.StatusForbidden)
		return
	}

	var req AddIPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apiauth.ErrorResp(w, "invalid_request", "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if req.IP == "" {
		apiauth.ErrorResp(w, "validation_error", "ip is required", http.StatusBadRequest)
		return
	}

	entry, err := h.whitelistService.Add(key.UserID, req.IP)
	if err != nil {
		apiauth.ErrorResp(w, "add_failed", err.Error(), http.StatusBadRequest)
		return
	}

	apiauth.Created(w, &WhitelistEntryResponse{
		ID:        entry.ID,
		IP:        entry.IP,
		CreatedAt: entry.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

func (h *IPListsHandler) RemoveWhitelist(w http.ResponseWriter, r *http.Request) {
	key := apiauth.GetAPIKey(r)
	if key == nil {
		apiauth.ErrorResp(w, "unauthorized", "API key required", http.StatusUnauthorized)
		return
	}

	if !key.HasScope("iplists:write") {
		apiauth.ErrorResp(w, "forbidden", "Missing scope: iplists:write", http.StatusForbidden)
		return
	}

	id := chi.URLParam(r, "id")

	if err := h.whitelistService.Remove(key.UserID, id); err != nil {
		apiauth.ErrorResp(w, "not_found", "Whitelist entry not found", http.StatusNotFound)
		return
	}

	apiauth.NoContent(w)
}
