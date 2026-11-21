package porkbun

import (
	"context"
	"encoding/json"
	"fmt"
)

// Domain represents a domain in the Porkbun account
type Domain struct {
	Domain             string   `json:"domain"`
	Status             string   `json:"status"` // ACTIVE, etc.
	TLD                string   `json:"tld"`
	CreateDate         string   `json:"createDate"`
	ExpireDate         string   `json:"expireDate"`
	SecurityLock       string   `json:"securityLock"`
	WhoisPrivacy       string   `json:"whoisPrivacy"`
	AutoRenew          int      `json:"autoRenew"`
	NotLocal           int      `json:"notLocal"`
	Labels             []string `json:"labels,omitempty"`
	Nameservers        []string `json:"ns,omitempty"`
	NameserversDefault bool     `json:"nsDefault"`
}

// ListDomainsResponse is the response from listing domains
type ListDomainsResponse struct {
	Status  string   `json:"status"`
	Domains []Domain `json:"domains"`
}

// ListDomains returns all domains in the account
func (c *Client) ListDomains(ctx context.Context) ([]Domain, error) {
	// POST /domain/listAll with credentials in body
	body, err := c.doPost(ctx, "/domain/listAll", nil)
	if err != nil {
		return nil, err
	}

	_, err = checkResponse(body)
	if err != nil {
		return nil, err
	}

	var resp ListDomainsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing domains response: %w", err)
	}

	return resp.Domains, nil
}

// GetDomainResponse is the response from getting a single domain
type GetDomainResponse struct {
	Status string `json:"status"`
	Domain Domain `json:"domain"`
}

// GetDomain returns details for a specific domain
func (c *Client) GetDomain(ctx context.Context, domain string) (*Domain, error) {
	path := fmt.Sprintf("/domain/get/%s", domain)

	// POST with credentials in body
	body, err := c.doPost(ctx, path, nil)
	if err != nil {
		return nil, err
	}

	_, err = checkResponse(body)
	if err != nil {
		return nil, err
	}

	var resp GetDomainResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing domain response: %w", err)
	}

	return &resp.Domain, nil
}

// IsDomainInAccount checks if a domain exists in the account
func (c *Client) IsDomainInAccount(ctx context.Context, domain string) (bool, error) {
	_, err := c.GetDomain(ctx, domain)
	if err != nil {
		if apiErr, ok := err.(*APIError); ok {
			if apiErr.Code == "DOMAIN_NOT_FOUND" {
				return false, nil
			}
		}
		return false, err
	}
	return true, nil
}

// SetNameservers updates the nameservers for a domain
func (c *Client) SetNameservers(ctx context.Context, domain string, nameservers []string) error {
	path := fmt.Sprintf("/domain/updateNs/%s", domain)

	body, err := c.doPost(ctx, path, map[string]interface{}{
		"ns": nameservers,
	})
	if err != nil {
		return err
	}

	_, err = checkResponse(body)
	return err
}

// GetNameservers returns the current nameservers for a domain
func (c *Client) GetNameservers(ctx context.Context, domain string) ([]string, error) {
	path := fmt.Sprintf("/domain/getNs/%s", domain)

	body, err := c.doPost(ctx, path, nil)
	if err != nil {
		return nil, err
	}

	_, err = checkResponse(body)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Status string   `json:"status"`
		NS     []string `json:"ns"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing nameservers response: %w", err)
	}

	return resp.NS, nil
}
