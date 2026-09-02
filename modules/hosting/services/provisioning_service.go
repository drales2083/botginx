// Package services provides business logic for the hosting module.
package services

import (
	"errors"
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
		return errors.New("account not found")
	}

	// Skip if already provisioned
	if account.ServerID != nil && account.Status == string(models.AccountStatusActive) {
		return nil
	}

	// Get the domain for this account
	var domain string
	err = p.db.Get(&domain, `SELECT domain FROM hosting_domains WHERE account_id = $1 LIMIT 1`, accountID)
	if err != nil {
		return errors.New("domain not found for account")
	}

	// Pick an available server
	server, err := p.lb.PickServer()
	if err != nil {
		return err
	}

	// Decrypt server credentials
	serverPassword, err := crypto.Decrypt(server.PasswordEncrypted)
	if err != nil {
		return errors.New("failed to decrypt server credentials")
	}

	// Decrypt panel password
	panelPassword, err := crypto.Decrypt(account.PanelPasswordEncrypted)
	if err != nil {
		return errors.New("failed to decrypt panel credentials")
	}

	// Connect to CloudPanel
	client := cloudpanel.NewClient(server.Hostname, server.Port, server.Username, serverPassword)
	defer client.Close()

	if err := client.Connect(); err != nil {
		log.Printf("[provisioning] failed to connect to server %s: %v", server.Name, err)
		return errors.New("failed to connect to hosting server")
	}

	// Create the site on CloudPanel
	// This creates: site user, vhost, PHP-FPM pool
	log.Printf("[provisioning] creating site %s on server %s with user %s", domain, server.Name, account.PanelUsername)
	if err := client.AddSite(domain, account.PanelUsername, cloudpanel.PHP82); err != nil {
		log.Printf("[provisioning] failed to create site: %v", err)
		return errors.New("failed to create site on server: " + err.Error())
	}

	// Set the user password (AddSite creates user but we need to set our password)
	cmd := "clpctl user:reset:password --userName=" + shellEscape(account.PanelUsername) + " --password=" + shellEscape(panelPassword)
	if _, err := client.Execute(cmd); err != nil {
		log.Printf("[provisioning] warning: failed to set user password: %v", err)
		// Don't fail - site is created, password can be reset manually
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
			// Mark account with provisioning error
			p.db.Exec(`
				UPDATE hosting_accounts
				SET status = 'pending', updated_at = $1
				WHERE id = $2
			`, time.Now(), accountID)
		}
	}()
}

// GetProvisioningStatus returns the current provisioning status for an account
func (p *ProvisioningService) GetProvisioningStatus(accountID string) (status string, serverIP string, err error) {
	var account struct {
		Status   string  `db:"status"`
		ServerID *string `db:"server_id"`
	}
	err = p.db.Get(&account, `SELECT status, server_id FROM hosting_accounts WHERE id = $1`, accountID)
	if err != nil {
		return "", "", err
	}

	if account.ServerID != nil {
		var hostname string
		p.db.Get(&hostname, `SELECT hostname FROM hosting_servers WHERE id = $1`, *account.ServerID)
		serverIP = hostname
	}

	return account.Status, serverIP, nil
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
