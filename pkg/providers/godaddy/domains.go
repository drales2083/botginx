package godaddy

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Domain represents a GoDaddy domain
type Domain struct {
	Domain           string    `json:"domain"`
	DomainID         int       `json:"domainId"`
	Status           string    `json:"status"` // ACTIVE, CANCELLED, etc.
	Expires          time.Time `json:"expires"`
	ExpirationProtected bool   `json:"expirationProtected"`
	HoldRegistrar    bool      `json:"holdRegistrar"`
	Locked           bool      `json:"locked"`
	Privacy          bool      `json:"privacy"`
	RenewAuto        bool      `json:"renewAuto"`
	RenewDeadline    time.Time `json:"renewDeadline"`
	Renewable        bool      `json:"renewable"`
	TransferProtected bool     `json:"transferProtected"`
}

// ListDomains returns all domains in the account
func (c *Client) ListDomains(ctx context.Context) ([]Domain, error) {
	path := "/v1/domains"
	body, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var domains []Domain
	if err := json.Unmarshal(body, &domains); err != nil {
		return nil, fmt.Errorf("parsing domains response: %w", err)
	}

	return domains, nil
}

// GetDomain returns details for a specific domain
func (c *Client) GetDomain(ctx context.Context, domain string) (*Domain, error) {
	path := fmt.Sprintf("/v1/domains/%s", domain)
	body, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var d Domain
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, fmt.Errorf("parsing domain response: %w", err)
	}

	return &d, nil
}

// VerifyCredentials checks if the API credentials are valid
func (c *Client) VerifyCredentials(ctx context.Context) error {
	_, err := c.ListDomains(ctx)
	return err
}
