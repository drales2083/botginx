// Package models contains database models for the hosting module.
package models

import (
	"time"
)

// ServerType represents the hosting panel type
type ServerType string

const (
	ServerTypeCloudPanel ServerType = "cloudpanel"
	ServerTypeHestiaCP   ServerType = "hestiacp"
)

// AccountStatus represents the status of a hosting account
type AccountStatus string

const (
	AccountStatusPending   AccountStatus = "pending"   // Purchased, waiting for admin to link
	AccountStatusActive    AccountStatus = "active"    // Linked and active
	AccountStatusSuspended AccountStatus = "suspended" // Suspended (billing or manual)
	AccountStatusCancelled AccountStatus = "cancelled" // Cancelled
)

// HostingServer represents a hosting panel server node
type HostingServer struct {
	ID                string     `db:"id" json:"id"`
	Name              string     `db:"name" json:"name"`
	Type              ServerType `db:"type" json:"type"` // cloudpanel or hestiacp
	Hostname          string     `db:"hostname" json:"hostname"`
	PanelURL          string     `db:"panel_url" json:"panelUrl"`
	Port              int        `db:"port" json:"port"`
	Username          string     `db:"username" json:"username"`
	PasswordEncrypted string     `db:"password_encrypted" json:"-"`
	MaxAccounts       int        `db:"max_accounts" json:"maxAccounts"`
	CurrentAccounts   int        `db:"current_accounts" json:"currentAccounts"`
	IsActive          bool       `db:"is_active" json:"isActive"`
	CreatedAt         time.Time  `db:"created_at" json:"createdAt"`

	// Antibot dashboard (for user domain settings)
	AntibotDashboardURL       string `db:"antibot_dashboard_url" json:"antibotDashboardUrl"`
	AntibotPasswordEncrypted  string `db:"antibot_password_encrypted" json:"-"`
}

// HostingPackage represents a hosting plan with resource limits
type HostingPackage struct {
	ID              string    `db:"id" json:"id"`
	Name            string    `db:"name" json:"name"`
	Description     string    `db:"description" json:"description"`
	PriceMonthly    float64   `db:"price_monthly" json:"priceMonthly"`
	DiskMB          int       `db:"disk_mb" json:"diskMb"`
	BandwidthMB     int       `db:"bandwidth_mb" json:"bandwidthMb"`
	MaxDomains      int       `db:"max_domains" json:"maxDomains"`
	MaxSubdomains   int       `db:"max_subdomains" json:"maxSubdomains"`
	MaxMailAccounts int       `db:"max_mail_accounts" json:"maxMailAccounts"`
	MaxDatabases    int       `db:"max_databases" json:"maxDatabases"`
	MaxFTPAccounts  int       `db:"max_ftp_accounts" json:"maxFtpAccounts"`
	IsActive        bool      `db:"is_active" json:"isActive"`
	SortOrder       int       `db:"sort_order" json:"sortOrder"`
	CreatedAt       time.Time `db:"created_at" json:"createdAt"`
}

// HostingAccount represents a user's hosting account on a panel server
type HostingAccount struct {
	ID                     string        `db:"id" json:"id"`
	UserID                 string        `db:"user_id" json:"userId"`
	ServerID               *string       `db:"server_id" json:"serverId,omitempty"`
	PackageID              *string       `db:"package_id" json:"packageId,omitempty"`
	PanelUsername          string        `db:"panel_username" json:"panelUsername"`
	PanelPasswordEncrypted string        `db:"panel_password_encrypted" json:"-"`
	Status                 AccountStatus `db:"status" json:"status"`
	ProvisioningError      *string       `db:"provisioning_error" json:"provisioningError,omitempty"`
	CustomPrice            *float64      `db:"custom_price" json:"customPrice,omitempty"`
	NextBillingAt          *time.Time    `db:"next_billing_at" json:"nextBillingAt,omitempty"`
	CreatedAt              time.Time     `db:"created_at" json:"createdAt"`
	UpdatedAt              time.Time     `db:"updated_at" json:"updatedAt"`

	// Joined fields (populated by queries with JOINs)
	ServerName     string `db:"server_name" json:"serverName,omitempty"`
	ServerHostname string `db:"server_hostname" json:"serverHostname,omitempty"`
	PanelURL       string `db:"panel_url" json:"panelUrl,omitempty"`
	PackageName   string  `db:"package_name" json:"packageName,omitempty"`
	PackagePrice  float64 `db:"package_price" json:"packagePrice,omitempty"`
	UserEmail     string  `db:"user_email" json:"userEmail,omitempty"`
	DomainCount   int     `db:"domain_count" json:"domainCount,omitempty"`
	PrimaryDomain string  `db:"primary_domain" json:"primaryDomain,omitempty"`
}

