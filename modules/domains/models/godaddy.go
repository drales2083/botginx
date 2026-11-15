package models

import "time"

// GoDaddyConnection represents user's GoDaddy API connection
type GoDaddyConnection struct {
	UserID      string     `json:"userId"`
	APIKey      string     `json:"-"` // decrypted at runtime, never in JSON
	APISecret   string     `json:"-"` // decrypted at runtime, never in JSON
	ConnectedAt *time.Time `json:"connectedAt,omitempty"`
	IsConnected bool       `json:"isConnected"`
}

// GoDaddyDomain represents a domain from the GoDaddy API
type GoDaddyDomain struct {
	Domain     string     `json:"domain"`
	Status     string     `json:"status"` // ACTIVE, CANCELLED, etc.
	Expires    *time.Time `json:"expires,omitempty"`
	IsImported bool       `json:"isImported"`        // already in our system
	LocalID    string     `json:"localId,omitempty"` // our domain ID if imported
}

// GoDaddyImportInput is the input for importing a domain
type GoDaddyImportInput struct {
	Domain       string `json:"domain" validate:"required"`
	ConfigureDNS bool   `json:"configureDns"` // auto-configure DNS records
}
