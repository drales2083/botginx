package services

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/botginx/botginx/modules/analytics/models"
	"github.com/jmoiron/sqlx"
)

type AnalyticsService struct {
	db *sqlx.DB
}

func NewAnalyticsService(db *sqlx.DB) *AnalyticsService {
	return &AnalyticsService{db: db}
}

func (s *AnalyticsService) generateID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// RecordVisit stores a new visit with all analytics fields
func (s *AnalyticsService) RecordVisit(visit *models.Visit) error {
	visit.ID = s.generateID()
	visit.CreatedAt = time.Now()

	// Check if unique (same IP + link in last 24h)
	var count int
	s.db.Get(&count, `
		SELECT COUNT(*) FROM visits
		WHERE link_id = $1 AND ip = $2 AND created_at > NOW() - INTERVAL '24 hours'
	`, visit.LinkID, visit.IP)
	visit.IsUnique = count == 0

	_, err := s.db.NamedExec(`
		INSERT INTO visits (
			id, link_id, user_id, session_id, ip, country, city, latitude, longitude,
			asn, asn_org, device, browser, os, referrer, referrer_domain, user_agent,
			language, timezone, screen_resolution,
			utm_source, utm_medium, utm_campaign, utm_term, utm_content,
			is_bot, bot_type, bot_score, bot_module, behavior_score, automation_tool,
			is_headless, is_tor, is_proxy, is_datacenter, cookies_enabled, js_enabled, fingerprint,
			action, is_unique, blocked, block_reason, duration_us, created_at
		) VALUES (
			:id, :link_id, :user_id, :session_id, :ip, :country, :city, :latitude, :longitude,
			:asn, :asn_org, :device, :browser, :os, :referrer, :referrer_domain, :user_agent,
			:language, :timezone, :screen_resolution,
			:utm_source, :utm_medium, :utm_campaign, :utm_term, :utm_content,
			:is_bot, :bot_type, :bot_score, :bot_module, :behavior_score, :automation_tool,
			:is_headless, :is_tor, :is_proxy, :is_datacenter, :cookies_enabled, :js_enabled, :fingerprint,
			:action, :is_unique, :blocked, :block_reason, :duration_us, :created_at
		)
	`, visit)
	return err
}

// RecordSession stores a new session
func (s *AnalyticsService) RecordSession(session *models.Session) error {
	session.ID = s.generateID()
	if session.StartedAt.IsZero() {
		session.StartedAt = time.Now()
	}

	_, err := s.db.NamedExec(`
		INSERT INTO visitor_sessions (
			id, session_id, link_id, user_id, ip, country, user_agent, fingerprint,
			started_at, referrer_domain, utm_source, utm_medium, utm_campaign
		) VALUES (
			:id, :session_id, :link_id, :user_id, :ip, :country, :user_agent, :fingerprint,
			:started_at, :referrer_domain, :utm_source, :utm_medium, :utm_campaign
		) ON CONFLICT (session_id) DO NOTHING
	`, session)
	return err
}

// UpdateSession updates session end data
func (s *AnalyticsService) UpdateSession(sessionID string, duration, pageViews int) error {
	_, err := s.db.Exec(`
		UPDATE visitor_sessions SET page_views = $2, ended_at = NOW(), duration = $3
		WHERE session_id = $1
	`, sessionID, pageViews, duration)
	return err
}

// IncrementPageViews adds one to the page view count for a session
func (s *AnalyticsService) IncrementPageViews(sessionID string) error {
	_, err := s.db.Exec(`
		UPDATE visitor_sessions SET page_views = page_views + 1
		WHERE session_id = $1
	`, sessionID)
	return err
}

// RecordConversion stores a conversion event
func (s *AnalyticsService) RecordConversion(conv *models.Conversion) error {
	conv.ID = s.generateID()
	conv.CreatedAt = time.Now()

	_, err := s.db.NamedExec(`
		INSERT INTO conversions (
			id, session_id, link_id, user_id, ip, country, path, time_to_convert,
			challenge_type, behavior_score, trust_token_solves,
			referrer_domain, utm_source, utm_medium, utm_campaign, created_at
		) VALUES (
			:id, :session_id, :link_id, :user_id, :ip, :country, :path, :time_to_convert,
			:challenge_type, :behavior_score, :trust_token_solves,
			:referrer_domain, :utm_source, :utm_medium, :utm_campaign, :created_at
		)
	`, conv)
	return err
}

