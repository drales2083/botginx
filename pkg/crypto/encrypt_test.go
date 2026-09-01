package crypto

import (
	"strings"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	// Set a test key (32 bytes)
	SetKey([]byte("12345678901234567890123456789012"))

	original := "my-secret-password"
	encrypted, err := Encrypt(original)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if encrypted == original {
		t.Error("Encrypted should not equal original")
	}

	decrypted, err := Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if decrypted != original {
		t.Errorf("Decrypted %q != original %q", decrypted, original)
	}
}

func TestEncryptDifferentNonce(t *testing.T) {
	SetKey([]byte("12345678901234567890123456789012"))

	original := "same-password"
	enc1, _ := Encrypt(original)
	enc2, _ := Encrypt(original)

	if enc1 == enc2 {
		t.Error("Same plaintext should produce different ciphertexts (different nonces)")
	}
}

func TestGeneratePassword(t *testing.T) {
	p1 := GeneratePassword(16)
	p2 := GeneratePassword(16)

	if len(p1) != 16 {
		t.Errorf("Password length should be 16, got %d", len(p1))
	}

	if p1 == p2 {
		t.Error("Generated passwords should be different")
	}
}

func TestGenerateUsername(t *testing.T) {
	u := GenerateUsername("bp_")

	if !strings.HasPrefix(u, "bp_") {
		t.Errorf("Username should start with bp_, got %s", u)
	}

	if len(u) != 8 { // "bp_" (3) + 5 chars
		t.Errorf("Username length should be 8, got %d", len(u))
	}
}

func TestEncryptNoKey(t *testing.T) {
	SetKey(nil)
	_, err := Encrypt("test")
	if err == nil {
		t.Error("Should error without key")
	}
}
