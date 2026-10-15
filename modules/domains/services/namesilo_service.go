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
	"github.com/botginx/botginx/pkg/providers/namesilo"
	"github.com/botginx/botginx/pkg/sshexec"
	"github.com/jmoiron/sqlx"
)

// NameSiloService handles NameSilo integration
type NameSiloService struct {
	db            *sqlx.DB
	encryptionKey []byte
}

// NewNameSiloService creates a new NameSilo service
func NewNameSiloService(db *sqlx.DB) *NameSiloService {
	key := os.Getenv("NAMESILO_ENCRYPTION_KEY")
	if key == "" {
		// Use session secret as fallback
		key = os.Getenv("SESSION_SECRET")
	}

	// Ensure key is 32 bytes for AES-256
	keyBytes := []byte(key)
	if len(keyBytes) < 32 {
		// Pad with zeros
		padded := make([]byte, 32)
		copy(padded, keyBytes)
		keyBytes = padded
	} else if len(keyBytes) > 32 {
		keyBytes = keyBytes[:32]
	}

	return &NameSiloService{
		db:            db,
		encryptionKey: keyBytes,
	}
}

// GetConnection returns the user's NameSilo connection status
func (s *NameSiloService) GetConnection(userID string) (*models.NameSiloConnection, error) {
	var encryptedKey sql.NullString
	var connectedAt sql.NullTime

	err := s.db.QueryRow(`
		SELECT namesilo_api_key_encrypted, namesilo_connected_at
		FROM users WHERE id = $1
	`, userID).Scan(&encryptedKey, &connectedAt)
	if err != nil {
		return nil, err
	}

	conn := &models.NameSiloConnection{
		UserID:      userID,
		IsConnected: encryptedKey.Valid && encryptedKey.String != "",
	}

	if connectedAt.Valid {
		conn.ConnectedAt = &connectedAt.Time
	}

	return conn, nil
}

// Connect stores the API key and tests the connection
func (s *NameSiloService) Connect(ctx context.Context, userID, apiKey string) error {
	// Test the API key first
	client := namesilo.NewClient(apiKey)
	if err := client.TestConnection(ctx); err != nil {
		var apiErr *namesilo.APIError
		if errors.As(err, &apiErr) && apiErr.IsInvalidKey() {
			return errors.New("invalid API key")
		}
		return fmt.Errorf("connection test failed: %w", err)
	}

	// Encrypt the API key
	encrypted, err := s.encrypt(apiKey)
	if err != nil {
		return fmt.Errorf("encrypting API key: %w", err)
	}

	// Store in database
	_, err = s.db.Exec(`
		UPDATE users
		SET namesilo_api_key_encrypted = $1, namesilo_connected_at = $2
		WHERE id = $3
	`, encrypted, time.Now(), userID)

	return err
}

// Disconnect removes the API key
func (s *NameSiloService) Disconnect(userID string) error {
	_, err := s.db.Exec(`
		UPDATE users
		SET namesilo_api_key_encrypted = NULL, namesilo_connected_at = NULL
		WHERE id = $1
	`, userID)
	return err
}

// GetClient returns a NameSilo client for the user
func (s *NameSiloService) GetClient(userID string) (*namesilo.Client, error) {
	var encryptedKey sql.NullString
	err := s.db.QueryRow(`
		SELECT namesilo_api_key_encrypted FROM users WHERE id = $1
	`, userID).Scan(&encryptedKey)
	if err != nil {
		return nil, err
	}

	if !encryptedKey.Valid || encryptedKey.String == "" {
		return nil, errors.New("not connected to NameSilo")
	}

	apiKey, err := s.decrypt(encryptedKey.String)
	if err != nil {
		return nil, fmt.Errorf("decrypting API key: %w", err)
	}

	return namesilo.NewClient(apiKey), nil
}

// ListDomains returns all domains from the user's NameSilo account
func (s *NameSiloService) ListDomains(ctx context.Context, userID string) ([]models.NameSiloDomain, error) {
	client, err := s.GetClient(userID)
	if err != nil {
		return nil, err
	}

	// Get domain list
	domainNames, err := client.ListDomains(ctx)
	if err != nil {
		return nil, err
	}

	// Get existing domains in our system for this user
	existingDomains := make(map[string]string) // domain -> local ID
	rows, err := s.db.Query(`
		SELECT id, name FROM domains
		WHERE user_id = $1 AND source = 'namesilo'
	`, userID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, name string
			rows.Scan(&id, &name)
			existingDomains[strings.ToLower(name)] = id
		}
	}

	// Build result with domain info
	var domains []models.NameSiloDomain
	for _, name := range domainNames {
		domain := models.NameSiloDomain{
			Domain: name,
			Status: "Active",
		}

		// Check if already imported
		if localID, ok := existingDomains[strings.ToLower(name)]; ok {
			domain.IsImported = true
			domain.LocalID = localID
		}

		// Get detailed info (optional, can be slow due to rate limiting)
		info, err := client.GetDomainInfo(ctx, name)
		if err == nil {
			domain.Created = info.Created
			domain.Expires = info.Expires
			domain.Status = info.Status
			domain.Locked = info.Locked
			domain.Private = info.Private
			domain.AutoRenew = info.AutoRenew
		}

		domains = append(domains, domain)
	}

	return domains, nil
}

