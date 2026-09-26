package models

import "time"

// CpanelConnection represents a user's cPanel account credentials
type CpanelConnection struct {
	ID                string     `db:"id" json:"id"`
	UserID            string     `db:"user_id" json:"userId"`
	Name              string     `db:"name" json:"name"`
	Host              string     `db:"host" json:"host"` // hostname:port
	Username          string     `db:"username" json:"username"`
	APITokenEncrypted string     `db:"api_token_encrypted" json:"-"`
	IsActive          bool       `db:"is_active" json:"isActive"`
	LastUsedAt        *time.Time `db:"last_used_at" json:"lastUsedAt,omitempty"`
	LastError         *string    `db:"last_error" json:"lastError,omitempty"`
	CreatedAt         time.Time  `db:"created_at" json:"createdAt"`
	UpdatedAt         time.Time  `db:"updated_at" json:"updatedAt"`

	// Decrypted token (not stored, populated at runtime)
	APIToken string `db:"-" json:"-"`

	// Stats (populated by joins)
	DomainCount int `db:"domain_count" json:"domainCount,omitempty"`
}

// CreateCpanelConnectionInput is the input for creating a cPanel connection
type CreateCpanelConnectionInput struct {
	Name     string `json:"name" validate:"required"`
	Host     string `json:"host" validate:"required"`
	Username string `json:"username" validate:"required"`
	APIToken string `json:"apiToken" validate:"required"`
}

// UpdateCpanelConnectionInput is the input for updating a cPanel connection
type UpdateCpanelConnectionInput struct {
	Name     *string `json:"name"`
	Host     *string `json:"host"`
	Username *string `json:"username"`
	APIToken *string `json:"apiToken"` // empty = keep current
	IsActive *bool   `json:"isActive"`
}

// CpanelDomainInfo is returned when listing domains from a cPanel account
type CpanelDomainInfo struct {
	MainDomain    string   `json:"mainDomain"`
	AddonDomains  []string `json:"addonDomains"`
	SubDomains    []string `json:"subDomains"`
	ParkedDomains []string `json:"parkedDomains"`
}
