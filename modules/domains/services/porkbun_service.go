package services

import (
	"context"
	"fmt"

	"github.com/botginx/botginx/pkg/providers/porkbun"
	"github.com/jmoiron/sqlx"
)

// PorkbunService handles DNS management for domains purchased from Porkbun
// Unlike NameSilo/Cloudflare which use per-user API keys, Porkbun domains
// are purchased into the admin's account, so we use a single API key.
type PorkbunService struct {
	db        *sqlx.DB
	apiKey    string
	apiSecret string
	client    *porkbun.Client
}

// NewPorkbunService creates a new Porkbun service
func NewPorkbunService(db *sqlx.DB, apiKey, apiSecret string) *PorkbunService {
	var client *porkbun.Client
	if apiKey != "" && apiSecret != "" {
		client = porkbun.NewClient(apiKey, apiSecret)
	}
	return &PorkbunService{
		db:        db,
		apiKey:    apiKey,
		apiSecret: apiSecret,
		client:    client,
	}
}

// SetCredentials updates the API credentials
func (s *PorkbunService) SetCredentials(apiKey, apiSecret string) {
	s.apiKey = apiKey
	s.apiSecret = apiSecret
	if apiKey != "" && apiSecret != "" {
		s.client = porkbun.NewClient(apiKey, apiSecret)
	}
}

// GetClient returns the Porkbun client
func (s *PorkbunService) GetClient() *porkbun.Client {
	return s.client
}

// SetupDNS configures DNS records for a domain (@ and * A records)
func (s *PorkbunService) SetupDNS(ctx context.Context, domainName, serverIP string) error {
	if s.client == nil {
		return fmt.Errorf("porkbun client not configured")
	}

	_, _, err := s.client.SetupDNS(ctx, domainName, serverIP)
	return err
}

// ReconfigureAllDomains updates DNS for all Porkbun domains
// Called when the deploy server IP changes
func (s *PorkbunService) ReconfigureAllDomains(ctx context.Context, newIP string) ([]ReconfigureResult, error) {
	if s.client == nil {
		return nil, fmt.Errorf("porkbun client not configured")
	}

	// Find all domains with source='porkbun' and dns_auto_configured=true
	type domainInfo struct {
		ID          string `db:"id"`
		Name        string `db:"name"`
		UserID      string `db:"user_id"`
		VerifyToken string `db:"verify_token"`
	}

	var domains []domainInfo
	err := s.db.SelectContext(ctx, &domains, `
		SELECT id, name, user_id, verify_token
		FROM domains
		WHERE source = 'porkbun' AND dns_auto_configured = true
	`)
	if err != nil {
		return nil, fmt.Errorf("querying porkbun domains: %w", err)
	}

	if len(domains) == 0 {
		return []ReconfigureResult{}, nil
	}

	var results []ReconfigureResult

	// Reconfigure each domain
	for _, d := range domains {
		_, _, err := s.client.SetupDNS(ctx, d.Name, newIP)
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
				s.client.CreateDNSRecord(ctx, d.Name, porkbun.DNSRecordInput{
					Type:    "TXT",
					Name:    "_guardbot-verify",
					Content: d.VerifyToken,
					TTL:     600,
				})
			}

			results = append(results, ReconfigureResult{
				Domain:  d.Name,
				Success: true,
			})
			s.db.Exec(`
				UPDATE domains SET dns_config_error = NULL, dns_verified = true, updated_at = NOW()
				WHERE id = $1
			`, d.ID)
		}
	}

	return results, nil
}

// ReconfigureDomain updates DNS for a single domain
func (s *PorkbunService) ReconfigureDomain(ctx context.Context, domainID, serverIP string) error {
	if s.client == nil {
		return fmt.Errorf("porkbun client not configured")
	}

	var domain struct {
		Name string `db:"name"`
	}
	err := s.db.GetContext(ctx, &domain, `SELECT name FROM domains WHERE id = $1`, domainID)
	if err != nil {
		return fmt.Errorf("domain not found: %w", err)
	}

	_, _, err = s.client.SetupDNS(ctx, domain.Name, serverIP)
	if err != nil {
		s.db.Exec(`UPDATE domains SET dns_config_error = $1, updated_at = NOW() WHERE id = $2`, err.Error(), domainID)
		return err
	}

	s.db.Exec(`UPDATE domains SET dns_config_error = NULL, dns_verified = true, updated_at = NOW() WHERE id = $1`, domainID)
	return nil
}

// CreateDomainFromPurchase creates a domain record from an aged domain purchase
func (s *PorkbunService) CreateDomainFromPurchase(ctx context.Context, userID, domainName, verifyToken, purchaseID string) (string, error) {
	if s.client == nil {
		return "", fmt.Errorf("porkbun client not configured")
	}

	// Get deploy server IP
	var serverIP string
	err := s.db.GetContext(ctx, &serverIP, `
		SELECT ip FROM servers WHERE is_deploy_vps = true LIMIT 1
	`)
	if err != nil {
		return "", fmt.Errorf("no deploy server configured: %w", err)
	}

	// Generate domain ID
	domainID := fmt.Sprintf("dom_%s", purchaseID[:8])

	// Create domain record
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO domains (id, user_id, name, verify_token, source, dns_auto_configured, is_wildcard, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'porkbun', true, true, NOW(), NOW())
		ON CONFLICT (name) DO UPDATE SET user_id = $2, verify_token = $4, dns_auto_configured = true, updated_at = NOW()
	`, domainID, userID, domainName, verifyToken)
	if err != nil {
		return "", fmt.Errorf("creating domain record: %w", err)
	}

	// Setup DNS
	_, _, err = s.client.SetupDNS(ctx, domainName, serverIP)
	if err != nil {
		s.db.Exec(`UPDATE domains SET dns_config_error = $1 WHERE id = $2`, err.Error(), domainID)
		return domainID, fmt.Errorf("setting up DNS: %w", err)
	}

	// Add verification TXT record
	if verifyToken != "" {
		s.client.CreateDNSRecord(ctx, domainName, porkbun.DNSRecordInput{
			Type:    "TXT",
			Name:    "_guardbot-verify",
			Content: verifyToken,
			TTL:     600,
		})
	}

	return domainID, nil
}
