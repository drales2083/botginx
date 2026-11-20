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
	"github.com/botginx/botginx/pkg/providers/cloudflare"
	"github.com/botginx/botginx/pkg/sshexec"
	"github.com/jmoiron/sqlx"
)

// CloudflareService handles Cloudflare integration
type CloudflareService struct {
	db                  *sqlx.DB
	encryptionKey       []byte
	domainService       *DomainService
	verificationService *VerificationService
}

// NewCloudflareService creates a new Cloudflare service
func NewCloudflareService(db *sqlx.DB, domainService *DomainService, verificationService *VerificationService) *CloudflareService {
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
		db:                  db,
		encryptionKey:       keyBytes,
		domainService:       domainService,
		verificationService: verificationService,
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

	// Normalize domain name to lowercase (DNS is case-insensitive, nginx uses lowercase)
	zone.Name = strings.ToLower(zone.Name)

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

	// Insert domain (dns_api_user_id tracks who added it via API for DNS updates even if ownership changes)
	err = s.db.QueryRow(`
		INSERT INTO domains (id, user_id, name, verify_token, source, provider_domain_id, dns_auto_configured, is_wildcard, dns_api_user_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, true, $2, NOW(), NOW())
		RETURNING created_at
	`, domain.ID, domain.UserID, domain.Name, domain.VerifyToken, domain.Source, input.ZoneID, domain.DNSAutoConfigured).
		Scan(&domain.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating domain: %w", err)
	}

	// Auto-configure DNS and SSL if requested
	if input.ConfigureDNS {
		go func(domID, uid, zoneID, domainName string) {
			setupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			// Step 1: Set up A records and acme-dns CNAME
			if err := s.setupDNSInternal(setupCtx, uid, zoneID, domainName); err != nil {
				log.Printf("[Cloudflare] DNS setup failed for %s: %v", domainName, err)
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
				// Fallback to slow method using direct Cloudflare TXT records
				log.Printf("[Cloudflare] No acme-dns credentials for %s, using slow TXT method", domainName)
				if err := s.GenerateWildcardSSLSlow(setupCtx, uid, domainName, zoneID); err != nil {
					log.Printf("[Cloudflare] Slow SSL also failed for %s: %v", domainName, err)
					s.db.Exec(`UPDATE domains SET ssl_error = $1 WHERE id = $2`, err.Error(), domID)
				} else {
					log.Printf("[Cloudflare] Wildcard SSL completed for %s (slow method)", domainName)
					s.domainService.CompleteSSLSetup(domID)
				}
				return
			}

			// Step 3: Wait for CNAME propagation (usually 60-120s)
			log.Printf("[Cloudflare] Waiting 90s for CNAME propagation for %s", domainName)
			time.Sleep(90 * time.Second)

			// Step 4: Generate wildcard SSL using acme-dns
			log.Printf("[Cloudflare] Generating wildcard SSL for %s", domainName)
			if err := s.verificationService.GenerateWildcardSSLWithAcmeDNS(domainName, acmeSubdomain, acmeUsername, acmePassword); err != nil {
				log.Printf("[Cloudflare] acme-dns SSL failed for %s: %v, trying slow method", domainName, err)
				// Fallback to slow method
				if err := s.GenerateWildcardSSLSlow(setupCtx, uid, domainName, zoneID); err != nil {
					log.Printf("[Cloudflare] Slow SSL also failed for %s: %v", domainName, err)
					s.db.Exec(`UPDATE domains SET ssl_error = $1 WHERE id = $2`, err.Error(), domID)
				} else {
					log.Printf("[Cloudflare] Wildcard SSL completed for %s (slow fallback)", domainName)
					s.domainService.CompleteSSLSetup(domID)
					// Setup nginx config
					if err := s.setupProxyNginxConfig(domainName); err != nil {
						log.Printf("[Cloudflare] Warning: nginx config failed for %s: %v", domainName, err)
					}
				}
				return
			}

			// Step 5: Complete setup and create nginx config
			log.Printf("[Cloudflare] Wildcard SSL completed for %s", domainName)
			s.domainService.CompleteSSLSetup(domID)
			s.db.Exec(`UPDATE domains SET acme_cname_verified = true WHERE id = $1`, domID)

			// Step 6: Setup nginx config
			if err := s.setupProxyNginxConfig(domainName); err != nil {
				log.Printf("[Cloudflare] Warning: nginx config failed for %s: %v", domainName, err)
			}
		}(domain.ID, userID, input.ZoneID, zone.Name)
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

	// Get deploy server IP (where SSL and traffic are handled)
	serverIP := s.domainService.GetDeployIP()
	if serverIP == "" || serverIP == "No server available" || net.ParseIP(serverIP) == nil {
		return errors.New("no deploy server available - add one in Admin > Servers")
	}

	// Setup A records
	_, _, err = client.SetupDNS(ctx, zoneID, domain, serverIP)
	if err != nil {
		return fmt.Errorf("setting up A records: %w", err)
	}

	// Get domain verify token for TXT record
	var verifyToken string
	s.db.QueryRow(`SELECT verify_token FROM domains WHERE name = $1 AND user_id = $2`, domain, userID).Scan(&verifyToken)

	// Add verification TXT record (_guardbot-verify.domain)
	if verifyToken != "" {
		if _, err := client.AddVerificationTXT(ctx, zoneID, domain, verifyToken); err != nil {
			log.Printf("[Cloudflare] Warning: failed to add verification TXT for %s: %v", domain, err)
		}
	}

	// ALWAYS delete any existing _acme-challenge records (TXT or CNAME) to ensure clean state
	// This must happen regardless of whether acme-dns is configured
	if records, err := client.ListDNSRecords(ctx, zoneID); err == nil {
		for _, r := range records {
			if r.Name == "_acme-challenge."+domain {
				log.Printf("[Cloudflare] Deleting existing %s record for _acme-challenge.%s", r.Type, domain)
				client.DeleteDNSRecord(ctx, zoneID, r.ID)
			}
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
			// Use existing credentials
			fulldomain = existingFulldomain
			log.Printf("[Cloudflare] Using existing acme-dns credentials for %s → %s", domain, fulldomain)
		} else {
			// Register new credentials
			reg, err := s.registerWithAcmeDNS(userID, domain)
			if err != nil {
				log.Printf("[Cloudflare] Warning: acme-dns registration failed for %s: %v", domain, err)
			} else {
				fulldomain = reg.Fulldomain
			}
		}

		// Add CNAME if we have a fulldomain
		if fulldomain != "" {
			if _, err := client.AddAcmeDNSCname(ctx, zoneID, domain, fulldomain); err != nil {
				log.Printf("[Cloudflare] Warning: failed to add ACME DNS CNAME for %s: %v", domain, err)
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

// ReconfigureDomain updates DNS for a single domain and triggers SSL if needed
func (s *CloudflareService) ReconfigureDomain(ctx context.Context, userID, domainID string) error {
	// Get domain with acme-dns credentials
	type domainInfo struct {
		ID                string  `db:"id"`
		Name              string  `db:"name"`
		ZoneID            string  `db:"provider_domain_id"`
		AcmeFulldomain    *string `db:"acme_fulldomain"`
		AcmeSubdomain     *string `db:"acme_subdomain"`
		AcmeUsername      *string `db:"acme_username"`
		AcmePassword      *string `db:"acme_password"`
		AcmeCnameVerified bool    `db:"acme_cname_verified"`
		SSLEnabled        bool    `db:"ssl_enabled"`
	}

	var d domainInfo
	err := s.db.Get(&d, `
		SELECT id, name, provider_domain_id, acme_fulldomain, acme_subdomain,
		       acme_username, acme_password, COALESCE(acme_cname_verified, false) as acme_cname_verified,
		       COALESCE(ssl_enabled, false) as ssl_enabled
		FROM domains
		WHERE id = $1 AND user_id = $2 AND source = 'cloudflare'
	`, domainID, userID)
	if err != nil {
		return fmt.Errorf("domain not found: %w", err)
	}

	// Setup DNS (A records, verification TXT, acme-dns CNAME)
	if err := s.setupDNSInternal(ctx, userID, d.ZoneID, d.Name); err != nil {
		s.db.Exec(`UPDATE domains SET dns_config_error = $1, updated_at = NOW() WHERE id = $2`, err.Error(), d.ID)
		return err
	}

	// Trigger SSL generation if CNAME added but SSL not yet generated
	if !d.SSLEnabled && d.AcmeSubdomain != nil && d.AcmeUsername != nil && d.AcmePassword != nil {
		go func(domID, domainName, subdomain, username, password string) {
			log.Printf("[Cloudflare] ReconfigureDomain: generating SSL for %s", domainName)
			time.Sleep(60 * time.Second) // Wait for CNAME propagation

			if err := s.verificationService.GenerateWildcardSSLWithAcmeDNS(domainName, subdomain, username, password); err != nil {
				log.Printf("[Cloudflare] ReconfigureDomain: SSL failed for %s: %v", domainName, err)
				s.db.Exec(`UPDATE domains SET ssl_error = $1 WHERE id = $2`, err.Error(), domID)
			} else {
				log.Printf("[Cloudflare] ReconfigureDomain: SSL completed for %s", domainName)
				s.domainService.CompleteSSLSetup(domID)
				s.db.Exec(`UPDATE domains SET acme_cname_verified = true WHERE id = $1`, domID)
			}
		}(d.ID, d.Name, *d.AcmeSubdomain, *d.AcmeUsername, *d.AcmePassword)
	}

	return nil
}

// ReconfigureAllDomains updates DNS for all Cloudflare domains
func (s *CloudflareService) ReconfigureAllDomains(ctx context.Context, serverIP string) ([]ReconfigureResult, error) {
	var results []ReconfigureResult

	// Use dns_api_user_id (the user who added the domain via API) for DNS operations
	// This allows domains to be sold/transferred while still using the original API key
	rows, err := s.db.Query(`
		SELECT d.id, d.name, d.provider_domain_id, COALESCE(d.dns_api_user_id, d.user_id) as api_user_id
		FROM domains d
		WHERE d.source = 'cloudflare' AND d.provider_domain_id IS NOT NULL
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var domainID, domainName, zoneID, apiUserID string
		if err := rows.Scan(&domainID, &domainName, &zoneID, &apiUserID); err != nil {
			continue
		}

		result := ReconfigureResult{
			Domain: domainName,
		}

		if err := s.setupDNSInternal(ctx, apiUserID, zoneID, domainName); err != nil {
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

// GenerateWildcardSSLSlow generates a wildcard SSL certificate using Cloudflare DNS for DNS-01 challenge
// This is the slow fallback method that waits 120s for DNS propagation (Cloudflare is faster than NameSilo)
// Prefer acme-dns CNAME method which is instant
func (s *CloudflareService) GenerateWildcardSSLSlow(ctx context.Context, userID, domainName, zoneID string) error {
	// Get the user's Cloudflare credentials
	var encryptedKey, email string
	err := s.db.QueryRow(`SELECT cloudflare_api_key_encrypted, cloudflare_email FROM users WHERE id = $1`, userID).Scan(&encryptedKey, &email)
	if err != nil || encryptedKey == "" {
		return fmt.Errorf("user not connected to Cloudflare")
	}

	apiKey, err := s.decrypt(encryptedKey)
	if err != nil {
		return fmt.Errorf("decrypting API key: %w", err)
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

	log.Printf("[Cloudflare] Generating wildcard SSL (slow method) for *.%s", domainName)

	// Create auth hook that adds TXT record via Cloudflare API
	authHook := fmt.Sprintf(`#!/bin/bash
ZONE_ID="%s"
API_KEY="%s"
EMAIL="%s"

# Delete existing _acme-challenge TXT records
RECORDS=$(curl -s -X GET "https://api.cloudflare.com/client/v4/zones/${ZONE_ID}/dns_records?type=TXT&name=_acme-challenge.%s" \
    -H "X-Auth-Email: ${EMAIL}" \
    -H "X-Auth-Key: ${API_KEY}" \
    -H "Content-Type: application/json")

OLD_IDS=$(echo "$RECORDS" | python3 -c "
import sys,json
try:
    d=json.load(sys.stdin)
    for r in d.get('result',[]):
        if r.get('name','').startswith('_acme-challenge'):
            print(r['id'])
except: pass
" 2>/dev/null)

for id in $OLD_IDS; do
    curl -s -X DELETE "https://api.cloudflare.com/client/v4/zones/${ZONE_ID}/dns_records/${id}" \
        -H "X-Auth-Email: ${EMAIL}" \
        -H "X-Auth-Key: ${API_KEY}" > /dev/null
    sleep 2
done

# Add new TXT record
curl -s -X POST "https://api.cloudflare.com/client/v4/zones/${ZONE_ID}/dns_records" \
    -H "X-Auth-Email: ${EMAIL}" \
    -H "X-Auth-Key: ${API_KEY}" \
    -H "Content-Type: application/json" \
    --data "{\"type\":\"TXT\",\"name\":\"_acme-challenge\",\"content\":\"${CERTBOT_VALIDATION}\",\"ttl\":120}" > /dev/null

# Wait for Cloudflare DNS propagation (usually fast, 60-120s to be safe)
sleep 120
`, zoneID, apiKey, email, domainName)

	// Create cleanup hook
	cleanupHook := fmt.Sprintf(`#!/bin/bash
ZONE_ID="%s"
API_KEY="%s"
EMAIL="%s"

RECORDS=$(curl -s -X GET "https://api.cloudflare.com/client/v4/zones/${ZONE_ID}/dns_records?type=TXT&name=_acme-challenge.%s" \
    -H "X-Auth-Email: ${EMAIL}" \
    -H "X-Auth-Key: ${API_KEY}" \
    -H "Content-Type: application/json")

IDS=$(echo "$RECORDS" | python3 -c "
import sys,json
try:
    d=json.load(sys.stdin)
    for r in d.get('result',[]):
        if r.get('name','').startswith('_acme-challenge'):
            print(r['id'])
except: pass
" 2>/dev/null)

for id in $IDS; do
    curl -s -X DELETE "https://api.cloudflare.com/client/v4/zones/${ZONE_ID}/dns_records/${id}" \
        -H "X-Auth-Email: ${EMAIL}" \
        -H "X-Auth-Key: ${API_KEY}" > /dev/null
done
`, zoneID, apiKey, email, domainName)

	// Write hooks to deploy server
	authPath := fmt.Sprintf("/tmp/cloudflare_auth_%s.sh", domainName)
	cleanupPath := fmt.Sprintf("/tmp/cloudflare_cleanup_%s.sh", domainName)

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
	log.Printf("[Cloudflare] Certbot output for %s: %s", domainName, output)

	// Cleanup hook scripts
	sshClient.Run(fmt.Sprintf("rm -f %s %s", authPath, cleanupPath))

	if err != nil {
		return fmt.Errorf("certbot failed: %w\nOutput: %s", err, output)
	}

	if !strings.Contains(output, "Successfully received certificate") && !strings.Contains(output, "Congratulations") {
		return fmt.Errorf("SSL generation failed: %s", output)
	}

	log.Printf("[Cloudflare] Wildcard SSL generated for *.%s (slow method)", domainName)
	return nil
}

// setupProxyNginxConfig creates nginx config with proper proxy to botection
func (s *CloudflareService) setupProxyNginxConfig(domainName string) error {
	// Get deploy server credentials
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
	log.Printf("[Cloudflare] Created proxy nginx config for %s", baseDomain)
	return nil
}
