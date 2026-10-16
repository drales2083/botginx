package actions

import (
	"context"
	"errors"
	"testing"
)

// mockSSLProvisioner implements SSLProvisioner for testing
type mockSSLProvisioner struct {
	generateError error
	calledWith    string
}

func (m *mockSSLProvisioner) GenerateSSL(domain string) error {
	m.calledWith = domain
	return m.generateError
}

func TestProvisionSSLInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   ProvisionSSLInput
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid input",
			input: ProvisionSSLInput{
				DomainID:   "dom-123",
				DomainName: "example.com",
			},
			wantErr: false,
		},
		{
			name: "valid wildcard",
			input: ProvisionSSLInput{
				DomainID:   "dom-123",
				DomainName: "*.example.com",
			},
			wantErr: false,
		},
		{
			name: "missing domain ID",
			input: ProvisionSSLInput{
				DomainName: "example.com",
			},
			wantErr: true,
			errMsg:  "domain ID required",
		},
		{
			name: "missing domain name",
			input: ProvisionSSLInput{
				DomainID: "dom-123",
			},
			wantErr: true,
			errMsg:  "domain name required",
		},
		{
			name: "invalid domain format",
			input: ProvisionSSLInput{
				DomainID:   "dom-123",
				DomainName: "nodot",
			},
			wantErr: true,
			errMsg:  "invalid domain format",
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

func TestProvisionSSLInput_BaseDomain(t *testing.T) {
	tests := []struct {
		domain   string
		expected string
	}{
		{"example.com", "example.com"},
		{"*.example.com", "example.com"},
		{"  *.example.com  ", "example.com"},
		{"sub.example.com", "sub.example.com"},
	}

	for _, tt := range tests {
		input := ProvisionSSLInput{DomainName: tt.domain}
		if got := input.BaseDomain(); got != tt.expected {
			t.Errorf("BaseDomain(%q) = %q, want %q", tt.domain, got, tt.expected)
		}
	}
}

func TestProvisionSSLAction_Execute(t *testing.T) {
	t.Run("successful provisioning", func(t *testing.T) {
		provisioner := &mockSSLProvisioner{}
		action := NewProvisionSSL(provisioner, ProvisionSSLInput{
			DomainID:   "dom-123",
			DomainName: "example.com",
		})

		result, err := action.Execute(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Success {
			t.Error("expected Success to be true")
		}
		if result.Error != "" {
			t.Errorf("unexpected error in result: %s", result.Error)
		}
		if provisioner.calledWith != "example.com" {
			t.Errorf("expected GenerateSSL called with example.com, got %s", provisioner.calledWith)
		}
	})

	t.Run("wildcard uses base domain", func(t *testing.T) {
		provisioner := &mockSSLProvisioner{}
		action := NewProvisionSSL(provisioner, ProvisionSSLInput{
			DomainID:   "dom-123",
			DomainName: "*.example.com",
		})

		_, err := action.Execute(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if provisioner.calledWith != "example.com" {
			t.Errorf("expected GenerateSSL called with example.com, got %s", provisioner.calledWith)
		}
	})

	t.Run("provisioning error", func(t *testing.T) {
		provisioner := &mockSSLProvisioner{generateError: errors.New("ACME challenge failed")}
		action := NewProvisionSSL(provisioner, ProvisionSSLInput{
			DomainID:   "dom-123",
			DomainName: "example.com",
		})

		result, err := action.Execute(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Success {
			t.Error("expected Success to be false")
		}
		if result.Error != "ACME challenge failed" {
			t.Errorf("expected error message, got %q", result.Error)
		}
	})

	t.Run("invalid input", func(t *testing.T) {
		provisioner := &mockSSLProvisioner{}
		action := NewProvisionSSL(provisioner, ProvisionSSLInput{
			DomainName: "example.com",
			// Missing DomainID
		})

		_, err := action.Execute(context.Background())
		if err == nil {
			t.Error("expected validation error")
		}
	})
}