// GetLinkStats returns comprehensive stats for a link
func (s *AnalyticsService) GetLinkStats(linkID string) (*models.LinkStats, error) {
	stats := &models.LinkStats{LinkID: linkID}

	s.db.Get(&stats.TotalVisits, `SELECT COUNT(*) FROM visits WHERE link_id = $1`, linkID)
	s.db.Get(&stats.UniqueVisits, `SELECT COUNT(DISTINCT ip) FROM visits WHERE link_id = $1`, linkID)
	s.db.Get(&stats.BotVisits, `SELECT COUNT(*) FROM visits WHERE link_id = $1 AND is_bot = true`, linkID)
	s.db.Get(&stats.BlockedVisits, `SELECT COUNT(*) FROM visits WHERE link_id = $1 AND blocked = true`, linkID)
	s.db.Get(&stats.TodayVisits, `SELECT COUNT(*) FROM visits WHERE link_id = $1 AND created_at > CURRENT_DATE`, linkID)
	s.db.Get(&stats.Conversions, `SELECT COUNT(*) FROM conversions WHERE link_id = $1`, linkID)
	s.db.Get(&stats.TorVisits, `SELECT COUNT(*) FROM visits WHERE link_id = $1 AND is_tor = true`, linkID)
	s.db.Get(&stats.ProxyVisits, `SELECT COUNT(*) FROM visits WHERE link_id = $1 AND is_proxy = true`, linkID)
	s.db.Get(&stats.AvgBehaviorScore, `SELECT COALESCE(AVG(behavior_score), 0) FROM visits WHERE link_id = $1`, linkID)

	if stats.TotalVisits > 0 {
		stats.ConversionRate = float64(stats.Conversions) / float64(stats.TotalVisits) * 100
	}

	return stats, nil
}

// GetThreatStats returns threat detection stats
func (s *AnalyticsService) GetThreatStats(linkID string) (*models.ThreatStats, error) {
	stats := &models.ThreatStats{}

	s.db.Get(&stats.TorCount, `SELECT COUNT(*) FROM visits WHERE link_id = $1 AND is_tor = true`, linkID)
	s.db.Get(&stats.ProxyCount, `SELECT COUNT(*) FROM visits WHERE link_id = $1 AND is_proxy = true`, linkID)
	s.db.Get(&stats.DatacenterCount, `SELECT COUNT(*) FROM visits WHERE link_id = $1 AND is_datacenter = true`, linkID)
	s.db.Get(&stats.HeadlessCount, `SELECT COUNT(*) FROM visits WHERE link_id = $1 AND is_headless = true`, linkID)
	s.db.Get(&stats.AutomationCount, `SELECT COUNT(*) FROM visits WHERE link_id = $1 AND automation_tool != ''`, linkID)
	s.db.Get(&stats.LowBehaviorCount, `SELECT COUNT(*) FROM visits WHERE link_id = $1 AND behavior_score < 40`, linkID)

	return stats, nil
}

// GetUserStats returns aggregate stats for all user's links
func (s *AnalyticsService) GetUserStats(userID string) (*models.LinkStats, error) {
	stats := &models.LinkStats{}

	s.db.Get(&stats.TotalVisits, `SELECT COUNT(*) FROM visits WHERE user_id = $1`, userID)
	s.db.Get(&stats.UniqueVisits, `SELECT COUNT(DISTINCT ip) FROM visits WHERE user_id = $1`, userID)
	s.db.Get(&stats.BotVisits, `SELECT COUNT(*) FROM visits WHERE user_id = $1 AND is_bot = true`, userID)
	s.db.Get(&stats.BlockedVisits, `SELECT COUNT(*) FROM visits WHERE user_id = $1 AND blocked = true`, userID)
	s.db.Get(&stats.TodayVisits, `SELECT COUNT(*) FROM visits WHERE user_id = $1 AND created_at > CURRENT_DATE`, userID)
	s.db.Get(&stats.Conversions, `SELECT COUNT(*) FROM conversions WHERE user_id = $1`, userID)

	if stats.TotalVisits > 0 {
		stats.ConversionRate = float64(stats.Conversions) / float64(stats.TotalVisits) * 100
	}

	return stats, nil
}

// GetCountryStats returns visits by country
func (s *AnalyticsService) GetCountryStats(linkID string, limit int) ([]models.CountryStats, error) {
	var stats []models.CountryStats
	err := s.db.Select(&stats, `
		SELECT country, COUNT(*) as count
		FROM visits
		WHERE link_id = $1 AND country != ''
		GROUP BY country
		ORDER BY count DESC
		LIMIT $2
	`, linkID, limit)
	return stats, err
}

// GetDeviceStats returns visits by device type
func (s *AnalyticsService) GetDeviceStats(linkID string) ([]models.DeviceStats, error) {
	var stats []models.DeviceStats
	err := s.db.Select(&stats, `
		SELECT device, COUNT(*) as count
		FROM visits
		WHERE link_id = $1
		GROUP BY device
		ORDER BY count DESC
	`, linkID)
	return stats, err
}

