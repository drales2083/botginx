package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/botginx/botginx/modules/reports/services"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
)

type Handler struct {
	service   *services.ReportService
	templates *module.TemplateEngine
}

func NewHandler(service *services.ReportService, templates *module.TemplateEngine) *Handler {
	return &Handler{
		service:   service,
		templates: templates,
	}
}

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	module.RenderUserSection(w, r, h.templates, "reports:index.html", map[string]interface{}{
		"Title": "Reports",
	})
}

func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	reportType := r.URL.Query().Get("type")
	if reportType == "" {
		reportType = "links"
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "csv"
	}

	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	var startDate, endDate time.Time
	var err error

	if startStr != "" {
		startDate, err = time.Parse("2006-01-02", startStr)
		if err != nil {
			http.Error(w, "Invalid start date", http.StatusBadRequest)
			return
		}
	} else {
		startDate = time.Now().AddDate(0, -1, 0)
	}

	if endStr != "" {
		endDate, err = time.Parse("2006-01-02", endStr)
		if err != nil {
			http.Error(w, "Invalid end date", http.StatusBadRequest)
			return
		}
	} else {
		endDate = time.Now()
	}

	var data []byte
	var filename string
	var contentType string

	dateStr := time.Now().Format("2006-01-02")

	if format == "pdf" {
		contentType = "application/pdf"
		switch reportType {
		case "links":
			data, err = h.service.LinkPerformancePDF(userID, startDate, endDate)
			filename = fmt.Sprintf("link-performance-%s.pdf", dateStr)
		case "traffic":
			data, err = h.service.TrafficPDF(userID, startDate, endDate)
			filename = fmt.Sprintf("traffic-report-%s.pdf", dateStr)
		case "bots":
			data, err = h.service.BotProtectionPDF(userID, startDate, endDate)
			filename = fmt.Sprintf("bot-protection-%s.pdf", dateStr)
		default:
			http.Error(w, "Invalid report type", http.StatusBadRequest)
			return
		}
	} else {
		contentType = "text/csv"
		switch reportType {
		case "links":
			data, err = h.service.LinkPerformanceCSV(userID, startDate, endDate)
			filename = fmt.Sprintf("link-performance-%s.csv", dateStr)
		case "traffic":
			data, err = h.service.TrafficCSV(userID, startDate, endDate)
			filename = fmt.Sprintf("traffic-report-%s.csv", dateStr)
		case "bots":
			data, err = h.service.BotProtectionCSV(userID, startDate, endDate)
			filename = fmt.Sprintf("bot-protection-%s.csv", dateStr)
		default:
			http.Error(w, "Invalid report type", http.StatusBadRequest)
			return
		}
	}

	if err != nil {
		http.Error(w, "Failed to generate report", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Write(data)
}
