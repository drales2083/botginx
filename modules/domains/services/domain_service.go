package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/botginx/botginx/modules/domains/models"
	"github.com/botginx/botginx/pkg/acmedns"
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
		WHERE d.user_id = $1 AND d.is_shared = FALSE AND COALESCE(d.is_marketplace, FALSE) = FALSE
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
		WHERE d.is_shared = TRUE AND COALESCE(d.is_marketplace, FALSE) = FALSE
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

// CreateForUser creates a domain for a specific user (admin assign feature).
// This allows admins to add domains directly to a user's account.
func (s *DomainService) CreateForUser(userID string, input models.CreateDomainInput) (*models.Domain, error) {
	return s.create(userID, input, false)
}

// SimpleUser is a minimal user representation for dropdowns
type SimpleUser struct {
	ID    string `db:"id" json:"id"`
	Email string `db:"email" json:"email"`
	Name  string `db:"name" json:"name"`
}

// ListAllUsers returns all users for admin dropdown
func (s *DomainService) ListAllUsers() ([]SimpleUser, error) {
	var users []SimpleUser
	err := s.db.Select(&users, `SELECT id, email, COALESCE(name, '') as name FROM users ORDER BY email`)
	return users, err
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

	// Detect if wildcard domain
	if len(domain.Name) > 2 && domain.Name[:2] == "*." {
		domain.IsWildcard = true
	}

	// Set default setup values
	domain.SetupType = models.SetupTypeDirect
	if domain.IsWildcard {
		domain.SetupStep = models.SetupStepPending // Wildcard needs CNAME setup
	} else {
		domain.SetupStep = models.SetupStepComplete
	}

	_, err := s.db.NamedExec(`
		INSERT INTO domains (id, user_id, name, verify_token, server_id, dns_verified, ssl_enabled, is_shared,
			setup_type, setup_step, is_wildcard, created_at, updated_at)
		VALUES (:id, :user_id, :name, :verify_token, :server_id, :dns_verified, :ssl_enabled, :is_shared,
			:setup_type, :setup_step, :is_wildcard, :created_at, :updated_at)
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
	if input.SetupType != nil {
		domain.SetupType = *input.SetupType
	}
	if input.SetupStep != nil {
		domain.SetupStep = *input.SetupStep
	}
	if input.AcmeToken != nil {
		domain.AcmeToken = input.AcmeToken
	}
	if input.AcmeTokenExpiresAt != nil {
		domain.AcmeTokenExpiresAt = input.AcmeTokenExpiresAt
	}
	if input.SSLError != nil {
		domain.SSLError = input.SSLError
	}
	// acme-dns fields
	if input.AcmeSubdomain != nil {
		domain.AcmeSubdomain = input.AcmeSubdomain
	}
	if input.AcmePassword != nil {
		domain.AcmePassword = input.AcmePassword
	}
	if input.AcmeFulldomain != nil {
		domain.AcmeFulldomain = input.AcmeFulldomain
	}
	if input.AcmeCnameVerified != nil {
		domain.AcmeCnameVerified = *input.AcmeCnameVerified
	}
	domain.UpdatedAt = time.Now()

	_, err = s.db.NamedExec(`
		UPDATE domains SET
			server_id = :server_id,
			dns_verified = :dns_verified,
			ssl_enabled = :ssl_enabled,
			setup_type = :setup_type,
			setup_step = :setup_step,
			acme_token = :acme_token,
			acme_token_expires_at = :acme_token_expires_at,
			ssl_error = :ssl_error,
			acme_subdomain = :acme_subdomain,
			acme_password = :acme_password,
			acme_fulldomain = :acme_fulldomain,
			acme_cname_verified = :acme_cname_verified,
			updated_at = :updated_at
		WHERE id = :id
	`, domain)

	return domain, err
}

func (s *DomainService) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM domains WHERE id = $1`, id)
	return err
}

// UpdateTurnstile updates the Cloudflare Turnstile credentials for a domain
func (s *DomainService) UpdateTurnstile(id, siteKey, secretKey string) error {
	_, err := s.db.Exec(`
		UPDATE domains
		SET turnstile_site_key = $1, turnstile_secret_key = $2, updated_at = NOW()
		WHERE id = $3
	`, siteKey, secretKey, id)
	return err
}

// GetTurnstileByDomain returns Turnstile credentials for a domain name
func (s *DomainService) GetTurnstileByDomain(domainName string) (siteKey, secretKey string, err error) {
	var result struct {
		SiteKey   *string `db:"turnstile_site_key"`
		SecretKey *string `db:"turnstile_secret_key"`
	}
	err = s.db.Get(&result, `
		SELECT turnstile_site_key, turnstile_secret_key
		FROM domains
		WHERE name = $1 OR name = $2
		LIMIT 1
	`, domainName, "*."+domainName)
	if err != nil {
		return "", "", err
	}
	if result.SiteKey != nil {
		siteKey = *result.SiteKey
	}
	if result.SecretKey != nil {
		secretKey = *result.SecretKey
	}
	return siteKey, secretKey, nil
}

