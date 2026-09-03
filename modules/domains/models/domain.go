package models

import "time"

// SetupType indicates how the domain DNS is managed
const (
	SetupTypeDirect   = "direct"   // A record points to our server
	SetupTypeExternal = "external" // cPanel/Cloudflare/other external DNS
)

// SetupStep tracks the external domain setup wizard progress
const (
	SetupStepPending       = "pending"        // Just created, detecting type
	SetupStepDNSWaiting    = "dns_waiting"    // Waiting for user to add DNS records
	SetupStepSSLWaiting    = "ssl_waiting"    // Waiting for user to add ACME TXT record
	SetupStepSSLGenerating = "ssl_generating" // DNS verified, generating SSL
	SetupStepComplete      = "complete"       // Fully set up
)

type Domain struct {
	ID          string    `db:"id" json:"id"`
	UserID      string    `db:"user_id" json:"userId"`
	Name        string    `db:"name" json:"name"` // e.g., example.com or *.example.com
	ServerID    *string   `db:"server_id" json:"serverId,omitempty"`
	VerifyToken string    `db:"verify_token" json:"verifyToken"`
	DNSVerified bool      `db:"dns_verified" json:"dnsVerified"`
	SSLEnabled  bool      `db:"ssl_enabled" json:"sslEnabled"`
	IsShared    bool      `db:"is_shared" json:"isShared"`
	CreatedAt   time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt   time.Time `db:"updated_at" json:"updatedAt"`

	// External setup fields
	SetupType          string     `db:"setup_type" json:"setupType"`
	SetupStep          string     `db:"setup_step" json:"setupStep"`
	AcmeToken          *string    `db:"acme_token" json:"acmeToken,omitempty"`
	AcmeTokenExpiresAt *time.Time `db:"acme_token_expires_at" json:"acmeTokenExpiresAt,omitempty"`
	IsWildcard         bool       `db:"is_wildcard" json:"isWildcard"`
	SSLError           *string    `db:"ssl_error" json:"sslError,omitempty"`

	// acme-dns delegation fields (for 100% reliable wildcard SSL)
	AcmeSubdomain     *string `db:"acme_subdomain" json:"acmeSubdomain,omitempty"`
	AcmeUsername      *string `db:"acme_username" json:"-"`  // For API auth (X-Api-User)
	AcmePassword      *string `db:"acme_password" json:"-"`  // Never expose in JSON
	AcmeFulldomain    *string `db:"acme_fulldomain" json:"acmeFulldomain,omitempty"`
	AcmeCnameVerified bool    `db:"acme_cname_verified" json:"acmeCnameVerified"`

	// dns-persist-01 fields (one-time TXT record, no renewal changes)
	PersistTXTValue    *string `db:"persist_txt_value" json:"persistTxtValue,omitempty"`
	PersistTXTVerified bool    `db:"persist_txt_verified" json:"persistTxtVerified"`
	LegoAccountURI     *string `db:"lego_account_uri" json:"-"` // Never expose in JSON

	// Marketplace fields
	IsMarketplace          bool       `db:"is_marketplace" json:"isMarketplace"`
	MarketplacePrice       *float64   `db:"marketplace_price" json:"marketplacePrice,omitempty"`
	MarketplaceDescription *string    `db:"marketplace_description" json:"marketplaceDescription,omitempty"`
	MarketplaceListedAt    *time.Time `db:"marketplace_listed_at" json:"marketplaceListedAt,omitempty"`

	// Joined fields
	ServerName string `db:"server_name" json:"serverName,omitempty"`
}

// SetupStatus represents the current state of DNS records for external setup
type SetupStatus struct {
	ARecordFound      bool   `json:"aRecordFound"`
	ARecordIP         string `json:"aRecordIp,omitempty"`
	VerifyTXTFound    bool   `json:"verifyTxtFound"`
	AcmeTXTFound      bool   `json:"acmeTxtFound"`
	AcmeTXTStale      bool   `json:"acmeTxtStale,omitempty"`      // Old ACME record exists that needs deletion
	AcmeTXTStaleValue string `json:"acmeTxtStaleValue,omitempty"` // The stale value to delete
	AllRecordsFound   bool   `json:"allRecordsFound"`
	SetupStep         string `json:"setupStep"`
	SSLReady          bool   `json:"sslReady"`
	ErrorMessage      string `json:"errorMessage,omitempty"`
	AcmeToken         string `json:"acmeToken,omitempty"` // Current expected token
}

// ExternalSetupInfo contains all info needed for the setup wizard
type ExternalSetupInfo struct {
	Domain          string `json:"domain"`
	BaseDomain      string `json:"baseDomain"` // For wildcard, the root domain
	IsWildcard      bool   `json:"isWildcard"`
	ServerIP        string `json:"serverIp"`
	VerifyToken     string `json:"verifyToken"`
	AcmeToken       string `json:"acmeToken"`
	AcmeTokenReady  bool   `json:"acmeTokenReady"`
	SetupStep       string `json:"setupStep"`
	VerifyTXTName   string `json:"verifyTxtName"` // _guardbot-verify.domain.com
	AcmeTXTName     string `json:"acmeTxtName"`   // _acme-challenge.domain.com (legacy)

	// acme-dns CNAME delegation (new - 100% reliable)
	AcmeCnameTarget   string `json:"acmeCnameTarget,omitempty"`   // abc123.acme.guardbot.sbs
	AcmeCnameVerified bool   `json:"acmeCnameVerified"`
	UseAcmeDns        bool   `json:"useAcmeDns"` // true = use CNAME delegation

	// dns-persist-01 (one-time persistent TXT record)
	PersistTXTName     string `json:"persistTxtName,omitempty"`     // _validation-persist.domain.com
	PersistTXTValue    string `json:"persistTxtValue,omitempty"`    // letsencrypt.org; accounturi=...; policy=wildcard
	PersistTXTVerified bool   `json:"persistTxtVerified"`
	UseDnsPersist      bool   `json:"useDnsPersist"` // true = use dns-persist-01
}

type CreateDomainInput struct {
	Name     string `json:"name" validate:"required"`
	ServerID string `json:"serverId"`
}

type UpdateDomainInput struct {
	ServerID           *string    `json:"serverId"`
	DNSVerified        *bool      `json:"dnsVerified"`
	SSLEnabled         *bool      `json:"sslEnabled"`
	SetupType          *string    `json:"setupType"`
	SetupStep          *string    `json:"setupStep"`
	AcmeToken          *string    `json:"acmeToken"`
	AcmeTokenExpiresAt *time.Time `json:"acmeTokenExpiresAt"`
	SSLError           *string    `json:"sslError"`

	// acme-dns fields
	AcmeSubdomain     *string `json:"acmeSubdomain"`
	AcmePassword      *string `json:"acmePassword"`
	AcmeFulldomain    *string `json:"acmeFulldomain"`
	AcmeCnameVerified *bool   `json:"acmeCnameVerified"`

	// dns-persist-01 fields
	PersistTXTValue    *string `json:"persistTxtValue"`
	PersistTXTVerified *bool   `json:"persistTxtVerified"`
	LegoAccountURI     *string `json:"legoAccountUri"`
}
