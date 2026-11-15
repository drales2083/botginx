package godaddy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// DNSRecord represents a DNS record
type DNSRecord struct {
	Type     string `json:"type"`              // A, AAAA, CNAME, TXT, MX, NS, SRV
	Name     string `json:"name"`              // subdomain or @ for root
	Data     string `json:"data"`              // value (IP, target, etc.)
	TTL      int    `json:"ttl"`               // 600-86400
	Priority int    `json:"priority,omitempty"` // MX priority
}

// ListDNSRecords returns all DNS records for a domain
func (c *Client) ListDNSRecords(ctx context.Context, domain string) ([]DNSRecord, error) {
	path := fmt.Sprintf("/v1/domains/%s/records", domain)
	body, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var records []DNSRecord
	if err := json.Unmarshal(body, &records); err != nil {
		return nil, fmt.Errorf("parsing DNS records response: %w", err)
	}

	return records, nil
}

// AddDNSRecords adds DNS records (PATCH appends to existing)
func (c *Client) AddDNSRecords(ctx context.Context, domain string, records []DNSRecord) error {
	path := fmt.Sprintf("/v1/domains/%s/records", domain)
	_, err := c.doRequest(ctx, "PATCH", path, records)
	return err
}

// ReplaceDNSRecords replaces all records of a given type and name
func (c *Client) ReplaceDNSRecords(ctx context.Context, domain, recordType, name string, records []DNSRecord) error {
	path := fmt.Sprintf("/v1/domains/%s/records/%s/%s", domain, recordType, name)
	_, err := c.doRequest(ctx, "PUT", path, records)
	return err
}

// DeleteDNSRecords deletes all records of a given type and name
func (c *Client) DeleteDNSRecords(ctx context.Context, domain, recordType, name string) error {
	path := fmt.Sprintf("/v1/domains/%s/records/%s/%s", domain, recordType, name)
	_, err := c.doRequest(ctx, "DELETE", path, nil)
	return err
}

// SetupDNS configures A records for root (@) and wildcard (*)
func (c *Client) SetupDNS(ctx context.Context, domain, serverIP string) error {
	// Replace root A record (@)
	rootRecords := []DNSRecord{{
		Type: "A",
		Name: "@",
		Data: serverIP,
		TTL:  600,
	}}
	if err := c.ReplaceDNSRecords(ctx, domain, "A", "@", rootRecords); err != nil {
		return fmt.Errorf("setting root A record: %w", err)
	}

	// Replace wildcard A record (*)
	wildcardRecords := []DNSRecord{{
		Type: "A",
		Name: "*",
		Data: serverIP,
		TTL:  600,
	}}
	if err := c.ReplaceDNSRecords(ctx, domain, "A", "*", wildcardRecords); err != nil {
		return fmt.Errorf("setting wildcard A record: %w", err)
	}

	return nil
}

// AddVerificationTXT adds the GuardBot verification TXT record
// Creates: _guardbot-verify → token
func (c *Client) AddVerificationTXT(ctx context.Context, domain, token string) error {
	// First try to delete existing
	c.DeleteDNSRecords(ctx, domain, "TXT", "_guardbot-verify")

	records := []DNSRecord{{
		Type: "TXT",
		Name: "_guardbot-verify",
		Data: token,
		TTL:  600,
	}}
	return c.AddDNSRecords(ctx, domain, records)
}

// AddAcmeDNSCname adds a CNAME record for acme-dns delegation
// Creates: _acme-challenge → fulldomain
func (c *Client) AddAcmeDNSCname(ctx context.Context, domain, fulldomain string) error {
	// Delete existing _acme-challenge records (TXT or CNAME)
	c.DeleteDNSRecords(ctx, domain, "CNAME", "_acme-challenge")
	c.DeleteDNSRecords(ctx, domain, "TXT", "_acme-challenge")

	// Remove trailing dot if present
	target := strings.TrimSuffix(fulldomain, ".")

	records := []DNSRecord{{
		Type: "CNAME",
		Name: "_acme-challenge",
		Data: target,
		TTL:  600,
	}}
	return c.AddDNSRecords(ctx, domain, records)
}

// AddTXTRecord adds a TXT record (for SSL challenges)
func (c *Client) AddTXTRecord(ctx context.Context, domain, name, value string) error {
	records := []DNSRecord{{
		Type: "TXT",
		Name: name,
		Data: value,
		TTL:  600,
	}}
	return c.AddDNSRecords(ctx, domain, records)
}
