package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/botginx/botginx/modules/pdfgenerator/services"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

// linkOption represents a redirect link for the dropdown
type linkOption struct {
	ID   string `db:"id" json:"id"`
	Name string `db:"name" json:"name"`
	URL  string `db:"url" json:"url"`
}

// BackgroundInfo describes a preset background
type BackgroundInfo struct {
	Name     string `json:"name"`
	Filename string `json:"filename"`
}

type Handler struct {
	service       *services.PDFService
	templates     *module.TemplateEngine
	db            *sqlx.DB
	backgroundsFS fs.FS
	backgrounds   []BackgroundInfo
}

func NewHandler(service *services.PDFService, templates *module.TemplateEngine, db *sqlx.DB, backgroundsFS fs.FS) *Handler {
	h := &Handler{
		service:       service,
		templates:     templates,
		db:            db,
		backgroundsFS: backgroundsFS,
	}

	// Load background list
	h.loadBackgrounds()

	return h
}

func (h *Handler) loadBackgrounds() {
	entries, err := fs.ReadDir(h.backgroundsFS, ".")
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
			continue
		}

		// Create display name from filename
		displayName := strings.TrimSuffix(name, ext)
		displayName = strings.ReplaceAll(displayName, "-", " ")
		displayName = strings.ReplaceAll(displayName, "_", " ")
		displayName = strings.Title(displayName)

		h.backgrounds = append(h.backgrounds, BackgroundInfo{
			Name:     displayName,
			Filename: name,
		})
	}
}

// Index renders the PDF generator page
func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	// Get user's links for dropdown
	links := h.getUserLinks(userID)

	data := map[string]interface{}{
		"Title":       "Generate PDF",
		"Links":       links,
		"Backgrounds": h.backgrounds,
	}

	module.RenderUserSection(w, r, h.templates, "pdfgenerator:index.html", data)
}

// GetLinks returns user's redirect links as JSON
func (h *Handler) GetLinks(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	links := h.getUserLinks(userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(links)
}

// getUserLinks fetches redirect links from database
func (h *Handler) getUserLinks(userID string) []linkOption {
	var links []linkOption
	err := h.db.Select(&links, `
		SELECT
			r.id,
			r.subdomain || '.' || REPLACE(d.name, '*.', '') as name,
			'https://' || r.subdomain || '.' || REPLACE(d.name, '*.', '') as url
		FROM redirect_links r
		JOIN domains d ON r.domain_id = d.id
		WHERE r.user_id = $1 AND r.is_active = true
		ORDER BY r.created_at DESC
		LIMIT 50
	`, userID)
	if err != nil {
		log.Printf("[pdfgenerator] Error fetching links for user %s: %v", userID, err)
	}
	log.Printf("[pdfgenerator] Fetched %d links for user %s", len(links), userID)
	return links
}

// GetBackgrounds returns list of preset backgrounds
func (h *Handler) GetBackgrounds(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h.backgrounds)
}

// GetBackgroundImage serves a preset background image
func (h *Handler) GetBackgroundImage(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	// Security: prevent path traversal
	name = filepath.Base(name)

	f, err := h.backgroundsFS.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	// Set content type based on extension
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}

	// Read and serve
	data, err := fs.ReadFile(h.backgroundsFS, name)
	if err != nil {
		http.Error(w, "Failed to read image", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(data)
}

// Preview generates a preview PNG
func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	var req services.PDFRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	preview, err := h.service.GeneratePreview(req)
	if err != nil {
		http.Error(w, "Preview generation failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return as base64 data URL
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(preview)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"preview": dataURL})
}

// Generate creates and returns the PDF for download
func (h *Handler) Generate(w http.ResponseWriter, r *http.Request) {
	var req services.PDFRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.ImageDataURL == "" && req.PresetImage == "" {
		http.Error(w, "No background image selected", http.StatusBadRequest)
		return
	}
	if req.LinkURL == "" {
		http.Error(w, "No destination link provided", http.StatusBadRequest)
		return
	}

	pdfData, err := h.service.GeneratePDF(req)
	if err != nil {
		log.Printf("[pdfgenerator] PDF generation failed: %v", err)
		http.Error(w, "PDF generation failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("[pdfgenerator] Generated PDF, size: %d bytes", len(pdfData))

	// Send as downloadable PDF
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfData)))
	w.Header().Set("Content-Disposition", "attachment; filename=generated.pdf")
	n, err := w.Write(pdfData)
	if err != nil {
		log.Printf("[pdfgenerator] Write error: %v (wrote %d of %d bytes)", err, n, len(pdfData))
	}
}
