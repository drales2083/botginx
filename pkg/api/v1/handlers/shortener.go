package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/botginx/botginx/modules/shortener/models"
	"github.com/botginx/botginx/modules/shortener/services"
	"github.com/botginx/botginx/pkg/apiauth"
	"github.com/botginx/botginx/pkg/protection"
	"github.com/go-chi/chi/v5"
)

type ShortenerHandler struct {
	service *services.ShortenerService
}

func NewShortenerHandler(service *services.ShortenerService) *ShortenerHandler {
	return &ShortenerHandler{service: service}
}

// ShortLinkResponse represents a short link in API responses
type ShortLinkResponse struct {
	ID           string   `json:"id"`
	DomainID     string   `json:"domainId"`
	DomainName   string   `json:"domainName,omitempty"`
	Subdomain    string   `json:"subdomain"`
	Path         string   `json:"path"`
	Destinations []string `json:"destinations"`
	RotationMode string   `json:"rotationMode"`
	BotError     int      `json:"botError"`
	QREnabled    bool     `json:"qrEnabled"`
	ClickCount   int      `json:"clickCount"`
	HumanCount   int      `json:"humanCount"`
	BotCount     int      `json:"botCount"`
	DeployStatus string   `json:"deployStatus"`
	DeployedURL  string   `json:"deployedUrl,omitempty"`
	CreatedAt    string   `json:"createdAt"`
	UpdatedAt    string   `json:"updatedAt"`
}

// CreateShortLinkRequest is the request body for creating a short link
type CreateShortLinkRequest struct {
	DomainID           string               `json:"domainId"`
	Subdomain          string               `json:"subdomain,omitempty"`
	Path               string               `json:"path,omitempty"`
	Destinations       []string             `json:"destinations"`
	RotationMode       string               `json:"rotationMode,omitempty"`
	BotError           int                  `json:"botError,omitempty"`
	QREnabled          bool                 `json:"qrEnabled"`
	ProtectionSettings *protection.Settings `json:"protectionSettings,omitempty"`
}

// UpdateShortLinkRequest is the request body for updating a short link
type UpdateShortLinkRequest struct {
	Destinations       []string             `json:"destinations,omitempty"`
	RotationMode       *string              `json:"rotationMode,omitempty"`
	BotError           *int                 `json:"botError,omitempty"`
	QREnabled          *bool                `json:"qrEnabled,omitempty"`
	ProtectionSettings *protection.Settings `json:"protectionSettings,omitempty"`
}

