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
	// Look up TXT record at _guardbot-verify.domain.com
	txtHost := "_guardbot-verify." + domain

	// Try multiple DNS servers to avoid caching issues
	dnsServers := []string{"8.8.8.8:53", "1.1.1.1:53", ""}
	var records []string
	var lastErr error

	for _, dnsServer := range dnsServers {
		var err error
		if dnsServer == "" {
			// Use system resolver as fallback
			records, err = net.LookupTXT(txtHost)
		} else {
			resolver := &net.Resolver{
				PreferGo: true,
				Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
					d := net.Dialer{Timeout: 3 * time.Second}
					return d.DialContext(ctx, "udp", dnsServer)
				},
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			records, err = resolver.LookupTXT(ctx, txtHost)
			cancel()
		}

		if err == nil && len(records) > 0 {
			for _, record := range records {
				if strings.TrimSpace(record) == expectedToken {
					return true, nil
				}
			}
			// Show which DNS server we checked
			dnsSource := "Google DNS"
			if dnsServer == "1.1.1.1:53" {
				dnsSource = "Cloudflare DNS"
			} else if dnsServer == "" {
				dnsSource = "System DNS"
			}
			return false, fmt.Errorf("TXT mismatch (%s). Expected: %s, Found: [%s]. Delete old record, wait 5-30 min for propagation", dnsSource, expectedToken, strings.Join(records, ", "))
		}
		lastErr = err
	}

	_ = lastErr // ignore, we show helpful message
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
		return fmt.Errorf("certbot failed: %s", output)
	}

	client.Run("nginx -t && systemctl reload nginx")
	return nil
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

	// Remove nginx config
	nginxCmd := fmt.Sprintf(`
rm -f /etc/nginx/sites-enabled/%s.conf
rm -f /etc/nginx/sites-available/%s.conf
`, domain, domain)
	client.Run(nginxCmd)

	// Remove self-signed SSL cert
	sslCmd := fmt.Sprintf(`
rm -f /etc/nginx/ssl/%s.pem
rm -f /etc/nginx/ssl/%s.key
`, domain, domain)
	client.Run(sslCmd)

	// Remove Let's Encrypt cert if exists
	letsEncryptCmd := fmt.Sprintf(`
if [ -d /etc/letsencrypt/live/%s ]; then
    certbot delete --cert-name %s --non-interactive 2>/dev/null || true
fi
`, domain, domain)
	client.Run(letsEncryptCmd)

	// Remove site directories
	siteDirCmd := fmt.Sprintf("rm -rf /var/www/sites/%s", domain)
	client.Run(siteDirCmd)

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

// PreGenerateAcmeToken starts certbot to get an ACME challenge token for wildcard SSL
// Returns the token that user needs to add as TXT record
func (s *VerificationService) PreGenerateAcmeToken(domain string) (string, error) {
	server, err := s.getServer()
	if err != nil {
		return "", fmt.Errorf("no deploy server available")
	}

	// For wildcard domains, use base domain
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

	// Create auth hook that captures and saves the token, then waits
	setupCmd := `
cat > /tmp/dns-auth-capture.sh << 'HOOKEOF'
#!/bin/bash
echo "$CERTBOT_VALIDATION" > /tmp/acme-token-$CERTBOT_DOMAIN.txt
# Wait for external verification (10 minutes max)
sleep 600
HOOKEOF
chmod +x /tmp/dns-auth-capture.sh
`
	client.Run(setupCmd)

	// Start certbot in background to capture the token
	certbotCmd := fmt.Sprintf(`
rm -f /tmp/acme-token-%s.txt
nohup certbot certonly --manual --preferred-challenges dns \
  -d "*.%s" \
  --agree-tos --email admin@%s \
  --manual-auth-hook /tmp/dns-auth-capture.sh \
  > /tmp/certbot-%s.log 2>&1 &
sleep 3
cat /tmp/acme-token-%s.txt 2>/dev/null || echo ""
`, baseDomain, baseDomain, baseDomain, baseDomain, baseDomain)

	output, err := client.Run(certbotCmd)
	if err != nil {
		return "", fmt.Errorf("failed to start certbot: %w", err)
	}

	token := strings.TrimSpace(output)
	if token == "" {
		return "", fmt.Errorf("token not generated yet, try again in a few seconds")
	}

	return token, nil
}

// CompleteWildcardSSL kills the waiting certbot process to let it verify
// Call this after user has added the TXT record and it has propagated
func (s *VerificationService) CompleteWildcardSSL(domain string) error {
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

	// Kill the sleep process to let certbot continue
	client.Run("pkill -f 'sleep 600'")

	// Wait a moment for certbot to verify
	time.Sleep(5 * time.Second)

	// Check if certificate was created
	checkCmd := fmt.Sprintf(`
if [ -f /etc/letsencrypt/live/%s/fullchain.pem ]; then
    echo "SUCCESS"
else
    cat /tmp/certbot-%s.log 2>/dev/null | tail -10
fi
`, baseDomain, baseDomain)

	output, err := client.Run(checkCmd)
	if err != nil {
		return fmt.Errorf("failed to check SSL status: %w", err)
	}

	if strings.Contains(output, "SUCCESS") {
		// Reload nginx
		client.Run("nginx -t && systemctl reload nginx 2>/dev/null || true")
		return nil
	}

	return fmt.Errorf("SSL generation failed: %s", strings.TrimSpace(output))
}

// GetBaseDomain returns the base domain for a wildcard or the domain itself
func GetBaseDomain(domain string) string {
	if strings.HasPrefix(domain, "*.") {
		return strings.TrimPrefix(domain, "*.")
	}
	return domain
}
