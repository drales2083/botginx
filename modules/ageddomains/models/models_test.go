package models

import (
	"encoding/json"
	"testing"
)

func TestProviderJSONSerialization(t *testing.T) {
	provider := Provider{
		ID:            "prov123",
		Name:          "porkbun",
		DisplayName:   "Porkbun",
		ProviderType:  "porkbun",
		APIKey:        "secret-api-key",
		APISecret:     "super-secret",
		IsActive:      true,
		MarkupPercent: 10.0,
	}

	data, err := json.Marshal(provider)
	if err != nil {
		t.Fatalf("Failed to marshal Provider: %v", err)
	}

	// Verify sensitive fields are hidden
	jsonStr := string(data)
	if contains(jsonStr, "secret-api-key") {
		t.Error("APIKey should be hidden from JSON (json:\"-\")")
	}
	if contains(jsonStr, "super-secret") {
		t.Error("APISecret should be hidden from JSON (json:\"-\")")
	}
	// ProviderType is intentionally visible (not sensitive like API keys)
	if !contains(jsonStr, "providerType") {
		t.Error("ProviderType should be visible in JSON")
	}

	// Verify visible fields are present
	if !contains(jsonStr, "prov123") {
		t.Error("ID should be visible in JSON")
	}
	if !contains(jsonStr, "Porkbun") {
		t.Error("DisplayName should be visible in JSON")
	}
}

func TestDomainInventoryJSONSerialization(t *testing.T) {
	domain := DomainInventory{
		ID:                "dom123",
		ProviderID:        "prov456",
		DomainName:        "example.com",
		TLD:               "com",
		RegistrationPrice: 999,  // Hidden
		SalePrice:         1299, // Visible as priceCents
		SafetyStatus:      "clean",
	}

	data, err := json.Marshal(domain)
	if err != nil {
		t.Fatalf("Failed to marshal DomainInventory: %v", err)
	}

	jsonStr := string(data)

	// Verify hidden fields
	if contains(jsonStr, "prov456") && contains(jsonStr, "providerId") {
		t.Error("ProviderID should be hidden from JSON")
	}
	if contains(jsonStr, "999") && contains(jsonStr, "registrationPrice") {
		t.Error("RegistrationPrice should be hidden from JSON")
	}

	// Verify visible fields
	if !contains(jsonStr, "example.com") {
		t.Error("DomainName should be visible as 'name' in JSON")
	}
	if !contains(jsonStr, "1299") {
		t.Error("SalePrice should be visible as 'priceCents' in JSON")
	}
}

func TestPurchaseJSONSerialization(t *testing.T) {
	purchase := Purchase{
		ID:              "pur123",
		UserID:          "user456",
		DomainName:      "bought.com",
		ProviderID:      "prov789",
		ProviderPrice:   999,  // Hidden
		SalePrice:       1299, // Visible
		BalanceLockID:   "lock123",
		IdempotencyKey:  "idem-key",
		Status:          StatusCompleted,
		ProviderOrderID: "order999",
	}

	data, err := json.Marshal(purchase)
	if err != nil {
		t.Fatalf("Failed to marshal Purchase: %v", err)
	}

	jsonStr := string(data)

	// Verify hidden fields
	if contains(jsonStr, "prov789") && contains(jsonStr, "providerId") {
		t.Error("ProviderID should be hidden from JSON")
	}
	if contains(jsonStr, "idem-key") {
		t.Error("IdempotencyKey should be hidden from JSON")
	}
	if contains(jsonStr, "lock123") && contains(jsonStr, "balanceLockId") {
		t.Error("BalanceLockID should be hidden from JSON")
	}
	if contains(jsonStr, "order999") && contains(jsonStr, "providerOrderId") {
		t.Error("ProviderOrderID should be hidden from JSON")
	}

	// Verify visible fields
	if !contains(jsonStr, "bought.com") {
		t.Error("DomainName should be visible in JSON")
	}
	if !contains(jsonStr, "completed") {
		t.Error("Status should be visible in JSON")
	}
}

func TestBalanceLockJSONSerialization(t *testing.T) {
	lock := BalanceLock{
		ID:             "lock123",
		UserID:         "user456",
		Amount:         5000,
		IdempotencyKey: "secret-idem",
		Purpose:        "Domain purchase",
		Status:         LockStatusPending,
	}

	data, err := json.Marshal(lock)
	if err != nil {
		t.Fatalf("Failed to marshal BalanceLock: %v", err)
	}

	jsonStr := string(data)

	// Verify hidden fields
	if contains(jsonStr, "secret-idem") {
		t.Error("IdempotencyKey should be hidden from JSON")
	}

	// Verify visible fields
	if !contains(jsonStr, "5000") {
		t.Error("Amount should be visible in JSON")
	}
	if !contains(jsonStr, "pending") {
		t.Error("Status should be visible in JSON")
	}
}

func TestPurchaseEventJSONMapping(t *testing.T) {
	event := PurchaseEvent{
		ID:         "evt123",
		PurchaseID: "pur456",
		EventType:  "dns_configured",
		Message:    "DNS records created",
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Failed to marshal PurchaseEvent: %v", err)
	}

	jsonStr := string(data)

	// EventType should serialize as "type"
	if !contains(jsonStr, `"type":"dns_configured"`) {
		t.Error("EventType should serialize as 'type'")
	}

	// CreatedAt should serialize as "at"
	// (can't fully test without time value)
}

