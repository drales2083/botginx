package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/botginx/botginx/modules/domains/services"
	"github.com/botginx/botginx/modules/redirectlinks/models"
	linkservices "github.com/botginx/botginx/modules/redirectlinks/services"
	"github.com/botginx/botginx/pkg/apiauth"
	"github.com/go-chi/chi/v5"
)

type LinksHandler struct {
	linksService   *linkservices.RedirectLinkService
	domainsService *services.DomainService
}

func NewLinksHandler(linksService *linkservices.RedirectLinkService, domainsService *services.DomainService) *LinksHandler {
	return &LinksHandler{
		linksService:   linksService,
		domainsService: domainsService,
	}
}

// LinkResponse represents a link in API responses
type LinkResponse struct {
	ID              string   `json:"id"`
	DomainID        string   `json:"domainId"`
	DomainName      string   `json:"domainName,omitempty"`
	Subdomain       string   `json:"subdomain"`
	Path            string   `json:"path"`
	Type            string   `json:"type"`
	DestinationURLs []string `json:"destinationUrls"`
	BotProtection   bool     `json:"botProtection"`
	PassParams      bool     `json:"passParams"`
	DeployStatus    string   `json:"deployStatus"`
	DeployedURL     string   `json:"deployedUrl,omitempty"`
	ViewCount       int      `json:"viewCount"`
	CreatedAt       string   `json:"createdAt"`
	UpdatedAt       string   `json:"updatedAt"`
}

// CreateLinkRequest is the request body for creating a link
type CreateLinkRequest struct {
	DomainID        string   `json:"domainId"`
	Subdomain       string   `json:"subdomain"`
	Path            string   `json:"path,omitempty"`
	Type            string   `json:"type,omitempty"`
	DestinationURLs []string `json:"destinationUrls"`
	HTMLContent     string   `json:"htmlContent,omitempty"`
	BotProtection   bool     `json:"botProtection"`
	PassParams      bool     `json:"passParams"`
}

// UpdateLinkRequest is the request body for updating a link
type UpdateLinkRequest struct {
	DestinationURLs []string `json:"destinationUrls,omitempty"`
	HTMLContent     string   `json:"htmlContent,omitempty"`
	BotProtection   *bool    `json:"botProtection,omitempty"`
	PassParams      *bool    `json:"passParams,omitempty"`
}

