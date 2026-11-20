package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/botginx/botginx/modules/ageddomains/models"
	healthservices "github.com/botginx/botginx/modules/domainhealth/services"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

var (
	ErrDomainNotFound    = errors.New("domain not found")
	ErrDomainUnavailable = errors.New("domain no longer available")
)

type InventoryService struct {
	db               *sqlx.DB
	providerRegistry *ProviderRegistry
	safetyChecker    *healthservices.SafetyChecker
}

func NewInventoryService(db *sqlx.DB, registry *ProviderRegistry) *InventoryService {
	// Get Google Safe Browsing API key from environment
	apiKey := os.Getenv("google_safe_browsing_api_key")

	return &InventoryService{
		db:               db,
		providerRegistry: registry,
		safetyChecker:    healthservices.NewSafetyChecker(apiKey),
	}
}

// ListAvailable returns available domains with optional filters
func (s *InventoryService) ListAvailable(ctx context.Context, tld string, maxPrice int, category string, page, perPage int) ([]models.DomainInventory, int, error) {
	if perPage <= 0 {
		perPage = 25
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * perPage

	// Build query with filters
	query := `SELECT * FROM domain_inventory WHERE is_available = true`
	countQuery := `SELECT COUNT(*) FROM domain_inventory WHERE is_available = true`
	args := []interface{}{}
	argNum := 1

	if tld != "" {
		query += ` AND tld = $` + string(rune('0'+argNum))
		countQuery += ` AND tld = $` + string(rune('0'+argNum))
		args = append(args, tld)
		argNum++
	}
	if maxPrice > 0 {
		query += ` AND sale_price <= $` + string(rune('0'+argNum))
		countQuery += ` AND sale_price <= $` + string(rune('0'+argNum))
		args = append(args, maxPrice)
		argNum++
	}
	if category != "" {
		query += ` AND category = $` + string(rune('0'+argNum))
		countQuery += ` AND category = $` + string(rune('0'+argNum))
		args = append(args, category)
		argNum++
	}

	// Get total count
	var total int
	err := s.db.GetContext(ctx, &total, countQuery, args...)
	if err != nil {
		return nil, 0, err
	}

	// Add ordering and pagination
	query += ` ORDER BY age_years DESC NULLS LAST, sale_price ASC LIMIT $` + string(rune('0'+argNum)) + ` OFFSET $` + string(rune('0'+argNum+1))
	args = append(args, perPage, offset)

	var domains []models.DomainInventory
	err = s.db.SelectContext(ctx, &domains, query, args...)
	return domains, total, err
}

// Get returns a domain by ID
func (s *InventoryService) Get(ctx context.Context, id string) (*models.DomainInventory, error) {
	var domain models.DomainInventory
	err := s.db.GetContext(ctx, &domain,
		`SELECT * FROM domain_inventory WHERE id = $1`, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDomainNotFound
		}
		return nil, err
	}
	return &domain, nil
}

// ReserveForPurchase marks a domain as unavailable and returns it
func (s *InventoryService) ReserveForPurchase(ctx context.Context, id string) (*models.DomainInventory, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var domain models.DomainInventory
	err = tx.GetContext(ctx, &domain,
		`SELECT * FROM domain_inventory WHERE id = $1 FOR UPDATE`, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDomainNotFound
		}
		return nil, err
	}

	if !domain.IsAvailable {
		return nil, ErrDomainUnavailable
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE domain_inventory SET is_available = false WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	domain.IsAvailable = false
	return &domain, nil
}

// MarkAvailable marks a domain as available again (after failed purchase)
func (s *InventoryService) MarkAvailable(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE domain_inventory SET is_available = true WHERE id = $1`, id)
	return err
}

// UpdateSafetyStatus updates the safety check results for a domain
func (s *InventoryService) UpdateSafetyStatus(ctx context.Context, id string, status string, reasons []string) error {
	reasonsJSON, _ := json.Marshal(reasons)
	_, err := s.db.ExecContext(ctx, `
		UPDATE domain_inventory
		SET safety_status = $1, safety_reasons = $2, safety_checked_at = $3
		WHERE id = $4
	`, status, string(reasonsJSON), time.Now(), id)
	return err
}

// CheckSafety performs an on-demand safety check for a domain
// This calls external APIs (Google Safe Browsing, URLhaus, PhishTank)
func (s *InventoryService) CheckSafety(ctx context.Context, id string) (*models.SafetyCheckResult, error) {
	domain, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Run safety check
	result := s.safetyChecker.CheckDomain(domain.DomainName)

	// Convert threats to reasons
	reasons := make([]string, 0, len(result.Threats))
	for _, t := range result.Threats {
		reasons = append(reasons, t.Source+": "+t.ThreatType)
	}

	// Update database
	status := string(result.Status)
	if err := s.UpdateSafetyStatus(ctx, id, status, reasons); err != nil {
		return nil, err
	}

	return &models.SafetyCheckResult{
		Status:    status,
		Threats:   reasons,
		CheckedAt: result.CheckedAt.Format(time.RFC3339),
	}, nil
}

// UpdateInventory syncs domains from a provider
func (s *InventoryService) UpdateInventory(ctx context.Context, providerID string, domains []ProviderDomain, markupPercent float64) (int, error) {
	updated := 0

	for _, d := range domains {
		// Calculate sale price with markup
		salePrice := d.PriceCents
		if markupPercent > 0 {
			salePrice = int(float64(d.PriceCents) * (1 + markupPercent/100))
		}

		// Upsert domain
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO domain_inventory
			(id, provider_id, domain_name, tld, registration_price, sale_price, age_years, is_premium, category, is_available, fetched_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, true, $10)
			ON CONFLICT (provider_id, domain_name) DO UPDATE SET
				registration_price = $5,
				sale_price = $6,
				age_years = $7,
				is_premium = $8,
				fetched_at = $10
		`, uuid.New().String(), providerID, d.Domain, d.TLD, d.PriceCents, salePrice, d.AgeYears, d.IsPremium, "aged", time.Now())
		if err == nil {
			updated++
		}
	}

	return updated, nil
}

// PruneStale removes domains not updated in the last fetch
func (s *InventoryService) PruneStale(ctx context.Context, providerID string, before time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM domain_inventory
		WHERE provider_id = $1 AND fetched_at < $2 AND is_available = true
	`, providerID, before)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// GetStats returns inventory statistics
func (s *InventoryService) GetStats(ctx context.Context) (map[string]int, error) {
	stats := make(map[string]int)

	var total int
	s.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM domain_inventory`)
	stats["total"] = total

	var available int
	s.db.GetContext(ctx, &available, `SELECT COUNT(*) FROM domain_inventory WHERE is_available = true`)
	stats["available"] = available

	var clean int
	s.db.GetContext(ctx, &clean, `SELECT COUNT(*) FROM domain_inventory WHERE safety_status = 'clean'`)
	stats["clean"] = clean

	return stats, nil
}
