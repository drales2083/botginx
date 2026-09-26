package services

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

type HealthService struct {
	db *sqlx.DB
}

func NewHealthService(db *sqlx.DB) *HealthService {
	return &HealthService{db: db}
}

type Domain struct {
	ID          string    `db:"id"`
	Name        string    `db:"name"`
	DNSVerified bool      `db:"dns_verified"`
	SSLEnabled  bool      `db:"ssl_enabled"`
	ServerID    *string   `db:"server_id"`
	CreatedAt   time.Time `db:"created_at"`
}

type HealthStatus struct {
	Domain       Domain
	DNSStatus    string
	DNSMessage   string
	SSLStatus    string
	SSLMessage   string
	SSLExpiry    *time.Time
	HTTPStatus   string
	HTTPMessage  string
	ResponseTime int64
}

func (s *HealthService) GetUserDomains(userID string) ([]Domain, error) {
	var domains []Domain
	err := s.db.Select(&domains, `
		SELECT id, name, dns_verified, ssl_enabled, server_id, created_at
		FROM domains
		WHERE user_id = $1
		ORDER BY name
	`, userID)
	return domains, err
}

func (s *HealthService) GetDomain(id, userID string) (*Domain, error) {
	var domain Domain
	err := s.db.Get(&domain, `
		SELECT id, name, dns_verified, ssl_enabled, server_id, created_at
		FROM domains
		WHERE id = $1 AND user_id = $2
	`, id, userID)
	if err != nil {
		return nil, err
	}
	return &domain, nil
}

// stripWildcard removes the *. prefix from wildcard domains for health checks
func stripWildcard(domain string) string {
	if strings.HasPrefix(domain, "*.") {
		return domain[2:]
	}
	return domain
}

func (s *HealthService) CheckDomain(domain Domain) HealthStatus {
	status := HealthStatus{Domain: domain}

	// Strip wildcard prefix for actual health checks
	checkName := stripWildcard(domain.Name)

	// Check DNS
	status.DNSStatus, status.DNSMessage = checkDNS(checkName)

	// Check SSL
	status.SSLStatus, status.SSLMessage, status.SSLExpiry = checkSSL(checkName)

	// Check HTTP
	status.HTTPStatus, status.HTTPMessage, status.ResponseTime = checkHTTP(checkName)

	return status
}

func checkDNS(domain string) (string, string) {
	ips, err := net.LookupIP(domain)
	if err != nil {
		return "error", fmt.Sprintf("DNS lookup failed: %v", err)
	}
	if len(ips) == 0 {
		return "error", "No DNS records found"
	}

	var ipList string
	for i, ip := range ips {
		if i > 0 {
			ipList += ", "
		}
		ipList += ip.String()
		if i >= 2 {
			ipList += "..."
			break
		}
	}
	return "ok", fmt.Sprintf("Resolves to %s", ipList)
}

func checkSSL(domain string) (string, string, *time.Time) {
	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 10 * time.Second},
		"tcp",
		domain+":443",
		&tls.Config{InsecureSkipVerify: true},
	)
	if err != nil {
		return "error", fmt.Sprintf("SSL connection failed: %v", err), nil
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return "error", "No SSL certificate found", nil
	}

	cert := certs[0]
	expiry := cert.NotAfter

	daysUntilExpiry := int(time.Until(expiry).Hours() / 24)

	if daysUntilExpiry < 0 {
		return "error", "Certificate expired", &expiry
	}
	if daysUntilExpiry < 7 {
		return "warning", fmt.Sprintf("Expires in %d days", daysUntilExpiry), &expiry
	}
	if daysUntilExpiry < 30 {
		return "warning", fmt.Sprintf("Expires in %d days", daysUntilExpiry), &expiry
	}

	return "ok", fmt.Sprintf("Valid, expires in %d days", daysUntilExpiry), &expiry
}

func checkHTTP(domain string) (string, string, int64) {
	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	start := time.Now()
	resp, err := client.Get("https://" + domain)
	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		// Try HTTP if HTTPS fails
		start = time.Now()
		resp, err = client.Get("http://" + domain)
		elapsed = time.Since(start).Milliseconds()
		if err != nil {
			return "error", fmt.Sprintf("Connection failed: %v", err), elapsed
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return "error", fmt.Sprintf("Server error: %d", resp.StatusCode), elapsed
	}
	if resp.StatusCode >= 400 {
		return "warning", fmt.Sprintf("Client error: %d", resp.StatusCode), elapsed
	}

	return "ok", fmt.Sprintf("HTTP %d (%dms)", resp.StatusCode, elapsed), elapsed
}
