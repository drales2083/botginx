// pkg/actions/create_domain.go
package actions

import (
	"context"
	"errors"
	"strings"

	"github.com/botginx/botginx/pkg/status"
)

// CreateDomainInput contains validated input for domain creation.
type CreateDomainInput struct {
	UserID     string
	DomainName string
	SetupType  status.SetupType
	IsWildcard bool
	ServerID   string // Optional
}

// Validate checks that required fields are present and valid.
func (i CreateDomainInput) Validate() error {
	if i.UserID == "" {
		return errors.New("user ID required")
	}
	if i.DomainName == "" {
		return errors.New("domain name required")
	}
	// Normalize and validate domain format
	name := i.NormalizedName()
	if len(name) < 3 {
		return errors.New("domain name too short")
	}
	if !strings.Contains(name, ".") {
		return errors.New("invalid domain format")
	}
	return nil
}

// NormalizedName returns the domain name in lowercase with whitespace trimmed.
func (i CreateDomainInput) NormalizedName() string {
	return strings.ToLower(strings.TrimSpace(i.DomainName))
}

// CreateDomainAction encapsulates domain creation logic.
// This is a data-only action that validates input. The actual database
// operations remain in the service layer for now.
type CreateDomainAction struct {
	deps  *Deps
	input CreateDomainInput
}

// NewCreateDomain creates a new CreateDomainAction.
func NewCreateDomain(deps *Deps, input CreateDomainInput) *CreateDomainAction {
	return &CreateDomainAction{deps: deps, input: input}
}

// Input returns the validated input after normalization.
func (a *CreateDomainAction) Input() CreateDomainInput {
	return CreateDomainInput{
		UserID:     a.input.UserID,
		DomainName: a.input.NormalizedName(),
		SetupType:  a.input.SetupType,
		IsWildcard: a.input.IsWildcard,
		ServerID:   a.input.ServerID,
	}
}

// Validate checks the action input is valid.
func (a *CreateDomainAction) Validate(ctx context.Context) error {
	return a.input.Validate()
}