// HostingDomain represents a domain hosted on a hosting account
// Domain setup status constants
const (
	DomainStatusPendingDNS    = "pending_dns"
	DomainStatusSSLGenerating = "ssl_generating"
	DomainStatusActive        = "active"
)

type HostingDomain struct {
	ID          string    `db:"id" json:"id"`
	AccountID   string    `db:"account_id" json:"accountId"`
	Domain      string    `db:"domain" json:"domain"`
	SSLEnabled  bool      `db:"ssl_enabled" json:"sslEnabled"`
	DNSVerified bool      `db:"dns_verified" json:"dnsVerified"`
	SetupStatus string    `db:"setup_status" json:"setupStatus"`
	SSLError    *string   `db:"ssl_error" json:"sslError,omitempty"`
	CreatedAt   time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt   time.Time `db:"updated_at" json:"updatedAt"`
}

// HostingDomainSettings contains antibot protection settings for a domain
type HostingDomainSettings struct {
	ID               string    `db:"id" json:"id"`
	DomainID         string    `db:"domain_id" json:"domainId"`
	CountryMode      string    `db:"country_mode" json:"countryMode"`
	CountryListRaw   string    `db:"country_list" json:"-"`
	CountryList      []string  `json:"countryList"`
	DeviceMode       string    `db:"device_mode" json:"deviceMode"`
	DeviceListRaw    string    `db:"device_list" json:"-"`
	DeviceList       []string  `json:"deviceList"`
	BlockBots        bool      `db:"block_bots" json:"blockBots"`
	BlockTor         bool      `db:"block_tor" json:"blockTor"`
	BlockProxy       bool      `db:"block_proxy" json:"blockProxy"`
	BlockDatacenter  bool      `db:"block_datacenter" json:"blockDatacenter"`
	BlockHeadless    bool      `db:"block_headless" json:"blockHeadless"`
	MinBehaviorScore int       `db:"min_behavior_score" json:"minBehaviorScore"`
	RedirectOnBlock  string    `db:"redirect_on_block" json:"redirectOnBlock"`
	UpdatedAt        time.Time `db:"updated_at" json:"updatedAt"`
}

// HostingVisit represents a visit to a hosting domain (for analytics)
type HostingVisit struct {
	ID        string `db:"id" json:"id"`
	DomainID  string `db:"domain_id" json:"domainId"`
	AccountID string `db:"account_id" json:"accountId"`

	// Request info
	IP       string `db:"ip" json:"ip"`
	Path     string `db:"path" json:"path"`
	Method   string `db:"method" json:"method"`
	Country  string `db:"country" json:"country"`
	City     string `db:"city" json:"city"`
	ASN      int    `db:"asn" json:"asn"`
	ASNOrg   string `db:"asn_org" json:"asnOrg"`

	// Device info
	Device           string `db:"device" json:"device"`
	Browser          string `db:"browser" json:"browser"`
	OS               string `db:"os" json:"os"`
	UserAgent        string `db:"user_agent" json:"userAgent"`
	Language         string `db:"language" json:"language"`
	Timezone         string `db:"timezone" json:"timezone"`
	ScreenResolution string `db:"screen_resolution" json:"screenResolution"`

	// Referrer
	Referrer       string `db:"referrer" json:"referrer"`
	ReferrerDomain string `db:"referrer_domain" json:"referrerDomain"`

	// UTM tracking
	UTMSource   string `db:"utm_source" json:"utmSource"`
	UTMMedium   string `db:"utm_medium" json:"utmMedium"`
	UTMCampaign string `db:"utm_campaign" json:"utmCampaign"`
	UTMTerm     string `db:"utm_term" json:"utmTerm"`
	UTMContent  string `db:"utm_content" json:"utmContent"`

	// Bot detection
	IsBot          bool    `db:"is_bot" json:"isBot"`
	BotScore       float64 `db:"bot_score" json:"botScore"`
	BehaviorScore  int     `db:"behavior_score" json:"behaviorScore"`
	AutomationTool string  `db:"automation_tool" json:"automationTool"`
	IsHeadless     bool    `db:"is_headless" json:"isHeadless"`
	IsTor          bool    `db:"is_tor" json:"isTor"`
	IsProxy        bool    `db:"is_proxy" json:"isProxy"`
	IsDatacenter   bool    `db:"is_datacenter" json:"isDatacenter"`
	Fingerprint    string  `db:"fingerprint" json:"fingerprint"`

	// Action taken
	Action      string `db:"action" json:"action"`
	Blocked     bool   `db:"blocked" json:"blocked"`
	BlockReason string `db:"block_reason" json:"blockReason"`

	// Session
	SessionID string `db:"session_id" json:"sessionId"`

	// Timestamps
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
}

