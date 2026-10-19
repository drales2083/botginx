package services

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/botginx/botginx/modules/promocodes/models"
	"github.com/jmoiron/sqlx"
)

// PromoService handles promo code operations
type PromoService struct {
	db *sqlx.DB
}

// NewPromoService creates a new promo service
func NewPromoService(db *sqlx.DB) *PromoService {
	return &PromoService{db: db}
}

func generateID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Create creates a new promo code
func (s *PromoService) Create(input models.CreatePromoInput, createdBy string) (*models.PromoCode, error) {
	code := strings.ToUpper(strings.TrimSpace(input.Code))
	if code == "" {
		return nil, fmt.Errorf("code is required")
	}
	if input.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if input.DiscountType != models.DiscountTypePercentage && input.DiscountType != models.DiscountTypeFixed {
		return nil, fmt.Errorf("invalid discount type")
	}
	if input.DiscountValue <= 0 {
		return nil, fmt.Errorf("discount value must be positive")
	}
	if input.DiscountType == models.DiscountTypePercentage && input.DiscountValue > 100 {
		return nil, fmt.Errorf("percentage discount cannot exceed 100")
	}

	promo := &models.PromoCode{
		ID:             generateID(),
		Code:           code,
		Name:           input.Name,
		Description:    input.Description,
		DiscountType:   input.DiscountType,
		DiscountValue:  input.DiscountValue,
		Eligibility:    input.Eligibility,
		MaxUses:        input.MaxUses,
		MaxUsesPerUser: input.MaxUsesPerUser,
		CurrentUses:    0,
		ValidFrom:      input.ValidFrom,
		ValidUntil:     input.ValidUntil,
		IsActive:       input.IsActive,
		CreatedBy:      &createdBy,
	}

	if promo.Eligibility == "" {
		promo.Eligibility = models.EligibilityAny
	}

	_, err := s.db.Exec(`
		INSERT INTO promo_codes (
			id, code, name, description, discount_type, discount_value,
			eligibility, max_uses, max_uses_per_user, current_uses,
			valid_from, valid_until, is_active, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`, promo.ID, promo.Code, promo.Name, promo.Description,
		promo.DiscountType, promo.DiscountValue, promo.Eligibility,
		promo.MaxUses, promo.MaxUsesPerUser, promo.CurrentUses,
		promo.ValidFrom, promo.ValidUntil, promo.IsActive, promo.CreatedBy)

	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, fmt.Errorf("code '%s' already exists", code)
		}
		return nil, err
	}

	return promo, nil
}

// Get returns a promo code by ID
func (s *PromoService) Get(id string) (*models.PromoCode, error) {
	var promo models.PromoCode
	err := s.db.Get(&promo, `SELECT * FROM promo_codes WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	return &promo, nil
}

// GetByCode returns a promo code by its code string
func (s *PromoService) GetByCode(code string) (*models.PromoCode, error) {
	var promo models.PromoCode
	err := s.db.Get(&promo, `SELECT * FROM promo_codes WHERE code = $1`, strings.ToUpper(code))
	if err != nil {
		return nil, err
	}
	return &promo, nil
}

// List returns all promo codes
func (s *PromoService) List() ([]models.PromoCode, error) {
	var promos []models.PromoCode
	err := s.db.Select(&promos, `
		SELECT * FROM promo_codes
		ORDER BY created_at DESC
	`)
	return promos, err
}

// Update updates a promo code
func (s *PromoService) Update(id string, input models.UpdatePromoInput) (*models.PromoCode, error) {
	promo, err := s.Get(id)
	if err != nil {
		return nil, err
	}

	if input.Name != nil {
		promo.Name = *input.Name
	}
	if input.Description != nil {
		promo.Description = *input.Description
	}
	if input.DiscountType != nil {
		promo.DiscountType = *input.DiscountType
	}
	if input.DiscountValue != nil {
		promo.DiscountValue = *input.DiscountValue
	}
	if input.Eligibility != nil {
		promo.Eligibility = *input.Eligibility
	}
	if input.MaxUses != nil {
		promo.MaxUses = input.MaxUses
	}
	if input.MaxUsesPerUser != nil {
		promo.MaxUsesPerUser = input.MaxUsesPerUser
	}
	if input.ValidFrom != nil {
		promo.ValidFrom = input.ValidFrom
	}
	if input.ValidUntil != nil {
		promo.ValidUntil = input.ValidUntil
	}
	if input.IsActive != nil {
		promo.IsActive = *input.IsActive
	}

	_, err = s.db.Exec(`
		UPDATE promo_codes SET
			name = $1, description = $2, discount_type = $3, discount_value = $4,
			eligibility = $5, max_uses = $6, max_uses_per_user = $7,
			valid_from = $8, valid_until = $9, is_active = $10, updated_at = NOW()
		WHERE id = $11
	`, promo.Name, promo.Description, promo.DiscountType, promo.DiscountValue,
		promo.Eligibility, promo.MaxUses, promo.MaxUsesPerUser,
		promo.ValidFrom, promo.ValidUntil, promo.IsActive, id)

	return promo, err
}

// Delete permanently removes a promo code
func (s *PromoService) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM promo_codes WHERE id = $1`, id)
	return err
}

