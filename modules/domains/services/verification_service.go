package services

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/botginx/botginx/pkg/sshexec"
)

// ServerInfo holds SSH connection details
type ServerInfo struct {
	IP       string
	Port     int
	User     string
	Password string
}

// VerificationService handles domain verification and SSL operations
type VerificationService struct {
	httpClient    *http.Client
	getServerFunc func() (*ServerInfo, error)
}

// NewVerificationService creates a new verification service
func NewVerificationService() *VerificationService {
	return &VerificationService{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // Allow self-signed during verification
				},
			},
		},
	}
}

// SetServerProvider sets the function to get server credentials from database
func (s *VerificationService) SetServerProvider(fn func() (*ServerInfo, error)) {
	s.getServerFunc = fn
}

func (s *VerificationService) getServer() (*ServerInfo, error) {
	if s.getServerFunc != nil {
		return s.getServerFunc()
	}
	return nil, fmt.Errorf("no server available")
}

// VerifyDNS checks if domain has the correct TXT record for verification.
// This works with Cloudflare proxy enabled.
func (s *VerificationService) VerifyDNS(domain, expectedToken string) (bool, error) {
	// For wildcard domains, use the base domain
	baseDomain := domain
	if strings.HasPrefix(domain, "*.") {
		baseDomain = strings.TrimPrefix(domain, "*.")
	}

	// Look up TXT record at _guardbot-verify.domain.com
	txtHost := "_guardbot-verify." + baseDomain

	// Helper to query a specific DNS server
	queryDNS := func(dnsServer string) ([]string, error) {
		if dnsServer == "" {
			return net.LookupTXT(txtHost)
		}
		resolver := &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{Timeout: 3 * time.Second}
				return d.DialContext(ctx, "udp", dnsServer)
			},
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return resolver.LookupTXT(ctx, txtHost)
	}

	// First, try to get authoritative nameservers and query them directly
	// This bypasses public DNS caching issues
	nsRecords, _ := net.LookupNS(baseDomain)
	for _, ns := range nsRecords {
		nsHost := strings.TrimSuffix(ns.Host, ".")
		// Resolve NS hostname to IP
		nsIPs, err := net.LookupHost(nsHost)
		if err != nil || len(nsIPs) == 0 {
			continue
		}
		records, err := queryDNS(nsIPs[0] + ":53")
		if err == nil && len(records) > 0 {
			for _, record := range records {
				if strings.TrimSpace(record) == expectedToken {
					return true, nil // Success from authoritative NS!
				}
			}
		}
	}

	// Fallback: try public DNS servers
	dnsServers := []string{"8.8.8.8:53", "1.1.1.1:53", ""}
	var allFoundRecords []string
	var lastSource string

	for _, dnsServer := range dnsServers {
		dnsSource := "Google DNS"
		if dnsServer == "1.1.1.1:53" {
			dnsSource = "Cloudflare DNS"
		} else if dnsServer == "" {
			dnsSource = "System DNS"
		}

		records, err := queryDNS(dnsServer)
		if err == nil && len(records) > 0 {
			for _, record := range records {
				if strings.TrimSpace(record) == expectedToken {
					return true, nil // Success from public DNS
				}
			}
			allFoundRecords = records
			lastSource = dnsSource
		}
	}

	// No DNS server had the correct value
	if len(allFoundRecords) > 0 {
		return false, fmt.Errorf("TXT mismatch (%s). Expected: %s, Found: [%s]. DNS may still be propagating", lastSource, expectedToken, strings.Join(allFoundRecords, ", "))
	}
	return false, fmt.Errorf("TXT not found at %s. Add value: %s", txtHost, expectedToken)
}

// SSLStatus represents the SSL certificate status for a domain
type SSLStatus struct {
	Exists      bool   `json:"exists"`
	IsWildcard  bool   `json:"is_wildcard"`
	ExpiresAt   string `json:"expires_at,omitempty"`
	Domains     string `json:"domains,omitempty"`
	Error       string `json:"error,omitempty"`
}

// CheckSSL checks if SSL certificate exists on the VPS for the domain
func (s *VerificationService) CheckSSL(domain string) (*SSLStatus, error) {
	server, err := s.getServer()
	if err != nil {
		return nil, fmt.Errorf("no deploy server available")
	}

	port := fmt.Sprintf("%d", server.Port)
	if server.Port == 0 {
		port = "22"
	}

	client, err := sshexec.NewClient(server.IP, port, server.User, server.Password)
	if err != nil {
		return nil, fmt.Errorf("connection to server failed: %w", err)
	}
	defer client.Close()

	// Check if cert exists
	certPath := fmt.Sprintf("/etc/letsencrypt/live/%s/fullchain.pem", domain)
	checkCmd := fmt.Sprintf("test -f %s && echo EXISTS || echo MISSING", certPath)

	output, err := client.Run(checkCmd)
	if err != nil {
		return &SSLStatus{Exists: false, Error: err.Error()}, nil
	}

	if strings.TrimSpace(output) != "EXISTS" {
		return &SSLStatus{Exists: false}, nil
	}

	// Check if it's a wildcard cert and get expiry
	infoCmd := fmt.Sprintf(`openssl x509 -in %s -noout -subject -enddate 2>/dev/null | tr '\n' ' '`, certPath)
	info, _ := client.Run(infoCmd)

	status := &SSLStatus{Exists: true}

	// Check for wildcard
	if strings.Contains(info, "*."+domain) {
		status.IsWildcard = true
	}

	// Parse expiry date
	if idx := strings.Index(info, "notAfter="); idx != -1 {
		expiry := strings.TrimSpace(info[idx+9:])
		status.ExpiresAt = expiry
	}

	// Get all domains covered
	sanCmd := fmt.Sprintf(`openssl x509 -in %s -noout -text 2>/dev/null | grep -A1 "Subject Alternative Name" | tail -1`, certPath)
	san, _ := client.Run(sanCmd)
	if san != "" {
		status.Domains = strings.TrimSpace(san)
	}

	return status, nil
}

