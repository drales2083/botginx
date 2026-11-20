package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/botginx/botginx/modules/ageddomains/models"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

var (
	ErrPurchaseNotFound  = errors.New("purchase not found")
	ErrDuplicatePurchase = errors.New("duplicate purchase attempt")
)

// DomainCreator interface for creating domain records in the domains module
type DomainCreator interface {
	CreateDomainFromPurchase(ctx context.Context, userID, domainName, verifyToken, purchaseID string) (string, error)
}

type PurchaseService struct {
	db               *sqlx.DB
	balanceService   *BalanceService
	inventoryService *InventoryService
	providerRegistry *ProviderRegistry
	domainCreator    DomainCreator
}

func NewPurchaseService(db *sqlx.DB, bs *BalanceService, is *InventoryService, pr *ProviderRegistry) *PurchaseService {
	return &PurchaseService{
		db:               db,
		balanceService:   bs,
		inventoryService: is,
		providerRegistry: pr,
	}
}

// SetDomainCreator sets the domain creator for creating domain records
func (s *PurchaseService) SetDomainCreator(dc DomainCreator) {
	s.domainCreator = dc
}

// PurchaseDomain initiates the full purchase flow
func (s *PurchaseService) PurchaseDomain(ctx context.Context, userID, inventoryID string) (*models.Purchase, error) {
	// 1. Get domain from inventory (mark unavailable to prevent race)
	domain, err := s.inventoryService.ReserveForPurchase(ctx, inventoryID)
	if err != nil {
		return nil, fmt.Errorf("reserve domain: %w", err)
	}

	// 2. Generate idempotency key
	idempotencyKey := GenerateIdempotencyKey(userID, domain.DomainName)

	// 3. Check for existing purchase (idempotent replay)
	existing, _ := s.GetByIdempotency(ctx, idempotencyKey)
	if existing != nil {
		return existing, nil
	}

	// 4. Lock funds
	lock, err := s.balanceService.LockFunds(ctx, userID, domain.SalePrice, idempotencyKey,
		fmt.Sprintf("domain_purchase:%s", domain.DomainName))
	if err != nil {
		s.inventoryService.MarkAvailable(ctx, inventoryID) // Release reservation
		return nil, fmt.Errorf("lock funds: %w", err)
	}

	// 5. Create purchase record
	purchase := &models.Purchase{
		ID:             uuid.New().String(),
		UserID:         userID,
		InventoryID:    inventoryID,
		DomainName:     domain.DomainName,
		ProviderID:     domain.ProviderID,
		ProviderPrice:  domain.RegistrationPrice,
		SalePrice:      domain.SalePrice,
		BalanceLockID:  lock.ID,
		IdempotencyKey: idempotencyKey,
		Status:         models.StatusPending,
		CreatedAt:      time.Now(),
	}

	if err := s.create(ctx, purchase); err != nil {
		s.balanceService.ReleaseLock(ctx, lock.ID)
		s.inventoryService.MarkAvailable(ctx, inventoryID)
		return nil, fmt.Errorf("create purchase: %w", err)
	}

	s.recordEvent(ctx, purchase.ID, "lock_acquired", "Funds reserved", "")

	// 6. Execute purchase asynchronously
	go s.executePurchase(context.Background(), purchase)

	return purchase, nil
}

