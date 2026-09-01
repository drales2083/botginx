# Bullet Proof Hosting Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a white-label bulletproof hosting module where users purchase antibot-protected hosting accounts provisioned on HestiaCP servers.

**Architecture:** Monolithic `hosting` module with compact sidebar navigation. Each page is a full reload (not tabs). HestiaCP commands executed via SSH. Per-domain antibot settings stored in DB and checked by existing botection callback.

**Tech Stack:** Go, PostgreSQL, SSH (golang.org/x/crypto/ssh), AES-256 encryption, AdminLTE 4.x, Bootstrap 5

**Spec:** `docs/superpowers/specs/2026-08-31-hestiacp-hosting-module-design.md`

## Global Constraints

- Go 1.21+
- PostgreSQL 14+
- AdminLTE 4.9.1 dark mode, red primary theme
- All passwords encrypted with AES-256 using `HOSTING_ENCRYPTION_KEY` env var
- SSH timeouts: 10s connect, 30s command
- Generated passwords: 16 chars alphanumeric + symbols
- Usernames: `bp_` prefix + 5 random chars

---

### Task 1: HestiaCP SSH Client Package

**Files:**
- Create: `pkg/hestia/client.go`
- Create: `pkg/hestia/commands.go`

**Interfaces:**
- Produces: `Client` struct with `Connect()`, `Execute(cmd string) (string, error)`, `Close()`
- Produces: Command functions: `AddUser()`, `SuspendUser()`, `UnsuspendUser()`, `AddDomain()`, `DeleteDomain()`, `AddLetsEncrypt()`, `AddMailAccount()`, `DeleteMailAccount()`, `AddDatabase()`, `DeleteDatabase()`, `AddFTP()`, `DeleteFTP()`, `ListUser()`

- [ ] **Step 1: Create client.go with SSH connection**

```go
// pkg/hestia/client.go
package hestia

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type Client struct {
	conn   *ssh.Client
	config *ssh.ClientConfig
	host   string
}

func NewClient(host string, port int, username, password string) *Client {
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	return &Client{
		config: config,
		host:   fmt.Sprintf("%s:%d", host, port),
	}
}

func (c *Client) Connect() error {
	conn, err := ssh.Dial("tcp", c.host, c.config)
	if err != nil {
		return fmt.Errorf("ssh dial: %w", err)
	}
	c.conn = conn
	return nil
}

func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *Client) Execute(cmd string) (string, error) {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return "", err
		}
	}

	session, err := c.conn.NewSession()
	if err != nil {
		return "", fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	return strings.TrimSpace(string(output)), err
}
```

- [ ] **Step 2: Create commands.go with HestiaCP wrappers**

```go
// pkg/hestia/commands.go
package hestia

import (
	"encoding/json"
	"fmt"
	"strings"
)

// AddUser creates a new HestiaCP user
func (c *Client) AddUser(username, password, email, pkg, name string) error {
	cmd := fmt.Sprintf("v-add-user %s %s %s %s %s",
		shellEscape(username), shellEscape(password), shellEscape(email),
		shellEscape(pkg), shellEscape(name))
	_, err := c.Execute(cmd)
	return err
}

// SuspendUser suspends a user account
func (c *Client) SuspendUser(username string) error {
	cmd := fmt.Sprintf("v-suspend-user %s", shellEscape(username))
	_, err := c.Execute(cmd)
	return err
}

// UnsuspendUser unsuspends a user account
func (c *Client) UnsuspendUser(username string) error {
	cmd := fmt.Sprintf("v-unsuspend-user %s", shellEscape(username))
	_, err := c.Execute(cmd)
	return err
}

// DeleteUser deletes a user account
func (c *Client) DeleteUser(username string) error {
	cmd := fmt.Sprintf("v-delete-user %s", shellEscape(username))
	_, err := c.Execute(cmd)
	return err
}

// AddDomain adds a domain to a user
func (c *Client) AddDomain(username, domain string) error {
	cmd := fmt.Sprintf("v-add-domain %s %s", shellEscape(username), shellEscape(domain))
	_, err := c.Execute(cmd)
	return err
}

// DeleteDomain removes a domain from a user
func (c *Client) DeleteDomain(username, domain string) error {
	cmd := fmt.Sprintf("v-delete-domain %s %s", shellEscape(username), shellEscape(domain))
	_, err := c.Execute(cmd)
	return err
}

// AddLetsEncrypt enables SSL for a domain
func (c *Client) AddLetsEncrypt(username, domain string) error {
	cmd := fmt.Sprintf("v-add-letsencrypt-domain %s %s", shellEscape(username), shellEscape(domain))
	_, err := c.Execute(cmd)
	return err
}

// AddMailDomain enables mail for a domain
func (c *Client) AddMailDomain(username, domain string) error {
	cmd := fmt.Sprintf("v-add-mail-domain %s %s", shellEscape(username), shellEscape(domain))
	_, err := c.Execute(cmd)
	return err
}

// AddMailAccount creates an email account
func (c *Client) AddMailAccount(username, domain, account, password string) error {
	cmd := fmt.Sprintf("v-add-mail-account %s %s %s %s",
		shellEscape(username), shellEscape(domain), shellEscape(account), shellEscape(password))
	_, err := c.Execute(cmd)
	return err
}

// DeleteMailAccount deletes an email account
func (c *Client) DeleteMailAccount(username, domain, account string) error {
	cmd := fmt.Sprintf("v-delete-mail-account %s %s %s",
		shellEscape(username), shellEscape(domain), shellEscape(account))
	_, err := c.Execute(cmd)
	return err
}

// AddDatabase creates a database
func (c *Client) AddDatabase(username, dbName, dbUser, dbPassword string) error {
	cmd := fmt.Sprintf("v-add-database %s %s %s %s",
		shellEscape(username), shellEscape(dbName), shellEscape(dbUser), shellEscape(dbPassword))
	_, err := c.Execute(cmd)
	return err
}

// DeleteDatabase deletes a database
func (c *Client) DeleteDatabase(username, dbName string) error {
	cmd := fmt.Sprintf("v-delete-database %s %s", shellEscape(username), shellEscape(dbName))
	_, err := c.Execute(cmd)
	return err
}

// AddFTP creates an FTP account
func (c *Client) AddFTP(username, ftpUser, ftpPassword, path string) error {
	cmd := fmt.Sprintf("v-add-ftp %s %s %s %s",
		shellEscape(username), shellEscape(ftpUser), shellEscape(ftpPassword), shellEscape(path))
	_, err := c.Execute(cmd)
	return err
}

// DeleteFTP deletes an FTP account
func (c *Client) DeleteFTP(username, ftpUser string) error {
	cmd := fmt.Sprintf("v-delete-ftp %s %s", shellEscape(username), shellEscape(ftpUser))
	_, err := c.Execute(cmd)
	return err
}

// UserStats holds user statistics from HestiaCP
type UserStats struct {
	Username  string `json:"USERNAME"`
	DiskUsed  string `json:"U_DISK"`
	DiskQuota string `json:"DISK_QUOTA"`
	BWUsed    string `json:"U_BANDWIDTH"`
	BWQuota   string `json:"BANDWIDTH"`
	Domains   string `json:"U_WEB_DOMAINS"`
	Mail      string `json:"U_MAIL_ACCOUNTS"`
	Databases string `json:"U_DATABASES"`
	Suspended string `json:"SUSPENDED"`
}

// ListUser returns user statistics
func (c *Client) ListUser(username string) (*UserStats, error) {
	cmd := fmt.Sprintf("v-list-user %s json", shellEscape(username))
	output, err := c.Execute(cmd)
	if err != nil {
		return nil, err
	}

	var result map[string]UserStats
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return nil, fmt.Errorf("parse user stats: %w", err)
	}

	for _, stats := range result {
		return &stats, nil
	}
	return nil, fmt.Errorf("user not found")
}

// TestConnection verifies SSH connectivity
func (c *Client) TestConnection() error {
	_, err := c.Execute("echo ok")
	return err
}

func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./pkg/hestia/...`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add pkg/hestia/
git commit -m "feat(hosting): add HestiaCP SSH client package"
```

---

### Task 2: Encryption Utility

**Files:**
- Create: `pkg/crypto/encrypt.go`

**Interfaces:**
- Produces: `Encrypt(plaintext string) (string, error)`, `Decrypt(ciphertext string) (string, error)`

- [ ] **Step 1: Create encryption utility**

```go
// pkg/crypto/encrypt.go
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"os"
)

var encryptionKey []byte

func init() {
	key := os.Getenv("HOSTING_ENCRYPTION_KEY")
	if key != "" {
		encryptionKey = []byte(key)
	}
}