// GenerateWildcardSSL generates a wildcard SSL certificate for the domain
// This uses DNS-01 challenge which requires manual DNS TXT record
// Returns the TXT record value that needs to be added
func (s *VerificationService) GenerateWildcardSSL(domain, email string) (*WildcardSSLRequest, error) {
	server, err := s.getServer()
	if err != nil {
		return nil, fmt.Errorf("no deploy server available")
	}

	port := fmt.Sprintf("%d", server.Port)
	if server.Port == 0 {
		port = "22"
	}

	if email == "" {
		email = "admin@" + domain
	}

	client, err := sshexec.NewClient(server.IP, port, server.User, server.Password)
	if err != nil {
		return nil, fmt.Errorf("connection to server failed: %w", err)
	}
	defer client.Close()

	// First, create the directory structure
	setupCmd := fmt.Sprintf(`mkdir -p /var/www/sites/%s/_root /var/www/sites/%s/_errors`, domain, domain)
	client.Run(setupCmd)

	// For wildcard, we need DNS-01 challenge
	// We'll use certbot with manual plugin and return the challenge
	// The user will need to add the TXT record, then we verify

	return &WildcardSSLRequest{
		Domain:         domain,
		WildcardDomain: "*." + domain,
		Email:          email,
		TXTRecordName:  "_acme-challenge." + domain,
		Instructions: fmt.Sprintf(`To enable wildcard SSL for %s:

1. Add a DNS TXT record:
   Name: _acme-challenge.%s
   Value: (will be provided during setup)

2. Contact support to complete the SSL setup

Your wildcard SSL will cover: *.%s and %s`,
			domain, domain, domain, domain),
	}, nil
}

// WildcardSSLRequest contains info for wildcard SSL setup
type WildcardSSLRequest struct {
	Domain         string `json:"domain"`
	WildcardDomain string `json:"wildcard_domain"`
	Email          string `json:"email"`
	TXTRecordName  string `json:"txt_record_name"`
	Instructions   string `json:"instructions"`
}

// IsCloudflare checks if domain is behind Cloudflare proxy
func (s *VerificationService) IsCloudflare(domain string) bool {
	ips, err := net.LookupIP(domain)
	if err != nil || len(ips) == 0 {
		return false
	}

	// Cloudflare IP ranges (simplified check)
	cfRanges := []string{"104.", "172.67.", "162.158.", "141.101.", "108.162.", "190.93.", "188.114.", "197.234.", "198.41.", "103.21.", "103.22.", "103.31."}
	ipStr := ips[0].String()
	for _, prefix := range cfRanges {
		if strings.HasPrefix(ipStr, prefix) {
			return true
		}
	}
	return false
}

// GenerateSSL runs certbot to get SSL certificate for the domain
// For Cloudflare domains, create self-signed cert for origin connection
func (s *VerificationService) GenerateSSL(domain string) error {
	// If behind Cloudflare, create self-signed cert for origin (Full mode)
	if s.IsCloudflare(domain) {
		return s.generateSelfSignedCert(domain)
	}

	server, err := s.getServer()
	if err != nil {
		return fmt.Errorf("no deploy server available")
	}

	port := fmt.Sprintf("%d", server.Port)
	if server.Port == 0 {
		port = "22"
	}

	client, err := sshexec.NewClient(server.IP, port, server.User, server.Password)
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer client.Close()

	// Run certbot for single domain (wildcard requires DNS challenge)
	cmd := fmt.Sprintf(`certbot certonly --nginx -d %s --non-interactive --agree-tos --email admin@%s 2>&1`,
		domain, domain)

	output, err := client.Run(cmd)
	if err != nil {
		if strings.Contains(output, "Certificate not yet due for renewal") {
			return nil
		}
		return fmt.Errorf("SSL failed: %s", parseCertbotError(output))
	}

	client.Run("nginx -t && systemctl reload nginx")
	return nil
}

// GenerateHTTPSSL generates SSL using HTTP-01 challenge (for non-wildcard domains)
// This is simpler than DNS-01 - no TXT records needed, just A record pointing to server
func (s *VerificationService) GenerateHTTPSSL(domain string) error {
	server, err := s.getServer()
	if err != nil {
		return fmt.Errorf("no deploy server available")
	}

	port := fmt.Sprintf("%d", server.Port)
	if server.Port == 0 {
		port = "22"
	}

	client, err := sshexec.NewClient(server.IP, port, server.User, server.Password)
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer client.Close()

	// Ensure webroot directory exists
	webrootCmd := fmt.Sprintf(`mkdir -p /var/www/sites/%s/_root/.well-known/acme-challenge`, domain)
	client.Run(webrootCmd)

	// Try nginx plugin first (most reliable), fallback to webroot
	cmd := fmt.Sprintf(`
# First try nginx plugin
if certbot certonly --nginx -d %s --non-interactive --agree-tos --email admin@%s 2>&1; then
    echo "SUCCESS_NGINX"
    exit 0
fi

# Fallback to webroot
if certbot certonly --webroot -w /var/www/sites/%s/_root -d %s --non-interactive --agree-tos --email admin@%s 2>&1; then
    echo "SUCCESS_WEBROOT"
    exit 0
fi

# Fallback to standalone (stops nginx temporarily)
systemctl stop nginx 2>/dev/null || true
if certbot certonly --standalone -d %s --non-interactive --agree-tos --email admin@%s 2>&1; then
    systemctl start nginx
    echo "SUCCESS_STANDALONE"
    exit 0
fi
systemctl start nginx
echo "FAILED"
exit 1
`, domain, domain, domain, domain, domain, domain, domain)

	output, err := client.Run(cmd)
	if err != nil {
		if strings.Contains(output, "Certificate not yet due for renewal") {
			return nil
		}
		return fmt.Errorf("SSL failed: %s", parseCertbotError(output))
	}

	// Reload nginx to pick up new cert
	client.Run("nginx -t && systemctl reload nginx")
	return nil
}

