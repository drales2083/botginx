package models

import "time"

type Domain struct {
	ID          string    `db:"id" json:"id"`
	UserID      string    `db:"user_id" json:"userId"`
	Name        string    `db:"name" json:"name"` // e.g., example.com
	ServerID    *string   `db:"server_id" json:"serverId,omitempty"`
	DNSVerified bool      `db:"dns_verified" json:"dnsVerified"`
	SSLEnabled  bool      `db:"ssl_enabled" json:"sslEnabled"`
	IsShared    bool      `db:"is_shared" json:"isShared"`
	CreatedAt   time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt   time.Time `db:"updated_at" json:"updatedAt"`

	// Joined fields
	ServerName string `db:"server_name" json:"serverName,omitempty"`
}

type CreateDomainInput struct {
	Name     string `json:"name" validate:"required,fqdn"`
	ServerID string `json:"serverId"`
}

type UpdateDomainInput struct {
	ServerID    *string `json:"serverId"`
	DNSVerified *bool   `json:"dnsVerified"`
	SSLEnabled  *bool   `json:"sslEnabled"`
}