// SetActive enables or disables a promo code
func (s *PromoService) SetActive(id string, active bool) error {
	_, err := s.db.Exec(`
		UPDATE promo_codes SET is_active = $1, updated_at = NOW() WHERE id = $2
	`, active, id)
	return err
}

// ValidateCode checks if a code is valid for a user at a given subscription price
func (s *PromoService) ValidateCode(code string, userID string, subscriptionPrice float64) *models.ValidateResult {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return &models.ValidateResult{Valid: false, Error: "Please enter a promo code", FinalPrice: subscriptionPrice}
	}

	promo, err := s.GetByCode(code)
	if err != nil {
		return &models.ValidateResult{Valid: false, Error: "Invalid promo code", FinalPrice: subscriptionPrice}
	}

	// Check active
	if !promo.IsActive {
		return &models.ValidateResult{Valid: false, Error: "This code is no longer active", FinalPrice: subscriptionPrice}
	}

	// Check date window
	now := time.Now()
	if promo.ValidFrom != nil && now.Before(*promo.ValidFrom) {
		return &models.ValidateResult{Valid: false, Error: "This code is not yet active", FinalPrice: subscriptionPrice}
	}
	if promo.ValidUntil != nil && now.After(*promo.ValidUntil) {
		return &models.ValidateResult{Valid: false, Error: "This code has expired", FinalPrice: subscriptionPrice}
	}

	// Check total usage limit
	if promo.MaxUses != nil && promo.CurrentUses >= *promo.MaxUses {
		return &models.ValidateResult{Valid: false, Error: "This code has reached its usage limit", FinalPrice: subscriptionPrice}
	}

	// Check per-user limit
	if promo.MaxUsesPerUser != nil {
		userUses := s.GetUserRedemptionCount(promo.ID, userID)
		if userUses >= *promo.MaxUsesPerUser {
			return &models.ValidateResult{Valid: false, Error: "You have already used this code", FinalPrice: subscriptionPrice}
		}
	}

	// Check eligibility
	eligible, eligErr := s.checkEligibility(promo.Eligibility, userID)
	if !eligible {
		return &models.ValidateResult{Valid: false, Error: eligErr, FinalPrice: subscriptionPrice}
	}

	// Calculate discount
	var discountAmount float64
	if promo.DiscountType == models.DiscountTypePercentage {
		discountAmount = subscriptionPrice * (promo.DiscountValue / 100)
	} else {
		discountAmount = promo.DiscountValue
	}

	// Cap discount at subscription price (can't go negative)
	if discountAmount > subscriptionPrice {
		discountAmount = subscriptionPrice
	}

	finalPrice := subscriptionPrice - discountAmount

	return &models.ValidateResult{
		Valid:          true,
		PromoID:        promo.ID,
		PromoCode:      promo.Code,
		DiscountType:   promo.DiscountType,
		DiscountValue:  promo.DiscountValue,
		DiscountAmount: discountAmount,
		FinalPrice:     finalPrice,
	}
}

// checkEligibility verifies if user meets the eligibility criteria
func (s *PromoService) checkEligibility(eligibility, userID string) (bool, string) {
	switch eligibility {
	case models.EligibilityAny:
		return true, ""

	case models.EligibilityNewUsers:
		// Never had a subscription
		var count int
		s.db.Get(&count, `SELECT COUNT(*) FROM subscriptions WHERE user_id = $1`, userID)
		if count > 0 {
			return false, "This code is for new users only"
		}
		return true, ""

	case models.EligibilityExpiredUsers:
		// Has at least one expired/cancelled subscription, no active ones
		var expiredCount int
		s.db.Get(&expiredCount, `
			SELECT COUNT(*) FROM subscriptions
			WHERE user_id = $1 AND status IN ('expired', 'cancelled')
		`, userID)
		if expiredCount == 0 {
			return false, "This code is for returning users only"
		}
		var activeCount int
		s.db.Get(&activeCount, `
			SELECT COUNT(*) FROM subscriptions
			WHERE user_id = $1 AND status = 'active'
		`, userID)
		if activeCount > 0 {
			return false, "This code cannot be used with an active subscription"
		}
		return true, ""

	case models.EligibilityNoActiveSub:
		// No current active subscription
		var count int
		s.db.Get(&count, `
			SELECT COUNT(*) FROM subscriptions
			WHERE user_id = $1 AND status = 'active'
		`, userID)
		if count > 0 {
			return false, "This code cannot be used with an active subscription"
		}
		return true, ""
	}

	return true, ""
}

