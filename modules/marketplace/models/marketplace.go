package models

import "time"

// MarketplaceDomain extends domain info for marketplace display
type MarketplaceDomain struct {
	ID                     string     `db:"id" json:"id"`
	Name                   string     `db:"name" json:"name"`
	UserID                 string     `db:"user_id" json:"userId"`
	DNSVerified            bool       `db:"dns_verified" json:"dnsVerified"`
	SSLEnabled             bool       `db:"ssl_enabled" json:"sslEnabled"`
	IsMarketplace          bool       `db:"is_marketplace" json:"isMarketplace"`
	MarketplacePrice       *float64   `db:"marketplace_price" json:"marketplacePrice,omitempty"`
	MarketplaceDescription *string    `db:"marketplace_description" json:"marketplaceDescription,omitempty"`
	MarketplaceListedAt    *time.Time `db:"marketplace_listed_at" json:"marketplaceListedAt,omitempty"`
	CreatedAt              time.Time  `db:"created_at" json:"createdAt"`
}

// Sale records a marketplace purchase
type Sale struct {
	ID           string    `db:"id" json:"id"`
	DomainID     string    `db:"domain_id" json:"domainId"`
	SellerUserID string    `db:"seller_user_id" json:"sellerUserId"`
	BuyerUserID  string    `db:"buyer_user_id" json:"buyerUserId"`
	Price        float64   `db:"price" json:"price"`
	PurchasedAt  time.Time `db:"purchased_at" json:"purchasedAt"`

	// Joined fields
	DomainName string `db:"domain_name" json:"domainName,omitempty"`
}

// ListForSaleInput for adding domain to marketplace
type ListForSaleInput struct {
	DomainID    string  `json:"domainId"`
	Price       float64 `json:"price"`
	Description string  `json:"description"`
}

// UpdateListingInput for updating marketplace listing
type UpdateListingInput struct {
	Price       *float64 `json:"price"`
	Description *string  `json:"description"`
}
