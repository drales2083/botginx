package services

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

type StatsService struct {
	db *sqlx.DB
}

func NewStatsService(db *sqlx.DB) *StatsService {
	return &StatsService{db: db}
}

type Summary struct {
	TotalRequests   int       `json:"totalRequests"`
	TotalPages      int       `json:"totalPages"`
	UniqueVisitors  int       `json:"uniqueVisitors"`
	BotRequests     int       `json:"botRequests"`
	HumanRequests   int       `json:"humanRequests"`
	DataTransferred int64     `json:"dataTransferred"`
	Period          string    `json:"period"`
	GeneratedAt     time.Time `json:"generatedAt"`
}

type HourlyStat struct {
	Hour     int `db:"hour" json:"hour"`
	Requests int `db:"requests" json:"requests"`
	Pages    int `db:"pages" json:"pages"`
}

type DailyStat struct {
	Date     string `db:"date" json:"date"`
	Requests int    `db:"requests" json:"requests"`
	Pages    int    `db:"pages" json:"pages"`
}

type BrowserStat struct {
	Browser  string  `db:"browser" json:"browser"`
	Requests int     `db:"requests" json:"requests"`
	Percent  float64 `json:"percent"`
}

type OSStat struct {
	OS       string  `db:"os" json:"os"`
	Requests int     `db:"requests" json:"requests"`
	Percent  float64 `json:"percent"`
}

type CountryStat struct {
	Country  string  `db:"country" json:"country"`
	Requests int     `db:"requests" json:"requests"`
	Pages    int     `db:"pages" json:"pages"`
	Percent  float64 `json:"percent"`
}

type PageStat struct {
	Path     string `db:"path" json:"path"`
	Domain   string `db:"domain" json:"domain"`
	Requests int    `db:"requests" json:"requests"`
}

func (s *StatsService) GetSummary(userID string, days int) (*Summary, error) {
	summary := &Summary{
		GeneratedAt: time.Now(),
		Period:      "Last " + string(rune(days+'0')) + " days",
	}

	if days == 7 {
		summary.Period = "Last 7 days"
	} else if days == 30 {
		summary.Period = "Last 30 days"
	} else if days == 1 {
		summary.Period = "Today"
	}

	interval := "NOW() - INTERVAL '" + string(rune(days+'0')) + " days'"
	if days == 1 {
		interval = "NOW()::date"
	} else if days == 7 {
		interval = "NOW() - INTERVAL '7 days'"
	} else if days == 30 {
		interval = "NOW() - INTERVAL '30 days'"
	}

	// Total requests
	s.db.Get(&summary.TotalRequests, `
		SELECT COUNT(*) FROM visits
		WHERE user_id = $1 AND created_at > `+interval, userID)

	// Unique visitors (by IP)
	s.db.Get(&summary.UniqueVisitors, `
		SELECT COUNT(DISTINCT ip) FROM visits
		WHERE user_id = $1 AND created_at > `+interval, userID)

	// Bot vs Human
	s.db.Get(&summary.BotRequests, `
		SELECT COUNT(*) FROM visits
		WHERE user_id = $1 AND created_at > `+interval+` AND is_bot = true`, userID)

	summary.HumanRequests = summary.TotalRequests - summary.BotRequests
	summary.TotalPages = summary.TotalRequests // Simplified - each visit is a page view

	return summary, nil
}

func (s *StatsService) GetHourlyStats(userID string, days int) ([]HourlyStat, error) {
	var stats []HourlyStat

	interval := "7 days"
	if days == 1 {
		interval = "1 day"
	} else if days == 30 {
		interval = "30 days"
	}

	err := s.db.Select(&stats, `
		SELECT
			EXTRACT(HOUR FROM created_at)::int as hour,
			COUNT(*) as requests,
			COUNT(*) as pages
		FROM visits
		WHERE user_id = $1 AND created_at > NOW() - INTERVAL '`+interval+`'
		GROUP BY EXTRACT(HOUR FROM created_at)
		ORDER BY hour
	`, userID)

	// Fill in missing hours with zeros
	hourMap := make(map[int]HourlyStat)
	for _, s := range stats {
		hourMap[s.Hour] = s
	}

	result := make([]HourlyStat, 24)
	for i := 0; i < 24; i++ {
		if stat, ok := hourMap[i]; ok {
			result[i] = stat
		} else {
			result[i] = HourlyStat{Hour: i, Requests: 0, Pages: 0}
		}
	}

	return result, err
}