// GenerateWildcardSSLWithAcmeDNS generates wildcard SSL using acme-dns CNAME delegation
// This is 100% reliable - we control the TXT record via acme-dns API
// acmeUsername is used for X-Api-User (different from subdomain in acme-dns)
func (s *VerificationService) GenerateWildcardSSLWithAcmeDNS(domain, acmeSubdomain, acmeUsername, acmePassword string) error {
	server, err := s.getServer()
	if err != nil {
		return fmt.Errorf("no deploy server available")
	}

	baseDomain := GetBaseDomain(domain)

	port := fmt.Sprintf("%d", server.Port)
	if server.Port == 0 {
		port = "22"
	}

	client, err := sshexec.NewClient(server.IP, port, server.User, server.Password)
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer client.Close()

	// Check if cert already exists
	checkExisting := fmt.Sprintf(`test -f /etc/letsencrypt/live/%s/fullchain.pem && echo "EXISTS"`, baseDomain)
	if out, _ := client.Run(checkExisting); strings.Contains(out, "EXISTS") {
		return nil // Already have cert
	}

	// Create auth hook that updates acme-dns TXT record
	// X-Api-User must be the username (not subdomain) from acme-dns registration
	authHookScript := fmt.Sprintf(`cat > /tmp/acmedns-auth-hook.sh << 'HOOKEOF'
#!/bin/bash
# Update acme-dns TXT record via API
curl -s -X POST http://127.0.0.1:8053/update \
    -H "X-Api-User: %s" \
    -H "X-Api-Key: %s" \
    -H "Content-Type: application/json" \
    -d "{\"subdomain\":\"%s\",\"txt\":\"$CERTBOT_VALIDATION\"}"

# Wait for DNS propagation (acme-dns is instant, but give it a moment)
sleep 5
HOOKEOF
chmod +x /tmp/acmedns-auth-hook.sh`, acmeUsername, acmePassword, acmeSubdomain)
	client.Run(authHookScript)

	// Run certbot with DNS-01 challenge using our auth hook
	certbotCmd := fmt.Sprintf(`
certbot certonly --manual --preferred-challenges dns \
  -d "*.%s" -d "%s" \
  --agree-tos --email admin@%s \
  --manual-auth-hook /tmp/acmedns-auth-hook.sh \
  --manual-cleanup-hook "echo cleanup" \
  --non-interactive 2>&1
`, baseDomain, baseDomain, baseDomain)

	output, err := client.Run(certbotCmd)
	if err != nil {
		if strings.Contains(output, "Certificate not yet due for renewal") {
			return nil
		}
		return fmt.Errorf("SSL generation failed: %s", parseCertbotError(output))
	}

	// Check if certificate was created
	checkCmd := fmt.Sprintf(`test -f /etc/letsencrypt/live/%s/fullchain.pem && echo "SUCCESS"`, baseDomain)
	checkOutput, _ := client.Run(checkCmd)

	if !strings.Contains(checkOutput, "SUCCESS") {
		return fmt.Errorf("SSL certificate not created: %s", parseCertbotError(output))
	}

	// Reload nginx
	client.Run("nginx -t && systemctl reload nginx 2>/dev/null || true")
	return nil
}

// CheckAcmeCnameRecord verifies if CNAME record is correctly pointing to acme-dns
func (s *VerificationService) CheckAcmeCnameRecord(domain, expectedTarget string) bool {
	baseDomain := GetBaseDomain(domain)
	cnameHost := "_acme-challenge." + baseDomain

	// Query CNAME record
	cname, err := net.LookupCNAME(cnameHost)
	if err != nil {
		return false
	}

	// Normalize (remove trailing dot)
	cname = strings.TrimSuffix(cname, ".")
	expectedTarget = strings.TrimSuffix(expectedTarget, ".")

	return strings.EqualFold(cname, expectedTarget)
}

// generateSelfSignedCert creates a self-signed cert for Cloudflare Full mode
func (s *VerificationService) generateSelfSignedCert(domain string) error {
	server, err := s.getServer()
	if err != nil {
		return err
	}

	port := fmt.Sprintf("%d", server.Port)
	if server.Port == 0 {
		port = "22"
	}

	client, err := sshexec.NewClient(server.IP, port, server.User, server.Password)
	if err != nil {
		return err
	}
	defer client.Close()

	// Create self-signed cert for origin
	cmd := fmt.Sprintf(`
mkdir -p /etc/nginx/ssl
if [ ! -f /etc/nginx/ssl/%s.pem ]; then
    openssl req -x509 -nodes -days 3650 -newkey rsa:2048 \
        -keyout /etc/nginx/ssl/%s.key \
        -out /etc/nginx/ssl/%s.pem \
        -subj '/CN=*.%s' 2>/dev/null
fi
`, domain, domain, domain, domain)

	_, err = client.Run(cmd)
	return err
}

// SetupDomainNginx creates nginx config for the domain on the VPS
func (s *VerificationService) SetupDomainNginx(domain string) error {
	server, err := s.getServer()
	if err != nil {
		return fmt.Errorf("no deploy server available")
	}

	port := fmt.Sprintf("%d", server.Port)
	if server.Port == 0 {
		port = "22"
	}

	client, err := sshexec.NewClient(server.IP, port, server.User, server.Password)
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer client.Close()

	// Check if add-domain.sh exists
	checkScript := "test -f /var/www/templates/scripts/add-domain.sh && echo EXISTS || echo MISSING"
	output, _ := client.Run(checkScript)

	if strings.TrimSpace(output) == "EXISTS" {
		// Use the existing script
		cmd := fmt.Sprintf("/var/www/templates/scripts/add-domain.sh %s", domain)
		_, err = client.Run(cmd)
		return err
	}

	// Check if Cloudflare - use self-signed cert if so
	isCloudflare := s.IsCloudflare(domain)

	// Fallback: create basic nginx config with SSL
	var nginxConfig string
	if isCloudflare {
		nginxConfig = fmt.Sprintf(`server {
    listen 80;
    listen 443 ssl;
    server_name %s *.%s;

    ssl_certificate /etc/nginx/ssl/%s.pem;
    ssl_certificate_key /etc/nginx/ssl/%s.key;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}`, domain, domain, domain, domain)
	} else {
		// For domains with Let's Encrypt SSL (including wildcard via DNS-01)
		// Strip wildcard prefix to get base domain for cert path
		baseDomain := domain
		if strings.HasPrefix(domain, "*.") {
			baseDomain = strings.TrimPrefix(domain, "*.")
		}

		nginxConfig = fmt.Sprintf(`server {
    listen 80;
    listen 443 ssl;
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
}`, baseDomain, baseDomain, baseDomain, baseDomain, baseDomain)
	}

	// Create directories
	mkdirCmd := fmt.Sprintf("mkdir -p /var/www/sites/%s/_root /var/www/sites/%s/_errors", domain, domain)
	client.Run(mkdirCmd)

	// Write nginx config
	configPath := fmt.Sprintf("/etc/nginx/sites-available/%s.conf", domain)
	writeCmd := fmt.Sprintf("cat > %s << 'NGINXEOF'\n%s\nNGINXEOF", configPath, nginxConfig)
	if _, err := client.Run(writeCmd); err != nil {
		return fmt.Errorf("failed to configure domain: %w", err)
	}

	// Enable site
	enableCmd := fmt.Sprintf("ln -sf %s /etc/nginx/sites-enabled/ && nginx -t && systemctl reload nginx", configPath)
	if _, err := client.Run(enableCmd); err != nil {
		return fmt.Errorf("failed to activate domain: %w", err)
	}

	return nil
}

