package services

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/botginx/botginx/modules/shortener/models"
	"github.com/jmoiron/sqlx"
)

type ShortenerService struct {
	db *sqlx.DB
}

func NewShortenerService(db *sqlx.DB) *ShortenerService {
	return &ShortenerService{db: db}
}

func (s *ShortenerService) generateID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// List returns all short links for a user
func (s *ShortenerService) List(userID string) ([]models.ShortLink, error) {
	var links []models.ShortLink
	err := s.db.Select(&links, `
		SELECT sl.*, d.name as domain_name
		FROM short_links sl
		JOIN domains d ON d.id = sl.domain_id
		WHERE sl.user_id = $1 AND sl.is_active = true
		ORDER BY sl.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	if links == nil {
		links = []models.ShortLink{}
	}
	return links, nil
}

// Get returns a single short link
func (s *ShortenerService) Get(id string) (*models.ShortLink, error) {
	var link models.ShortLink
	err := s.db.Get(&link, `
		SELECT sl.*, d.name as domain_name
		FROM short_links sl
		JOIN domains d ON d.id = sl.domain_id
		WHERE sl.id = $1
	`, id)
	if err != nil {
		return nil, err
	}
	return &link, nil
}

// Create creates a new short link
func (s *ShortenerService) Create(userID string, input models.CreateShortLinkInput) (*models.ShortLink, error) {
	id := s.generateID()

	if input.RotationMode == "" {
		input.RotationMode = "random"
	}
	if input.BotError == 0 {
		input.BotError = 403
	}

	now := time.Now()
	_, err := s.db.Exec(`
		INSERT INTO short_links (
			id, user_id, domain_id, path, destinations, rotation_mode,
			bot_error, qr_enabled, protection_settings, deploy_status,
			is_active, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`,
		id, userID, input.DomainID, input.Path,
		models.JSONStringArray(input.Destinations), input.RotationMode,
		input.BotError, input.QREnabled, input.ProtectionSettings,
		"pending", true, now, now,
	)
	if err != nil {
		return nil, err
	}

	return s.Get(id)
}

// Update updates a short link
func (s *ShortenerService) Update(id string, input models.UpdateShortLinkInput) (*models.ShortLink, error) {
	var updates []string
	var args []interface{}
	argNum := 2
	args = append(args, id)

	if input.Destinations != nil {
		updates = append(updates, fmt.Sprintf("destinations = $%d", argNum))
		args = append(args, models.JSONStringArray(input.Destinations))
		argNum++
	}
	if input.RotationMode != nil {
		updates = append(updates, fmt.Sprintf("rotation_mode = $%d", argNum))
		args = append(args, *input.RotationMode)
		argNum++
	}
	if input.BotError != nil {
		updates = append(updates, fmt.Sprintf("bot_error = $%d", argNum))
		args = append(args, *input.BotError)
		argNum++
	}
	if input.QREnabled != nil {
		updates = append(updates, fmt.Sprintf("qr_enabled = $%d", argNum))
		args = append(args, *input.QREnabled)
		argNum++
	}
	if input.ProtectionSettings != nil {
		updates = append(updates, fmt.Sprintf("protection_settings = $%d", argNum))
		args = append(args, *input.ProtectionSettings)
		argNum++
	}

	if len(updates) == 0 {
		return s.Get(id)
	}

	query := fmt.Sprintf(`UPDATE short_links SET %s, updated_at = NOW() WHERE id = $1`,
		strings.Join(updates, ", "))
	_, err := s.db.Exec(query, args...)
	if err != nil {
		return nil, err
	}

	return s.Get(id)
}

// Delete soft-deletes a short link
func (s *ShortenerService) Delete(id string) error {
	_, err := s.db.Exec(`UPDATE short_links SET is_active = false, updated_at = NOW() WHERE id = $1`, id)
	return err
}

// GetStats returns analytics for a short link
func (s *ShortenerService) GetStats(id string, days int) (*models.ShortLinkStats, error) {
	stats := &models.ShortLinkStats{
		ByCountry: make(map[string]int),
		ByDevice:  make(map[string]int),
		ByDay:     []models.DayStats{},
	}

	// Get totals from the link itself
	var link models.ShortLink
	err := s.db.Get(&link, `SELECT click_count, human_count, bot_count FROM short_links WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	stats.TotalClicks = link.ClickCount
	stats.HumanClicks = link.HumanCount
	stats.BotClicks = link.BotCount

	// By country
	rows, err := s.db.Query(`
		SELECT COALESCE(country, 'Unknown') as country, COUNT(*) as count
		FROM short_link_clicks
		WHERE link_id = $1 AND created_at > NOW() - INTERVAL '1 day' * $2
		GROUP BY country
		ORDER BY count DESC
		LIMIT 10
	`, id, days)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var country string
			var count int
			rows.Scan(&country, &count)
			stats.ByCountry[country] = count
		}
	}

	// By device
	rows, err = s.db.Query(`
		SELECT COALESCE(device, 'Unknown') as device, COUNT(*) as count
		FROM short_link_clicks
		WHERE link_id = $1 AND created_at > NOW() - INTERVAL '1 day' * $2
		GROUP BY device
	`, id, days)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var device string
			var count int
			rows.Scan(&device, &count)
			stats.ByDevice[device] = count
		}
	}

	// By day
	rows, err = s.db.Query(`
		SELECT DATE(created_at)::text as date,
		       SUM(CASE WHEN is_bot = false THEN 1 ELSE 0 END) as humans,
		       SUM(CASE WHEN is_bot = true THEN 1 ELSE 0 END) as bots
		FROM short_link_clicks
		WHERE link_id = $1 AND created_at > NOW() - INTERVAL '1 day' * $2
		GROUP BY DATE(created_at)
		ORDER BY date
	`, id, days)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var ds models.DayStats
			rows.Scan(&ds.Date, &ds.Humans, &ds.Bots)
			stats.ByDay = append(stats.ByDay, ds)
		}
	}

	return stats, nil
}