// BalanceTransaction represents a balance top-up or deduction
type BalanceTransaction struct {
	ID          string    `db:"id" json:"id"`
	UserID      string    `db:"user_id" json:"userId"`
	Amount      float64   `db:"amount" json:"amount"`
	Type        string    `db:"type" json:"type"` // "topup" or "deduct"
	Description string    `db:"description" json:"description"`
	CreatedAt   time.Time `db:"created_at" json:"createdAt"`
}

// Input types for API requests

// CreateServerInput is the input for creating a hosting server
type CreateServerInput struct {
	Name        string     `json:"name" validate:"required"`
	Type        ServerType `json:"type"` // cloudpanel (default) or hestiacp
	Hostname    string     `json:"hostname" validate:"required"`
	PanelURL    string     `json:"panelUrl" validate:"required"`
	Port        int        `json:"port"`
	Username    string     `json:"username" validate:"required"`
	Password    string     `json:"password" validate:"required"`
	MaxAccounts int        `json:"maxAccounts"`

	// Antibot dashboard (where users manage domain settings)
	AntibotDashboardURL string `json:"antibotDashboardUrl"`
	AntibotPassword     string `json:"antibotPassword"`
}

// UpdateServerInput is the input for updating a hosting server
type UpdateServerInput struct {
	Name        string     `json:"name"`
	Type        ServerType `json:"type"`
	Hostname    string     `json:"hostname"`
	PanelURL    string     `json:"panelUrl"`
	Port        int        `json:"port"`
	Username    string     `json:"username"`
	Password    string     `json:"password"` // empty = keep current
	MaxAccounts int        `json:"maxAccounts"`

	// Antibot dashboard
	AntibotDashboardURL string `json:"antibotDashboardUrl"`
	AntibotPassword     string `json:"antibotPassword"` // empty = keep current
}

// CreatePackageInput is the input for creating a hosting package
type CreatePackageInput struct {
	Name            string  `json:"name" validate:"required"`
	Description     string  `json:"description"`
	PriceMonthly    float64 `json:"priceMonthly" validate:"required"`
	DiskMB          int     `json:"diskMb" validate:"required"`
	BandwidthMB     int     `json:"bandwidthMb" validate:"required"`
	MaxDomains      int     `json:"maxDomains" validate:"required"`
	MaxSubdomains   int     `json:"maxSubdomains"`
	MaxMailAccounts int     `json:"maxMailAccounts"`
	MaxDatabases    int     `json:"maxDatabases"`
	MaxFTPAccounts  int     `json:"maxFtpAccounts"`
}

// PurchaseInput is the input for purchasing hosting
type PurchaseInput struct {
	PackageID string `json:"packageId" validate:"required"`
}

// AddDomainInput is the input for adding a domain (admin only)
type AddDomainInput struct {
	Domain string `json:"domain" validate:"required"`
}

// LinkAccountInput is the input for admin linking an account to a server
type LinkAccountInput struct {
	ServerID string `json:"serverId" validate:"required"`
}

// UpdateDomainSettingsInput is the input for updating domain antibot settings
type UpdateDomainSettingsInput struct {
	CountryMode      string   `json:"countryMode"`
	CountryList      []string `json:"countryList"`
	DeviceMode       string   `json:"deviceMode"`
	DeviceList       []string `json:"deviceList"`
	BlockBots        bool     `json:"blockBots"`
	BlockTor         bool     `json:"blockTor"`
	BlockProxy       bool     `json:"blockProxy"`
	BlockDatacenter  bool     `json:"blockDatacenter"`
	BlockHeadless    bool     `json:"blockHeadless"`
	MinBehaviorScore int      `json:"minBehaviorScore"`
	RedirectOnBlock  string   `json:"redirectOnBlock"`
}

// BotectionStats contains traffic statistics from botection
type BotectionStats struct {
	TotalRequests   int64            `json:"total_requests"`
	BlockedRequests int64            `json:"blocked_requests"`
	AllowedRequests int64            `json:"allowed_requests"`
	Challenges      int64            `json:"challenges"`
	BlockRate       float64          `json:"block_rate"`
	RequestsPerMin  []int64          `json:"requests_per_min"`
	BlocksPerMin    []int64          `json:"blocks_per_min"`
	ModuleBlocks    map[string]int64 `json:"module_blocks"`
	RecentBlocks    []BlockEvent     `json:"recent_blocks"`
	Uptime          int64            `json:"uptime_seconds"`
}

// BlockEvent represents a single blocked request
type BlockEvent struct {
	Time   string  `json:"time" db:"time"`
	IP     string  `json:"ip" db:"ip"`
	Host   string  `json:"host" db:"host"`
	Path   string  `json:"path" db:"path"`
	Reason string  `json:"reason" db:"reason"`
	Module string  `json:"module" db:"module"`
	Score  float64 `json:"score" db:"score"`
}

// TopUpInput is the input for admin balance top-up
type TopUpInput struct {
	UserID string  `json:"user_id" validate:"required"`
	Amount float64 `json:"amount" validate:"required"`
}
