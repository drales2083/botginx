package services

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/jung-kurt/gofpdf"
)

type ReportService struct {
	db *sqlx.DB
}

func NewReportService(db *sqlx.DB) *ReportService {
	return &ReportService{db: db}
}

type LinkStats struct {
	LinkID      string `db:"link_id"`
	Subdomain   string `db:"subdomain"`
	Domain      string `db:"domain"`
	Path        string `db:"path"`
	Destination string `db:"destination_url"`
	TotalClicks int    `db:"total_clicks"`
	UniqueIPs   int    `db:"unique_ips"`
}

type TrafficRow struct {
	Date      string `db:"date"`
	Country   string `db:"country"`
	Device    string `db:"device"`
	Clicks    int    `db:"clicks"`
	UniqueIPs int    `db:"unique_ips"`
}

type BotRow struct {
	Date         string `db:"date"`
	BotsBlocked  int    `db:"bots_blocked"`
	HumansPassed int    `db:"humans_passed"`
	BlockRate    string `db:"block_rate"`
}

func (s *ReportService) LinkPerformanceCSV(userID string, start, end time.Time) ([]byte, error) {
	rows, err := s.getLinkStats(userID, start, end)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	w.Write([]string{"Link ID", "Subdomain", "Domain", "Path", "Destination", "Total Clicks", "Unique IPs"})

	for _, r := range rows {
		w.Write([]string{
			r.LinkID,
			r.Subdomain,
			r.Domain,
			r.Path,
			r.Destination,
			fmt.Sprintf("%d", r.TotalClicks),
			fmt.Sprintf("%d", r.UniqueIPs),
		})
	}

	w.Flush()
	return buf.Bytes(), nil
}

func (s *ReportService) TrafficCSV(userID string, start, end time.Time) ([]byte, error) {
	query := `
		SELECT
			DATE(v.created_at) as date,
			COALESCE(v.country, 'Unknown') as country,
			COALESCE(v.device, 'Unknown') as device,
			COUNT(*) as clicks,
			COUNT(DISTINCT v.ip) as unique_ips
		FROM visits v
		INNER JOIN redirect_links rl ON rl.id = v.link_id
		WHERE rl.user_id = $1
			AND v.created_at >= $2
			AND v.created_at <= $3
		GROUP BY DATE(v.created_at), v.country, v.device
		ORDER BY date DESC, clicks DESC
	`

	var rows []TrafficRow
	err := s.db.Select(&rows, query, userID, start, end)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	w.Write([]string{"Date", "Country", "Device", "Clicks", "Unique IPs"})

	for _, r := range rows {
		w.Write([]string{
			r.Date,
			r.Country,
			r.Device,
			fmt.Sprintf("%d", r.Clicks),
			fmt.Sprintf("%d", r.UniqueIPs),
		})
	}

	w.Flush()
	return buf.Bytes(), nil
}

func (s *ReportService) BotProtectionCSV(userID string, start, end time.Time) ([]byte, error) {
	query := `
		SELECT
			DATE(v.created_at) as date,
			COUNT(*) FILTER (WHERE v.is_bot = true) as bots_blocked,
			COUNT(*) FILTER (WHERE v.is_bot = false OR v.is_bot IS NULL) as humans_passed
		FROM visits v
		INNER JOIN redirect_links rl ON rl.id = v.link_id
		WHERE rl.user_id = $1
			AND v.created_at >= $2
			AND v.created_at <= $3
		GROUP BY DATE(v.created_at)
		ORDER BY date DESC
	`

	type row struct {
		Date         string `db:"date"`
		BotsBlocked  int    `db:"bots_blocked"`
		HumansPassed int    `db:"humans_passed"`
	}

	var rows []row
	err := s.db.Select(&rows, query, userID, start, end)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	w.Write([]string{"Date", "Bots Blocked", "Humans Passed", "Block Rate %"})

	for _, r := range rows {
		total := r.BotsBlocked + r.HumansPassed
		rate := "0.0"
		if total > 0 {
			rate = fmt.Sprintf("%.1f", float64(r.BotsBlocked)/float64(total)*100)
		}
		w.Write([]string{
			r.Date,
			fmt.Sprintf("%d", r.BotsBlocked),
			fmt.Sprintf("%d", r.HumansPassed),
			rate,
		})
	}

	w.Flush()
	return buf.Bytes(), nil
}

