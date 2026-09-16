package domainsync

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/botginx/botginx/pkg/ssh"
	"github.com/jmoiron/sqlx"
)

const (
	SyncStatusPending       = "pending"
	SyncStatusSyncing       = "syncing"
	SyncStatusDNSWaiting    = "dns_waiting"
	SyncStatusSSLGenerating = "ssl_generating"
	SyncStatusActive        = "active"
	SyncStatusError         = "error"
)

// Domain represents a domain with server assignment
type Domain struct {
	ID              string  `db:"id"`
	Name            string  `db:"name"`
	ServerID        *string `db:"server_id"`
	SyncStatus      string  `db:"sync_status"`
	SSLEnabled      bool    `db:"ssl_enabled"`
	AcmeSubdomain   *string `db:"acme_subdomain"`
	AcmeUsername    *string `db:"acme_username"`
	AcmePassword    *string `db:"acme_password"`
	AcmeFulldomain  *string `db:"acme_fulldomain"`
	IsWildcard      bool    `db:"is_wildcard"`
}

// Server represents a deploy server
type Server struct {
	ID       string `db:"id"`
	IP       string `db:"ip"`
	Port     int    `db:"port"`
	SSHUser  string `db:"ssh_user"`
	Password string `db:"ssh_password"`
}

// RedirectLink for redeployment
type RedirectLink struct {
	ID        string `db:"id"`
	Subdomain string `db:"subdomain"`
}

// ShortLink for redeployment
type ShortLink struct {
	ID   string `db:"id"`
	Path string `db:"path"`
}

// LinkRedeployer handles redeploying links to servers
type LinkRedeployer interface {
	RedeployRedirectLink(linkID string, server Server) error
	RedeployShortLink(linkID string, server Server) error
}

// SSLGenerator handles SSL certificate generation
type SSLGenerator interface {
	GenerateWildcardSSL(domain Domain, server Server) error
}

// Service handles automatic domain synchronization
type Service struct {
	db             *sqlx.DB
	interval       time.Duration
	linkRedeployer LinkRedeployer
	sslGenerator   SSLGenerator
	mu             sync.Mutex
	running        bool
	stopCh         chan struct{}
}

// NewService creates a new domain sync service
func NewService(db *sqlx.DB, interval time.Duration) *Service {
	return &Service{
		db:       db,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// SetLinkRedeployer sets the link redeployer
func (s *Service) SetLinkRedeployer(r LinkRedeployer) {
	s.linkRedeployer = r
}

// SetSSLGenerator sets the SSL generator
func (s *Service) SetSSLGenerator(g SSLGenerator) {
	s.sslGenerator = g
}

// Start begins the periodic sync loop
func (s *Service) Start(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.mu.Unlock()

	log.Println("[DomainSync] Starting automatic domain sync service")

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Run once immediately
	s.syncAll()

	for {
		select {
		case <-ticker.C:
			s.syncAll()
		case <-s.stopCh:
			log.Println("[DomainSync] Stopping sync service")
			return
		case <-ctx.Done():
			log.Println("[DomainSync] Context cancelled, stopping")
			return
		}
	}
}

// Stop halts the sync loop
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		close(s.stopCh)
		s.running = false
	}
}

// syncAll checks all domains with servers assigned
func (s *Service) syncAll() {
	log.Println("[DomainSync] Running sync cycle")

	domains, err := s.getDomainsWithServers()
	if err != nil {
		log.Printf("[DomainSync] Error fetching domains: %v", err)
		return
	}

	for _, domain := range domains {
		if err := s.syncDomain(domain); err != nil {
			log.Printf("[DomainSync] Error syncing domain %s: %v", domain.Name, err)
			s.updateSyncStatus(domain.ID, SyncStatusError, err.Error())
		}
	}

	log.Printf("[DomainSync] Sync cycle complete, checked %d domains", len(domains))
}

