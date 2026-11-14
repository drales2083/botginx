package services

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/botginx/botginx/modules/domains/models"
	"github.com/botginx/botginx/pkg/acmedns"
	"github.com/botginx/botginx/pkg/providers/cloudflare"
	"github.com/jmoiron/sqlx"
)

// CloudflareService handles Cloudflare integration
type CloudflareService struct {
	db            *sqlx.DB
	encryptionKey []byte
	domainService *DomainService
}

// NewCloudflareService creates a new Cloudflare service
func NewCloudflareService(db *sqlx.DB, domainService *DomainService) *CloudflareService {
	key := os.Getenv("CLOUDFLARE_ENCRYPTION_KEY")
	if key == "" {
		key = os.Getenv("SESSION_SECRET")
	}

	keyBytes := []byte(key)
	if len(keyBytes) < 32 {
		padded := make([]byte, 32)
		copy(padded, keyBytes)
		keyBytes = padded
	} else if len(keyBytes) > 32 {
		keyBytes = keyBytes[:32]
	}

	return &CloudflareService{
		db:            db,
		encryptionKey: keyBytes,
		domainService: domainService,
	}
}

// encrypt encrypts data using AES-256-GCM
func (s *CloudflareService) encrypt(plaintext string) (string, error) {
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

// decrypt decrypts data using AES-256-GCM
func (s *CloudflareService) decrypt(ciphertext string) (string, error) {
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

// GetConnection returns the user's Cloudflare connection status
func (s *CloudflareService) GetConnection(userID string) (*models.CloudflareConnection, error) {
	var email sql.NullString
	var encryptedKey sql.NullString
	var connectedAt sql.NullTime

	err := s.db.QueryRow(`
		SELECT cloudflare_email, cloudflare_api_key_encrypted, cloudflare_connected_at
		FROM users WHERE id = $1
	`, userID).Scan(&email, &encryptedKey, &connectedAt)
	if err != nil {
		return nil, err
	}

	conn := &models.CloudflareConnection{
		UserID:      userID,
		IsConnected: email.Valid && email.String != "" && encryptedKey.Valid && encryptedKey.String != "",
	}

	if email.Valid {
		conn.Email = email.String
	}
	if connectedAt.Valid {
		conn.ConnectedAt = &connectedAt.Time
	}

	return conn, nil
}

// Connect validates and saves the Cloudflare credentials
func (s *CloudflareService) Connect(ctx context.Context, userID, email, apiKey string) error {
	// Validate credentials by making a test request
	client := cloudflare.NewClient(email, apiKey)
	if err := client.VerifyCredentials(ctx); err != nil {
		return fmt.Errorf("invalid credentials: %w", err)
	}

	// Encrypt API key
	encryptedKey, err := s.encrypt(apiKey)
	if err != nil {
		return fmt.Errorf("encrypting API key: %w", err)
	}

	// Save to database
	_, err = s.db.Exec(`
		UPDATE users 
		SET cloudflare_email = $1, 
		    cloudflare_api_key_encrypted = $2, 
		    cloudflare_connected_at = NOW()
		WHERE id = $3
	`, email, encryptedKey, userID)
	if err != nil {
		return fmt.Errorf("saving credentials: %w", err)
	}

	return nil
}

// Disconnect removes the Cloudflare credentials
func (s *CloudflareService) Disconnect(userID string) error {
	_, err := s.db.Exec(`
		UPDATE users 
		SET cloudflare_email = NULL, 
		    cloudflare_api_key_encrypted = NULL, 
		    cloudflare_connected_at = NULL
		WHERE id = $1
	`, userID)
	return err
}

// getClient creates an authenticated Cloudflare client for the user
func (s *CloudflareService) getClient(userID string) (*cloudflare.Client, error) {
	var email sql.NullString
	var encryptedKey sql.NullString

	err := s.db.QueryRow(`
		SELECT cloudflare_email, cloudflare_api_key_encrypted
		FROM users WHERE id = $1
	`, userID).Scan(&email, &encryptedKey)
	if err != nil {
		return nil, err
	}

	if !email.Valid || !encryptedKey.Valid {
		return nil, errors.New("cloudflare not connected")
	}

	apiKey, err := s.decrypt(encryptedKey.String)
	if err != nil {
		return nil, fmt.Errorf("decrypting API key: %w", err)
	}

	return cloudflare.NewClient(email.String, apiKey), nil
}

// ListZones returns all zones from the user's Cloudflare account
func (s *CloudflareService) ListZones(ctx context.Context, userID string) ([]models.CloudflareZone, error) {
	client, err := s.getClient(userID)
	if err != nil {
		return nil, err
	}

	cfZones, err := client.ListZones(ctx)
	if err != nil {
		return nil, err
	}

	// Get list of already-imported domains
	importedDomains := make(map[string]string)
	rows, err := s.db.Query(`
		SELECT name, id FROM domains 
		WHERE user_id = $1 AND source = 'cloudflare'
	`, userID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var name, id string
			rows.Scan(&name, &id)
			importedDomains[name] = id
		}
	}

	zones := make([]models.CloudflareZone, len(cfZones))
	for i, z := range cfZones {
		zones[i] = models.CloudflareZone{
			ZoneID:      z.ID,
			Domain:      z.Name,
			Status:      z.Status,
			NameServers: z.NameServers,
		}
		if localID, ok := importedDomains[z.Name]; ok {
			zones[i].IsImported = true
			zones[i].LocalID = localID
		}
	}

	return zones, nil
}

// ListDNSRecords returns DNS records for a zone
func (s *CloudflareService) ListDNSRecords(ctx context.Context, userID, zoneID string) ([]models.CloudflareDNSRecord, error) {
	client, err := s.getClient(userID)
	if err != nil {
		return nil, err
	}

	cfRecords, err := client.ListDNSRecords(ctx, zoneID)
	if err != nil {
		return nil, err
	}

	records := make([]models.CloudflareDNSRecord, len(cfRecords))
	for i, r := range cfRecords {
		records[i] = models.CloudflareDNSRecord{
			RecordID: r.ID,
			Type:     r.Type,
			Name:     r.Name,
			Content:  r.Content,
			TTL:      r.TTL,
			Proxied:  r.Proxied,
		}
	}

	return records, nil
}

// Import imports a Cloudflare zone as a GuardBot domain
func (s *CloudflareService) Import(ctx context.Context, userID string, input models.CloudflareImportInput) (*models.Domain, error) {
	client, err := s.getClient(userID)
	if err != nil {
		return nil, err
	}

	// Verify zone exists and get details
	zone, err := client.GetZone(ctx, input.ZoneID)
	if err != nil {
		return nil, fmt.Errorf("getting zone: %w", err)
	}

	// Check if domain already exists
	var existingID string
	err = s.db.QueryRow(`SELECT id FROM domains WHERE name = $1 AND user_id = $2`, zone.Name, userID).Scan(&existingID)
	if err == nil {
		return nil, fmt.Errorf("domain %s already exists", zone.Name)
	}

	// Create domain in GuardBot
	domain := &models.Domain{
		UserID:            userID,
		Name:              zone.Name,
		Source:            "cloudflare",
		ProviderDomainID:  &input.ZoneID,
		DNSAutoConfigured: input.ConfigureDNS,
		IsWildcard:        true,
		VerifyToken:       fmt.Sprintf("guardbot-%s", generateID(12)),
	}
	domain.ID = fmt.Sprintf("dom_%s", generateID(12))

	// Insert domain
	err = s.db.QueryRow(`
		INSERT INTO domains (id, user_id, name, verify_token, source, provider_domain_id, dns_auto_configured, is_wildcard, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, true, NOW(), NOW())
		RETURNING created_at
	`, domain.ID, domain.UserID, domain.Name, domain.VerifyToken, domain.Source, input.ZoneID, domain.DNSAutoConfigured).
		Scan(&domain.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating domain: %w", err)
	}

	// Auto-configure DNS if requested
	if input.ConfigureDNS {
		go func() {
			setupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			if err := s.setupDNSInternal(setupCtx, userID, input.ZoneID, zone.Name); err != nil {
				log.Printf("Cloudflare DNS setup failed for %s: %v", zone.Name, err)
				s.db.Exec(`UPDATE domains SET dns_config_error = $1 WHERE id = $2`, err.Error(), domain.ID)
			} else {
				s.db.Exec(`UPDATE domains SET dns_verified = true WHERE id = $1`, domain.ID)
			}
		}()
	}

	return domain, nil
}

// SetupDNS configures DNS records for a domain
func (s *CloudflareService) SetupDNS(ctx context.Context, userID, zoneID, domain string) error {
	return s.setupDNSInternal(ctx, userID, zoneID, domain)
}

func (s *CloudflareService) setupDNSInternal(ctx context.Context, userID, zoneID, domain string) error {
	client, err := s.getClient(userID)
	if err != nil {
		return err
	}

	// Get server IP
	serverIP := os.Getenv("GUARDBOT_SERVER_IP")
	if serverIP == "" {
		// Try to get external IP
		resp, err := http.Get("https://api.ipify.org")
		if err == nil {
			defer resp.Body.Close()
			ipBytes, _ := io.ReadAll(resp.Body)
			serverIP = strings.TrimSpace(string(ipBytes))
		}
	}

	if serverIP == "" || net.ParseIP(serverIP) == nil {
		return errors.New("could not determine server IP")
	}

	// Setup A records
	_, _, err = client.SetupDNS(ctx, zoneID, domain, serverIP)
	if err != nil {
		return fmt.Errorf("setting up A records: %w", err)
	}

	// Setup acme-dns CNAME if configured
	acmeDomain := os.Getenv("ACME_DNS_DOMAIN")
	if acmeDomain != "" {
		reg, err := s.registerWithAcmeDNS(userID, domain)
		if err != nil {
			log.Printf("Warning: acme-dns registration failed for %s: %v", domain, err)
		} else if _, err = client.AddAcmeDNSCname(ctx, zoneID, domain, reg.Fulldomain); err != nil {
			log.Printf("Warning: failed to add ACME DNS CNAME for %s: %v", domain, err)
		}
	}

	// Update domain status
	s.db.Exec(`
		UPDATE domains 
		SET dns_auto_configured = true, dns_config_error = NULL, dns_verified = true
		WHERE name = $1 AND user_id = $2
	`, domain, userID)

	return nil
}

// ReconfigureDomain updates DNS for a single domain
func (s *CloudflareService) ReconfigureDomain(ctx context.Context, userID, domainID string) error {
	var zoneID, domainName string
	err := s.db.QueryRow(`
		SELECT provider_domain_id, name FROM domains 
		WHERE id = $1 AND user_id = $2 AND source = 'cloudflare'
	`, domainID, userID).Scan(&zoneID, &domainName)
	if err != nil {
		return fmt.Errorf("domain not found: %w", err)
	}

	return s.setupDNSInternal(ctx, userID, zoneID, domainName)
}

// ReconfigureAllDomains updates DNS for all Cloudflare domains
func (s *CloudflareService) ReconfigureAllDomains(ctx context.Context, serverIP string) ([]ReconfigureResult, error) {
	var results []ReconfigureResult

	rows, err := s.db.Query(`
		SELECT d.id, d.name, d.provider_domain_id, d.user_id
		FROM domains d
		WHERE d.source = 'cloudflare' AND d.provider_domain_id IS NOT NULL
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var domainID, domainName, zoneID, userID string
		if err := rows.Scan(&domainID, &domainName, &zoneID, &userID); err != nil {
			continue
		}

		result := ReconfigureResult{
			Domain: domainName,
		}

		if err := s.setupDNSInternal(ctx, userID, zoneID, domainName); err != nil {
			result.Success = false
			result.Error = err.Error()
		} else {
			result.Success = true
		}

		results = append(results, result)
	}

	return results, nil
}

// registerWithAcmeDNS registers a domain with acme-dns on the Deploy VPS
// and stores the registration on the domain record.
func (s *CloudflareService) registerWithAcmeDNS(userID, domainName string) (*acmedns.Registration, error) {
	var server struct {
		IP       string `db:"ip"`
		Port     int    `db:"port"`
		User     string `db:"ssh_user"`
		Password string `db:"ssh_password"`
	}
	err := s.db.Get(&server, `
		SELECT ip, COALESCE(port, 22) as port, ssh_user, ssh_password
		FROM servers WHERE status = 'ready' ORDER BY created_at LIMIT 1
	`)
	if err != nil {
		return nil, fmt.Errorf("no deploy server found: %w", err)
	}

	client, err := acmedns.NewClient(server.IP, server.Port, server.User, server.Password)
	if err != nil {
		return nil, fmt.Errorf("SSH connection failed: %w", err)
	}
	defer client.Close()

	if !client.IsInstalled() {
		acmeDomain := os.Getenv("ACME_DNS_DOMAIN")
		if acmeDomain == "" {
			return nil, errors.New("ACME_DNS_DOMAIN not set and acme-dns not installed")
		}
		if err := client.Setup(acmeDomain); err != nil {
			return nil, fmt.Errorf("acme-dns setup failed: %w", err)
		}
	}

	reg, err := client.Register()
	if err != nil {
		return nil, fmt.Errorf("acme-dns registration failed: %w", err)
	}

	_, err = s.db.Exec(`
		UPDATE domains SET
			acme_subdomain = $1, acme_username = $2, acme_password = $3,
			acme_fulldomain = $4, updated_at = NOW()
		WHERE name = $5 AND user_id = $6
	`, reg.Subdomain, reg.Username, reg.Password, reg.Fulldomain, domainName, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to store acme-dns credentials: %w", err)
	}
	return reg, nil
}