func toAPILink(link *models.RedirectLink) *LinkResponse {
	deployedURL := ""
	if link.DeployedURL != nil {
		deployedURL = *link.DeployedURL
	}
	return &LinkResponse{
		ID:              link.ID,
		DomainID:        link.DomainID,
		DomainName:      link.DomainName,
		Subdomain:       link.Subdomain,
		Path:            link.Path,
		Type:            string(link.Type),
		DestinationURLs: link.DestinationURLs,
		BotProtection:   link.BotProtection,
		PassParams:      link.PassParams,
		DeployStatus:    string(link.DeployStatus),
		DeployedURL:     deployedURL,
		ViewCount:       link.ViewCount,
		CreatedAt:       link.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:       link.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func (h *LinksHandler) List(w http.ResponseWriter, r *http.Request) {
	key := apiauth.GetAPIKey(r)
	if key == nil {
		apiauth.ErrorResp(w, "unauthorized", "API key required", http.StatusUnauthorized)
		return
	}

	if !key.HasScope("links:read") {
		apiauth.ErrorResp(w, "forbidden", "Missing scope: links:read", http.StatusForbidden)
		return
	}

	links, err := h.linksService.List(key.UserID)
	if err != nil {
		apiauth.ErrorResp(w, "fetch_failed", "Failed to fetch links", http.StatusInternalServerError)
		return
	}

	result := make([]*LinkResponse, 0, len(links))
	for i := range links {
		result = append(result, toAPILink(&links[i]))
	}

	apiauth.Success(w, result)
}

func (h *LinksHandler) Get(w http.ResponseWriter, r *http.Request) {
	key := apiauth.GetAPIKey(r)
	if key == nil {
		apiauth.ErrorResp(w, "unauthorized", "API key required", http.StatusUnauthorized)
		return
	}

	if !key.HasScope("links:read") {
		apiauth.ErrorResp(w, "forbidden", "Missing scope: links:read", http.StatusForbidden)
		return
	}

	id := chi.URLParam(r, "id")
	link, err := h.linksService.Get(id)
	if err != nil {
		apiauth.ErrorResp(w, "not_found", "Link not found", http.StatusNotFound)
		return
	}

	// Verify ownership
	if link.UserID != key.UserID {
		apiauth.ErrorResp(w, "not_found", "Link not found", http.StatusNotFound)
		return
	}

	apiauth.Success(w, toAPILink(link))
}

func (h *LinksHandler) Create(w http.ResponseWriter, r *http.Request) {
	key := apiauth.GetAPIKey(r)
	if key == nil {
		apiauth.ErrorResp(w, "unauthorized", "API key required", http.StatusUnauthorized)
		return
	}

	if !key.HasScope("links:write") {
		apiauth.ErrorResp(w, "forbidden", "Missing scope: links:write", http.StatusForbidden)
		return
	}

	var req CreateLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apiauth.ErrorResp(w, "invalid_request", "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if req.DomainID == "" {
		apiauth.ErrorResp(w, "validation_error", "domainId is required", http.StatusBadRequest)
		return
	}

	if len(req.DestinationURLs) == 0 && req.HTMLContent == "" {
		apiauth.ErrorResp(w, "validation_error", "destinationUrls or htmlContent is required", http.StatusBadRequest)
		return
	}

	// Verify domain ownership
	domain, err := h.domainsService.Get(req.DomainID)
	if err != nil || (domain.UserID != key.UserID && !domain.IsShared) {
		apiauth.ErrorResp(w, "validation_error", "Invalid domain", http.StatusBadRequest)
		return
	}

	input := models.CreateRedirectLinkInput{
		DomainID:        req.DomainID,
		Subdomain:       req.Subdomain,
		Path:            req.Path,
		Type:            req.Type,
		DestinationURLs: req.DestinationURLs,
		HTMLContent:     req.HTMLContent,
		BotProtection:   req.BotProtection,
		PassParams:      req.PassParams,
	}

	link, err := h.linksService.Create(key.UserID, input)
	if err != nil {
		apiauth.ErrorResp(w, "create_failed", err.Error(), http.StatusInternalServerError)
		return
	}

	apiauth.Created(w, toAPILink(link))
}

func (h *LinksHandler) Update(w http.ResponseWriter, r *http.Request) {
	key := apiauth.GetAPIKey(r)
	if key == nil {
		apiauth.ErrorResp(w, "unauthorized", "API key required", http.StatusUnauthorized)
		return
	}

	if !key.HasScope("links:write") {
		apiauth.ErrorResp(w, "forbidden", "Missing scope: links:write", http.StatusForbidden)
		return
	}

	id := chi.URLParam(r, "id")

	// Verify ownership
	existing, err := h.linksService.Get(id)
	if err != nil {
		apiauth.ErrorResp(w, "not_found", "Link not found", http.StatusNotFound)
		return
	}
	if existing.UserID != key.UserID {
		apiauth.ErrorResp(w, "not_found", "Link not found", http.StatusNotFound)
		return
	}

	var req UpdateLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apiauth.ErrorResp(w, "invalid_request", "Invalid JSON body", http.StatusBadRequest)
		return
	}

	input := models.UpdateRedirectLinkInput{
		DestinationURLs:  req.DestinationURLs,
		HTMLContent:      req.HTMLContent,
		BotProtection:    req.BotProtection,
		PassParams:       req.PassParams,
	}

	link, err := h.linksService.Update(id, input)
	if err != nil {
		apiauth.ErrorResp(w, "update_failed", err.Error(), http.StatusInternalServerError)
		return
	}

	apiauth.Success(w, toAPILink(link))
}

func (h *LinksHandler) Delete(w http.ResponseWriter, r *http.Request) {
	key := apiauth.GetAPIKey(r)
	if key == nil {
		apiauth.ErrorResp(w, "unauthorized", "API key required", http.StatusUnauthorized)
		return
	}

	if !key.HasScope("links:delete") {
		apiauth.ErrorResp(w, "forbidden", "Missing scope: links:delete", http.StatusForbidden)
		return
	}

	id := chi.URLParam(r, "id")

	// Verify ownership
	existing, err := h.linksService.Get(id)
	if err != nil {
		apiauth.ErrorResp(w, "not_found", "Link not found", http.StatusNotFound)
		return
	}
	if existing.UserID != key.UserID {
		apiauth.ErrorResp(w, "not_found", "Link not found", http.StatusNotFound)
		return
	}

	if err := h.linksService.Delete(id); err != nil {
		apiauth.ErrorResp(w, "delete_failed", err.Error(), http.StatusInternalServerError)
		return
	}

	apiauth.NoContent(w)
}
