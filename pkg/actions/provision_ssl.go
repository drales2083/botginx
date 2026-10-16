package actions

import (
	"context"
	"errors"
	"strings"
)

// SSLProvisioner is the interface for SSL certificate provisioning.
type SSLProvisioner interface {
	GenerateSSL(domain string) error
}

// ProvisionSSLInput contains validated input for SSL provisioning.
type ProvisionSSLInput struct {
	DomainID   string
	DomainName string
	Force      bool // Force renewal even if cert exists
}

// Validate checks that required fields are present.
func (i ProvisionSSLInput) Validate() error {
	if i.DomainID == "" {
		return errors.New("domain ID required")
	}
	if i.DomainName == "" {
		return errors.New("domain name required")
	}
	// Basic domain format check
	name := strings.ToLower(strings.TrimSpace(i.DomainName))
	if !strings.Contains(name, ".") {
		return errors.New("invalid domain format")
	}
	return nil
}

// BaseDomain returns the base domain for wildcards.
func (i ProvisionSSLInput) BaseDomain() string {
	name := strings.TrimSpace(i.DomainName)
	if strings.HasPrefix(name, "*.") {
		return strings.TrimPrefix(name, "*.")
	}
	return name
}

// ProvisionSSLResult contains the provisioning outcome.
type ProvisionSSLResult struct {
	Success bool
	Error   string
}

// ProvisionSSLAction encapsulates SSL provisioning logic.
type ProvisionSSLAction struct {
	provisioner SSLProvisioner
	input       ProvisionSSLInput
}

// NewProvisionSSL creates a new ProvisionSSLAction.
func NewProvisionSSL(provisioner SSLProvisioner, input ProvisionSSLInput) *ProvisionSSLAction {
	return &ProvisionSSLAction{provisioner: provisioner, input: input}
}

// Validate checks the action input is valid.
func (a *ProvisionSSLAction) Validate(ctx context.Context) error {
	return a.input.Validate()
}

// Execute runs the SSL provisioning.
func (a *ProvisionSSLAction) Execute(ctx context.Context) (*ProvisionSSLResult, error) {
	if err := a.input.Validate(); err != nil {
		return nil, err
	}

	err := a.provisioner.GenerateSSL(a.input.BaseDomain())

	result := &ProvisionSSLResult{
		Success: err == nil,
	}

	if err != nil {
		result.Error = err.Error()
	}

	return result, nil
}
