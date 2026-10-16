// pkg/actions/create_domain_test.go
package actions

import (
	"context"
	"strings"
	"testing"

	"github.com/botginx/botginx/pkg/status"
)

func TestCreateDomainInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   CreateDomainInput
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid input",
			input: CreateDomainInput{
				UserID:     "user-123",
				DomainName: "example.com",
				SetupType:  status.SetupTypeDirect,
			},
			wantErr: false,
		},
		{
			name: "missing user ID",
			input: CreateDomainInput{
				DomainName: "example.com",
			},
			wantErr: true,
			errMsg:  "user ID required",
		},
		{
			name: "missing domain name",
			input: CreateDomainInput{
				UserID: "user-123",
			},
			wantErr: true,
			errMsg:  "domain name required",
		},
		{
			name: "domain too short",
			input: CreateDomainInput{
				UserID:     "user-123",
				DomainName: "ab",
			},
			wantErr: true,
			errMsg:  "domain name too short",
		},
		{
			name: "invalid domain format",
			input: CreateDomainInput{
				UserID:     "user-123",
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
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestCreateDomainInput_NormalizedName(t *testing.T) {
	input := CreateDomainInput{
		DomainName: "  EXAMPLE.COM  ",
	}
	if got := input.NormalizedName(); got != "example.com" {
		t.Errorf("NormalizedName() = %q, want %q", got, "example.com")
	}
}

func TestCreateDomainAction_Validate(t *testing.T) {
	deps := &Deps{DB: nil} // DB not needed for validation
	action := NewCreateDomain(deps, CreateDomainInput{
		UserID:     "user-123",
		DomainName: "example.com",
		SetupType:  status.SetupTypeDirect,
	})

	if err := action.Validate(context.Background()); err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}