// getDomainsWithServers returns all domains that have a server assigned
func (s *Service) getDomainsWithServers() ([]Domain, error) {
	var domains []Domain
	err := s.db.Select(&domains, `
		SELECT id, name, server_id, COALESCE(sync_status, 'pending') as sync_status,
		       ssl_enabled, acme_subdomain, acme_username, acme_password, acme_fulldomain, is_wildcard
		FROM domains
		WHERE server_id IS NOT NULL
	`)
	return domains, err
}

// getServer returns server details by ID
func (s *Service) getServer(serverID string) (*Server, error) {
	var server Server
	err := s.db.Get(&server, `
		SELECT id, ip, port, ssh_user, ssh_password
		FROM servers
		WHERE id = $1 AND status = 'ready'
	`, serverID)
	if err != nil {
		return nil, err
	}
	return &server, nil
}

// syncDomain syncs a single domain
func (s *Service) syncDomain(domain Domain) error {
	if domain.ServerID == nil {
		return nil
	}

	server, err := s.getServer(*domain.ServerID)
	if err != nil {
		return fmt.Errorf("server not found or not ready: %w", err)
	}

	// FIRST: Check if domain config exists on VPS server
	// If config exists, domain is working - don't touch status
	configExists, err := s.checkDomainConfigExists(domain, *server)
	if err != nil {
		log.Printf("[DomainSync] Domain %s config check failed: %v", domain.Name, err)
		return nil // Don't change status on check failure
	}

	if configExists {
		// Domain config exists on VPS - it's working, don't touch anything
		return nil
	}

	// Config doesn't exist - domain needs setup
	log.Printf("[DomainSync] Domain %s config missing on server, starting setup", domain.Name)
	s.updateSyncStatus(domain.ID, SyncStatusSyncing, "")

	// For wildcard domains, ensure acme-dns credentials are registered
	if domain.IsWildcard {
		if err := s.ensureAcmeDnsRegistration(&domain, *server); err != nil {
			log.Printf("[DomainSync] Warning: acme-dns registration failed for %s: %v", domain.Name, err)
		}
	}

	// Check DNS records
	dnsOK, missingRecords := s.checkDNSRecords(domain, server.IP)
	if !dnsOK {
		log.Printf("[DomainSync] Domain %s waiting for DNS: %v", domain.Name, missingRecords)
		s.updateSyncStatus(domain.ID, SyncStatusDNSWaiting, "Missing: "+strings.Join(missingRecords, ", "))
		return nil
	}

	// Step 3: Check/Generate SSL
	if !domain.SSLEnabled {
		log.Printf("[DomainSync] Domain %s DNS verified, checking SSL", domain.Name)

		sslExists, err := s.checkSSLExists(domain, *server)
		if err != nil {
			return fmt.Errorf("failed to check SSL: %w", err)
		}

		if !sslExists {
			s.updateSyncStatus(domain.ID, SyncStatusSSLGenerating, "")

			if err := s.generateSSL(domain, *server); err != nil {
				return fmt.Errorf("failed to generate SSL: %w", err)
			}
		}

		// Mark SSL enabled
		s.db.Exec(`UPDATE domains SET ssl_enabled = TRUE, updated_at = NOW() WHERE id = $1`, domain.ID)
	}

	// All good - mark active
	s.updateSyncStatus(domain.ID, SyncStatusActive, "")
	return nil
}

