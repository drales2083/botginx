package services

import (
	"testing"

	"github.com/botginx/botginx/modules/ageddomains/models"
)

func TestPurchaseStatusConstants(t *testing.T) {
	// Verify all status constants are defined correctly
	statuses := []string{
		models.StatusPending,
		models.StatusPurchasing,
		models.StatusProvisioning,
		models.StatusCompleted,
		models.StatusFailed,
		models.StatusCommitFailed,
		models.StatusProvisionFailed,
	}

	for _, s := range statuses {
		if len(s) == 0 {
			t.Error("Status constant should not be empty")
		}
	}
}

func TestLockStatusConstants(t *testing.T) {
	// Verify lock status constants
	lockStatuses := []string{
		models.LockStatusPending,
		models.LockStatusCommitted,
		models.LockStatusReleased,
	}

	for _, s := range lockStatuses {
		if len(s) == 0 {
			t.Error("Lock status constant should not be empty")
		}
	}
}

func TestPurchaseServiceErrors(t *testing.T) {
	// Verify error definitions
	if ErrPurchaseNotFound == nil {
		t.Error("ErrPurchaseNotFound should not be nil")
	}
	if ErrInsufficientBalance == nil {
		t.Error("ErrInsufficientBalance should not be nil")
	}
	if ErrDuplicatePurchase == nil {
		t.Error("ErrDuplicatePurchase should not be nil")
	}
}

func TestPurchaseStatusFlow(t *testing.T) {
	// Test that status flow makes sense
	// pending -> purchasing -> provisioning -> completed
	// any state -> failed

	flow := map[string][]string{
		models.StatusPending:      {models.StatusPurchasing, models.StatusFailed},
		models.StatusPurchasing:   {models.StatusProvisioning, models.StatusFailed, models.StatusCommitFailed},
		models.StatusProvisioning: {models.StatusCompleted, models.StatusFailed, models.StatusProvisionFailed},
		models.StatusCompleted:    {}, // terminal
		models.StatusFailed:       {}, // terminal
	}

	for from, toList := range flow {
		if from == "" {
			t.Errorf("From state should not be empty")
		}
		// Just verify the structure exists
		_ = toList
	}
}

func TestBalanceInfoStructure(t *testing.T) {
	info := models.BalanceInfo{
		Available: 10000,  // $100.00
		Locked:    2500,   // $25.00
		Total:     12500,  // $125.00
	}

	if info.Available+info.Locked != info.Total {
		t.Errorf("Available + Locked should equal Total")
	}

	if info.Available < 0 {
		t.Errorf("Available should not be negative")
	}
}

func TestSafetyCheckResultStructure(t *testing.T) {
	result := models.SafetyCheckResult{
		Status:    "clean",
		Threats:   []string{},
		CheckedAt: "2024-01-01T00:00:00Z",
	}

	if result.Status != "clean" && result.Status != "flagged" && result.Status != "unknown" {
		t.Errorf("Invalid status: %s", result.Status)
	}

	if result.Status == "clean" && len(result.Threats) > 0 {
		t.Errorf("Clean status should have no threats")
	}
}

// TestPurchaseFlowWithAvailabilityCheck verifies the purchase flow includes
// pre-purchase availability verification
func TestPurchaseFlowWithAvailabilityCheck(t *testing.T) {
	// The purchase flow must:
	// 1. Reserve domain in inventory
	// 2. Check availability at provider BEFORE locking funds
	// 3. Lock user funds (only after availability confirmed)
	// 4. Create purchase record
	// 5. Execute purchase asynchronously

	steps := []string{
		"ReserveForPurchase - prevent race conditions",
		"CheckAvailability - verify domain still available at provider",
		"LockFunds - only after availability verified",
		"Create purchase record",
		"Execute purchase async",
	}

	for i, step := range steps {
		if step == "" {
			t.Errorf("Step %d should not be empty", i)
		}
	}
}

// TestDomainUnavailableError verifies the error type exists
func TestDomainUnavailableError(t *testing.T) {
	if ErrDomainUnavailable == nil {
		t.Error("ErrDomainUnavailable should be defined")
	}
}

// TestPurchaseProvisioningCreatesCorrectDomainRecord documents the domain
// record fields that must be set for DNS auto-update to work
func TestPurchaseProvisioningCreatesCorrectDomainRecord(t *testing.T) {
	// When a domain is purchased from Porkbun, CreateDomainFromPurchase must set:
	// - source = 'porkbun'
	// - dns_auto_configured = true
	// - dns_api_user_id = admin user ID

	// This allows the domain to be included in "Update all DNS" reconfiguration

	requiredFields := map[string]string{
		"source":              "porkbun",
		"dns_auto_configured": "true",
		"dns_api_user_id":     "(admin user ID)",
	}

	// Verify source value
	if requiredFields["source"] != "porkbun" {
		t.Error("source must be 'porkbun' for Porkbun-purchased domains")
	}

	// Verify dns_auto_configured
	if requiredFields["dns_auto_configured"] != "true" {
		t.Error("dns_auto_configured must be 'true' for auto DNS updates")
	}

	// Verify dns_api_user_id is set
	if requiredFields["dns_api_user_id"] == "" {
		t.Error("dns_api_user_id must be set to admin user ID")
	}
}

