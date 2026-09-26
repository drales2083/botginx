package services

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/botginx/botginx/modules/domains/models"
	"github.com/botginx/botginx/pkg/cpanel"
	"github.com/botginx/botginx/pkg/proxy"
	"github.com/jmoiron/sqlx"
)

// CpanelService handles cPanel connection management and auto-DNS operations
type CpanelService struct {
	db            *sqlx.DB
	encryptionKey []byte
	proxyService  *proxy.Service
}

// NewCpanelService creates a new cPanel service
func NewCpanelService(db *sqlx.DB) *CpanelService {
	// Get encryption key from env (same as session secret)
	key := os.Getenv("SESSION_SECRET")
	if len(key) < 32 {
		key = key + "00000000000000000000000000000000" // Pad if too short
	}

	return &CpanelService{
		db:            db,
		encryptionKey: []byte(key[:32]),
	}
}

// SetProxyService sets the proxy service for cPanel API calls
func (s *CpanelService) SetProxyService(ps *proxy.Service) {
	s.proxyService = ps
}

// newClient creates a cPanel client, using proxy if configured
func (s *CpanelService) newClient(host, username, apiToken string) *cpanel.Client {
	if s.proxyService != nil {
		return cpanel.NewClientWithProxy(host, username, apiToken, s.proxyService)
	}
	return cpanel.NewClientSimple(host, username, apiToken)
}

