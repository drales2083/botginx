package actions

import (
	"context"
	"errors"
	"strings"
)

// DNSVerifier is the interface for DNS verification.
// This allows the action to work with any implementation.
type DNSVerifier interface {
	VerifyDNS(domain, expectedToken string) (bool, error)
}

// VerifyDNSInput contains validated input for DNS verification.
type VerifyDNSInput struct {
	DomainID      string
	DomainName    string
	ExpectedToken string
}

// Validate checks that required fields are present.
func (i VerifyDNSInput) Validate() error {
	if i.DomainID == "" {
		return errors.New("domain ID required")
	}
	if i.DomainName == "" {
		return errors.New("domain name required")
	}
	if i.ExpectedToken == "" {
		return errors.New("expected token required")
	}
	return nil
}

// BaseDomain returns the base domain for wildcards.
func (i VerifyDNSInput) BaseDomain() string {
	if strings.HasPrefix(i.DomainName, "*.") {
		return strings.TrimPrefix(i.DomainName, "*.")
	}
	return i.DomainName
}

// VerifyDNSResult contains the verification outcome.
type VerifyDNSResult struct {
	Verified bool
	Error    string
}

// VerifyDNSAction encapsulates DNS verification logic.
type VerifyDNSAction struct {
	verifier DNSVerifier
	input    VerifyDNSInput
}

// NewVerifyDNS creates a new VerifyDNSAction.
func NewVerifyDNS(verifier DNSVerifier, input VerifyDNSInput) *VerifyDNSAction {
	return &VerifyDNSAction{verifier: verifier, input: input}
}

// Validate checks the action input is valid.
func (a *VerifyDNSAction) Validate(ctx context.Context) error {
	return a.input.Validate()
}

// Execute runs the DNS verification.
func (a *VerifyDNSAction) Execute(ctx context.Context) (*VerifyDNSResult, error) {
	if err := a.input.Validate(); err != nil {
		return nil, err
	}

	verified, err := a.verifier.VerifyDNS(a.input.DomainName, a.input.ExpectedToken)

	result := &VerifyDNSResult{
		Verified: verified,
	}

	if err != nil {
		result.Error = err.Error()
	}

	return result, nil
}