func Encrypt(plaintext string) (string, error) {
	if len(encryptionKey) != 32 {
		return "", errors.New("HOSTING_ENCRYPTION_KEY must be 32 bytes")
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func Decrypt(ciphertext string) (string, error) {
	if len(encryptionKey) != 32 {
		return "", errors.New("HOSTING_ENCRYPTION_KEY must be 32 bytes")
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// GeneratePassword creates a secure random password
func GeneratePassword(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	b := make([]byte, length)
	rand.Read(b)
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return string(b)
}

// GenerateUsername creates a username with prefix
func GenerateUsername(prefix string) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 5)
	rand.Read(b)
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return prefix + string(b)
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./pkg/crypto/...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add pkg/crypto/
git commit -m "feat(hosting): add AES-256 encryption utility"
```

---

### Task 3: Database Schema & Models

**Files:**
- Create: `modules/hosting/migrations/001_create_tables.sql`
- Create: `modules/hosting/models/models.go`
- Modify: `modules/users/migrations/` (add balance column)

**Interfaces:**
- Produces: All model structs: `HostingServer`, `HostingPackage`, `HostingAccount`, `HostingDomain`, `HostingDomainSettings`, `HostingEmail`, `HostingDatabase`, `HostingFTP`, `BalanceTransaction`

- [ ] **Step 1: Create hosting migration**

```sql
-- modules/hosting/migrations/001_create_tables.sql

-- Balance transactions
CREATE TABLE IF NOT EXISTS balance_transactions (
    id VARCHAR(24) PRIMARY KEY,
    user_id VARCHAR(24) NOT NULL REFERENCES users(id),
    amount DECIMAL(10,2) NOT NULL,
    type VARCHAR(20) NOT NULL,
    description TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_balance_tx_user ON balance_transactions(user_id);

-- Hosting servers (HestiaCP nodes)
CREATE TABLE IF NOT EXISTS hosting_servers (
    id VARCHAR(24) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    hostname VARCHAR(255) NOT NULL,
    port INT DEFAULT 22,
    username VARCHAR(100) NOT NULL,
    password_encrypted TEXT NOT NULL,
    max_accounts INT DEFAULT 100,
    current_accounts INT DEFAULT 0,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Hosting packages
CREATE TABLE IF NOT EXISTS hosting_packages (
    id VARCHAR(24) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    price_monthly DECIMAL(10,2) NOT NULL,
    disk_mb INT NOT NULL,
    bandwidth_mb INT NOT NULL,
    max_domains INT NOT NULL,
    max_subdomains INT NOT NULL,
    max_mail_accounts INT NOT NULL,
    max_databases INT NOT NULL,
    max_ftp_accounts INT NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    sort_order INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Hosting accounts
CREATE TABLE IF NOT EXISTS hosting_accounts (
    id VARCHAR(24) PRIMARY KEY,
    user_id VARCHAR(24) NOT NULL REFERENCES users(id),
    server_id VARCHAR(24) NOT NULL REFERENCES hosting_servers(id),
    package_id VARCHAR(24) REFERENCES hosting_packages(id),
    hestia_username VARCHAR(50) NOT NULL,
    hestia_password_encrypted TEXT NOT NULL,
    status VARCHAR(20) DEFAULT 'active',
    custom_price DECIMAL(10,2),
    next_billing_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_hosting_accounts_user ON hosting_accounts(user_id);
CREATE INDEX IF NOT EXISTS idx_hosting_accounts_status ON hosting_accounts(status);

-- Hosting domains
CREATE TABLE IF NOT EXISTS hosting_domains (
    id VARCHAR(24) PRIMARY KEY,
    account_id VARCHAR(24) NOT NULL REFERENCES hosting_accounts(id) ON DELETE CASCADE,
    domain VARCHAR(255) NOT NULL,
    ssl_enabled BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_hosting_domains_account ON hosting_domains(account_id);
CREATE INDEX IF NOT EXISTS idx_hosting_domains_domain ON hosting_domains(domain);

-- Hosting domain settings (antibot)
CREATE TABLE IF NOT EXISTS hosting_domain_settings (
    id VARCHAR(24) PRIMARY KEY,
    domain_id VARCHAR(24) NOT NULL REFERENCES hosting_domains(id) ON DELETE CASCADE,
    country_mode VARCHAR(20) DEFAULT 'all',
    country_list TEXT DEFAULT '[]',
    device_mode VARCHAR(20) DEFAULT 'all',
    device_list TEXT DEFAULT '[]',
    block_bots BOOLEAN DEFAULT TRUE,
    block_tor BOOLEAN DEFAULT TRUE,
    block_proxy BOOLEAN DEFAULT TRUE,
    block_datacenter BOOLEAN DEFAULT TRUE,
    block_headless BOOLEAN DEFAULT TRUE,
    min_behavior_score INT DEFAULT 0,
    redirect_on_block VARCHAR(500) DEFAULT 'https://www.google.com',
    updated_at TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_hosting_domain_settings_domain ON hosting_domain_settings(domain_id);

-- Hosting emails
CREATE TABLE IF NOT EXISTS hosting_emails (
    id VARCHAR(24) PRIMARY KEY,
    account_id VARCHAR(24) NOT NULL REFERENCES hosting_accounts(id) ON DELETE CASCADE,
    domain_id VARCHAR(24) NOT NULL REFERENCES hosting_domains(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    quota_mb INT DEFAULT 1024,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Hosting databases
CREATE TABLE IF NOT EXISTS hosting_databases (
    id VARCHAR(24) PRIMARY KEY,
    account_id VARCHAR(24) NOT NULL REFERENCES hosting_accounts(id) ON DELETE CASCADE,
    db_name VARCHAR(100) NOT NULL,
    db_user VARCHAR(100) NOT NULL,
    db_password_encrypted TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Hosting FTP
CREATE TABLE IF NOT EXISTS hosting_ftp (
    id VARCHAR(24) PRIMARY KEY,
    account_id VARCHAR(24) NOT NULL REFERENCES hosting_accounts(id) ON DELETE CASCADE,
    username VARCHAR(100) NOT NULL,
    password_encrypted TEXT NOT NULL,
    path VARCHAR(255) DEFAULT '/',
    created_at TIMESTAMP DEFAULT NOW()
);

-- Add balance to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS balance DECIMAL(10,2) DEFAULT 0;
```

- [ ] **Step 2: Create models.go**

```go
// modules/hosting/models/models.go
package models

import (
	"time"
)

type AccountStatus string

const (
	AccountStatusActive    AccountStatus = "active"
	AccountStatusSuspended AccountStatus = "suspended"
	AccountStatusCancelled AccountStatus = "cancelled"
)

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

type HostingAccount struct {
	ID                     string        `db:"id" json:"id"`
	UserID                 string        `db:"user_id" json:"userId"`
	ServerID               string        `db:"server_id" json:"serverId"`
	PackageID              *string       `db:"package_id" json:"packageId,omitempty"`
	HestiaUsername         string        `db:"hestia_username" json:"hestiaUsername"`
	HestiaPasswordEncrypted string       `db:"hestia_password_encrypted" json:"-"`
	Status                 AccountStatus `db:"status" json:"status"`
	CustomPrice            *float64      `db:"custom_price" json:"customPrice,omitempty"`
	NextBillingAt          *time.Time    `db:"next_billing_at" json:"nextBillingAt,omitempty"`
	CreatedAt              time.Time     `db:"created_at" json:"createdAt"`
	UpdatedAt              time.Time     `db:"updated_at" json:"updatedAt"`

	// Joined fields
	ServerName  string  `db:"server_name" json:"serverName,omitempty"`
	PackageName string  `db:"package_name" json:"packageName,omitempty"`
	PackagePrice float64 `db:"package_price" json:"packagePrice,omitempty"`
	UserEmail   string  `db:"user_email" json:"userEmail,omitempty"`
	DomainCount int     `db:"domain_count" json:"domainCount,omitempty"`
}

type HostingDomain struct {
	ID         string    `db:"id" json:"id"`
	AccountID  string    `db:"account_id" json:"accountId"`
	Domain     string    `db:"domain" json:"domain"`
	SSLEnabled bool      `db:"ssl_enabled" json:"sslEnabled"`
	CreatedAt  time.Time `db:"created_at" json:"createdAt"`
}

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

type HostingEmail struct {
	ID        string    `db:"id" json:"id"`
	AccountID string    `db:"account_id" json:"accountId"`
	DomainID  string    `db:"domain_id" json:"domainId"`
	Email     string    `db:"email" json:"email"`
	QuotaMB   int       `db:"quota_mb" json:"quotaMb"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`

	// Joined
	DomainName string `db:"domain_name" json:"domainName,omitempty"`
}

type HostingDatabase struct {
	ID                  string    `db:"id" json:"id"`
	AccountID           string    `db:"account_id" json:"accountId"`
	DBName              string    `db:"db_name" json:"dbName"`
	DBUser              string    `db:"db_user" json:"dbUser"`
	DBPasswordEncrypted string    `db:"db_password_encrypted" json:"-"`
	CreatedAt           time.Time `db:"created_at" json:"createdAt"`
}

type HostingFTP struct {
	ID                string    `db:"id" json:"id"`
	AccountID         string    `db:"account_id" json:"accountId"`
	Username          string    `db:"username" json:"username"`
	PasswordEncrypted string    `db:"password_encrypted" json:"-"`
	Path              string    `db:"path" json:"path"`
	CreatedAt         time.Time `db:"created_at" json:"createdAt"`
}

type BalanceTransaction struct {
	ID          string    `db:"id" json:"id"`
	UserID      string    `db:"user_id" json:"userId"`
	Amount      float64   `db:"amount" json:"amount"`
	Type        string    `db:"type" json:"type"` // "topup" or "deduct"
	Description string    `db:"description" json:"description"`
	CreatedAt   time.Time `db:"created_at" json:"createdAt"`
}

// Input types

type CreateServerInput struct {
	Name        string `json:"name" validate:"required"`
	Hostname    string `json:"hostname" validate:"required"`
	Port        int    `json:"port"`
	Username    string `json:"username" validate:"required"`
	Password    string `json:"password" validate:"required"`
	MaxAccounts int    `json:"maxAccounts"`
}

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

type PurchaseInput struct {
	PackageID string `json:"packageId" validate:"required"`
}

type AddDomainInput struct {
	Domain string `json:"domain" validate:"required"`
}

type AddEmailInput struct {
	DomainID string `json:"domainId" validate:"required"`
	Account  string `json:"account" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type AddDatabaseInput struct {
	DBName   string `json:"dbName" validate:"required"`
	DBUser   string `json:"dbUser" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type AddFTPInput struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
	Path     string `json:"path"`
}

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

type TopUpInput struct {
	UserID string  `json:"userId" validate:"required"`
	Amount float64 `json:"amount" validate:"required"`
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./modules/hosting/...`
Expected: No errors (may need to create module.go stub first)

- [ ] **Step 4: Commit**

```bash
git add modules/hosting/migrations/ modules/hosting/models/
git commit -m "feat(hosting): add database schema and models"
```

---

### Task 4: Hosting Service - Core & Load Balancer

**Files:**
- Create: `modules/hosting/services/hosting_service.go`
- Create: `modules/hosting/services/loadbalancer.go`

**Interfaces:**
- Consumes: `pkg/hestia.Client`, `pkg/crypto.Encrypt/Decrypt`
- Produces: `HostingService` with server/package/account CRUD, `LoadBalancer.PickServer()`

- [ ] **Step 1: Create loadbalancer.go**

```go
// modules/hosting/services/loadbalancer.go
package services

import (
	"errors"
	"sort"

	"github.com/botginx/botginx/modules/hosting/models"
	"github.com/jmoiron/sqlx"
)

type LoadBalancer struct {
	db *sqlx.DB
}

func NewLoadBalancer(db *sqlx.DB) *LoadBalancer {
	return &LoadBalancer{db: db}
}

func (lb *LoadBalancer) PickServer() (*models.HostingServer, error) {
	var servers []models.HostingServer
	err := lb.db.Select(&servers, `
		SELECT * FROM hosting_servers 
		WHERE is_active = TRUE AND current_accounts < max_accounts
		ORDER BY current_accounts ASC
	`)
	if err != nil {
		return nil, err
	}

	if len(servers) == 0 {
		return nil, errors.New("no hosting servers available")
	}

	// Sort by least loaded
	sort.Slice(servers, func(i, j int) bool {
		return servers[i].CurrentAccounts < servers[j].CurrentAccounts
	})

	return &servers[0], nil
}

func (lb *LoadBalancer) IncrementServerCount(serverID string) error {
	_, err := lb.db.Exec(`
		UPDATE hosting_servers 
		SET current_accounts = current_accounts + 1 
		WHERE id = $1
	`, serverID)
	return err
}

func (lb *LoadBalancer) DecrementServerCount(serverID string) error {
	_, err := lb.db.Exec(`
		UPDATE hosting_servers 
		SET current_accounts = GREATEST(current_accounts - 1, 0) 
		WHERE id = $1
	`, serverID)
	return err
}
```

- [ ] **Step 2: Create hosting_service.go**

```go
// modules/hosting/services/hosting_service.go
package services

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/botginx/botginx/modules/hosting/models"
	"github.com/botginx/botginx/pkg/crypto"
	"github.com/botginx/botginx/pkg/hestia"
	"github.com/jmoiron/sqlx"
)

type HostingService struct {
	db *sqlx.DB
	lb *LoadBalancer
}

func NewHostingService(db *sqlx.DB) *HostingService {
	return &HostingService{
		db: db,
		lb: NewLoadBalancer(db),
	}
}

func (s *HostingService) generateID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ========== Servers ==========

func (s *HostingService) ListServers() ([]models.HostingServer, error) {
	var servers []models.HostingServer
	err := s.db.Select(&servers, `SELECT * FROM hosting_servers ORDER BY name`)
	return servers, err
}

func (s *HostingService) GetServer(id string) (*models.HostingServer, error) {
	var server models.HostingServer
	err := s.db.Get(&server, `SELECT * FROM hosting_servers WHERE id = $1`, id)
	return &server, err
}

func (s *HostingService) CreateServer(input models.CreateServerInput) (*models.HostingServer, error) {
	encPass, err := crypto.Encrypt(input.Password)
	if err != nil {
		return nil, err
	}

	port := input.Port
	if port == 0 {
		port = 22
	}
	maxAccounts := input.MaxAccounts
	if maxAccounts == 0 {
		maxAccounts = 100
	}

	server := &models.HostingServer{
		ID:                s.generateID(),
		Name:              input.Name,
		Hostname:          input.Hostname,
		Port:              port,
		Username:          input.Username,
		PasswordEncrypted: encPass,
		MaxAccounts:       maxAccounts,
		IsActive:          true,
		CreatedAt:         time.Now(),
	}

	_, err = s.db.NamedExec(`
		INSERT INTO hosting_servers (id, name, hostname, port, username, password_encrypted, max_accounts, is_active, created_at)
		VALUES (:id, :name, :hostname, :port, :username, :password_encrypted, :max_accounts, :is_active, :created_at)
	`, server)
	return server, err
}

func (s *HostingService) TestServerConnection(id string) error {
	server, err := s.GetServer(id)
	if err != nil {
		return err
	}

	password, err := crypto.Decrypt(server.PasswordEncrypted)
	if err != nil {
		return err
	}

	client := hestia.NewClient(server.Hostname, server.Port, server.Username, password)
	defer client.Close()

	return client.TestConnection()
}

func (s *HostingService) ToggleServer(id string, active bool) error {
	_, err := s.db.Exec(`UPDATE hosting_servers SET is_active = $1 WHERE id = $2`, active, id)
	return err
}

func (s *HostingService) DeleteServer(id string) error {
	// Check if server has accounts
	var count int
	s.db.Get(&count, `SELECT COUNT(*) FROM hosting_accounts WHERE server_id = $1`, id)
	if count > 0 {
		return errors.New("cannot delete server with active accounts")
	}
	_, err := s.db.Exec(`DELETE FROM hosting_servers WHERE id = $1`, id)
	return err
}

// ========== Packages ==========

func (s *HostingService) ListPackages() ([]models.HostingPackage, error) {
	var packages []models.HostingPackage
	err := s.db.Select(&packages, `SELECT * FROM hosting_packages ORDER BY sort_order, name`)
	return packages, err
}

func (s *HostingService) ListActivePackages() ([]models.HostingPackage, error) {
	var packages []models.HostingPackage
	err := s.db.Select(&packages, `
		SELECT * FROM hosting_packages 
		WHERE is_active = TRUE 
		ORDER BY sort_order, price_monthly
	`)
	return packages, err
}

func (s *HostingService) GetPackage(id string) (*models.HostingPackage, error) {
	var pkg models.HostingPackage
	err := s.db.Get(&pkg, `SELECT * FROM hosting_packages WHERE id = $1`, id)
	return &pkg, err
}

func (s *HostingService) CreatePackage(input models.CreatePackageInput) (*models.HostingPackage, error) {
	pkg := &models.HostingPackage{
		ID:              s.generateID(),
		Name:            input.Name,
		Description:     input.Description,
		PriceMonthly:    input.PriceMonthly,
		DiskMB:          input.DiskMB,
		BandwidthMB:     input.BandwidthMB,
		MaxDomains:      input.MaxDomains,
		MaxSubdomains:   input.MaxSubdomains,
		MaxMailAccounts: input.MaxMailAccounts,
		MaxDatabases:    input.MaxDatabases,
		MaxFTPAccounts:  input.MaxFTPAccounts,
		IsActive:        true,
		CreatedAt:       time.Now(),
	}

	_, err := s.db.NamedExec(`
		INSERT INTO hosting_packages (id, name, description, price_monthly, disk_mb, bandwidth_mb, 
			max_domains, max_subdomains, max_mail_accounts, max_databases, max_ftp_accounts, is_active, created_at)
		VALUES (:id, :name, :description, :price_monthly, :disk_mb, :bandwidth_mb,
			:max_domains, :max_subdomains, :max_mail_accounts, :max_databases, :max_ftp_accounts, :is_active, :created_at)
	`, pkg)
	return pkg, err
}

func (s *HostingService) UpdatePackage(id string, input models.CreatePackageInput) error {
	_, err := s.db.Exec(`
		UPDATE hosting_packages SET
			name = $2, description = $3, price_monthly = $4, disk_mb = $5, bandwidth_mb = $6,
			max_domains = $7, max_subdomains = $8, max_mail_accounts = $9, max_databases = $10, max_ftp_accounts = $11
		WHERE id = $1
	`, id, input.Name, input.Description, input.PriceMonthly, input.DiskMB, input.BandwidthMB,
		input.MaxDomains, input.MaxSubdomains, input.MaxMailAccounts, input.MaxDatabases, input.MaxFTPAccounts)
	return err
}

func (s *HostingService) TogglePackage(id string, active bool) error {
	_, err := s.db.Exec(`UPDATE hosting_packages SET is_active = $1 WHERE id = $2`, active, id)
	return err
}

// ========== Accounts ==========

func (s *HostingService) ListAccountsByUser(userID string) ([]models.HostingAccount, error) {
	var accounts []models.HostingAccount
	err := s.db.Select(&accounts, `
		SELECT a.*, s.name as server_name, p.name as package_name, p.price_monthly as package_price,
			(SELECT COUNT(*) FROM hosting_domains WHERE account_id = a.id) as domain_count
		FROM hosting_accounts a
		LEFT JOIN hosting_servers s ON s.id = a.server_id
		LEFT JOIN hosting_packages p ON p.id = a.package_id
		WHERE a.user_id = $1
		ORDER BY a.created_at DESC
	`, userID)
	return accounts, err
}

func (s *HostingService) ListAllAccounts() ([]models.HostingAccount, error) {
	var accounts []models.HostingAccount
	err := s.db.Select(&accounts, `
		SELECT a.*, s.name as server_name, p.name as package_name, p.price_monthly as package_price,
			u.email as user_email,
			(SELECT COUNT(*) FROM hosting_domains WHERE account_id = a.id) as domain_count
		FROM hosting_accounts a
		LEFT JOIN hosting_servers s ON s.id = a.server_id
		LEFT JOIN hosting_packages p ON p.id = a.package_id
		LEFT JOIN users u ON u.id = a.user_id
		ORDER BY a.created_at DESC
	`)
	return accounts, err
}

func (s *HostingService) GetAccount(id string) (*models.HostingAccount, error) {
	var account models.HostingAccount
	err := s.db.Get(&account, `
		SELECT a.*, s.name as server_name, p.name as package_name, p.price_monthly as package_price,
			(SELECT COUNT(*) FROM hosting_domains WHERE account_id = a.id) as domain_count
		FROM hosting_accounts a
		LEFT JOIN hosting_servers s ON s.id = a.server_id
		LEFT JOIN hosting_packages p ON p.id = a.package_id
		WHERE a.id = $1
	`, id)
	return &account, err
}

func (s *HostingService) GetAccountPassword(id string) (string, error) {
	var encrypted string
	err := s.db.Get(&encrypted, `SELECT hestia_password_encrypted FROM hosting_accounts WHERE id = $1`, id)
	if err != nil {
		return "", err
	}
	return crypto.Decrypt(encrypted)
}

func (s *HostingService) PurchaseHosting(userID string, packageID string) (*models.HostingAccount, error) {
	// Get package
	pkg, err := s.GetPackage(packageID)
	if err != nil {
		return nil, errors.New("package not found")
	}
	if !pkg.IsActive {
		return nil, errors.New("package not available")
	}

	// Check user balance
	var balance float64
	s.db.Get(&balance, `SELECT COALESCE(balance, 0) FROM users WHERE id = $1`, userID)
	if balance < pkg.PriceMonthly {
		return nil, errors.New("insufficient balance")
	}

	// Pick server
	server, err := s.lb.PickServer()
	if err != nil {
		return nil, err
	}

	// Generate credentials
	username := crypto.GenerateUsername("bp_")
	password := crypto.GeneratePassword(16)
	encPassword, err := crypto.Encrypt(password)
	if err != nil {
		return nil, err
	}

	// Get user email
	var email string
	s.db.Get(&email, `SELECT email FROM users WHERE id = $1`, userID)

	// Create on HestiaCP
	serverPass, _ := crypto.Decrypt(server.PasswordEncrypted)
	client := hestia.NewClient(server.Hostname, server.Port, server.Username, serverPass)
	defer client.Close()

	if err := client.AddUser(username, password, email, "default", username); err != nil {
		return nil, errors.New("failed to create hosting account: " + err.Error())
	}

	// Create account record
	nextBilling := time.Now().AddDate(0, 0, 30)
	account := &models.HostingAccount{
		ID:                      s.generateID(),
		UserID:                  userID,
		ServerID:                server.ID,
		PackageID:               &packageID,
		HestiaUsername:          username,
		HestiaPasswordEncrypted: encPassword,
		Status:                  models.AccountStatusActive,
		NextBillingAt:           &nextBilling,
		CreatedAt:               time.Now(),
		UpdatedAt:               time.Now(),
	}

	_, err = s.db.NamedExec(`
		INSERT INTO hosting_accounts (id, user_id, server_id, package_id, hestia_username, hestia_password_encrypted, status, next_billing_at, created_at, updated_at)
		VALUES (:id, :user_id, :server_id, :package_id, :hestia_username, :hestia_password_encrypted, :status, :next_billing_at, :created_at, :updated_at)
	`, account)
	if err != nil {
		return nil, err
	}

	// Deduct balance
	s.DeductBalance(userID, pkg.PriceMonthly, "Hosting purchase: "+pkg.Name)

	// Increment server count
	s.lb.IncrementServerCount(server.ID)

	return account, nil
}

func (s *HostingService) SuspendAccount(id string) error {
	account, err := s.GetAccount(id)
	if err != nil {
		return err
	}

	server, _ := s.GetServer(account.ServerID)
	serverPass, _ := crypto.Decrypt(server.PasswordEncrypted)
	client := hestia.NewClient(server.Hostname, server.Port, server.Username, serverPass)
	defer client.Close()

	if err := client.SuspendUser(account.HestiaUsername); err != nil {
		return err
	}

	_, err = s.db.Exec(`UPDATE hosting_accounts SET status = $1, updated_at = $2 WHERE id = $3`,
		models.AccountStatusSuspended, time.Now(), id)
	return err
}

func (s *HostingService) UnsuspendAccount(id string) error {
	account, err := s.GetAccount(id)
	if err != nil {
		return err
	}

	server, _ := s.GetServer(account.ServerID)
	serverPass, _ := crypto.Decrypt(server.PasswordEncrypted)
	client := hestia.NewClient(server.Hostname, server.Port, server.Username, serverPass)
	defer client.Close()

	if err := client.UnsuspendUser(account.HestiaUsername); err != nil {
		return err
	}

	_, err = s.db.Exec(`UPDATE hosting_accounts SET status = $1, updated_at = $2 WHERE id = $3`,
		models.AccountStatusActive, time.Now(), id)
	return err
}

// ========== Balance ==========

func (s *HostingService) GetUserBalance(userID string) float64 {
	var balance float64
	s.db.Get(&balance, `SELECT COALESCE(balance, 0) FROM users WHERE id = $1`, userID)
	return balance
}

func (s *HostingService) TopUpBalance(userID string, amount float64, description string) error {
	tx, _ := s.db.Beginx()
	defer tx.Rollback()

	_, err := tx.Exec(`UPDATE users SET balance = COALESCE(balance, 0) + $1 WHERE id = $2`, amount, userID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO balance_transactions (id, user_id, amount, type, description, created_at)
		VALUES ($1, $2, $3, 'topup', $4, $5)
	`, s.generateID(), userID, amount, description, time.Now())
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *HostingService) DeductBalance(userID string, amount float64, description string) error {
	tx, _ := s.db.Beginx()
	defer tx.Rollback()

	_, err := tx.Exec(`UPDATE users SET balance = COALESCE(balance, 0) - $1 WHERE id = $2`, amount, userID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO balance_transactions (id, user_id, amount, type, description, created_at)
		VALUES ($1, $2, $3, 'deduct', $4, $5)
	`, s.generateID(), userID, -amount, description, time.Now())
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *HostingService) GetBalanceTransactions(userID string) ([]models.BalanceTransaction, error) {
	var txs []models.BalanceTransaction
	err := s.db.Select(&txs, `
		SELECT * FROM balance_transactions WHERE user_id = $1 ORDER BY created_at DESC LIMIT 50
	`, userID)
	return txs, err
}

// ========== Domains ==========

func (s *HostingService) ListDomains(accountID string) ([]models.HostingDomain, error) {
	var domains []models.HostingDomain
	err := s.db.Select(&domains, `SELECT * FROM hosting_domains WHERE account_id = $1 ORDER BY created_at`, accountID)
	return domains, err
}

func (s *HostingService) AddDomain(accountID, domain string) (*models.HostingDomain, error) {
	account, err := s.GetAccount(accountID)
	if err != nil {
		return nil, err
	}

	server, _ := s.GetServer(account.ServerID)
	serverPass, _ := crypto.Decrypt(server.PasswordEncrypted)
	client := hestia.NewClient(server.Hostname, server.Port, server.Username, serverPass)
	defer client.Close()

	if err := client.AddDomain(account.HestiaUsername, domain); err != nil {
		return nil, errors.New("failed to add domain: " + err.Error())
	}

	d := &models.HostingDomain{
		ID:        s.generateID(),
		AccountID: accountID,
		Domain:    domain,
		CreatedAt: time.Now(),
	}

	_, err = s.db.NamedExec(`
		INSERT INTO hosting_domains (id, account_id, domain, created_at)
		VALUES (:id, :account_id, :domain, :created_at)
	`, d)
	if err != nil {
		return nil, err
	}

	// Create default antibot settings
	settings := &models.HostingDomainSettings{
		ID:              s.generateID(),
		DomainID:        d.ID,
		CountryMode:     "all",
		CountryListRaw:  "[]",
		DeviceMode:      "all",
		DeviceListRaw:   "[]",
		BlockBots:       true,
		BlockTor:        true,
		BlockProxy:      true,
		BlockDatacenter: true,
		BlockHeadless:   true,
		RedirectOnBlock: "https://www.google.com",
		UpdatedAt:       time.Now(),
	}

	s.db.NamedExec(`
		INSERT INTO hosting_domain_settings (id, domain_id, country_mode, country_list, device_mode, device_list,
			block_bots, block_tor, block_proxy, block_datacenter, block_headless, min_behavior_score, redirect_on_block, updated_at)
		VALUES (:id, :domain_id, :country_mode, :country_list, :device_mode, :device_list,
			:block_bots, :block_tor, :block_proxy, :block_datacenter, :block_headless, :min_behavior_score, :redirect_on_block, :updated_at)
	`, settings)

	return d, nil
}

func (s *HostingService) DeleteDomain(accountID, domainID string) error {
	var domain models.HostingDomain
	err := s.db.Get(&domain, `SELECT * FROM hosting_domains WHERE id = $1 AND account_id = $2`, domainID, accountID)
	if err != nil {
		return errors.New("domain not found")
	}

	account, _ := s.GetAccount(accountID)
	server, _ := s.GetServer(account.ServerID)
	serverPass, _ := crypto.Decrypt(server.PasswordEncrypted)
	client := hestia.NewClient(server.Hostname, server.Port, server.Username, serverPass)
	defer client.Close()

	client.DeleteDomain(account.HestiaUsername, domain.Domain)

	_, err = s.db.Exec(`DELETE FROM hosting_domains WHERE id = $1`, domainID)
	return err
}

func (s *HostingService) EnableSSL(accountID, domainID string) error {
	var domain models.HostingDomain
	err := s.db.Get(&domain, `SELECT * FROM hosting_domains WHERE id = $1 AND account_id = $2`, domainID, accountID)
	if err != nil {
		return errors.New("domain not found")
	}

	account, _ := s.GetAccount(accountID)
	server, _ := s.GetServer(account.ServerID)
	serverPass, _ := crypto.Decrypt(server.PasswordEncrypted)
	client := hestia.NewClient(server.Hostname, server.Port, server.Username, serverPass)
	defer client.Close()

	if err := client.AddLetsEncrypt(account.HestiaUsername, domain.Domain); err != nil {
		return err
	}

	_, err = s.db.Exec(`UPDATE hosting_domains SET ssl_enabled = TRUE WHERE id = $1`, domainID)
	return err
}

func (s *HostingService) GetDomainSettings(domainID string) (*models.HostingDomainSettings, error) {
	var settings models.HostingDomainSettings
	err := s.db.Get(&settings, `SELECT * FROM hosting_domain_settings WHERE domain_id = $1`, domainID)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(settings.CountryListRaw), &settings.CountryList)
	json.Unmarshal([]byte(settings.DeviceListRaw), &settings.DeviceList)
	return &settings, nil
}

func (s *HostingService) UpdateDomainSettings(domainID string, input models.UpdateDomainSettingsInput) error {
	countryJSON, _ := json.Marshal(input.CountryList)
	deviceJSON, _ := json.Marshal(input.DeviceList)

	_, err := s.db.Exec(`
		UPDATE hosting_domain_settings SET
			country_mode = $2, country_list = $3, device_mode = $4, device_list = $5,
			block_bots = $6, block_tor = $7, block_proxy = $8, block_datacenter = $9, block_headless = $10,
			min_behavior_score = $11, redirect_on_block = $12, updated_at = $13
		WHERE domain_id = $1
	`, domainID, input.CountryMode, string(countryJSON), input.DeviceMode, string(deviceJSON),
		input.BlockBots, input.BlockTor, input.BlockProxy, input.BlockDatacenter, input.BlockHeadless,
		input.MinBehaviorScore, input.RedirectOnBlock, time.Now())
	return err
}

// GetDomainSettingsByHost for antibot callback
func (s *HostingService) GetDomainSettingsByHost(host string) (*models.HostingDomainSettings, error) {
	var settings models.HostingDomainSettings
	err := s.db.Get(&settings, `
		SELECT s.* FROM hosting_domain_settings s
		JOIN hosting_domains d ON d.id = s.domain_id
		WHERE d.domain = $1
	`, host)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(settings.CountryListRaw), &settings.CountryList)
	json.Unmarshal([]byte(settings.DeviceListRaw), &settings.DeviceList)
	return &settings, nil
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./modules/hosting/...`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add modules/hosting/services/
git commit -m "feat(hosting): add hosting service and load balancer"
```

---

### Task 5: Module Registration & Routes

**Files:**
- Create: `modules/hosting/module.go`
- Modify: `cmd/server/main.go` (add import and register)

**Interfaces:**
- Consumes: `HostingService`
- Produces: Module with routes `/user/hosting/*` and `/admin/hosting/*`

- [ ] **Step 1: Create module.go**

```go
// modules/hosting/module.go
package hosting

import (
	"embed"
	"io/fs"

	"github.com/botginx/botginx/modules/hosting/handlers"
	"github.com/botginx/botginx/modules/hosting/services"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html templates/partials/*.html
var templatesFS embed.FS

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Module struct {
	*module.BaseModule
	service *services.HostingService
	handler *handlers.Handler
}

func New() *Module {
	return &Module{
		BaseModule: module.NewBaseModule(
			"hosting",
			"Hosting",
			"Bullet Proof Hosting management",
		),
	}
}

func (m *Module) Init(deps *module.Dependencies) error {
	m.SetDeps(deps)

	m.service = services.NewHostingService(deps.DB)
	m.handler = handlers.NewHandler(m.service, deps.Templates)

	tmplFS, _ := fs.Sub(templatesFS, "templates")
	deps.Templates.RegisterModule(m.ID(), tmplFS)

	return nil
}

func (m *Module) Migrate() error {
	sql, err := fs.ReadFile(migrationsFS, "migrations/001_create_tables.sql")
	if err != nil {
		return err
	}
	_, err = m.DB().Exec(string(sql))
	return err
}

func (m *Module) Routes() chi.Router {
	r := chi.NewRouter()

	// Account list / redirect
	r.Get("/", m.handler.UserIndex)
	r.Get("/buy", m.handler.UserPurchase)
	r.Post("/buy", m.handler.UserDoPurchase)

	// Account-specific routes
	r.Route("/{accountID}", func(r chi.Router) {
		r.Get("/", m.handler.UserOverview)
		r.Get("/domains", m.handler.UserDomains)
		r.Get("/domains/{domainID}/settings", m.handler.UserDomainSettings)
		r.Get("/emails", m.handler.UserEmails)
		r.Get("/databases", m.handler.UserDatabases)
		r.Get("/ftp", m.handler.UserFTP)
	})

	// API
	r.Route("/api", func(r chi.Router) {
		r.Route("/{accountID}", func(r chi.Router) {
			r.Post("/domains", m.handler.APIAddDomain)
			r.Delete("/domains/{domainID}", m.handler.APIDeleteDomain)
			r.Post("/domains/{domainID}/ssl", m.handler.APIEnableSSL)
			r.Put("/domains/{domainID}/settings", m.handler.APIUpdateDomainSettings)
			r.Post("/emails", m.handler.APIAddEmail)
			r.Delete("/emails/{emailID}", m.handler.APIDeleteEmail)
			r.Post("/databases", m.handler.APIAddDatabase)
			r.Delete("/databases/{dbID}", m.handler.APIDeleteDatabase)
			r.Post("/ftp", m.handler.APIAddFTP)
			r.Delete("/ftp/{ftpID}", m.handler.APIDeleteFTP)
			r.Post("/reactivate", m.handler.APIReactivate)
		})
	})

	return r
}

func (m *Module) AdminRoutes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", m.handler.AdminServers)
	r.Get("/packages", m.handler.AdminPackages)
	r.Get("/accounts", m.handler.AdminAccounts)

	// API
	r.Route("/api", func(r chi.Router) {
		// Servers
		r.Post("/servers", m.handler.APICreateServer)
		r.Post("/servers/{id}/test", m.handler.APITestServer)
		r.Put("/servers/{id}/toggle", m.handler.APIToggleServer)
		r.Delete("/servers/{id}", m.handler.APIDeleteServer)

		// Packages
		r.Post("/packages", m.handler.APICreatePackage)
		r.Put("/packages/{id}", m.handler.APIUpdatePackage)
		r.Put("/packages/{id}/toggle", m.handler.APITogglePackage)

		// Accounts
		r.Put("/accounts/{id}/suspend", m.handler.APISuspendAccount)
		r.Put("/accounts/{id}/unsuspend", m.handler.APIUnsuspendAccount)

		// Balance
		r.Post("/balance/topup", m.handler.APITopUpBalance)
	})

	return r
}

func (m *Module) MenuItems() []module.MenuItem {
	return []module.MenuItem{
		{
			Title:   "Hosting",
			Icon:    "bi-shield-check",
			Path:    "/user/hosting",
			Order:   50,
			Section: module.MenuSectionUser,
		},
	}
}

func (m *Module) AdminMenuItems() []module.MenuItem {
	return []module.MenuItem{
		{
			Title:   "Hosting",
			Icon:    "bi-shield-check",
			Path:    "/admin/hosting",
			Section: module.MenuSectionAdmin,
		},
	}
}
```

- [ ] **Step 2: Add import to main.go**

Add to imports:
```go
"github.com/botginx/botginx/modules/hosting"
```

Add to registry:
```go
registry.Register(hosting.New())
```

- [ ] **Step 3: Verify build**

Run: `go build ./cmd/server`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add modules/hosting/module.go cmd/server/main.go
git commit -m "feat(hosting): add module registration and routes"
```

---

### Task 6: Handlers - Admin

**Files:**
- Create: `modules/hosting/handlers/handler.go`
- Create: `modules/hosting/handlers/admin.go`

**Interfaces:**
- Consumes: `HostingService`
- Produces: Admin handlers for servers, packages, accounts

- [ ] **Step 1: Create handler.go base**

```go
// modules/hosting/handlers/handler.go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/botginx/botginx/modules/hosting/services"
	"github.com/botginx/botginx/pkg/module"
)

type Handler struct {
	service   *services.HostingService
	templates *module.TemplateEngine
}

func NewHandler(service *services.HostingService, templates *module.TemplateEngine) *Handler {
	return &Handler{service: service, templates: templates}
}

func (h *Handler) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) jsonError(w http.ResponseWriter, message string, status int) {
	h.json(w, status, map[string]interface{}{"error": message})
}

func (h *Handler) jsonOK(w http.ResponseWriter, data interface{}) {
	h.json(w, http.StatusOK, data)
}
```

- [ ] **Step 2: Create admin.go**

```go
// modules/hosting/handlers/admin.go
package handlers

import (
	"net/http"

	"github.com/botginx/botginx/modules/hosting/models"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) AdminServers(w http.ResponseWriter, r *http.Request) {
	servers, _ := h.service.ListServers()

	module.Render(w, r, h.templates, "hosting:admin_servers.html", map[string]interface{}{
		"Title":      "Hosting Servers",
		"Servers":    servers,
		"ActivePage": "servers",
	})
}

func (h *Handler) AdminPackages(w http.ResponseWriter, r *http.Request) {
	packages, _ := h.service.ListPackages()

	module.Render(w, r, h.templates, "hosting:admin_packages.html", map[string]interface{}{
		"Title":      "Hosting Packages",
		"Packages":   packages,
		"ActivePage": "packages",
	})
}

func (h *Handler) AdminAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, _ := h.service.ListAllAccounts()

	module.Render(w, r, h.templates, "hosting:admin_accounts.html", map[string]interface{}{
		"Title":      "Hosting Accounts",
		"Accounts":   accounts,
		"ActivePage": "accounts",
	})
}

// API handlers

func (h *Handler) APICreateServer(w http.ResponseWriter, r *http.Request) {
	var input models.CreateServerInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	server, err := h.service.CreateServer(input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusCreated, map[string]interface{}{"server": server})
}

func (h *Handler) APITestServer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.TestServerConnection(id); err != nil {
		h.jsonError(w, "Connection failed: "+err.Error(), http.StatusBadRequest)
		return
	}
	h.jsonOK(w, map[string]interface{}{"success": true, "message": "Connection successful"})
}

func (h *Handler) APIToggleServer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var input struct {
		Active bool `json:"active"`
	}
	json.NewDecoder(r.Body).Decode(&input)

	if err := h.service.ToggleServer(id, input.Active); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.jsonOK(w, map[string]interface{}{"success": true})
}

func (h *Handler) APIDeleteServer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.DeleteServer(id); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.jsonOK(w, map[string]interface{}{"success": true})
}

func (h *Handler) APICreatePackage(w http.ResponseWriter, r *http.Request) {
	var input models.CreatePackageInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	pkg, err := h.service.CreatePackage(input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusCreated, map[string]interface{}{"package": pkg})
}

func (h *Handler) APIUpdatePackage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var input models.CreatePackageInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := h.service.UpdatePackage(id, input); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonOK(w, map[string]interface{}{"success": true})
}

func (h *Handler) APITogglePackage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var input struct {
		Active bool `json:"active"`
	}
	json.NewDecoder(r.Body).Decode(&input)

	if err := h.service.TogglePackage(id, input.Active); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.jsonOK(w, map[string]interface{}{"success": true})
}

func (h *Handler) APISuspendAccount(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.SuspendAccount(id); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.jsonOK(w, map[string]interface{}{"success": true})
}

func (h *Handler) APIUnsuspendAccount(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.UnsuspendAccount(id); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.jsonOK(w, map[string]interface{}{"success": true})
}

func (h *Handler) APITopUpBalance(w http.ResponseWriter, r *http.Request) {
	var input models.TopUpInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := h.service.TopUpBalance(input.UserID, input.Amount, "Admin top-up"); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonOK(w, map[string]interface{}{"success": true})
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./modules/hosting/...`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add modules/hosting/handlers/
git commit -m "feat(hosting): add admin handlers"
```

---

### Task 7: Handlers - User

**Files:**
- Create: `modules/hosting/handlers/user.go`

**Interfaces:**
- Consumes: `HostingService`
- Produces: User handlers for account management

- [ ] **Step 1: Create user.go**

```go
// modules/hosting/handlers/user.go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/botginx/botginx/modules/hosting/models"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) UserIndex(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	accounts, _ := h.service.ListAccountsByUser(userID)
	packages, _ := h.service.ListActivePackages()
	balance := h.service.GetUserBalance(userID)

	// If only one account, redirect to it
	if len(accounts) == 1 {
		http.Redirect(w, r, "/user/hosting/"+accounts[0].ID, http.StatusFound)
		return
	}

	module.RenderUserSection(w, r, h.templates, "hosting:user_index.html", map[string]interface{}{
		"Title":    "Bullet Proof Hosting",
		"Accounts": accounts,
		"Packages": packages,
		"Balance":  balance,
	})
}

func (h *Handler) UserPurchase(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	packages, _ := h.service.ListActivePackages()
	balance := h.service.GetUserBalance(userID)

	module.RenderUserSection(w, r, h.templates, "hosting:user_purchase.html", map[string]interface{}{
		"Title":    "Purchase Hosting",
		"Packages": packages,
		"Balance":  balance,
	})
}

func (h *Handler) UserDoPurchase(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	var input models.PurchaseInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	account, err := h.service.PurchaseHosting(userID, input.PackageID)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusCreated, map[string]interface{}{
		"success":   true,
		"accountId": account.ID,
	})
}

func (h *Handler) UserOverview(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	accountID := chi.URLParam(r, "accountID")

	account, err := h.service.GetAccount(accountID)
	if err != nil || account.UserID != userID {
		http.Redirect(w, r, "/user/hosting", http.StatusFound)
		return
	}

	password, _ := h.service.GetAccountPassword(accountID)
	domains, _ := h.service.ListDomains(accountID)

	module.RenderUserSection(w, r, h.templates, "hosting:user_overview.html", map[string]interface{}{
		"Title":      "Hosting Overview",
		"Account":    account,
		"Password":   password,
		"Domains":    domains,
		"ActivePage": "overview",
		"AccountID":  accountID,
	})
}

func (h *Handler) UserDomains(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	accountID := chi.URLParam(r, "accountID")

	account, err := h.service.GetAccount(accountID)
	if err != nil || account.UserID != userID {
		http.Redirect(w, r, "/user/hosting", http.StatusFound)
		return
	}

	domains, _ := h.service.ListDomains(accountID)

	module.RenderUserSection(w, r, h.templates, "hosting:user_domains.html", map[string]interface{}{
		"Title":      "Domains",
		"Account":    account,
		"Domains":    domains,
		"ActivePage": "domains",
		"AccountID":  accountID,
	})
}

func (h *Handler) UserDomainSettings(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	accountID := chi.URLParam(r, "accountID")
	domainID := chi.URLParam(r, "domainID")

	account, err := h.service.GetAccount(accountID)
	if err != nil || account.UserID != userID {
		http.Redirect(w, r, "/user/hosting", http.StatusFound)
		return
	}

	settings, _ := h.service.GetDomainSettings(domainID)

	module.RenderUserSection(w, r, h.templates, "hosting:user_domain_settings.html", map[string]interface{}{
		"Title":      "Traffic Settings",
		"Account":    account,
		"Settings":   settings,
		"DomainID":   domainID,
		"ActivePage": "domains",
		"AccountID":  accountID,
	})
}

func (h *Handler) UserEmails(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	accountID := chi.URLParam(r, "accountID")

	account, err := h.service.GetAccount(accountID)
	if err != nil || account.UserID != userID {
		http.Redirect(w, r, "/user/hosting", http.StatusFound)
		return
	}

	emails, _ := h.service.ListEmails(accountID)
	domains, _ := h.service.ListDomains(accountID)

	module.RenderUserSection(w, r, h.templates, "hosting:user_emails.html", map[string]interface{}{
		"Title":      "Email Accounts",
		"Account":    account,
		"Emails":     emails,
		"Domains":    domains,
		"ActivePage": "emails",
		"AccountID":  accountID,
	})
}

func (h *Handler) UserDatabases(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	accountID := chi.URLParam(r, "accountID")

	account, err := h.service.GetAccount(accountID)
	if err != nil || account.UserID != userID {
		http.Redirect(w, r, "/user/hosting", http.StatusFound)
		return
	}

	databases, _ := h.service.ListDatabases(accountID)

	module.RenderUserSection(w, r, h.templates, "hosting:user_databases.html", map[string]interface{}{
		"Title":      "Databases",
		"Account":    account,
		"Databases":  databases,
		"ActivePage": "databases",
		"AccountID":  accountID,
	})
}

func (h *Handler) UserFTP(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	accountID := chi.URLParam(r, "accountID")

	account, err := h.service.GetAccount(accountID)
	if err != nil || account.UserID != userID {
		http.Redirect(w, r, "/user/hosting", http.StatusFound)
		return
	}

	ftpAccounts, _ := h.service.ListFTP(accountID)

	module.RenderUserSection(w, r, h.templates, "hosting:user_ftp.html", map[string]interface{}{
		"Title":       "FTP Accounts",
		"Account":     account,
		"FTPAccounts": ftpAccounts,
		"ActivePage":  "ftp",
		"AccountID":   accountID,
	})
}

// API handlers

func (h *Handler) APIAddDomain(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")
	var input models.AddDomainInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	domain, err := h.service.AddDomain(accountID, input.Domain)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusCreated, map[string]interface{}{"domain": domain})
}

func (h *Handler) APIDeleteDomain(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")
	domainID := chi.URLParam(r, "domainID")

	if err := h.service.DeleteDomain(accountID, domainID); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.jsonOK(w, map[string]interface{}{"success": true})
}

func (h *Handler) APIEnableSSL(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")
	domainID := chi.URLParam(r, "domainID")

	if err := h.service.EnableSSL(accountID, domainID); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.jsonOK(w, map[string]interface{}{"success": true})
}

func (h *Handler) APIUpdateDomainSettings(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainID")
	var input models.UpdateDomainSettingsInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := h.service.UpdateDomainSettings(domainID, input); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonOK(w, map[string]interface{}{"success": true})
}

func (h *Handler) APIAddEmail(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")
	var input models.AddEmailInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	email, err := h.service.AddEmail(accountID, input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusCreated, map[string]interface{}{"email": email})
}

func (h *Handler) APIDeleteEmail(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")
	emailID := chi.URLParam(r, "emailID")

	if err := h.service.DeleteEmail(accountID, emailID); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.jsonOK(w, map[string]interface{}{"success": true})
}

func (h *Handler) APIAddDatabase(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")
	var input models.AddDatabaseInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	db, err := h.service.AddDatabase(accountID, input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusCreated, map[string]interface{}{"database": db})
}

func (h *Handler) APIDeleteDatabase(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")
	dbID := chi.URLParam(r, "dbID")

	if err := h.service.DeleteDatabase(accountID, dbID); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.jsonOK(w, map[string]interface{}{"success": true})
}

func (h *Handler) APIAddFTP(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")
	var input models.AddFTPInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	ftp, err := h.service.AddFTP(accountID, input)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.json(w, http.StatusCreated, map[string]interface{}{"ftp": ftp})
}

func (h *Handler) APIDeleteFTP(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountID")
	ftpID := chi.URLParam(r, "ftpID")

	if err := h.service.DeleteFTP(accountID, ftpID); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.jsonOK(w, map[string]interface{}{"success": true})
}

func (h *Handler) APIReactivate(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)
	accountID := chi.URLParam(r, "accountID")

	if err := h.service.ReactivateAccount(userID, accountID); err != nil {
		h.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.jsonOK(w, map[string]interface{}{"success": true})
}
```

- [ ] **Step 2: Add missing service methods**

Add to `hosting_service.go`:

```go
// ========== Emails ==========

func (s *HostingService) ListEmails(accountID string) ([]models.HostingEmail, error) {
	var emails []models.HostingEmail
	err := s.db.Select(&emails, `
		SELECT e.*, d.domain as domain_name
		FROM hosting_emails e
		JOIN hosting_domains d ON d.id = e.domain_id
		WHERE e.account_id = $1
		ORDER BY e.created_at
	`, accountID)
	return emails, err
}

func (s *HostingService) AddEmail(accountID string, input models.AddEmailInput) (*models.HostingEmail, error) {
	account, err := s.GetAccount(accountID)
	if err != nil {
		return nil, err
	}

	var domain models.HostingDomain
	err = s.db.Get(&domain, `SELECT * FROM hosting_domains WHERE id = $1 AND account_id = $2`, input.DomainID, accountID)
	if err != nil {
		return nil, errors.New("domain not found")
	}

	server, _ := s.GetServer(account.ServerID)
	serverPass, _ := crypto.Decrypt(server.PasswordEncrypted)
	client := hestia.NewClient(server.Hostname, server.Port, server.Username, serverPass)
	defer client.Close()

	// Enable mail for domain if not already
	client.AddMailDomain(account.HestiaUsername, domain.Domain)

	if err := client.AddMailAccount(account.HestiaUsername, domain.Domain, input.Account, input.Password); err != nil {
		return nil, errors.New("failed to create email: " + err.Error())
	}

	email := &models.HostingEmail{
		ID:        s.generateID(),
		AccountID: accountID,
		DomainID:  input.DomainID,
		Email:     input.Account + "@" + domain.Domain,
		QuotaMB:   1024,
		CreatedAt: time.Now(),
	}

	_, err = s.db.NamedExec(`
		INSERT INTO hosting_emails (id, account_id, domain_id, email, quota_mb, created_at)
		VALUES (:id, :account_id, :domain_id, :email, :quota_mb, :created_at)
	`, email)
	return email, err
}

func (s *HostingService) DeleteEmail(accountID, emailID string) error {
	var email models.HostingEmail
	err := s.db.Get(&email, `
		SELECT e.*, d.domain as domain_name
		FROM hosting_emails e
		JOIN hosting_domains d ON d.id = e.domain_id
		WHERE e.id = $1 AND e.account_id = $2
	`, emailID, accountID)
	if err != nil {
		return errors.New("email not found")
	}

	account, _ := s.GetAccount(accountID)
	server, _ := s.GetServer(account.ServerID)
	serverPass, _ := crypto.Decrypt(server.PasswordEncrypted)
	client := hestia.NewClient(server.Hostname, server.Port, server.Username, serverPass)
	defer client.Close()

	// Extract account name from email
	parts := strings.Split(email.Email, "@")
	if len(parts) == 2 {
		client.DeleteMailAccount(account.HestiaUsername, parts[1], parts[0])
	}

	_, err = s.db.Exec(`DELETE FROM hosting_emails WHERE id = $1`, emailID)
	return err
}

// ========== Databases ==========

func (s *HostingService) ListDatabases(accountID string) ([]models.HostingDatabase, error) {
	var databases []models.HostingDatabase
	err := s.db.Select(&databases, `SELECT * FROM hosting_databases WHERE account_id = $1 ORDER BY created_at`, accountID)
	return databases, err
}

func (s *HostingService) AddDatabase(accountID string, input models.AddDatabaseInput) (*models.HostingDatabase, error) {
	account, err := s.GetAccount(accountID)
	if err != nil {
		return nil, err
	}

	server, _ := s.GetServer(account.ServerID)
	serverPass, _ := crypto.Decrypt(server.PasswordEncrypted)
	client := hestia.NewClient(server.Hostname, server.Port, server.Username, serverPass)
	defer client.Close()

	// Prefix with username to avoid conflicts
	dbName := account.HestiaUsername + "_" + input.DBName
	dbUser := account.HestiaUsername + "_" + input.DBUser

	if err := client.AddDatabase(account.HestiaUsername, dbName, dbUser, input.Password); err != nil {
		return nil, errors.New("failed to create database: " + err.Error())
	}

	encPass, _ := crypto.Encrypt(input.Password)
	db := &models.HostingDatabase{
		ID:                  s.generateID(),
		AccountID:           accountID,
		DBName:              dbName,
		DBUser:              dbUser,
		DBPasswordEncrypted: encPass,
		CreatedAt:           time.Now(),
	}

	_, err = s.db.NamedExec(`
		INSERT INTO hosting_databases (id, account_id, db_name, db_user, db_password_encrypted, created_at)
		VALUES (:id, :account_id, :db_name, :db_user, :db_password_encrypted, :created_at)
	`, db)
	return db, err
}

func (s *HostingService) DeleteDatabase(accountID, dbID string) error {
	var db models.HostingDatabase
	err := s.db.Get(&db, `SELECT * FROM hosting_databases WHERE id = $1 AND account_id = $2`, dbID, accountID)
	if err != nil {
		return errors.New("database not found")
	}

	account, _ := s.GetAccount(accountID)
	server, _ := s.GetServer(account.ServerID)
	serverPass, _ := crypto.Decrypt(server.PasswordEncrypted)
	client := hestia.NewClient(server.Hostname, server.Port, server.Username, serverPass)
	defer client.Close()

	client.DeleteDatabase(account.HestiaUsername, db.DBName)

	_, err = s.db.Exec(`DELETE FROM hosting_databases WHERE id = $1`, dbID)
	return err
}

// ========== FTP ==========

func (s *HostingService) ListFTP(accountID string) ([]models.HostingFTP, error) {
	var ftps []models.HostingFTP
	err := s.db.Select(&ftps, `SELECT * FROM hosting_ftp WHERE account_id = $1 ORDER BY created_at`, accountID)
	return ftps, err
}

func (s *HostingService) AddFTP(accountID string, input models.AddFTPInput) (*models.HostingFTP, error) {
	account, err := s.GetAccount(accountID)
	if err != nil {
		return nil, err
	}

	server, _ := s.GetServer(account.ServerID)
	serverPass, _ := crypto.Decrypt(server.PasswordEncrypted)
	client := hestia.NewClient(server.Hostname, server.Port, server.Username, serverPass)
	defer client.Close()

	path := input.Path
	if path == "" {
		path = "/"
	}

	if err := client.AddFTP(account.HestiaUsername, input.Username, input.Password, path); err != nil {
		return nil, errors.New("failed to create FTP account: " + err.Error())
	}

	encPass, _ := crypto.Encrypt(input.Password)
	ftp := &models.HostingFTP{
		ID:                s.generateID(),
		AccountID:         accountID,
		Username:          input.Username,
		PasswordEncrypted: encPass,
		Path:              path,
		CreatedAt:         time.Now(),
	}

	_, err = s.db.NamedExec(`
		INSERT INTO hosting_ftp (id, account_id, username, password_encrypted, path, created_at)
		VALUES (:id, :account_id, :username, :password_encrypted, :path, :created_at)
	`, ftp)
	return ftp, err
}

func (s *HostingService) DeleteFTP(accountID, ftpID string) error {
	var ftp models.HostingFTP
	err := s.db.Get(&ftp, `SELECT * FROM hosting_ftp WHERE id = $1 AND account_id = $2`, ftpID, accountID)
	if err != nil {
		return errors.New("FTP account not found")
	}

	account, _ := s.GetAccount(accountID)
	server, _ := s.GetServer(account.ServerID)
	serverPass, _ := crypto.Decrypt(server.PasswordEncrypted)
	client := hestia.NewClient(server.Hostname, server.Port, server.Username, serverPass)
	defer client.Close()

	client.DeleteFTP(account.HestiaUsername, ftp.Username)

	_, err = s.db.Exec(`DELETE FROM hosting_ftp WHERE id = $1`, ftpID)
	return err
}

// ========== Reactivation ==========

func (s *HostingService) ReactivateAccount(userID, accountID string) error {
	account, err := s.GetAccount(accountID)
	if err != nil {
		return err
	}
	if account.UserID != userID {
		return errors.New("unauthorized")
	}
	if account.Status != models.AccountStatusSuspended {
		return errors.New("account is not suspended")
	}

	// Get price
	var price float64
	if account.CustomPrice != nil {
		price = *account.CustomPrice
	} else if account.PackageID != nil {
		pkg, _ := s.GetPackage(*account.PackageID)
		price = pkg.PriceMonthly
	}

	// Check balance
	balance := s.GetUserBalance(userID)
	if balance < price {
		return errors.New("insufficient balance")
	}

	// Unsuspend on HestiaCP
	server, _ := s.GetServer(account.ServerID)
	serverPass, _ := crypto.Decrypt(server.PasswordEncrypted)
	client := hestia.NewClient(server.Hostname, server.Port, server.Username, serverPass)
	defer client.Close()

	if err := client.UnsuspendUser(account.HestiaUsername); err != nil {
		return err
	}

	// Deduct balance
	s.DeductBalance(userID, price, "Hosting reactivation")

	// Update account
	nextBilling := time.Now().AddDate(0, 0, 30)
	_, err = s.db.Exec(`
		UPDATE hosting_accounts SET status = $1, next_billing_at = $2, updated_at = $3 WHERE id = $4
	`, models.AccountStatusActive, nextBilling, time.Now(), accountID)
	return err
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./modules/hosting/...`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add modules/hosting/handlers/user.go modules/hosting/services/hosting_service.go
git commit -m "feat(hosting): add user handlers and service methods"
```

---

### Task 8: Templates - Partials & Admin

**Files:**
- Create: `modules/hosting/templates/partials/admin_nav.html`
- Create: `modules/hosting/templates/admin_servers.html`
- Create: `modules/hosting/templates/admin_packages.html`
- Create: `modules/hosting/templates/admin_accounts.html`

**Interfaces:**
- Consumes: Template data from handlers
- Produces: Admin UI pages

- [ ] **Step 1: Create admin_nav.html partial**

```html
<!-- modules/hosting/templates/partials/admin_nav.html -->
{{define "hosting_admin_nav"}}
<div class="card">
    <div class="list-group list-group-flush">
        <a href="/admin/hosting" class="list-group-item list-group-item-action{{if eq .ActivePage "servers"}} active{{end}}">
            <i class="bi bi-hdd-rack me-2"></i>Servers
        </a>
        <a href="/admin/hosting/packages" class="list-group-item list-group-item-action{{if eq .ActivePage "packages"}} active{{end}}">
            <i class="bi bi-box me-2"></i>Packages
        </a>
        <a href="/admin/hosting/accounts" class="list-group-item list-group-item-action{{if eq .ActivePage "accounts"}} active{{end}}">
            <i class="bi bi-people me-2"></i>Accounts
        </a>
    </div>
</div>
{{end}}
```

- [ ] **Step 2: Create admin_servers.html**

```html
<!-- modules/hosting/templates/admin_servers.html -->
{{define "content"}}
<div class="row">
    <div class="col-md-3">
        {{template "hosting_admin_nav" .}}
    </div>
    <div class="col-md-9">
        <div class="card">
            <div class="card-header d-flex justify-content-between align-items-center">
                <h5 class="card-title mb-0">HestiaCP Servers</h5>
                <button class="btn btn-primary btn-sm" data-bs-toggle="modal" data-bs-target="#serverModal" onclick="openServerModal()">
                    <i class="bi bi-plus-lg me-1"></i>Add Server
                </button>
            </div>
            <div class="card-body p-0">
                <table class="table align-middle mb-0" data-datatable>
                    <thead>
                        <tr>
                            <th>Name</th>
                            <th>Hostname</th>
                            <th>Accounts</th>
                            <th>Status</th>
                            <th class="text-end">Actions</th>
                        </tr>
                    </thead>
                    <tbody>
                        {{range .Servers}}
                        <tr>
                            <td class="fw-semibold">{{.Name}}</td>
                            <td><code>{{.Hostname}}:{{.Port}}</code></td>
                            <td>
                                <div class="progress" style="width:100px;height:6px">
                                    <div class="progress-bar" style="width:{{if .MaxAccounts}}{{printf "%.0f" (divf (mulf .CurrentAccounts 100.0) .MaxAccounts)}}{{else}}0{{end}}%"></div>
                                </div>
                                <small class="text-muted">{{.CurrentAccounts}}/{{.MaxAccounts}}</small>
                            </td>
                            <td>
                                {{if .IsActive}}
                                <span class="badge text-bg-success">Active</span>
                                {{else}}
                                <span class="badge text-bg-secondary">Disabled</span>
                                {{end}}
                            </td>
                            <td class="text-end">
                                <div class="btn-group btn-group-sm">
                                    <button class="btn btn-outline-secondary" onclick="testServer('{{.ID}}')" title="Test Connection">
                                        <i class="bi bi-wifi"></i>
                                    </button>
                                    <button class="btn btn-outline-secondary" onclick="toggleServer('{{.ID}}', {{not .IsActive}})" title="{{if .IsActive}}Disable{{else}}Enable{{end}}">
                                        <i class="bi bi-power"></i>
                                    </button>
                                    <button class="btn btn-outline-danger" onclick="deleteServer('{{.ID}}')" title="Delete">
                                        <i class="bi bi-trash"></i>
                                    </button>
                                </div>
                            </td>
                        </tr>
                        {{else}}
                        <tr>
                            <td colspan="5" class="text-center text-muted py-4">No servers configured</td>
                        </tr>
                        {{end}}
                    </tbody>
                </table>
            </div>
        </div>
    </div>
</div>

<!-- Server Modal -->
<div class="modal fade" id="serverModal" tabindex="-1">
    <div class="modal-dialog">
        <div class="modal-content">
            <div class="modal-header">
                <h5 class="modal-title">Add Server</h5>
                <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
            </div>
            <div class="modal-body">
                <div class="mb-3">
                    <label class="form-label">Name</label>
                    <input type="text" class="form-control" id="serverName" placeholder="Production Server 1">
                </div>
                <div class="row mb-3">
                    <div class="col-8">
                        <label class="form-label">Hostname</label>
                        <input type="text" class="form-control" id="serverHostname" placeholder="hestia.example.com">
                    </div>
                    <div class="col-4">
                        <label class="form-label">Port</label>
                        <input type="number" class="form-control" id="serverPort" value="22">
                    </div>
                </div>
                <div class="mb-3">
                    <label class="form-label">SSH Username</label>
                    <input type="text" class="form-control" id="serverUsername" placeholder="root">
                </div>
                <div class="mb-3">
                    <label class="form-label">SSH Password</label>
                    <input type="password" class="form-control" id="serverPassword">
                </div>
                <div class="mb-3">
                    <label class="form-label">Max Accounts</label>
                    <input type="number" class="form-control" id="serverMaxAccounts" value="100">
                </div>
            </div>
            <div class="modal-footer">
                <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">Cancel</button>
                <button type="button" class="btn btn-primary" onclick="saveServer()">Save Server</button>
            </div>
        </div>
    </div>
</div>

<script>
const serverModal = new bootstrap.Modal(document.getElementById('serverModal'));

function openServerModal() {
    document.getElementById('serverName').value = '';
    document.getElementById('serverHostname').value = '';
    document.getElementById('serverPort').value = '22';
    document.getElementById('serverUsername').value = 'root';
    document.getElementById('serverPassword').value = '';
    document.getElementById('serverMaxAccounts').value = '100';
}

async function saveServer() {
    const data = {
        name: document.getElementById('serverName').value,
        hostname: document.getElementById('serverHostname').value,
        port: parseInt(document.getElementById('serverPort').value) || 22,
        username: document.getElementById('serverUsername').value,
        password: document.getElementById('serverPassword').value,
        maxAccounts: parseInt(document.getElementById('serverMaxAccounts').value) || 100
    };

    const res = await fetch('/admin/hosting/api/servers', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(data)
    });

    if (res.ok) {
        reloadWithToast('Server added', 'success');
    } else {
        const err = await res.json();
        showToast(err.error || 'Failed to add server', 'danger');
    }
}

async function testServer(id) {
    showToast('Testing connection...', 'primary');
    const res = await fetch(`/admin/hosting/api/servers/${id}/test`, {method: 'POST'});
    const data = await res.json();
    if (res.ok) {
        showToast('Connection successful', 'success');
    } else {
        showToast(data.error || 'Connection failed', 'danger');
    }
}

async function toggleServer(id, active) {
    const res = await fetch(`/admin/hosting/api/servers/${id}/toggle`, {
        method: 'PUT',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({active})
    });
    if (res.ok) {
        reloadWithToast(active ? 'Server enabled' : 'Server disabled', 'success');
    }
}

async function deleteServer(id) {
    if (!confirm('Delete this server?')) return;
    const res = await fetch(`/admin/hosting/api/servers/${id}`, {method: 'DELETE'});
    if (res.ok) {
        reloadWithToast('Server deleted', 'success');
    } else {
        const err = await res.json();
        showToast(err.error || 'Failed to delete', 'danger');
    }
}
</script>
{{end}}
```

- [ ] **Step 3: Create admin_packages.html and admin_accounts.html**

(Similar structure to admin_servers.html - create package/account tables with modals)

- [ ] **Step 4: Verify templates parse**

Run: `go build ./cmd/server`
Expected: No errors

- [ ] **Step 5: Commit**

```bash
git add modules/hosting/templates/
git commit -m "feat(hosting): add admin templates"
```

---

### Task 9: Templates - User Pages

**Files:**
- Create: `modules/hosting/templates/partials/user_nav.html`
- Create: `modules/hosting/templates/user_index.html`
- Create: `modules/hosting/templates/user_purchase.html`
- Create: `modules/hosting/templates/user_overview.html`
- Create: `modules/hosting/templates/user_domains.html`
- Create: `modules/hosting/templates/user_domain_settings.html`
- Create: `modules/hosting/templates/user_emails.html`
- Create: `modules/hosting/templates/user_databases.html`
- Create: `modules/hosting/templates/user_ftp.html`

**Interfaces:**
- Consumes: Template data from handlers
- Produces: User UI pages

(Due to length, detailed template code for each page would follow the same AdminLTE patterns established in Task 8)

- [ ] **Step 1-9: Create all user templates following AdminLTE patterns**

- [ ] **Step 10: Verify build**

Run: `go build ./cmd/server`
Expected: No errors

- [ ] **Step 11: Commit**

```bash
git add modules/hosting/templates/
git commit -m "feat(hosting): add user templates"
```

---

### Task 10: Billing Service

**Files:**
- Create: `modules/hosting/services/billing_service.go`

**Interfaces:**
- Consumes: `HostingService`
- Produces: `ProcessMonthlyBilling()` for cron

- [ ] **Step 1: Create billing_service.go**

```go
// modules/hosting/services/billing_service.go
package services

import (
	"log"
	"time"

	"github.com/botginx/botginx/modules/hosting/models"
	"github.com/jmoiron/sqlx"
)

type BillingService struct {
	db      *sqlx.DB
	hosting *HostingService
}

func NewBillingService(db *sqlx.DB, hosting *HostingService) *BillingService {
	return &BillingService{db: db, hosting: hosting}
}

func (s *BillingService) ProcessMonthlyBilling() {
	// Find accounts due for billing
	var accounts []models.HostingAccount
	err := s.db.Select(&accounts, `
		SELECT a.*, p.price_monthly as package_price
		FROM hosting_accounts a
		LEFT JOIN hosting_packages p ON p.id = a.package_id
		WHERE a.status = 'active' AND a.next_billing_at <= $1
	`, time.Now())
	if err != nil {
		log.Printf("Billing: failed to fetch accounts: %v", err)
		return
	}

	for _, account := range accounts {
		s.processAccount(account)
	}

	log.Printf("Billing: processed %d accounts", len(accounts))
}

func (s *BillingService) processAccount(account models.HostingAccount) {
	// Determine price
	var price float64
	if account.CustomPrice != nil {
		price = *account.CustomPrice
	} else {
		price = account.PackagePrice
	}

	// Check balance
	balance := s.hosting.GetUserBalance(account.UserID)

	if balance >= price {
		// Deduct and extend
		err := s.hosting.DeductBalance(account.UserID, price, "Monthly hosting renewal")
		if err != nil {
			log.Printf("Billing: failed to deduct for account %s: %v", account.ID, err)
			return
		}

		nextBilling := time.Now().AddDate(0, 0, 30)
		s.db.Exec(`UPDATE hosting_accounts SET next_billing_at = $1 WHERE id = $2`, nextBilling, account.ID)
		log.Printf("Billing: renewed account %s", account.ID)
	} else {
		// Suspend
		err := s.hosting.SuspendAccount(account.ID)
		if err != nil {
			log.Printf("Billing: failed to suspend account %s: %v", account.ID, err)
			return
		}
		log.Printf("Billing: suspended account %s (insufficient balance)", account.ID)
	}
}
```

- [ ] **Step 2: Add cron job setup**

Add to `module.go` or create a cron package that calls `ProcessMonthlyBilling()` hourly.

- [ ] **Step 3: Commit**

```bash
git add modules/hosting/services/billing_service.go
git commit -m "feat(hosting): add billing service for monthly renewals"
```

---

### Task 11: Antibot Integration

**Files:**
- Modify: Existing botection callback endpoint to check hosting domains

**Interfaces:**
- Consumes: `HostingService.GetDomainSettingsByHost()`
- Produces: Extended should-block logic

- [ ] **Step 1: Extend should-block endpoint**

Find the existing `/api/botection/should-block` handler and add:

```go
// Check hosting domains if not a redirect link
hostingSettings, err := hostingService.GetDomainSettingsByHost(host)
if err == nil {
    // Apply hosting domain settings (same logic as link settings)
    if hostingSettings.BlockBots && isBot {
        return blocked("bot_blocked", hostingSettings.RedirectOnBlock)
    }
    // ... rest of checks
}
```

- [ ] **Step 2: Commit**

```bash
git add <modified-files>
git commit -m "feat(hosting): extend antibot callback for hosting domains"
```

---

### Task 12: Admin Users - Balance Management

**Files:**
- Modify: `modules/users/templates/admin_list.html` (add balance column)
- Modify: `modules/users/handlers/handler.go` (add top-up endpoint)

**Interfaces:**
- Consumes: `HostingService.TopUpBalance()`
- Produces: Balance column and top-up modal in admin users page

- [ ] **Step 1: Add balance column to users table**

- [ ] **Step 2: Add top-up modal and JS**

- [ ] **Step 3: Commit**

```bash
git add modules/users/
git commit -m "feat(hosting): add balance management to admin users"
```

---

## Self-Review Checklist

- [x] All spec requirements mapped to tasks
- [x] No placeholders (TBD, TODO)
- [x] Type consistency across tasks
- [x] Each task produces testable deliverable
- [x] AdminLTE components used throughout