// ImportDomain imports a domain from NameSilo to our system
func (s *NameSiloService) ImportDomain(ctx context.Context, userID string, input models.NameSiloImportInput) (string, error) {
	client, err := s.GetClient(userID)
	if err != nil {
		return "", err
	}

	// Verify domain exists in user's NameSilo account
	info, err := client.GetDomainInfo(ctx, input.Domain)
	if err != nil {
		return "", fmt.Errorf("domain not found in NameSilo account: %w", err)
	}

	// Check if already imported
	var existingID string
	err = s.db.QueryRow(`
		SELECT id FROM domains
		WHERE user_id = $1 AND LOWER(name) = LOWER($2)
	`, userID, input.Domain).Scan(&existingID)
	if err == nil {
		return existingID, errors.New("domain already imported")
	}

	// Generate domain ID and verify token
	domainID := fmt.Sprintf("dom_%s", generateID(12))
	verifyToken := fmt.Sprintf("guardbot-%s", generateID(12))
	serverID := s.getDeployServerID()

	// Insert domain as wildcard (redirect links use random subdomains)
	// NameSilo-imported domains need wildcard SSL via DNS-01 challenge
	_, err = s.db.Exec(`
		INSERT INTO domains (id, user_id, name, verify_token, server_id, source, provider_domain_id, is_wildcard, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), 'namesilo', $6, true, $7, $7)
	`, domainID, userID, input.Domain, verifyToken, serverID, input.Domain, time.Now())
	if err != nil {
		return "", fmt.Errorf("inserting domain: %w", err)
	}

	// Auto-configure DNS if requested
	if input.ConfigureDNS {
		serverIP, err := s.getDeployServerIP()
		if err != nil {
			// Log but don't fail the import
			s.db.Exec(`
				UPDATE domains SET dns_config_error = $1 WHERE id = $2
			`, err.Error(), domainID)
		} else {
			// Add A records (root + wildcard)
			_, _, err = client.SetupDNS(ctx, input.Domain, serverIP)
			if err != nil {
				s.db.Exec(`
					UPDATE domains SET dns_config_error = $1 WHERE id = $2
				`, err.Error(), domainID)
			} else {
				// Add verification TXT record
				_, txtErr := client.AddVerificationTXT(ctx, input.Domain, verifyToken)
				if txtErr != nil {
					// Log TXT error but don't fail - A records are more important
					s.db.Exec(`
						UPDATE domains SET dns_config_error = $1 WHERE id = $2
					`, "A records OK, TXT failed: "+txtErr.Error(), domainID)
				}

				s.db.Exec(`
					UPDATE domains SET dns_auto_configured = true WHERE id = $1
				`, domainID)

				// Generate wildcard SSL in background (takes ~7 minutes due to DNS propagation)
				go func(uid, domain, domID string) {
					log.Printf("[NameSilo] Starting background wildcard SSL generation for %s", domain)
					bgCtx := context.Background()
					if err := s.GenerateWildcardSSL(bgCtx, uid, domain); err != nil {
						log.Printf("[NameSilo] Wildcard SSL failed for %s: %v", domain, err)
						s.db.Exec(`UPDATE domains SET ssl_error = $1 WHERE id = $2`, err.Error(), domID)
					} else {
						log.Printf("[NameSilo] Wildcard SSL completed for %s", domain)
						s.db.Exec(`UPDATE domains SET ssl_enabled = true, ssl_error = NULL WHERE id = $1`, domID)
					}
				}(userID, input.Domain, domainID)
			}
		}
	}

	_ = info // suppress unused warning
	return domainID, nil
}

