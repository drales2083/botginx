package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/botginx/botginx/modules/domains/models"
	"github.com/jmoiron/sqlx"
)

var (
	ErrDomainExists   = errors.New("domain already registered")
	ErrDomainNotFound = errors.New("domain not found")
)

type DomainService struct {
	db *sqlx.DB
}

func NewDomainService(db *sqlx.DB) *DomainService {
	return &DomainService{db: db}
}

func (s *DomainService) generateID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// List returns the domains a user owns. Shared platform domains are excluded --
// they are not the user's to manage.
func (s *DomainService) List(userID string) ([]models.Domain, error) {
	var domains []models.Domain
	err := s.db.Select(&domains, `
		SELECT d.*, COALESCE(s.name, '') as server_name
		FROM domains d
		LEFT JOIN servers s ON s.id = d.server_id
		WHERE d.user_id = $1 AND d.is_shared = FALSE
		ORDER BY d.created_at DESC
	`, userID)
	return domains, err
}

// ListShared returns the platform-wide domains added by admins.
func (s *DomainService) ListShared() ([]models.Domain, error) {
	var domains []models.Domain
	err := s.db.Select(&domains, `
		SELECT d.*, COALESCE(s.name, '') as server_name
		FROM domains d
		LEFT JOIN servers s ON s.id = d.server_id
		WHERE d.is_shared = TRUE
		ORDER BY d.created_at DESC
	`)
	return domains, err
}

// ListAvailable returns every domain a user may deploy to: their own, plus the
// shared pool when an admin has enabled it and added at least one domain.
func (s *DomainService) ListAvailable(userID string) ([]models.Domain, error) {
	own, err := s.List(userID)
	if err != nil {
		return nil, err
	}

	if !s.SharedDomainsEnabled() {
		return own, nil
	}

	shared, err := s.ListShared()
	if err != nil {
		return own, nil
	}

	return append(own, shared...), nil
}

// SharedDomainsEnabled reports whether users may deploy to the shared pool.
// The toggle only counts when there is actually something in the pool: with no
// shared domains added, the feature is off however the flag is set.
func (s *DomainService) SharedDomainsEnabled() bool {
	var value string
	if err := s.db.Get(&value,
		`SELECT value FROM platform_settings WHERE key = 'shared_domains_enabled'`,
	); err != nil {
		return false
	}
	if value != "true" {
		return false
	}

	var count int
	if err := s.db.Get(&count, `SELECT COUNT(*) FROM domains WHERE is_shared = TRUE`); err != nil {
		return false
	}
	return count > 0
}

// SetSharedDomainsEnabled flips the admin toggle.
func (s *DomainService) SetSharedDomainsEnabled(enabled bool) error {
	value := "false"
	if enabled {
		value = "true"
	}
	_, err := s.db.Exec(`
		INSERT INTO platform_settings (key, value, updated_at)
		VALUES ('shared_domains_enabled', $1, NOW())
		ON CONFLICT (key) DO UPDATE SET value = $1, updated_at = NOW()
	`, value)
	return err
}

// SharedDomainsFlag reads the raw toggle, ignoring whether the pool is empty.
// The admin UI needs this to show the switch in the position the admin left it.
func (s *DomainService) SharedDomainsFlag() bool {
	var value string
	if err := s.db.Get(&value,
		`SELECT value FROM platform_settings WHERE key = 'shared_domains_enabled'`,
	); err != nil {
		return false
	}
	return value == "true"
}

func (s *DomainService) Get(id string) (*models.Domain, error) {
	var domain models.Domain
	err := s.db.Get(&domain, `
		SELECT d.*, COALESCE(s.name, '') as server_name
		FROM domains d
		LEFT JOIN servers s ON s.id = d.server_id
		WHERE d.id = $1
	`, id)
	if err != nil {
		return nil, ErrDomainNotFound
	}

	// Generate token for old domains that don't have one
	if domain.VerifyToken == "" {
		domain.VerifyToken = "guardbot-" + s.generateID()[:12]
		s.db.Exec(`UPDATE domains SET verify_token = $1 WHERE id = $2`, domain.VerifyToken, domain.ID)
	}

	return &domain, nil
}

