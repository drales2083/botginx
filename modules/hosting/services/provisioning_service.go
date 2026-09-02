// Package services provides business logic for the hosting module.
package services

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/botginx/botginx/modules/hosting/models"
	"github.com/botginx/botginx/pkg/cloudpanel"
	"github.com/botginx/botginx/pkg/crypto"
	"github.com/jmoiron/sqlx"
)

// ProvisioningService handles automatic provisioning on CloudPanel servers
type ProvisioningService struct {
	db *sqlx.DB
	lb *LoadBalancer
}

// NewProvisioningService creates a new provisioning service
func NewProvisioningService(db *sqlx.DB) *ProvisioningService {
	return &ProvisioningService{
		db: db,
		lb: NewLoadBalancer(db),
	}
}

// ProvisionAccount automatically provisions a hosting account on CloudPanel.
// This should be called after PurchaseHosting creates the pending account.
// It picks an available server, creates the site, and updates the account status.
func (p *ProvisioningService) ProvisionAccount(accountID string) error {
	log.Printf("[provisioning] starting provisioning for account %s", accountID)

	// Get account details
	var account struct {
		ID                     string  `db:"id"`
		PanelUsername          string  `db:"panel_username"`
		PanelPasswordEncrypted string  `db:"panel_password_encrypted"`
		ServerID               *string `db:"server_id"`
		Status                 string  `db:"status"`
	}
	err := p.db.Get(&account, `SELECT id, panel_username, panel_password_encrypted, server_id, status FROM hosting_accounts WHERE id = $1`, accountID)
	if err != nil {
		log.Printf("[provisioning] account not found: %v", err)
		return errors.New("account not found: " + err.Error())
	}

	// Skip if already provisioned
	if account.ServerID != nil && account.Status == string(models.AccountStatusActive) {
		log.Printf("[provisioning] account %s already provisioned, skipping", accountID)
		return nil
	}

	// Get the domain for this account
	var domain string
	err = p.db.Get(&domain, `SELECT domain FROM hosting_domains WHERE account_id = $1 LIMIT 1`, accountID)
	if err != nil {
		log.Printf("[provisioning] domain not found for account: %v", err)
		return errors.New("domain not found for account: " + err.Error())
	}
	log.Printf("[provisioning] provisioning domain: %s", domain)

	// Pick an available server
	server, err := p.lb.PickServer()
	if err != nil {
		log.Printf("[provisioning] no server available: %v", err)
		return err
	}
	log.Printf("[provisioning] selected server: %s (%s)", server.Name, server.Hostname)

	// Decrypt server credentials
	serverPassword, err := crypto.Decrypt(server.PasswordEncrypted)
	if err != nil {
		log.Printf("[provisioning] failed to decrypt server credentials: %v", err)
		return errors.New("failed to decrypt server credentials - check encryption key")
	}

	// Decrypt panel password
	panelPassword, err := crypto.Decrypt(account.PanelPasswordEncrypted)
	if err != nil {
		log.Printf("[provisioning] failed to decrypt panel credentials: %v", err)
		return errors.New("failed to decrypt panel credentials - check encryption key")
	}
	log.Printf("[provisioning] panel password decrypted, length: %d chars", len(panelPassword))

	// Connect to CloudPanel
	log.Printf("[provisioning] connecting to %s:%d as %s", server.Hostname, server.Port, server.Username)
	client := cloudpanel.NewClient(server.Hostname, server.Port, server.Username, serverPassword)
	defer client.Close()

	if err := client.Connect(); err != nil {
		log.Printf("[provisioning] SSH connection failed to %s: %v", server.Name, err)
		return fmt.Errorf("SSH connection failed to %s: %v", server.Hostname, err)
	}
	log.Printf("[provisioning] SSH connected successfully")

	// Create the site on CloudPanel
	// This creates: site user, vhost, PHP-FPM pool
	log.Printf("[provisioning] creating site %s on server %s with user %s", domain, server.Name, account.PanelUsername)
	if err := client.AddSite(domain, account.PanelUsername, panelPassword, cloudpanel.PHP82); err != nil {
		log.Printf("[provisioning] failed to create site: %v", err)
		return errors.New("failed to create site on server: " + err.Error())
	}

	// Create CloudPanel panel user so they can login to web interface
	email := "user@" + domain
	log.Printf("[provisioning] creating panel user %s for site %s", account.PanelUsername, domain)
	if err := client.AddPanelUser(account.PanelUsername, panelPassword, email, domain); err != nil {
		log.Printf("[provisioning] warning: failed to create panel user: %v (site was created successfully)", err)
		// Don't fail - site is created, panel user can be added manually
	}

	// Update account with server link and active status
	_, err = p.db.Exec(`
		UPDATE hosting_accounts
		SET server_id = $1, status = $2, updated_at = $3
		WHERE id = $4
	`, server.ID, models.AccountStatusActive, time.Now(), accountID)
	if err != nil {
		log.Printf("[provisioning] warning: site created but failed to update account: %v", err)
		return errors.New("site created but failed to update account status")
	}

	// Increment server account count
	if err := p.lb.IncrementServerCount(server.ID); err != nil {
		log.Printf("[provisioning] warning: failed to increment server count: %v", err)
	}

	// Enable SSL in background (don't block on this - DNS may not be ready)
	go p.tryEnableSSL(client, domain, server.Name)

	log.Printf("[provisioning] successfully provisioned %s on %s", domain, server.Name)
	return nil
}

