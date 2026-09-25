package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/botginx/botginx/modules/domainhealth/services"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service   *services.HealthService
	templates *module.TemplateEngine
}

func NewHandler(service *services.HealthService, templates *module.TemplateEngine) *Handler {
	return &Handler{
		service:   service,
		templates: templates,
	}
}

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	domains, _ := h.service.GetUserDomains(userID)

	module.RenderUserSection(w, r, h.templates, "domainhealth:index.html", map[string]interface{}{
		"Title":   "Domain Health",
		"Domains": domains,
	})
}

func (h *Handler) CheckDomain(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	id := chi.URLParam(r, "id")

	domain, err := h.service.GetDomain(id, userID)
	if err != nil {
		h.jsonError(w, http.StatusNotFound, "Domain not found")
		return
	}

	status := h.service.CheckDomain(*domain)

	h.json(w, http.StatusOK, map[string]interface{}{
		"domain":       status.Domain.Name,
		"dns":          map[string]string{"status": status.DNSStatus, "message": status.DNSMessage},
		"ssl":          map[string]interface{}{"status": status.SSLStatus, "message": status.SSLMessage, "expiry": status.SSLExpiry},
		"http":         map[string]interface{}{"status": status.HTTPStatus, "message": status.HTTPMessage, "responseTime": status.ResponseTime},
	})
}

func (h *Handler) CheckAll(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	domains, err := h.service.GetUserDomains(userID)
	if err != nil {
		h.jsonError(w, http.StatusInternalServerError, "Failed to get domains")
		return
	}

	results := make([]map[string]interface{}, 0, len(domains))
	for _, d := range domains {
		status := h.service.CheckDomain(d)
		results = append(results, map[string]interface{}{
			"id":     d.ID,
			"domain": d.Name,
			"dns":    map[string]string{"status": status.DNSStatus, "message": status.DNSMessage},
			"ssl":    map[string]interface{}{"status": status.SSLStatus, "message": status.SSLMessage, "expiry": status.SSLExpiry},
			"http":   map[string]interface{}{"status": status.HTTPStatus, "message": status.HTTPMessage, "responseTime": status.ResponseTime},
		})
	}

	h.json(w, http.StatusOK, map[string]interface{}{"results": results})
}

func (h *Handler) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) jsonError(w http.ResponseWriter, status int, message string) {
	h.json(w, status, map[string]string{"error": message})
}
