package services

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/botginx/botginx/modules/redirectlinks/models"
	"github.com/jmoiron/sqlx"
)

type RedirectLinkService struct {
	db *sqlx.DB
}

// linkOwner is the minimum needed to attribute an inbound request to a link and
// the account that owns it.
type linkOwner struct {
	ID     string `db:"id"`
	UserID string `db:"user_id"`
}

// ResolveByHost maps an inbound hostname such as "amber-canyon.example.com" to
// the redirect link serving it. Analytics events arrive identified by host
// rather than by link id, so this is how incoming traffic is attributed.
func (s *RedirectLinkService) ResolveByHost(host string) (linkID, userID string, err error) {
	host = normalizeHost(host)
	if host == "" {
		return "", "", fmt.Errorf("empty host")
	}

	var row linkOwner
	err = s.db.Get(&row, `
		SELECT rl.id, rl.user_id
		FROM redirect_links rl
		JOIN domains d ON d.id = rl.domain_id
		WHERE LOWER(rl.subdomain || '.' || REGEXP_REPLACE(d.name, '^\*\.', '')) = $1
		LIMIT 1
	`, host)
	if err != nil {
		return "", "", err
	}

	return row.ID, row.UserID, nil
}

// OwnerOf returns the account a link belongs to.
func (s *RedirectLinkService) OwnerOf(linkID string) (string, error) {
	var userID string
	err := s.db.Get(&userID, `SELECT user_id FROM redirect_links WHERE id = $1`, linkID)
	return userID, err
}

// normalizeHost strips the port, any trailing root dot, and case, so hosts
// compare equal however the upstream reported them.
func normalizeHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	host = strings.TrimSuffix(host, ".")

	if i := strings.LastIndex(host, ":"); i > -1 && !strings.Contains(host[i:], "]") {
		host = host[:i]
	}

	return host
}

// stripWildcard removes the "*." prefix from wildcard domain names.
// e.g., "*.example.com" -> "example.com"
func stripWildcard(domain string) string {
	if len(domain) > 2 && domain[:2] == "*." {
		return domain[2:]
	}
	return domain
}

func NewRedirectLinkService(db *sqlx.DB) *RedirectLinkService {
	return &RedirectLinkService{db: db}
}

func (s *RedirectLinkService) generateID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *RedirectLinkService) List(userID string) ([]models.RedirectLink, error) {
	var links []models.RedirectLink
	err := s.db.Select(&links, `
		SELECT r.*, d.name as domain_name,
			COALESCE((SELECT COUNT(*) FROM visits v WHERE v.link_id = r.id), 0) as view_count
		FROM redirect_links r
		LEFT JOIN domains d ON d.id = r.domain_id
		WHERE r.user_id = $1 AND r.is_active = true
		ORDER BY r.created_at DESC
	`, userID)
	return links, err
}

func (s *RedirectLinkService) Get(id string) (*models.RedirectLink, error) {
	var link models.RedirectLink
	err := s.db.Get(&link, `
		SELECT r.*, d.name as domain_name
		FROM redirect_links r
		LEFT JOIN domains d ON d.id = r.domain_id
		WHERE r.id = $1
	`, id)
	if err != nil {
		return nil, err
	}
	return &link, nil
}

