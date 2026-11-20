package models

import (
	"database/sql"
	"time"
)

// Provider represents a domain registrar configuration
type Provider struct {
	ID            string    `db:"id" json:"id"`
	Name          string    `db:"name" json:"name"`
	DisplayName   string    `db:"display_name" json:"displayName"`
	ProviderType  string    `db:"provider_type" json:"-"` // Hidden from users
	APIKey        string    `db:"api_key" json:"-"`       // Never expose
	APISecret     string    `db:"api_secret" json:"-"`    // Never expose
	IsActive      bool      `db:"is_active" json:"isActive"`
	MarkupPercent float64   `db:"markup_percent" json:"markupPercent"`
	MinAgeYears   int       `db:"min_age_years" json:"minAgeYears"`
	MaxPriceCents int       `db:"max_price_cents" json:"maxPriceCents"`
	AllowedTLDs   string    `db:"allowed_tlds" json:"allowedTlds"`
	CreatedAt     time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt     time.Time `db:"updated_at" json:"updatedAt"`
}

// DomainInventory represents a cached domain from a provider
type DomainInventory struct {
	ID                string         `db:"id" json:"id"`
	ProviderID        string         `db:"provider_id" json:"-"`
	DomainName        string         `db:"domain_name" json:"name"`
	TLD               string         `db:"tld" json:"tld"`
	RegistrationPrice int            `db:"registration_price" json:"-"` // Provider price, hidden
	SalePrice         int            `db:"sale_price" json:"priceCents"`
	AgeYears          sql.NullInt64  `db:"age_years" json:"ageYears"`
	IsPremium         bool           `db:"is_premium" json:"isPremium"`
	Category          string         `db:"category" json:"category"`
	IsAvailable       bool           `db:"is_available" json:"isAvailable"`
	SafetyStatus      string         `db:"safety_status" json:"safetyStatus"`
	SafetyCheckedAt   sql.NullTime   `db:"safety_checked_at" json:"safetyCheckedAt,omitempty"`
	SafetyReasons     string         `db:"safety_reasons" json:"safetyReasons,omitempty"`
	FetchedAt         time.Time      `db:"fetched_at" json:"-"`
	ExpiresAt         sql.NullTime   `db:"expires_at" json:"expiresAt,omitempty"`
}

// BalanceLock represents a temporary fund reservation
type BalanceLock struct {
	ID             string       `db:"id" json:"id"`
	UserID         string       `db:"user_id" json:"userId"`
	Amount         int          `db:"amount" json:"amountCents"`
	IdempotencyKey string       `db:"idempotency_key" json:"-"`
	Purpose        string       `db:"purpose" json:"purpose"`
	Status         string       `db:"status" json:"status"` // pending, committed, released
	LockedAt       time.Time    `db:"locked_at" json:"lockedAt"`
	ExpiresAt      time.Time    `db:"expires_at" json:"expiresAt"`
	ResolvedAt     sql.NullTime `db:"resolved_at" json:"resolvedAt,omitempty"`
}

// Purchase represents a domain purchase record
type Purchase struct {
	ID              string       `db:"id" json:"id"`
	UserID          string       `db:"user_id" json:"userId"`
	InventoryID     string       `db:"inventory_id" json:"-"`
	DomainName      string       `db:"domain_name" json:"domain"`
	ProviderID      string       `db:"provider_id" json:"-"`
	ProviderPrice   int          `db:"provider_price" json:"-"`
	SalePrice       int          `db:"sale_price" json:"priceCents"`
	BalanceLockID   string       `db:"balance_lock_id" json:"-"`
	IdempotencyKey  string       `db:"idempotency_key" json:"-"`
	Status          string       `db:"status" json:"status"`
	ProviderOrderID string       `db:"provider_order_id" json:"-"`
	ErrorMessage    string       `db:"error_message" json:"error,omitempty"`
	DomainID        string       `db:"domain_id" json:"domainId,omitempty"`
	CreatedAt       time.Time    `db:"created_at" json:"createdAt"`
	CompletedAt     sql.NullTime `db:"completed_at" json:"completedAt,omitempty"`
}

// PurchaseEvent represents a status event during purchase
type PurchaseEvent struct {
	ID         string    `db:"id" json:"id"`
	PurchaseID string    `db:"purchase_id" json:"purchaseId"`
	EventType  string    `db:"event_type" json:"type"`
	Message    string    `db:"message" json:"message"`
	Details    string    `db:"details" json:"details,omitempty"`
	CreatedAt  time.Time `db:"created_at" json:"at"`
}

// PurchaseStatus is the response for status polling
type PurchaseStatus struct {
	PurchaseID string          `json:"purchaseId"`
	Domain     string          `json:"domain"`
	Status     string          `json:"status"`
	PriceCents int             `json:"priceCents"`
	Events     []PurchaseEvent `json:"events"`
	Error      string          `json:"error,omitempty"`
}

// BalanceInfo shows available and locked balance
type BalanceInfo struct {
	Available int `json:"available"`
	Locked    int `json:"locked"`
	Total     int `json:"total"`
}

// SafetyCheckResult is the response from safety check
type SafetyCheckResult struct {
	Status    string   `json:"status"` // clean, flagged, unknown
	Threats   []string `json:"threats"`
	CheckedAt string   `json:"checkedAt"`
}

// Purchase status constants
const (
	StatusPending          = "pending"
	StatusPurchasing       = "purchasing"
	StatusProvisioning     = "provisioning"
	StatusCompleted        = "completed"
	StatusFailed           = "failed"
	StatusCommitFailed     = "commit_failed"
	StatusProvisionFailed  = "provisioning_failed"
)

// Lock status constants
const (
	LockStatusPending   = "pending"
	LockStatusCommitted = "committed"
	LockStatusReleased  = "released"
)

// Safety status constants
const (
	SafetyUnchecked = "unchecked"
	SafetyClean     = "clean"
	SafetyFlagged   = "flagged"
	SafetyUnknown   = "unknown"
)

// Favorite represents a user's saved domain
type Favorite struct {
	ID          string    `db:"id" json:"id"`
	UserID      string    `db:"user_id" json:"-"`
	InventoryID string    `db:"inventory_id" json:"inventoryId"`
	CreatedAt   time.Time `db:"created_at" json:"createdAt"`
}

// DomainWithFavorite extends DomainInventory with favorite status
type DomainWithFavorite struct {
	DomainInventory
	IsFavorite bool `json:"isFavorite"`
}

// ListFilters contains all filter and sort options for domain listing
type ListFilters struct {
	TLD          string // Filter by TLD (e.g., ".com")
	MinPrice     int    // Minimum price in cents
	MaxPrice     int    // Maximum price in cents
	MinAge       int    // Minimum age in years
	MaxAge       int    // Maximum age in years
	Category     string // Filter by category (aged, premium)
	SafetyStatus string // Filter by safety (clean, unchecked, flagged, all)
	Search       string // Search domain name
	FavoritesOnly bool  // Show only favorites
	SortBy       string // Sort field (price, age, name)
	SortDir      string // Sort direction (asc, desc)
	Page         int
	PerPage      int
}

// Sort constants
const (
	SortByPrice = "price"
	SortByAge   = "age"
	SortByName  = "name"
	SortAsc     = "asc"
	SortDesc    = "desc"
)
