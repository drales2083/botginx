package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/botginx/botginx/modules/domains/models"
	"github.com/botginx/botginx/modules/domains/services"
	"github.com/botginx/botginx/pkg/apiauth"
	"github.com/go-chi/chi/v5"
)

type DomainsHandler struct {
	service *services.DomainService
}

func NewDomainsHandler(service *services.DomainService) *DomainsHandler {
	return &DomainsHandler{service: service}
}

// DomainResponse represents a domain in API responses
type DomainResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DNSVerified bool   `json:"dnsVerified"`
	SSLEnabled  bool   `json:"sslEnabled"`
	IsWildcard  bool   `json:"isWildcard"`
	SetupType   string `json:"setupType"`
	SetupStep   string `json:"setupStep"`
	VerifyToken string `json:"verifyToken,omitempty"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

// CreateDomainRequest is the request body for creating a domain
type CreateDomainRequest struct {
	Name     string `json:"name"`
	ServerID string `json:"serverId,omitempty"`
}

// UpdateDomainRequest is the request body for updating a domain
type UpdateDomainRequest struct {
	ServerID *string `json:"serverId,omitempty"`
}

func toAPIDomain(domain *models.Domain) *DomainResponse {
	return &DomainResponse{
		ID:          domain.ID,
		Name:        domain.Name,
		DNSVerified: domain.DNSVerified,
		SSLEnabled:  domain.SSLEnabled,
		IsWildcard:  domain.IsWildcard,
		SetupType:   string(domain.SetupType),
		SetupStep:   string(domain.SetupStep),
		VerifyToken: domain.VerifyToken,
		CreatedAt:   domain.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   domain.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func (h *DomainsHandler) List(w http.ResponseWriter, r *http.Request) {
	key := apiauth.GetAPIKey(r)
	if key == nil {
		apiauth.ErrorResp(w, "unauthorized", "API key required", http.StatusUnauthorized)
		return
	}

	if !key.HasScope("domains:read") {
		apiauth.ErrorResp(w, "forbidden", "Missing scope: domains:read", http.StatusForbidden)
		return
	}

	// Get user's own domains plus available shared domains
	domains, err := h.service.ListAvailable(key.UserID)
	if err != nil {
		apiauth.ErrorResp(w, "fetch_failed", "Failed to fetch domains", http.StatusInternalServerError)
		return
	}

	result := make([]*DomainResponse, 0, len(domains))
	for i := range domains {
		result = append(result, toAPIDomain(&domains[i]))
	}

	apiauth.Success(w, result)
}

func (h *DomainsHandler) Get(w http.ResponseWriter, r *http.Request) {
	key := apiauth.GetAPIKey(r)
	if key == nil {
		apiauth.ErrorResp(w, "unauthorized", "API key required", http.StatusUnauthorized)
		return
	}

	if !key.HasScope("domains:read") {
		apiauth.ErrorResp(w, "forbidden", "Missing scope: domains:read", http.StatusForbidden)
		return
	}

	id := chi.URLParam(r, "id")
	domain, err := h.service.Get(id)
	if err != nil {
		apiauth.ErrorResp(w, "not_found", "Domain not found", http.StatusNotFound)
		return
	}

	// Allow access to own domains or shared domains
	if domain.UserID != key.UserID && !domain.IsShared {
		apiauth.ErrorResp(w, "not_found", "Domain not found", http.StatusNotFound)
		return
	}

	apiauth.Success(w, toAPIDomain(domain))
}

func (h *DomainsHandler) Create(w http.ResponseWriter, r *http.Request) {
	key := apiauth.GetAPIKey(r)
	if key == nil {
		apiauth.ErrorResp(w, "unauthorized", "API key required", http.StatusUnauthorized)
		return
	}

	if !key.HasScope("domains:manage") {
		apiauth.ErrorResp(w, "forbidden", "Missing scope: domains:manage", http.StatusForbidden)
		return
	}

	var req CreateDomainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apiauth.ErrorResp(w, "invalid_request", "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		apiauth.ErrorResp(w, "validation_error", "name is required", http.StatusBadRequest)
		return
	}

	input := models.CreateDomainInput{
		Name:     req.Name,
		ServerID: req.ServerID,
	}

	domain, err := h.service.Create(key.UserID, input)
	if err != nil {
		if err == services.ErrDomainExists {
			apiauth.ErrorResp(w, "domain_exists", "Domain already registered", http.StatusConflict)
			return
		}
		apiauth.ErrorResp(w, "create_failed", err.Error(), http.StatusInternalServerError)
		return
	}

	apiauth.Created(w, toAPIDomain(domain))
}

func (h *DomainsHandler) Update(w http.ResponseWriter, r *http.Request) {
	key := apiauth.GetAPIKey(r)
	if key == nil {
		apiauth.ErrorResp(w, "unauthorized", "API key required", http.StatusUnauthorized)
		return
	}

	if !key.HasScope("domains:manage") {
		apiauth.ErrorResp(w, "forbidden", "Missing scope: domains:manage", http.StatusForbidden)
		return
	}

	id := chi.URLParam(r, "id")

	// Verify ownership
	existing, err := h.service.Get(id)
	if err != nil {
		apiauth.ErrorResp(w, "not_found", "Domain not found", http.StatusNotFound)
		return
	}
	if existing.UserID != key.UserID {
		apiauth.ErrorResp(w, "not_found", "Domain not found", http.StatusNotFound)
		return
	}

	var req UpdateDomainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apiauth.ErrorResp(w, "invalid_request", "Invalid JSON body", http.StatusBadRequest)
		return
	}

	input := models.UpdateDomainInput{
		ServerID: req.ServerID,
	}

	domain, err := h.service.Update(id, input)
	if err != nil {
		apiauth.ErrorResp(w, "update_failed", err.Error(), http.StatusInternalServerError)
		return
	}

	apiauth.Success(w, toAPIDomain(domain))
}

func (h *DomainsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	key := apiauth.GetAPIKey(r)
	if key == nil {
		apiauth.ErrorResp(w, "unauthorized", "API key required", http.StatusUnauthorized)
		return
	}

	if !key.HasScope("domains:manage") {
		apiauth.ErrorResp(w, "forbidden", "Missing scope: domains:manage", http.StatusForbidden)
		return
	}

	id := chi.URLParam(r, "id")

	// Verify ownership
	existing, err := h.service.Get(id)
	if err != nil {
		apiauth.ErrorResp(w, "not_found", "Domain not found", http.StatusNotFound)
		return
	}
	if existing.UserID != key.UserID {
		apiauth.ErrorResp(w, "not_found", "Domain not found", http.StatusNotFound)
		return
	}

	// Check if domain has links
	linkCount, _ := h.service.CountRedirectLinks(id)
	if linkCount > 0 {
		apiauth.ErrorResp(w, "has_links", "Cannot delete domain with active links", http.StatusConflict)
		return
	}

	if err := h.service.Delete(id); err != nil {
		apiauth.ErrorResp(w, "delete_failed", err.Error(), http.StatusInternalServerError)
		return
	}

	apiauth.NoContent(w)
}

func (h *DomainsHandler) Verify(w http.ResponseWriter, r *http.Request) {
	key := apiauth.GetAPIKey(r)
	if key == nil {
		apiauth.ErrorResp(w, "unauthorized", "API key required", http.StatusUnauthorized)
		return
	}

	if !key.HasScope("domains:manage") {
		apiauth.ErrorResp(w, "forbidden", "Missing scope: domains:manage", http.StatusForbidden)
		return
	}

	id := chi.URLParam(r, "id")

	// Verify ownership
	domain, err := h.service.Get(id)
	if err != nil {
		apiauth.ErrorResp(w, "not_found", "Domain not found", http.StatusNotFound)
		return
	}
	if domain.UserID != key.UserID {
		apiauth.ErrorResp(w, "not_found", "Domain not found", http.StatusNotFound)
		return
	}

	// Return current verification status with instructions
	deployIP := h.service.GetDeployIP()

	apiauth.Success(w, map[string]interface{}{
		"id":          domain.ID,
		"name":        domain.Name,
		"verified":    domain.DNSVerified,
		"verifyToken": domain.VerifyToken,
		"instructions": map[string]interface{}{
			"type":   "TXT",
			"record": "_guardbot." + domain.Name,
			"value":  domain.VerifyToken,
			"aRecord": map[string]string{
				"type":  "A",
				"value": deployIP,
			},
		},
	})
}
