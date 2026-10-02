package services

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/botginx/botginx/pkg/sshexec"
	"github.com/jmoiron/sqlx"
)

type HealthService struct {
	db            *sqlx.DB
	safetyChecker *SafetyChecker
}

func NewHealthService(db *sqlx.DB) *HealthService {
	return &HealthService{
		db:            db,
		safetyChecker: NewSafetyChecker(""), // Initialize with no Google key (URLhaus + PhishTank still work)
	}
}

// SetGoogleAPIKey sets the Google Safe Browsing API key
func (s *HealthService) SetGoogleAPIKey(apiKey string) {
	s.safetyChecker = NewSafetyChecker(apiKey)
}

type Domain struct {
	ID              string     `db:"id"`
	Name            string     `db:"name"`
	DNSVerified     bool       `db:"dns_verified"`
	SSLEnabled      bool       `db:"ssl_enabled"`
	ServerID        *string    `db:"server_id"`
	VerifyToken     string     `db:"verify_token"`
	CreatedAt       time.Time  `db:"created_at"`
	SafetyStatus    string     `db:"safety_status"`
	SafetyCheckedAt *time.Time `db:"safety_checked_at"`
	SafetyThreats   []byte     `db:"safety_threats"`
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
	// Safety check results
	SafetyStatus  string
	SafetyMessage string
	Threats       []Threat
	SafetyCached  bool
}

func (s *HealthService) GetUserDomains(userID string) ([]Domain, error) {
	var domains []Domain
	err := s.db.Select(&domains, `
		SELECT id, name, dns_verified, ssl_enabled, server_id, verify_token, created_at,
		       COALESCE(safety_status, 'unknown') as safety_status,
		       safety_checked_at,
		       COALESCE(safety_threats, '[]'::jsonb) as safety_threats
		FROM domains
		WHERE user_id = $1
		  AND COALESCE(is_marketplace, FALSE) = FALSE
		  AND is_shared = FALSE
		ORDER BY name
	`, userID)
	return domains, err
}

func (s *HealthService) GetDomain(id, userID string) (*Domain, error) {
	var domain Domain
	err := s.db.Get(&domain, `
		SELECT id, name, dns_verified, ssl_enabled, server_id, verify_token, created_at,
		       COALESCE(safety_status, 'unknown') as safety_status,
		       safety_checked_at,
		       COALESCE(safety_threats, '[]'::jsonb) as safety_threats
		FROM domains
		WHERE id = $1 AND user_id = $2
	`, id, userID)
	if err != nil {
		return nil, err
	}
	return &domain, nil
}

// getDeployServer gets deploy VPS credentials from database
func (s *HealthService) getDeployServer() (ip string, port int, user string, password string, err error) {
	var server struct {
		IP       string `db:"ip"`
		Port     int    `db:"port"`
		User     string `db:"ssh_user"`
		Password string `db:"ssh_password"`
	}
	err = s.db.Get(&server, `
		SELECT ip, port, ssh_user, ssh_password
		FROM servers
		WHERE status = 'ready'
		ORDER BY created_at LIMIT 1
	`)
	if err != nil {
		return "", 0, "", "", err
	}
	return server.IP, server.Port, server.User, server.Password, nil
}

// stripWildcard removes the *. prefix from wildcard domains
func stripWildcard(domain string) string {
	if strings.HasPrefix(domain, "*.") {
		return domain[2:]
	}
	return domain
}

func (s *HealthService) CheckDomain(domain Domain) HealthStatus {
	status := HealthStatus{Domain: domain}

	// Check DNS: verify our TXT record is still present
	status.DNSStatus, status.DNSMessage = s.checkDNS(domain.Name, domain.VerifyToken)

	// Check SSL: verify certificate exists on our VPS
	status.SSLStatus, status.SSLMessage, status.SSLExpiry = s.checkSSLOnVPS(domain.Name)

	// Check HTTP: verify domain is accessible via our VPS
	status.HTTPStatus, status.HTTPMessage, status.ResponseTime = s.checkHTTP(domain.Name)

	// Check Safety: use cached result if <24h old, otherwise fresh check
	status.SafetyStatus, status.SafetyMessage, status.Threats, status.SafetyCached = s.checkSafety(domain)

	return status
}

// checkSafety checks domain safety using cached results or fresh API calls
func (s *HealthService) checkSafety(domain Domain) (status string, message string, threats []Threat, cached bool) {
	// Check if we have a recent cached result (< 24 hours old)
	if domain.SafetyCheckedAt != nil && time.Since(*domain.SafetyCheckedAt) < 24*time.Hour {
		// Use cached result
		var cachedThreats []Threat
		if len(domain.SafetyThreats) > 0 {
			json.Unmarshal(domain.SafetyThreats, &cachedThreats)
		}

		if domain.SafetyStatus == "flagged" {
			return "error", fmt.Sprintf("%d threat(s) detected", len(cachedThreats)), cachedThreats, true
		}
		return "ok", "Clean", nil, true
	}

	// No cache or expired - perform fresh check

	result := s.safetyChecker.CheckDomain(domain.Name)

	// Save to database
	threatsJSON, _ := json.Marshal(result.Threats)
	s.db.Exec(`
		UPDATE domains
		SET safety_status = $1, safety_checked_at = $2, safety_threats = $3
		WHERE id = $4
	`, string(result.Status), result.CheckedAt, threatsJSON, domain.ID)

	if result.Status == SafetyFlagged {
		return "error", fmt.Sprintf("%d threat(s) detected", len(result.Threats)), result.Threats, false
	}
	return "ok", "Clean", nil, false
}

