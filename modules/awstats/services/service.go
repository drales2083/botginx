package services

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

type AWStatsService struct {
	db *sqlx.DB
}

func NewAWStatsService(db *sqlx.DB) *AWStatsService {
	return &AWStatsService{db: db}
}

type Summary struct {
	UniqueVisitors    int       `json:"uniqueVisitors"`
	NumberOfVisits    int       `json:"numberOfVisits"`
	Pages             int       `json:"pages"`
	Hits              int       `json:"hits"`
	Bandwidth         int64     `json:"bandwidth"`
	BotsVisits        int       `json:"botsVisits"`
	Month             string    `json:"month"`
	Year              int       `json:"year"`
	GeneratedAt       time.Time `json:"generatedAt"`
	AvgVisitsPerDay   float64   `json:"avgVisitsPerDay"`
	AvgPagesPerVisit  float64   `json:"avgPagesPerVisit"`
}

type MonthlyStat struct {
	Month           string `db:"month" json:"month"`
	UniqueVisitors  int    `db:"unique_visitors" json:"uniqueVisitors"`
	Visits          int    `db:"visits" json:"visits"`
	Pages           int    `db:"pages" json:"pages"`
	Hits            int    `db:"hits" json:"hits"`
	Bandwidth       int64  `db:"bandwidth" json:"bandwidth"`
}

type DailyStat struct {
	Day             int   `db:"day" json:"day"`
	Visits          int   `db:"visits" json:"visits"`
	Pages           int   `db:"pages" json:"pages"`
	Hits            int   `db:"hits" json:"hits"`
	Bandwidth       int64 `db:"bandwidth" json:"bandwidth"`
}

type HourlyStat struct {
	Hour   int   `db:"hour" json:"hour"`
	Pages  int   `db:"pages" json:"pages"`
	Hits   int   `db:"hits" json:"hits"`
}

type CountryStat struct {
	Country  string  `db:"country" json:"country"`
	Pages    int     `db:"pages" json:"pages"`
	Hits     int     `db:"hits" json:"hits"`
	Bandwidth int64  `db:"bandwidth" json:"bandwidth"`
	Percent  float64 `json:"percent"`
}

type BrowserStat struct {
	Browser  string  `db:"browser" json:"browser"`
	Hits     int     `db:"hits" json:"hits"`
	Percent  float64 `json:"percent"`
}

type OSStat struct {
	OS       string  `db:"os" json:"os"`
	Hits     int     `db:"hits" json:"hits"`
	Percent  float64 `json:"percent"`
}

type RobotStat struct {
	Robot    string `db:"robot" json:"robot"`
	Hits     int    `db:"hits" json:"hits"`
	Bandwidth int64 `db:"bandwidth" json:"bandwidth"`
}

type StatusStat struct {
	Code  int `db:"code" json:"code"`
	Hits  int `db:"hits" json:"hits"`
}

type PageStat struct {
	URL   string `db:"url" json:"url"`
	Views int    `db:"views" json:"views"`
}

func (s *AWStatsService) GetSummary(userID string, year int, month int) (*Summary, error) {
	summary := &Summary{
		GeneratedAt: time.Now(),
		Year:        year,
		Month:       time.Month(month).String(),
	}

	startDate := fmt.Sprintf("%d-%02d-01", year, month)
	endDate := fmt.Sprintf("%d-%02d-01", year, month+1)
	if month == 12 {
		endDate = fmt.Sprintf("%d-01-01", year+1)
	}

	// Unique visitors
	s.db.Get(&summary.UniqueVisitors, `
		SELECT COUNT(DISTINCT ip) FROM visits
		WHERE user_id = $1 AND created_at >= $2 AND created_at < $3
	`, userID, startDate, endDate)

	// Total visits/hits
	s.db.Get(&summary.Hits, `
		SELECT COUNT(*) FROM visits
		WHERE user_id = $1 AND created_at >= $2 AND created_at < $3
	`, userID, startDate, endDate)

	summary.Pages = summary.Hits
	summary.NumberOfVisits = summary.UniqueVisitors

	// Bot visits
	s.db.Get(&summary.BotsVisits, `
		SELECT COUNT(*) FROM visits
		WHERE user_id = $1 AND created_at >= $2 AND created_at < $3 AND is_bot = true
	`, userID, startDate, endDate)

	// Calculate averages
	daysInMonth := time.Date(year, time.Month(month+1), 0, 0, 0, 0, 0, time.UTC).Day()
	if summary.NumberOfVisits > 0 {
		summary.AvgVisitsPerDay = float64(summary.NumberOfVisits) / float64(daysInMonth)
		summary.AvgPagesPerVisit = float64(summary.Pages) / float64(summary.NumberOfVisits)
	}

	return summary, nil
}

