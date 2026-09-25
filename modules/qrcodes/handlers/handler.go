package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/botginx/botginx/modules/qrcodes/models"
	"github.com/botginx/botginx/modules/qrcodes/services"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

// RedirectLink represents a basic link for the dropdown
type RedirectLink struct {
	ID        string
	Subdomain string
	Domain    string
	Path      string
}

// RedirectLinkLister returns links for a user
type RedirectLinkLister func(userID string) ([]RedirectLink, error)

type Handler struct {
	service   *services.QRCodeService
	templates *module.TemplateEngine
	listLinks RedirectLinkLister
}

func NewHandler(service *services.QRCodeService, templates *module.TemplateEngine) *Handler {
	return &Handler{
		service:   service,
		templates: templates,
	}
}

func (h *Handler) SetLinkLister(fn RedirectLinkLister) {
	h.listLinks = fn
}

// Page handlers

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if pInt, err := strconv.Atoi(p); err == nil && pInt > 0 {
			page = pInt
		}
	}

	limit := 20
	codes, total, _ := h.service.ListPaginated(userID, page, limit)

	totalPages := (total + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	module.RenderUserSection(w, r, h.templates, "qrcodes:list.html", map[string]interface{}{
		"Title":      "QR Codes",
		"QRCodes":    codes,
		"Page":       page,
		"TotalPages": totalPages,
		"Total":      total,
		"Limit":      limit,
	})
}

func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var links []RedirectLink
	if h.listLinks != nil {
		links, _ = h.listLinks(userID)
	}

	module.RenderUserSection(w, r, h.templates, "qrcodes:new.html", map[string]interface{}{
		"Title":         "Create QR Code",
		"RedirectLinks": links,
	})
}

func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	id := chi.URLParam(r, "id")

	code, err := h.service.GetByUser(id, userID)
	if err != nil {
		http.Error(w, "QR code not found", http.StatusNotFound)
		return
	}

	module.RenderUserSection(w, r, h.templates, "qrcodes:show.html", map[string]interface{}{
		"Title":  "QR Code",
		"QRCode": code,
	})
}

// API handlers

func (h *Handler) APIList(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	codes, err := h.service.List(userID)
	if err != nil {
		h.jsonError(w, http.StatusInternalServerError, "Failed to list QR codes")
		return
	}
	h.json(w, http.StatusOK, map[string]interface{}{"qrcodes": codes})
}

func (h *Handler) APICreate(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var req models.CreateQRCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	if req.URL == "" {
		h.jsonError(w, http.StatusBadRequest, "URL is required")
		return
	}

	code, err := h.service.Create(userID, req)
	if err != nil {
		h.jsonError(w, http.StatusInternalServerError, "Failed to create QR code")
		return
	}

	h.json(w, http.StatusCreated, map[string]interface{}{"qrcode": code})
}

func (h *Handler) APIGet(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	id := chi.URLParam(r, "id")

	code, err := h.service.GetByUser(id, userID)
	if err != nil {
		h.jsonError(w, http.StatusNotFound, "QR code not found")
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"qrcode": code})
}

func (h *Handler) APIUpdate(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	id := chi.URLParam(r, "id")

	var req models.UpdateQRCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	code, err := h.service.Update(id, userID, req)
	if err != nil {
		h.jsonError(w, http.StatusInternalServerError, "Failed to update QR code")
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"qrcode": code})
}

func (h *Handler) APIDelete(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	id := chi.URLParam(r, "id")

	if err := h.service.Delete(id, userID); err != nil {
		h.jsonError(w, http.StatusInternalServerError, "Failed to delete QR code")
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"success": true})
}

func (h *Handler) APIDownload(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	id := chi.URLParam(r, "id")

	code, err := h.service.GetByUser(id, userID)
	if err != nil {
		h.jsonError(w, http.StatusNotFound, "QR code not found")
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "png"
	}

	sizeStr := r.URL.Query().Get("size")
	size := 400
	if sizeStr != "" {
		if s, err := strconv.Atoi(sizeStr); err == nil && s > 0 && s <= 2000 {
			size = s
		}
	}

	if format == "svg" {
		svg, err := services.GenerateSVG(code.URL, size, code.FGColor, code.BGColor)
		if err != nil {
			h.jsonError(w, http.StatusInternalServerError, "Failed to generate QR code")
			return
		}
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Content-Disposition", "attachment; filename=qrcode.svg")
		w.Write([]byte(svg))
	} else {
		png, err := services.GeneratePNG(code.URL, size, code.FGColor, code.BGColor)
		if err != nil {
			h.jsonError(w, http.StatusInternalServerError, "Failed to generate QR code")
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Disposition", "attachment; filename=qrcode.png")
		w.Write(png)
	}
}

func (h *Handler) APIPreview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL     string `json:"url"`
		FGColor string `json:"fgColor"`
		BGColor string `json:"bgColor"`
		Size    int    `json:"size"`
		Format  string `json:"format"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	if req.URL == "" {
		h.jsonError(w, http.StatusBadRequest, "URL is required")
		return
	}

	if req.FGColor == "" {
		req.FGColor = "#000000"
	}
	if req.BGColor == "" {
		req.BGColor = "#FFFFFF"
	}
	if req.Size == 0 || req.Size > 2000 {
		req.Size = 200
	}

	if req.Format == "svg" {
		svg, err := services.GenerateSVG(req.URL, req.Size, req.FGColor, req.BGColor)
		if err != nil {
			h.jsonError(w, http.StatusInternalServerError, "Failed to generate QR code")
			return
		}
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Write([]byte(svg))
	} else {
		png, err := services.GeneratePNG(req.URL, req.Size, req.FGColor, req.BGColor)
		if err != nil {
			h.jsonError(w, http.StatusInternalServerError, "Failed to generate QR code")
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write(png)
	}
}

func (h *Handler) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) jsonError(w http.ResponseWriter, status int, message string) {
	h.json(w, status, map[string]string{"error": message})
}