// GetBrowserStats returns visits by browser
func (s *AnalyticsService) GetBrowserStats(linkID string, limit int) ([]models.BrowserStats, error) {
	var stats []models.BrowserStats
	err := s.db.Select(&stats, `
		SELECT browser, COUNT(*) as count
		FROM visits
		WHERE link_id = $1 AND browser != ''
		GROUP BY browser
		ORDER BY count DESC
		LIMIT $2
	`, linkID, limit)
	return stats, err
}

// GetOSStats returns visits by OS
func (s *AnalyticsService) GetOSStats(linkID string, limit int) ([]models.OSStats, error) {
	var stats []models.OSStats
	err := s.db.Select(&stats, `
		SELECT os, COUNT(*) as count
		FROM visits
		WHERE link_id = $1 AND os != ''
		GROUP BY os
		ORDER BY count DESC
		LIMIT $2
	`, linkID, limit)
	return stats, err
}

// GetReferrerStats returns visits by referrer
func (s *AnalyticsService) GetReferrerStats(linkID string, limit int) ([]models.ReferrerStats, error) {
	var stats []models.ReferrerStats
	err := s.db.Select(&stats, `
		SELECT COALESCE(NULLIF(referrer_domain, ''), 'Direct') as referrer, COUNT(*) as count
		FROM visits
		WHERE link_id = $1
		GROUP BY referrer_domain
		ORDER BY count DESC
		LIMIT $2
	`, linkID, limit)
	return stats, err
}

// GetUTMStats returns visits by UTM source/medium/campaign
func (s *AnalyticsService) GetUTMStats(linkID string, limit int) ([]models.UTMStats, error) {
	var stats []models.UTMStats
	err := s.db.Select(&stats, `
		SELECT utm_source, utm_medium, utm_campaign, COUNT(*) as count
		FROM visits
		WHERE link_id = $1 AND utm_source != ''
		GROUP BY utm_source, utm_medium, utm_campaign
		ORDER BY count DESC
		LIMIT $2
	`, linkID, limit)
	return stats, err
}

// GetBehaviorScoreDistribution returns distribution of behavior scores
func (s *AnalyticsService) GetBehaviorScoreDistribution(linkID string) ([]models.BehaviorScoreDistribution, error) {
	var stats []models.BehaviorScoreDistribution
	err := s.db.Select(&stats, `
		SELECT
			CASE
				WHEN behavior_score >= 90 THEN '90-100 (Very Human)'
				WHEN behavior_score >= 70 THEN '70-89 (Likely Human)'
				WHEN behavior_score >= 40 THEN '40-69 (Uncertain)'
				WHEN behavior_score >= 20 THEN '20-39 (Suspicious)'
				ELSE '0-19 (Likely Bot)'
			END as score_range,
			COUNT(*) as count
		FROM visits
		WHERE link_id = $1
		GROUP BY score_range
		ORDER BY MIN(behavior_score) DESC
	`, linkID)
	return stats, err
}

// GetAutomationStats returns detected automation tools
func (s *AnalyticsService) GetAutomationStats(linkID string) ([]models.AutomationStats, error) {
	var stats []models.AutomationStats
	err := s.db.Select(&stats, `
		SELECT automation_tool, COUNT(*) as count
		FROM visits
		WHERE link_id = $1 AND automation_tool != ''
		GROUP BY automation_tool
		ORDER BY count DESC
	`, linkID)
	return stats, err
}

// GetTimeline returns visits over time
func (s *AnalyticsService) GetTimeline(linkID string, period string) ([]models.TimelinePoint, error) {
	var query string
	switch period {
	case "hourly":
		query = `
			SELECT TO_CHAR(created_at, 'YYYY-MM-DD HH24:00') as time_bucket, COUNT(*) as count
			FROM visits
			WHERE link_id = $1 AND created_at > NOW() - INTERVAL '24 hours'
			GROUP BY time_bucket
			ORDER BY time_bucket
		`
	case "daily":
		query = `
			SELECT TO_CHAR(created_at, 'YYYY-MM-DD') as time_bucket, COUNT(*) as count
			FROM visits
			WHERE link_id = $1 AND created_at > NOW() - INTERVAL '30 days'
			GROUP BY time_bucket
			ORDER BY time_bucket
		`
	case "monthly":
		query = `
			SELECT TO_CHAR(created_at, 'YYYY-MM') as time_bucket, COUNT(*) as count
			FROM visits
			WHERE link_id = $1 AND created_at > NOW() - INTERVAL '12 months'
			GROUP BY time_bucket
			ORDER BY time_bucket
		`
	default:
		query = `
			SELECT TO_CHAR(created_at, 'YYYY-MM-DD') as time_bucket, COUNT(*) as count
			FROM visits
			WHERE link_id = $1 AND created_at > NOW() - INTERVAL '7 days'
			GROUP BY time_bucket
			ORDER BY time_bucket
		`
	}

	var timeline []models.TimelinePoint
	err := s.db.Select(&timeline, query, linkID)
	return timeline, err
}

