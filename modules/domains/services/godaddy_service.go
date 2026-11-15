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
	"os"
	"strings"
	"time"

	"github.com/botginx/botginx/modules/domains/models"
	"github.com/botginx/botginx/pkg/acmedns"
	"github.com/botginx/botginx/pkg/providers/godaddy"
	"github.com/botginx/botginx/pkg/sshexec"
	"github.com/jmoiron/sqlx"
)

// GoDaddyService handles GoDaddy integration
type GoDaddyService struct {
	db                  *sqlx.DB
	encryptionKey       []byte
	domainService       *DomainService
	verificationService *VerificationService
}

// NewGoDaddyService creates a new GoDaddy service
func NewGoDaddyService(db *sqlx.DB, domainService *DomainService, verificationService *VerificationService) *GoDaddyService {
	key := os.Getenv("GODADDY_ENCRYPTION_KEY")
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

	return &GoDaddyService{
		db:                  db,
		encryptionKey:       keyBytes,
		domainService:       domainService,
		verificationService: verificationService,
	}
}

// encrypt encrypts data using AES-256-GCM
func (s *GoDaddyService) encrypt(plaintext string) (string, error) {
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
func (s *GoDaddyService) decrypt(ciphertext string) (string, error) {
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

// GetConnection returns the user's GoDaddy connection status
func (s *GoDaddyService) GetConnection(userID string) (*models.GoDaddyConnection, error) {
	var encryptedKey sql.NullString
	var encryptedSecret sql.NullString
	var connectedAt sql.NullTime

	err := s.db.QueryRow(`
		SELECT godaddy_api_key_encrypted, godaddy_api_secret_encrypted, godaddy_connected_at
		FROM users WHERE id = $1
	`, userID).Scan(&encryptedKey, &encryptedSecret, &connectedAt)
	if err != nil {
		return nil, err
	}

	conn := &models.GoDaddyConnection{
		UserID:      userID,
		IsConnected: encryptedKey.Valid && encryptedKey.String != "" && encryptedSecret.Valid && encryptedSecret.String != "",
	}

	if connectedAt.Valid {
		conn.ConnectedAt = &connectedAt.Time
	}

	return conn, nil
}

// Connect validates and saves the GoDaddy credentials
func (s *GoDaddyService) Connect(ctx context.Context, userID, apiKey, apiSecret string) error {
	// Validate credentials by making a test request
	client := godaddy.NewClient(apiKey, apiSecret)
	if err := client.VerifyCredentials(ctx); err != nil {
		return fmt.Errorf("invalid credentials: %w", err)
	}

	// Encrypt credentials
	encryptedKey, err := s.encrypt(apiKey)
	if err != nil {
		return fmt.Errorf("encrypting API key: %w", err)
	}

	encryptedSecret, err := s.encrypt(apiSecret)
	if err != nil {
		return fmt.Errorf("encrypting API secret: %w", err)
	}

	// Save to database
	_, err = s.db.Exec(`
		UPDATE users
		SET godaddy_api_key_encrypted = $1,
		    godaddy_api_secret_encrypted = $2,
		    godaddy_connected_at = NOW()
		WHERE id = $3
	`, encryptedKey, encryptedSecret, userID)
	if err != nil {
		return fmt.Errorf("saving credentials: %w", err)
	}

	return nil
}

// Disconnect removes the GoDaddy credentials
func (s *GoDaddyService) Disconnect(userID string) error {
	_, err := s.db.Exec(`
		UPDATE users
		SET godaddy_api_key_encrypted = NULL,
		    godaddy_api_secret_encrypted = NULL,
		    godaddy_connected_at = NULL
		WHERE id = $1
	`, userID)
	return err
}

// getClient creates an authenticated GoDaddy client for the user
func (s *GoDaddyService) getClient(userID string) (*godaddy.Client, error) {
	var encryptedKey sql.NullString
	var encryptedSecret sql.NullString

	err := s.db.QueryRow(`
		SELECT godaddy_api_key_encrypted, godaddy_api_secret_encrypted
		FROM users WHERE id = $1
	`, userID).Scan(&encryptedKey, &encryptedSecret)
	if err != nil {
		return nil, err
	}

	if !encryptedKey.Valid || !encryptedSecret.Valid {
		return nil, errors.New("godaddy not connected")
	}

	apiKey, err := s.decrypt(encryptedKey.String)
	if err != nil {
		return nil, fmt.Errorf("decrypting API key: %w", err)
	}

	apiSecret, err := s.decrypt(encryptedSecret.String)
	if err != nil {
		return nil, fmt.Errorf("decrypting API secret: %w", err)
	}

	return godaddy.NewClient(apiKey, apiSecret), nil
}

// ListDomains returns all domains from the user's GoDaddy account
func (s *GoDaddyService) ListDomains(ctx context.Context, userID string) ([]models.GoDaddyDomain, error) {
	client, err := s.getClient(userID)
	if err != nil {
		return nil, err
	}

	gdDomains, err := client.ListDomains(ctx)
	if err != nil {
		return nil, err
	}

	// Get list of already-imported domains
	importedDomains := make(map[string]string)
	rows, err := s.db.Query(`
		SELECT name, id FROM domains
		WHERE user_id = $1 AND source = 'godaddy'
	`, userID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var name, id string
			rows.Scan(&name, &id)
			importedDomains[name] = id
		}
	}

	domains := make([]models.GoDaddyDomain, len(gdDomains))
	for i, d := range gdDomains {
		domains[i] = models.GoDaddyDomain{
			Domain:  d.Domain,
			Status:  d.Status,
			Expires: &d.Expires,
		}
		if localID, ok := importedDomains[d.Domain]; ok {
			domains[i].IsImported = true
			domains[i].LocalID = localID
		}
	}

	return domains, nil
}

// Import imports a GoDaddy domain as a GuardBot domain
func (s *GoDaddyService) Import(ctx context.Context, userID string, input models.GoDaddyImportInput) (*models.Domain, error) {
	client, err := s.getClient(userID)
	if err != nil {
		return nil, err
	}

	// Verify domain exists
	gdDomain, err := client.GetDomain(ctx, input.Domain)
	if err != nil {
		return nil, fmt.Errorf("getting domain: %w", err)
	}

	// Check if domain already exists
	var existingID string
	err = s.db.QueryRow(`SELECT id FROM domains WHERE name = $1 AND user_id = $2`, gdDomain.Domain, userID).Scan(&existingID)
	if err == nil {
		return nil, fmt.Errorf("domain %s already exists", gdDomain.Domain)
	}

	// Create domain in GuardBot
	domain := &models.Domain{
		UserID:            userID,
		Name:              gdDomain.Domain,
		Source:            "godaddy",
		DNSAutoConfigured: input.ConfigureDNS,
		IsWildcard:        true,
		VerifyToken:       fmt.Sprintf("guardbot-%s", generateID(12)),
	}
	domain.ID = fmt.Sprintf("dom_%s", generateID(12))

	// Insert domain
	err = s.db.QueryRow(`
		INSERT INTO domains (id, user_id, name, verify_token, source, dns_auto_configured, is_wildcard, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, true, NOW(), NOW())
		RETURNING created_at
	`, domain.ID, domain.UserID, domain.Name, domain.VerifyToken, domain.Source, domain.DNSAutoConfigured).
		Scan(&domain.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating domain: %w", err)
	}

	// Auto-configure DNS and SSL if requested
	if input.ConfigureDNS {
		go func(domID, uid, domainName string) {
			setupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			// Step 1: Set up A records and acme-dns CNAME
			if err := s.setupDNSInternal(setupCtx, uid, domainName); err != nil {
				log.Printf("[GoDaddy] DNS setup failed for %s: %v", domainName, err)
				s.db.Exec(`UPDATE domains SET dns_config_error = $1 WHERE id = $2`, err.Error(), domID)
				return
			}

			s.db.Exec(`UPDATE domains SET dns_auto_configured = true, dns_verified = true WHERE id = $1`, domID)

			// Step 2: Get acme-dns credentials for SSL generation
			var acmeSubdomain, acmeUsername, acmePassword string
			err := s.db.QueryRow(`
				SELECT COALESCE(acme_subdomain, ''), COALESCE(acme_username, ''), COALESCE(acme_password, '')
				FROM domains WHERE id = $1
			`, domID).Scan(&acmeSubdomain, &acmeUsername, &acmePassword)

			if err != nil || acmeSubdomain == "" {
				// Fallback to slow method using direct GoDaddy TXT records
				log.Printf("[GoDaddy] No acme-dns credentials for %s, using slow TXT method", domainName)
				if err := s.GenerateWildcardSSLSlow(setupCtx, uid, domainName); err != nil {
					log.Printf("[GoDaddy] Slow SSL also failed for %s: %v", domainName, err)
					s.db.Exec(`UPDATE domains SET ssl_error = $1 WHERE id = $2`, err.Error(), domID)
				} else {
					log.Printf("[GoDaddy] Wildcard SSL completed for %s (slow method)", domainName)
					s.domainService.CompleteSSLSetup(domID)
				}
				return
			}

			// Step 3: Wait for CNAME propagation
			log.Printf("[GoDaddy] Waiting 90s for CNAME propagation for %s", domainName)
			time.Sleep(90 * time.Second)

			// Step 4: Generate wildcard SSL using acme-dns
			log.Printf("[GoDaddy] Generating wildcard SSL for %s", domainName)
			if err := s.verificationService.GenerateWildcardSSLWithAcmeDNS(domainName, acmeSubdomain, acmeUsername, acmePassword); err != nil {
				log.Printf("[GoDaddy] acme-dns SSL failed for %s: %v, trying slow method", domainName, err)
				if err := s.GenerateWildcardSSLSlow(setupCtx, uid, domainName); err != nil {
					log.Printf("[GoDaddy] Slow SSL also failed for %s: %v", domainName, err)
					s.db.Exec(`UPDATE domains SET ssl_error = $1 WHERE id = $2`, err.Error(), domID)
				} else {
					log.Printf("[GoDaddy] Wildcard SSL completed for %s (slow fallback)", domainName)
					s.domainService.CompleteSSLSetup(domID)
					if err := s.setupProxyNginxConfig(domainName); err != nil {
						log.Printf("[GoDaddy] Warning: nginx config failed for %s: %v", domainName, err)
					}
				}
				return
			}

			// Step 5: Complete setup
			log.Printf("[GoDaddy] Wildcard SSL completed for %s", domainName)
			s.domainService.CompleteSSLSetup(domID)
			s.db.Exec(`UPDATE domains SET acme_cname_verified = true WHERE id = $1`, domID)

			// Step 6: Setup nginx config
			if err := s.setupProxyNginxConfig(domainName); err != nil {
				log.Printf("[GoDaddy] Warning: nginx config failed for %s: %v", domainName, err)
			}
		}(domain.ID, userID, gdDomain.Domain)
	}

	return domain, nil
}

func (s *GoDaddyService) setupDNSInternal(ctx context.Context, userID, domain string) error {
	client, err := s.getClient(userID)
	if err != nil {
		return err
	}

	// Get deploy server IP (where SSL and traffic are handled)
	serverIP := s.domainService.GetDeployIP()
	if serverIP == "" || serverIP == "No server available" || net.ParseIP(serverIP) == nil {
		return errors.New("no deploy server available - add one in Admin > Servers")
	}

	// Setup A records
	if err := client.SetupDNS(ctx, domain, serverIP); err != nil {
		return fmt.Errorf("setting up A records: %w", err)
	}

	// Get domain verify token for TXT record
	var verifyToken string
	s.db.QueryRow(`SELECT verify_token FROM domains WHERE name = $1 AND user_id = $2`, domain, userID).Scan(&verifyToken)

	// Add verification TXT record
	if verifyToken != "" {
		if err := client.AddVerificationTXT(ctx, domain, verifyToken); err != nil {
			log.Printf("[GoDaddy] Warning: failed to add verification TXT for %s: %v", domain, err)
		}
	}

	// Setup acme-dns CNAME if configured
	acmeDomain := os.Getenv("ACME_DNS_DOMAIN")
	if acmeDomain != "" {
		// Check if domain already has acme-dns credentials
		var existingFulldomain string
		s.db.QueryRow(`SELECT acme_fulldomain FROM domains WHERE name = $1 AND user_id = $2 AND acme_fulldomain IS NOT NULL`,
			domain, userID).Scan(&existingFulldomain)

		var fulldomain string
		if existingFulldomain != "" {
			fulldomain = existingFulldomain
			log.Printf("[GoDaddy] Using existing acme-dns credentials for %s → %s", domain, fulldomain)
		} else {
			reg, err := s.registerWithAcmeDNS(userID, domain)
			if err != nil {
				log.Printf("[GoDaddy] Warning: acme-dns registration failed for %s: %v", domain, err)
			} else {
				fulldomain = reg.Fulldomain
			}
		}

		// Add CNAME if we have a fulldomain
		if fulldomain != "" {
			if err := client.AddAcmeDNSCname(ctx, domain, fulldomain); err != nil {
				log.Printf("[GoDaddy] Warning: failed to add ACME DNS CNAME for %s: %v", domain, err)
			}
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
func (s *GoDaddyService) ReconfigureDomain(ctx context.Context, userID, domainID string) error {
	type domainInfo struct {
		ID                string  `db:"id"`
		Name              string  `db:"name"`
		AcmeFulldomain    *string `db:"acme_fulldomain"`
		AcmeSubdomain     *string `db:"acme_subdomain"`
		AcmeUsername      *string `db:"acme_username"`
		AcmePassword      *string `db:"acme_password"`
		AcmeCnameVerified bool    `db:"acme_cname_verified"`
		SSLEnabled        bool    `db:"ssl_enabled"`
	}

	var d domainInfo
	err := s.db.Get(&d, `
		SELECT id, name, acme_fulldomain, acme_subdomain,
		       acme_username, acme_password, COALESCE(acme_cname_verified, false) as acme_cname_verified,
		       COALESCE(ssl_enabled, false) as ssl_enabled
		FROM domains
		WHERE id = $1 AND user_id = $2 AND source = 'godaddy'
	`, domainID, userID)
	if err != nil {
		return fmt.Errorf("domain not found: %w", err)
	}

	if err := s.setupDNSInternal(ctx, userID, d.Name); err != nil {
		s.db.Exec(`UPDATE domains SET dns_config_error = $1, updated_at = NOW() WHERE id = $2`, err.Error(), d.ID)
		return err
	}

	// Trigger SSL generation if needed
	if !d.SSLEnabled && d.AcmeSubdomain != nil && d.AcmeUsername != nil && d.AcmePassword != nil {
		go func(domID, domainName, subdomain, username, password string) {
			log.Printf("[GoDaddy] ReconfigureDomain: generating SSL for %s", domainName)
			time.Sleep(60 * time.Second)

			if err := s.verificationService.GenerateWildcardSSLWithAcmeDNS(domainName, subdomain, username, password); err != nil {
				log.Printf("[GoDaddy] ReconfigureDomain: SSL failed for %s: %v", domainName, err)
				s.db.Exec(`UPDATE domains SET ssl_error = $1 WHERE id = $2`, err.Error(), domID)
			} else {
				log.Printf("[GoDaddy] ReconfigureDomain: SSL completed for %s", domainName)
				s.domainService.CompleteSSLSetup(domID)
				s.db.Exec(`UPDATE domains SET acme_cname_verified = true WHERE id = $1`, domID)
			}
		}(d.ID, d.Name, *d.AcmeSubdomain, *d.AcmeUsername, *d.AcmePassword)
	}

	return nil
}

// ReconfigureAllDomains updates DNS for all GoDaddy domains
func (s *GoDaddyService) ReconfigureAllDomains(ctx context.Context, serverIP string) ([]ReconfigureResult, error) {
	var results []ReconfigureResult

	rows, err := s.db.Query(`
		SELECT d.id, d.name, d.user_id
		FROM domains d
		WHERE d.source = 'godaddy'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var domainID, domainName, userID string
		if err := rows.Scan(&domainID, &domainName, &userID); err != nil {
			continue
		}

		result := ReconfigureResult{
			Domain: domainName,
		}

		if err := s.setupDNSInternal(ctx, userID, domainName); err != nil {
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
func (s *GoDaddyService) registerWithAcmeDNS(userID, domainName string) (*acmedns.Registration, error) {
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

// GenerateWildcardSSLSlow generates wildcard SSL using GoDaddy DNS for DNS-01 challenge
func (s *GoDaddyService) GenerateWildcardSSLSlow(ctx context.Context, userID, domainName string) error {
	// Get the user's GoDaddy credentials
	var encryptedKey, encryptedSecret string
	err := s.db.QueryRow(`SELECT godaddy_api_key_encrypted, godaddy_api_secret_encrypted FROM users WHERE id = $1`, userID).Scan(&encryptedKey, &encryptedSecret)
	if err != nil || encryptedKey == "" {
		return fmt.Errorf("user not connected to GoDaddy")
	}

	apiKey, err := s.decrypt(encryptedKey)
	if err != nil {
		return fmt.Errorf("decrypting API key: %w", err)
	}

	apiSecret, err := s.decrypt(encryptedSecret)
	if err != nil {
		return fmt.Errorf("decrypting API secret: %w", err)
	}

	// Get deploy server credentials
	var server struct {
		IP       string `db:"ip"`
		Port     int    `db:"port"`
		User     string `db:"ssh_user"`
		Password string `db:"ssh_password"`
	}
	err = s.db.Get(&server, `
		SELECT ip, COALESCE(port, 22) as port, ssh_user, ssh_password
		FROM servers WHERE status = 'ready' ORDER BY created_at LIMIT 1
	`)
	if err != nil {
		return fmt.Errorf("no deploy server available: %w", err)
	}

	port := fmt.Sprintf("%d", server.Port)
	sshClient, err := sshexec.NewClient(server.IP, port, server.User, server.Password)
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer sshClient.Close()

	log.Printf("[GoDaddy] Generating wildcard SSL (slow method) for *.%s", domainName)

	// Create auth hook that adds TXT record via GoDaddy API
	authHook := fmt.Sprintf(`#!/bin/bash
API_KEY="%s"
API_SECRET="%s"
DOMAIN="%s"

# Delete existing _acme-challenge TXT records
curl -s -X DELETE "https://api.godaddy.com/v1/domains/${DOMAIN}/records/TXT/_acme-challenge" \
    -H "Authorization: sso-key ${API_KEY}:${API_SECRET}" > /dev/null 2>&1

sleep 2

# Add new TXT record
curl -s -X PATCH "https://api.godaddy.com/v1/domains/${DOMAIN}/records" \
    -H "Authorization: sso-key ${API_KEY}:${API_SECRET}" \
    -H "Content-Type: application/json" \
    --data "[{\"type\":\"TXT\",\"name\":\"_acme-challenge\",\"data\":\"${CERTBOT_VALIDATION}\",\"ttl\":600}]" > /dev/null

# Wait for GoDaddy DNS propagation (can be slow, 180s to be safe)
sleep 180
`, apiKey, apiSecret, domainName)

	// Create cleanup hook
	cleanupHook := fmt.Sprintf(`#!/bin/bash
API_KEY="%s"
API_SECRET="%s"
DOMAIN="%s"

curl -s -X DELETE "https://api.godaddy.com/v1/domains/${DOMAIN}/records/TXT/_acme-challenge" \
    -H "Authorization: sso-key ${API_KEY}:${API_SECRET}" > /dev/null 2>&1
`, apiKey, apiSecret, domainName)

	// Write hooks to deploy server
	authPath := fmt.Sprintf("/tmp/godaddy_auth_%s.sh", domainName)
	cleanupPath := fmt.Sprintf("/tmp/godaddy_cleanup_%s.sh", domainName)

	sshClient.Run(fmt.Sprintf("cat > %s << 'AUTHEOF'\n%sAUTHEOF\nchmod +x %s", authPath, authHook, authPath))
	sshClient.Run(fmt.Sprintf("cat > %s << 'CLEANEOF'\n%sCLEANEOF\nchmod +x %s", cleanupPath, cleanupHook, cleanupPath))

	// Run certbot for wildcard cert
	certbotCmd := fmt.Sprintf(`certbot certonly --manual --preferred-challenges dns \
		-d "*.%s" -d "%s" \
		--manual-auth-hook %s \
		--manual-cleanup-hook %s \
		--agree-tos --email admin@%s \
		--non-interactive --force-renewal 2>&1`,
		domainName, domainName, authPath, cleanupPath, domainName)

	output, err := sshClient.Run(certbotCmd)
	log.Printf("[GoDaddy] Certbot output for %s: %s", domainName, output)

	// Cleanup hook scripts
	sshClient.Run(fmt.Sprintf("rm -f %s %s", authPath, cleanupPath))

	if err != nil {
		return fmt.Errorf("certbot failed: %w\nOutput: %s", err, output)
	}

	log.Printf("[GoDaddy] Wildcard SSL generated for *.%s (slow method)", domainName)
	return nil
}

// setupProxyNginxConfig creates nginx config with proper proxy to botection
func (s *GoDaddyService) setupProxyNginxConfig(domainName string) error {
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
		return fmt.Errorf("no deploy server available: %w", err)
	}

	port := fmt.Sprintf("%d", server.Port)
	sshClient, err := sshexec.NewClient(server.IP, port, server.User, server.Password)
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer sshClient.Close()

	baseDomain := GetBaseDomain(domainName)

	nginxConf := fmt.Sprintf(`server {
    listen 80;
    server_name %s *.%s;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name %s *.%s;

    ssl_certificate /etc/letsencrypt/live/%s/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/%s/privkey.pem;

    location /.well-known/acme-challenge/ {
        root /var/www/sites/%s;
        allow all;
    }

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}`, baseDomain, baseDomain, baseDomain, baseDomain, baseDomain, baseDomain, baseDomain)

	configPath := fmt.Sprintf("/etc/nginx/sites-available/*.%s.conf", baseDomain)
	writeCmd := fmt.Sprintf("cat > %s << 'NGINXEOF'\n%s\nNGINXEOF", configPath, nginxConf)
	if _, err := sshClient.Run(writeCmd); err != nil {
		return fmt.Errorf("failed to write nginx config: %w", err)
	}

	// Enable site
	linkCmd := fmt.Sprintf("ln -sf %s /etc/nginx/sites-enabled/", configPath)
	sshClient.Run(linkCmd)

	// Test and reload nginx
	testOut, err := sshClient.Run("nginx -t 2>&1")
	if err != nil || strings.Contains(testOut, "failed") {
		sshClient.Run(fmt.Sprintf("rm -f /etc/nginx/sites-enabled/*.%s.conf", baseDomain))
		return fmt.Errorf("nginx config invalid: %s", testOut)
	}

	sshClient.Run("systemctl reload nginx")
	log.Printf("[GoDaddy] Created proxy nginx config for %s", baseDomain)
	return nil
}