// RedeemCode atomically increments usage and records redemption
func (s *PromoService) RedeemCode(promoID string, userID string, subscriptionID string, originalAmount, discountAmount, finalAmount float64) (*models.PromoRedemption, error) {
	// Atomic increment with condition to prevent over-redemption
	result, err := s.db.Exec(`
		UPDATE promo_codes
		SET current_uses = current_uses + 1, updated_at = NOW()
		WHERE id = $1 AND is_active = TRUE
		AND (max_uses IS NULL OR current_uses < max_uses)
	`, promoID)
	if err != nil {
		return nil, fmt.Errorf("failed to update promo usage: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, fmt.Errorf("code is no longer valid or has reached its limit")
	}

	// Record redemption
	redemption := &models.PromoRedemption{
		ID:             generateID(),
		PromoID:        promoID,
		UserID:         userID,
		SubscriptionID: &subscriptionID,
		OriginalAmount: originalAmount,
		DiscountAmount: discountAmount,
		FinalAmount:    finalAmount,
	}

	_, err = s.db.Exec(`
		INSERT INTO promo_redemptions (
			id, promo_id, user_id, subscription_id,
			original_amount, discount_amount, final_amount
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, redemption.ID, redemption.PromoID, redemption.UserID, redemption.SubscriptionID,
		redemption.OriginalAmount, redemption.DiscountAmount, redemption.FinalAmount)

	if err != nil {
		return nil, fmt.Errorf("failed to record redemption: %w", err)
	}

	return redemption, nil
}

// GetUserRedemptionCount returns how many times a user has used a specific promo
func (s *PromoService) GetUserRedemptionCount(promoID, userID string) int {
	var count int
	s.db.Get(&count, `
		SELECT COUNT(*) FROM promo_redemptions
		WHERE promo_id = $1 AND user_id = $2
	`, promoID, userID)
	return count
}

// GetRedemptions returns all redemptions for a promo code
func (s *PromoService) GetRedemptions(promoID string) ([]models.PromoRedemption, error) {
	var redemptions []models.PromoRedemption
	err := s.db.Select(&redemptions, `
		SELECT r.*, u.email as user_email, p.code as promo_code
		FROM promo_redemptions r
		JOIN users u ON r.user_id = u.id
		JOIN promo_codes p ON r.promo_id = p.id
		WHERE r.promo_id = $1
		ORDER BY r.redeemed_at DESC
	`, promoID)
	return redemptions, err
}

// GetStats returns usage statistics for a promo code
func (s *PromoService) GetStats(promoID string) (*models.PromoStats, error) {
	promo, err := s.Get(promoID)
	if err != nil {
		return nil, err
	}

	stats := &models.PromoStats{}

	// Total uses
	s.db.Get(&stats.TotalUses, `SELECT COUNT(*) FROM promo_redemptions WHERE promo_id = $1`, promoID)

	// Unique users
	s.db.Get(&stats.UniqueUsers, `SELECT COUNT(DISTINCT user_id) FROM promo_redemptions WHERE promo_id = $1`, promoID)

	// Total saved
	var totalSaved sql.NullFloat64
	s.db.Get(&totalSaved, `SELECT SUM(discount_amount) FROM promo_redemptions WHERE promo_id = $1`, promoID)
	if totalSaved.Valid {
		stats.TotalSaved = totalSaved.Float64
	}

	// Average discount
	if stats.TotalUses > 0 {
		stats.AvgDiscount = stats.TotalSaved / float64(stats.TotalUses)
	}

	// Uses remaining
	if promo.MaxUses != nil {
		remaining := *promo.MaxUses - promo.CurrentUses
		if remaining < 0 {
			remaining = 0
		}
		stats.UsesRemaining = &remaining
	}

	return stats, nil
}
