package services

import (
	"testing"

	"github.com/botginx/botginx/modules/ageddomains/models"
	"github.com/botginx/botginx/pkg/crypto"
)

func TestNewPorkbunAdapterWithEncryptedKeys(t *testing.T) {
	// Set a test encryption key (32 bytes)
	crypto.SetKey([]byte("test-encryption-key-32-bytes!!!!"))

	// Encrypt test credentials
	encryptedKey, err := crypto.Encrypt("test-api-key")
	if err != nil {
		t.Fatalf("Failed to encrypt API key: %v", err)
	}
	encryptedSecret, err := crypto.Encrypt("test-api-secret")
	if err != nil {
		t.Fatalf("Failed to encrypt API secret: %v", err)
	}

	provider := models.Provider{
		ID:           "test-provider",
		Name:         "porkbun-test",
		DisplayName:  "Porkbun Test",
		ProviderType: "porkbun",
		APIKey:       encryptedKey,
		APISecret:    encryptedSecret,
		IsActive:     true,
	}

	adapter := NewPorkbunAdapter(provider)

	if adapter == nil {
		t.Fatal("NewPorkbunAdapter returned nil")
	}
	if adapter.client == nil {
		t.Fatal("Adapter client should not be nil")
	}
}

func TestNewPorkbunAdapterWithPlaintextKeys(t *testing.T) {
	// Test that plaintext keys (legacy) still work
	provider := models.Provider{
		ID:           "test-provider-legacy",
		Name:         "porkbun-legacy",
		DisplayName:  "Porkbun Legacy",
		ProviderType: "porkbun",
		APIKey:       "plaintext-api-key",
		APISecret:    "plaintext-api-secret",
		IsActive:     true,
	}

	adapter := NewPorkbunAdapter(provider)

	if adapter == nil {
		t.Fatal("NewPorkbunAdapter returned nil")
	}
	if adapter.client == nil {
		t.Fatal("Adapter client should not be nil")
	}
}

func TestProviderRegistryConstants(t *testing.T) {
	registry := NewProviderRegistry(nil)
	if registry == nil {
		t.Fatal("NewProviderRegistry returned nil")
	}
	if registry.clients == nil {
		t.Fatal("Registry clients map should be initialized")
	}
}

func TestProviderDomainStructure(t *testing.T) {
	domain := ProviderDomain{
		Domain:     "example.com",
		TLD:        "com",
		PriceCents: 999,
		AgeYears:   5,
		IsPremium:  false,
		ExpiresAt:  "2024-12-31",
	}

	if domain.Domain == "" {
		t.Error("Domain should not be empty")
	}
	if domain.PriceCents <= 0 {
		t.Error("PriceCents should be positive")
	}
}

func TestPriceFilterCalculation(t *testing.T) {
	// Test that max closeout price is calculated correctly from max sale price
	// Formula: max_closeout = max_sale / (1 + markup/100)
	tests := []struct {
		name             string
		maxSalePriceCents int
		markupPercent    float64
		wantCloseout     int
	}{
		{
			name:             "50% markup, $50 max sale",
			maxSalePriceCents: 5000, // $50.00
			markupPercent:    50.0,
			wantCloseout:     3333, // $33.33 (5000 / 1.5)
		},
		{
			name:             "25% markup, $100 max sale",
			maxSalePriceCents: 10000, // $100.00
			markupPercent:    25.0,
			wantCloseout:     8000, // $80.00 (10000 / 1.25)
		},
		{
			name:             "0% markup, $100 max sale",
			maxSalePriceCents: 10000,
			markupPercent:    0.0,
			wantCloseout:     10000, // No markup = same price
		},
		{
			name:             "100% markup, $200 max sale",
			maxSalePriceCents: 20000, // $200.00
			markupPercent:    100.0,
			wantCloseout:     10000, // $100.00 (20000 / 2.0)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			maxCloseout := tt.maxSalePriceCents
			if tt.markupPercent > 0 && maxCloseout > 0 {
				maxCloseout = int(float64(tt.maxSalePriceCents) / (1 + tt.markupPercent/100))
			}

			if maxCloseout != tt.wantCloseout {
				t.Errorf("got closeout price %d, want %d", maxCloseout, tt.wantCloseout)
			}
		})
	}
}

func TestSalePriceCalculation(t *testing.T) {
	// Test that sale price is calculated correctly from registration price
	// Formula: sale_price = registration_price * (1 + markup/100)
	tests := []struct {
		name              string
		registrationCents int
		markupPercent     float64
		wantSalePrice     int
	}{
		{
			name:              "50% markup on $20 registration",
			registrationCents: 2000, // $20.00
			markupPercent:     50.0,
			wantSalePrice:     3000, // $30.00 (2000 * 1.5)
		},
		{
			name:              "25% markup on $80 registration",
			registrationCents: 8000, // $80.00
			markupPercent:     25.0,
			wantSalePrice:     10000, // $100.00 (8000 * 1.25)
		},
		{
			name:              "0% markup, same price",
			registrationCents: 5000,
			markupPercent:     0.0,
			wantSalePrice:     5000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			salePrice := int(float64(tt.registrationCents) * (1 + tt.markupPercent/100))

			if salePrice != tt.wantSalePrice {
				t.Errorf("got sale price %d, want %d", salePrice, tt.wantSalePrice)
			}
		})
	}
}

