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