// checkDNS verifies our TXT record is still present (same method as VerificationService.VerifyDNS)
func (s *HealthService) checkDNS(domain, expectedToken string) (string, string) {
	// For wildcard domains, use the base domain
	baseDomain := stripWildcard(domain)

	// Look up TXT record at _guardbot-verify.domain.com
	txtHost := "_guardbot-verify." + baseDomain

	records, err := net.LookupTXT(txtHost)
	if err != nil {
		return "error", "TXT lookup failed"
	}

	for _, record := range records {
		if strings.TrimSpace(record) == expectedToken {
			return "ok", "Verified"
		}
	}

	if len(records) > 0 {
		return "warning", "TXT mismatch"
	}
	return "error", "TXT not found"
}

// checkSSLOnVPS checks SSL certificate on the deploy VPS (same as VerificationService.CheckSSL)
func (s *HealthService) checkSSLOnVPS(domain string) (string, string, *time.Time) {
	// Strip wildcard prefix - certs are stored at base domain path
	baseDomain := stripWildcard(domain)

	ip, port, user, password, err := s.getDeployServer()
	if err != nil {
		return "error", "No deploy server", nil
	}

	portStr := fmt.Sprintf("%d", port)
	if port == 0 {
		portStr = "22"
	}

	client, err := sshexec.NewClient(ip, portStr, user, password)
	if err != nil {
		return "error", "Server connection failed", nil
	}
	defer client.Close()

	// Check if cert exists (same path as verification service)
	certPath := fmt.Sprintf("/etc/letsencrypt/live/%s/fullchain.pem", baseDomain)
	checkCmd := fmt.Sprintf("test -f %s && echo EXISTS || echo MISSING", certPath)

	output, err := client.Run(checkCmd)
	if err != nil {
		return "error", "Check failed", nil
	}

	if strings.TrimSpace(output) != "EXISTS" {
		return "error", "Certificate not found", nil
	}

	// Get expiry date
	infoCmd := fmt.Sprintf(`openssl x509 -in %s -noout -enddate 2>/dev/null | cut -d= -f2`, certPath)
	info, _ := client.Run(infoCmd)
	info = strings.TrimSpace(info)

	if info != "" {
		// Parse expiry: "Mar 15 12:00:00 2025 GMT"
		expiry, err := time.Parse("Jan 2 15:04:05 2006 MST", info)
		if err == nil {
			daysLeft := int(time.Until(expiry).Hours() / 24)
			if daysLeft < 0 {
				return "error", "Expired", &expiry
			}
			if daysLeft < 14 {
				return "warning", fmt.Sprintf("Expires in %d days", daysLeft), &expiry
			}
			return "ok", fmt.Sprintf("Valid (%d days)", daysLeft), &expiry
		}
	}

	return "ok", "Certificate exists", nil
}

// checkHTTP tests if domain responds to HTTP/HTTPS
func (s *HealthService) checkHTTP(domain string) (string, string, int64) {
	baseDomain := stripWildcard(domain)

	// Use dig or curl on the VPS to check connectivity
	ip, port, user, password, err := s.getDeployServer()
	if err != nil {
		return "error", "No deploy server", 0
	}

	portStr := fmt.Sprintf("%d", port)
	if port == 0 {
		portStr = "22"
	}

	client, err := sshexec.NewClient(ip, portStr, user, password)
	if err != nil {
		return "error", "Server connection failed", 0
	}
	defer client.Close()

	// Check nginx config exists and test with curl
	start := time.Now()
	cmd := fmt.Sprintf(`curl -sS -o /dev/null -w "%%{http_code}" --max-time 10 -k https://%s 2>/dev/null || echo "000"`, baseDomain)
	output, err := client.Run(cmd)
	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		return "error", "Check failed", elapsed
	}

	code := strings.TrimSpace(output)
	switch {
	case code == "000":
		return "error", "Connection failed", elapsed
	case code == "200", code == "301", code == "302", code == "304":
		return "ok", fmt.Sprintf("HTTP %s", code), elapsed
	case strings.HasPrefix(code, "4"):
		return "warning", fmt.Sprintf("HTTP %s", code), elapsed
	case strings.HasPrefix(code, "5"):
		return "error", fmt.Sprintf("HTTP %s", code), elapsed
	default:
		return "ok", fmt.Sprintf("HTTP %s", code), elapsed
	}
}