func (s *StatsService) GetDailyStats(userID string, days int) ([]DailyStat, error) {
	var stats []DailyStat

	err := s.db.Select(&stats, `
		SELECT
			TO_CHAR(created_at::date, 'YYYY-MM-DD') as date,
			COUNT(*) as requests,
			COUNT(*) as pages
		FROM visits
		WHERE user_id = $1 AND created_at > NOW() - $2::interval
		GROUP BY created_at::date
		ORDER BY date DESC
		LIMIT $3
	`, userID, fmt.Sprintf("%d days", days), days)

	return stats, err
}

func (s *StatsService) GetBrowserStats(userID string, days int) ([]BrowserStat, error) {
	var stats []BrowserStat

	interval := "7 days"
	if days == 30 {
		interval = "30 days"
	}

	err := s.db.Select(&stats, `
		SELECT
			COALESCE(NULLIF(browser, ''), 'Unknown') as browser,
			COUNT(*) as requests
		FROM visits
		WHERE user_id = $1 AND created_at > NOW() - INTERVAL '`+interval+`'
		GROUP BY browser
		ORDER BY requests DESC
		LIMIT 10
	`, userID)

	// Calculate percentages
	var total int
	for _, s := range stats {
		total += s.Requests
	}
	if total > 0 {
		for i := range stats {
			stats[i].Percent = float64(stats[i].Requests) * 100 / float64(total)
		}
	}

	return stats, err
}

func (s *StatsService) GetOSStats(userID string, days int) ([]OSStat, error) {
	var stats []OSStat

	interval := "7 days"
	if days == 30 {
		interval = "30 days"
	}

	err := s.db.Select(&stats, `
		SELECT
			COALESCE(NULLIF(os, ''), 'Unknown') as os,
			COUNT(*) as requests
		FROM visits
		WHERE user_id = $1 AND created_at > NOW() - INTERVAL '`+interval+`'
		GROUP BY os
		ORDER BY requests DESC
		LIMIT 10
	`, userID)

	// Calculate percentages
	var total int
	for _, s := range stats {
		total += s.Requests
	}
	if total > 0 {
		for i := range stats {
			stats[i].Percent = float64(stats[i].Requests) * 100 / float64(total)
		}
	}

	return stats, err
}

func (s *StatsService) GetCountryStats(userID string, days int) ([]CountryStat, error) {
	var stats []CountryStat

	interval := "7 days"
	if days == 30 {
		interval = "30 days"
	}

	err := s.db.Select(&stats, `
		SELECT
			COALESCE(NULLIF(country, ''), 'Unknown') as country,
			COUNT(*) as requests,
			COUNT(*) as pages
		FROM visits
		WHERE user_id = $1 AND created_at > NOW() - INTERVAL '`+interval+`'
		GROUP BY country
		ORDER BY requests DESC
		LIMIT 20
	`, userID)

	// Calculate percentages
	var total int
	for _, s := range stats {
		total += s.Requests
	}
	if total > 0 {
		for i := range stats {
			stats[i].Percent = float64(stats[i].Requests) * 100 / float64(total)
		}
	}

	return stats, err
}

func (s *StatsService) GetPageStats(userID string, days int) ([]PageStat, error) {
	var stats []PageStat

	interval := "7 days"
	if days == 30 {
		interval = "30 days"
	}

	err := s.db.Select(&stats, `
		SELECT
			COALESCE(rl.path, '/') as path,
			COALESCE(d.name, 'Unknown') as domain,
			COUNT(*) as requests
		FROM visits v
		LEFT JOIN redirect_links rl ON rl.id = v.link_id
		LEFT JOIN domains d ON d.id = rl.domain_id
		WHERE v.user_id = $1 AND v.created_at > NOW() - INTERVAL '`+interval+`'
		GROUP BY rl.path, d.name
		ORDER BY requests DESC
		LIMIT 20
	`, userID)

	return stats, err
}
