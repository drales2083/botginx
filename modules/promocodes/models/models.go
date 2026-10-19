package models

import "time"

// PromoCode represents a promotional discount code for subscriptions
type PromoCode struct {
	ID          string `db:"id" json:"id"`
	Code        string `db:"code" json:"code"`
	Name        string `db:"name" json:"name"`
	Description string `db:"description" json:"description,omitempty"`

	// Discount
	DiscountType  string  `db:"discount_type" json:"discountType"`   // percentage, fixed
	DiscountValue float64 `db:"discount_value" json:"discountValue"` // 20 = 20% or $20

	// Eligibility: any, new_users, expired_users, no_active_sub
	Eligibility string `db:"eligibility" json:"eligibility"`

	// Usage limits
	MaxUses        *int `db:"max_uses" json:"maxUses,omitempty"`                 // NULL = unlimited
	MaxUsesPerUser *int `db:"max_uses_per_user" json:"maxUsesPerUser,omitempty"` // NULL = unlimited, 1 = one-time
	CurrentUses    int  `db:"current_uses" json:"currentUses"`

	// Time window
	ValidFrom  *time.Time `db:"valid_from" json:"validFrom,omitempty"`
	ValidUntil *time.Time `db:"valid_until" json:"validUntil,omitempty"` // NULL = never expires

	// Status
	IsActive  bool      `db:"is_active" json:"isActive"`
	CreatedBy *string   `db:"created_by" json:"createdBy,omitempty"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt time.Time `db:"updated_at" json:"updatedAt"`
}

// PromoRedemption records each use of a promo code
type PromoRedemption struct {
	ID             string    `db:"id" json:"id"`
	PromoID        string    `db:"promo_id" json:"promoId"`
	UserID         string    `db:"user_id" json:"userId"`
	SubscriptionID *string   `db:"subscription_id" json:"subscriptionId,omitempty"`
	OriginalAmount float64   `db:"original_amount" json:"originalAmount"`
	DiscountAmount float64   `db:"discount_amount" json:"discountAmount"`
	FinalAmount    float64   `db:"final_amount" json:"finalAmount"`
	RedeemedAt     time.Time `db:"redeemed_at" json:"redeemedAt"`

	// Joined fields
	UserEmail string `db:"user_email" json:"userEmail,omitempty"`
	PromoCode string `db:"promo_code" json:"promoCode,omitempty"`
}

// ValidateResult is returned when checking if a code is valid
type ValidateResult struct {
	Valid          bool    `json:"valid"`
	Error          string  `json:"error,omitempty"`
	PromoID        string  `json:"promoId,omitempty"`
	PromoCode      string  `json:"promoCode,omitempty"`
	DiscountType   string  `json:"discountType,omitempty"`
	DiscountValue  float64 `json:"discountValue,omitempty"`
	DiscountAmount float64 `json:"discountAmount"` // actual $ off
	FinalPrice     float64 `json:"finalPrice"`     // price after discount
}

// PromoStats holds usage statistics for a promo code
type PromoStats struct {
	TotalUses    int     `json:"totalUses"`
	UniqueUsers  int     `json:"uniqueUsers"`
	TotalSaved   float64 `json:"totalSaved"`
	AvgDiscount  float64 `json:"avgDiscount"`
	UsesRemaining *int   `json:"usesRemaining,omitempty"`
}

// CreatePromoInput for creating a new promo code
type CreatePromoInput struct {
	Code           string     `json:"code"`
	Name           string     `json:"name"`
	Description    string     `json:"description"`
	DiscountType   string     `json:"discountType"`
	DiscountValue  float64    `json:"discountValue"`
	Eligibility    string     `json:"eligibility"`
	MaxUses        *int       `json:"maxUses"`
	MaxUsesPerUser *int       `json:"maxUsesPerUser"`
	ValidFrom      *time.Time `json:"validFrom"`
	ValidUntil     *time.Time `json:"validUntil"`
	IsActive       bool       `json:"isActive"`
}

// UpdatePromoInput for updating an existing promo code
type UpdatePromoInput struct {
	Name           *string    `json:"name"`
	Description    *string    `json:"description"`
	DiscountType   *string    `json:"discountType"`
	DiscountValue  *float64   `json:"discountValue"`
	Eligibility    *string    `json:"eligibility"`
	MaxUses        *int       `json:"maxUses"`
	MaxUsesPerUser *int       `json:"maxUsesPerUser"`
	ValidFrom      *time.Time `json:"validFrom"`
	ValidUntil     *time.Time `json:"validUntil"`
	IsActive       *bool      `json:"isActive"`
}

// Eligibility constants
const (
	EligibilityAny          = "any"
	EligibilityNewUsers     = "new_users"
	EligibilityExpiredUsers = "expired_users"
	EligibilityNoActiveSub  = "no_active_sub"
)

// Discount type constants
const (
	DiscountTypePercentage = "percentage"
	DiscountTypeFixed      = "fixed"
)

// EligibilityOptions returns all eligibility options for UI
func EligibilityOptions() []struct {
	Value string
	Label string
} {
	return []struct {
		Value string
		Label string
	}{
		{EligibilityAny, "Any user"},
		{EligibilityNewUsers, "New users only (never subscribed)"},
		{EligibilityExpiredUsers, "Expired users only (lapsed subscription)"},
		{EligibilityNoActiveSub, "Users without active subscription"},
	}
}