// TransferOwnership changes the owner of a domain to another user
func (s *DomainService) TransferOwnership(domainID, newUserID string) error {
	result, err := s.db.Exec(`
		UPDATE domains SET user_id = $1, is_shared = FALSE, updated_at = NOW()
		WHERE id = $2
	`, newUserID, domainID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrDomainNotFound
	}
	return nil
}

// CountRedirectLinks returns the number of redirect links using this domain
func (s *DomainService) CountRedirectLinks(domainID string) (int, error) {
	var count int
	err := s.db.Get(&count, `SELECT COUNT(*) FROM redirect_links WHERE domain_id = $1`, domainID)
	return count, err
}

// GetRedirectLinkIDs returns all redirect link IDs for a domain (for cleanup)
func (s *DomainService) GetRedirectLinkIDs(domainID string) ([]string, error) {
	var ids []string
	err := s.db.Select(&ids, `SELECT id FROM redirect_links WHERE domain_id = $1`, domainID)
	return ids, err
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

// GetDeployServerID returns the ID of an available deploy server.
func (s *DomainService) GetDeployServerID() string {
	var id string
	err := s.db.Get(&id, `SELECT id FROM servers WHERE status = 'ready' ORDER BY created_at LIMIT 1`)
	if err != nil {
		return ""
	}
	return id
}

// GetDeployServer returns full SSH credentials for an available deploy server.
func (s *DomainService) GetDeployServer() (ip string, port int, user string, password string, err error) {
	var server struct {
		IP       string `db:"ip"`
		Port     int    `db:"port"`
		User     string `db:"ssh_user"`
		Password string `db:"ssh_password"`
	}
	err = s.db.Get(&server, `
		SELECT ip, port, ssh_user, ssh_password
		FROM servers
		WHERE status = 'ready'
		ORDER BY created_at LIMIT 1
	`)
	if err != nil {
		return "", 0, "", "", err
	}
	return server.IP, server.Port, server.User, server.Password, nil
}

// RegisterWithAcmeDNS registers a domain with acme-dns on the Deploy VPS
// Returns the registration info to be stored in the domain record
func (s *DomainService) RegisterWithAcmeDNS(domainID string) (*acmedns.Registration, error) {
	// Get Deploy VPS credentials
	ip, port, user, password, err := s.GetDeployServer()
	if err != nil {
		return nil, fmt.Errorf("no deploy server available: %w", err)
	}

	// Connect to Deploy VPS
	client, err := acmedns.NewClient(ip, port, user, password)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to deploy server: %w", err)
	}
	defer client.Close()

	// Check if acme-dns is installed
	if !client.IsInstalled() {
		// Try to set it up
		acmeDomain := os.Getenv("ACME_DNS_DOMAIN")
		if acmeDomain == "" {
			return nil, fmt.Errorf("ACME_DNS_DOMAIN environment variable not set")
		}
		if err := client.Setup(acmeDomain); err != nil {
			return nil, fmt.Errorf("acme-dns not installed and setup failed: %w", err)
		}
	}

	// Register new subdomain
	reg, err := client.Register()
	if err != nil {
		return nil, fmt.Errorf("acme-dns registration failed: %w", err)
	}

	// Store registration in database (including username for API auth)
	_, err = s.db.Exec(`
		UPDATE domains SET
			acme_subdomain = $1,
			acme_username = $2,
			acme_password = $3,
			acme_fulldomain = $4,
			updated_at = NOW()
		WHERE id = $5
	`, reg.Subdomain, reg.Username, reg.Password, reg.Fulldomain, domainID)
	if err != nil {
		return nil, fmt.Errorf("failed to store acme-dns credentials: %w", err)
	}

	return reg, nil
}

// GetAcmeDNSRegistration returns the acme-dns registration info for a domain
func (s *DomainService) GetAcmeDNSRegistration(domainID string) (subdomain, password, fulldomain string, err error) {
	var reg struct {
		Subdomain  *string `db:"acme_subdomain"`
		Password   *string `db:"acme_password"`
		Fulldomain *string `db:"acme_fulldomain"`
	}
	err = s.db.Get(&reg, `SELECT acme_subdomain, acme_password, acme_fulldomain FROM domains WHERE id = $1`, domainID)
	if err != nil {
		return "", "", "", err
	}
	if reg.Subdomain == nil || reg.Password == nil {
		return "", "", "", fmt.Errorf("domain not registered with acme-dns")
	}
	return *reg.Subdomain, *reg.Password, *reg.Fulldomain, nil
}