// GetUserTimeline returns aggregate timeline for all user's links
func (s *AnalyticsService) GetUserTimeline(userID string, period string) ([]models.TimelinePoint, error) {
	var query string
	switch period {
	case "hourly":
		query = `
			SELECT TO_CHAR(created_at, 'YYYY-MM-DD HH24:00') as time_bucket, COUNT(*) as count
			FROM visits
			WHERE user_id = $1 AND created_at > NOW() - INTERVAL '24 hours'
			GROUP BY time_bucket
			ORDER BY time_bucket
		`
	case "daily":
		query = `
			SELECT TO_CHAR(created_at, 'YYYY-MM-DD') as time_bucket, COUNT(*) as count
			FROM visits
			WHERE user_id = $1 AND created_at > NOW() - INTERVAL '30 days'
			GROUP BY time_bucket
			ORDER BY time_bucket
		`
	default:
		query = `
			SELECT TO_CHAR(created_at, 'YYYY-MM-DD') as time_bucket, COUNT(*) as count
			FROM visits
			WHERE user_id = $1 AND created_at > NOW() - INTERVAL '7 days'
			GROUP BY time_bucket
			ORDER BY time_bucket
		`
	}

	var timeline []models.TimelinePoint
	err := s.db.Select(&timeline, query, userID)
	return timeline, err
}

// GetRecentVisits returns recent visits for a link
func (s *AnalyticsService) GetRecentVisits(linkID string, limit int) ([]models.Visit, error) {
	var visits []models.Visit
	err := s.db.Select(&visits, `
		SELECT * FROM visits
		WHERE link_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, linkID, limit)
	return visits, err
}

// GetSessions returns sessions for a link
func (s *AnalyticsService) GetSessions(linkID string, limit int) ([]models.Session, error) {
	var sessions []models.Session
	err := s.db.Select(&sessions, `
		SELECT * FROM visitor_sessions
		WHERE link_id = $1
		ORDER BY started_at DESC
		LIMIT $2
	`, linkID, limit)
	return sessions, err
}

// GetConversions returns conversions for a link
func (s *AnalyticsService) GetConversions(linkID string, limit int) ([]models.Conversion, error) {
	var conversions []models.Conversion
	err := s.db.Select(&conversions, `
		SELECT * FROM conversions
		WHERE link_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, linkID, limit)
	return conversions, err
}

// Link Settings

func (s *AnalyticsService) GetLinkSettings(linkID string) (*models.LinkSettings, error) {
	var settings models.LinkSettings
	err := s.db.Get(&settings, `SELECT * FROM link_settings WHERE link_id = $1`, linkID)
	if err != nil {
		// Default settings: challenge over block (Tor/VPN/Datacenter see challenge, not block)
		return &models.LinkSettings{
			LinkID:           linkID,
			Template:         "cloudflare",
			ThemeMode:        "light",
			CountryMode:      "",
			CountryList:      []string{},
			ASNMode:          "",
			ASNList:          []string{},
			DeviceMode:       "",
			DeviceList:       []string{},
			BlockBots:        true,
			BlockTor:         false,
			BlockProxy:       false,
			BlockDatacenter:  false,
			BlockHeadless:    false,
			MinBehaviorScore: 0,
			RedirectOnBlock:  "",
		}, nil
	}

	json.Unmarshal([]byte(settings.CountryListRaw), &settings.CountryList)
	json.Unmarshal([]byte(settings.ASNListRaw), &settings.ASNList)
	json.Unmarshal([]byte(settings.DeviceListRaw), &settings.DeviceList)

	return &settings, nil
}

func (s *AnalyticsService) SaveLinkSettings(settings *models.LinkSettings) error {
	countryJSON, _ := json.Marshal(settings.CountryList)
	asnJSON, _ := json.Marshal(settings.ASNList)
	deviceJSON, _ := json.Marshal(settings.DeviceList)
	settings.CountryListRaw = string(countryJSON)
	settings.ASNListRaw = string(asnJSON)
	settings.DeviceListRaw = string(deviceJSON)
	settings.UpdatedAt = time.Now()

	if settings.ID == "" {
		settings.ID = s.generateID()
		_, err := s.db.Exec(`
			INSERT INTO link_settings (id, link_id, country_mode, country_list, asn_mode, asn_list,
				device_mode, device_list, block_bots, block_tor, block_proxy, block_datacenter,
				block_headless, min_behavior_score, redirect_on_block, template, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		`, settings.ID, settings.LinkID, settings.CountryMode, settings.CountryListRaw,
			settings.ASNMode, settings.ASNListRaw, settings.DeviceMode, settings.DeviceListRaw,
			settings.BlockBots, settings.BlockTor, settings.BlockProxy, settings.BlockDatacenter,
			settings.BlockHeadless, settings.MinBehaviorScore, settings.RedirectOnBlock,
			settings.Template, settings.UpdatedAt)
		return err
	}

	_, err := s.db.Exec(`
		UPDATE link_settings
		SET country_mode = $2, country_list = $3, asn_mode = $4, asn_list = $5,
			device_mode = $6, device_list = $7, block_bots = $8, block_tor = $9,
			block_proxy = $10, block_datacenter = $11, block_headless = $12,
			min_behavior_score = $13, redirect_on_block = $14, template = $15, updated_at = $16
		WHERE link_id = $1
	`, settings.LinkID, settings.CountryMode, settings.CountryListRaw,
		settings.ASNMode, settings.ASNListRaw, settings.DeviceMode, settings.DeviceListRaw,
		settings.BlockBots, settings.BlockTor, settings.BlockProxy, settings.BlockDatacenter,
		settings.BlockHeadless, settings.MinBehaviorScore, settings.RedirectOnBlock,
		settings.Template, settings.UpdatedAt)
	return err
}