// SetupDNS configures DNS records for a domain
func (s *NameSiloService) SetupDNS(ctx context.Context, userID, domainName string) error {
	client, err := s.GetClient(userID)
	if err != nil {
		return err
	}

	serverIP, err := s.getDeployServerIP()
	if err != nil {
		return fmt.Errorf("getting server IP: %w", err)
	}

	_, _, err = client.SetupDNS(ctx, domainName, serverIP)
	if err != nil {
		// Update error in database
		s.db.Exec(`
			UPDATE domains SET dns_config_error = $1, dns_auto_configured = false
			WHERE user_id = $2 AND LOWER(name) = LOWER($3)
		`, err.Error(), userID, domainName)
		return err
	}

	// Mark as configured
	s.db.Exec(`
		UPDATE domains SET dns_auto_configured = true, dns_config_error = NULL
		WHERE user_id = $1 AND LOWER(name) = LOWER($2)
	`, userID, domainName)

	return nil
}

// GetDNSRecords returns current DNS records for a domain
func (s *NameSiloService) GetDNSRecords(ctx context.Context, userID, domainName string) ([]models.NameSiloDNSRecord, error) {
	client, err := s.GetClient(userID)
	if err != nil {
		return nil, err
	}

	records, err := client.ListDNSRecords(ctx, domainName)
	if err != nil {
		return nil, err
	}

	var result []models.NameSiloDNSRecord
	for _, r := range records {
		result = append(result, models.NameSiloDNSRecord{
			RecordID: r.RecordID,
			Type:     r.Type,
			Host:     r.Host,
			Value:    r.Value,
			TTL:      r.TTL,
			Distance: r.Distance,
		})
	}

	return result, nil
}

// ReconfigureResult holds the result of a domain reconfiguration
type ReconfigureResult struct {
	Domain  string `json:"domain"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// ReconfigureAllDomains updates DNS for all NameSilo domains with auto-configured DNS
// This is called when the deploy server IP changes
func (s *NameSiloService) ReconfigureAllDomains(ctx context.Context, newIP string) ([]ReconfigureResult, error) {
	// Find all domains with source='namesilo' and dns_auto_configured=true
	type domainInfo struct {
		ID          string `db:"id"`
		Name        string `db:"name"`
		UserID      string `db:"user_id"`
		VerifyToken string `db:"verify_token"`
	}

	var domains []domainInfo
	err := s.db.Select(&domains, `
		SELECT id, name, user_id, verify_token FROM domains
		WHERE source = 'namesilo' AND dns_auto_configured = true
	`)
	if err != nil {
		return nil, fmt.Errorf("querying domains: %w", err)
	}

	if len(domains) == 0 {
		return []ReconfigureResult{}, nil
	}

	// Group domains by user (each user has their own API key)
	userDomains := make(map[string][]domainInfo)
	for _, d := range domains {
		userDomains[d.UserID] = append(userDomains[d.UserID], d)
	}

	var results []ReconfigureResult

	// Process each user's domains
	for userID, userDoms := range userDomains {
		client, err := s.GetClient(userID)
		if err != nil {
			// Mark all domains for this user as failed
			for _, d := range userDoms {
				results = append(results, ReconfigureResult{
					Domain:  d.Name,
					Success: false,
					Error:   "failed to get API client: " + err.Error(),
				})
			}
			continue
		}

		// Reconfigure each domain
		for _, d := range userDoms {
			_, _, err := client.SetupDNS(ctx, d.Name, newIP)
			if err != nil {
				results = append(results, ReconfigureResult{
					Domain:  d.Name,
					Success: false,
					Error:   err.Error(),
				})
				s.db.Exec(`
					UPDATE domains SET dns_config_error = $1, updated_at = NOW()
					WHERE id = $2
				`, err.Error(), d.ID)
			} else {
				// Also ensure TXT verification record exists
				if d.VerifyToken != "" {
					client.AddVerificationTXT(ctx, d.Name, d.VerifyToken)
				}
				results = append(results, ReconfigureResult{
					Domain:  d.Name,
					Success: true,
				})
				s.db.Exec(`
					UPDATE domains SET dns_config_error = NULL, updated_at = NOW()
					WHERE id = $1
				`, d.ID)
			}
		}
	}

	return results, nil
}

// ReconfigureUserDomains updates DNS for all of a user's NameSilo domains
func (s *NameSiloService) ReconfigureUserDomains(ctx context.Context, userID string) ([]ReconfigureResult, error) {
	serverIP, err := s.getDeployServerIP()
	if err != nil {
		return nil, fmt.Errorf("getting server IP: %w", err)
	}

	// Find all domains for this user with auto-configured DNS
	type domainInfo struct {
		ID          string `db:"id"`
		Name        string `db:"name"`
		VerifyToken string `db:"verify_token"`
	}

	var domains []domainInfo
	err = s.db.Select(&domains, `
		SELECT id, name, verify_token FROM domains
		WHERE user_id = $1 AND source = 'namesilo' AND dns_auto_configured = true
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("querying domains: %w", err)
	}

	if len(domains) == 0 {
		return []ReconfigureResult{}, nil
	}

	client, err := s.GetClient(userID)
	if err != nil {
		return nil, fmt.Errorf("getting API client: %w", err)
	}

	var results []ReconfigureResult
	for _, d := range domains {
		_, _, err := client.SetupDNS(ctx, d.Name, serverIP)
		if err != nil {
			results = append(results, ReconfigureResult{
				Domain:  d.Name,
				Success: false,
				Error:   err.Error(),
			})
			s.db.Exec(`UPDATE domains SET dns_config_error = $1, updated_at = NOW() WHERE id = $2`, err.Error(), d.ID)
		} else {
			// Also ensure TXT verification record exists
			if d.VerifyToken != "" {
				client.AddVerificationTXT(ctx, d.Name, d.VerifyToken)
			}
			results = append(results, ReconfigureResult{
				Domain:  d.Name,
				Success: true,
			})
			s.db.Exec(`UPDATE domains SET dns_config_error = NULL, updated_at = NOW() WHERE id = $1`, d.ID)
		}
	}

	return results, nil
}