func toAPIShortLink(link *models.ShortLink) *ShortLinkResponse {
	deployedURL := ""
	if link.DeployedURL != nil {
		deployedURL = *link.DeployedURL
	}
	return &ShortLinkResponse{
		ID:           link.ID,
		DomainID:     link.DomainID,
		DomainName:   link.DomainName,
		Subdomain:    link.Subdomain,
		Path:         link.Path,
		Destinations: link.Destinations,
		RotationMode: link.RotationMode,
		BotError:     link.BotError,
		QREnabled:    link.QREnabled,
		ClickCount:   link.ClickCount,
		HumanCount:   link.HumanCount,
		BotCount:     link.BotCount,
		DeployStatus: link.DeployStatus,
		DeployedURL:  deployedURL,
		CreatedAt:    link.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:    link.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func (h *ShortenerHandler) List(w http.ResponseWriter, r *http.Request) {
	key := apiauth.GetAPIKey(r)
	if key == nil {
		apiauth.ErrorResp(w, "unauthorized", "API key required", http.StatusUnauthorized)
		return
	}

	if !key.HasScope("shortener:read") {
		apiauth.ErrorResp(w, "forbidden", "Missing scope: shortener:read", http.StatusForbidden)
		return
	}

	links, err := h.service.List(key.UserID)
	if err != nil {
		apiauth.ErrorResp(w, "fetch_failed", "Failed to fetch short links", http.StatusInternalServerError)
		return
	}

	result := make([]*ShortLinkResponse, 0, len(links))
	for i := range links {
		result = append(result, toAPIShortLink(&links[i]))
	}

	apiauth.Success(w, result)
}

func (h *ShortenerHandler) Get(w http.ResponseWriter, r *http.Request) {
	key := apiauth.GetAPIKey(r)
	if key == nil {
		apiauth.ErrorResp(w, "unauthorized", "API key required", http.StatusUnauthorized)
		return
	}

	if !key.HasScope("shortener:read") {
		apiauth.ErrorResp(w, "forbidden", "Missing scope: shortener:read", http.StatusForbidden)
		return
	}

	id := chi.URLParam(r, "id")
	link, err := h.service.Get(id)
	if err != nil {
		apiauth.ErrorResp(w, "not_found", "Short link not found", http.StatusNotFound)
		return
	}

	// Verify ownership
	if link.UserID != key.UserID {
		apiauth.ErrorResp(w, "not_found", "Short link not found", http.StatusNotFound)
		return
	}

	apiauth.Success(w, toAPIShortLink(link))
}

func (h *ShortenerHandler) Create(w http.ResponseWriter, r *http.Request) {
	key := apiauth.GetAPIKey(r)
	if key == nil {
		apiauth.ErrorResp(w, "unauthorized", "API key required", http.StatusUnauthorized)
		return
	}

	if !key.HasScope("shortener:write") {
		apiauth.ErrorResp(w, "forbidden", "Missing scope: shortener:write", http.StatusForbidden)
		return
	}

	var req CreateShortLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apiauth.ErrorResp(w, "invalid_request", "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if req.DomainID == "" {
		apiauth.ErrorResp(w, "validation_error", "domainId is required", http.StatusBadRequest)
		return
	}

	if len(req.Destinations) == 0 {
		apiauth.ErrorResp(w, "validation_error", "destinations is required", http.StatusBadRequest)
		return
	}

	// Generate subdomain if not provided
	subdomain := req.Subdomain
	if subdomain == "" {
		subdomain = h.service.GenerateSubdomain()
	}

	// Generate path if not provided
	path := req.Path
	if path == "" {
		path = h.service.GenerateRandomPath(6)
	}

	protSettings := protection.GetDefaultSettings()
	if req.ProtectionSettings != nil {
		protSettings = *req.ProtectionSettings
	}

	input := models.CreateShortLinkInput{
		DomainID:           req.DomainID,
		Subdomain:          subdomain,
		Path:               path,
		Destinations:       req.Destinations,
		RotationMode:       req.RotationMode,
		BotError:           req.BotError,
		QREnabled:          req.QREnabled,
		ProtectionSettings: protSettings,
	}

	link, err := h.service.Create(key.UserID, input)
	if err != nil {
		apiauth.ErrorResp(w, "create_failed", err.Error(), http.StatusInternalServerError)
		return
	}

	apiauth.Created(w, toAPIShortLink(link))
}

func (h *ShortenerHandler) Update(w http.ResponseWriter, r *http.Request) {
	key := apiauth.GetAPIKey(r)
	if key == nil {
		apiauth.ErrorResp(w, "unauthorized", "API key required", http.StatusUnauthorized)
		return
	}

	if !key.HasScope("shortener:write") {
		apiauth.ErrorResp(w, "forbidden", "Missing scope: shortener:write", http.StatusForbidden)
		return
	}

	id := chi.URLParam(r, "id")

	// Verify ownership
	existing, err := h.service.Get(id)
	if err != nil {
		apiauth.ErrorResp(w, "not_found", "Short link not found", http.StatusNotFound)
		return
	}
	if existing.UserID != key.UserID {
		apiauth.ErrorResp(w, "not_found", "Short link not found", http.StatusNotFound)
		return
	}

	var req UpdateShortLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apiauth.ErrorResp(w, "invalid_request", "Invalid JSON body", http.StatusBadRequest)
		return
	}

	input := models.UpdateShortLinkInput{
		Destinations:       req.Destinations,
		RotationMode:       req.RotationMode,
		BotError:           req.BotError,
		QREnabled:          req.QREnabled,
		ProtectionSettings: req.ProtectionSettings,
	}

	link, err := h.service.Update(id, input)
	if err != nil {
		apiauth.ErrorResp(w, "update_failed", err.Error(), http.StatusInternalServerError)
		return
	}

	apiauth.Success(w, toAPIShortLink(link))
}

func (h *ShortenerHandler) Delete(w http.ResponseWriter, r *http.Request) {
	key := apiauth.GetAPIKey(r)
	if key == nil {
		apiauth.ErrorResp(w, "unauthorized", "API key required", http.StatusUnauthorized)
		return
	}

	if !key.HasScope("shortener:write") {
		apiauth.ErrorResp(w, "forbidden", "Missing scope: shortener:write", http.StatusForbidden)
		return
	}

	id := chi.URLParam(r, "id")

	// Verify ownership
	existing, err := h.service.Get(id)
	if err != nil {
		apiauth.ErrorResp(w, "not_found", "Short link not found", http.StatusNotFound)
		return
	}
	if existing.UserID != key.UserID {
		apiauth.ErrorResp(w, "not_found", "Short link not found", http.StatusNotFound)
		return
	}

	if err := h.service.Delete(id); err != nil {
		apiauth.ErrorResp(w, "delete_failed", err.Error(), http.StatusInternalServerError)
		return
	}

	apiauth.NoContent(w)
}

func (h *ShortenerHandler) Stats(w http.ResponseWriter, r *http.Request) {
	key := apiauth.GetAPIKey(r)
	if key == nil {
		apiauth.ErrorResp(w, "unauthorized", "API key required", http.StatusUnauthorized)
		return
	}

	if !key.HasScope("shortener:read") {
		apiauth.ErrorResp(w, "forbidden", "Missing scope: shortener:read", http.StatusForbidden)
		return
	}

	id := chi.URLParam(r, "id")

	// Verify ownership
	existing, err := h.service.Get(id)
	if err != nil {
		apiauth.ErrorResp(w, "not_found", "Short link not found", http.StatusNotFound)
		return
	}
	if existing.UserID != key.UserID {
		apiauth.ErrorResp(w, "not_found", "Short link not found", http.StatusNotFound)
		return
	}

	days := 7
	if d := r.URL.Query().Get("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 && parsed <= 90 {
			days = parsed
		}
	}

	stats, err := h.service.GetStats(id, days)
	if err != nil {
		apiauth.ErrorResp(w, "fetch_failed", "Failed to fetch stats", http.StatusInternalServerError)
		return
	}

	apiauth.Success(w, map[string]interface{}{
		"totalClicks": stats.TotalClicks,
		"humanClicks": stats.HumanClicks,
		"botClicks":   stats.BotClicks,
		"byCountry":   stats.ByCountry,
		"byDevice":    stats.ByDevice,
		"byDay":       stats.ByDay,
	})
}