// CleanupDomain removes all domain-related files from the VPS:
// - nginx config (sites-available and sites-enabled)
// - SSL certificates (self-signed and Let's Encrypt)
// - site directories
func (s *VerificationService) CleanupDomain(domain string) error {
	server, err := s.getServer()
	if err != nil {
		return fmt.Errorf("no deploy server available")
	}

	port := fmt.Sprintf("%d", server.Port)
	if server.Port == 0 {
		port = "22"
	}

	client, err := sshexec.NewClient(server.IP, port, server.User, server.Password)
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer client.Close()

	// Get base domain for wildcard cleanup
	baseDomain := GetBaseDomain(domain)

	// Remove nginx config (both wildcard and base domain patterns)
	nginxCmd := fmt.Sprintf(`
rm -f /etc/nginx/sites-enabled/%s.conf
rm -f /etc/nginx/sites-available/%s.conf
rm -f /etc/nginx/sites-enabled/*.%s.conf
rm -f /etc/nginx/sites-available/*.%s.conf
`, domain, domain, baseDomain, baseDomain)
	client.Run(nginxCmd)

	// Remove self-signed SSL certs
	sslCmd := fmt.Sprintf(`
rm -f /etc/nginx/ssl/%s.pem
rm -f /etc/nginx/ssl/%s.key
rm -f /etc/nginx/ssl/%s.pem
rm -f /etc/nginx/ssl/%s.key
`, domain, domain, baseDomain, baseDomain)
	client.Run(sslCmd)

	// Remove Let's Encrypt certs (certbot)
	letsEncryptCmd := fmt.Sprintf(`
if [ -d /etc/letsencrypt/live/%s ]; then
    certbot delete --cert-name %s --non-interactive 2>/dev/null || true
fi
if [ -d /etc/letsencrypt/live/%s ]; then
    certbot delete --cert-name %s --non-interactive 2>/dev/null || true
fi
rm -rf /etc/letsencrypt/live/%s
rm -rf /etc/letsencrypt/archive/%s
rm -rf /etc/letsencrypt/renewal/%s.conf
`, domain, domain, baseDomain, baseDomain, baseDomain, baseDomain, baseDomain)
	client.Run(letsEncryptCmd)

	// Remove lego SSL certificates and data
	legoCmd := fmt.Sprintf(`
rm -rf /root/.lego-%s
rm -f /tmp/lego-*%s*
`, baseDomain, baseDomain)
	client.Run(legoCmd)

	// Remove site directories
	siteDirCmd := fmt.Sprintf(`
rm -rf /var/www/sites/%s
rm -rf /var/www/sites/%s
`, domain, baseDomain)
	client.Run(siteDirCmd)

	// Remove botection link settings
	botectionCmd := fmt.Sprintf(`
rm -f /etc/botection/links/%s.json
rm -f /etc/botection/links/%s.json
rm -f /etc/botection/links/*.%s.json
`, domain, baseDomain, baseDomain)
	client.Run(botectionCmd)

	// Reload nginx
	client.Run("nginx -t && systemctl reload nginx 2>/dev/null || true")

	return nil
}

// DetectDomainType checks if domain A record points to our server
func (s *VerificationService) DetectDomainType(domain string, ourIP string) string {
	// For wildcard domains, check the base domain
	checkDomain := domain
	if strings.HasPrefix(domain, "*.") {
		checkDomain = strings.TrimPrefix(domain, "*.")
	}

	// Try to resolve the domain
	ips, err := net.LookupHost(checkDomain)
	if err != nil {
		// Can't resolve = external DNS needed
		return "external"
	}

	for _, ip := range ips {
		if ip == ourIP {
			return "direct"
		}
	}

	return "external"
}

// CheckARecord checks if a domain's A record points to the expected IP
func (s *VerificationService) CheckARecord(domain string, expectedIP string) (bool, string) {
	// For wildcard, check a random subdomain
	checkDomain := domain
	if strings.HasPrefix(domain, "*.") {
		checkDomain = "wildcard-check." + strings.TrimPrefix(domain, "*.")
	}

	ips, err := net.LookupHost(checkDomain)
	if err != nil {
		return false, ""
	}

	for _, ip := range ips {
		if ip == expectedIP {
			return true, ip
		}
	}

	if len(ips) > 0 {
		return false, ips[0]
	}
	return false, ""
}

// GetAcmeTXTValue returns the current value of the ACME challenge TXT record (if any)
func (s *VerificationService) GetAcmeTXTValue(domain string) string {
	baseDomain := domain
	if strings.HasPrefix(domain, "*.") {
		baseDomain = strings.TrimPrefix(domain, "*.")
	}

	txtHost := "_acme-challenge." + baseDomain

	// Try system resolver first
	records, err := net.LookupTXT(txtHost)
	if err == nil && len(records) > 0 {
		return strings.TrimSpace(records[0])
	}

	// Try Google DNS
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 5 * time.Second}
			return d.DialContext(ctx, "udp", "8.8.8.8:53")
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	records, err = r.LookupTXT(ctx, txtHost)
	cancel()

	if err == nil && len(records) > 0 {
		return strings.TrimSpace(records[0])
	}

	return ""
}

