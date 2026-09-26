package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/botginx/botginx/modules/threatlog/services"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
)

type Handler struct {
	service   *services.ThreatLogService
	templates *module.TemplateEngine
}

func NewHandler(service *services.ThreatLogService, tmpl *module.TemplateEngine) *Handler {
	return &Handler{
		service:   service,
		templates: tmpl,
	}
}

func (h *Handler) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) jsonError(w http.ResponseWriter, msg string, status int) {
	h.json(w, status, map[string]string{"error": msg})
}

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	module.RenderUserSection(w, r, h.templates, "threatlog:index.html", map[string]interface{}{
		"Title": "Threat Log",
	})
}

func (h *Handler) APIThreats(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	filters := map[string]string{
		"country":  r.URL.Query().Get("country"),
		"botType":  r.URL.Query().Get("botType"),
		"dateFrom": r.URL.Query().Get("dateFrom"),
		"dateTo":   r.URL.Query().Get("dateTo"),
	}

	threats, total, err := h.service.GetThreats(userID, limit, offset, filters)
	if err != nil {
		h.jsonError(w, "Failed to get threats", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"threats": threats,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

func (h *Handler) APIStats(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	stats, err := h.service.GetStats(userID)
	if err != nil {
		h.jsonError(w, "Failed to get stats", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, stats)
}

func (h *Handler) APIRecent(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	threats, err := h.service.GetRecentThreats(userID, limit)
	if err != nil {
		h.jsonError(w, "Failed to get recent threats", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, threats)
}

func (h *Handler) APIFilters(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	countries, _ := h.service.GetCountries(userID)
	botTypes, _ := h.service.GetBotTypes(userID)

	h.json(w, http.StatusOK, map[string]interface{}{
		"countries": countries,
		"botTypes":  botTypes,
	})
}
