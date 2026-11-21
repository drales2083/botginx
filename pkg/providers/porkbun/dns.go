package porkbun

import (
	"context"
	"encoding/json"
	"fmt"
)

// DNSRecord represents a DNS record
type DNSRecord struct {
	ID      string `json:"id"`
	Name    string `json:"name"`    // subdomain, blank for apex
	Type    string `json:"type"`    // A, AAAA, CNAME, TXT, MX
	Content string `json:"content"` // IP, target, etc.
	TTL     int    `json:"ttl"`
	Prio    int    `json:"prio,omitempty"` // MX/SRV priority
	Notes   string `json:"notes,omitempty"`
}

// DNSRecordInput is the input for creating/updating a DNS record
type DNSRecordInput struct {
	Type    string `json:"type"`
	Name    string `json:"name"`    // subdomain, * for wildcard, blank for apex
	Content string `json:"content"` // IP or target
	TTL     int    `json:"ttl,omitempty"`
	Prio    int    `json:"prio,omitempty"`
	Notes   string `json:"notes,omitempty"`
}

// ListDNSRecordsResponse is the response from listing DNS records
type ListDNSRecordsResponse struct {
	Status     string      `json:"status"`
	Cloudflare string      `json:"cloudflare,omitempty"`
	Records    []DNSRecord `json:"records"`
}

// ListDNSRecords returns all DNS records for a domain
func (c *Client) ListDNSRecords(ctx context.Context, domain string) ([]DNSRecord, error) {
	path := fmt.Sprintf("/dns/retrieve/%s", domain)

	body, err := c.doPost(ctx, path, nil)
	if err != nil {
		return nil, err
	}

	_, err = checkResponse(body)
	if err != nil {
		return nil, err
	}

	var resp ListDNSRecordsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing DNS records response: %w", err)
	}

	return resp.Records, nil
}

// CreateDNSRecordResponse is the response from creating a DNS record
type CreateDNSRecordResponse struct {
	Status   string   `json:"status"`
	ID       string   `json:"id"`
	Warnings []string `json:"warnings,omitempty"`
}

// CreateDNSRecord creates a new DNS record
func (c *Client) CreateDNSRecord(ctx context.Context, domain string, input DNSRecordInput) (string, error) {
	path := fmt.Sprintf("/dns/create/%s", domain)

	data := map[string]interface{}{
		"type":    input.Type,
		"content": input.Content,
	}
	if input.Name != "" {
		data["name"] = input.Name
	}
	if input.TTL > 0 {
		data["ttl"] = input.TTL
	}
	if input.Prio > 0 {
		data["prio"] = input.Prio
	}
	if input.Notes != "" {
		data["notes"] = input.Notes
	}

	body, err := c.doPost(ctx, path, data)
	if err != nil {
		return "", err
	}

	_, err = checkResponse(body)
	if err != nil {
		return "", err
	}

	var resp CreateDNSRecordResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("parsing create response: %w", err)
	}

	return resp.ID, nil
}

// DeleteDNSRecord deletes a DNS record by ID
func (c *Client) DeleteDNSRecord(ctx context.Context, domain, recordID string) error {
	path := fmt.Sprintf("/dns/delete/%s/%s", domain, recordID)

	body, err := c.doPost(ctx, path, nil)
	if err != nil {
		return err
	}

	_, err = checkResponse(body)
	return err
}

// DeleteDNSRecordByNameType deletes all DNS records matching name and type
func (c *Client) DeleteDNSRecordByNameType(ctx context.Context, domain, recordType, subdomain string) error {
	path := fmt.Sprintf("/dns/deleteByNameType/%s/%s/%s", domain, recordType, subdomain)

	body, err := c.doPost(ctx, path, nil)
	if err != nil {
		return err
	}

	_, err = checkResponse(body)
	return err
}

// SetupDNS configures A records for root (@) and wildcard (*)
// Deletes existing A records first to avoid conflicts
func (c *Client) SetupDNS(ctx context.Context, domain, serverIP string) (rootID, wildcardID string, err error) {
	// First, list existing records to find any A records to delete
	records, err := c.ListDNSRecords(ctx, domain)
	if err == nil {
		for _, r := range records {
			if r.Type == "A" && (r.Name == "" || r.Name == "*") {
				c.DeleteDNSRecord(ctx, domain, r.ID)
			}
		}
	}

	// Add root A record (@)
	rootID, err = c.CreateDNSRecord(ctx, domain, DNSRecordInput{
		Type:    "A",
		Name:    "", // blank for apex
		Content: serverIP,
		TTL:     600,
	})
	if err != nil {
		return "", "", fmt.Errorf("adding root A record: %w", err)
	}

	// Add wildcard A record (*)
	wildcardID, err = c.CreateDNSRecord(ctx, domain, DNSRecordInput{
		Type:    "A",
		Name:    "*",
		Content: serverIP,
		TTL:     600,
	})
	if err != nil {
		return rootID, "", fmt.Errorf("adding wildcard A record: %w", err)
	}

	return rootID, wildcardID, nil
}

// AddAcmeDNSCname adds a CNAME record for acme-dns delegation
// Creates: _acme-challenge.domain → fulldomain (e.g., xyz.acme.pamach.xyz)
func (c *Client) AddAcmeDNSCname(ctx context.Context, domain, fulldomain string) (string, error) {
	// First check if it already exists and delete it
	records, err := c.ListDNSRecords(ctx, domain)
	if err == nil {
		for _, r := range records {
			if r.Type == "CNAME" && r.Name == "_acme-challenge" {
				c.DeleteDNSRecord(ctx, domain, r.ID)
			}
		}
	}

	return c.CreateDNSRecord(ctx, domain, DNSRecordInput{
		Type:    "CNAME",
		Name:    "_acme-challenge",
		Content: fulldomain,
		TTL:     600,
	})
}
