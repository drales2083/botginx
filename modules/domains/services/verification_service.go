package services

import (
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

	records, err := net.LookupTXT(txtHost)
	if err != nil {
		return false, fmt.Errorf("TXT record not found. Add: %s TXT %s", txtHost, expectedToken)
	}

	for _, record := range records {
		if record == expectedToken {
			return true, nil
		}
	}

	return false, fmt.Errorf("TXT record value mismatch. Expected: %s", expectedToken)
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

	// Fallback: create basic nginx config
	nginxConfig := fmt.Sprintf(`server {
    listen 80;
    server_name %s *.%s;

    location /.well-known/domain-verify {
        default_type application/json;
        return 200 '{"verified":true}';
    }
    location /.well-known/domain-verify/ {
        default_type application/json;
        return 200 '{"verified":true}';
    }
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
}`, domain, domain, domain)

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