// CheckAcmeTXT checks if the ACME challenge TXT record exists with correct value
func (s *VerificationService) CheckAcmeTXT(domain string, expectedToken string) bool {
	// For wildcard, use base domain
	baseDomain := domain
	if strings.HasPrefix(domain, "*.") {
		baseDomain = strings.TrimPrefix(domain, "*.")
	}

	txtHost := "_acme-challenge." + baseDomain

	// Try multiple resolvers
	resolvers := []string{"8.8.8.8:53", "1.1.1.1:53", ""}
	for _, resolver := range resolvers {
		var records []string
		var err error

		if resolver == "" {
			records, err = net.LookupTXT(txtHost)
		} else {
			r := &net.Resolver{
				PreferGo: true,
				Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
					d := net.Dialer{Timeout: 5 * time.Second}
					return d.DialContext(ctx, "udp", resolver)
				},
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			records, err = r.LookupTXT(ctx, txtHost)
			cancel()
		}

		if err == nil {
			for _, record := range records {
				if strings.TrimSpace(record) == expectedToken {
					return true
				}
			}
		}
	}

	return false
}

// PreGenerateAcmeToken gets an ACME challenge token for wildcard SSL
// Runs certbot briefly to capture the token, then kills it
// The actual SSL generation happens in CompleteWildcardSSL
// Returns "CERT_EXISTS" if SSL cert already exists (no token needed)
func (s *VerificationService) PreGenerateAcmeToken(domain string) (string, error) {
	server, err := s.getServer()
	if err != nil {
		return "", fmt.Errorf("no deploy server available")
	}

	baseDomain := domain
	if strings.HasPrefix(domain, "*.") {
		baseDomain = strings.TrimPrefix(domain, "*.")
	}

	port := fmt.Sprintf("%d", server.Port)
	if server.Port == 0 {
		port = "22"
	}

	client, err := sshexec.NewClient(server.IP, port, server.User, server.Password)
	if err != nil {
		return "", fmt.Errorf("SSH connection failed: %w", err)
	}
	defer client.Close()

	// Check if SSL cert already exists - no need for ACME token
	checkCert := fmt.Sprintf(`test -f /etc/letsencrypt/live/%s/fullchain.pem && echo "EXISTS" || echo "NOTFOUND"`, baseDomain)
	out, err := client.Run(checkCert)
	if err != nil {
		return "", fmt.Errorf("SSH cert check failed: %w", err)
	}
	if strings.Contains(out, "EXISTS") {
		return "CERT_EXISTS", nil
	}

	// Check if we already have a valid token
	checkCmd := fmt.Sprintf(`cat /tmp/acme-token-%s.txt 2>/dev/null || echo ""`, baseDomain)
	existing, _ := client.Run(checkCmd)
	if existingToken := strings.TrimSpace(existing); existingToken != "" && len(existingToken) > 20 {
		return existingToken, nil
	}

	// Create auth hook that captures token and exits immediately
	// We just need the token value, actual SSL generation is separate
	setupCmd := `cat > /tmp/dns-auth-capture.sh << 'HOOKEOF'
#!/bin/bash
echo "$CERTBOT_VALIDATION" > /tmp/acme-token-$CERTBOT_DOMAIN.txt
# Exit with error to stop certbot - we just needed the token
exit 1
HOOKEOF
chmod +x /tmp/dns-auth-capture.sh`
	client.Run(setupCmd)

	// Run certbot to get token (will fail after capturing token, that's intentional)
	// Redirect both stdout and stderr to /dev/null so only the final cat output is returned
	certbotCmd := fmt.Sprintf(`
rm -f /tmp/acme-token-%s.txt
timeout 30 certbot certonly --manual --preferred-challenges dns \
  -d "*.%s" \
  --agree-tos --email admin@%s \
  --manual-auth-hook /tmp/dns-auth-capture.sh \
  --non-interactive >/dev/null 2>&1 || true
cat /tmp/acme-token-%s.txt 2>/dev/null || echo ""
`, baseDomain, baseDomain, baseDomain, baseDomain)

	output, _ := client.Run(certbotCmd)
	token := strings.TrimSpace(output)

	if token == "" || len(token) < 20 {
		return "", fmt.Errorf("failed to generate ACME token")
	}

	return token, nil
}

