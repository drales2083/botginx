package models

import "time"

// NameSiloConnection represents user's NameSilo API connection
// Unlike cPanel, each user has only one NameSilo connection (stored in users table)
type NameSiloConnection struct {
	UserID      string     `json:"userId"`
	APIKey      string     `json:"-"` // decrypted at runtime, never in JSON
	ConnectedAt *time.Time `json:"connectedAt,omitempty"`
	IsConnected bool       `json:"isConnected"`
}

// NameSiloDomain represents a domain from the NameSilo API
type NameSiloDomain struct {
	Domain     string     `json:"domain"`
	Created    *time.Time `json:"created,omitempty"`
	Expires    *time.Time `json:"expires,omitempty"`
	Status     string     `json:"status"` // Active, Expired, etc.
	Locked     bool       `json:"locked"`
	Private    bool       `json:"private"`  // WHOIS privacy
	AutoRenew  bool       `json:"autoRenew"`
	IsImported bool       `json:"isImported"` // already in our system
	LocalID    string     `json:"localId,omitempty"` // our domain ID if imported
}

// NameSiloDNSRecord represents a DNS record from NameSilo
type NameSiloDNSRecord struct {
	RecordID string `json:"recordId"` // rrid for updates/deletes
	Type     string `json:"type"`     // A, AAAA, CNAME, MX, TXT
	Host     string `json:"host"`     // subdomain or @ for root
	Value    string `json:"value"`    // IP address or target
	TTL      int    `json:"ttl"`      // seconds
	Distance int    `json:"distance"` // MX priority
}

// NameSiloImportInput is the input for importing a domain
type NameSiloImportInput struct {
	Domain       string `json:"domain" validate:"required"`
	ConfigureDNS bool   `json:"configureDns"` // auto-configure DNS records
}

// NameSiloDNSSetupInput is the input for setting up DNS
type NameSiloDNSSetupInput struct {
	Domain   string `json:"domain" validate:"required"`
	ServerIP string `json:"serverIp,omitempty"` // optional, auto-detect if empty
}
