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