// ShouldBlock checks if a visit should be blocked based on settings
func (s *AnalyticsService) ShouldBlock(linkID, country, device string, isBot, isTor, isProxy, isDatacenter, isHeadless bool, behaviorScore int) (bool, string) {
	settings, _ := s.GetLinkSettings(linkID)

	if settings.BlockBots && isBot {
		return true, "bot_blocked"
	}
	if settings.BlockTor && isTor {
		return true, "tor_blocked"
	}
	if settings.BlockProxy && isProxy {
		return true, "proxy_blocked"
	}
	if settings.BlockDatacenter && isDatacenter {
		return true, "datacenter_blocked"
	}
	if settings.BlockHeadless && isHeadless {
		return true, "headless_blocked"
	}
	// Only check behavior score if we actually have one (> 0 means data was provided)
	if settings.MinBehaviorScore > 0 && behaviorScore > 0 && behaviorScore < settings.MinBehaviorScore {
		return true, "low_behavior_score"
	}

	if settings.CountryMode == "whitelist" && len(settings.CountryList) > 0 && country != "" {
		if !contains(settings.CountryList, strings.ToUpper(country)) {
			return true, "country_not_whitelisted"
		}
	}
	if settings.CountryMode == "blacklist" && len(settings.CountryList) > 0 && country != "" {
		if contains(settings.CountryList, strings.ToUpper(country)) {
			return true, "country_blacklisted"
		}
	}

	if settings.DeviceMode == "whitelist" && len(settings.DeviceList) > 0 {
		if !contains(settings.DeviceList, strings.ToLower(device)) {
			return true, "device_not_whitelisted"
		}
	}
	if settings.DeviceMode == "blacklist" && len(settings.DeviceList) > 0 {
		if contains(settings.DeviceList, strings.ToLower(device)) {
			return true, "device_blacklisted"
		}
	}

	return false, ""
}

