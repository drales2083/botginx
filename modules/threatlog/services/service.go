package services

import (
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
)

type ThreatLogService struct {
	db *sqlx.DB
}

func NewThreatLogService(db *sqlx.DB) *ThreatLogService {
	return &ThreatLogService{db: db}
}

type ThreatEntry struct {
	ID        string    `db:"id" json:"id"`
	Timestamp time.Time `db:"created_at" json:"timestamp"`
	IP        string    `db:"ip" json:"ip"`
	Country   string    `db:"country" json:"country"`
	BotType   string    `db:"bot_type" json:"botType"`
	UserAgent string    `db:"user_agent" json:"userAgent"`
	Path      string    `db:"path" json:"path"`
	Domain    string    `db:"domain" json:"domain"`
	Device    string    `db:"device" json:"device"`
	Browser   string    `db:"browser" json:"browser"`
	OS        string    `db:"os" json:"os"`
	Referrer  string    `db:"referrer" json:"referrer"`
}

type ThreatStats struct {
	TotalThreats   int            `json:"totalThreats"`
	Last24h        int            `json:"last24h"`
	LastHour       int            `json:"lastHour"`
	TopCountries   []CountryStat  `json:"topCountries"`
	TopBotTypes    []BotTypeStat  `json:"topBotTypes"`
	HourlyTrend    []HourlyStat   `json:"hourlyTrend"`
	ThreatsByDay   []DailyStat    `json:"threatsByDay"`
}

type CountryStat struct {
	Country string `db:"country" json:"country"`
	Count   int    `db:"count" json:"count"`
}

type BotTypeStat struct {
	BotType string `db:"bot_type" json:"botType"`
	Count   int    `db:"count" json:"count"`
}

type HourlyStat struct {
	Hour  int `db:"hour" json:"hour"`
	Count int `db:"count" json:"count"`
}

type DailyStat struct {
	Date  string `db:"date" json:"date"`
	Count int    `db:"count" json:"count"`
}

func (s *ThreatLogService) GetThreats(userID string, limit int, offset int, filters map[string]string) ([]ThreatEntry, int, error) {
	var threats []ThreatEntry
	var total int

	baseQuery := `
		FROM visits v
		LEFT JOIN redirect_links rl ON rl.id = v.link_id
		LEFT JOIN domains d ON d.id = rl.domain_id
		WHERE v.user_id = $1 AND v.is_bot = true
	`

	args := []interface{}{userID}
	argIdx := 2

	if country := filters["country"]; country != "" {
		baseQuery += ` AND v.country = $` + itoa(argIdx)
		args = append(args, country)
		argIdx++
	}

	if botType := filters["botType"]; botType != "" {
		baseQuery += ` AND v.bot_type = $` + itoa(argIdx)
		args = append(args, botType)
		argIdx++
	}

	if dateFrom := filters["dateFrom"]; dateFrom != "" {
		baseQuery += ` AND v.created_at >= $` + itoa(argIdx)
		args = append(args, dateFrom)
		argIdx++
	}

	if dateTo := filters["dateTo"]; dateTo != "" {
		baseQuery += ` AND v.created_at <= $` + itoa(argIdx)
		args = append(args, dateTo)
		argIdx++
	}

	// Get total count
	countQuery := `SELECT COUNT(*) ` + baseQuery
	s.db.Get(&total, countQuery, args...)

	// Get threats
	selectQuery := `
		SELECT
			v.id,
			v.created_at,
			v.ip,
			COALESCE(v.country, 'Unknown') as country,
			COALESCE(v.bot_type, 'Unknown Bot') as bot_type,
			COALESCE(v.user_agent, '') as user_agent,
			COALESCE(rl.path, '/') as path,
			COALESCE(d.name, '') as domain,
			COALESCE(v.device, '') as device,
			COALESCE(v.browser, '') as browser,
			COALESCE(v.os, '') as os,
			COALESCE(v.referrer, '') as referrer
	` + baseQuery + `
		ORDER BY v.created_at DESC
		LIMIT $` + itoa(argIdx) + ` OFFSET $` + itoa(argIdx+1)

	args = append(args, limit, offset)

	err := s.db.Select(&threats, selectQuery, args...)
	if err != nil {
		return nil, 0, err
	}

	return threats, total, nil
}

