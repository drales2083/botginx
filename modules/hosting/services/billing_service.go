// Package services contains business logic for the hosting module.
package services

import (
	"log"
	"time"

	"github.com/botginx/botginx/modules/hosting/models"
	"github.com/jmoiron/sqlx"
)

// BillingService handles monthly recurring charges for hosting accounts
type BillingService struct {
	db      *sqlx.DB
	hosting *HostingService
}

// NewBillingService creates a new billing service instance
func NewBillingService(db *sqlx.DB, hosting *HostingService) *BillingService {
	return &BillingService{db: db, hosting: hosting}
}

// ProcessMonthlyBilling processes all accounts due for billing
// This should be called periodically (e.g., hourly via cron)
func (s *BillingService) ProcessMonthlyBilling() {
	// Find accounts due for billing: active accounts where next_billing_at <= now
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

	if len(accounts) == 0 {
		return
	}

	var renewed, suspended int
	for _, account := range accounts {
		if s.processAccount(account) {
			renewed++
		} else {
			suspended++
		}
	}

	log.Printf("Billing: processed %d accounts (%d renewed, %d suspended)", len(accounts), renewed, suspended)
}

// processAccount processes a single account for billing
// Returns true if renewed, false if suspended
func (s *BillingService) processAccount(account models.HostingAccount) bool {
	// Determine price: use custom_price if set, otherwise package price
	var price float64
	if account.CustomPrice != nil {
		price = *account.CustomPrice
	} else {
		price = account.PackagePrice
	}

	// Skip if price is zero (free accounts)
	if price <= 0 {
		nextBilling := time.Now().AddDate(0, 0, 30)
		s.db.Exec(`UPDATE hosting_accounts SET next_billing_at = $1, updated_at = $2 WHERE id = $3`,
			nextBilling, time.Now(), account.ID)
		log.Printf("Billing: extended free account %s", account.ID)
		return true
	}

	// Check user balance
	balance := s.hosting.GetUserBalance(account.UserID)

	if balance >= price {
		// Deduct and extend billing period
		err := s.hosting.DeductBalance(account.UserID, price, "Monthly hosting renewal")
		if err != nil {
			log.Printf("Billing: failed to deduct for account %s: %v", account.ID, err)
			return false
		}

		nextBilling := time.Now().AddDate(0, 0, 30)
		_, err = s.db.Exec(`UPDATE hosting_accounts SET next_billing_at = $1, updated_at = $2 WHERE id = $3`,
			nextBilling, time.Now(), account.ID)
		if err != nil {
			log.Printf("Billing: failed to update next_billing_at for account %s: %v", account.ID, err)
		}
		log.Printf("Billing: renewed account %s (deducted $%.2f)", account.ID, price)
		return true
	}

	// Insufficient balance - suspend the account
	err := s.hosting.SuspendAccount(account.ID)
	if err != nil {
		log.Printf("Billing: failed to suspend account %s: %v", account.ID, err)
		return false
	}
	log.Printf("Billing: suspended account %s (insufficient balance: $%.2f < $%.2f)", account.ID, balance, price)
	return false
}

// GetAccountsDueSoon returns accounts that will be due within the specified duration
// Useful for sending payment reminders
func (s *BillingService) GetAccountsDueSoon(within time.Duration) ([]models.HostingAccount, error) {
	var accounts []models.HostingAccount
	deadline := time.Now().Add(within)
	err := s.db.Select(&accounts, `
		SELECT a.*, s.name as server_name, p.name as package_name, p.price_monthly as package_price,
			u.email as user_email
		FROM hosting_accounts a
		LEFT JOIN hosting_servers s ON s.id = a.server_id
		LEFT JOIN hosting_packages p ON p.id = a.package_id
		LEFT JOIN users u ON u.id = a.user_id
		WHERE a.status = 'active' AND a.next_billing_at > $1 AND a.next_billing_at <= $2
		ORDER BY a.next_billing_at
	`, time.Now(), deadline)
	return accounts, err
}
