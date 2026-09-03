package services

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/botginx/botginx/modules/hosting/models"
	"github.com/botginx/botginx/pkg/cloudpanel"
	"github.com/botginx/botginx/pkg/crypto"
	"github.com/jmoiron/sqlx"
)

var domainRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)

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

// GetServerAntibotPassword returns the decrypted antibot dashboard password
func (s *HostingService) GetServerAntibotPassword(serverID string) (string, error) {
	var encPass string
	err := s.db.Get(&encPass, `SELECT antibot_password_encrypted FROM hosting_servers WHERE id = $1`, serverID)
	if err != nil || encPass == "" {
		return "", err
	}
	return crypto.Decrypt(encPass)
}

// CreateServer creates a new hosting server
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
	serverType := input.Type
	if serverType == "" {
		serverType = models.ServerTypeCloudPanel
	}

	// Encrypt antibot password if provided
	var encAntibotPass string
	if input.AntibotPassword != "" {
		encAntibotPass, _ = crypto.Encrypt(input.AntibotPassword)
	}

	server := &models.HostingServer{
		ID:                       s.generateID(),
		Name:                     input.Name,
		Type:                     serverType,
		Hostname:                 input.Hostname,
		PanelURL:                 input.PanelURL,
		Port:                     port,
		Username:                 input.Username,
		PasswordEncrypted:        encPass,
		MaxAccounts:              maxAccounts,
		IsActive:                 true,
		CreatedAt:                time.Now(),
		AntibotDashboardURL:      input.AntibotDashboardURL,
		AntibotPasswordEncrypted: encAntibotPass,
	}

	_, err = s.db.NamedExec(`
		INSERT INTO hosting_servers (id, name, type, hostname, panel_url, port, username, password_encrypted, max_accounts, is_active, created_at, antibot_dashboard_url, antibot_password_encrypted)
		VALUES (:id, :name, :type, :hostname, :panel_url, :port, :username, :password_encrypted, :max_accounts, :is_active, :created_at, :antibot_dashboard_url, :antibot_password_encrypted)
	`, server)
	return server, err
}

// ToggleServer enables or disables a server
func (s *HostingService) ToggleServer(id string, active bool) error {
	_, err := s.db.Exec(`UPDATE hosting_servers SET is_active = $1 WHERE id = $2`, active, id)
	return err
}

