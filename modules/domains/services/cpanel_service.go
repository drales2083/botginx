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
	"github.com/jmoiron/sqlx"
)

// CpanelService handles cPanel connection management and auto-DNS operations
type CpanelService struct {
	db            *sqlx.DB
	encryptionKey []byte
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
	client := cpanel.NewClientSimple(input.Host, input.Username, input.APIToken)
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
		client := cpanel.NewClientSimple(conn.Host, conn.Username, *input.APIToken)
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

// Delete deletes a cPanel connection
func (s *CpanelService) Delete(userID, connectionID string) error {
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

	client := cpanel.NewClientSimple(conn.Host, conn.Username, conn.APIToken)
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

	client := cpanel.NewClientSimple(conn.Host, conn.Username, conn.APIToken)
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

	client := cpanel.NewClientSimple(conn.Host, conn.Username, conn.APIToken)

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

	client := cpanel.NewClientSimple(conn.Host, conn.Username, conn.APIToken)
	return client.RemoveVerificationTXT(domain)
}

// AddAcmeChallengeTXT adds the ACME challenge TXT record via cPanel
func (s *CpanelService) AddAcmeChallengeTXT(connectionID, domain, token string) error {
	conn, err := s.GetByID(connectionID)
	if err != nil {
		return err
	}

	client := cpanel.NewClientSimple(conn.Host, conn.Username, conn.APIToken)
	return client.AddAcmeChallengeTXT(domain, token)
}

// RemoveAcmeChallengeTXT removes the ACME challenge TXT record via cPanel
func (s *CpanelService) RemoveAcmeChallengeTXT(connectionID, domain string) error {
	conn, err := s.GetByID(connectionID)
	if err != nil {
		return err
	}

	client := cpanel.NewClientSimple(conn.Host, conn.Username, conn.APIToken)
	return client.RemoveAcmeChallengeTXT(domain)
}

// GetClient returns a cPanel client for a connection
func (s *CpanelService) GetClient(connectionID string) (*cpanel.Client, error) {
	conn, err := s.GetByID(connectionID)
	if err != nil {
		return nil, err
	}

	return cpanel.NewClientSimple(conn.Host, conn.Username, conn.APIToken), nil
}

// TriggerAutoDNS adds the verification TXT record for a domain using its cPanel connection
// Returns true if TXT was added, false if no cPanel connection or error
func (s *CpanelService) TriggerAutoDNS(domainID, domainName, verifyToken, cpanelConnectionID string) bool {
	if cpanelConnectionID == "" {
		return false
	}

	log.Printf("[cpanel] Auto-DNS triggered for domain %s", domainName)

	err := s.AddVerificationTXT(cpanelConnectionID, domainName, verifyToken)
	if err != nil {
		log.Printf("[cpanel] Auto-DNS failed for %s: %v", domainName, err)
		return false
	}

	log.Printf("[cpanel] Auto-DNS success for %s - TXT record added", domainName)
	return true
}
