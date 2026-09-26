package handlers

import (
	"net/http"
	"strconv"

	"github.com/botginx/botginx/modules/analytics/services"
	"github.com/botginx/botginx/pkg/apiauth"
	"github.com/go-chi/chi/v5"
)

type AnalyticsHandler struct {
	service *services.AnalyticsService
}

func NewAnalyticsHandler(service *services.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{service: service}
}

func (h *AnalyticsHandler) Overview(w http.ResponseWriter, r *http.Request) {
	key := apiauth.GetAPIKey(r)
	if key == nil {
		apiauth.ErrorResp(w, "unauthorized", "API key required", http.StatusUnauthorized)
		return
	}

	if !key.HasScope("analytics:read") {
		apiauth.ErrorResp(w, "forbidden", "Missing scope: analytics:read", http.StatusForbidden)
		return
	}

	stats, err := h.service.GetUserStats(key.UserID)
	if err != nil {
		apiauth.ErrorResp(w, "fetch_failed", "Failed to fetch analytics", http.StatusInternalServerError)
		return
	}

	apiauth.Success(w, map[string]interface{}{
		"totalVisits":    stats.TotalVisits,
		"uniqueVisits":   stats.UniqueVisits,
		"botVisits":      stats.BotVisits,
		"blockedVisits":  stats.BlockedVisits,
		"todayVisits":    stats.TodayVisits,
		"conversions":    stats.Conversions,
		"conversionRate": stats.ConversionRate,
	})
}

func (h *AnalyticsHandler) LinkStats(w http.ResponseWriter, r *http.Request) {
	key := apiauth.GetAPIKey(r)
	if key == nil {
		apiauth.ErrorResp(w, "unauthorized", "API key required", http.StatusUnauthorized)
		return
	}

	if !key.HasScope("analytics:read") {
		apiauth.ErrorResp(w, "forbidden", "Missing scope: analytics:read", http.StatusForbidden)
		return
	}

	linkID := chi.URLParam(r, "id")

	stats, err := h.service.GetLinkStats(linkID)
	if err != nil {
		apiauth.ErrorResp(w, "fetch_failed", "Failed to fetch link stats", http.StatusInternalServerError)
		return
	}

	apiauth.Success(w, map[string]interface{}{
		"linkId":           stats.LinkID,
		"totalVisits":      stats.TotalVisits,
		"uniqueVisits":     stats.UniqueVisits,
		"botVisits":        stats.BotVisits,
		"blockedVisits":    stats.BlockedVisits,
		"todayVisits":      stats.TodayVisits,
		"conversions":      stats.Conversions,
		"conversionRate":   stats.ConversionRate,
		"torVisits":        stats.TorVisits,
		"proxyVisits":      stats.ProxyVisits,
		"avgBehaviorScore": stats.AvgBehaviorScore,
	})
}

func (h *AnalyticsHandler) LinkTimeline(w http.ResponseWriter, r *http.Request) {
	key := apiauth.GetAPIKey(r)
	if key == nil {
		apiauth.ErrorResp(w, "unauthorized", "API key required", http.StatusUnauthorized)
		return
	}

	if !key.HasScope("analytics:read") {
		apiauth.ErrorResp(w, "forbidden", "Missing scope: analytics:read", http.StatusForbidden)
		return
	}

	linkID := chi.URLParam(r, "id")
	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "daily"
	}

	timeline, err := h.service.GetTimeline(linkID, interval)
	if err != nil {
		apiauth.ErrorResp(w, "fetch_failed", "Failed to fetch timeline", http.StatusInternalServerError)
		return
	}

	result := make([]map[string]interface{}, 0, len(timeline))
	for _, point := range timeline {
		result = append(result, map[string]interface{}{
			"time":  point.Time,
			"count": point.Count,
		})
	}

	apiauth.Success(w, result)
}

func (h *AnalyticsHandler) LinkCountries(w http.ResponseWriter, r *http.Request) {
	key := apiauth.GetAPIKey(r)
	if key == nil {
		apiauth.ErrorResp(w, "unauthorized", "API key required", http.StatusUnauthorized)
		return
	}

	if !key.HasScope("analytics:read") {
		apiauth.ErrorResp(w, "forbidden", "Missing scope: analytics:read", http.StatusForbidden)
		return
	}

	linkID := chi.URLParam(r, "id")
	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	countries, err := h.service.GetCountryStats(linkID, limit)
	if err != nil {
		apiauth.ErrorResp(w, "fetch_failed", "Failed to fetch country stats", http.StatusInternalServerError)
		return
	}

	result := make([]map[string]interface{}, 0, len(countries))
	for _, c := range countries {
		result = append(result, map[string]interface{}{
			"country": c.Country,
			"count":   c.Count,
		})
	}

	apiauth.Success(w, result)
}

func (h *AnalyticsHandler) LinkDevices(w http.ResponseWriter, r *http.Request) {
	key := apiauth.GetAPIKey(r)
	if key == nil {
		apiauth.ErrorResp(w, "unauthorized", "API key required", http.StatusUnauthorized)
		return
	}

	if !key.HasScope("analytics:read") {
		apiauth.ErrorResp(w, "forbidden", "Missing scope: analytics:read", http.StatusForbidden)
		return
	}

	linkID := chi.URLParam(r, "id")

	devices, err := h.service.GetDeviceStats(linkID)
	if err != nil {
		apiauth.ErrorResp(w, "fetch_failed", "Failed to fetch device stats", http.StatusInternalServerError)
		return
	}

	result := make([]map[string]interface{}, 0, len(devices))
	for _, d := range devices {
		result = append(result, map[string]interface{}{
			"device": d.Device,
			"count":  d.Count,
		})
	}

	apiauth.Success(w, result)
}

func (h *AnalyticsHandler) LinkReferrers(w http.ResponseWriter, r *http.Request) {
	key := apiauth.GetAPIKey(r)
	if key == nil {
		apiauth.ErrorResp(w, "unauthorized", "API key required", http.StatusUnauthorized)
		return
	}

	if !key.HasScope("analytics:read") {
		apiauth.ErrorResp(w, "forbidden", "Missing scope: analytics:read", http.StatusForbidden)
		return
	}

	linkID := chi.URLParam(r, "id")
	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	referrers, err := h.service.GetReferrerStats(linkID, limit)
	if err != nil {
		apiauth.ErrorResp(w, "fetch_failed", "Failed to fetch referrer stats", http.StatusInternalServerError)
		return
	}

	result := make([]map[string]interface{}, 0, len(referrers))
	for _, r := range referrers {
		result = append(result, map[string]interface{}{
			"referrer": r.Referrer,
			"count":    r.Count,
		})
	}

	apiauth.Success(w, result)
}
