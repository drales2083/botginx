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
	s.db.Get(&stats.UniqueVisits, `SELECT COUNT(*) FROM visits WHERE link_id = $1 AND is_unique = true`, linkID)
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
	s.db.Get(&stats.UniqueVisits, `SELECT COUNT(*) FROM visits WHERE user_id = $1 AND is_unique = true`, userID)
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
		return &models.LinkSettings{
			LinkID:      linkID,
			CountryMode: "all",
			CountryList: []string{},
			DeviceMode:  "all",
			DeviceList:  []string{},
		}, nil
	}

	json.Unmarshal([]byte(settings.CountryListRaw), &settings.CountryList)
	json.Unmarshal([]byte(settings.DeviceListRaw), &settings.DeviceList)

	return &settings, nil
}

func (s *AnalyticsService) SaveLinkSettings(settings *models.LinkSettings) error {
	countryJSON, _ := json.Marshal(settings.CountryList)
	deviceJSON, _ := json.Marshal(settings.DeviceList)
	settings.CountryListRaw = string(countryJSON)
	settings.DeviceListRaw = string(deviceJSON)
	settings.UpdatedAt = time.Now()

	if settings.ID == "" {
		settings.ID = s.generateID()
		_, err := s.db.Exec(`
			INSERT INTO link_settings (id, link_id, country_mode, country_list, device_mode, device_list,
				block_bots, block_tor, block_proxy, block_datacenter, block_headless,
				min_behavior_score, redirect_on_block, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		`, settings.ID, settings.LinkID, settings.CountryMode, settings.CountryListRaw,
			settings.DeviceMode, settings.DeviceListRaw, settings.BlockBots, settings.BlockTor,
			settings.BlockProxy, settings.BlockDatacenter, settings.BlockHeadless,
			settings.MinBehaviorScore, settings.RedirectOnBlock, settings.UpdatedAt)
		return err
	}

	_, err := s.db.Exec(`
		UPDATE link_settings
		SET country_mode = $2, country_list = $3, device_mode = $4, device_list = $5,
			block_bots = $6, block_tor = $7, block_proxy = $8, block_datacenter = $9,
			block_headless = $10, min_behavior_score = $11, redirect_on_block = $12, updated_at = $13
		WHERE link_id = $1
	`, settings.LinkID, settings.CountryMode, settings.CountryListRaw,
		settings.DeviceMode, settings.DeviceListRaw, settings.BlockBots, settings.BlockTor,
		settings.BlockProxy, settings.BlockDatacenter, settings.BlockHeadless,
		settings.MinBehaviorScore, settings.RedirectOnBlock, settings.UpdatedAt)
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
	if settings.MinBehaviorScore > 0 && behaviorScore < settings.MinBehaviorScore {
		return true, "low_behavior_score"
	}

	if settings.CountryMode == "whitelist" && len(settings.CountryList) > 0 {
		if !contains(settings.CountryList, strings.ToUpper(country)) {
			return true, "country_not_whitelisted"
		}
	}
	if settings.CountryMode == "blacklist" && len(settings.CountryList) > 0 {
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
		SELECT link_id, COUNT(*) as total_visits,
			   COUNT(*) FILTER (WHERE is_unique = true) as unique_visits,
			   COUNT(*) FILTER (WHERE is_bot = true) as bot_visits,
			   COALESCE(AVG(behavior_score), 0) as avg_behavior
		FROM visits
		WHERE user_id = $1
		GROUP BY link_id
		ORDER BY total_visits DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var linkID string
		var total, unique, bots int
		var avgBehavior float64
		rows.Scan(&linkID, &total, &unique, &bots, &avgBehavior)
		results = append(results, map[string]interface{}{
			"linkId":       linkID,
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