func (s *PurchaseService) executePurchase(ctx context.Context, purchase *models.Purchase) {
	defer func() {
		if r := recover(); r != nil {
			s.handleFailure(ctx, purchase, fmt.Errorf("panic: %v", r))
		}
	}()

	// Phase 1: Purchase from provider
	s.updateStatus(ctx, purchase.ID, models.StatusPurchasing)
	s.recordEvent(ctx, purchase.ID, "provider_started", "Contacting domain registrar", "")

	client := s.providerRegistry.GetClient(purchase.ProviderID)
	if client == nil {
		s.handleFailure(ctx, purchase, fmt.Errorf("provider not configured"))
		return
	}

	orderID, err := client.RegisterDomain(ctx, purchase.DomainName, purchase.ProviderPrice)
	if err != nil {
		s.handleFailure(ctx, purchase, fmt.Errorf("provider registration: %w", err))
		return
	}

	purchase.ProviderOrderID = orderID
	s.updateProviderOrder(ctx, purchase.ID, orderID)
	s.recordEvent(ctx, purchase.ID, "provider_completed", "Domain registered successfully", "")

	// Phase 2: Commit balance (POINT OF NO RETURN)
	if err := s.balanceService.CommitLock(ctx, purchase.BalanceLockID); err != nil {
		// CRITICAL: Domain purchased but balance not debited
		log.Printf("[CRITICAL] Purchase %s: domain bought but commit failed: %v", purchase.ID, err)
		s.updateStatus(ctx, purchase.ID, models.StatusCommitFailed)
		s.recordEvent(ctx, purchase.ID, "commit_failed", "CRITICAL: Manual intervention required", err.Error())
		return
	}
	s.recordEvent(ctx, purchase.ID, "balance_committed", "Payment processed", "")

	// Phase 3: Provision in our system
	s.updateStatus(ctx, purchase.ID, models.StatusProvisioning)
	s.recordEvent(ctx, purchase.ID, "dns_started", "Configuring DNS records", "")

	domainID, err := s.provisionDomain(ctx, purchase)
	if err != nil {
		// Domain paid for but provisioning failed - needs manual fix
		s.updateStatus(ctx, purchase.ID, models.StatusProvisionFailed)
		s.recordEvent(ctx, purchase.ID, "provision_failed", err.Error(), "")
		log.Printf("[WARNING] Purchase %s: provisioning failed: %v", purchase.ID, err)
		return
	}

	// Phase 4: Complete
	s.completePurchase(ctx, purchase.ID, domainID)
	s.recordEvent(ctx, purchase.ID, "completed", "Domain ready to use", "")
	log.Printf("[ageddomains] Purchase %s completed: %s", purchase.ID, purchase.DomainName)
}

func (s *PurchaseService) handleFailure(ctx context.Context, purchase *models.Purchase, err error) {
	log.Printf("[ageddomains] Purchase %s failed: %v", purchase.ID, err)

	// Release balance lock
	if releaseErr := s.balanceService.ReleaseLock(ctx, purchase.BalanceLockID); releaseErr != nil {
		log.Printf("[WARNING] Failed to release lock %s: %v", purchase.BalanceLockID, releaseErr)
	}

	// Mark inventory available again
	s.inventoryService.MarkAvailable(ctx, purchase.InventoryID)

	// Update purchase status
	s.updateStatus(ctx, purchase.ID, models.StatusFailed)
	s.updateError(ctx, purchase.ID, err.Error())
	s.recordEvent(ctx, purchase.ID, "failed", err.Error(), "")
}

func (s *PurchaseService) provisionDomain(ctx context.Context, purchase *models.Purchase) (string, error) {
	// Generate verify token for domain
	verifyToken := uuid.New().String()[:8]

	// Create domain record and configure DNS via domains module
	if s.domainCreator != nil {
		domainID, err := s.domainCreator.CreateDomainFromPurchase(ctx, purchase.UserID, purchase.DomainName, verifyToken, purchase.ID)
		if err != nil {
			return "", fmt.Errorf("create domain record: %w", err)
		}
		s.recordEvent(ctx, purchase.ID, "dns_configured", "DNS records created, waiting for propagation", "")
		return domainID, nil
	}

	// Fallback: Configure DNS directly via provider if domains module not available
	client := s.providerRegistry.GetClient(purchase.ProviderID)
	if client != nil {
		// Get deploy server IP
		var serverIP string
		s.db.Get(&serverIP, `SELECT ip FROM servers WHERE is_deploy_vps = true LIMIT 1`)
		if serverIP == "" {
			return "", fmt.Errorf("no deploy server configured")
		}

		// Setup DNS via provider
		if err := client.SetNameservers(ctx, purchase.DomainName, []string{serverIP}); err != nil {
			log.Printf("[ageddomains] Warning: nameserver setup failed: %v", err)
		}
	}

	// Generate fallback domain ID
	domainID := uuid.New().String()
	s.recordEvent(ctx, purchase.ID, "dns_configured", "Waiting for DNS propagation", "")

	return domainID, nil
}

// Database operations

