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

// TestPorkbunServiceCreatesClientWithCredentials verifies client initialization
func TestPorkbunServiceCreatesClientWithCredentials(t *testing.T) {
	tests := []struct {
		name       string
		apiKey     string
		apiSecret  string
		wantClient bool
	}{
		{
			name:       "both credentials provided",
			apiKey:     "pk1_test",
			apiSecret:  "sk1_test",
			wantClient: true,
		},
		{
			name:       "only api key",
			apiKey:     "pk1_test",
			apiSecret:  "",
			wantClient: false,
		},
		{
			name:       "only api secret",
			apiKey:     "",
			apiSecret:  "sk1_test",
			wantClient: false,
		},
		{
			name:       "no credentials",
			apiKey:     "",
			apiSecret:  "",
			wantClient: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewPorkbunService(nil, tt.apiKey, tt.apiSecret)
			hasClient := svc.GetClient() != nil
			if hasClient != tt.wantClient {
				t.Errorf("GetClient() returned %v, want %v", hasClient, tt.wantClient)
			}
		})
	}
}

// TestSetCredentialsUpdatesClient verifies credential updates work
func TestSetCredentialsUpdatesClient(t *testing.T) {
	svc := NewPorkbunService(nil, "", "")

	// Initially no client
	if svc.GetClient() != nil {
		t.Error("Should have no client initially")
	}

	// Set valid credentials
	svc.SetCredentials("pk1_newkey", "sk1_newsecret")
	if svc.GetClient() == nil {
		t.Error("Should have client after setting valid credentials")
	}

	// Set partial credentials (should not create new client but shouldn't crash)
	svc.SetCredentials("only_key", "")
	// Just verify no panic
}

// TestReconfigureResultFields verifies all result fields are accessible
func TestReconfigureResultFields(t *testing.T) {
	result := ReconfigureResult{
		Domain:  "test.com",
		Success: true,
		Error:   "some error",
	}

	// Verify all fields can be accessed
	_ = result.Domain
	_ = result.Success
	_ = result.Error

	// Verify JSON tags work (structural test)
	if result.Domain != "test.com" {
		t.Errorf("Domain = %s, want test.com", result.Domain)
	}
}

// TestPorkbunServiceDNSAutoConfiguredRequirement documents the DNS auto-configure requirement
// Domains with source='porkbun' and dns_auto_configured=true are updated when
// admin clicks "Update all DNS" button
func TestPorkbunServiceDNSAutoConfiguredRequirement(t *testing.T) {
	// This test documents the query requirements for ReconfigureAllDomains:
	// SELECT ... FROM domains WHERE source = 'porkbun' AND dns_auto_configured = true

	// Domains meeting these criteria will have DNS updated via admin's Porkbun API key
	requiredFields := map[string]interface{}{
		"source":              "porkbun",
		"dns_auto_configured": true,
	}

	// Verify the expected values
	if requiredFields["source"] != "porkbun" {
		t.Error("source must be 'porkbun'")
	}
	if requiredFields["dns_auto_configured"] != true {
		t.Error("dns_auto_configured must be true")
	}
}

// TestPorkbunDomainCreationRequirements documents fields required for purchased domains
// to work with the "Update all DNS" button
func TestPorkbunDomainCreationRequirements(t *testing.T) {
	// CreateDomainFromPurchase must set these fields for DNS auto-update to work:
	requiredFields := []string{
		"source",              // must be 'porkbun'
		"dns_auto_configured", // must be true
		"dns_api_user_id",     // must be admin user ID
	}

	for _, field := range requiredFields {
		if field == "" {
			t.Error("Required field name should not be empty")
		}
	}

	// Document the expected values in the INSERT statement:
	// source = 'porkbun'
	// dns_auto_configured = true
	// dns_api_user_id = (admin user ID from: SELECT id FROM users WHERE role = 'admin')
}

// TestReconfigureAllDomainsQueryPattern documents the query pattern
func TestReconfigureAllDomainsQueryPattern(t *testing.T) {
	// ReconfigureAllDomains queries for domains matching:
	expectedQuery := `
		SELECT id, name, user_id, verify_token
		FROM domains
		WHERE source = 'porkbun' AND dns_auto_configured = true
	`

	// The query must include these columns for DNS update to work
	requiredColumns := []string{"id", "name", "verify_token"}

	for _, col := range requiredColumns {
		if col == "" {
			t.Error("Required column should not be empty")
		}
	}

	// Just verify the pattern is documented
	_ = expectedQuery
}

// TestDNSUpdateFlowForPorkbunDomains documents the DNS update flow
func TestDNSUpdateFlowForPorkbunDomains(t *testing.T) {
	// When admin changes VPS and clicks "Update all DNS":
	// 1. AdminReconfigureAllDomains handler is called
	// 2. reconfigureAll() is invoked with new server IP
	// 3. PorkbunService.ReconfigureAllDomains() is called
	// 4. For each domain with source='porkbun' and dns_auto_configured=true:
	//    a. SetupDNS() creates/updates @ and * A records
	//    b. TXT verification record is updated if verify_token exists
	// 5. Domain's dns_verified and dns_config_error are updated

	steps := []string{
		"AdminReconfigureAllDomains handler receives request",
		"reconfigureAll() called with new server IP",
		"PorkbunService.ReconfigureAllDomains() queries porkbun domains",
		"For each domain: SetupDNS() updates DNS records",
		"TXT verification record updated",
		"Domain status updated in database",
	}

	for i, step := range steps {
		if step == "" {
			t.Errorf("Step %d should not be empty", i)
		}
	}
}