// UpdateServer updates an existing server
func (s *HostingService) UpdateServer(id string, input models.UpdateServerInput) error {
	// Build update query dynamically
	updates := []string{}
	args := []interface{}{}
	argIdx := 1

	if input.Name != "" {
		updates = append(updates, fmt.Sprintf("name = $%d", argIdx))
		args = append(args, input.Name)
		argIdx++
	}
	if input.Type != "" {
		updates = append(updates, fmt.Sprintf("type = $%d", argIdx))
		args = append(args, input.Type)
		argIdx++
	}
	if input.Hostname != "" {
		updates = append(updates, fmt.Sprintf("hostname = $%d", argIdx))
		args = append(args, input.Hostname)
		argIdx++
	}
	if input.PanelURL != "" {
		updates = append(updates, fmt.Sprintf("panel_url = $%d", argIdx))
		args = append(args, input.PanelURL)
		argIdx++
	}
	if input.Port > 0 {
		updates = append(updates, fmt.Sprintf("port = $%d", argIdx))
		args = append(args, input.Port)
		argIdx++
	}
	if input.Username != "" {
		updates = append(updates, fmt.Sprintf("username = $%d", argIdx))
		args = append(args, input.Username)
		argIdx++
	}
	if input.Password != "" {
		encPass, err := crypto.Encrypt(input.Password)
		if err != nil {
			return err
		}
		updates = append(updates, fmt.Sprintf("password_encrypted = $%d", argIdx))
		args = append(args, encPass)
		argIdx++
	}
	if input.MaxAccounts > 0 {
		updates = append(updates, fmt.Sprintf("max_accounts = $%d", argIdx))
		args = append(args, input.MaxAccounts)
		argIdx++
	}
	if input.AntibotDashboardURL != "" {
		updates = append(updates, fmt.Sprintf("antibot_dashboard_url = $%d", argIdx))
		args = append(args, input.AntibotDashboardURL)
		argIdx++
	}
	if input.AntibotPassword != "" {
		encPass, err := crypto.Encrypt(input.AntibotPassword)
		if err != nil {
			return err
		}
		updates = append(updates, fmt.Sprintf("antibot_password_encrypted = $%d", argIdx))
		args = append(args, encPass)
		argIdx++
	}

	if len(updates) == 0 {
		return nil
	}

	query := fmt.Sprintf("UPDATE hosting_servers SET %s WHERE id = $%d", strings.Join(updates, ", "), argIdx)
	args = append(args, id)

	_, err := s.db.Exec(query, args...)
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
		SELECT a.*,
			COALESCE(s.name, '') as server_name,
			COALESCE(s.hostname, '') as server_hostname,
			COALESCE(s.panel_url, '') as panel_url,
			COALESCE(p.name, '') as package_name,
			COALESCE(p.price_monthly, 0) as package_price,
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
		SELECT a.*,
			COALESCE(s.name, '') as server_name,
			COALESCE(s.hostname, '') as server_hostname,
			COALESCE(s.panel_url, '') as panel_url,
			COALESCE(p.name, '') as package_name,
			COALESCE(p.price_monthly, 0) as package_price,
			COALESCE(u.email, '') as user_email,
			(SELECT COUNT(*) FROM hosting_domains WHERE account_id = a.id) as domain_count
		FROM hosting_accounts a
		LEFT JOIN hosting_servers s ON s.id = a.server_id
		LEFT JOIN hosting_packages p ON p.id = a.package_id
		LEFT JOIN users u ON u.id = a.user_id
		ORDER BY a.created_at DESC
	`)
	return accounts, err
}

// ListPendingAccounts returns accounts waiting for admin to link
func (s *HostingService) ListPendingAccounts() ([]models.HostingAccount, error) {
	var accounts []models.HostingAccount
	err := s.db.Select(&accounts, `
		SELECT a.*, p.name as package_name, p.price_monthly as package_price, u.email as user_email
		FROM hosting_accounts a
		LEFT JOIN hosting_packages p ON p.id = a.package_id
		LEFT JOIN users u ON u.id = a.user_id
		WHERE a.status = 'pending'
		ORDER BY a.created_at ASC
	`)
	return accounts, err
}

// GetAccount retrieves an account by ID with joined data
func (s *HostingService) GetAccount(id string) (*models.HostingAccount, error) {
	var account models.HostingAccount
	err := s.db.Get(&account, `
		SELECT a.*,
			COALESCE(s.name, '') as server_name,
			COALESCE(s.hostname, '') as server_hostname,
			COALESCE(s.panel_url, '') as panel_url,
			COALESCE(p.name, '') as package_name,
			COALESCE(p.price_monthly, 0) as package_price,
			(SELECT COUNT(*) FROM hosting_domains WHERE account_id = a.id) as domain_count
		FROM hosting_accounts a
		LEFT JOIN hosting_servers s ON s.id = a.server_id
		LEFT JOIN hosting_packages p ON p.id = a.package_id
		WHERE a.id = $1
	`, id)
	return &account, err
}

// GetPanelCredentials returns the decrypted panel username and password for user login
func (s *HostingService) GetPanelCredentials(accountID string) (username, password string, err error) {
	var account struct {
		Username          string `db:"panel_username"`
		PasswordEncrypted string `db:"panel_password_encrypted"`
	}
	err = s.db.Get(&account, `SELECT panel_username, panel_password_encrypted FROM hosting_accounts WHERE id = $1`, accountID)
	if err != nil {
		return "", "", err
	}
	if account.PasswordEncrypted == "" {
		return "", "", errors.New("credentials not available")
	}
	password, err = crypto.Decrypt(account.PasswordEncrypted)
	return account.Username, password, err
}

// GetAccountWithCredentials returns account with decrypted credentials (admin view)
func (s *HostingService) GetAccountWithCredentials(accountID string) (*models.HostingAccount, string, error) {
	account, err := s.GetAccount(accountID)
	if err != nil {
		return nil, "", err
	}

	// Decrypt password
	password := ""
	if account.PanelPasswordEncrypted != "" {
		password, _ = crypto.Decrypt(account.PanelPasswordEncrypted)
	}

	return account, password, nil
}

// PurchaseHosting creates a pending hosting account with auto-generated credentials
func (s *HostingService) PurchaseHosting(userID string, packageID string, domain string) (*models.HostingAccount, error) {
	// Validate domain format
	domain = strings.ToLower(strings.TrimSpace(domain))
	if !domainRegex.MatchString(domain) {
		return nil, errors.New("invalid domain format")
	}

	// Get package
	pkg, err := s.GetPackage(packageID)
	if err != nil {
		return nil, errors.New("package not found")
	}
	if !pkg.IsActive {
		return nil, errors.New("package not available")
	}

	// Generate username from domain (first 8 chars of domain + random suffix)
	username := s.generateUsername(domain)

	// Generate random password (16 chars)
	password := s.generatePassword()

	// Encrypt password
	encPassword, err := crypto.Encrypt(password)
	if err != nil {
		return nil, err
	}

	// Create pending account with auto-generated credentials
	nextBilling := time.Now().AddDate(0, 0, 30)
	account := &models.HostingAccount{
		ID:                     s.generateID(),
		UserID:                 userID,
		PackageID:              &packageID,
		PanelUsername:          username,
		PanelPasswordEncrypted: encPassword,
		Status:                 models.AccountStatusPending,
		NextBillingAt:          &nextBilling,
		CreatedAt:              time.Now(),
		UpdatedAt:              time.Now(),
	}

	// Use transaction to ensure atomicity
	tx, err := s.db.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Check and lock user balance within transaction to prevent race condition
	var balance float64
	err = tx.Get(&balance, `SELECT COALESCE(balance, 0) FROM users WHERE id = $1 FOR UPDATE`, userID)
	if err != nil {
		return nil, errors.New("user not found")
	}
	if balance < pkg.PriceMonthly {
		return nil, errors.New("insufficient balance")
	}

	// Check domain not already used (inside transaction to prevent race)
	var exists int
	tx.Get(&exists, `SELECT COUNT(*) FROM hosting_domains WHERE domain = $1`, domain)
	if exists > 0 {
		return nil, errors.New("domain already registered")
	}

	_, err = tx.NamedExec(`
		INSERT INTO hosting_accounts (id, user_id, package_id, panel_username, panel_password_encrypted, status, next_billing_at, created_at, updated_at)
		VALUES (:id, :user_id, :package_id, :panel_username, :panel_password_encrypted, :status, :next_billing_at, :created_at, :updated_at)
	`, account)
	if err != nil {
		return nil, err
	}

	// Create the domain record
	domainID := s.generateID()
	_, err = tx.Exec(`
		INSERT INTO hosting_domains (id, account_id, domain, created_at)
		VALUES ($1, $2, $3, $4)
	`, domainID, account.ID, domain, time.Now())
	if err != nil {
		return nil, errors.New("domain already registered")
	}

	// Create default antibot settings for the domain
	settingsID := s.generateID()
	_, err = tx.Exec(`
		INSERT INTO hosting_domain_settings (id, domain_id, country_mode, country_list, device_mode, device_list,
			block_bots, block_tor, block_proxy, block_datacenter, block_headless, min_behavior_score, redirect_on_block, updated_at)
		VALUES ($1, $2, 'all', '[]', 'all', '[]', true, true, true, true, true, 0, 'https://www.google.com', $3)
	`, settingsID, domainID, time.Now())
	if err != nil {
		return nil, errors.New("failed to create domain settings")
	}

	// Deduct balance
	_, err = tx.Exec(`UPDATE users SET balance = COALESCE(balance, 0) - $1 WHERE id = $2`, pkg.PriceMonthly, userID)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(`
		INSERT INTO balance_transactions (id, user_id, amount, type, description, created_at)
		VALUES ($1, $2, $3, 'deduct', $4, $5)
	`, s.generateID(), userID, -pkg.PriceMonthly, "Hosting purchase: "+pkg.Name, time.Now())
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return account, nil
}

// generateUsername creates a panel username from domain
func (s *HostingService) generateUsername(domain string) string {
	// Remove TLD and take first part
	parts := strings.Split(domain, ".")
	base := parts[0]

	// Keep only alphanumeric, max 8 chars
	clean := ""
	for _, c := range base {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			clean += string(c)
		}
		if len(clean) >= 8 {
			break
		}
	}
	if len(clean) < 3 {
		clean = "user"
	}

	// Add random suffix
	suffix := make([]byte, 4)
	rand.Read(suffix)
	return clean + hex.EncodeToString(suffix)[:4]
}

// generatePassword creates a secure random password
func (s *HostingService) generatePassword() string {
	const chars = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789!@#$%"
	b := make([]byte, 16)
	rand.Read(b)
	password := make([]byte, 16)
	for i := range password {
		password[i] = chars[int(b[i])%len(chars)]
	}
	return string(password)
}

// LinkAccount links a pending account to a server and activates it (admin action)
func (s *HostingService) LinkAccount(accountID string, input models.LinkAccountInput) error {
	// Verify account exists and is pending
	account, err := s.GetAccount(accountID)
	if err != nil {
		return errors.New("account not found")
	}
	if account.Status != models.AccountStatusPending {
		return errors.New("account is not pending")
	}

	// Update account with server and activate
	_, err = s.db.Exec(`
		UPDATE hosting_accounts SET
			server_id = $2, status = $3, updated_at = $4
		WHERE id = $1
	`, accountID, input.ServerID, models.AccountStatusActive, time.Now())
	if err != nil {
		return err
	}

	// Increment server count
	s.lb.IncrementServerCount(input.ServerID)

	return nil
}

// SuspendAccount suspends a hosting account
func (s *HostingService) SuspendAccount(id string) error {
	_, err := s.db.Exec(`UPDATE hosting_accounts SET status = $1, updated_at = $2 WHERE id = $3`,
		models.AccountStatusSuspended, time.Now(), id)
	return err
}

// UnsuspendAccount reactivates a suspended hosting account
func (s *HostingService) UnsuspendAccount(id string) error {
	_, err := s.db.Exec(`UPDATE hosting_accounts SET status = $1, updated_at = $2 WHERE id = $3`,
		models.AccountStatusActive, time.Now(), id)
	return err
}

// DeleteAccount deletes a hosting account and cleans up on CloudPanel
func (s *HostingService) DeleteAccount(id string) error {
	fmt.Printf("[hosting] DeleteAccount starting for %s\n", id)

	// Get account with server info
	var account struct {
		ID                     string  `db:"id"`
		PanelUsername          string  `db:"panel_username"`
		ServerID               *string `db:"server_id"`
		ServerHostname         string  `db:"server_hostname"`
		ServerPort             int     `db:"server_port"`
		ServerUsername         string  `db:"server_username"`
		ServerPasswordEnc      string  `db:"server_password_encrypted"`
	}
	err := s.db.Get(&account, `
		SELECT a.id, a.panel_username, a.server_id,
			COALESCE(s.hostname, '') as server_hostname,
			COALESCE(s.port, 22) as server_port,
			COALESCE(s.username, '') as server_username,
			COALESCE(s.password_encrypted, '') as server_password_encrypted
		FROM hosting_accounts a
		LEFT JOIN hosting_servers s ON s.id = a.server_id
		WHERE a.id = $1
	`, id)
	if err != nil {
		return errors.New("account not found")
	}

	// Get domains for this account
	var domains []string
	s.db.Select(&domains, `SELECT domain FROM hosting_domains WHERE account_id = $1`, id)

	// If linked to a server, clean up on CloudPanel
	if account.ServerID != nil && account.ServerHostname != "" && account.ServerPasswordEnc != "" {
		serverPassword, err := crypto.Decrypt(account.ServerPasswordEnc)
		if err == nil {
			client := cloudpanel.NewClient(account.ServerHostname, account.ServerPort, account.ServerUsername, serverPassword)
			defer client.Close()

			if err := client.Connect(); err == nil {
				// Delete each domain/site on CloudPanel
				for _, domain := range domains {
					if err := client.DeleteSite(domain); err != nil {
						// Log but don't fail - site might not exist
						fmt.Printf("[hosting] warning: failed to delete site %s on CloudPanel: %v\n", domain, err)
					}
				}
				// Delete the user on CloudPanel
				if err := client.DeleteUser(account.PanelUsername); err != nil {
					fmt.Printf("[hosting] warning: failed to delete user %s on CloudPanel: %v\n", account.PanelUsername, err)
				}
			}
		}
	}

	// Decrement server account count if linked
	if account.ServerID != nil {
		s.db.Exec(`UPDATE hosting_servers SET current_accounts = GREATEST(current_accounts - 1, 0) WHERE id = $1`, *account.ServerID)
	}

	// Delete domains from our database
	_, err = s.db.Exec(`DELETE FROM hosting_domains WHERE account_id = $1`, id)
	if err != nil {
		fmt.Printf("[hosting] DeleteAccount failed to delete domains for %s: %v\n", id, err)
		return err
	}
	fmt.Printf("[hosting] DeleteAccount deleted domains for %s\n", id)

	// Delete the account from our database
	result, err := s.db.Exec(`DELETE FROM hosting_accounts WHERE id = $1`, id)
	if err != nil {
		fmt.Printf("[hosting] DeleteAccount failed to delete account %s: %v\n", id, err)
		return err
	}
	rows, _ := result.RowsAffected()
	fmt.Printf("[hosting] DeleteAccount deleted account %s (rows affected: %d)\n", id, rows)
	return nil
}

// ========== Balance ==========

// GetUserIDByEmail looks up a user by their email address
func (s *HostingService) GetUserIDByEmail(email string) (string, error) {
	var userID string
	err := s.db.Get(&userID, `SELECT id FROM users WHERE email = $1`, email)
	return userID, err
}

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

// ClearProvisioningError clears the provisioning error for retry
func (s *HostingService) ClearProvisioningError(accountID string) error {
	_, err := s.db.Exec(`UPDATE hosting_accounts SET provisioning_error = NULL, updated_at = NOW() WHERE id = $1`, accountID)
	return err
}

// ========== Domains (for antibot settings tracking) ==========

// ListDomains returns all domains for an account
func (s *HostingService) ListDomains(accountID string) ([]models.HostingDomain, error) {
	var domains []models.HostingDomain
	err := s.db.Select(&domains, `SELECT * FROM hosting_domains WHERE account_id = $1 ORDER BY created_at`, accountID)
	return domains, err
}

// AddDomain adds a domain to an account (admin action for antibot tracking)
func (s *HostingService) AddDomain(accountID, domain string) (*models.HostingDomain, error) {
	d := &models.HostingDomain{
		ID:        s.generateID(),
		AccountID: accountID,
		Domain:    domain,
		CreatedAt: time.Now(),
	}

	_, err := s.db.NamedExec(`
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

// DeleteDomain removes a domain from an account (database only - admin use)
func (s *HostingService) DeleteDomain(accountID, domainID string) error {
	_, err := s.db.Exec(`DELETE FROM hosting_domains WHERE id = $1 AND account_id = $2`, domainID, accountID)
	return err
}

// ProvisionDomain creates a domain on CloudPanel and adds to database
func (s *HostingService) ProvisionDomain(accountID, domain string) (*models.HostingDomain, error) {
	// Get account with server details
	account, err := s.GetAccount(accountID)
	if err != nil {
		return nil, errors.New("account not found")
	}

	if account.ServerID == nil {
		return nil, errors.New("account not provisioned to a server")
	}

	// Get server credentials
	var server struct {
		Hostname          string `db:"hostname"`
		Port              int    `db:"port"`
		Username          string `db:"username"`
		PasswordEncrypted string `db:"password_encrypted"`
	}
	err = s.db.Get(&server, `SELECT hostname, port, username, password_encrypted FROM hosting_servers WHERE id = $1`, *account.ServerID)
	if err != nil {
		return nil, errors.New("server not found")
	}

	serverPassword, err := crypto.Decrypt(server.PasswordEncrypted)
	if err != nil {
		return nil, errors.New("failed to decrypt server credentials")
	}

	// Get panel credentials for the site user
	panelPassword, err := crypto.Decrypt(account.PanelPasswordEncrypted)
	if err != nil {
		return nil, errors.New("failed to decrypt panel credentials")
	}

	// Connect to CloudPanel and create site
	client := cloudpanel.NewClient(server.Hostname, server.Port, server.Username, serverPassword)
	defer client.Close()

	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("SSH connection failed: %v", err)
	}

	// Create site on CloudPanel
	// Uses "Botection" vhost template if available (routes traffic through antibot)
	if err := client.AddSite(domain, account.PanelUsername, panelPassword, cloudpanel.PHP82); err != nil {
		return nil, fmt.Errorf("failed to create site: %v", err)
	}

	// Add to database
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

// DeleteDomainWithCloudPanel removes a domain from CloudPanel and database
func (s *HostingService) DeleteDomainWithCloudPanel(accountID, domainID string) error {
	// Get domain
	domain, err := s.GetDomain(domainID)
	if err != nil || domain.AccountID != accountID {
		return errors.New("domain not found")
	}

	// Get account with server details
	account, err := s.GetAccount(accountID)
	if err != nil {
		return errors.New("account not found")
	}

	if account.ServerID != nil {
		// Get server credentials
		var server struct {
			Hostname          string `db:"hostname"`
			Port              int    `db:"port"`
			Username          string `db:"username"`
			PasswordEncrypted string `db:"password_encrypted"`
		}
		err = s.db.Get(&server, `SELECT hostname, port, username, password_encrypted FROM hosting_servers WHERE id = $1`, *account.ServerID)
		if err == nil {
			serverPassword, _ := crypto.Decrypt(server.PasswordEncrypted)
			if serverPassword != "" {
				client := cloudpanel.NewClient(server.Hostname, server.Port, server.Username, serverPassword)
				defer client.Close()
				if client.Connect() == nil {
					client.DeleteSite(domain.Domain)
				}
			}
		}
	}

	// Delete from database
	_, err = s.db.Exec(`DELETE FROM hosting_domains WHERE id = $1 AND account_id = $2`, domainID, accountID)
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

// UpdateDomainSettings updates antibot settings for a domain and pushes to server
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
	if err != nil {
		return err
	}

	// Push settings to server in background
	go s.pushDomainSettingsToServer(domainID)
	return nil
}

// pushDomainSettingsToServer pushes domain settings to the CloudPanel server
func (s *HostingService) pushDomainSettingsToServer(domainID string) {
	// Get domain info
	domain, err := s.GetDomain(domainID)
	if err != nil {
		return
	}

	// Get account to find server
	account, err := s.GetAccount(domain.AccountID)
	if err != nil || account.ServerID == nil {
		return
	}

	// Get server SSH credentials
	server, err := s.GetServer(*account.ServerID)
	if err != nil {
		return
	}

	// Get settings
	settings, err := s.GetDomainSettings(domainID)
	if err != nil {
		return
	}

	// Decrypt server password
	password, err := crypto.Decrypt(server.PasswordEncrypted)
	if err != nil {
		return
	}

	// Build settings JSON for botection
	settingsData := map[string]interface{}{
		"link_id":            domainID,
		"user_id":            account.UserID,
		"host":               domain.Domain,
		"block_bots":         settings.BlockBots,
		"block_tor":          settings.BlockTor,
		"block_proxy":        settings.BlockProxy,
		"block_datacenter":   settings.BlockDatacenter,
		"block_headless":     settings.BlockHeadless,
		"country_mode":       settings.CountryMode,
		"country_list":       settings.CountryList,
		"device_mode":        settings.DeviceMode,
		"device_list":        settings.DeviceList,
		"min_behavior_score": settings.MinBehaviorScore,
		"redirect_on_block":  settings.RedirectOnBlock,
		"updated_at":         time.Now().UTC().Format(time.RFC3339),
	}
	jsonData, _ := json.MarshalIndent(settingsData, "", "  ")

	// Create CloudPanel client (SSH)
	client := cloudpanel.NewClient(server.Hostname, server.Port, server.Username, password)
	if err := client.Connect(); err != nil {
		return
	}
	defer client.Close()

	// Write settings file using cat heredoc
	remotePath := fmt.Sprintf("/etc/botection/links/%s.json", domainID)
	client.Execute("mkdir -p /etc/botection/links")

	// Write file via echo + base64 decode (safe for any content)
	encoded := base64.StdEncoding.EncodeToString(jsonData)
	writeCmd := fmt.Sprintf("echo '%s' | base64 -d > %s", encoded, remotePath)
	client.Execute(writeCmd)
}

// GetDomainStats fetches traffic stats from botection on the CloudPanel server
func (s *HostingService) GetDomainStats(domainID string) (*models.BotectionStats, error) {
	// Get domain info
	domain, err := s.GetDomain(domainID)
	if err != nil {
		return nil, err
	}

	// Get account to find server
	account, err := s.GetAccount(domain.AccountID)
	if err != nil || account.ServerID == nil {
		return nil, errors.New("account or server not found")
	}

	// Get server SSH credentials
	server, err := s.GetServer(*account.ServerID)
	if err != nil {
		return nil, err
	}

	// Decrypt server password
	password, err := crypto.Decrypt(server.PasswordEncrypted)
	if err != nil {
		return nil, err
	}

	// Create CloudPanel client (SSH)
	client := cloudpanel.NewClient(server.Hostname, server.Port, server.Username, password)
	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	// Fetch stats from botection API via curl
	output, err := client.Execute("curl -s http://127.0.0.1:8080/api/stats 2>/dev/null")
	if err != nil || output == "" {
		return &models.BotectionStats{}, nil
	}

	var stats models.BotectionStats
	if err := json.Unmarshal([]byte(output), &stats); err != nil {
		return &models.BotectionStats{}, nil
	}

	// Filter recent blocks to only show this domain
	filteredBlocks := make([]models.BlockEvent, 0)
	for _, block := range stats.RecentBlocks {
		if block.Host == domain.Domain {
			filteredBlocks = append(filteredBlocks, block)
		}
	}
	stats.RecentBlocks = filteredBlocks

	return &stats, nil
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

// ========== Analytics (for Antibot Dashboard) ==========

// AnalyticsSummary contains overview stats
type AnalyticsSummary struct {
	TotalVisits  int     `json:"totalVisits"`
	BotBlocks    int     `json:"botBlocks"`
	HumanVisits  int     `json:"humanVisits"`
	BlockRate    float64 `json:"blockRate"`
}

// Analytics contains detailed traffic data
type Analytics struct {
	Summary      AnalyticsSummary `json:"summary"`
	ByCountry    []CountryStats   `json:"byCountry"`
	ByDevice     []DeviceStats    `json:"byDevice"`
	ByDay        []DayStats       `json:"byDay"`
	RecentBlocks []BlockEvent     `json:"recentBlocks"`
}

// CountryStats represents visits by country
type CountryStats struct {
	Country string `json:"country" db:"country"`
	Visits  int    `json:"visits" db:"visits"`
	Blocks  int    `json:"blocks" db:"blocks"`
}

// DeviceStats represents visits by device
type DeviceStats struct {
	Device string `json:"device" db:"device"`
	Visits int    `json:"visits" db:"visits"`
}

// DayStats represents daily visits
type DayStats struct {
	Date   string `json:"date" db:"date"`
	Visits int    `json:"visits" db:"visits"`
	Blocks int    `json:"blocks" db:"blocks"`
}

// BlockEvent represents a blocked request
type BlockEvent struct {
	Time    string `json:"time" db:"time"`
	IP      string `json:"ip" db:"ip"`
	Country string `json:"country" db:"country"`
	Reason  string `json:"reason" db:"reason"`
	Domain  string `json:"domain" db:"domain"`
}

// GetAnalyticsSummary returns summary stats for an account's domains
func (s *HostingService) GetAnalyticsSummary(accountID string) *AnalyticsSummary {
	summary := &AnalyticsSummary{}

	// Get domains for this account
	domains, _ := s.ListDomains(accountID)
	if len(domains) == 0 {
		return summary
	}

	// Query visits from hosting_visits table (last 7 days)
	query := `
		SELECT
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE is_bot = true OR blocked = true) as blocked
		FROM hosting_visits
		WHERE account_id = $1
		AND created_at > NOW() - INTERVAL '7 days'
	`
	var stats struct {
		Total   int `db:"total"`
		Blocked int `db:"blocked"`
	}
	s.db.Get(&stats, query, accountID)

	summary.TotalVisits = stats.Total
	summary.BotBlocks = stats.Blocked
	summary.HumanVisits = stats.Total - stats.Blocked
	if stats.Total > 0 {
		summary.BlockRate = float64(stats.Blocked) / float64(stats.Total) * 100
	}

	return summary
}

// GetAnalytics returns detailed analytics for an account's domains
func (s *HostingService) GetAnalytics(accountID, period string) *Analytics {
	analytics := &Analytics{
		ByCountry:    []CountryStats{},
		ByDevice:     []DeviceStats{},
		ByDay:        []DayStats{},
		RecentBlocks: []BlockEvent{},
	}

	// Determine time interval
	interval := "7 days"
	switch period {
	case "24h":
		interval = "24 hours"
	case "30d":
		interval = "30 days"
	}

	// Summary
	var summaryStats struct {
		Total   int `db:"total"`
		Blocked int `db:"blocked"`
	}
	s.db.Get(&summaryStats, `
		SELECT
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE is_bot = true OR blocked = true) as blocked
		FROM hosting_visits
		WHERE account_id = $1
		AND created_at > NOW() - INTERVAL '`+interval+`'
	`, accountID)

	analytics.Summary.TotalVisits = summaryStats.Total
	analytics.Summary.BotBlocks = summaryStats.Blocked
	analytics.Summary.HumanVisits = summaryStats.Total - summaryStats.Blocked
	if summaryStats.Total > 0 {
		analytics.Summary.BlockRate = float64(summaryStats.Blocked) / float64(summaryStats.Total) * 100
	}

	// By country
	s.db.Select(&analytics.ByCountry, `
		SELECT
			COALESCE(country, 'Unknown') as country,
			COUNT(*) as visits,
			COUNT(*) FILTER (WHERE is_bot = true OR blocked = true) as blocks
		FROM hosting_visits
		WHERE account_id = $1
		AND created_at > NOW() - INTERVAL '`+interval+`'
		GROUP BY country
		ORDER BY visits DESC
		LIMIT 10
	`, accountID)

	// By device
	s.db.Select(&analytics.ByDevice, `
		SELECT
			COALESCE(device, 'unknown') as device,
			COUNT(*) as visits
		FROM hosting_visits
		WHERE account_id = $1
		AND created_at > NOW() - INTERVAL '`+interval+`'
		GROUP BY device
		ORDER BY visits DESC
	`, accountID)

	// Recent blocks - join with hosting_domains to get domain name
	s.db.Select(&analytics.RecentBlocks, `
		SELECT
			TO_CHAR(v.created_at, 'YYYY-MM-DD HH24:MI') as time,
			v.ip,
			COALESCE(v.country, 'Unknown') as country,
			COALESCE(v.block_reason, 'bot') as reason,
			d.domain
		FROM hosting_visits v
		JOIN hosting_domains d ON d.id = v.domain_id
		WHERE v.account_id = $1
		AND (v.is_bot = true OR v.blocked = true)
		AND v.created_at > NOW() - INTERVAL '`+interval+`'
		ORDER BY v.created_at DESC
		LIMIT 20
	`, accountID)

	return analytics
}

// GetDomain returns a domain by ID
func (s *HostingService) GetDomain(id string) (*models.HostingDomain, error) {
	var domain models.HostingDomain
	err := s.db.Get(&domain, `SELECT * FROM hosting_domains WHERE id = $1`, id)
	if err != nil {
		return nil, errors.New("domain not found")
	}
	return &domain, nil
}

// UpdateDomainStatus updates the DNS/SSL status of a domain
func (s *HostingService) UpdateDomainStatus(domainID string, dnsVerified bool, setupStatus string, sslError *string) error {
	_, err := s.db.Exec(`
		UPDATE hosting_domains 
		SET dns_verified = $1, setup_status = $2, ssl_error = $3, updated_at = $4
		WHERE id = $5
	`, dnsVerified, setupStatus, sslError, time.Now(), domainID)
	return err
}

// SetDomainSSLEnabled marks a domain as having SSL enabled
func (s *HostingService) SetDomainSSLEnabled(domainID string, enabled bool) error {
	_, err := s.db.Exec(`
		UPDATE hosting_domains SET ssl_enabled = $1, updated_at = $2 WHERE id = $3
	`, enabled, time.Now(), domainID)
	return err
}

// GenerateSSLForDomain generates SSL certificate via CloudPanel
func (s *HostingService) GenerateSSLForDomain(account *models.HostingAccount, domain *models.HostingDomain) error {
	if account.ServerID == nil {
		return errors.New("account not linked to server")
	}

	// Get server credentials
	var server struct {
		Hostname         string `db:"hostname"`
		Port             int    `db:"port"`
		Username         string `db:"username"`
		PasswordEnc      string `db:"password_encrypted"`
	}
	err := s.db.Get(&server, `SELECT hostname, port, username, password_encrypted FROM hosting_servers WHERE id = $1`, *account.ServerID)
	if err != nil {
		return errors.New("server not found")
	}

	password, err := crypto.Decrypt(server.PasswordEnc)
	if err != nil {
		return errors.New("failed to decrypt server credentials")
	}

	// Connect to CloudPanel
	client := cloudpanel.NewClient(server.Hostname, server.Port, server.Username, password)
	defer client.Close()

	if err := client.Connect(); err != nil {
		return fmt.Errorf("SSH connection failed: %v", err)
	}

	// Run Let's Encrypt
	if err := client.AddLetsEncrypt(domain.Domain); err != nil {
		return fmt.Errorf("SSL generation failed: %v", err)
	}

	return nil
}

// FixAccountStatus corrects account status when it's stuck at wrong value
func (s *HostingService) FixAccountStatus(accountID string, status models.AccountStatus) {
	s.db.Exec(`UPDATE hosting_accounts SET status = $1, updated_at = NOW() WHERE id = $2`, status, accountID)
}

// ListPendingDomains returns all domains that need DNS verification or SSL setup
func (s *HostingService) ListPendingDomains() ([]models.HostingDomain, error) {
	var domains []models.HostingDomain
	err := s.db.Select(&domains, `
		SELECT d.* FROM hosting_domains d
		JOIN hosting_accounts a ON a.id = d.account_id
		WHERE a.server_id IS NOT NULL
		AND a.status = 'active'
		AND (d.dns_verified = FALSE OR d.ssl_enabled = FALSE OR d.setup_status != 'active')
		ORDER BY d.created_at DESC
	`)
	return domains, err
}

// ========== Hosting Visits (Analytics) ==========

// HostingDomainInfo contains domain and account IDs for visit recording
type HostingDomainInfo struct {
	DomainID  string `db:"domain_id"`
	AccountID string `db:"account_id"`
	UserID    string `db:"user_id"`
}

// GetDomainByHost looks up a hosting domain by its hostname.
// Returns domain ID, account ID, and owner user ID if found.
func (s *HostingService) GetDomainByHost(host string) (*HostingDomainInfo, error) {
	// Normalize host (remove www. prefix if present)
	host = strings.TrimPrefix(strings.ToLower(host), "www.")

	var info HostingDomainInfo
	err := s.db.Get(&info, `
		SELECT d.id as domain_id, d.account_id, a.user_id
		FROM hosting_domains d
		JOIN hosting_accounts a ON a.id = d.account_id
		WHERE LOWER(d.domain) = $1 OR LOWER(d.domain) = $2
		LIMIT 1
	`, host, "www."+host)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

// RecordHostingVisit stores a visit in the hosting_visits table
func (s *HostingService) RecordHostingVisit(visit *models.HostingVisit) error {
	if visit.ID == "" {
		visit.ID = s.generateID()
	}
	if visit.CreatedAt.IsZero() {
		visit.CreatedAt = time.Now()
	}

	_, err := s.db.Exec(`
		INSERT INTO hosting_visits (
			id, domain_id, account_id,
			ip, path, method, country, city, asn, asn_org,
			device, browser, os, user_agent, language, timezone, screen_resolution,
			referrer, referrer_domain,
			utm_source, utm_medium, utm_campaign, utm_term, utm_content,
			is_bot, bot_score, behavior_score, automation_tool,
			is_headless, is_tor, is_proxy, is_datacenter, fingerprint,
			action, blocked, block_reason, session_id, created_at
		) VALUES (
			$1, $2, $3,
			$4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17,
			$18, $19,
			$20, $21, $22, $23, $24,
			$25, $26, $27, $28,
			$29, $30, $31, $32, $33,
			$34, $35, $36, $37, $38
		)
	`,
		visit.ID, visit.DomainID, visit.AccountID,
		visit.IP, visit.Path, visit.Method, visit.Country, visit.City, visit.ASN, visit.ASNOrg,
		visit.Device, visit.Browser, visit.OS, visit.UserAgent, visit.Language, visit.Timezone, visit.ScreenResolution,
		visit.Referrer, visit.ReferrerDomain,
		visit.UTMSource, visit.UTMMedium, visit.UTMCampaign, visit.UTMTerm, visit.UTMContent,
		visit.IsBot, visit.BotScore, visit.BehaviorScore, visit.AutomationTool,
		visit.IsHeadless, visit.IsTor, visit.IsProxy, visit.IsDatacenter, visit.Fingerprint,
		visit.Action, visit.Blocked, visit.BlockReason, visit.SessionID, visit.CreatedAt,
	)
	return err
}

// GetDomainVisitStats returns visit statistics for a domain
func (s *HostingService) GetDomainVisitStats(domainID string, since time.Time) (total, blocked, bots int, err error) {
	var stats struct {
		Total   int `db:"total"`
		Blocked int `db:"blocked"`
		Bots    int `db:"bots"`
	}
	err = s.db.Get(&stats, `
		SELECT
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE blocked = true) as blocked,
			COUNT(*) FILTER (WHERE is_bot = true) as bots
		FROM hosting_visits
		WHERE domain_id = $1 AND created_at >= $2
	`, domainID, since)
	return stats.Total, stats.Blocked, stats.Bots, err
}

// GetAccountVisitStats returns visit statistics for all domains in an account
func (s *HostingService) GetAccountVisitStats(accountID string, since time.Time) (total, blocked, bots int, err error) {
	var stats struct {
		Total   int `db:"total"`
		Blocked int `db:"blocked"`
		Bots    int `db:"bots"`
	}
	err = s.db.Get(&stats, `
		SELECT
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE blocked = true) as blocked,
			COUNT(*) FILTER (WHERE is_bot = true) as bots
		FROM hosting_visits
		WHERE account_id = $1 AND created_at >= $2
	`, accountID, since)
	return stats.Total, stats.Blocked, stats.Bots, err
}
