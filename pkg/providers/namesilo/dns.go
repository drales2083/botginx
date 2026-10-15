package namesilo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// DNSRecord represents a DNS record
type DNSRecord struct {
	RecordID string `json:"record_id"`
	Type     string `json:"type"`
	Host     string `json:"host"`
	Value    string `json:"value"`
	TTL      int    `json:"ttl"`
	Distance int    `json:"distance,omitempty"` // MX priority
}

// DNSRecordInput is the input for adding/updating a DNS record
type DNSRecordInput struct {
	Type     string // A, AAAA, CNAME, MX, TXT
	Host     string // subdomain or @ for root
	Value    string // IP or target
	TTL      int    // seconds (default 3600)
	Distance int    // MX priority (optional)
}

// dnsListResponse is the response from dnsListRecords
type dnsListResponse struct {
	Reply struct {
		Code          int    `json:"code"`
		Detail        string `json:"detail"`
		ResourceRecord interface{} `json:"resource_record"` // can be single object or array
	} `json:"reply"`
}

// dnsRecordItem is a single DNS record in the response
type dnsRecordItem struct {
	RecordID string `json:"record_id"`
	Type     string `json:"type"`
	Host     string `json:"host"`
	Value    string `json:"value"`
	TTL      int    `json:"ttl"`
	Distance int    `json:"distance"`
}

// dnsAddResponse is the response from dnsAddRecord
type dnsAddResponse struct {
	Reply struct {
		Code     int    `json:"code"`
		Detail   string `json:"detail"`
		RecordID string `json:"record_id"`
	} `json:"reply"`
}

// ListDNSRecords returns all DNS records for a domain
func (c *Client) ListDNSRecords(ctx context.Context, domain string) ([]DNSRecord, error) {
	params := url.Values{}
	params.Set("domain", domain)

	body, err := c.doRequest(ctx, "dnsListRecords", params)
	if err != nil {
		return nil, err
	}

	var resp dnsListResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	if resp.Reply.Code != 300 {
		return nil, &APIError{Code: resp.Reply.Code, Message: resp.Reply.Detail}
	}

	// Parse resource records - can be single object or array
	var records []DNSRecord

	switch v := resp.Reply.ResourceRecord.(type) {
	case map[string]interface{}:
		// Single record
		record := parseRecordMap(v)
		if record != nil {
			records = append(records, *record)
		}
	case []interface{}:
		// Multiple records
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				record := parseRecordMap(m)
				if record != nil {
					records = append(records, *record)
				}
			}
		}
	}

	return records, nil
}

func parseRecordMap(m map[string]interface{}) *DNSRecord {
	record := &DNSRecord{}

	if v, ok := m["record_id"].(string); ok {
		record.RecordID = v
	}
	if v, ok := m["type"].(string); ok {
		record.Type = v
	}
	if v, ok := m["host"].(string); ok {
		record.Host = v
	}
	if v, ok := m["value"].(string); ok {
		record.Value = v
	}
	if v, ok := m["ttl"].(float64); ok {
		record.TTL = int(v)
	} else if v, ok := m["ttl"].(string); ok {
		fmt.Sscanf(v, "%d", &record.TTL)
	}
	if v, ok := m["distance"].(float64); ok {
		record.Distance = int(v)
	} else if v, ok := m["distance"].(string); ok {
		fmt.Sscanf(v, "%d", &record.Distance)
	}

	return record
}

// AddDNSRecord adds a new DNS record
func (c *Client) AddDNSRecord(ctx context.Context, domain string, input DNSRecordInput) (string, error) {
	params := url.Values{}
	params.Set("domain", domain)
	params.Set("rrtype", input.Type)
	params.Set("rrhost", input.Host)
	params.Set("rrvalue", input.Value)

	ttl := input.TTL
	if ttl == 0 {
		ttl = 3600
	}
	params.Set("rrttl", fmt.Sprintf("%d", ttl))

	if input.Type == "MX" && input.Distance > 0 {
		params.Set("rrdistance", fmt.Sprintf("%d", input.Distance))
	}

	body, err := c.doRequest(ctx, "dnsAddRecord", params)
	if err != nil {
		return "", err
	}

	var resp dnsAddResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", err
	}

	if resp.Reply.Code != 300 && resp.Reply.Code != 301 {
		return "", &APIError{Code: resp.Reply.Code, Message: resp.Reply.Detail}
	}

	return resp.Reply.RecordID, nil
}