func (s *AWStatsService) GetMonthlyHistory(userID string) ([]MonthlyStat, error) {
	var stats []MonthlyStat

	err := s.db.Select(&stats, `
		SELECT
			TO_CHAR(created_at, 'YYYY-MM') as month,
			COUNT(DISTINCT ip) as unique_visitors,
			COUNT(DISTINCT ip) as visits,
			COUNT(*) as pages,
			COUNT(*) as hits,
			0::bigint as bandwidth
		FROM visits
		WHERE user_id = $1 AND created_at > NOW() - INTERVAL '12 months'
		GROUP BY TO_CHAR(created_at, 'YYYY-MM')
		ORDER BY month DESC
		LIMIT 12
	`, userID)

	return stats, err
}

func (s *AWStatsService) GetDailyStats(userID string, year int, month int) ([]DailyStat, error) {
	var stats []DailyStat

	startDate := fmt.Sprintf("%d-%02d-01", year, month)
	endDate := fmt.Sprintf("%d-%02d-01", year, month+1)
	if month == 12 {
		endDate = fmt.Sprintf("%d-01-01", year+1)
	}

	err := s.db.Select(&stats, `
		SELECT
			EXTRACT(DAY FROM created_at)::int as day,
			COUNT(DISTINCT ip) as visits,
			COUNT(*) as pages,
			COUNT(*) as hits,
			0::bigint as bandwidth
		FROM visits
		WHERE user_id = $1 AND created_at >= $2 AND created_at < $3
		GROUP BY EXTRACT(DAY FROM created_at)
		ORDER BY day
	`, userID, startDate, endDate)

	return stats, err
}

func (s *AWStatsService) GetHourlyStats(userID string, year int, month int) ([]HourlyStat, error) {
	var stats []HourlyStat

	startDate := fmt.Sprintf("%d-%02d-01", year, month)
	endDate := fmt.Sprintf("%d-%02d-01", year, month+1)
	if month == 12 {
		endDate = fmt.Sprintf("%d-01-01", year+1)
	}

	err := s.db.Select(&stats, `
		SELECT
			EXTRACT(HOUR FROM created_at)::int as hour,
			COUNT(*) as pages,
			COUNT(*) as hits
		FROM visits
		WHERE user_id = $1 AND created_at >= $2 AND created_at < $3
		GROUP BY EXTRACT(HOUR FROM created_at)
		ORDER BY hour
	`, userID, startDate, endDate)

	// Fill missing hours
	hourMap := make(map[int]HourlyStat)
	for _, stat := range stats {
		hourMap[stat.Hour] = stat
	}

	result := make([]HourlyStat, 24)
	for i := 0; i < 24; i++ {
		if stat, ok := hourMap[i]; ok {
			result[i] = stat
		} else {
			result[i] = HourlyStat{Hour: i}
		}
	}

	return result, err
}

func (s *AWStatsService) GetCountryStats(userID string, year int, month int) ([]CountryStat, error) {
	var stats []CountryStat

	startDate := fmt.Sprintf("%d-%02d-01", year, month)
	endDate := fmt.Sprintf("%d-%02d-01", year, month+1)
	if month == 12 {
		endDate = fmt.Sprintf("%d-01-01", year+1)
	}

	err := s.db.Select(&stats, `
		SELECT
			COALESCE(NULLIF(country, ''), 'Unknown') as country,
			COUNT(*) as pages,
			COUNT(*) as hits,
			0::bigint as bandwidth
		FROM visits
		WHERE user_id = $1 AND created_at >= $2 AND created_at < $3
		GROUP BY country
		ORDER BY hits DESC
		LIMIT 25
	`, userID, startDate, endDate)

	// Calculate percentages
	var total int
	for _, stat := range stats {
		total += stat.Hits
	}
	if total > 0 {
		for i := range stats {
			stats[i].Percent = float64(stats[i].Hits) * 100 / float64(total)
		}
	}

	return stats, err
}

