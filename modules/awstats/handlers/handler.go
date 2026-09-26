package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/botginx/botginx/modules/awstats/services"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
)

type Handler struct {
	service   *services.AWStatsService
	templates *module.TemplateEngine
}

func NewHandler(service *services.AWStatsService, tmpl *module.TemplateEngine) *Handler {
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

func (h *Handler) getYearMonth(r *http.Request) (int, int) {
	now := time.Now()
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	month, _ := strconv.Atoi(r.URL.Query().Get("month"))

	if year == 0 {
		year = now.Year()
	}
	if month == 0 || month < 1 || month > 12 {
		month = int(now.Month())
	}

	return year, month
}

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	module.RenderUserSection(w, r, h.templates, "awstats:index.html", map[string]interface{}{
		"Title": "AWStats",
	})
}

func (h *Handler) APISummary(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	year, month := h.getYearMonth(r)

	summary, err := h.service.GetSummary(userID, year, month)
	if err != nil {
		h.jsonError(w, "Failed to get summary", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, summary)
}

func (h *Handler) APIMonthly(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	stats, err := h.service.GetMonthlyHistory(userID)
	if err != nil {
		h.jsonError(w, "Failed to get monthly stats", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, stats)
}

func (h *Handler) APIDaily(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	year, month := h.getYearMonth(r)

	stats, err := h.service.GetDailyStats(userID, year, month)
	if err != nil {
		h.jsonError(w, "Failed to get daily stats", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, stats)
}

func (h *Handler) APIHours(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	year, month := h.getYearMonth(r)

	stats, err := h.service.GetHourlyStats(userID, year, month)
	if err != nil {
		h.jsonError(w, "Failed to get hourly stats", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, stats)
}

func (h *Handler) APICountries(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	year, month := h.getYearMonth(r)

	stats, err := h.service.GetCountryStats(userID, year, month)
	if err != nil {
		h.jsonError(w, "Failed to get country stats", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, stats)
}

func (h *Handler) APIBrowsers(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	year, month := h.getYearMonth(r)

	stats, err := h.service.GetBrowserStats(userID, year, month)
	if err != nil {
		h.jsonError(w, "Failed to get browser stats", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, stats)
}

func (h *Handler) APIOS(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	year, month := h.getYearMonth(r)

	stats, err := h.service.GetOSStats(userID, year, month)
	if err != nil {
		h.jsonError(w, "Failed to get OS stats", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, stats)
}

func (h *Handler) APIRobots(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	year, month := h.getYearMonth(r)

	stats, err := h.service.GetRobotStats(userID, year, month)
	if err != nil {
		h.jsonError(w, "Failed to get robot stats", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, stats)
}

func (h *Handler) APIStatus(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	year, month := h.getYearMonth(r)

	stats, err := h.service.GetStatusStats(userID, year, month)
	if err != nil {
		h.jsonError(w, "Failed to get status stats", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, stats)
}

func (h *Handler) APIPages(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	year, month := h.getYearMonth(r)

	stats, err := h.service.GetPageStats(userID, year, month)
	if err != nil {
		h.jsonError(w, "Failed to get page stats", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, stats)
}
