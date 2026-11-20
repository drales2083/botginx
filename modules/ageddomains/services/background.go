package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
)

// AlertNotifier interface for sending admin alerts
type AlertNotifier interface {
	SendAdminAlert(message string) error
}

type BackgroundJobs struct {
	db               *sqlx.DB
	inventoryService *InventoryService
	balanceService   *BalanceService
	providerRegistry *ProviderRegistry
	alertNotifier    AlertNotifier
	stopCh           chan struct{}
}

func NewBackgroundJobs(db *sqlx.DB, is *InventoryService, bs *BalanceService, pr *ProviderRegistry) *BackgroundJobs {
	return &BackgroundJobs{
		db:               db,
		inventoryService: is,
		balanceService:   bs,
		providerRegistry: pr,
		stopCh:           make(chan struct{}),
	}
}

// SetAlertNotifier sets the notifier for admin alerts
func (b *BackgroundJobs) SetAlertNotifier(n AlertNotifier) {
	b.alertNotifier = n
}

func (b *BackgroundJobs) Start() {
	go b.runInventoryRefresh()
	go b.runLockCleanup()
	go b.runStalePurchaseCheck()
	log.Printf("[ageddomains] Background jobs started")
}

func (b *BackgroundJobs) Stop() {
	close(b.stopCh)
	log.Printf("[ageddomains] Background jobs stopped")
}

// runInventoryRefresh fetches fresh domain listings from providers
func (b *BackgroundJobs) runInventoryRefresh() {
	// Initial delay for startup
	time.Sleep(30 * time.Second)

	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()

	// Run immediately on start
	b.refreshAllInventories()

	for {
		select {
		case <-ticker.C:
			b.refreshAllInventories()
		case <-b.stopCh:
			return
		}
	}
}

func (b *BackgroundJobs) refreshAllInventories() {
	ctx := context.Background()
	providers, err := b.providerRegistry.ListActive(ctx)
	if err != nil {
		log.Printf("[ageddomains] Failed to list providers: %v", err)
		return
	}

	for _, p := range providers {
		log.Printf("[ageddomains] Refreshing inventory from provider %s", p.DisplayName)

		client := b.providerRegistry.GetClient(p.ID)
		if client == nil {
			log.Printf("[ageddomains] No client for provider %s", p.DisplayName)
			continue
		}

		fetchStart := time.Now()
		domains, err := client.ListAgedDomains(ctx)
		if err != nil {
			log.Printf("[ageddomains] Provider %s fetch failed: %v", p.DisplayName, err)
			continue
		}

		updated, err := b.inventoryService.UpdateInventory(ctx, p.ID, domains, p.MarkupPercent)
		if err != nil {
			log.Printf("[ageddomains] Provider %s inventory update failed: %v", p.DisplayName, err)
			continue
		}

		// Prune domains not seen in this fetch
		pruned, _ := b.inventoryService.PruneStale(ctx, p.ID, fetchStart)

		log.Printf("[ageddomains] Provider %s: %d domains updated, %d pruned", p.DisplayName, updated, pruned)
	}
}

// runLockCleanup releases expired balance locks
func (b *BackgroundJobs) runLockCleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			released, err := b.balanceService.CleanupExpiredLocks(context.Background())
			if err != nil {
				log.Printf("[ageddomains] Lock cleanup error: %v", err)
			} else if released > 0 {
				log.Printf("[ageddomains] Released %d expired balance locks", released)
			}
		case <-b.stopCh:
			return
		}
	}
}

// runStalePurchaseCheck handles purchases stuck in intermediate states
func (b *BackgroundJobs) runStalePurchaseCheck() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			b.checkStalePurchases()
		case <-b.stopCh:
			return
		}
	}
}

func (b *BackgroundJobs) checkStalePurchases() {
	ctx := context.Background()

	// Find purchases stuck > 30 minutes in purchasing/provisioning state
	var stale []struct {
		ID     string `db:"id"`
		Status string `db:"status"`
		Domain string `db:"domain_name"`
	}

	err := b.db.SelectContext(ctx, &stale, `
		SELECT id, status, domain_name FROM aged_domain_purchases
		WHERE status IN ('pending', 'purchasing', 'provisioning')
		AND created_at < NOW() - INTERVAL '30 minutes'
	`)
	if err != nil {
		log.Printf("[ageddomains] Stale purchase check error: %v", err)
		return
	}

	for _, p := range stale {
		msg := fmt.Sprintf("⚠️ Stale purchase detected: %s\nDomain: %s\nStatus: %s\nStuck for >30 minutes", p.ID[:8], p.Domain, p.Status)
		log.Printf("[WARNING] %s", msg)

		// Send alert via telegram if configured
		if b.alertNotifier != nil {
			if err := b.alertNotifier.SendAdminAlert(msg); err != nil {
				log.Printf("[ageddomains] Failed to send alert: %v", err)
			}
		}
	}
}

// RefreshInventoryNow triggers an immediate inventory refresh (for admin use)
func (b *BackgroundJobs) RefreshInventoryNow() {
	go b.refreshAllInventories()
}