func (s *PurchaseService) create(ctx context.Context, p *models.Purchase) error {
	_, err := s.db.NamedExecContext(ctx, `
		INSERT INTO aged_domain_purchases
		(id, user_id, inventory_id, domain_name, provider_id, provider_price, sale_price, balance_lock_id, idempotency_key, status, created_at)
		VALUES (:id, :user_id, :inventory_id, :domain_name, :provider_id, :provider_price, :sale_price, :balance_lock_id, :idempotency_key, :status, :created_at)
	`, p)
	return err
}

func (s *PurchaseService) updateStatus(ctx context.Context, id, status string) {
	s.db.ExecContext(ctx, `UPDATE aged_domain_purchases SET status = $1 WHERE id = $2`, status, id)
}

func (s *PurchaseService) updateProviderOrder(ctx context.Context, id, orderID string) {
	s.db.ExecContext(ctx, `UPDATE aged_domain_purchases SET provider_order_id = $1 WHERE id = $2`, orderID, id)
}

func (s *PurchaseService) updateError(ctx context.Context, id, errMsg string) {
	s.db.ExecContext(ctx, `UPDATE aged_domain_purchases SET error_message = $1 WHERE id = $2`, errMsg, id)
}

func (s *PurchaseService) completePurchase(ctx context.Context, id, domainID string) {
	s.db.ExecContext(ctx, `
		UPDATE aged_domain_purchases
		SET status = $1, domain_id = $2, completed_at = $3
		WHERE id = $4
	`, models.StatusCompleted, domainID, time.Now(), id)
}

func (s *PurchaseService) recordEvent(ctx context.Context, purchaseID, eventType, message, details string) {
	s.db.ExecContext(ctx, `
		INSERT INTO purchase_events (id, purchase_id, event_type, message, details, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, uuid.New().String(), purchaseID, eventType, message, details, time.Now())
}

// Query operations

func (s *PurchaseService) Get(ctx context.Context, id string) (*models.Purchase, error) {
	var purchase models.Purchase
	err := s.db.GetContext(ctx, &purchase,
		`SELECT * FROM aged_domain_purchases WHERE id = $1`, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPurchaseNotFound
		}
		return nil, err
	}
	return &purchase, nil
}

func (s *PurchaseService) GetByIdempotency(ctx context.Context, key string) (*models.Purchase, error) {
	var purchase models.Purchase
	err := s.db.GetContext(ctx, &purchase,
		`SELECT * FROM aged_domain_purchases WHERE idempotency_key = $1`, key)
	if err != nil {
		return nil, err
	}
	return &purchase, nil
}

func (s *PurchaseService) GetStatus(ctx context.Context, id string) (*models.PurchaseStatus, error) {
	purchase, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	var events []models.PurchaseEvent
	s.db.SelectContext(ctx, &events,
		`SELECT * FROM purchase_events WHERE purchase_id = $1 ORDER BY created_at`, id)

	return &models.PurchaseStatus{
		PurchaseID: purchase.ID,
		Domain:     purchase.DomainName,
		Status:     purchase.Status,
		PriceCents: purchase.SalePrice,
		Events:     events,
		Error:      purchase.ErrorMessage,
	}, nil
}

func (s *PurchaseService) ListByUser(ctx context.Context, userID string) ([]models.Purchase, error) {
	var purchases []models.Purchase
	err := s.db.SelectContext(ctx, &purchases,
		`SELECT * FROM aged_domain_purchases WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	return purchases, err
}

func (s *PurchaseService) ListAll(ctx context.Context, page, perPage int) ([]models.Purchase, int, error) {
	if perPage <= 0 {
		perPage = 25
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * perPage

	var total int
	s.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM aged_domain_purchases`)

	var purchases []models.Purchase
	err := s.db.SelectContext(ctx, &purchases,
		`SELECT * FROM aged_domain_purchases ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		perPage, offset)
	return purchases, total, err
}

func (s *PurchaseService) GetStalePurchases(ctx context.Context, maxAge time.Duration) ([]models.Purchase, error) {
	var purchases []models.Purchase
	err := s.db.SelectContext(ctx, &purchases, `
		SELECT * FROM aged_domain_purchases
		WHERE status IN ('pending', 'purchasing', 'provisioning')
		AND created_at < $1
	`, time.Now().Add(-maxAge))
	return purchases, err
}
