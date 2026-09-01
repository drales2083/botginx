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

// HostingService provides business logic for hosting operations
type HostingService struct {
	db *sqlx.DB
	lb *LoadBalancer
}

// NewHostingService creates a new hosting service instance
func NewHostingService(db *sqlx.DB) *HostingService {
	return &HostingService{
		db: db,
		lb: NewLoadBalancer(db),
	}
}

// generateID creates a unique 24-character hex ID
func (s *HostingService) generateID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ========== Servers ==========

// ListServers returns all hosting servers ordered by name
func (s *HostingService) ListServers() ([]models.HostingServer, error) {
	var servers []models.HostingServer
	err := s.db.Select(&servers, `SELECT * FROM hosting_servers ORDER BY name`)
	return servers, err
}

// GetServer retrieves a server by ID
func (s *HostingService) GetServer(id string) (*models.HostingServer, error) {
	var server models.HostingServer
	err := s.db.Get(&server, `SELECT * FROM hosting_servers WHERE id = $1`, id)
	return &server, err
}

// CreateServer creates a new hosting server with encrypted password
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

// TestServerConnection verifies SSH connectivity to a server
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

// ToggleServer enables or disables a server
func (s *HostingService) ToggleServer(id string, active bool) error {
	_, err := s.db.Exec(`UPDATE hosting_servers SET is_active = $1 WHERE id = $2`, active, id)
	return err
}

// DeleteServer removes a server if it has no active accounts
func (s *HostingService) DeleteServer(id string) error {
	var count int
	s.db.Get(&count, `SELECT COUNT(*) FROM hosting_accounts WHERE server_id = $1`, id)
	if count > 0 {
		return errors.New("cannot delete server with active accounts")
	}
	_, err := s.db.Exec(`DELETE FROM hosting_servers WHERE id = $1`, id)
	return err
}

// ========== Packages ==========

// ListPackages returns all hosting packages ordered by sort_order and name
func (s *HostingService) ListPackages() ([]models.HostingPackage, error) {
	var packages []models.HostingPackage
	err := s.db.Select(&packages, `SELECT * FROM hosting_packages ORDER BY sort_order, name`)
	return packages, err
}

// ListActivePackages returns only active packages for purchase display
func (s *HostingService) ListActivePackages() ([]models.HostingPackage, error) {
	var packages []models.HostingPackage
	err := s.db.Select(&packages, `
		SELECT * FROM hosting_packages
		WHERE is_active = TRUE
		ORDER BY sort_order, price_monthly
	`)
	return packages, err
}

// GetPackage retrieves a package by ID
func (s *HostingService) GetPackage(id string) (*models.HostingPackage, error) {
	var pkg models.HostingPackage
	err := s.db.Get(&pkg, `SELECT * FROM hosting_packages WHERE id = $1`, id)
	return &pkg, err
}

// CreatePackage creates a new hosting package
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

// UpdatePackage updates an existing hosting package
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

// TogglePackage enables or disables a package for new purchases
func (s *HostingService) TogglePackage(id string, active bool) error {
	_, err := s.db.Exec(`UPDATE hosting_packages SET is_active = $1 WHERE id = $2`, active, id)
	return err
}

// ========== Accounts ==========

// ListAccountsByUser returns all hosting accounts for a user
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

// ListAllAccounts returns all hosting accounts (admin view)
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

// GetAccount retrieves an account by ID with joined data
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

// GetAccountPassword decrypts and returns the HestiaCP password for an account
func (s *HostingService) GetAccountPassword(id string) (string, error) {
	var encrypted string
	err := s.db.Get(&encrypted, `SELECT hestia_password_encrypted FROM hosting_accounts WHERE id = $1`, id)
	if err != nil {
		return "", err
	}
	return crypto.Decrypt(encrypted)
}

// PurchaseHosting creates a new hosting account on the least loaded server
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

// SuspendAccount suspends a hosting account on HestiaCP
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

// UnsuspendAccount reactivates a suspended hosting account
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

// GetUserBalance returns a user's current balance
func (s *HostingService) GetUserBalance(userID string) float64 {
	var balance float64
	s.db.Get(&balance, `SELECT COALESCE(balance, 0) FROM users WHERE id = $1`, userID)
	return balance
}

// TopUpBalance adds funds to a user's balance
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