func TestStatusConstants(t *testing.T) {
	// Verify status string values
	if StatusPending != "pending" {
		t.Errorf("StatusPending = %q, want %q", StatusPending, "pending")
	}
	if StatusPurchasing != "purchasing" {
		t.Errorf("StatusPurchasing = %q, want %q", StatusPurchasing, "purchasing")
	}
	if StatusProvisioning != "provisioning" {
		t.Errorf("StatusProvisioning = %q, want %q", StatusProvisioning, "provisioning")
	}
	if StatusCompleted != "completed" {
		t.Errorf("StatusCompleted = %q, want %q", StatusCompleted, "completed")
	}
	if StatusFailed != "failed" {
		t.Errorf("StatusFailed = %q, want %q", StatusFailed, "failed")
	}
}

func TestLockStatusConstants(t *testing.T) {
	if LockStatusPending != "pending" {
		t.Errorf("LockStatusPending = %q, want %q", LockStatusPending, "pending")
	}
	if LockStatusCommitted != "committed" {
		t.Errorf("LockStatusCommitted = %q, want %q", LockStatusCommitted, "committed")
	}
	if LockStatusReleased != "released" {
		t.Errorf("LockStatusReleased = %q, want %q", LockStatusReleased, "released")
	}
}

func TestSafetyStatusConstants(t *testing.T) {
	if SafetyUnchecked != "unchecked" {
		t.Errorf("SafetyUnchecked = %q, want %q", SafetyUnchecked, "unchecked")
	}
	if SafetyClean != "clean" {
		t.Errorf("SafetyClean = %q, want %q", SafetyClean, "clean")
	}
	if SafetyFlagged != "flagged" {
		t.Errorf("SafetyFlagged = %q, want %q", SafetyFlagged, "flagged")
	}
}

func TestBalanceInfoCalculation(t *testing.T) {
	info := BalanceInfo{
		Available: 7500,
		Locked:    2500,
		Total:     10000,
	}

	if info.Available+info.Locked != info.Total {
		t.Errorf("Available (%d) + Locked (%d) should equal Total (%d)", info.Available, info.Locked, info.Total)
	}
}

func TestSafetyCheckResult(t *testing.T) {
	// Clean result
	clean := SafetyCheckResult{
		Status:    "clean",
		Threats:   []string{},
		CheckedAt: "2024-01-01T12:00:00Z",
	}

	if len(clean.Threats) != 0 {
		t.Error("Clean result should have no threats")
	}

	// Flagged result
	flagged := SafetyCheckResult{
		Status:    "flagged",
		Threats:   []string{"google: malware", "urlhaus: botnet"},
		CheckedAt: "2024-01-01T12:00:00Z",
	}

	if len(flagged.Threats) == 0 {
		t.Error("Flagged result should have threats")
	}
}

func TestSortConstants(t *testing.T) {
	if SortByPrice != "price" {
		t.Errorf("SortByPrice = %q, want %q", SortByPrice, "price")
	}
	if SortByAge != "age" {
		t.Errorf("SortByAge = %q, want %q", SortByAge, "age")
	}
	if SortByName != "name" {
		t.Errorf("SortByName = %q, want %q", SortByName, "name")
	}
	if SortAsc != "asc" {
		t.Errorf("SortAsc = %q, want %q", SortAsc, "asc")
	}
	if SortDesc != "desc" {
		t.Errorf("SortDesc = %q, want %q", SortDesc, "desc")
	}
}

func TestListFiltersDefaults(t *testing.T) {
	filters := ListFilters{}

	if filters.Page != 0 {
		t.Error("Page should default to 0")
	}
	if filters.PerPage != 0 {
		t.Error("PerPage should default to 0")
	}
	if filters.FavoritesOnly {
		t.Error("FavoritesOnly should default to false")
	}
}

func TestListFiltersWithValues(t *testing.T) {
	filters := ListFilters{
		TLD:           ".com",
		MinPrice:      500,
		MaxPrice:      5000,
		MinAge:        3,
		MaxAge:        10,
		Category:      "aged",
		SafetyStatus:  "clean",
		Search:        "example",
		FavoritesOnly: true,
		SortBy:        SortByPrice,
		SortDir:       SortAsc,
		Page:          1,
		PerPage:       25,
	}

	if filters.TLD != ".com" {
		t.Errorf("TLD = %q, want %q", filters.TLD, ".com")
	}
	if filters.MinPrice != 500 {
		t.Errorf("MinPrice = %d, want %d", filters.MinPrice, 500)
	}
	if filters.MaxPrice != 5000 {
		t.Errorf("MaxPrice = %d, want %d", filters.MaxPrice, 5000)
	}
	if !filters.FavoritesOnly {
		t.Error("FavoritesOnly should be true")
	}
	if filters.SortBy != SortByPrice {
		t.Errorf("SortBy = %q, want %q", filters.SortBy, SortByPrice)
	}
}

func TestFavoriteStructure(t *testing.T) {
	fav := Favorite{
		ID:          "fav123",
		UserID:      "user456",
		InventoryID: "inv789",
	}

	if fav.ID == "" {
		t.Error("ID should not be empty")
	}
	if fav.UserID == "" {
		t.Error("UserID should not be empty")
	}
	if fav.InventoryID == "" {
		t.Error("InventoryID should not be empty")
	}
}

func TestDomainWithFavorite(t *testing.T) {
	domain := DomainWithFavorite{
		DomainInventory: DomainInventory{
			ID:         "dom123",
			DomainName: "example.com",
			TLD:        "com",
			SalePrice:  1299,
		},
		IsFavorite: true,
	}

	if domain.DomainName != "example.com" {
		t.Error("DomainName should be example.com")
	}
	if !domain.IsFavorite {
		t.Error("IsFavorite should be true")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