// CheckPathAvailable checks if a path is available for a domain
func (s *ShortenerService) CheckPathAvailable(domainID, path string) (bool, error) {
	var count int
	err := s.db.Get(&count, `
		SELECT COUNT(*) FROM short_links
		WHERE domain_id = $1 AND path = $2 AND is_active = true
	`, domainID, path)
	return count == 0, err
}

// RecordClick increments click counts for a short link
func (s *ShortenerService) RecordClick(linkID string, isBot bool, country, device, ip, userAgent string) error {
	// Increment counters on the link
	if isBot {
		_, err := s.db.Exec(`
			UPDATE short_links SET click_count = click_count + 1, bot_count = bot_count + 1, updated_at = NOW()
			WHERE id = $1
		`, linkID)
		if err != nil {
			return err
		}
	} else {
		_, err := s.db.Exec(`
			UPDATE short_links SET click_count = click_count + 1, human_count = human_count + 1, updated_at = NOW()
			WHERE id = $1
		`, linkID)
		if err != nil {
			return err
		}
	}

	// Also record in clicks table for detailed analytics
	_, err := s.db.Exec(`
		INSERT INTO short_link_clicks (id, link_id, visitor_ip_hash, country, device, user_agent, is_bot, created_at)
		VALUES (gen_random_uuid()::text, $1, $2, $3, $4, $5, $6, NOW())
	`, linkID, ip, country, device, userAgent, isBot)
	return err
}

// SetDeployStatus updates deployment status
func (s *ShortenerService) SetDeployStatus(id string, status string, url, errorMsg *string) error {
	_, err := s.db.Exec(`
		UPDATE short_links SET
			deploy_status = $2,
			deployed_url = COALESCE($3, deployed_url),
			deploy_error = $4,
			updated_at = NOW()
		WHERE id = $1
	`, id, status, url, errorMsg)
	return err
}

// GenerateRandomPath generates a random short path
func (s *ShortenerService) GenerateRandomPath(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	rand.Read(b)
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b)
}

// ResolveByHostPath resolves a short link by host and path
// Returns linkID, userID, destinations, error
func (s *ShortenerService) ResolveByHostPath(host, path string) (linkID, userID string, destinations []string, err error) {
	// Normalize path - remove leading slash
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return "", "", nil, fmt.Errorf("empty path")
	}

	// Normalize host
	host = strings.ToLower(strings.TrimSpace(host))
	host = strings.TrimSuffix(host, ".")
	if idx := strings.LastIndex(host, ":"); idx > -1 {
		host = host[:idx]
	}

	var row struct {
		ID           string                 `db:"id"`
		UserID       string                 `db:"user_id"`
		Destinations models.JSONStringArray `db:"destinations"`
	}
	err = s.db.Get(&row, `
		SELECT sl.id, sl.user_id, sl.destinations
		FROM short_links sl
		JOIN domains d ON d.id = sl.domain_id
		WHERE sl.path = $1
		  AND sl.is_active = true
		  AND (LOWER(d.name) = $2 OR LOWER(REGEXP_REPLACE(d.name, '^\*\.', '')) = $2)
		LIMIT 1
	`, path, host)
	if err != nil {
		return "", "", nil, err
	}

	return row.ID, row.UserID, []string(row.Destinations), nil
}

// GetLinkHost returns the full hostname for a short link (for settings push)
func (s *ShortenerService) GetLinkHost(linkID string) (host string, path string, domainID string, err error) {
	var row struct {
		Path       string `db:"path"`
		DomainID   string `db:"domain_id"`
		DomainName string `db:"domain_name"`
	}
	err = s.db.Get(&row, `
		SELECT sl.path, sl.domain_id, d.name as domain_name
		FROM short_links sl
		JOIN domains d ON d.id = sl.domain_id
		WHERE sl.id = $1
	`, linkID)
	if err != nil {
		return "", "", "", err
	}
	// Strip wildcard prefix
	domainName := row.DomainName
	if len(domainName) > 2 && domainName[:2] == "*." {
		domainName = domainName[2:]
	}
	return domainName, row.Path, row.DomainID, nil
}

// OwnerOf returns the user ID that owns a short link
func (s *ShortenerService) OwnerOf(linkID string) (string, error) {
	var userID string
	err := s.db.Get(&userID, `SELECT user_id FROM short_links WHERE id = $1`, linkID)
	return userID, err
}