// CompleteWildcardSSL generates wildcard SSL certificate
// Uses a polling approach: certbot generates its token, hook polls DNS until it matches
// savedACMEToken is updated in the hook and returned via callback
func (s *VerificationService) CompleteWildcardSSL(domain string, savedACMEToken string) error {
	server, err := s.getServer()
	if err != nil {
		return fmt.Errorf("no deploy server available")
	}

	baseDomain := domain
	if strings.HasPrefix(domain, "*.") {
		baseDomain = strings.TrimPrefix(domain, "*.")
	}

	port := fmt.Sprintf("%d", server.Port)
	if server.Port == 0 {
		port = "22"
	}

	client, err := sshexec.NewClient(server.IP, port, server.User, server.Password)
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer client.Close()

	// Check if cert already exists
	checkExisting := fmt.Sprintf(`test -f /etc/letsencrypt/live/%s/fullchain.pem && echo "EXISTS"`, baseDomain)
	if out, _ := client.Run(checkExisting); strings.Contains(out, "EXISTS") {
		return nil // Already have cert
	}

	// Create auth hook that:
	// 1. Saves certbot's NEW token to a file (so UI can show it)
	// 2. Polls DNS for up to 10 minutes waiting for user to update
	// 3. Exits success when DNS matches certbot's token
	authHookScript := fmt.Sprintf(`cat > /tmp/dns-auth-poll.sh << 'HOOKEOF'
#!/bin/bash
DOMAIN="$CERTBOT_DOMAIN"
TOKEN="$CERTBOT_VALIDATION"

# Save the token so the UI can display it
echo "$TOKEN" > /tmp/acme-token-%s-live.txt

echo "ACME Challenge Token: $TOKEN"
echo "Waiting for DNS TXT record at _acme-challenge.$DOMAIN"
echo "Add TXT record: _acme-challenge.$DOMAIN = $TOKEN"

# Poll DNS for up to 10 minutes (60 attempts, 10 seconds apart)
for i in $(seq 1 60); do
    GOOGLE_VAL=$(dig @8.8.8.8 _acme-challenge.$DOMAIN TXT +short 2>/dev/null | tr -d '"')
    CF_VAL=$(dig @1.1.1.1 _acme-challenge.$DOMAIN TXT +short 2>/dev/null | tr -d '"')

    if [ "$GOOGLE_VAL" = "$TOKEN" ] || [ "$CF_VAL" = "$TOKEN" ]; then
        echo "DNS verified! Token matches."
        exit 0
    fi

    echo "Attempt $i/60: waiting for TXT record..."
    sleep 10
done

echo "Timeout: DNS not updated within 10 minutes"
exit 1
HOOKEOF
chmod +x /tmp/dns-auth-poll.sh`, baseDomain)
	client.Run(authHookScript)

	// Run certbot with polling hook - gives user 5 minutes to update DNS
	certbotCmd := fmt.Sprintf(`
certbot certonly --manual --preferred-challenges dns \
  -d "*.%s" \
  --agree-tos --email admin@%s \
  --manual-auth-hook /tmp/dns-auth-poll.sh \
  --non-interactive 2>&1
`, baseDomain, baseDomain)

	output, err := client.Run(certbotCmd)
	if err != nil {
		// Check if certbot saved a token (means it ran but DNS wasn't updated in time)
		liveTokenCmd := fmt.Sprintf(`cat /tmp/acme-token-%s-live.txt 2>/dev/null || echo ""`, baseDomain)
		liveToken, _ := client.Run(liveTokenCmd)
		liveToken = strings.TrimSpace(liveToken)
		if liveToken != "" {
			return fmt.Errorf("SSL generation timed out. Update DNS TXT record _acme-challenge.%s to: %s", baseDomain, liveToken)
		}
		return fmt.Errorf("SSL generation failed: %s", parseCertbotError(output))
	}

	// Check if certificate was created
	checkCmd := fmt.Sprintf(`
if [ -f /etc/letsencrypt/live/%s/fullchain.pem ]; then
    echo "SUCCESS"
else
    echo "FAILED"
fi
`, baseDomain)

	checkOutput, _ := client.Run(checkCmd)

	if strings.Contains(checkOutput, "SUCCESS") {
		// Reload nginx
		client.Run("nginx -t && systemctl reload nginx 2>/dev/null || true")
		return nil
	}

	return fmt.Errorf("SSL generation failed: %s", parseCertbotError(output))
}

// parseCertbotError extracts a user-friendly message from certbot output
func parseCertbotError(output string) string {
	output = strings.TrimSpace(output)

	// Check for common error patterns and return friendly messages
	switch {
	case strings.Contains(output, "NXDOMAIN") && strings.Contains(output, "_acme-challenge"):
		return "DNS TXT record not found. Please add the _acme-challenge TXT record and wait for DNS propagation (up to 10 minutes)."

	case strings.Contains(output, "DNS problem") && strings.Contains(output, "SERVFAIL"):
		return "DNS server error. Please check your DNS configuration and try again."

	case strings.Contains(output, "Timeout during connect"):
		return "Connection timeout. The server could not reach the certificate authority."

	case strings.Contains(output, "too many certificates") || strings.Contains(output, "rate limit"):
		return "Rate limit reached. Too many certificate requests for this domain. Please wait 1 hour and try again."

	case strings.Contains(output, "propagation timeout") || strings.Contains(output, "timeout after"):
		return "DNS propagation timeout. The TXT record was not detected within 10 minutes. Please verify the record is correct and try again."

	case strings.Contains(output, "failed to authenticate"):
		return "DNS verification failed. Please ensure the _acme-challenge TXT record is set correctly with the exact value shown."

	case strings.Contains(output, "Could not bind"):
		return "Port 80 is in use. Please ensure the web server is properly configured."

	case strings.Contains(output, "unauthorized"):
		return "Domain verification failed. Please check that the domain points to this server."

	default:
		// For unknown errors, return a generic message
		// Log the full output for debugging
		if len(output) > 200 {
			return "SSL generation failed. Please check DNS records and try again."
		}
		return output
	}
}

// GetBaseDomain returns the base domain for a wildcard or the domain itself
func GetBaseDomain(domain string) string {
	if strings.HasPrefix(domain, "*.") {
		return strings.TrimPrefix(domain, "*.")
	}
	return domain
}

// GetSavedAcmeToken retrieves the latest ACME token saved on the VPS
func (s *VerificationService) GetSavedAcmeToken(baseDomain string) string {
	server, err := s.getServer()
	if err != nil {
		return ""
	}

	port := fmt.Sprintf("%d", server.Port)
	if server.Port == 0 {
		port = "22"
	}

	client, err := sshexec.NewClient(server.IP, port, server.User, server.Password)
	if err != nil {
		return ""
	}
	defer client.Close()

	cmd := fmt.Sprintf(`cat /tmp/acme-token-%s.txt 2>/dev/null || echo ""`, baseDomain)
	output, err := client.Run(cmd)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(output)
}

// GetLiveAcmeToken reads the current ACME token that certbot is waiting for
// This is used when certbot is running and waiting for DNS to be updated
func (s *VerificationService) GetLiveAcmeToken(domain string) (string, error) {
	server, err := s.getServer()
	if err != nil {
		return "", fmt.Errorf("no deploy server available")
	}

	baseDomain := domain
	if strings.HasPrefix(domain, "*.") {
		baseDomain = strings.TrimPrefix(domain, "*.")
	}

	port := fmt.Sprintf("%d", server.Port)
	if server.Port == 0 {
		port = "22"
	}

	client, err := sshexec.NewClient(server.IP, port, server.User, server.Password)
	if err != nil {
		return "", fmt.Errorf("SSH connection failed: %w", err)
	}
	defer client.Close()

	// Read the live token file
	cmd := fmt.Sprintf(`cat /tmp/acme-token-%s-live.txt 2>/dev/null || echo ""`, baseDomain)
	token, err := client.Run(cmd)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(token), nil
}

// LegoChallenge holds the ACME challenge info from lego
type LegoChallenge struct {
	Token     string `json:"token"`
	Domain    string `json:"domain"`
	TXTRecord string `json:"txt_record"` // _acme-challenge.domain
}

