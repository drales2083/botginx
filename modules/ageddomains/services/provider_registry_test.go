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