func (s *ReportService) LinkPerformancePDF(userID string, start, end time.Time) ([]byte, error) {
	rows, err := s.getLinkStats(userID, start, end)
	if err != nil {
		return nil, err
	}

	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.SetTitle("Link Performance Report", true)
	pdf.AddPage()

	addHeader(pdf, "Link Performance Report", start, end)

	headers := []string{"Subdomain", "Domain", "Path", "Clicks", "Unique IPs"}
	widths := []float64{40, 50, 60, 30, 30}

	pdf.SetFont("Arial", "B", 10)
	pdf.SetFillColor(220, 220, 220)
	for i, h := range headers {
		pdf.CellFormat(widths[i], 8, h, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Arial", "", 9)
	for _, r := range rows {
		pdf.CellFormat(widths[0], 7, truncate(r.Subdomain, 20), "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[1], 7, truncate(r.Domain, 25), "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[2], 7, truncate(r.Path, 30), "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[3], 7, fmt.Sprintf("%d", r.TotalClicks), "1", 0, "R", false, 0, "")
		pdf.CellFormat(widths[4], 7, fmt.Sprintf("%d", r.UniqueIPs), "1", 0, "R", false, 0, "")
		pdf.Ln(-1)
	}

	var buf bytes.Buffer
	err = pdf.Output(&buf)
	return buf.Bytes(), err
}

func (s *ReportService) TrafficPDF(userID string, start, end time.Time) ([]byte, error) {
	rows, err := s.getTrafficStats(userID, start, end)
	if err != nil {
		return nil, err
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetTitle("Traffic Report", true)
	pdf.AddPage()

	addHeader(pdf, "Traffic Report", start, end)

	headers := []string{"Date", "Country", "Device", "Clicks", "Unique IPs"}
	widths := []float64{35, 40, 35, 30, 30}

	pdf.SetFont("Arial", "B", 10)
	pdf.SetFillColor(220, 220, 220)
	for i, h := range headers {
		pdf.CellFormat(widths[i], 8, h, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Arial", "", 9)
	for _, r := range rows {
		pdf.CellFormat(widths[0], 7, r.Date, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[1], 7, truncate(r.Country, 20), "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[2], 7, r.Device, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[3], 7, fmt.Sprintf("%d", r.Clicks), "1", 0, "R", false, 0, "")
		pdf.CellFormat(widths[4], 7, fmt.Sprintf("%d", r.UniqueIPs), "1", 0, "R", false, 0, "")
		pdf.Ln(-1)
	}

	var buf bytes.Buffer
	err = pdf.Output(&buf)
	return buf.Bytes(), err
}

func (s *ReportService) BotProtectionPDF(userID string, start, end time.Time) ([]byte, error) {
	rows, err := s.getBotStats(userID, start, end)
	if err != nil {
		return nil, err
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetTitle("Bot Protection Report", true)
	pdf.AddPage()

	addHeader(pdf, "Bot Protection Report", start, end)

	headers := []string{"Date", "Bots Blocked", "Humans Passed", "Block Rate %"}
	widths := []float64{45, 40, 40, 40}

	pdf.SetFont("Arial", "B", 10)
	pdf.SetFillColor(220, 220, 220)
	for i, h := range headers {
		pdf.CellFormat(widths[i], 8, h, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Arial", "", 9)
	for _, r := range rows {
		total := r.BotsBlocked + r.HumansPassed
		rate := "0.0%"
		if total > 0 {
			rate = fmt.Sprintf("%.1f%%", float64(r.BotsBlocked)/float64(total)*100)
		}
		pdf.CellFormat(widths[0], 7, r.Date, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[1], 7, fmt.Sprintf("%d", r.BotsBlocked), "1", 0, "R", false, 0, "")
		pdf.CellFormat(widths[2], 7, fmt.Sprintf("%d", r.HumansPassed), "1", 0, "R", false, 0, "")
		pdf.CellFormat(widths[3], 7, rate, "1", 0, "R", false, 0, "")
		pdf.Ln(-1)
	}

	var buf bytes.Buffer
	err = pdf.Output(&buf)
	return buf.Bytes(), err
}

func (s *ReportService) getLinkStats(userID string, start, end time.Time) ([]LinkStats, error) {
	query := `
		SELECT
			rl.id as link_id,
			rl.subdomain,
			d.name as domain,
			rl.path,
			COALESCE(rl.destination_urls->>0, '') as destination_url,
			COUNT(v.id) as total_clicks,
			COUNT(DISTINCT v.ip) as unique_ips
		FROM redirect_links rl
		INNER JOIN domains d ON d.id = rl.domain_id
		LEFT JOIN visits v ON v.link_id = rl.id
			AND v.created_at >= $2
			AND v.created_at <= $3
		WHERE rl.user_id = $1
		GROUP BY rl.id, rl.subdomain, d.name, rl.path, rl.destination_urls
		ORDER BY total_clicks DESC
	`
	var rows []LinkStats
	err := s.db.Select(&rows, query, userID, start, end)
	return rows, err
}

func (s *ReportService) getTrafficStats(userID string, start, end time.Time) ([]TrafficRow, error) {
	query := `
		SELECT
			DATE(v.created_at) as date,
			COALESCE(v.country, 'Unknown') as country,
			COALESCE(v.device, 'Unknown') as device,
			COUNT(*) as clicks,
			COUNT(DISTINCT v.ip) as unique_ips
		FROM visits v
		INNER JOIN redirect_links rl ON rl.id = v.link_id
		WHERE rl.user_id = $1
			AND v.created_at >= $2
			AND v.created_at <= $3
		GROUP BY DATE(v.created_at), v.country, v.device
		ORDER BY date DESC, clicks DESC
	`
	var rows []TrafficRow
	err := s.db.Select(&rows, query, userID, start, end)
	return rows, err
}

type botRow struct {
	Date         string `db:"date"`
	BotsBlocked  int    `db:"bots_blocked"`
	HumansPassed int    `db:"humans_passed"`
}

func (s *ReportService) getBotStats(userID string, start, end time.Time) ([]botRow, error) {
	query := `
		SELECT
			DATE(v.created_at) as date,
			COUNT(*) FILTER (WHERE v.is_bot = true) as bots_blocked,
			COUNT(*) FILTER (WHERE v.is_bot = false OR v.is_bot IS NULL) as humans_passed
		FROM visits v
		INNER JOIN redirect_links rl ON rl.id = v.link_id
		WHERE rl.user_id = $1
			AND v.created_at >= $2
			AND v.created_at <= $3
		GROUP BY DATE(v.created_at)
		ORDER BY date DESC
	`
	var rows []botRow
	err := s.db.Select(&rows, query, userID, start, end)
	return rows, err
}

func addHeader(pdf *gofpdf.Fpdf, title string, start, end time.Time) {
	appName := os.Getenv("UI_APP_NAME")
	if appName == "" {
		appName = "GuardBot"
	}

	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(0, 10, title)
	pdf.Ln(12)

	pdf.SetFont("Arial", "", 10)
	pdf.SetTextColor(100, 100, 100)
	pdf.Cell(0, 6, fmt.Sprintf("Generated by %s on %s", appName, time.Now().Format("Jan 2, 2006")))
	pdf.Ln(6)
	pdf.Cell(0, 6, fmt.Sprintf("Date Range: %s to %s", start.Format("Jan 2, 2006"), end.Format("Jan 2, 2006")))
	pdf.Ln(12)
	pdf.SetTextColor(0, 0, 0)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