// EnsureLegoInstalled installs lego on the server if not present
func (s *VerificationService) EnsureLegoInstalled() error {
	server, err := s.getServer()
	if err != nil {
		return fmt.Errorf("no deploy server available")
	}

	port := fmt.Sprintf("%d", server.Port)
	if server.Port == 0 {
		port = "22"
	}

	client, err := sshexec.NewClient(server.IP, port, server.User, server.Password)
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer client.Close()

	return s.ensureLegoInstalledWithClient(client)
}

// ensureLegoInstalledWithClient installs lego using an existing SSH client
func (s *VerificationService) ensureLegoInstalledWithClient(client *sshexec.Client) error {
	// Check if lego is installed
	checkCmd := "which lego >/dev/null 2>&1 && lego --version | head -1"
	if out, err := client.Run(checkCmd); err == nil && strings.Contains(out, "lego") {
		return nil // Already installed
	}

	// Install lego
	installCmd := `
cd /tmp
LEGO_VERSION="v4.14.2"
curl -fsSL "https://github.com/go-acme/lego/releases/download/${LEGO_VERSION}/lego_${LEGO_VERSION}_linux_amd64.tar.gz" -o lego.tar.gz
tar xzf lego.tar.gz lego
mv lego /usr/local/bin/
chmod +x /usr/local/bin/lego
rm -f lego.tar.gz
lego --version
`
	if _, err := client.Run(installCmd); err != nil {
		return fmt.Errorf("failed to install lego: %w", err)
	}

	return nil
}

// LegoStartChallenge starts the ACME DNS-01 challenge and returns the token
// The token is valid until LegoCompleteChallenge is called or ~7 days pass
func (s *VerificationService) LegoStartChallenge(domain, email string) (*LegoChallenge, error) {
	server, err := s.getServer()
	if err != nil {
		return nil, fmt.Errorf("no deploy server available")
	}

	baseDomain := GetBaseDomain(domain)

	port := fmt.Sprintf("%d", server.Port)
	if server.Port == 0 {
		port = "22"
	}

	client, err := sshexec.NewClient(server.IP, port, server.User, server.Password)
	if err != nil {
		return nil, fmt.Errorf("SSH connection failed: %w", err)
	}
	defer client.Close()

	// Ensure lego is installed (reuse this connection)
	if err := s.ensureLegoInstalledWithClient(client); err != nil {
		return nil, err
	}

	// Check if cert already exists - return special marker so handler can auto-complete
	checkExisting := fmt.Sprintf(`test -f /etc/letsencrypt/live/%s/fullchain.pem && echo "EXISTS"`, baseDomain)
	if out, _ := client.Run(checkExisting); strings.Contains(out, "EXISTS") {
		return &LegoChallenge{
			Token:     "CERT_EXISTS",
			Domain:    baseDomain,
			TXTRecord: "_acme-challenge." + baseDomain,
		}, nil
	}

	if email == "" {
		email = "admin@" + baseDomain
	}

	// Create lego data directory
	legoDir := fmt.Sprintf("/root/.lego-%s", baseDomain)
	client.Run(fmt.Sprintf("mkdir -p %s", legoDir))

	// Create exec DNS hook that captures the token
	// Lego exec provider calls: script.sh "present|cleanup" "_acme-challenge.domain." "token-value"
	// We don't wait - lego handles DNS propagation checking itself
	hookScript := fmt.Sprintf(`cat > /tmp/lego-dns-hook-%s.sh << 'HOOKEOF'
#!/bin/bash
ACTION="$1"
FQDN="$2"
TOKEN="$3"
DOMAIN="%s"

if [ "$ACTION" = "present" ]; then
    # Save the token for the user to see
    echo "$TOKEN" > /tmp/lego-token-${DOMAIN}.txt
    echo "FQDN=$FQDN" >> /tmp/lego-challenge-${DOMAIN}.txt
    echo "TOKEN=$TOKEN" >> /tmp/lego-challenge-${DOMAIN}.txt
    echo "READY" > /tmp/lego-status-${DOMAIN}.txt
fi

# Exit immediately - lego will poll DNS servers itself
exit 0
HOOKEOF
chmod +x /tmp/lego-dns-hook-%s.sh`, baseDomain, baseDomain, baseDomain)
	client.Run(hookScript)

	// Clear previous state
	client.Run(fmt.Sprintf(`rm -f /tmp/lego-token-%s.txt /tmp/lego-status-%s.txt /tmp/lego-continue-%s.txt /tmp/lego-challenge-%s.txt`, baseDomain, baseDomain, baseDomain, baseDomain))

	// Start lego in background with exec DNS provider
	// Only request wildcard cert (base domain may point elsewhere)
	// Long propagation timeout gives user time to add DNS record
	legoCmd := fmt.Sprintf(`nohup env EXEC_PATH=/tmp/lego-dns-hook-%s.sh \
		EXEC_PROPAGATION_TIMEOUT=43200 \
		EXEC_POLLING_INTERVAL=30 \
		lego --accept-tos --email="%s" \
		--domains="*.%s" \
		--dns exec \
		--path=%s \
		run > /tmp/lego-output-%s.txt 2>&1 &
echo $!`, baseDomain, email, baseDomain, legoDir, baseDomain)

	pidOut, err := client.Run(legoCmd)
	if err != nil {
		return nil, fmt.Errorf("failed to start lego: %w", err)
	}

	// Save PID for later
	pid := strings.TrimSpace(pidOut)
	client.Run(fmt.Sprintf(`echo "%s" > /tmp/lego-pid-%s.txt`, pid, baseDomain))

	// Wait for token to be captured (up to 30 seconds)
	for i := 0; i < 30; i++ {
		time.Sleep(1 * time.Second)
		statusCmd := fmt.Sprintf(`cat /tmp/lego-status-%s.txt 2>/dev/null || echo ""`, baseDomain)
		status, _ := client.Run(statusCmd)
		if strings.Contains(status, "READY") {
			break
		}
	}

	// Read the captured token
	tokenCmd := fmt.Sprintf(`cat /tmp/lego-token-%s.txt 2>/dev/null || echo ""`, baseDomain)
	token, _ := client.Run(tokenCmd)
	token = strings.TrimSpace(token)

	if token == "" {
		// Check lego output for errors
		outputCmd := fmt.Sprintf(`cat /tmp/lego-output-%s.txt 2>/dev/null | tail -20`, baseDomain)
		output, _ := client.Run(outputCmd)
		return nil, fmt.Errorf("failed to get ACME token: %s", output)
	}

	return &LegoChallenge{
		Token:     token,
		Domain:    baseDomain,
		TXTRecord: "_acme-challenge." + baseDomain,
	}, nil
}

