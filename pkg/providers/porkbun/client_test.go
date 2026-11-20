package porkbun

import (
	"testing"
)

func TestNewClient(t *testing.T) {
	client := NewClient("test-api-key", "test-secret-key")

	if client == nil {
		t.Fatal("NewClient returned nil")
	}

	if client.apiKey != "test-api-key" {
		t.Errorf("apiKey = %q, want %q", client.apiKey, "test-api-key")
	}

	if client.secretKey != "test-secret-key" {
		t.Errorf("secretKey = %q, want %q", client.secretKey, "test-secret-key")
	}

	if client.baseURL != defaultBaseURL {
		t.Errorf("baseURL = %q, want %q", client.baseURL, defaultBaseURL)
	}

	if client.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
}

func TestSetBaseURL(t *testing.T) {
	client := NewClient("key", "secret")
	customURL := "https://custom.api.example.com"

	client.SetBaseURL(customURL)

	if client.baseURL != customURL {
		t.Errorf("baseURL = %q, want %q", client.baseURL, customURL)
	}
}

func TestAPIErrorMethods(t *testing.T) {
	tests := []struct {
		name    string
		err     *APIError
		wantStr string
	}{
		{
			name:    "error with code",
			err:     &APIError{Status: "ERROR", Message: "Domain not found", Code: "DOMAIN_NOT_FOUND"},
			wantStr: "porkbun: Domain not found (code: DOMAIN_NOT_FOUND)",
		},
		{
			name:    "error without code",
			err:     &APIError{Status: "ERROR", Message: "Something went wrong"},
			wantStr: "porkbun: Something went wrong",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.wantStr {
				t.Errorf("Error() = %q, want %q", got, tt.wantStr)
			}
		})
	}
}

func TestAPIErrorHelpers(t *testing.T) {
	tests := []struct {
		code                 string
		isCloseoutUnavailable bool
		isCostMismatch       bool
		isInsufficientFunds  bool
		isRateLimited        bool
	}{
		{"CLOSEOUT_UNAVAILABLE", true, false, false, false},
		{"COST_MISMATCH", false, true, false, false},
		{"INSUFFICIENT_FUNDS", false, false, true, false},
		{"RATE_LIMIT_EXCEEDED", false, false, false, true},
		{"OTHER_ERROR", false, false, false, false},
	}

	for _, tt := range tests {
		err := &APIError{Code: tt.code}

		if got := err.IsCloseoutUnavailable(); got != tt.isCloseoutUnavailable {
			t.Errorf("Code %q: IsCloseoutUnavailable() = %v, want %v", tt.code, got, tt.isCloseoutUnavailable)
		}
		if got := err.IsCostMismatch(); got != tt.isCostMismatch {
			t.Errorf("Code %q: IsCostMismatch() = %v, want %v", tt.code, got, tt.isCostMismatch)
		}
		if got := err.IsInsufficientFunds(); got != tt.isInsufficientFunds {
			t.Errorf("Code %q: IsInsufficientFunds() = %v, want %v", tt.code, got, tt.isInsufficientFunds)
		}
		if got := err.IsRateLimited(); got != tt.isRateLimited {
			t.Errorf("Code %q: IsRateLimited() = %v, want %v", tt.code, got, tt.isRateLimited)
		}
	}
}

func TestCheckResponseSuccess(t *testing.T) {
	body := []byte(`{"status":"SUCCESS","message":"ok"}`)

	resp, err := checkResponse(body)

	if err != nil {
		t.Errorf("checkResponse returned error for success: %v", err)
	}

	if resp.Status != "SUCCESS" {
		t.Errorf("Status = %q, want %q", resp.Status, "SUCCESS")
	}
}

func TestCheckResponseError(t *testing.T) {
	body := []byte(`{"status":"ERROR","message":"Invalid API key","code":"AUTH_FAILED"}`)

	resp, err := checkResponse(body)

	if err == nil {
		t.Error("checkResponse should return error for ERROR status")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatal("Error should be *APIError")
	}

	if apiErr.Code != "AUTH_FAILED" {
		t.Errorf("Error code = %q, want %q", apiErr.Code, "AUTH_FAILED")
	}

	if resp == nil {
		t.Error("Response should not be nil even on error")
	}
}

func TestMinRequestInterval(t *testing.T) {
	// Verify rate limiting is configured
	if minRequestInterval <= 0 {
		t.Error("minRequestInterval should be positive")
	}

	// Should be at least 100ms to avoid rate limiting
	if minRequestInterval.Milliseconds() < 100 {
		t.Errorf("minRequestInterval = %v, want >= 100ms", minRequestInterval)
	}
}