// getDeployServerIP returns the IP of the deploy server (where SSL/traffic goes)
// This is different from the panel server - DNS should point to deploy server
func (s *NameSiloService) getDeployServerIP() (string, error) {
	// First check environment variable override
	if ip := os.Getenv("DEPLOY_SERVER_IP"); ip != "" {
		return ip, nil
	}

	// Query the deploy server from database
	var ip string
	err := s.db.QueryRow(`
		SELECT ip FROM servers
		WHERE status = 'ready'
		ORDER BY created_at LIMIT 1
	`).Scan(&ip)
	if err == nil && ip != "" {
		return ip, nil
	}

	// Fallback to environment variable (legacy)
	if ip := os.Getenv("SERVER_PUBLIC_IP"); ip != "" {
		return ip, nil
	}

	// Last resort: auto-detect (not recommended for production)
	resp, err := http.Get("https://api.ipify.org")
	if err != nil {
		resp, err = http.Get("https://ifconfig.me/ip")
		if err != nil {
			return "", fmt.Errorf("could not determine server IP: %w", err)
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	ip = strings.TrimSpace(string(body))
	if net.ParseIP(ip) == nil {
		return "", fmt.Errorf("invalid IP response: %s", ip)
	}

	return ip, nil
}

// getDeployServerID returns the ID of the deploy server
func (s *NameSiloService) getDeployServerID() string {
	var id string
	s.db.QueryRow(`
		SELECT id FROM servers
		WHERE status = 'ready'
		ORDER BY created_at LIMIT 1
	`).Scan(&id)
	return id
}

// encrypt encrypts a string using AES-256-GCM
func (s *NameSiloService) encrypt(plaintext string) (string, error) {
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

// decrypt decrypts a string encrypted with encrypt()
func (s *NameSiloService) decrypt(encrypted string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
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

	if len(ciphertext) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// generateID generates a random alphanumeric ID
func generateID(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	rand.Read(b)
	for i := range b {
		b[i] = chars[b[i]%byte(len(chars))]
	}
	return string(b)
}

// GenerateWildcardSSL generates a wildcard SSL certificate using NameSilo DNS for DNS-01 challenge
// This runs certbot with hooks that call NameSilo API directly via curl on the deploy server
func (s *NameSiloService) GenerateWildcardSSL(ctx context.Context, userID, domainName string) error {
	// Get the user's NameSilo API key
	var encryptedKey string
	err := s.db.QueryRow(`SELECT namesilo_api_key_encrypted FROM users WHERE id = $1`, userID).Scan(&encryptedKey)
	if err != nil || encryptedKey == "" {
		return fmt.Errorf("user not connected to NameSilo")
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
		SELECT ip, port, ssh_user, ssh_password FROM servers
		WHERE status = 'ready' ORDER BY created_at LIMIT 1
	`)
	if err != nil {
		return fmt.Errorf("no deploy server available: %w", err)
	}

	port := fmt.Sprintf("%d", server.Port)
	if server.Port == 0 {
		port = "22"
	}

	sshClient, err := sshexec.NewClient(server.IP, port, server.User, server.Password)
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer sshClient.Close()

	log.Printf("[NameSilo] Generating wildcard SSL for *.%s", domainName)

	// Create auth hook that adds TXT record via NameSilo API
	// Note: We escape $ for shell but not for Go's Sprintf
	authHook := fmt.Sprintf(`#!/bin/bash
DOMAIN="%s"
API_KEY="%s"

# Delete existing _acme-challenge TXT records
RECORDS=$(curl -s "https://www.namesilo.com/api/dnsListRecords?version=1&type=json&key=${API_KEY}&domain=${DOMAIN}")
OLD_IDS=$(echo "$RECORDS" | python3 -c "
import sys,json
try:
    d=json.load(sys.stdin)
    recs=d.get('reply',{}).get('resource_record',[])
    recs=[recs] if isinstance(recs,dict) else (recs or [])
    for r in recs:
        if r.get('host')=='_acme-challenge' and r.get('type')=='TXT':
            print(r['record_id'])
except: pass
" 2>/dev/null)

for id in $OLD_IDS; do
    curl -s "https://www.namesilo.com/api/dnsDeleteRecord?version=1&type=json&key=${API_KEY}&domain=${DOMAIN}&rrid=${id}" > /dev/null
    sleep 3
done

# Add new TXT record
curl -s "https://www.namesilo.com/api/dnsAddRecord?version=1&type=json&key=${API_KEY}&domain=${DOMAIN}&rrtype=TXT&rrhost=_acme-challenge&rrvalue=${CERTBOT_VALIDATION}&rrttl=3600" > /dev/null

# Wait for NameSilo DNS propagation (they're slow - need 120-180s)
sleep 180
`, domainName, apiKey)

	// Create cleanup hook
	cleanupHook := fmt.Sprintf(`#!/bin/bash
DOMAIN="%s"
API_KEY="%s"

RECORDS=$(curl -s "https://www.namesilo.com/api/dnsListRecords?version=1&type=json&key=${API_KEY}&domain=${DOMAIN}")
IDS=$(echo "$RECORDS" | python3 -c "
import sys,json
try:
    d=json.load(sys.stdin)
    recs=d.get('reply',{}).get('resource_record',[])
    recs=[recs] if isinstance(recs,dict) else (recs or [])
    for r in recs:
        if r.get('host')=='_acme-challenge' and r.get('type')=='TXT':
            print(r['record_id'])
except: pass
" 2>/dev/null)

for id in $IDS; do
    curl -s "https://www.namesilo.com/api/dnsDeleteRecord?version=1&type=json&key=${API_KEY}&domain=${DOMAIN}&rrid=${id}" > /dev/null
done
`, domainName, apiKey)

	// Write hooks to deploy server
	authPath := fmt.Sprintf("/tmp/namesilo_auth_%s.sh", domainName)
	cleanupPath := fmt.Sprintf("/tmp/namesilo_cleanup_%s.sh", domainName)

	sshClient.Run(fmt.Sprintf("cat > %s << 'AUTHEOF'\n%sAUTHEOF\nchmod +x %s", authPath, authHook, authPath))
	sshClient.Run(fmt.Sprintf("cat > %s << 'CLEANEOF'\n%sCLEANEOF\nchmod +x %s", cleanupPath, cleanupHook, cleanupPath))

	// Run certbot for wildcard cert
	// Note: This will take ~6-7 minutes (180s wait × 2 domains)
	certbotCmd := fmt.Sprintf(`certbot certonly --manual --preferred-challenges dns \
		-d "*.%s" -d "%s" \
		--manual-auth-hook %s \
		--manual-cleanup-hook %s \
		--agree-tos --email admin@%s \
		--non-interactive --force-renewal 2>&1`,
		domainName, domainName, authPath, cleanupPath, domainName)

	output, err := sshClient.Run(certbotCmd)
	if err != nil {
		return fmt.Errorf("certbot failed: %w\nOutput: %s", err, output)
	}

	if !strings.Contains(output, "Successfully received certificate") {
		return fmt.Errorf("certbot did not succeed: %s", output)
	}

	// Create nginx config for wildcard domain
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
}`, domainName, domainName, domainName, domainName, domainName, domainName, domainName)

	confPath := fmt.Sprintf("/etc/nginx/sites-available/%s.conf", domainName)
	sshClient.Run(fmt.Sprintf("cat > %s << 'NGINXEOF'\n%sNGINXEOF", confPath, nginxConf))
	sshClient.Run(fmt.Sprintf("ln -sf %s /etc/nginx/sites-enabled/", confPath))

	// Test and reload nginx
	testOut, err := sshClient.Run("nginx -t 2>&1")
	if err != nil || strings.Contains(testOut, "failed") {
		return fmt.Errorf("nginx config invalid: %s", testOut)
	}
	sshClient.Run("systemctl reload nginx")

	log.Printf("[NameSilo] Wildcard SSL generated successfully for *.%s", domainName)
	return nil
}
