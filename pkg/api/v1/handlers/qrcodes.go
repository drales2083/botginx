package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/botginx/botginx/modules/qrcodes/models"
	"github.com/botginx/botginx/modules/qrcodes/services"
	"github.com/botginx/botginx/pkg/apiauth"
	"github.com/go-chi/chi/v5"
)

type QRCodesHandler struct {
	service *services.QRCodeService
}

func NewQRCodesHandler(service *services.QRCodeService) *QRCodesHandler {
	return &QRCodesHandler{service: service}
}

// QRCodeResponse represents a QR code in API responses
type QRCodeResponse struct {
	ID             string  `json:"id"`
	Title          string  `json:"title"`
	URL            string  `json:"url"`
	RedirectLinkID *string `json:"redirectLinkId,omitempty"`
	FGColor        string  `json:"fgColor"`
	BGColor        string  `json:"bgColor"`
	CreatedAt      string  `json:"createdAt"`
	UpdatedAt      string  `json:"updatedAt"`
}

// CreateQRCodeRequest is the request body for creating a QR code
type CreateQRCodeRequest struct {
	Title          string `json:"title"`
	URL            string `json:"url"`
	RedirectLinkID string `json:"redirectLinkId,omitempty"`
	FGColor        string `json:"fgColor,omitempty"`
	BGColor        string `json:"bgColor,omitempty"`
}

func toAPIQRCode(qr *models.QRCode) *QRCodeResponse {
	var redirectLinkID *string
	if qr.RedirectLinkID.Valid {
		redirectLinkID = &qr.RedirectLinkID.String
	}
	return &QRCodeResponse{
		ID:             qr.ID,
		Title:          qr.Title,
		URL:            qr.URL,
		RedirectLinkID: redirectLinkID,
		FGColor:        qr.FGColor,
		BGColor:        qr.BGColor,
		CreatedAt:      qr.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:      qr.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func (h *QRCodesHandler) List(w http.ResponseWriter, r *http.Request) {
	key := apiauth.GetAPIKey(r)
	if key == nil {
		apiauth.ErrorResp(w, "unauthorized", "API key required", http.StatusUnauthorized)
		return
	}

	if !key.HasScope("qrcodes:read") {
		apiauth.ErrorResp(w, "forbidden", "Missing scope: qrcodes:read", http.StatusForbidden)
		return
	}

	page := 1
	limit := 25
	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	qrcodes, total, err := h.service.ListPaginated(key.UserID, page, limit)
	if err != nil {
		apiauth.ErrorResp(w, "fetch_failed", "Failed to fetch QR codes", http.StatusInternalServerError)
		return
	}

	result := make([]*QRCodeResponse, 0, len(qrcodes))
	for i := range qrcodes {
		result = append(result, toAPIQRCode(&qrcodes[i]))
	}

	apiauth.Paginated(w, result, page, limit, total)
}

func (h *QRCodesHandler) Get(w http.ResponseWriter, r *http.Request) {
	key := apiauth.GetAPIKey(r)
	if key == nil {
		apiauth.ErrorResp(w, "unauthorized", "API key required", http.StatusUnauthorized)
		return
	}

	if !key.HasScope("qrcodes:read") {
		apiauth.ErrorResp(w, "forbidden", "Missing scope: qrcodes:read", http.StatusForbidden)
		return
	}

	id := chi.URLParam(r, "id")
	qr, err := h.service.GetByUser(id, key.UserID)
	if err != nil {
		apiauth.ErrorResp(w, "not_found", "QR code not found", http.StatusNotFound)
		return
	}

	apiauth.Success(w, toAPIQRCode(qr))
}

func (h *QRCodesHandler) Create(w http.ResponseWriter, r *http.Request) {
	key := apiauth.GetAPIKey(r)
	if key == nil {
		apiauth.ErrorResp(w, "unauthorized", "API key required", http.StatusUnauthorized)
		return
	}

	if !key.HasScope("qrcodes:write") {
		apiauth.ErrorResp(w, "forbidden", "Missing scope: qrcodes:write", http.StatusForbidden)
		return
	}

	var req CreateQRCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apiauth.ErrorResp(w, "invalid_request", "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if req.Title == "" {
		apiauth.ErrorResp(w, "validation_error", "title is required", http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		apiauth.ErrorResp(w, "validation_error", "url is required", http.StatusBadRequest)
		return
	}

	input := models.CreateQRCodeRequest{
		Title:          req.Title,
		URL:            req.URL,
		RedirectLinkID: req.RedirectLinkID,
		FGColor:        req.FGColor,
		BGColor:        req.BGColor,
	}

	qr, err := h.service.Create(key.UserID, input)
	if err != nil {
		apiauth.ErrorResp(w, "create_failed", err.Error(), http.StatusInternalServerError)
		return
	}

	apiauth.Created(w, toAPIQRCode(qr))
}

func (h *QRCodesHandler) Download(w http.ResponseWriter, r *http.Request) {
	key := apiauth.GetAPIKey(r)
	if key == nil {
		apiauth.ErrorResp(w, "unauthorized", "API key required", http.StatusUnauthorized)
		return
	}

	if !key.HasScope("qrcodes:read") {
		apiauth.ErrorResp(w, "forbidden", "Missing scope: qrcodes:read", http.StatusForbidden)
		return
	}

	id := chi.URLParam(r, "id")
	qr, err := h.service.GetByUser(id, key.UserID)
	if err != nil {
		apiauth.ErrorResp(w, "not_found", "QR code not found", http.StatusNotFound)
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "png"
	}

	size := 512
	if s := r.URL.Query().Get("size"); s != "" {
		if parsed, err := strconv.Atoi(s); err == nil && parsed >= 64 && parsed <= 2048 {
			size = parsed
		}
	}

	switch format {
	case "svg":
		svg, err := services.GenerateSVG(qr.URL, size, qr.FGColor, qr.BGColor)
		if err != nil {
			apiauth.ErrorResp(w, "generate_failed", "Failed to generate QR code", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Content-Disposition", "attachment; filename=qr-"+id+".svg")
		w.Write([]byte(svg))
	default:
		png, err := services.GeneratePNG(qr.URL, size, qr.FGColor, qr.BGColor)
		if err != nil {
			apiauth.ErrorResp(w, "generate_failed", "Failed to generate QR code", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Disposition", "attachment; filename=qr-"+id+".png")
		w.Write(png)
	}
}

func (h *QRCodesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	key := apiauth.GetAPIKey(r)
	if key == nil {
		apiauth.ErrorResp(w, "unauthorized", "API key required", http.StatusUnauthorized)
		return
	}

	if !key.HasScope("qrcodes:write") {
		apiauth.ErrorResp(w, "forbidden", "Missing scope: qrcodes:write", http.StatusForbidden)
		return
	}

	id := chi.URLParam(r, "id")

	// Verify ownership
	_, err := h.service.GetByUser(id, key.UserID)
	if err != nil {
		apiauth.ErrorResp(w, "not_found", "QR code not found", http.StatusNotFound)
		return
	}

	if err := h.service.Delete(id, key.UserID); err != nil {
		apiauth.ErrorResp(w, "delete_failed", err.Error(), http.StatusInternalServerError)
		return
	}

	apiauth.NoContent(w)
}
