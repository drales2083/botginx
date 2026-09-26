package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/botginx/botginx/modules/analogstats/services"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
)

type Handler struct {
	service   *services.StatsService
	templates *module.TemplateEngine
}

func NewHandler(service *services.StatsService, tmpl *module.TemplateEngine) *Handler {
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
	module.RenderUserSection(w, r, h.templates, "analogstats:index.html", map[string]interface{}{
		"Title": "Analog Stats",
	})
}

func (h *Handler) APISummary(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	days := h.getDays(r)

	summary, err := h.service.GetSummary(userID, days)
	if err != nil {
		h.jsonError(w, "Failed to get summary", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, summary)
}

func (h *Handler) APIHourly(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	days := h.getDays(r)

	stats, err := h.service.GetHourlyStats(userID, days)
	if err != nil {
		h.jsonError(w, "Failed to get hourly stats", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, stats)
}

func (h *Handler) APIDaily(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	days := h.getDays(r)

	stats, err := h.service.GetDailyStats(userID, days)
	if err != nil {
		h.jsonError(w, "Failed to get daily stats", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, stats)
}

func (h *Handler) APIBrowsers(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	days := h.getDays(r)

	stats, err := h.service.GetBrowserStats(userID, days)
	if err != nil {
		h.jsonError(w, "Failed to get browser stats", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, stats)
}

func (h *Handler) APIOS(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	days := h.getDays(r)

	stats, err := h.service.GetOSStats(userID, days)
	if err != nil {
		h.jsonError(w, "Failed to get OS stats", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, stats)
}

func (h *Handler) APICountries(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	days := h.getDays(r)

	stats, err := h.service.GetCountryStats(userID, days)
	if err != nil {
		h.jsonError(w, "Failed to get country stats", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, stats)
}

func (h *Handler) APIPages(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	days := h.getDays(r)

	stats, err := h.service.GetPageStats(userID, days)
	if err != nil {
		h.jsonError(w, "Failed to get page stats", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, stats)
}

func (h *Handler) getDays(r *http.Request) int {
	days, err := strconv.Atoi(r.URL.Query().Get("days"))
	if err != nil || days < 1 {
		return 7
	}
	if days > 30 {
		return 30
	}
	return days
}