func (s *DomainService) GetByName(userID, name string) (*models.Domain, error) {
	var domain models.Domain
	err := s.db.Get(&domain, `SELECT * FROM domains WHERE user_id = $1 AND name = $2`, userID, name)
	if err != nil {
		return nil, ErrDomainNotFound
	}
	return &domain, nil
}

// Create adds a domain owned by one user.
func (s *DomainService) Create(userID string, input models.CreateDomainInput) (*models.Domain, error) {
	return s.create(userID, input, false)
}

// CreateShared adds a platform domain every user can deploy to. Called only
// from the admin panel; an admin adding a domain in their own user section goes
// through Create like anyone else.
func (s *DomainService) CreateShared(adminID string, input models.CreateDomainInput) (*models.Domain, error) {
	return s.create(adminID, input, true)
}

func (s *DomainService) create(userID string, input models.CreateDomainInput, shared bool) (*models.Domain, error) {
	// Check if domain exists for this user
	existing, _ := s.GetByName(userID, input.Name)
	if existing != nil {
		return nil, ErrDomainExists
	}

	// A shared domain must be unique platform-wide, not just per user.
	if shared {
		var count int
		s.db.Get(&count, `SELECT COUNT(*) FROM domains WHERE name = $1 AND is_shared = TRUE`, input.Name)
		if count > 0 {
			return nil, ErrDomainExists
		}
	}

	domain := &models.Domain{
		ID:          s.generateID(),
		UserID:      userID,
		Name:        input.Name,
		VerifyToken: "guardbot-" + s.generateID()[:12],
		DNSVerified: false,
		SSLEnabled:  false,
		IsShared:    shared,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if input.ServerID != "" {
		domain.ServerID = &input.ServerID
	}

	_, err := s.db.NamedExec(`
		INSERT INTO domains (id, user_id, name, verify_token, server_id, dns_verified, ssl_enabled, is_shared, created_at, updated_at)
		VALUES (:id, :user_id, :name, :verify_token, :server_id, :dns_verified, :ssl_enabled, :is_shared, :created_at, :updated_at)
	`, domain)

	return domain, err
}

func (s *DomainService) Update(id string, input models.UpdateDomainInput) (*models.Domain, error) {
	domain, err := s.Get(id)
	if err != nil {
		return nil, err
	}

	if input.ServerID != nil {
		domain.ServerID = input.ServerID
	}
	if input.DNSVerified != nil {
		domain.DNSVerified = *input.DNSVerified
	}
	if input.SSLEnabled != nil {
		domain.SSLEnabled = *input.SSLEnabled
	}
	domain.UpdatedAt = time.Now()

	_, err = s.db.NamedExec(`
		UPDATE domains SET
			server_id = :server_id,
			dns_verified = :dns_verified,
			ssl_enabled = :ssl_enabled,
			updated_at = :updated_at
		WHERE id = :id
	`, domain)

	return domain, err
}

func (s *DomainService) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM domains WHERE id = $1`, id)
	return err
}

func (s *DomainService) Count(userID string) (int, error) {
	var count int
	err := s.db.Get(&count, `SELECT COUNT(*) FROM domains WHERE user_id = $1`, userID)
	return count, err
}

func (s *DomainService) CountAll() (int, error) {
	var count int
	err := s.db.Get(&count, `SELECT COUNT(*) FROM domains`)
	return count, err
}

// ListUnverified returns domains that haven't been DNS verified yet
func (s *DomainService) ListUnverified() ([]models.Domain, error) {
	var domains []models.Domain
	err := s.db.Select(&domains, `
		SELECT * FROM domains
		WHERE dns_verified = FALSE AND verify_token != ''
		ORDER BY created_at DESC
	`)
	return domains, err
}

// ListVerifiedWithoutSSL returns domains that are verified but don't have SSL enabled
func (s *DomainService) ListVerifiedWithoutSSL() ([]models.Domain, error) {
	var domains []models.Domain
	err := s.db.Select(&domains, `
		SELECT * FROM domains
		WHERE dns_verified = TRUE AND ssl_enabled = FALSE
		ORDER BY created_at DESC
	`)
	return domains, err
}

// GetDeployIP returns the IP of an available deploy server for DNS instructions.
func (s *DomainService) GetDeployIP() string {
	var ip string
	err := s.db.Get(&ip, `SELECT ip FROM servers WHERE status = 'ready' ORDER BY created_at LIMIT 1`)
	if err != nil {
		return "No server available"
	}
	return ip
}