func (s *RedirectLinkService) Create(userID string, input models.CreateRedirectLinkInput) (*models.RedirectLink, error) {
	id := s.generateID()

	linkType := models.LinkTypeRedirect
	if input.Type == "HTML" {
		linkType = models.LinkTypeHTML
	}

	link := &models.RedirectLink{
		ID:                id,
		UserID:            userID,
		DomainID:          input.DomainID,
		Subdomain:         input.Subdomain,
		Path:              input.Path,
		Type:              linkType,
		DestinationURLs:   input.DestinationURLs,
		AnimationDuration: 3,
		TurnstileEnabled:  input.TurnstileEnabled,
		BotProtection:     input.BotProtection,
		PassParams:        input.PassParams,
		DeployStatus:      models.DeployStatusPending,
		IsActive:          true,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	if input.HTMLContent != "" {
		link.HTMLContent = &input.HTMLContent
	}

	_, err := s.db.NamedExec(`
		INSERT INTO redirect_links (
			id, user_id, domain_id, subdomain, path, type, destination_urls, html_content,
			animation_duration, turnstile_enabled, bot_protection, pass_params,
			deploy_status, is_active, created_at, updated_at
		) VALUES (
			:id, :user_id, :domain_id, :subdomain, :path, :type, :destination_urls, :html_content,
			:animation_duration, :turnstile_enabled, :bot_protection, :pass_params,
			:deploy_status, :is_active, :created_at, :updated_at
		)
	`, link)

	return link, err
}

func (s *RedirectLinkService) Update(id string, input models.UpdateRedirectLinkInput) (*models.RedirectLink, error) {
	updates := []string{}
	args := []interface{}{id}
	argNum := 2

	if input.DestinationURLs != nil {
		updates = append(updates, fmt.Sprintf("destination_urls = $%d", argNum))
		args = append(args, models.JSONArray(input.DestinationURLs))
		argNum++
	}
	if input.HTMLContent != "" {
		updates = append(updates, fmt.Sprintf("html_content = $%d", argNum))
		args = append(args, input.HTMLContent)
		argNum++
	}
	if input.TurnstileEnabled != nil {
		updates = append(updates, fmt.Sprintf("turnstile_enabled = $%d", argNum))
		args = append(args, *input.TurnstileEnabled)
		argNum++
	}
	if input.BotProtection != nil {
		updates = append(updates, fmt.Sprintf("bot_protection = $%d", argNum))
		args = append(args, *input.BotProtection)
		argNum++
	}
	if input.PassParams != nil {
		updates = append(updates, fmt.Sprintf("pass_params = $%d", argNum))
		args = append(args, *input.PassParams)
		argNum++
	}

	if len(updates) == 0 {
		return s.Get(id)
	}

	query := fmt.Sprintf(`UPDATE redirect_links SET %s, updated_at = NOW() WHERE id = $1`,
		stringJoin(updates, ", "))
	_, err := s.db.Exec(query, args...)
	if err != nil {
		return nil, err
	}

	return s.Get(id)
}

func (s *RedirectLinkService) Delete(id string) error {
	// Soft delete
	_, err := s.db.Exec(`UPDATE redirect_links SET is_active = false, updated_at = NOW() WHERE id = $1`, id)
	return err
}

func (s *RedirectLinkService) UpdateCustomization(id string, customization models.JSONMap) error {
	_, err := s.db.Exec(`
		UPDATE redirect_links SET customization = $2, updated_at = NOW() WHERE id = $1
	`, id, customization)
	return err
}

func (s *RedirectLinkService) UpdateDestinationAndDelay(id string, url string, delay int) error {
	return s.UpdateDestinationsAndDelay(id, []string{url}, delay)
}

func (s *RedirectLinkService) UpdateDestinationsAndDelay(id string, urls []string, delay int) error {
	if delay < 1 {
		delay = 3
	}
	if delay > 30 {
		delay = 30
	}
	_, err := s.db.Exec(`
		UPDATE redirect_links SET destination_urls = $2, animation_duration = $3, updated_at = NOW() WHERE id = $1
	`, id, models.JSONArray(urls), delay)
	return err
}

func (s *RedirectLinkService) UpdateDestinationURLs(id string, urls []string) (*models.RedirectLink, error) {
	_, err := s.db.Exec(`
		UPDATE redirect_links SET destination_urls = $2, updated_at = NOW() WHERE id = $1
	`, id, models.JSONArray(urls))
	if err != nil {
		return nil, err
	}
	return s.Get(id)
}

func (s *RedirectLinkService) SetDeployStatus(id string, status models.DeployStatus, url, errorMsg *string) error {
	_, err := s.db.Exec(`
		UPDATE redirect_links SET
			deploy_status = $2,
			deployed_url = COALESCE($3, deployed_url),
			deploy_error = $4,
			updated_at = NOW()
		WHERE id = $1
	`, id, status, url, errorMsg)
	return err
}

// DeleteByDomainID deletes all redirect links for a domain (when domain is removed)
func (s *RedirectLinkService) DeleteByDomainID(domainID string) error {
	_, err := s.db.Exec(`UPDATE redirect_links SET is_active = false, updated_at = NOW() WHERE domain_id = $1`, domainID)
	return err
}

// GetLinkHost returns the full hostname (including path) and domain ID for a link
// Used by analytics to push settings to the deploy VPS
func (s *RedirectLinkService) GetLinkHost(linkID string) (host string, domainID string, err error) {
	var row struct {
		Subdomain  string `db:"subdomain"`
		Path       string `db:"path"`
		DomainID   string `db:"domain_id"`
		DomainName string `db:"domain_name"`
	}
	err = s.db.Get(&row, `
		SELECT rl.subdomain, rl.path, rl.domain_id, d.name as domain_name
		FROM redirect_links rl
		JOIN domains d ON d.id = rl.domain_id
		WHERE rl.id = $1
	`, linkID)
	if err != nil {
		return "", "", err
	}
	host = row.Subdomain + "." + stripWildcard(row.DomainName)
	if row.Path != "" {
		host = host + "/" + row.Path
	}
	return host, row.DomainID, nil
}

// Helper
func stringJoin(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for _, s := range strs[1:] {
		result += sep + s
	}
	return result
}
