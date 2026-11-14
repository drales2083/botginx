package cloudflare

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// DNSRecord represents a DNS record
type DNSRecord struct {
	ID       string `json:"id"`
	ZoneID   string `json:"zone_id"`
	ZoneName string `json:"zone_name"`
	Name     string `json:"name"`               // full name: subdomain.domain.com
	Type     string `json:"type"`               // A, AAAA, CNAME, TXT, MX
	Content  string `json:"content"`            // value (IP, target, etc.)
	TTL      int    `json:"ttl"`                // 1 = automatic
	Proxied  bool   `json:"proxied"`            // orange cloud on/off
	Priority int    `json:"priority,omitempty"` // MX priority
}

// DNSRecordInput is the input for adding/updating a DNS record
type DNSRecordInput struct {
	Type     string `json:"type"`
	Name     string `json:"name"`    // subdomain or @ or full name
	Content  string `json:"content"` // IP or target
	TTL      int    `json:"ttl"`     // 1 = automatic
	Proxied  bool   `json:"proxied"`
	Priority int    `json:"priority,omitempty"`
}

// ListDNSRecords returns all DNS records for a zone
func (c *Client) ListDNSRecords(ctx context.Context, zoneID string) ([]DNSRecord, error) {
	var allRecords []DNSRecord
	page := 1
	perPage := 100

	for {
		path := fmt.Sprintf("/zones/%s/dns_records?page=%d&per_page=%d", zoneID, page, perPage)
		body, err := c.doRequest(ctx, "GET", path, nil)
		if err != nil {
			return nil, err
		}

		_, err = checkResponse(body)
		if err != nil {
			return nil, err
		}

		var resp struct {
			Result     []DNSRecord `json:"result"`
			ResultInfo struct {
				Page       int `json:"page"`
				TotalPages int `json:"total_pages"`
			} `json:"result_info"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("parsing DNS records response: %w", err)
		}

		allRecords = append(allRecords, resp.Result...)

		if page >= resp.ResultInfo.TotalPages {
			break
		}
		page++
	}

	return allRecords, nil
}

// AddDNSRecord creates a new DNS record
func (c *Client) AddDNSRecord(ctx context.Context, zoneID string, input DNSRecordInput) (string, error) {
	path := fmt.Sprintf("/zones/%s/dns_records", zoneID)

	body, err := c.doRequest(ctx, "POST", path, input)
	if err != nil {
		return "", err
	}

	resp, err := checkResponse(body)
	if err != nil {
		return "", err
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("parsing create response: %w", err)
	}

	return result.ID, nil
}

// UpdateDNSRecord updates an existing DNS record
func (c *Client) UpdateDNSRecord(ctx context.Context, zoneID, recordID string, input DNSRecordInput) error {
	path := fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, recordID)

	body, err := c.doRequest(ctx, "PATCH", path, input)
	if err != nil {
		return err
	}

	_, err = checkResponse(body)
	return err
}

// DeleteDNSRecord deletes a DNS record
func (c *Client) DeleteDNSRecord(ctx context.Context, zoneID, recordID string) error {
	path := fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, recordID)

	body, err := c.doRequest(ctx, "DELETE", path, nil)
	if err != nil {
		return err
	}

	_, err = checkResponse(body)
	return err
}

// SetupDNS configures A records for root (@) and wildcard (*)
// Returns the record IDs created
func (c *Client) SetupDNS(ctx context.Context, zoneID, zoneName, serverIP string) (rootID, wildcardID string, err error) {
	// First, list existing records to check for conflicts
	records, err := c.ListDNSRecords(ctx, zoneID)
	if err != nil {
		return "", "", fmt.Errorf("listing DNS records: %w", err)
	}

	// Find and delete existing A records for @ and *
	for _, r := range records {
		if r.Type == "A" {
			// Root record: name equals zone name
			if r.Name == zoneName {
				if err := c.DeleteDNSRecord(ctx, zoneID, r.ID); err != nil {
					return "", "", fmt.Errorf("deleting existing root A record: %w", err)
				}
			}
			// Wildcard record: name is *.zonename
			if r.Name == "*."+zoneName {
				if err := c.DeleteDNSRecord(ctx, zoneID, r.ID); err != nil {
					return "", "", fmt.Errorf("deleting existing wildcard A record: %w", err)
				}
			}
		}
	}

	// Add root A record (@)
	rootID, err = c.AddDNSRecord(ctx, zoneID, DNSRecordInput{
		Type:    "A",
		Name:    "@",
		Content: serverIP,
		TTL:     1, // automatic
		Proxied: false,
	})
	if err != nil {
		return "", "", fmt.Errorf("adding root A record: %w", err)
	}

	// Add wildcard A record (*)
	wildcardID, err = c.AddDNSRecord(ctx, zoneID, DNSRecordInput{
		Type:    "A",
		Name:    "*",
		Content: serverIP,
		TTL:     1,
		Proxied: false,
	})
	if err != nil {
		return rootID, "", fmt.Errorf("adding wildcard A record: %w", err)
	}

	return rootID, wildcardID, nil
}

// AddVerificationTXT adds the GuardBot verification TXT record
// Creates: _guardbot-verify.domain.com → token
func (c *Client) AddVerificationTXT(ctx context.Context, zoneID, zoneName, token string) (string, error) {
	// First check if it already exists and delete it
	records, err := c.ListDNSRecords(ctx, zoneID)
	if err == nil {
		for _, r := range records {
			if r.Type == "TXT" && r.Name == "_guardbot-verify."+zoneName {
				c.DeleteDNSRecord(ctx, zoneID, r.ID)
			}
		}
	}

	return c.AddDNSRecord(ctx, zoneID, DNSRecordInput{
		Type:    "TXT",
		Name:    "_guardbot-verify",
		Content: token,
		TTL:     1,
		Proxied: false,
	})
}

// RemoveVerificationTXT removes the GuardBot verification TXT record
func (c *Client) RemoveVerificationTXT(ctx context.Context, zoneID, zoneName string) error {
	records, err := c.ListDNSRecords(ctx, zoneID)
	if err != nil {
		return err
	}

	for _, r := range records {
		if r.Type == "TXT" && r.Name == "_guardbot-verify."+zoneName {
			return c.DeleteDNSRecord(ctx, zoneID, r.ID)
		}
	}

	return nil // Not found, nothing to delete
}

// AddAcmeDNSCname adds a CNAME record for acme-dns delegation
// Creates: _acme-challenge.domain.com → fulldomain (e.g., xyz.acme.pamach.xyz)
func (c *Client) AddAcmeDNSCname(ctx context.Context, zoneID, zoneName, fulldomain string) (string, error) {
	// First check if it already exists and delete it
	records, err := c.ListDNSRecords(ctx, zoneID)
	if err == nil {
		for _, r := range records {
			if r.Name == "_acme-challenge."+zoneName {
				c.DeleteDNSRecord(ctx, zoneID, r.ID)
			}
		}
	}

	// Remove trailing dot if present
	target := strings.TrimSuffix(fulldomain, ".")

	return c.AddDNSRecord(ctx, zoneID, DNSRecordInput{
		Type:    "CNAME",
		Name:    "_acme-challenge",
		Content: target,
		TTL:     1,
		Proxied: false,
	})
}