// DeductBalance removes funds from a user's balance
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

// GetBalanceTransactions returns recent balance transactions for a user
func (s *HostingService) GetBalanceTransactions(userID string) ([]models.BalanceTransaction, error) {
	var txs []models.BalanceTransaction
	err := s.db.Select(&txs, `
		SELECT * FROM balance_transactions WHERE user_id = $1 ORDER BY created_at DESC LIMIT 50
	`, userID)
	return txs, err
}

// ========== Domains ==========

// ListDomains returns all domains for an account
func (s *HostingService) ListDomains(accountID string) ([]models.HostingDomain, error) {
	var domains []models.HostingDomain
	err := s.db.Select(&domains, `SELECT * FROM hosting_domains WHERE account_id = $1 ORDER BY created_at`, accountID)
	return domains, err
}

// AddDomain adds a domain to an account on HestiaCP
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

// DeleteDomain removes a domain from an account
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

// EnableSSL enables Let's Encrypt SSL for a domain
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

// GetDomainSettings retrieves antibot settings for a domain
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

// UpdateDomainSettings updates antibot settings for a domain
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

// GetDomainSettingsByHost retrieves antibot settings by domain hostname (for callback API)
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

// ========== Email Accounts ==========

// ListEmails returns all email accounts for a hosting account
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

// AddEmail creates an email account on HestiaCP
func (s *HostingService) AddEmail(accountID string, input models.AddEmailInput) (*models.HostingEmail, error) {
	account, err := s.GetAccount(accountID)
	if err != nil {
		return nil, err
	}

	// Get domain
	var domain models.HostingDomain
	err = s.db.Get(&domain, `SELECT * FROM hosting_domains WHERE id = $1 AND account_id = $2`, input.DomainID, accountID)
	if err != nil {
		return nil, errors.New("domain not found")
	}

	server, _ := s.GetServer(account.ServerID)
	serverPass, _ := crypto.Decrypt(server.PasswordEncrypted)
	client := hestia.NewClient(server.Hostname, server.Port, server.Username, serverPass)
	defer client.Close()

	// Ensure mail domain exists
	client.AddMailDomain(account.HestiaUsername, domain.Domain)

	// Create mail account
	if err := client.AddMailAccount(account.HestiaUsername, domain.Domain, input.Account, input.Password); err != nil {
		return nil, errors.New("failed to create email: " + err.Error())
	}

	email := &models.HostingEmail{
		ID:        s.generateID(),
		AccountID: accountID,
		DomainID:  input.DomainID,
		Email:     input.Account + "@" + domain.Domain,
		QuotaMB:   500, // Default quota
		CreatedAt: time.Now(),
	}

	_, err = s.db.NamedExec(`
		INSERT INTO hosting_emails (id, account_id, domain_id, email, quota_mb, created_at)
		VALUES (:id, :account_id, :domain_id, :email, :quota_mb, :created_at)
	`, email)
	return email, err
}

// DeleteEmail removes an email account from HestiaCP
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

	// Extract local part from email
	localPart := email.Email[:len(email.Email)-len(email.DomainName)-1]
	client.DeleteMailAccount(account.HestiaUsername, email.DomainName, localPart)

	_, err = s.db.Exec(`DELETE FROM hosting_emails WHERE id = $1`, emailID)
	return err
}

// ========== Databases ==========

// ListDatabases returns all databases for a hosting account
func (s *HostingService) ListDatabases(accountID string) ([]models.HostingDatabase, error) {
	var dbs []models.HostingDatabase
	err := s.db.Select(&dbs, `SELECT * FROM hosting_databases WHERE account_id = $1 ORDER BY created_at`, accountID)
	return dbs, err
}