func (s *ThreatLogService) GetStats(userID string) (*ThreatStats, error) {
	stats := &ThreatStats{}

	// Total threats
	s.db.Get(&stats.TotalThreats, `
		SELECT COUNT(*) FROM visits WHERE user_id = $1 AND is_bot = true
	`, userID)

	// Last 24 hours
	s.db.Get(&stats.Last24h, `
		SELECT COUNT(*) FROM visits
		WHERE user_id = $1 AND is_bot = true AND created_at > NOW() - INTERVAL '24 hours'
	`, userID)

	// Last hour
	s.db.Get(&stats.LastHour, `
		SELECT COUNT(*) FROM visits
		WHERE user_id = $1 AND is_bot = true AND created_at > NOW() - INTERVAL '1 hour'
	`, userID)

	// Top countries
	s.db.Select(&stats.TopCountries, `
		SELECT COALESCE(NULLIF(country, ''), 'Unknown') as country, COUNT(*) as count
		FROM visits WHERE user_id = $1 AND is_bot = true
		GROUP BY country ORDER BY count DESC LIMIT 10
	`, userID)

	// Top bot types
	s.db.Select(&stats.TopBotTypes, `
		SELECT COALESCE(NULLIF(bot_type, ''), 'Unknown') as bot_type, COUNT(*) as count
		FROM visits WHERE user_id = $1 AND is_bot = true
		GROUP BY bot_type ORDER BY count DESC LIMIT 10
	`, userID)

	// Hourly trend (last 24 hours)
	s.db.Select(&stats.HourlyTrend, `
		SELECT EXTRACT(HOUR FROM created_at)::int as hour, COUNT(*) as count
		FROM visits
		WHERE user_id = $1 AND is_bot = true AND created_at > NOW() - INTERVAL '24 hours'
		GROUP BY hour ORDER BY hour
	`, userID)

	// Daily threats (last 7 days)
	s.db.Select(&stats.ThreatsByDay, `
		SELECT TO_CHAR(created_at, 'YYYY-MM-DD') as date, COUNT(*) as count
		FROM visits
		WHERE user_id = $1 AND is_bot = true AND created_at > NOW() - INTERVAL '7 days'
		GROUP BY date ORDER BY date
	`, userID)

	return stats, nil
}

func (s *ThreatLogService) GetRecentThreats(userID string, limit int) ([]ThreatEntry, error) {
	var threats []ThreatEntry

	err := s.db.Select(&threats, `
		SELECT
			v.id,
			v.created_at,
			v.ip,
			COALESCE(v.country, 'Unknown') as country,
			COALESCE(v.bot_type, 'Unknown Bot') as bot_type,
			COALESCE(v.user_agent, '') as user_agent,
			COALESCE(rl.path, '/') as path,
			COALESCE(d.name, '') as domain,
			COALESCE(v.device, '') as device,
			COALESCE(v.browser, '') as browser,
			COALESCE(v.os, '') as os,
			COALESCE(v.referrer, '') as referrer
		FROM visits v
		LEFT JOIN redirect_links rl ON rl.id = v.link_id
		LEFT JOIN domains d ON d.id = rl.domain_id
		WHERE v.user_id = $1 AND v.is_bot = true
		ORDER BY v.created_at DESC
		LIMIT $2
	`, userID, limit)

	return threats, err
}

func (s *ThreatLogService) GetCountries(userID string) ([]string, error) {
	var countries []string
	err := s.db.Select(&countries, `
		SELECT DISTINCT COALESCE(NULLIF(country, ''), 'Unknown') as country
		FROM visits WHERE user_id = $1 AND is_bot = true
		ORDER BY country
	`, userID)
	return countries, err
}

func (s *ThreatLogService) GetBotTypes(userID string) ([]string, error) {
	var types []string
	err := s.db.Select(&types, `
		SELECT DISTINCT COALESCE(NULLIF(bot_type, ''), 'Unknown') as bot_type
		FROM visits WHERE user_id = $1 AND is_bot = true
		ORDER BY bot_type
	`, userID)
	return types, err
}

func itoa(i int) string {
	return strconv.Itoa(i)
}