// LegoCompleteChallenge checks if cert is ready and installs it
func (s *VerificationService) LegoCompleteChallenge(domain string) error {
	server, err := s.getServer()
	if err != nil {
		return fmt.Errorf("no deploy server available")
	}

	baseDomain := GetBaseDomain(domain)

	port := fmt.Sprintf("%d", server.Port)
	if server.Port == 0 {
		port = "22"
	}

	client, err := sshexec.NewClient(server.IP, port, server.User, server.Password)
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer client.Close()

	legoDir := fmt.Sprintf("/root/.lego-%s", baseDomain)

	// Check if cert already exists (wildcard cert is named _.domain.crt)
	checkCert := fmt.Sprintf(`test -f %s/certificates/_.%s.crt && echo "EXISTS"`, legoDir, baseDomain)
	if out, _ := client.Run(checkCert); strings.Contains(out, "EXISTS") {
		// Cert exists - install it
		return s.installLegoCert(client, baseDomain)
	}

	// Check if lego process is still running
	pidCmd := fmt.Sprintf(`cat /tmp/lego-pid-%s.txt 2>/dev/null || echo ""`, baseDomain)
	pid, _ := client.Run(pidCmd)
	pid = strings.TrimSpace(pid)

	if pid != "" {
		checkPid := fmt.Sprintf(`ps -p %s >/dev/null 2>&1 && echo "RUNNING" || echo "STOPPED"`, pid)
		status, _ := client.Run(checkPid)

		if strings.Contains(status, "RUNNING") {
			// Lego is still running - waiting for DNS propagation
			return fmt.Errorf("SSL generation in progress - lego is waiting for DNS propagation")
		}

		// Process stopped but no cert - check for errors
		outputCmd := fmt.Sprintf(`tail -20 /tmp/lego-output-%s.txt 2>/dev/null`, baseDomain)
		output, _ := client.Run(outputCmd)
		if strings.Contains(output, "error") || strings.Contains(output, "Error") || strings.Contains(output, "ERRO") {
			return fmt.Errorf("SSL generation failed: %s", output)
		}
	}

	return fmt.Errorf("no SSL certificate found for %s - click 'Get SSL Token' to start", baseDomain)
}

// installLegoCert copies lego cert to Let's Encrypt location and reloads nginx
func (s *VerificationService) installLegoCert(client *sshexec.Client, baseDomain string) error {
	legoDir := fmt.Sprintf("/root/.lego-%s", baseDomain)
	letsencryptDir := fmt.Sprintf("/etc/letsencrypt/live/%s", baseDomain)
	// Wildcard certs are named _.domain.crt by lego
	wildcardCertName := fmt.Sprintf("_.%s", baseDomain)

	installCmd := fmt.Sprintf(`
mkdir -p %s
cp %s/certificates/%s.crt %s/fullchain.pem
cp %s/certificates/%s.key %s/privkey.pem
chmod 600 %s/privkey.pem
nginx -t && systemctl reload nginx 2>/dev/null || true
echo "SUCCESS"
`, letsencryptDir, legoDir, wildcardCertName, letsencryptDir, legoDir, wildcardCertName, letsencryptDir, letsencryptDir)

	out, err := client.Run(installCmd)
	if err != nil || !strings.Contains(out, "SUCCESS") {
		return fmt.Errorf("failed to install certificate: %v", err)
	}

	// Cleanup temp files
	client.Run(fmt.Sprintf(`rm -f /tmp/lego-token-%s.txt /tmp/lego-status-%s.txt /tmp/lego-pid-%s.txt /tmp/lego-challenge-%s.txt`, baseDomain, baseDomain, baseDomain, baseDomain))

	return nil
}

// LegoGetPendingToken returns the pending ACME token for a domain (if any)
func (s *VerificationService) LegoGetPendingToken(domain string) (string, error) {
	server, err := s.getServer()
	if err != nil {
		return "", fmt.Errorf("no deploy server available")
	}

	baseDomain := GetBaseDomain(domain)

	port := fmt.Sprintf("%d", server.Port)
	if server.Port == 0 {
		port = "22"
	}

	client, err := sshexec.NewClient(server.IP, port, server.User, server.Password)
	if err != nil {
		return "", fmt.Errorf("SSH connection failed: %w", err)
	}
	defer client.Close()

	tokenCmd := fmt.Sprintf(`cat /tmp/lego-token-%s.txt 2>/dev/null || echo ""`, baseDomain)
	token, _ := client.Run(tokenCmd)
	return strings.TrimSpace(token), nil
}

// LegoCancelChallenge cancels a pending SSL challenge
func (s *VerificationService) LegoCancelChallenge(domain string) error {
	server, err := s.getServer()
	if err != nil {
		return fmt.Errorf("no deploy server available")
	}

	baseDomain := GetBaseDomain(domain)

	port := fmt.Sprintf("%d", server.Port)
	if server.Port == 0 {
		port = "22"
	}

	client, err := sshexec.NewClient(server.IP, port, server.User, server.Password)
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer client.Close()

	// Kill lego process if running
	pidCmd := fmt.Sprintf(`cat /tmp/lego-pid-%s.txt 2>/dev/null || echo ""`, baseDomain)
	pid, _ := client.Run(pidCmd)
	pid = strings.TrimSpace(pid)
	if pid != "" {
		client.Run(fmt.Sprintf(`kill %s 2>/dev/null || true`, pid))
	}

	// Cleanup temp files
	client.Run(fmt.Sprintf(`rm -f /tmp/lego-token-%s.txt /tmp/lego-status-%s.txt /tmp/lego-pid-%s.txt /tmp/lego-continue-%s.txt /tmp/lego-challenge-%s.txt`, baseDomain, baseDomain, baseDomain, baseDomain, baseDomain))

	return nil
}
