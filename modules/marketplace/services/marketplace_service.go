package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/botginx/botginx/modules/marketplace/models"
	"github.com/jmoiron/sqlx"
)

type MarketplaceService struct {
	db *sqlx.DB
}

func NewMarketplaceService(db *sqlx.DB) *MarketplaceService {
	return &MarketplaceService{db: db}
}

func (s *MarketplaceService) generateID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ListForSale lists all marketplace domains (for admin)
func (s *MarketplaceService) ListForSale() ([]models.MarketplaceDomain, error) {
	var domains []models.MarketplaceDomain
	err := s.db.Select(&domains, `
		SELECT id, name, user_id, status, is_marketplace,
		       marketplace_price, marketplace_description, marketplace_listed_at, created_at
		FROM domains
		WHERE is_marketplace = TRUE
		ORDER BY marketplace_listed_at DESC
	`)
	return domains, err
}

// ListAvailable lists domains available for purchase (for users)
func (s *MarketplaceService) ListAvailable() ([]models.MarketplaceDomain, error) {
	var domains []models.MarketplaceDomain
	err := s.db.Select(&domains, `
		SELECT id, name, user_id, status, is_marketplace,
		       marketplace_price, marketplace_description, marketplace_listed_at, created_at
		FROM domains
		WHERE is_marketplace = TRUE AND status = 'active'
		ORDER BY marketplace_listed_at DESC
	`)
	return domains, err
}

// GetDomain gets a single domain with marketplace info
func (s *MarketplaceService) GetDomain(domainID string) (*models.MarketplaceDomain, error) {
	var domain models.MarketplaceDomain
	err := s.db.Get(&domain, `
		SELECT id, name, user_id, status, is_marketplace,
		       marketplace_price, marketplace_description, marketplace_listed_at, created_at
		FROM domains WHERE id = $1
	`, domainID)
	if err != nil {
		return nil, err
	}
	return &domain, nil
}

// MarkForSale marks a domain as for sale in marketplace
func (s *MarketplaceService) MarkForSale(domainID string, price float64, description string) error {
	if price <= 0 {
		return errors.New("price must be greater than 0")
	}

	result, err := s.db.Exec(`
		UPDATE domains SET
			is_marketplace = TRUE,
			marketplace_price = $2,
			marketplace_description = $3,
			marketplace_listed_at = $4
		WHERE id = $1 AND status = 'active'
	`, domainID, price, description, time.Now())
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return errors.New("domain not found or not active")
	}
	return nil
}

// UpdateListing updates marketplace listing details
func (s *MarketplaceService) UpdateListing(domainID string, input models.UpdateListingInput) error {
	updates := []string{}
	args := []interface{}{domainID}
	argNum := 2

	if input.Price != nil {
		if *input.Price <= 0 {
			return errors.New("price must be greater than 0")
		}
		updates = append(updates, fmt.Sprintf("marketplace_price = $%d", argNum))
		args = append(args, *input.Price)
		argNum++
	}
	if input.Description != nil {
		updates = append(updates, fmt.Sprintf("marketplace_description = $%d", argNum))
		args = append(args, *input.Description)
		argNum++
	}

	if len(updates) == 0 {
		return nil
	}

	query := fmt.Sprintf(`UPDATE domains SET %s WHERE id = $1 AND is_marketplace = TRUE`,
		joinStrings(updates, ", "))
	_, err := s.db.Exec(query, args...)
	return err
}

// RemoveFromSale removes domain from marketplace (keeps domain, just not for sale)
func (s *MarketplaceService) RemoveFromSale(domainID string) error {
	_, err := s.db.Exec(`
		UPDATE domains SET
			is_marketplace = FALSE,
			marketplace_price = NULL,
			marketplace_description = NULL,
			marketplace_listed_at = NULL
		WHERE id = $1
	`, domainID)
	return err
}

// Purchase handles the domain purchase transaction
func (s *MarketplaceService) Purchase(domainID, buyerUserID string) error {
	// Start transaction
	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get domain with lock
	var domain struct {
		ID               string   `db:"id"`
		UserID           string   `db:"user_id"`
		IsMarketplace    bool     `db:"is_marketplace"`
		MarketplacePrice *float64 `db:"marketplace_price"`
		Status           string   `db:"status"`
	}
	err = tx.Get(&domain, `
		SELECT id, user_id, is_marketplace, marketplace_price, status
		FROM domains WHERE id = $1 FOR UPDATE
	`, domainID)
	if err != nil {
		return errors.New("domain not found")
	}

	if !domain.IsMarketplace {
		return errors.New("domain is not for sale")
	}
	if domain.Status != "active" {
		return errors.New("domain is not active")
	}
	if domain.MarketplacePrice == nil {
		return errors.New("domain price not set")
	}

	price := *domain.MarketplacePrice
	sellerUserID := domain.UserID

	// Check buyer balance
	var balance float64
	err = tx.Get(&balance, `SELECT COALESCE(balance, 0) FROM users WHERE id = $1 FOR UPDATE`, buyerUserID)
	if err != nil {
		return errors.New("buyer not found")
	}
	if balance < price {
		return errors.New("insufficient balance")
	}

	// Deduct from buyer
	_, err = tx.Exec(`UPDATE users SET balance = balance - $1 WHERE id = $2`, price, buyerUserID)
	if err != nil {
		return err
	}

	// Transfer domain ownership
	_, err = tx.Exec(`
		UPDATE domains SET
			user_id = $1,
			is_marketplace = FALSE,
			marketplace_price = NULL,
			marketplace_description = NULL,
			marketplace_listed_at = NULL
		WHERE id = $2
	`, buyerUserID, domainID)
	if err != nil {
		return err
	}

	// Record sale
	_, err = tx.Exec(`
		INSERT INTO marketplace_sales (id, domain_id, seller_user_id, buyer_user_id, price, purchased_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, s.generateID(), domainID, sellerUserID, buyerUserID, price, time.Now())
	if err != nil {
		return err
	}

	return tx.Commit()
}

// GetSalesHistory returns purchase history
func (s *MarketplaceService) GetSalesHistory() ([]models.Sale, error) {
	var sales []models.Sale
	err := s.db.Select(&sales, `
		SELECT s.*, d.name as domain_name
		FROM marketplace_sales s
		LEFT JOIN domains d ON d.id = s.domain_id
		ORDER BY s.purchased_at DESC
		LIMIT 100
	`)
	return sales, err
}

// GetUserBalance returns user's current balance
func (s *MarketplaceService) GetUserBalance(userID string) float64 {
	var balance float64
	s.db.Get(&balance, `SELECT COALESCE(balance, 0) FROM users WHERE id = $1`, userID)
	return balance
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
