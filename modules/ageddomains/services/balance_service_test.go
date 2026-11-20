package services

import (
	"testing"
)

func TestGenerateIdempotencyKey(t *testing.T) {
	key1 := GenerateIdempotencyKey("user123", "example.com")
	key2 := GenerateIdempotencyKey("user123", "example.com")
	key3 := GenerateIdempotencyKey("user456", "example.com")

	// Same inputs on same day should produce same key
	if key1 != key2 {
		t.Errorf("Expected same key for same inputs, got %s and %s", key1, key2)
	}

	// Different user should produce different key
	if key1 == key3 {
		t.Errorf("Expected different key for different user, got same %s", key1)
	}

	// Key should be 64 characters (SHA256 hex)
	if len(key1) != 64 {
		t.Errorf("Expected key length 64, got %d", len(key1))
	}
}

func TestBalanceLockConstants(t *testing.T) {
	// Verify lock duration is 10 minutes
	expectedDuration := 10 * 60 // 10 minutes in seconds
	actualDuration := int(LockDuration.Seconds())

	if actualDuration != expectedDuration {
		t.Errorf("Expected lock duration %d seconds, got %d", expectedDuration, actualDuration)
	}
}