// AddDatabase creates a database on HestiaCP
func (s *HostingService) AddDatabase(accountID string, input models.AddDatabaseInput) (*models.HostingDatabase, error) {
	account, err := s.GetAccount(accountID)
	if err != nil {
		return nil, err
	}

	encPassword, err := crypto.Encrypt(input.Password)
	if err != nil {
		return nil, err
	}

	server, _ := s.GetServer(account.ServerID)
	serverPass, _ := crypto.Decrypt(server.PasswordEncrypted)
	client := hestia.NewClient(server.Hostname, server.Port, server.Username, serverPass)
	defer client.Close()

	if err := client.AddDatabase(account.HestiaUsername, input.DBName, input.DBUser, input.Password); err != nil {
		return nil, errors.New("failed to create database: " + err.Error())
	}

	db := &models.HostingDatabase{
		ID:                  s.generateID(),
		AccountID:           accountID,
		DBName:              input.DBName,
		DBUser:              input.DBUser,
		DBPasswordEncrypted: encPassword,
		CreatedAt:           time.Now(),
	}

	_, err = s.db.NamedExec(`
		INSERT INTO hosting_databases (id, account_id, db_name, db_user, db_password_encrypted, created_at)
		VALUES (:id, :account_id, :db_name, :db_user, :db_password_encrypted, :created_at)
	`, db)
	return db, err
}

// GetDatabasePassword decrypts and returns a database password
func (s *HostingService) GetDatabasePassword(accountID, databaseID string) (string, error) {
	var encrypted string
	err := s.db.Get(&encrypted, `SELECT db_password_encrypted FROM hosting_databases WHERE id = $1 AND account_id = $2`, databaseID, accountID)
	if err != nil {
		return "", err
	}
	return crypto.Decrypt(encrypted)
}

// DeleteDatabase removes a database from HestiaCP
func (s *HostingService) DeleteDatabase(accountID, databaseID string) error {
	var db models.HostingDatabase
	err := s.db.Get(&db, `SELECT * FROM hosting_databases WHERE id = $1 AND account_id = $2`, databaseID, accountID)
	if err != nil {
		return errors.New("database not found")
	}

	account, _ := s.GetAccount(accountID)
	server, _ := s.GetServer(account.ServerID)
	serverPass, _ := crypto.Decrypt(server.PasswordEncrypted)
	client := hestia.NewClient(server.Hostname, server.Port, server.Username, serverPass)
	defer client.Close()

	client.DeleteDatabase(account.HestiaUsername, db.DBName)

	_, err = s.db.Exec(`DELETE FROM hosting_databases WHERE id = $1`, databaseID)
	return err
}

// ========== FTP Accounts ==========

// ListFTP returns all FTP accounts for a hosting account
func (s *HostingService) ListFTP(accountID string) ([]models.HostingFTP, error) {
	var ftps []models.HostingFTP
	err := s.db.Select(&ftps, `SELECT * FROM hosting_ftp WHERE account_id = $1 ORDER BY created_at`, accountID)
	return ftps, err
}

// AddFTP creates an FTP account on HestiaCP
func (s *HostingService) AddFTP(accountID string, input models.AddFTPInput) (*models.HostingFTP, error) {
	account, err := s.GetAccount(accountID)
	if err != nil {
		return nil, err
	}

	encPassword, err := crypto.Encrypt(input.Password)
	if err != nil {
		return nil, err
	}

	path := input.Path
	if path == "" {
		path = "/"
	}

	server, _ := s.GetServer(account.ServerID)
	serverPass, _ := crypto.Decrypt(server.PasswordEncrypted)
	client := hestia.NewClient(server.Hostname, server.Port, server.Username, serverPass)
	defer client.Close()

	if err := client.AddFTP(account.HestiaUsername, input.Username, input.Password, path); err != nil {
		return nil, errors.New("failed to create FTP account: " + err.Error())
	}

	ftp := &models.HostingFTP{
		ID:                s.generateID(),
		AccountID:         accountID,
		Username:          input.Username,
		PasswordEncrypted: encPassword,
		Path:              path,
		CreatedAt:         time.Now(),
	}

	_, err = s.db.NamedExec(`
		INSERT INTO hosting_ftp (id, account_id, username, password_encrypted, path, created_at)
		VALUES (:id, :account_id, :username, :password_encrypted, :path, :created_at)
	`, ftp)
	return ftp, err
}

// GetFTPPassword decrypts and returns an FTP password
func (s *HostingService) GetFTPPassword(accountID, ftpID string) (string, error) {
	var encrypted string
	err := s.db.Get(&encrypted, `SELECT password_encrypted FROM hosting_ftp WHERE id = $1 AND account_id = $2`, ftpID, accountID)
	if err != nil {
		return "", err
	}
	return crypto.Decrypt(encrypted)
}

// DeleteFTP removes an FTP account from HestiaCP
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
