package namesilo

import (
	"context"
	"encoding/json"
	"net/url"
	"time"
)

// Domain represents a domain from the NameSilo account
type Domain struct {
	Domain    string     `json:"domain"`
	Created   *time.Time `json:"created,omitempty"`
	Expires   *time.Time `json:"expires,omitempty"`
	Status    string     `json:"status"`
	Locked    bool       `json:"locked"`
	Private   bool       `json:"private"`
	AutoRenew bool       `json:"auto_renew"`
}

// listDomainsResponse is the response from listDomains
type listDomainsResponse struct {
	Reply struct {
		Code    int         `json:"code"`
		Detail  string      `json:"detail"`
		Domains interface{} `json:"domains"` // can be object with domain field, or array
	} `json:"reply"`
}

// getDomainInfoResponse is the response from getDomainInfo
type getDomainInfoResponse struct {
	Reply struct {
		Code      int    `json:"code"`
		Detail    string `json:"detail"`
		Created   string `json:"created"`
		Expires   string `json:"expires"`
		Status    string `json:"status"`
		Locked    string `json:"locked"`    // "Yes" or "No"
		Private   string `json:"private"`   // "Yes" or "No"
		AutoRenew string `json:"auto_renew"` // "Yes" or "No"
	} `json:"reply"`
}

// ListDomains returns all domains in the NameSilo account
func (c *Client) ListDomains(ctx context.Context) ([]string, error) {
	body, err := c.doRequest(ctx, "listDomains", nil)
	if err != nil {
		return nil, err
	}

	var resp listDomainsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	if resp.Reply.Code != 300 {
		return nil, &APIError{Code: resp.Reply.Code, Message: resp.Reply.Detail}
	}

	// Handle various response formats from NameSilo API
	var domains []string
	switch v := resp.Reply.Domains.(type) {
	case map[string]interface{}:
		// Format: {"domains": {"domain": "example.com"}} or {"domains": {"domain": ["a.com", "b.com"]}}
		if domainField, ok := v["domain"]; ok {
			switch d := domainField.(type) {
			case string:
				if d != "" {
					domains = []string{d}
				}
			case []interface{}:
				for _, item := range d {
					if s, ok := item.(string); ok {
						domains = append(domains, s)
					}
				}
			}
		}
	case []interface{}:
		// Format: {"domains": [{"domain": "a.com", ...}, {"domain": "b.com", ...}]}
		// or: {"domains": ["example.com", "example2.com"]}
		for _, d := range v {
			switch item := d.(type) {
			case string:
				// Direct string in array
				domains = append(domains, item)
			case map[string]interface{}:
				// Object with domain field
				if domainName, ok := item["domain"].(string); ok && domainName != "" {
					domains = append(domains, domainName)
				}
			}
		}
	case string:
		// Format: {"domains": "example.com"}
		if v != "" {
			domains = []string{v}
		}
	}

	return domains, nil
}

// GetDomainInfo returns detailed information about a specific domain
func (c *Client) GetDomainInfo(ctx context.Context, domain string) (*Domain, error) {
	params := url.Values{}
	params.Set("domain", domain)

	body, err := c.doRequest(ctx, "getDomainInfo", params)
	if err != nil {
		return nil, err
	}

	var resp getDomainInfoResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	if resp.Reply.Code != 300 {
		return nil, &APIError{Code: resp.Reply.Code, Message: resp.Reply.Detail}
	}

	// Parse dates
	var created, expires *time.Time
	if resp.Reply.Created != "" {
		if t, err := time.Parse("2006-01-02", resp.Reply.Created); err == nil {
			created = &t
		}
	}
	if resp.Reply.Expires != "" {
		if t, err := time.Parse("2006-01-02", resp.Reply.Expires); err == nil {
			expires = &t
		}
	}

	return &Domain{
		Domain:    domain,
		Created:   created,
		Expires:   expires,
		Status:    resp.Reply.Status,
		Locked:    resp.Reply.Locked == "Yes",
		Private:   resp.Reply.Private == "Yes",
		AutoRenew: resp.Reply.AutoRenew == "Yes",
	}, nil
}

// TestConnection tests if the API key is valid by listing domains
func (c *Client) TestConnection(ctx context.Context) error {
	_, err := c.ListDomains(ctx)
	return err
}
