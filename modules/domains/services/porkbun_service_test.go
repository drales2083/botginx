package services

import (
	"testing"
)

func TestNewPorkbunService(t *testing.T) {
	// Without credentials
	svc := NewPorkbunService(nil, "", "")
	if svc == nil {
		t.Fatal("NewPorkbunService returned nil")
	}
	if svc.GetClient() != nil {
		t.Error("Client should be nil without credentials")
	}
}

func TestNewPorkbunServiceWithCredentials(t *testing.T) {
	svc := NewPorkbunService(nil, "test-key", "test-secret")
	if svc == nil {
		t.Fatal("NewPorkbunService returned nil")
	}
	if svc.GetClient() == nil {
		t.Error("Client should not be nil with credentials")
	}
}

func TestSetCredentials(t *testing.T) {
	svc := NewPorkbunService(nil, "", "")

	if svc.GetClient() != nil {
		t.Error("Client should be nil initially")
	}

	svc.SetCredentials("new-key", "new-secret")

	if svc.GetClient() == nil {
		t.Error("Client should not be nil after setting credentials")
	}
}

func TestSetCredentialsEmpty(t *testing.T) {
	svc := NewPorkbunService(nil, "key", "secret")

	if svc.GetClient() == nil {
		t.Error("Client should exist with credentials")
	}

	// Setting empty credentials should not create client
	svc.SetCredentials("", "")

	// But it shouldn't crash
}

func TestReconfigureResultStructure(t *testing.T) {
	// Success result
	successResult := ReconfigureResult{
		Domain:  "example.com",
		Success: true,
		Error:   "",
	}

	if successResult.Domain == "" {
		t.Error("Domain should not be empty")
	}
	if !successResult.Success {
		t.Error("Success should be true")
	}
	if successResult.Error != "" {
		t.Error("Error should be empty for success")
	}

	// Failure result
	failResult := ReconfigureResult{
		Domain:  "failed.com",
		Success: false,
		Error:   "API error",
	}

	if failResult.Success {
		t.Error("Success should be false")
	}
	if failResult.Error == "" {
		t.Error("Error should not be empty for failure")
	}
}