// GetTopLinks returns top performing links for a user
func (s *AnalyticsService) GetTopLinks(userID string, limit int) ([]map[string]interface{}, error) {
	rows, err := s.db.Query(`
		SELECT v.link_id, COUNT(*) as total_visits,
			   COUNT(*) FILTER (WHERE v.is_unique = true) as unique_visits,
			   COUNT(*) FILTER (WHERE v.is_bot = true) as bot_visits,
			   COALESCE(AVG(v.behavior_score), 0) as avg_behavior,
			   COALESCE(r.subdomain, '') as subdomain,
			   COALESCE(REGEXP_REPLACE(d.name, '^\*\.', ''), '') as domain,
			   COALESCE(r.path, '') as path
		FROM visits v
		LEFT JOIN redirect_links r ON r.id = v.link_id
		LEFT JOIN domains d ON d.id = r.domain_id
		WHERE v.user_id = $1
		GROUP BY v.link_id, r.subdomain, d.name, r.path
		ORDER BY total_visits DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var linkID, subdomain, domain, path string
		var total, unique, bots int
		var avgBehavior float64
		rows.Scan(&linkID, &total, &unique, &bots, &avgBehavior, &subdomain, &domain, &path)

		// Build display URL
		displayURL := linkID
		if subdomain != "" && domain != "" {
			displayURL = subdomain + "." + domain
			if path != "" {
				displayURL += "/" + path
			}
		}

		results = append(results, map[string]interface{}{
			"linkId":       linkID,
			"displayUrl":   displayURL,
			"totalVisits":  total,
			"uniqueVisits": unique,
			"botVisits":    bots,
			"avgBehavior":  avgBehavior,
		})
	}
	return results, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}

// GetUserCountryStats returns visits by country for all user's links
func (s *AnalyticsService) GetUserCountryStats(userID string, limit int) ([]models.CountryStats, error) {
	var stats []models.CountryStats
	err := s.db.Select(&stats, `
		SELECT country, COUNT(*) as count
		FROM visits
		WHERE user_id = $1 AND country != ''
		GROUP BY country
		ORDER BY count DESC
		LIMIT $2
	`, userID, limit)
	return stats, err
}

// GetUserTimelineMultiSeries returns timeline with total, blocked, and unique visits
func (s *AnalyticsService) GetUserTimelineMultiSeries(userID string, period string) ([]models.TimelineMultiPoint, error) {
	var query string
	switch period {
	case "hourly":
		query = `
			SELECT TO_CHAR(created_at, 'YYYY-MM-DD HH24:00') as time_bucket,
				COUNT(*) as total,
				COUNT(*) FILTER (WHERE blocked = true) as blocked,
				COUNT(*) FILTER (WHERE is_unique = true) as unique_count
			FROM visits
			WHERE user_id = $1 AND created_at > NOW() - INTERVAL '24 hours'
			GROUP BY time_bucket
			ORDER BY time_bucket
		`
	case "daily":
		query = `
			SELECT TO_CHAR(created_at, 'YYYY-MM-DD') as time_bucket,
				COUNT(*) as total,
				COUNT(*) FILTER (WHERE blocked = true) as blocked,
				COUNT(*) FILTER (WHERE is_unique = true) as unique_count
			FROM visits
			WHERE user_id = $1 AND created_at > NOW() - INTERVAL '7 days'
			GROUP BY time_bucket
			ORDER BY time_bucket
		`
	case "monthly":
		query = `
			SELECT TO_CHAR(created_at, 'YYYY-MM') as time_bucket,
				COUNT(*) as total,
				COUNT(*) FILTER (WHERE blocked = true) as blocked,
				COUNT(*) FILTER (WHERE is_unique = true) as unique_count
			FROM visits
			WHERE user_id = $1 AND created_at > NOW() - INTERVAL '12 months'
			GROUP BY time_bucket
			ORDER BY time_bucket
		`
	default:
		query = `
			SELECT TO_CHAR(created_at, 'YYYY-MM-DD HH24:00') as time_bucket,
				COUNT(*) as total,
				COUNT(*) FILTER (WHERE blocked = true) as blocked,
				COUNT(*) FILTER (WHERE is_unique = true) as unique_count
			FROM visits
			WHERE user_id = $1 AND created_at > NOW() - INTERVAL '24 hours'
			GROUP BY time_bucket
			ORDER BY time_bucket
		`
	}

	var timeline []models.TimelineMultiPoint
	err := s.db.Select(&timeline, query, userID)
	return timeline, err
}

// GetUserVisitorPoints returns lat/lng points for map visualization
func (s *AnalyticsService) GetUserVisitorPoints(userID string, limit int) ([]models.VisitorPoint, error) {
	var points []models.VisitorPoint
	err := s.db.Select(&points, `
		SELECT latitude as lat, longitude as lng, country, city,
			created_at as time, blocked
		FROM visits
		WHERE user_id = $1 AND latitude != 0 AND longitude != 0
		ORDER BY created_at DESC
		LIMIT $2
	`, userID, limit)
	return points, err
}

// LinkInfo represents a redirect link for filtering
type LinkInfo struct {
	ID     string `db:"id" json:"id"`
	Domain string `db:"domain" json:"domain"`
	Path   string `db:"path" json:"path"`
}

// GetUserLinks returns a list of user's redirect links for filter dropdown
func (s *AnalyticsService) GetUserLinks(userID string) ([]LinkInfo, error) {
	var links []LinkInfo
	err := s.db.Select(&links, `
		SELECT rl.id, d.name as domain, rl.path
		FROM redirect_links rl
		INNER JOIN domains d ON d.id = rl.domain_id
		WHERE rl.user_id = $1
		ORDER BY d.name, rl.path
	`, userID)
	return links, err
}

// VisitLog represents a single visit log entry
type VisitLog struct {
	ID             string    `db:"id" json:"id"`
	CreatedAt      time.Time `db:"created_at" json:"createdAt"`
	Domain         string    `db:"domain" json:"domain"`
	Path           string    `db:"path" json:"path"`
	IP             string    `db:"ip" json:"ip"`
	Country        string    `db:"country" json:"country"`
	Device         string    `db:"device" json:"device"`
	Browser        string    `db:"browser" json:"browser"`
	OS             string    `db:"os" json:"os"`
	Referrer       string    `db:"referrer" json:"referrer"`
	ReferrerDomain string    `db:"referrer_domain" json:"referrerDomain"`
	IsBot          bool      `db:"is_bot" json:"isBot"`
	BotScore       float64   `db:"bot_score" json:"botScore"`
	Blocked        bool      `db:"blocked" json:"blocked"`
	BlockReason    string    `db:"block_reason" json:"blockReason"`
	UserAgent      string    `db:"user_agent" json:"userAgent"`
	UTMSource      string    `db:"utm_source" json:"utmSource"`
	UTMMedium      string    `db:"utm_medium" json:"utmMedium"`
	UTMCampaign    string    `db:"utm_campaign" json:"utmCampaign"`
}

// GetVisitLogs returns paginated and filtered visit logs
func (s *AnalyticsService) GetVisitLogs(userID, linkID, country, botFilter, search, startDate, endDate string, page, limit int) ([]VisitLog, int, error) {
	args := []interface{}{userID}
	argIdx := 2

	whereClause := "WHERE v.user_id = $1"

	if linkID != "" {
		whereClause += " AND v.link_id = $" + itoa(argIdx)
		args = append(args, linkID)
		argIdx++
	}

	if country != "" {
		whereClause += " AND v.country = $" + itoa(argIdx)
		args = append(args, country)
		argIdx++
	}

	if botFilter == "bots" {
		whereClause += " AND v.is_bot = true"
	} else if botFilter == "humans" {
		whereClause += " AND (v.is_bot = false OR v.is_bot IS NULL)"
	}

	if search != "" {
		whereClause += " AND (v.ip ILIKE $" + itoa(argIdx) + " OR v.referrer ILIKE $" + itoa(argIdx) + " OR v.user_agent ILIKE $" + itoa(argIdx) + ")"
		args = append(args, "%"+search+"%")
		argIdx++
	}

	if startDate != "" {
		whereClause += " AND v.created_at >= $" + itoa(argIdx)
		args = append(args, startDate)
		argIdx++
	}

	if endDate != "" {
		whereClause += " AND v.created_at <= $" + itoa(argIdx) + "::date + INTERVAL '1 day'"
		args = append(args, endDate)
		argIdx++
	}

	// Count total
	var total int
	countQuery := "SELECT COUNT(*) FROM visits v " + whereClause
	if err := s.db.Get(&total, countQuery, args...); err != nil {
		return nil, 0, err
	}

	// Get page
	offset := (page - 1) * limit
	args = append(args, limit, offset)

	query := `
		SELECT
			v.id, v.created_at,
			COALESCE(d.name, '') as domain,
			COALESCE(rl.path, '') as path,
			COALESCE(CONCAT(LEFT(v.ip, POSITION('.' IN v.ip) + 3), 'xxx'), '') as ip,
			COALESCE(v.country, '') as country,
			COALESCE(v.device, 'desktop') as device,
			COALESCE(v.browser, '') as browser,
			COALESCE(v.os, '') as os,
			COALESCE(v.referrer, '') as referrer,
			COALESCE(v.referrer_domain, '') as referrer_domain,
			COALESCE(v.is_bot, false) as is_bot,
			COALESCE(v.bot_score, 0) as bot_score,
			COALESCE(v.blocked, false) as blocked,
			COALESCE(v.block_reason, '') as block_reason,
			COALESCE(v.user_agent, '') as user_agent,
			COALESCE(v.utm_source, '') as utm_source,
			COALESCE(v.utm_medium, '') as utm_medium,
			COALESCE(v.utm_campaign, '') as utm_campaign
		FROM visits v
		LEFT JOIN redirect_links rl ON rl.id = v.link_id
		LEFT JOIN domains d ON d.id = rl.domain_id
		` + whereClause + `
		ORDER BY v.created_at DESC
		LIMIT $` + itoa(argIdx) + ` OFFSET $` + itoa(argIdx+1)

	var logs []VisitLog
	if err := s.db.Select(&logs, query, args...); err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// RealtimeData contains all real-time analytics data
type RealtimeData struct {
	ActiveCount  int              `json:"activeCount"`
	RecentVisits []RecentVisit    `json:"recentVisits"`
	LastHour     int              `json:"lastHour"`
	Today        int              `json:"today"`
	TopCountry   string           `json:"topCountry"`
	TopLink      *TopLinkInfo     `json:"topLink"`
	HumanPercent int              `json:"humanPercent"`
	HumanCount   int              `json:"humanCount"`
	BotCount     int              `json:"botCount"`
	TopCountries []CountryCount   `json:"topCountries"`
	TopLinks     []TopLinkInfo    `json:"topLinks"`
	Timeline     []TimelinePoint  `json:"timeline"`
}

type CountryCount struct {
	Country string `db:"country" json:"country"`
	Count   int    `db:"count" json:"count"`
}

type RecentVisit struct {
	Country   string    `db:"country" json:"country"`
	Device    string    `db:"device" json:"device"`
	Domain    string    `db:"domain" json:"domain"`
	Path      string    `db:"path" json:"path"`
	IsBot     bool      `db:"is_bot" json:"isBot"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
}

type TopLinkInfo struct {
	Domain string `json:"domain"`
	Path   string `json:"path"`
	Count  int    `json:"count"`
}

type TimelinePoint struct {
	Minute string `db:"minute" json:"minute"`
	Count  int    `db:"count" json:"count"`
}

// GetRealtimeData returns all real-time analytics data for dashboard
func (s *AnalyticsService) GetRealtimeData(userID string) (*RealtimeData, error) {
	data := &RealtimeData{}

	// Active count (last 5 minutes)
	s.db.Get(&data.ActiveCount, `
		SELECT COUNT(*) FROM visits
		WHERE user_id = $1 AND created_at > NOW() - INTERVAL '5 minutes'
	`, userID)

	// Last hour count
	s.db.Get(&data.LastHour, `
		SELECT COUNT(*) FROM visits
		WHERE user_id = $1 AND created_at > NOW() - INTERVAL '1 hour'
	`, userID)

	// Today count
	s.db.Get(&data.Today, `
		SELECT COUNT(*) FROM visits
		WHERE user_id = $1 AND created_at > NOW()::date
	`, userID)

	// Recent visits (last 20)
	s.db.Select(&data.RecentVisits, `
		SELECT
			COALESCE(v.country, '') as country,
			COALESCE(v.device, 'desktop') as device,
			COALESCE(d.name, '') as domain,
			COALESCE(rl.path, '') as path,
			COALESCE(v.is_bot, false) as is_bot,
			v.created_at
		FROM visits v
		LEFT JOIN redirect_links rl ON rl.id = v.link_id
		LEFT JOIN domains d ON d.id = rl.domain_id
		WHERE v.user_id = $1
		ORDER BY v.created_at DESC
		LIMIT 20
	`, userID)

	// Top country (last hour)
	s.db.Get(&data.TopCountry, `
		SELECT COALESCE(country, 'Unknown')
		FROM visits
		WHERE user_id = $1 AND created_at > NOW() - INTERVAL '1 hour' AND country != ''
		GROUP BY country
		ORDER BY COUNT(*) DESC
		LIMIT 1
	`, userID)

	// Top link (last hour)
	var topLink struct {
		Domain string `db:"domain"`
		Path   string `db:"path"`
		Count  int    `db:"count"`
	}
	err := s.db.Get(&topLink, `
		SELECT
			COALESCE(d.name, '') as domain,
			COALESCE(rl.path, '') as path,
			COUNT(*) as count
		FROM visits v
		LEFT JOIN redirect_links rl ON rl.id = v.link_id
		LEFT JOIN domains d ON d.id = rl.domain_id
		WHERE v.user_id = $1 AND v.created_at > NOW() - INTERVAL '1 hour'
		GROUP BY d.name, rl.path
		ORDER BY count DESC
		LIMIT 1
	`, userID)
	if err == nil && topLink.Domain != "" {
		data.TopLink = &TopLinkInfo{
			Domain: topLink.Domain,
			Path:   topLink.Path,
			Count:  topLink.Count,
		}
	}

	// Human vs Bot counts (last hour)
	s.db.Get(&data.HumanCount, `
		SELECT COUNT(*) FROM visits
		WHERE user_id = $1 AND created_at > NOW() - INTERVAL '1 hour' AND (is_bot = false OR is_bot IS NULL)
	`, userID)
	s.db.Get(&data.BotCount, `
		SELECT COUNT(*) FROM visits
		WHERE user_id = $1 AND created_at > NOW() - INTERVAL '1 hour' AND is_bot = true
	`, userID)
	total := data.HumanCount + data.BotCount
	if total > 0 {
		data.HumanPercent = (data.HumanCount * 100) / total
	} else {
		data.HumanPercent = 100
	}

	// Top countries (last hour)
	s.db.Select(&data.TopCountries, `
		SELECT COALESCE(country, 'Unknown') as country, COUNT(*) as count
		FROM visits
		WHERE user_id = $1 AND created_at > NOW() - INTERVAL '1 hour'
		GROUP BY country
		ORDER BY count DESC
		LIMIT 10
	`, userID)

	// Top links (last hour)
	s.db.Select(&data.TopLinks, `
		SELECT
			COALESCE(d.name, '') as domain,
			COALESCE(rl.path, '/') as path,
			COUNT(*) as count
		FROM visits v
		LEFT JOIN redirect_links rl ON rl.id = v.link_id
		LEFT JOIN domains d ON d.id = rl.domain_id
		WHERE v.user_id = $1 AND v.created_at > NOW() - INTERVAL '1 hour'
		GROUP BY d.name, rl.path
		ORDER BY count DESC
		LIMIT 8
	`, userID)

	// Timeline (last 30 minutes, per minute)
	s.db.Select(&data.Timeline, `
		SELECT
			TO_CHAR(created_at, 'HH24:MI') as minute,
			COUNT(*) as count
		FROM visits
		WHERE user_id = $1 AND created_at > NOW() - INTERVAL '30 minutes'
		GROUP BY TO_CHAR(created_at, 'HH24:MI')
		ORDER BY minute
	`, userID)

	return data, nil
}
