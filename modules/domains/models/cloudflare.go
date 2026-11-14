package models

import "time"

// CloudflareConnection represents user's Cloudflare API connection
type CloudflareConnection struct {
	UserID      string     `json:"userId"`
	Email       string     `json:"email"`
	APIKey      string     `json:"-"` // decrypted at runtime, never in JSON
	ConnectedAt *time.Time `json:"connectedAt,omitempty"`
	IsConnected bool       `json:"isConnected"`
}

// CloudflareZone represents a zone from the Cloudflare API
type CloudflareZone struct {
	ZoneID      string   `json:"zoneId"`
	Domain      string   `json:"domain"`
	Status      string   `json:"status"` // active, pending, etc.
	NameServers []string `json:"nameServers"`
	IsImported  bool     `json:"isImported"`        // already in our system
	LocalID     string   `json:"localId,omitempty"` // our domain ID if imported
}

// CloudflareDNSRecord represents a DNS record from Cloudflare
type CloudflareDNSRecord struct {
	RecordID string `json:"recordId"`
	Type     string `json:"type"`    // A, AAAA, CNAME, MX, TXT
	Name     string `json:"name"`    // full name (subdomain.domain.com)
	Content  string `json:"content"` // IP address or target
	TTL      int    `json:"ttl"`     // 1 = automatic
	Proxied  bool   `json:"proxied"` // orange cloud
}

// CloudflareImportInput is the input for importing a domain
type CloudflareImportInput struct {
	ZoneID       string `json:"zoneId" validate:"required"`
	Domain       string `json:"domain" validate:"required"`
	ConfigureDNS bool   `json:"configureDns"` // auto-configure DNS records
}

// CloudflareDNSSetupInput is the input for setting up DNS
type CloudflareDNSSetupInput struct {
	ZoneID   string `json:"zoneId" validate:"required"`
	Domain   string `json:"domain" validate:"required"`
	ServerIP string `json:"serverIp,omitempty"` // optional, auto-detect if empty
}