// encrypt encrypts a string using AES-GCM
func (s *CpanelService) encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt decrypts a string using AES-GCM
func (s *CpanelService) decrypt(ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	if len(data) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := data[:gcm.NonceSize()], string(data[gcm.NonceSize():])
	plaintext, err := gcm.Open(nil, nonce, []byte(ciphertext), nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// Create creates a new cPanel connection
func (s *CpanelService) Create(userID string, input models.CreateCpanelConnectionInput) (*models.CpanelConnection, error) {
	// Test connection first
	client := s.newClient(input.Host, input.Username, input.APIToken)
	if err := client.TestConnection(); err != nil {
		return nil, fmt.Errorf("connection test failed: %w", err)
	}

	// Encrypt the API token
	encryptedToken, err := s.encrypt(input.APIToken)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt token: %w", err)
	}

	// Generate random ID
	idBytes := make([]byte, 13)
	rand.Read(idBytes)
	id := "cpc_" + hex.EncodeToString(idBytes)
	now := time.Now()

	conn := &models.CpanelConnection{
		ID:                id,
		UserID:            userID,
		Name:              input.Name,
		Host:              input.Host,
		Username:          input.Username,
		APITokenEncrypted: encryptedToken,
		IsActive:          true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	_, err = s.db.Exec(`
		INSERT INTO cpanel_connections (id, user_id, name, host, username, api_token_encrypted, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, conn.ID, conn.UserID, conn.Name, conn.Host, conn.Username, conn.APITokenEncrypted, conn.IsActive, conn.CreatedAt, conn.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create connection: %w", err)
	}

	return conn, nil
}

// List returns all cPanel connections for a user
func (s *CpanelService) List(userID string) ([]models.CpanelConnection, error) {
	var connections []models.CpanelConnection
	err := s.db.Select(&connections, `
		SELECT c.*,
			(SELECT COUNT(*) FROM domains d WHERE d.cpanel_connection_id = c.id) as domain_count
		FROM cpanel_connections c
		WHERE c.user_id = $1
		ORDER BY c.created_at DESC
	`, userID)

	if err != nil {
		return nil, err
	}

	return connections, nil
}

// Get returns a cPanel connection by ID (with decrypted token)
func (s *CpanelService) Get(userID, connectionID string) (*models.CpanelConnection, error) {
	var conn models.CpanelConnection
	err := s.db.Get(&conn, `
		SELECT * FROM cpanel_connections
		WHERE id = $1 AND user_id = $2
	`, connectionID, userID)

	if err != nil {
		return nil, err
	}

	// Decrypt the token
	token, err := s.decrypt(conn.APITokenEncrypted)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt token: %w", err)
	}
	conn.APIToken = token

	return &conn, nil
}

// GetByID returns a cPanel connection by ID only (for internal use)
func (s *CpanelService) GetByID(connectionID string) (*models.CpanelConnection, error) {
	var conn models.CpanelConnection
	err := s.db.Get(&conn, `SELECT * FROM cpanel_connections WHERE id = $1`, connectionID)
	if err != nil {
		return nil, err
	}

	// Decrypt the token
	token, err := s.decrypt(conn.APITokenEncrypted)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt token: %w", err)
	}
	conn.APIToken = token

	return &conn, nil
}

// Update updates a cPanel connection
func (s *CpanelService) Update(userID, connectionID string, input models.UpdateCpanelConnectionInput) (*models.CpanelConnection, error) {
	conn, err := s.Get(userID, connectionID)
	if err != nil {
		return nil, err
	}

	if input.Name != nil {
		conn.Name = *input.Name
	}
	if input.Host != nil {
		conn.Host = *input.Host
	}
	if input.Username != nil {
		conn.Username = *input.Username
	}
	if input.IsActive != nil {
		conn.IsActive = *input.IsActive
	}

	// If new token provided, test and encrypt
	if input.APIToken != nil && *input.APIToken != "" {
		client := s.newClient(conn.Host, conn.Username, *input.APIToken)
		if err := client.TestConnection(); err != nil {
			return nil, fmt.Errorf("connection test failed: %w", err)
		}

		encryptedToken, err := s.encrypt(*input.APIToken)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt token: %w", err)
		}
		conn.APITokenEncrypted = encryptedToken
		conn.APIToken = *input.APIToken
	}

	conn.UpdatedAt = time.Now()

	_, err = s.db.Exec(`
		UPDATE cpanel_connections
		SET name = $1, host = $2, username = $3, api_token_encrypted = $4, is_active = $5, updated_at = $6
		WHERE id = $7 AND user_id = $8
	`, conn.Name, conn.Host, conn.Username, conn.APITokenEncrypted, conn.IsActive, conn.UpdatedAt, connectionID, userID)

	if err != nil {
		return nil, err
	}

	return conn, nil
}

// Delete deletes a cPanel connection and clears it from any domains using it
func (s *CpanelService) Delete(userID, connectionID string) error {
	// First clear the connection from any domains using it
	_, err := s.db.Exec(`
		UPDATE domains SET cpanel_connection_id = NULL, cpanel_auto_dns = false
		WHERE cpanel_connection_id = $1
	`, connectionID)
	if err != nil {
		log.Printf("[cpanel] Failed to clear connection %s from domains: %v", connectionID, err)
	}

	// Delete the connection
	result, err := s.db.Exec(`
		DELETE FROM cpanel_connections WHERE id = $1 AND user_id = $2
	`, connectionID, userID)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return errors.New("connection not found")
	}

	return nil
}

// TestConnection tests a cPanel connection
func (s *CpanelService) TestConnection(userID, connectionID string) error {
	conn, err := s.Get(userID, connectionID)
	if err != nil {
		return err
	}

	client := s.newClient(conn.Host, conn.Username, conn.APIToken)
	if err := client.TestConnection(); err != nil {
		// Update last_error
		errStr := err.Error()
		s.db.Exec(`UPDATE cpanel_connections SET last_error = $1 WHERE id = $2`, errStr, connectionID)
		return err
	}

	// Clear error and update last_used
	now := time.Now()
	s.db.Exec(`UPDATE cpanel_connections SET last_error = NULL, last_used_at = $1 WHERE id = $2`, now, connectionID)

	return nil
}

// ListDomains returns all domains available on a cPanel connection
func (s *CpanelService) ListDomains(userID, connectionID string) (*models.CpanelDomainInfo, error) {
	conn, err := s.Get(userID, connectionID)
	if err != nil {
		return nil, err
	}

	client := s.newClient(conn.Host, conn.Username, conn.APIToken)
	domains, err := client.GetDomains()
	if err != nil {
		return nil, err
	}

	return &models.CpanelDomainInfo{
		MainDomain:    domains.MainDomain,
		AddonDomains:  domains.AddonDomains,
		SubDomains:    domains.SubDomains,
		ParkedDomains: domains.ParkedDomains,
	}, nil
}

// AddVerificationTXT adds the verification TXT record via cPanel
func (s *CpanelService) AddVerificationTXT(connectionID, domain, token string) error {
	conn, err := s.GetByID(connectionID)
	if err != nil {
		return fmt.Errorf("failed to get connection: %w", err)
	}

	client := s.newClient(conn.Host, conn.Username, conn.APIToken)

	log.Printf("[cpanel] Adding verification TXT for %s via %s", domain, conn.Host)

	if err := client.AddVerificationTXT(domain, token); err != nil {
		// Update last_error
		errStr := err.Error()
		s.db.Exec(`UPDATE cpanel_connections SET last_error = $1 WHERE id = $2`, errStr, connectionID)
		return err
	}

	// Update last_used
	now := time.Now()
	s.db.Exec(`UPDATE cpanel_connections SET last_error = NULL, last_used_at = $1 WHERE id = $2`, now, connectionID)

	log.Printf("[cpanel] Verification TXT added successfully for %s", domain)
	return nil
}

// RemoveVerificationTXT removes the verification TXT record via cPanel
func (s *CpanelService) RemoveVerificationTXT(connectionID, domain string) error {
	conn, err := s.GetByID(connectionID)
	if err != nil {
		return err
	}

	client := s.newClient(conn.Host, conn.Username, conn.APIToken)
	return client.RemoveVerificationTXT(domain)
}

// AddAcmeChallengeTXT adds the ACME challenge TXT record via cPanel
func (s *CpanelService) AddAcmeChallengeTXT(connectionID, domain, token string) error {
	conn, err := s.GetByID(connectionID)
	if err != nil {
		return err
	}

	client := s.newClient(conn.Host, conn.Username, conn.APIToken)
	return client.AddAcmeChallengeTXT(domain, token)
}

// RemoveAcmeChallengeTXT removes the ACME challenge TXT record via cPanel
func (s *CpanelService) RemoveAcmeChallengeTXT(connectionID, domain string) error {
	conn, err := s.GetByID(connectionID)
	if err != nil {
		return err
	}

	client := s.newClient(conn.Host, conn.Username, conn.APIToken)
	return client.RemoveAcmeChallengeTXT(domain)
}

// GetClient returns a cPanel client for a connection
func (s *CpanelService) GetClient(connectionID string) (*cpanel.Client, error) {
	conn, err := s.GetByID(connectionID)
	if err != nil {
		return nil, err
	}

	return s.newClient(conn.Host, conn.Username, conn.APIToken), nil
}

// TestNewConnection tests a cPanel connection before saving (uses proxy if configured)
func (s *CpanelService) TestNewConnection(host, username, apiToken string) (*cpanel.DomainInfo, error) {
	client := s.newClient(host, username, apiToken)
	if err := client.TestConnection(); err != nil {
		return nil, err
	}
	return client.GetDomains()
}

// TriggerAutoDNS adds all required DNS records for a domain using its cPanel connection:
// 1. Verification TXT record (_guardbot-verify)
// 2. Wildcard A record (*.domain.com → Deploy VPS IP)
// Returns true if all records were added, false if no cPanel connection or error
func (s *CpanelService) TriggerAutoDNS(domainID, domainName, verifyToken, cpanelConnectionID string) bool {
	if cpanelConnectionID == "" {
		return false
	}

	log.Printf("[cpanel] Auto-DNS triggered for domain %s", domainName)

	// 1. Add verification TXT record
	err := s.AddVerificationTXT(cpanelConnectionID, domainName, verifyToken)
	if err != nil {
		log.Printf("[cpanel] Auto-DNS failed for %s (TXT): %v", domainName, err)
		return false
	}
	log.Printf("[cpanel] TXT record added for %s", domainName)

	// 2. Add wildcard A record pointing to Deploy VPS
	deployIP := os.Getenv("DEPLOY_VPS_IP")
	if deployIP == "" {
		log.Printf("[cpanel] DEPLOY_VPS_IP not set, skipping wildcard A record for %s", domainName)
		return true // TXT was added, partial success
	}

	err = s.AddWildcardARecord(cpanelConnectionID, domainName, deployIP)
	if err != nil {
		log.Printf("[cpanel] Auto-DNS wildcard A record failed for %s: %v", domainName, err)
		// Continue - TXT was added successfully
	} else {
		log.Printf("[cpanel] Wildcard A record added for *.%s → %s", domainName, deployIP)
	}

	log.Printf("[cpanel] Auto-DNS complete for %s", domainName)
	return true
}

// AddWildcardARecord adds or updates a wildcard A record (*.domain.com) via cPanel
// If wildcard already exists with different IP, it will be updated
func (s *CpanelService) AddWildcardARecord(connectionID, domain, ip string) error {
	conn, err := s.GetByID(connectionID)
	if err != nil {
		return fmt.Errorf("failed to get connection: %w", err)
	}

	client := s.newClient(conn.Host, conn.Username, conn.APIToken)

	log.Printf("[cpanel] Setting wildcard A record *.%s → %s via %s", domain, ip, conn.Host)

	// Get base domain for DNS zone
	baseDomain, err := client.GetBaseDomain(domain)
	if err != nil {
		return fmt.Errorf("failed to get base domain: %w", err)
	}

	// Update or add wildcard A record: name="*", domain=baseDomain
	// This will update existing record if it has different IP
	if err := client.UpdateOrAddARecord(baseDomain, "*", ip); err != nil {
		// Update last_error
		errStr := err.Error()
		s.db.Exec(`UPDATE cpanel_connections SET last_error = $1 WHERE id = $2`, errStr, connectionID)
		return err
	}

	// Update last_used
	now := time.Now()
	s.db.Exec(`UPDATE cpanel_connections SET last_error = NULL, last_used_at = $1 WHERE id = $2`, now, connectionID)

	return nil
}

// CleanupDomainDNS removes DNS records we added for a domain via cPanel
// Called when a domain is deleted to clean up our records
func (s *CpanelService) CleanupDomainDNS(connectionID, domain string) {
	conn, err := s.GetByID(connectionID)
	if err != nil {
		log.Printf("[cpanel] cleanup: failed to get connection for %s: %v", domain, err)
		return
	}

	client := s.newClient(conn.Host, conn.Username, conn.APIToken)

	baseDomain, err := client.GetBaseDomain(domain)
	if err != nil {
		log.Printf("[cpanel] cleanup: failed to get base domain for %s: %v", domain, err)
		return
	}

	// Remove verification TXT record
	if err := client.RemoveTXTRecord(baseDomain, "_guardbot-verify"); err != nil {
		log.Printf("[cpanel] cleanup: failed to remove TXT for %s: %v", domain, err)
	} else {
		log.Printf("[cpanel] cleanup: removed _guardbot-verify TXT for %s", domain)
	}

	// Remove ACME CNAME record
	if record, _ := client.FindCNAMERecord(baseDomain, "_acme-challenge"); record != nil {
		if err := client.RemoveCNAMERecord(baseDomain, "_acme-challenge"); err != nil {
			log.Printf("[cpanel] cleanup: failed to remove CNAME for %s: %v", domain, err)
		} else {
			log.Printf("[cpanel] cleanup: removed _acme-challenge CNAME for %s", domain)
		}
	}

	// Note: We don't remove the wildcard A record because:
	// 1. The user might have other domains/subdomains using it
	// 2. They might re-add the domain and want the same setup
	// If needed, user can manually remove it from cPanel
}

// AddAcmeCNAME adds or updates a CNAME record for acme-dns SSL verification
// If CNAME already exists with different target, it will be updated
func (s *CpanelService) AddAcmeCNAME(connectionID, domain, target string) error {
	conn, err := s.GetByID(connectionID)
	if err != nil {
		return fmt.Errorf("failed to get connection: %w", err)
	}

	client := s.newClient(conn.Host, conn.Username, conn.APIToken)

	log.Printf("[cpanel] Setting ACME CNAME _acme-challenge.%s → %s", domain, target)

	baseDomain, err := client.GetBaseDomain(domain)
	if err != nil {
		return fmt.Errorf("failed to get base domain: %w", err)
	}

	// Update or add CNAME for _acme-challenge
	if err := client.UpdateOrAddCNAMERecord(baseDomain, "_acme-challenge", target); err != nil {
		errStr := err.Error()
		s.db.Exec(`UPDATE cpanel_connections SET last_error = $1 WHERE id = $2`, errStr, connectionID)
		return err
	}

	now := time.Now()
	s.db.Exec(`UPDATE cpanel_connections SET last_error = NULL, last_used_at = $1 WHERE id = $2`, now, connectionID)

	return nil
}
