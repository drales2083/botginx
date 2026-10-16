package actions

import (
	"context"
	"errors"
	"testing"
)

// mockDNSVerifier implements DNSVerifier for testing
type mockDNSVerifier struct {
	verifyResult bool
	verifyError  error
}

func (m *mockDNSVerifier) VerifyDNS(domain, expectedToken string) (bool, error) {
	return m.verifyResult, m.verifyError
}

func TestVerifyDNSInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   VerifyDNSInput
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid input",
			input: VerifyDNSInput{
				DomainID:      "dom-123",
				DomainName:    "example.com",
				ExpectedToken: "abc123",
			},
			wantErr: false,
		},
		{
			name: "missing domain ID",
			input: VerifyDNSInput{
				DomainName:    "example.com",
				ExpectedToken: "abc123",
			},
			wantErr: true,
			errMsg:  "domain ID required",
		},
		{
			name: "missing domain name",
			input: VerifyDNSInput{
				DomainID:      "dom-123",
				ExpectedToken: "abc123",
			},
			wantErr: true,
			errMsg:  "domain name required",
		},
		{
			name: "missing token",
			input: VerifyDNSInput{
				DomainID:   "dom-123",
				DomainName: "example.com",
			},
			wantErr: true,
			errMsg:  "expected token required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestVerifyDNSInput_BaseDomain(t *testing.T) {
	tests := []struct {
		domain   string
		expected string
	}{
		{"example.com", "example.com"},
		{"*.example.com", "example.com"},
		{"sub.example.com", "sub.example.com"},
	}

	for _, tt := range tests {
		input := VerifyDNSInput{DomainName: tt.domain}
		if got := input.BaseDomain(); got != tt.expected {
			t.Errorf("BaseDomain(%q) = %q, want %q", tt.domain, got, tt.expected)
		}
	}
}

func TestVerifyDNSAction_Execute(t *testing.T) {
	t.Run("successful verification", func(t *testing.T) {
		verifier := &mockDNSVerifier{verifyResult: true}
		action := NewVerifyDNS(verifier, VerifyDNSInput{
			DomainID:      "dom-123",
			DomainName:    "example.com",
			ExpectedToken: "abc123",
		})

		result, err := action.Execute(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Verified {
			t.Error("expected Verified to be true")
		}
		if result.Error != "" {
			t.Errorf("unexpected error in result: %s", result.Error)
		}
	})

	t.Run("failed verification", func(t *testing.T) {
		verifier := &mockDNSVerifier{verifyResult: false}
		action := NewVerifyDNS(verifier, VerifyDNSInput{
			DomainID:      "dom-123",
			DomainName:    "example.com",
			ExpectedToken: "abc123",
		})

		result, err := action.Execute(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Verified {
			t.Error("expected Verified to be false")
		}
	})

	t.Run("verification error", func(t *testing.T) {
		verifier := &mockDNSVerifier{verifyResult: false, verifyError: errors.New("DNS lookup failed")}
		action := NewVerifyDNS(verifier, VerifyDNSInput{
			DomainID:      "dom-123",
			DomainName:    "example.com",
			ExpectedToken: "abc123",
		})

		result, err := action.Execute(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Error != "DNS lookup failed" {
			t.Errorf("expected error message, got %q", result.Error)
		}
	})

	t.Run("invalid input", func(t *testing.T) {
		verifier := &mockDNSVerifier{}
		action := NewVerifyDNS(verifier, VerifyDNSInput{
			DomainName: "example.com",
			// Missing DomainID
		})

		_, err := action.Execute(context.Background())
		if err == nil {
			t.Error("expected validation error")
		}
	})
}