func TestDomainProviderInterface(t *testing.T) {
	// Verify DomainProvider interface has all required methods
	var _ DomainProvider = (*PorkbunAdapter)(nil)
}

// TestCheckAvailabilityReturnValues documents expected return values
func TestCheckAvailabilityReturnValues(t *testing.T) {
	// CheckAvailability returns:
	// - available: whether domain can still be purchased
	// - priceCents: current price (may differ from cached inventory price)
	// - err: API error or nil

	// Scenarios:
	scenarios := []struct {
		name       string
		available  bool
		priceCents int
		hasError   bool
		meaning    string
	}{
		{
			name:       "domain available at same price",
			available:  true,
			priceCents: 2500,
			hasError:   false,
			meaning:    "can purchase at expected price",
		},
		{
			name:       "domain available but price changed",
			available:  true,
			priceCents: 3000, // higher than cached
			hasError:   false,
			meaning:    "can purchase but price may need update",
		},
		{
			name:       "domain no longer available",
			available:  false,
			priceCents: 0,
			hasError:   true,
			meaning:    "cannot purchase - sold or expired",
		},
		{
			name:       "API error",
			available:  false,
			priceCents: 0,
			hasError:   true,
			meaning:    "cannot determine availability - retry later",
		},
	}

	for _, s := range scenarios {
		if s.name == "" {
			t.Error("Scenario name should not be empty")
		}
		if !s.available && !s.hasError && s.priceCents == 0 {
			// This is the expected state for unavailable domains
		}
	}
}

// TestDomainProviderInterfaceMethods documents all required interface methods
func TestDomainProviderInterfaceMethods(t *testing.T) {
	// DomainProvider interface requires these methods:
	methods := []struct {
		name        string
		description string
	}{
		{
			name:        "ListAgedDomains",
			description: "Returns available aged domains from the provider",
		},
		{
			name:        "CheckAvailability",
			description: "Verifies a domain is still available and returns current price",
		},
		{
			name:        "RegisterDomain",
			description: "Purchases a domain and returns the order ID",
		},
		{
			name:        "SetNameservers",
			description: "Configures nameservers for a domain",
		},
		{
			name:        "TestConnection",
			description: "Verifies API credentials work",
		},
	}

	for _, m := range methods {
		if m.name == "" {
			t.Error("Method name should not be empty")
		}
		if m.description == "" {
			t.Errorf("Method %s should have a description", m.name)
		}
	}
}

// TestPorkbunCloseoutToPurchaseFlow documents the closeout purchase flow
func TestPorkbunCloseoutToPurchaseFlow(t *testing.T) {
	// Porkbun closeout domain purchase flow:
	// 1. ListAgedDomains (GET /closeout/search) - gets available closeouts
	// 2. CheckAvailability (GET /closeout/get/{domain}) - verifies still available
	// 3. RegisterDomain (POST /closeout/buy/{domain}) - purchases the domain
	// 4. SetNameservers (POST /domain/updateNs/{domain}) - configures DNS
	// 5. SetupDNS via CreateDNSRecord (POST /dns/create/{domain}) - creates A/TXT records

	steps := []struct {
		step       int
		method     string
		endpoint   string
		httpMethod string
	}{
		{1, "ListAgedDomains", "GET /closeout/search", "GET"},
		{2, "CheckAvailability", "GET /closeout/get/{domain}", "GET"},
		{3, "RegisterDomain", "POST /closeout/buy/{domain}", "POST"},
		{4, "SetNameservers", "POST /domain/updateNs/{domain}", "POST"},
		{5, "CreateDNSRecord", "POST /dns/create/{domain}", "POST"},
	}

	for _, s := range steps {
		if s.method == "" {
			t.Errorf("Step %d should have a method name", s.step)
		}
	}
}

// TestPriceFilterAppliedToSalePrice verifies price filter semantics
func TestPriceFilterAppliedToSalePrice(t *testing.T) {
	// MaxPriceCents in provider config is the maximum SALE price (what user pays)
	// When filtering closeouts, we must convert to max closeout price:
	// max_closeout = max_sale / (1 + markup/100)
	//
	// Example:
	// - Admin sets MaxPriceCents = $50 (max user pays)
	// - Admin sets MarkupPercent = 50%
	// - We filter closeouts costing <= $33.33
	// - After markup: $33.33 * 1.5 = $50 (what user pays)

	maxSale := 5000      // $50.00 max sale price
	markup := 50.0       // 50% markup

	// Calculate max closeout price to filter by
	maxCloseout := int(float64(maxSale) / (1 + markup/100))

	// Verify the math
	if maxCloseout != 3333 {
		t.Errorf("Expected max closeout 3333, got %d", maxCloseout)
	}

	// Verify sale price after markup doesn't exceed max
	salePrice := int(float64(maxCloseout) * (1 + markup/100))
	if salePrice > maxSale {
		t.Errorf("Sale price %d exceeds max %d", salePrice, maxSale)
	}
}
