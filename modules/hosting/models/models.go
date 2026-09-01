// Package models contains database models for the hosting module.
package models

import (
	"time"
)

// AccountStatus represents the status of a hosting account
type AccountStatus string

const (
	AccountStatusActive    AccountStatus = "active"
	AccountStatusSuspended AccountStatus = "suspended"
	AccountStatusCancelled AccountStatus = "cancelled"
)

// HostingServer represents a HestiaCP server node
type HostingServer struct {
	ID                string    `db:"id" json:"id"`
	Name              string    `db:"name" json:"name"`
	Hostname          string    `db:"hostname" json:"hostname"`
	Port              int       `db:"port" json:"port"`
	Username          string    `db:"username" json:"username"`
	PasswordEncrypted string    `db:"password_encrypted" json:"-"`
	MaxAccounts       int       `db:"max_accounts" json:"maxAccounts"`
	CurrentAccounts   int       `db:"current_accounts" json:"currentAccounts"`
	IsActive          bool      `db:"is_active" json:"isActive"`
	CreatedAt         time.Time `db:"created_at" json:"createdAt"`
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

// HostingAccount represents a user's hosting account on a HestiaCP server
type HostingAccount struct {
	ID                      string        `db:"id" json:"id"`
	UserID                  string        `db:"user_id" json:"userId"`
	ServerID                string        `db:"server_id" json:"serverId"`
	PackageID               *string       `db:"package_id" json:"packageId,omitempty"`
	HestiaUsername          string        `db:"hestia_username" json:"hestiaUsername"`
	HestiaPasswordEncrypted string        `db:"hestia_password_encrypted" json:"-"`
	Status                  AccountStatus `db:"status" json:"status"`
	CustomPrice             *float64      `db:"custom_price" json:"customPrice,omitempty"`
	NextBillingAt           *time.Time    `db:"next_billing_at" json:"nextBillingAt,omitempty"`
	CreatedAt               time.Time     `db:"created_at" json:"createdAt"`
	UpdatedAt               time.Time     `db:"updated_at" json:"updatedAt"`

	// Joined fields (populated by queries with JOINs)
	ServerName   string  `db:"server_name" json:"serverName,omitempty"`
	PackageName  string  `db:"package_name" json:"packageName,omitempty"`
	PackagePrice float64 `db:"package_price" json:"packagePrice,omitempty"`
	UserEmail    string  `db:"user_email" json:"userEmail,omitempty"`
	DomainCount  int     `db:"domain_count" json:"domainCount,omitempty"`
}

// HostingDomain represents a domain hosted on a hosting account
type HostingDomain struct {
	ID         string    `db:"id" json:"id"`
	AccountID  string    `db:"account_id" json:"accountId"`
	Domain     string    `db:"domain" json:"domain"`
	SSLEnabled bool      `db:"ssl_enabled" json:"sslEnabled"`
	CreatedAt  time.Time `db:"created_at" json:"createdAt"`
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

// HostingEmail represents an email account on a hosted domain
type HostingEmail struct {
	ID        string    `db:"id" json:"id"`
	AccountID string    `db:"account_id" json:"accountId"`
	DomainID  string    `db:"domain_id" json:"domainId"`
	Email     string    `db:"email" json:"email"`
	QuotaMB   int       `db:"quota_mb" json:"quotaMb"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`

	// Joined field
	DomainName string `db:"domain_name" json:"domainName,omitempty"`
}

// HostingDatabase represents a database on a hosting account
type HostingDatabase struct {
	ID                  string    `db:"id" json:"id"`
	AccountID           string    `db:"account_id" json:"accountId"`
	DBName              string    `db:"db_name" json:"dbName"`
	DBUser              string    `db:"db_user" json:"dbUser"`
	DBPasswordEncrypted string    `db:"db_password_encrypted" json:"-"`
	CreatedAt           time.Time `db:"created_at" json:"createdAt"`
}

// HostingFTP represents an FTP account on a hosting account
type HostingFTP struct {
	ID                string    `db:"id" json:"id"`
	AccountID         string    `db:"account_id" json:"accountId"`
	Username          string    `db:"username" json:"username"`
	PasswordEncrypted string    `db:"password_encrypted" json:"-"`
	Path              string    `db:"path" json:"path"`
	CreatedAt         time.Time `db:"created_at" json:"createdAt"`
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
	Name        string `json:"name" validate:"required"`
	Hostname    string `json:"hostname" validate:"required"`
	Port        int    `json:"port"`
	Username    string `json:"username" validate:"required"`
	Password    string `json:"password" validate:"required"`
	MaxAccounts int    `json:"maxAccounts"`
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

// AddDomainInput is the input for adding a domain
type AddDomainInput struct {
	Domain string `json:"domain" validate:"required"`
}

// AddEmailInput is the input for creating an email account
type AddEmailInput struct {
	DomainID string `json:"domainId" validate:"required"`
	Account  string `json:"account" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// AddDatabaseInput is the input for creating a database
type AddDatabaseInput struct {
	DBName   string `json:"dbName" validate:"required"`
	DBUser   string `json:"dbUser" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// AddFTPInput is the input for creating an FTP account
type AddFTPInput struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
	Path     string `json:"path"`
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

// TopUpInput is the input for admin balance top-up
type TopUpInput struct {
	UserID string  `json:"userId" validate:"required"`
	Amount float64 `json:"amount" validate:"required"`
}