// UpdateDNSRecord updates an existing DNS record
func (c *Client) UpdateDNSRecord(ctx context.Context, domain, recordID string, input DNSRecordInput) error {
	params := url.Values{}
	params.Set("domain", domain)
	params.Set("rrid", recordID)
	params.Set("rrhost", input.Host)
	params.Set("rrvalue", input.Value)

	ttl := input.TTL
	if ttl == 0 {
		ttl = 3600
	}
	params.Set("rrttl", fmt.Sprintf("%d", ttl))

	if input.Type == "MX" && input.Distance > 0 {
		params.Set("rrdistance", fmt.Sprintf("%d", input.Distance))
	}

	body, err := c.doRequest(ctx, "dnsUpdateRecord", params)
	if err != nil {
		return err
	}

	return checkResponse(body)
}

// DeleteDNSRecord deletes a DNS record
func (c *Client) DeleteDNSRecord(ctx context.Context, domain, recordID string) error {
	params := url.Values{}
	params.Set("domain", domain)
	params.Set("rrid", recordID)

	body, err := c.doRequest(ctx, "dnsDeleteRecord", params)
	if err != nil {
		return err
	}

	return checkResponse(body)
}

// SetupDNS configures A records for root and wildcard
// Returns the record IDs created
func (c *Client) SetupDNS(ctx context.Context, domain, serverIP string) (rootID, wildcardID string, err error) {
	// First, list existing records to check for conflicts
	records, err := c.ListDNSRecords(ctx, domain)
	if err != nil {
		return "", "", fmt.Errorf("listing DNS records: %w", err)
	}

	// Find existing A records for @ and *
	for _, r := range records {
		if r.Type == "A" {
			if r.Host == "" || r.Host == "@" {
				// Delete existing root A record
				if err := c.DeleteDNSRecord(ctx, domain, r.RecordID); err != nil {
					return "", "", fmt.Errorf("deleting existing root A record: %w", err)
				}
			} else if r.Host == "*" {
				// Delete existing wildcard A record
				if err := c.DeleteDNSRecord(ctx, domain, r.RecordID); err != nil {
					return "", "", fmt.Errorf("deleting existing wildcard A record: %w", err)
				}
			}
		}
	}

	// Add root A record (@)
	rootID, err = c.AddDNSRecord(ctx, domain, DNSRecordInput{
		Type:  "A",
		Host:  "",
		Value: serverIP,
		TTL:   3600,
	})
	if err != nil {
		return "", "", fmt.Errorf("adding root A record: %w", err)
	}

	// Add wildcard A record (*)
	wildcardID, err = c.AddDNSRecord(ctx, domain, DNSRecordInput{
		Type:  "A",
		Host:  "*",
		Value: serverIP,
		TTL:   3600,
	})
	if err != nil {
		return rootID, "", fmt.Errorf("adding wildcard A record: %w", err)
	}

	return rootID, wildcardID, nil
}

// AddVerificationTXT adds the GuardBot verification TXT record
// Creates: _guardbot-verify.domain.com → token
func (c *Client) AddVerificationTXT(ctx context.Context, domain, token string) (string, error) {
	// First check if it already exists and delete it
	records, err := c.ListDNSRecords(ctx, domain)
	if err == nil {
		for _, r := range records {
			if r.Type == "TXT" && r.Host == "_guardbot-verify" {
				c.DeleteDNSRecord(ctx, domain, r.RecordID)
			}
		}
	}

	return c.AddDNSRecord(ctx, domain, DNSRecordInput{
		Type:  "TXT",
		Host:  "_guardbot-verify",
		Value: token,
		TTL:   3600,
	})
}

// RemoveVerificationTXT removes the GuardBot verification TXT record
func (c *Client) RemoveVerificationTXT(ctx context.Context, domain string) error {
	records, err := c.ListDNSRecords(ctx, domain)
	if err != nil {
		return err
	}

	for _, r := range records {
		if r.Type == "TXT" && r.Host == "_guardbot-verify" {
			return c.DeleteDNSRecord(ctx, domain, r.RecordID)
		}
	}

	return nil // Not found, nothing to delete
}

// AddAcmeDNSCname adds a CNAME record for acme-dns delegation
// Creates: _acme-challenge.domain.com → fulldomain (e.g., xyz.acme.pamach.xyz)
// This enables instant wildcard SSL generation via acme-dns
func (c *Client) AddAcmeDNSCname(ctx context.Context, domain, fulldomain string) (string, error) {
	// First check if it already exists and delete it
	records, err := c.ListDNSRecords(ctx, domain)
	if err == nil {
		for _, r := range records {
			if r.Host == "_acme-challenge" {
				c.DeleteDNSRecord(ctx, domain, r.RecordID)
			}
		}
	}

	// Add CNAME record pointing to acme-dns subdomain
	// Note: NameSilo requires CNAME value without trailing dot
	target := fulldomain
	if strings.HasSuffix(target, ".") {
		target = strings.TrimSuffix(target, ".")
	}

	return c.AddDNSRecord(ctx, domain, DNSRecordInput{
		Type:  "CNAME",
		Host:  "_acme-challenge",
		Value: target,
		TTL:   3600,
	})
}