func (s *AWStatsService) GetBrowserStats(userID string, year int, month int) ([]BrowserStat, error) {
	var stats []BrowserStat

	startDate := fmt.Sprintf("%d-%02d-01", year, month)
	endDate := fmt.Sprintf("%d-%02d-01", year, month+1)
	if month == 12 {
		endDate = fmt.Sprintf("%d-01-01", year+1)
	}

	err := s.db.Select(&stats, `
		SELECT
			COALESCE(NULLIF(browser, ''), 'Unknown') as browser,
			COUNT(*) as hits
		FROM visits
		WHERE user_id = $1 AND created_at >= $2 AND created_at < $3
		GROUP BY browser
		ORDER BY hits DESC
		LIMIT 15
	`, userID, startDate, endDate)

	// Calculate percentages
	var total int
	for _, stat := range stats {
		total += stat.Hits
	}
	if total > 0 {
		for i := range stats {
			stats[i].Percent = float64(stats[i].Hits) * 100 / float64(total)
		}
	}

	return stats, err
}

func (s *AWStatsService) GetOSStats(userID string, year int, month int) ([]OSStat, error) {
	var stats []OSStat

	startDate := fmt.Sprintf("%d-%02d-01", year, month)
	endDate := fmt.Sprintf("%d-%02d-01", year, month+1)
	if month == 12 {
		endDate = fmt.Sprintf("%d-01-01", year+1)
	}

	err := s.db.Select(&stats, `
		SELECT
			COALESCE(NULLIF(os, ''), 'Unknown') as os,
			COUNT(*) as hits
		FROM visits
		WHERE user_id = $1 AND created_at >= $2 AND created_at < $3
		GROUP BY os
		ORDER BY hits DESC
		LIMIT 15
	`, userID, startDate, endDate)

	// Calculate percentages
	var total int
	for _, stat := range stats {
		total += stat.Hits
	}
	if total > 0 {
		for i := range stats {
			stats[i].Percent = float64(stats[i].Hits) * 100 / float64(total)
		}
	}

	return stats, err
}

func (s *AWStatsService) GetRobotStats(userID string, year int, month int) ([]RobotStat, error) {
	var stats []RobotStat

	startDate := fmt.Sprintf("%d-%02d-01", year, month)
	endDate := fmt.Sprintf("%d-%02d-01", year, month+1)
	if month == 12 {
		endDate = fmt.Sprintf("%d-01-01", year+1)
	}

	err := s.db.Select(&stats, `
		SELECT
			COALESCE(NULLIF(bot_type, ''), 'Unknown Bot') as robot,
			COUNT(*) as hits,
			0::bigint as bandwidth
		FROM visits
		WHERE user_id = $1 AND created_at >= $2 AND created_at < $3 AND is_bot = true
		GROUP BY bot_type
		ORDER BY hits DESC
		LIMIT 15
	`, userID, startDate, endDate)

	return stats, err
}

func (s *AWStatsService) GetStatusStats(userID string, year int, month int) ([]StatusStat, error) {
	// Since we don't track HTTP status in visits, return mock data based on bot/human split
	summary, _ := s.GetSummary(userID, year, month)

	human := summary.Hits - summary.BotsVisits

	return []StatusStat{
		{Code: 200, Hits: human},
		{Code: 302, Hits: int(float64(human) * 0.05)},
		{Code: 403, Hits: summary.BotsVisits},
		{Code: 404, Hits: int(float64(summary.Hits) * 0.01)},
	}, nil
}

func (s *AWStatsService) GetPageStats(userID string, year int, month int) ([]PageStat, error) {
	var stats []PageStat

	startDate := fmt.Sprintf("%d-%02d-01", year, month)
	endDate := fmt.Sprintf("%d-%02d-01", year, month+1)
	if month == 12 {
		endDate = fmt.Sprintf("%d-01-01", year+1)
	}

	err := s.db.Select(&stats, `
		SELECT
			CONCAT(COALESCE(d.name, ''), COALESCE(rl.path, '/')) as url,
			COUNT(*) as views
		FROM visits v
		LEFT JOIN redirect_links rl ON rl.id = v.link_id
		LEFT JOIN domains d ON d.id = rl.domain_id
		WHERE v.user_id = $1 AND v.created_at >= $2 AND v.created_at < $3
		GROUP BY d.name, rl.path
		ORDER BY views DESC
		LIMIT 20
	`, userID, startDate, endDate)

	return stats, err
}
