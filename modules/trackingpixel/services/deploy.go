package services

import (
	"fmt"
	"strings"

	"github.com/botginx/botginx/pkg/sshexec"
)

// ServerInfo holds SSH connection details for Deploy VPS
type ServerInfo struct {
	IP       string
	Port     int
	User     string
	Password string
}

// DeployService handles tracking pixel subdomain deployment to Deploy VPS
type DeployService struct {
	getServerFunc func() (*ServerInfo, error)
	guardHost     string // Guard VPS hostname (e.g., guardbot.sbs)
}

// NewDeployService creates a new deploy service
func NewDeployService(guardHost string) *DeployService {
	return &DeployService{guardHost: guardHost}
}

// SetServerProvider sets the function to get Deploy VPS credentials
func (s *DeployService) SetServerProvider(fn func() (*ServerInfo, error)) {
	s.getServerFunc = fn
}

func (s *DeployService) getServer() (*ServerInfo, error) {
	if s.getServerFunc != nil {
		return s.getServerFunc()
	}
	return nil, fmt.Errorf("no deploy server configured")
}

// SetupPixelSubdomain creates nginx config for px.{domain} on Deploy VPS
// that proxies to Guard VPS where botginx serves /t/{token}
func (s *DeployService) SetupPixelSubdomain(domain string) error {
	server, err := s.getServer()
	if err != nil {
		return fmt.Errorf("no deploy server available: %w", err)
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

	// Get base domain (strip wildcard prefix if present)
	baseDomain := domain
	if strings.HasPrefix(domain, "*.") {
		baseDomain = strings.TrimPrefix(domain, "*.")
	}

	pxDomain := "px." + baseDomain

	// Check if config already exists
	checkCmd := fmt.Sprintf("test -f /etc/nginx/sites-enabled/%s.conf && echo EXISTS || echo MISSING", pxDomain)
	out, _ := client.Run(checkCmd)
	if strings.TrimSpace(out) == "EXISTS" {
		return nil // Already configured
	}

	// Get SSL cert path - use parent domain's wildcard cert
	// Check for Let's Encrypt wildcard cert first, then self-signed
	certPath := fmt.Sprintf("/etc/letsencrypt/live/%s/fullchain.pem", baseDomain)
	keyPath := fmt.Sprintf("/etc/letsencrypt/live/%s/privkey.pem", baseDomain)

	checkCert := fmt.Sprintf("test -f %s && echo LE || echo MISSING", certPath)
	certOut, _ := client.Run(checkCert)
	if strings.TrimSpace(certOut) != "LE" {
		// Try self-signed
		certPath = fmt.Sprintf("/etc/nginx/ssl/%s.pem", baseDomain)
		keyPath = fmt.Sprintf("/etc/nginx/ssl/%s.key", baseDomain)
	}

	// Create nginx config for px.{domain} that proxies to Guard VPS
	// All traffic goes to guardbot.sbs which has the /t/{token} endpoint
	nginxConfig := fmt.Sprintf(`server {
    listen 80;
    listen 443 ssl;
    server_name %s;

    ssl_certificate %s;
    ssl_certificate_key %s;

    location / {
        proxy_pass https://%s;
        proxy_http_version 1.1;
        proxy_set_header Host %s;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Original-Host $host;
        proxy_ssl_verify off;
    }
}`, pxDomain, certPath, keyPath, s.guardHost, s.guardHost)

	// Write nginx config
	configPath := fmt.Sprintf("/etc/nginx/sites-available/%s.conf", pxDomain)
	writeCmd := fmt.Sprintf("cat > %s << 'NGINXEOF'\n%s\nNGINXEOF", configPath, nginxConfig)
	if _, err := client.Run(writeCmd); err != nil {
		return fmt.Errorf("failed to write nginx config: %w", err)
	}

	// Enable site and reload nginx
	enableCmd := fmt.Sprintf("ln -sf %s /etc/nginx/sites-enabled/ && nginx -t && systemctl reload nginx", configPath)
	if _, err := client.Run(enableCmd); err != nil {
		return fmt.Errorf("failed to enable site: %w", err)
	}

	return nil
}

// CheckPixelSubdomainExists checks if px.{domain} nginx config exists
func (s *DeployService) CheckPixelSubdomainExists(domain string) (bool, error) {
	server, err := s.getServer()
	if err != nil {
		return false, err
	}

	port := fmt.Sprintf("%d", server.Port)
	if server.Port == 0 {
		port = "22"
	}

	client, err := sshexec.NewClient(server.IP, port, server.User, server.Password)
	if err != nil {
		return false, err
	}
	defer client.Close()

	baseDomain := domain
	if strings.HasPrefix(domain, "*.") {
		baseDomain = strings.TrimPrefix(domain, "*.")
	}
	pxDomain := "px." + baseDomain

	checkCmd := fmt.Sprintf("test -f /etc/nginx/sites-enabled/%s.conf && echo EXISTS || echo MISSING", pxDomain)
	out, _ := client.Run(checkCmd)
	return strings.TrimSpace(out) == "EXISTS", nil
}

// CleanupPixelSubdomain removes nginx config for px.{domain}
func (s *DeployService) CleanupPixelSubdomain(domain string) error {
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

	baseDomain := GetBaseDomain(domain)
	pxDomain := "px." + baseDomain

	cleanupCmd := fmt.Sprintf(`
rm -f /etc/nginx/sites-enabled/%s.conf
rm -f /etc/nginx/sites-available/%s.conf
nginx -t && systemctl reload nginx 2>/dev/null || true
`, pxDomain, pxDomain)
	_, err = client.Run(cleanupCmd)
	return err
}

// GetBaseDomain returns the base domain, stripping wildcard prefix if present.
func GetBaseDomain(domain string) string {
	if strings.HasPrefix(domain, "*.") {
		return strings.TrimPrefix(domain, "*.")
	}
	return domain
}