// AcmeDnsRegisterResponse is the response from acme-dns register endpoint
type AcmeDnsRegisterResponse struct {
	Subdomain  string `json:"subdomain"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	Fulldomain string `json:"fulldomain"`
}

// ensureAcmeDnsRegistration ensures acme-dns credentials are valid for this server
func (s *Service) ensureAcmeDnsRegistration(domain *Domain, server Server) error {
	// Only for wildcard domains
	if !domain.IsWildcard {
		return nil
	}

	baseDomain := domain.Name
	if strings.HasPrefix(baseDomain, "*.") {
		baseDomain = baseDomain[2:]
	}

	// FIRST: Check if user has CNAME set up - this is the source of truth
	// If CNAME exists pointing to acme-dns, NEVER register new credentials
	cname, cnameErr := net.LookupCNAME("_acme-challenge." + baseDomain)
	cnamePointsToAcmeDns := cnameErr == nil && strings.HasSuffix(strings.ToLower(cname), ".acme.pamach.xyz.")

	if cnamePointsToAcmeDns {
		// User has CNAME set up - extract the subdomain they're using
		existingFulldomain := strings.TrimSuffix(cname, ".")
		parts := strings.Split(existingFulldomain, ".")
		if len(parts) >= 1 {
			existingSubdomain := parts[0]

			// Check if our stored subdomain matches what CNAME points to
			if domain.AcmeSubdomain != nil && *domain.AcmeSubdomain == existingSubdomain {
				// Subdomain matches - test if credentials work on this server
				if domain.AcmeUsername != nil && domain.AcmePassword != nil {
					if s.testAcmeDnsCredentials(server, *domain.AcmeSubdomain, *domain.AcmeUsername, *domain.AcmePassword) {
						log.Printf("[DomainSync] Domain %s CNAME verified, credentials work", domain.Name)
						return nil
					}
				}
				// Credentials don't work on this server - adopt the subdomain with new creds
				log.Printf("[DomainSync] Domain %s CNAME verified but credentials invalid on server, adopting subdomain", domain.Name)
				return s.adoptAcmeDnsSubdomain(domain, server, existingFulldomain)
			} else {
				// CNAME points to different subdomain than we have stored - adopt it
				log.Printf("[DomainSync] Domain %s CNAME points to %s but we have %v stored, adopting", domain.Name, existingSubdomain, domain.AcmeSubdomain)
				return s.adoptAcmeDnsSubdomain(domain, server, existingFulldomain)
			}
		}
	}

	// No CNAME detected - but could be DNS lookup failure
	// If we already have credentials with a fulldomain, assume CNAME still exists
	// and DON'T register new credentials (would break user's existing CNAME)
	if domain.AcmeFulldomain != nil && *domain.AcmeFulldomain != "" {
		// We have stored credentials - DNS lookup may have failed transiently
		// Test if credentials work on this server
		if domain.AcmeUsername != nil && domain.AcmePassword != nil && domain.AcmeSubdomain != nil {
			if s.testAcmeDnsCredentials(server, *domain.AcmeSubdomain, *domain.AcmeUsername, *domain.AcmePassword) {
				log.Printf("[DomainSync] Domain %s credentials work (DNS lookup may have failed)", domain.Name)
				return nil
			}
			// Credentials don't work - try to adopt the stored subdomain on this server
			log.Printf("[DomainSync] Domain %s has stored fulldomain %s, adopting on this server", domain.Name, *domain.AcmeFulldomain)
			return s.adoptAcmeDnsSubdomain(domain, server, *domain.AcmeFulldomain)
		}
	}

	// No stored credentials at all - safe to register new
	if domain.AcmeSubdomain == nil || domain.AcmeUsername == nil || domain.AcmePassword == nil {
		log.Printf("[DomainSync] Domain %s has no acme-dns credentials, registering new", domain.Name)
		// Fall through to registration below
	} else {
		// We have partial credentials but no fulldomain - test if they work
		if s.testAcmeDnsCredentials(server, *domain.AcmeSubdomain, *domain.AcmeUsername, *domain.AcmePassword) {
			log.Printf("[DomainSync] Domain %s credentials work, waiting for CNAME setup", domain.Name)
			return nil
		}
		// Credentials don't work and no fulldomain stored - register new
		log.Printf("[DomainSync] Domain %s credentials invalid and no fulldomain stored, registering new", domain.Name)
	}

	// Register new credentials via SSH to the server's acme-dns API
	// acme-dns listens on 127.0.0.1:8053, so we need to call it via SSH
	client := ssh.NewClient(ssh.Config{
		Host:         server.IP,
		Port:         server.Port,
		User:         server.SSHUser,
		Password:     server.Password,
		TrustOnFirst: true,
	})
	defer client.Close()

	if err := client.Connect(); err != nil {
		return fmt.Errorf("SSH connect failed: %w", err)
	}

	// Call acme-dns register API via curl on the server
	result, err := client.Exec(`curl -s -X POST "http://127.0.0.1:8053/register"`)
	if err != nil {
		return fmt.Errorf("failed to call acme-dns register: %w", err)
	}

	var regResp AcmeDnsRegisterResponse
	if err := json.Unmarshal([]byte(result.Output), &regResp); err != nil {
		return fmt.Errorf("failed to decode acme-dns response: %w - output: %s", err, result.Output)
	}

	if regResp.Subdomain == "" {
		return fmt.Errorf("acme-dns register returned empty subdomain: %s", result.Output)
	}

	// Update database with new credentials
	_, err = s.db.Exec(`
		UPDATE domains
		SET acme_subdomain = $1, acme_username = $2, acme_password = $3,
		    acme_fulldomain = $4, acme_cname_verified = FALSE, updated_at = NOW()
		WHERE id = $5
	`, regResp.Subdomain, regResp.Username, regResp.Password, regResp.Fulldomain, domain.ID)
	if err != nil {
		return fmt.Errorf("failed to update database with acme-dns credentials: %w", err)
	}

	// Update the domain object for subsequent operations
	domain.AcmeSubdomain = &regResp.Subdomain
	domain.AcmeUsername = &regResp.Username
	domain.AcmePassword = &regResp.Password
	domain.AcmeFulldomain = &regResp.Fulldomain

	log.Printf("[DomainSync] Registered new acme-dns credentials for %s: %s", domain.Name, regResp.Fulldomain)
	return nil
}

// testAcmeDnsCredentials tests if acme-dns credentials work on a server via SSH
func (s *Service) testAcmeDnsCredentials(server Server, subdomain, username, password string) bool {
	client := ssh.NewClient(ssh.Config{
		Host:         server.IP,
		Port:         server.Port,
		User:         server.SSHUser,
		Password:     server.Password,
		TrustOnFirst: true,
	})
	defer client.Close()

	if err := client.Connect(); err != nil {
		return false
	}

	// Test update via curl on the server
	testToken := fmt.Sprintf("test_validation_%d", time.Now().Unix())
	cmd := fmt.Sprintf(`curl -s -X POST "http://127.0.0.1:8053/update" \
		-H "X-Api-User: %s" \
		-H "X-Api-Key: %s" \
		-H "Content-Type: application/json" \
		-d '{"subdomain":"%s","txt":"%s"}'`, username, password, subdomain, testToken)

	result, err := client.Exec(cmd)
	if err != nil {
		return false
	}

	// If we get txt in response, credentials are valid
	var response map[string]interface{}
	if err := json.Unmarshal([]byte(result.Output), &response); err == nil {
		if _, ok := response["txt"]; ok {
			return true
		}
	}

	return false
}

// adoptAcmeDnsSubdomain handles the case where user has CNAME set up but we don't have valid credentials
// It registers new credentials in acme-dns and updates the database to use the existing subdomain
func (s *Service) adoptAcmeDnsSubdomain(domain *Domain, server Server, existingFulldomain string) error {
	// Extract subdomain from fulldomain (e.g., "abc123.acme.pamach.xyz" -> "abc123")
	parts := strings.Split(existingFulldomain, ".")
	if len(parts) < 1 {
		return fmt.Errorf("invalid fulldomain format: %s", existingFulldomain)
	}
	existingSubdomain := parts[0]

	log.Printf("[DomainSync] Attempting to adopt acme-dns subdomain %s for domain %s", existingSubdomain, domain.Name)

	// Connect to server via SSH
	client := ssh.NewClient(ssh.Config{
		Host:         server.IP,
		Port:         server.Port,
		User:         server.SSHUser,
		Password:     server.Password,
		TrustOnFirst: true,
	})
	defer client.Close()

	if err := client.Connect(); err != nil {
		return fmt.Errorf("SSH connect failed: %w", err)
	}

	// Register new credentials in acme-dns
	result, err := client.Exec(`curl -s -X POST "http://127.0.0.1:8053/register"`)
	if err != nil {
		return fmt.Errorf("failed to register with acme-dns: %w", err)
	}

	var regResp AcmeDnsRegisterResponse
	if err := json.Unmarshal([]byte(result.Output), &regResp); err != nil {
		return fmt.Errorf("failed to decode acme-dns response: %w - output: %s", err, result.Output)
	}

	// Now update the acme-dns database to change the new subdomain to the existing one
	// This allows us to use new credentials with the user's existing CNAME
	updateCmd := fmt.Sprintf(`
		# Delete any existing record with this subdomain
		sqlite3 /var/lib/acme-dns/acme-dns.db "DELETE FROM records WHERE Subdomain = '%s';"
		# Update the new record to use the existing subdomain
		sqlite3 /var/lib/acme-dns/acme-dns.db "UPDATE records SET Subdomain = '%s' WHERE Username = '%s';"
		# Add TXT table entries for the subdomain
		sqlite3 /var/lib/acme-dns/acme-dns.db "INSERT OR IGNORE INTO txt (Subdomain, Value, LastUpdate) VALUES ('%s', '', 0);"
		sqlite3 /var/lib/acme-dns/acme-dns.db "INSERT OR IGNORE INTO txt (Subdomain, Value, LastUpdate) VALUES ('%s', '', 0);"
		echo "OK"
	`, existingSubdomain, existingSubdomain, regResp.Username, existingSubdomain, existingSubdomain)

	result, err = client.Exec(updateCmd)
	if err != nil {
		return fmt.Errorf("failed to update acme-dns database: %w", err)
	}

	if !strings.Contains(result.Output, "OK") {
		return fmt.Errorf("acme-dns database update failed: %s", result.Output)
	}

	// Restart acme-dns to pick up changes
	if _, err := client.Exec("systemctl restart acme-dns"); err != nil {
		log.Printf("[DomainSync] Warning: failed to restart acme-dns: %v", err)
	}

	// Update our database with the new credentials but existing subdomain
	_, err = s.db.Exec(`
		UPDATE domains
		SET acme_subdomain = $1, acme_username = $2, acme_password = $3,
		    acme_fulldomain = $4, acme_cname_verified = TRUE, updated_at = NOW()
		WHERE id = $5
	`, existingSubdomain, regResp.Username, regResp.Password, existingFulldomain, domain.ID)
	if err != nil {
		return fmt.Errorf("failed to update database: %w", err)
	}

	// Update domain object
	domain.AcmeSubdomain = &existingSubdomain
	domain.AcmeUsername = &regResp.Username
	domain.AcmePassword = &regResp.Password
	fulldomainStr := existingFulldomain
	domain.AcmeFulldomain = &fulldomainStr

	log.Printf("[DomainSync] Successfully adopted acme-dns subdomain %s for domain %s", existingSubdomain, domain.Name)
	return nil
}

// checkDomainConfigExists checks if the domain has config on the server
func (s *Service) checkDomainConfigExists(domain Domain, server Server) (bool, error) {
	client := ssh.NewClient(ssh.Config{
		Host:         server.IP,
		Port:         server.Port,
		User:         server.SSHUser,
		Password:     server.Password,
		TrustOnFirst: true,
	})
	defer client.Close()

	if err := client.Connect(); err != nil {
		return false, fmt.Errorf("SSH connect failed: %w", err)
	}

	// Check if any link config exists for this domain
	// Link configs are stored in /etc/botection/links/*.json with host field
	cmd := fmt.Sprintf(`grep -l '"host":.*"%s"' /etc/botection/links/*.json 2>/dev/null | head -1`, domain.Name)
	result, err := client.Exec(cmd)
	if err != nil {
		// No configs found is not an error, just means empty
		return false, nil
	}

	return strings.TrimSpace(result.Output) != "", nil
}

// checkDNSRecords verifies DNS records for the domain
func (s *Service) checkDNSRecords(domain Domain, serverIP string) (bool, []string) {
	var missing []string
	baseDomain := domain.Name
	if strings.HasPrefix(baseDomain, "*.") {
		baseDomain = baseDomain[2:]
	}

	// Check A record for base domain
	aRecords, err := net.LookupHost(baseDomain)
	if err != nil || !containsIP(aRecords, serverIP) {
		missing = append(missing, fmt.Sprintf("A @ → %s", serverIP))
	}

	// Check wildcard A record (test with a random subdomain)
	wildcardRecords, err := net.LookupHost("sync-test." + baseDomain)
	if err != nil || !containsIP(wildcardRecords, serverIP) {
		missing = append(missing, fmt.Sprintf("A * → %s", serverIP))
	}

	// Check CNAME for acme-dns (if configured)
	if domain.AcmeFulldomain != nil && *domain.AcmeFulldomain != "" {
		cname, err := net.LookupCNAME("_acme-challenge." + baseDomain)
		expectedCname := *domain.AcmeFulldomain
		if !strings.HasSuffix(expectedCname, ".") {
			expectedCname += "."
		}
		if err != nil || !strings.EqualFold(strings.TrimSuffix(cname, "."), strings.TrimSuffix(expectedCname, ".")) {
			missing = append(missing, fmt.Sprintf("CNAME _acme-challenge → %s", *domain.AcmeFulldomain))
		}
	}

	return len(missing) == 0, missing
}

// checkSSLExists checks if SSL certificate exists on the server
func (s *Service) checkSSLExists(domain Domain, server Server) (bool, error) {
	client := ssh.NewClient(ssh.Config{
		Host:         server.IP,
		Port:         server.Port,
		User:         server.SSHUser,
		Password:     server.Password,
		TrustOnFirst: true,
	})
	defer client.Close()

	if err := client.Connect(); err != nil {
		return false, fmt.Errorf("SSH connect failed: %w", err)
	}

	baseDomain := domain.Name
	if strings.HasPrefix(baseDomain, "*.") {
		baseDomain = baseDomain[2:]
	}

	// Check if Let's Encrypt cert exists
	cmd := fmt.Sprintf(`test -f /etc/letsencrypt/live/%s/fullchain.pem && echo "exists"`, baseDomain)
	result, _ := client.Exec(cmd)

	return result != nil && strings.TrimSpace(result.Output) == "exists", nil
}

// generateSSL generates SSL certificate for the domain
func (s *Service) generateSSL(domain Domain, server Server) error {
	if s.sslGenerator != nil {
		return s.sslGenerator.GenerateWildcardSSL(domain, server)
	}

	// Default implementation using certbot via SSH
	client := ssh.NewClient(ssh.Config{
		Host:         server.IP,
		Port:         server.Port,
		User:         server.SSHUser,
		Password:     server.Password,
		TrustOnFirst: true,
	})
	defer client.Close()

	if err := client.Connect(); err != nil {
		return fmt.Errorf("SSH connect failed: %w", err)
	}

	baseDomain := domain.Name
	if strings.HasPrefix(baseDomain, "*.") {
		baseDomain = baseDomain[2:]
	}

	// Create the update script for acme-dns
	if domain.AcmeUsername != nil && domain.AcmePassword != nil && domain.AcmeSubdomain != nil {
		updateScript := fmt.Sprintf(`cat > /root/update-acme-dns-%s.sh << 'SCRIPT'
#!/bin/bash
TOKEN="$CERTBOT_VALIDATION"
curl -s -X POST "http://127.0.0.1:8053/update" \
  -H "X-Api-User: %s" \
  -H "X-Api-Key: %s" \
  -H "Content-Type: application/json" \
  -d "{\"subdomain\":\"%s\",\"txt\":\"$TOKEN\"}"
sleep 15
SCRIPT
chmod +x /root/update-acme-dns-%s.sh`, baseDomain, *domain.AcmeUsername, *domain.AcmePassword, *domain.AcmeSubdomain, baseDomain)

		if _, err := client.Exec(updateScript); err != nil {
			return fmt.Errorf("failed to create update script: %w", err)
		}

		// Run certbot
		certbotCmd := fmt.Sprintf(`certbot certonly --manual \
			--preferred-challenges dns \
			-d "*.%s" \
			-d "%s" \
			--manual-auth-hook /root/update-acme-dns-%s.sh \
			--agree-tos \
			--email admin@%s \
			--no-eff-email \
			--non-interactive 2>&1`, baseDomain, baseDomain, baseDomain, baseDomain)

		result, err := client.Exec(certbotCmd)
		if err != nil {
			output := ""
			if result != nil {
				output = result.Output
			}
			return fmt.Errorf("certbot failed: %w - output: %s", err, output)
		}

		if result == nil || !strings.Contains(result.Output, "Successfully received certificate") {
			output := ""
			if result != nil {
				output = result.Output
			}
			return fmt.Errorf("certbot did not succeed: %s", output)
		}
	}

	return nil
}

// redeployDomainLinks redeploys all links for a domain
func (s *Service) redeployDomainLinks(domain Domain, server Server) error {
	// Get all redirect links for this domain
	var redirectLinks []RedirectLink
	err := s.db.Select(&redirectLinks, `
		SELECT id, subdomain FROM redirect_links
		WHERE domain_id = $1
	`, domain.ID)
	if err != nil {
		return fmt.Errorf("failed to fetch redirect links: %w", err)
	}

	// Get all short links for this domain
	var shortLinks []ShortLink
	err = s.db.Select(&shortLinks, `
		SELECT id, path FROM short_links
		WHERE domain_id = $1
	`, domain.ID)
	if err != nil {
		return fmt.Errorf("failed to fetch short links: %w", err)
	}

	log.Printf("[DomainSync] Redeploying %d redirect links and %d short links for %s",
		len(redirectLinks), len(shortLinks), domain.Name)

	// Redeploy using the redeployer if set
	if s.linkRedeployer != nil {
		for _, link := range redirectLinks {
			if err := s.linkRedeployer.RedeployRedirectLink(link.ID, server); err != nil {
				log.Printf("[DomainSync] Failed to redeploy redirect link %s: %v", link.ID, err)
			}
		}
		for _, link := range shortLinks {
			if err := s.linkRedeployer.RedeployShortLink(link.ID, server); err != nil {
				log.Printf("[DomainSync] Failed to redeploy short link %s: %v", link.ID, err)
			}
		}
	}

	return nil
}

// updateSyncStatus updates the sync status of a domain and keeps legacy fields in sync
func (s *Service) updateSyncStatus(domainID, status, errorMsg string) {
	var errPtr *string
	if errorMsg != "" {
		errPtr = &errorMsg
	}

	// Keep legacy fields (ssl_enabled, dns_verified, setup_step) in sync with sync_status
	// so the UI displays correct status and shows/hides setup wizard appropriately
	switch status {
	case SyncStatusActive:
		// Fully synced - DNS verified and SSL enabled
		s.db.Exec(`
			UPDATE domains
			SET sync_status = $1, last_sync_at = NOW(), last_sync_error = $2,
			    dns_verified = TRUE, ssl_enabled = TRUE, setup_step = 'complete', updated_at = NOW()
			WHERE id = $3
		`, status, errPtr, domainID)
	case SyncStatusDNSWaiting, SyncStatusPending, SyncStatusSyncing:
		// Not ready yet - reset legacy fields, show setup wizard
		s.db.Exec(`
			UPDATE domains
			SET sync_status = $1, last_sync_at = NOW(), last_sync_error = $2,
			    dns_verified = FALSE, ssl_enabled = FALSE, setup_step = 'dns', updated_at = NOW()
			WHERE id = $3
		`, status, errPtr, domainID)
	case SyncStatusSSLGenerating:
		// DNS verified but SSL not yet
		s.db.Exec(`
			UPDATE domains
			SET sync_status = $1, last_sync_at = NOW(), last_sync_error = $2,
			    dns_verified = TRUE, ssl_enabled = FALSE, setup_step = 'ssl', updated_at = NOW()
			WHERE id = $3
		`, status, errPtr, domainID)
	default:
		// Error or unknown - just update sync fields
		s.db.Exec(`
			UPDATE domains
			SET sync_status = $1, last_sync_at = NOW(), last_sync_error = $2, updated_at = NOW()
			WHERE id = $3
		`, status, errPtr, domainID)
	}
}

// containsIP checks if an IP is in a list
func containsIP(ips []string, target string) bool {
	for _, ip := range ips {
		if ip == target {
			return true
		}
	}
	return false
}