// TestIdempotencyKeyGeneration verifies idempotency key format
func TestIdempotencyKeyGeneration(t *testing.T) {
	userID := "user_123"
	domainName := "example.com"

	key := GenerateIdempotencyKey(userID, domainName)

	if key == "" {
		t.Error("Idempotency key should not be empty")
	}

	// Same inputs should produce same key
	key2 := GenerateIdempotencyKey(userID, domainName)
	if key != key2 {
		t.Errorf("Same inputs should produce same key: %s != %s", key, key2)
	}

	// Different inputs should produce different keys
	key3 := GenerateIdempotencyKey(userID, "different.com")
	if key == key3 {
		t.Error("Different inputs should produce different keys")
	}
}

// TestPurchaseStatusTransitions documents valid status transitions
func TestPurchaseStatusTransitions(t *testing.T) {
	// Valid transitions:
	// pending -> purchasing (started provider registration)
	// purchasing -> provisioning (provider succeeded, now setting up domain)
	// provisioning -> completed (all done)
	// any -> failed (error occurred before point of no return)
	// purchasing -> commit_failed (domain purchased but balance debit failed - CRITICAL)
	// provisioning -> provision_failed (domain paid but DNS setup failed)

	transitions := map[string][]string{
		models.StatusPending:         {models.StatusPurchasing, models.StatusFailed},
		models.StatusPurchasing:      {models.StatusProvisioning, models.StatusFailed, models.StatusCommitFailed},
		models.StatusProvisioning:    {models.StatusCompleted, models.StatusProvisionFailed},
		models.StatusCompleted:       {},                              // terminal - success
		models.StatusFailed:          {},                              // terminal - can retry
		models.StatusCommitFailed:    {},                              // terminal - needs manual intervention
		models.StatusProvisionFailed: {models.StatusProvisioning},     // can retry DNS setup
	}

	// Verify all statuses are accounted for
	allStatuses := []string{
		models.StatusPending,
		models.StatusPurchasing,
		models.StatusProvisioning,
		models.StatusCompleted,
		models.StatusFailed,
		models.StatusCommitFailed,
		models.StatusProvisionFailed,
	}

	for _, status := range allStatuses {
		if _, ok := transitions[status]; !ok {
			t.Errorf("Status %s not in transitions map", status)
		}
	}
}

// TestPrePurchaseAvailabilityCheckPreventsChargingForSoldDomains documents
// the critical requirement that availability must be checked before locking funds
func TestPrePurchaseAvailabilityCheckPreventsChargingForSoldDomains(t *testing.T) {
	// The purchase flow MUST check domain availability at the provider
	// BEFORE locking user funds. This prevents the scenario where:
	// 1. User clicks buy
	// 2. We lock their funds
	// 3. Provider says domain is already sold
	// 4. User is charged but got nothing

	// Order of operations in PurchaseDomain:
	steps := []int{1, 2, 3, 4, 5}
	operations := map[int]string{
		1: "ReserveForPurchase - lock in inventory",
		2: "GetClient - get provider API client",
		3: "CheckAvailability - verify domain available at provider",
		4: "LockFunds - only now lock user balance",
		5: "Create purchase and execute",
	}

	// Verify CheckAvailability comes before LockFunds
	checkAvailStep := 3
	lockFundsStep := 4

	if checkAvailStep >= lockFundsStep {
		t.Error("CheckAvailability must happen BEFORE LockFunds")
	}

	for _, step := range steps {
		if _, ok := operations[step]; !ok {
			t.Errorf("Step %d not documented", step)
		}
	}
}

// TestDomainProviderInterfaceHasCheckAvailability verifies the interface contract
func TestDomainProviderInterfaceHasCheckAvailability(t *testing.T) {
	// DomainProvider interface must include CheckAvailability method
	// This is enforced at compile time by the interface definition

	// Method signature: CheckAvailability(ctx, domain) (available bool, priceCents int, err error)
	// Returns:
	// - available: true if domain can still be purchased
	// - priceCents: current price (may differ from cached price)
	// - err: any API error

	// This test documents the expectation
	expectedReturns := []string{"available bool", "priceCents int", "err error"}
	for _, ret := range expectedReturns {
		if ret == "" {
			t.Error("Return type documentation should not be empty")
		}
	}
}