// tryEnableSSL attempts to enable Let's Encrypt SSL (non-blocking)
func (p *ProvisioningService) tryEnableSSL(client *cloudpanel.Client, domain, serverName string) {
	// Wait a bit for DNS propagation
	time.Sleep(30 * time.Second)

	if err := client.AddLetsEncrypt(domain); err != nil {
		log.Printf("[provisioning] SSL setup deferred for %s: %v (user can enable later)", domain, err)
	} else {
		log.Printf("[provisioning] SSL enabled for %s", domain)
		// Update domain ssl_enabled flag
		p.db.Exec(`UPDATE hosting_domains SET ssl_enabled = true WHERE domain = $1`, domain)
	}
}

// ProvisionAccountAsync provisions an account in a background goroutine.
// Use this from the purchase handler for non-blocking provisioning.
func (p *ProvisioningService) ProvisionAccountAsync(accountID string) {
	go func() {
		if err := p.ProvisionAccount(accountID); err != nil {
			log.Printf("[provisioning] async provisioning failed for %s: %v", accountID, err)
			// Store the error so user can see why it failed
			errMsg := err.Error()
			p.db.Exec(`
				UPDATE hosting_accounts
				SET status = 'pending', provisioning_error = $1, updated_at = $2
				WHERE id = $3
			`, errMsg, time.Now(), accountID)
		}
	}()
}

// GetProvisioningStatus returns the current provisioning status for an account
func (p *ProvisioningService) GetProvisioningStatus(accountID string) (status string, serverIP string, provisioningError string, err error) {
	var account struct {
		Status            string  `db:"status"`
		ServerID          *string `db:"server_id"`
		ProvisioningError *string `db:"provisioning_error"`
	}
	err = p.db.Get(&account, `SELECT status, server_id, provisioning_error FROM hosting_accounts WHERE id = $1`, accountID)
	if err != nil {
		return "", "", "", err
	}

	if account.ServerID != nil {
		var hostname string
		p.db.Get(&hostname, `SELECT hostname FROM hosting_servers WHERE id = $1`, *account.ServerID)
		serverIP = hostname
	}

	if account.ProvisioningError != nil {
		provisioningError = *account.ProvisioningError
	}

	return account.Status, serverIP, provisioningError, nil
}

// shellEscape escapes a string for safe use in shell commands
func shellEscape(s string) string {
	return "'" + escapeQuotes(s) + "'"
}

func escapeQuotes(s string) string {
	result := ""
	for _, c := range s {
		if c == '\'' {
			result += "'\"'\"'"
		} else {
			result += string(c)
		}
	}
	return result
}
