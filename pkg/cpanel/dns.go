package cpanel

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// GetZone fetches all DNS records for a domain
func (c *Client) GetZone(domain string) ([]ZoneRecord, error) {
	params := url.Values{}
	params.Set("domain", domain)

	body, err := c.doRequest(http.MethodGet, "/json-api/cpanel", params)
	if err != nil {
		return nil, err
	}

	// Parse the nested response structure
	var resp struct {
		CPanelResult struct {
			Data []struct {
				StatusMsg string                   `json:"statusmsg"`
				Record    []map[string]interface{} `json:"record"`
			} `json:"data"`
			Error string `json:"error,omitempty"`
		} `json:"cpanelresult"`
	}

	// Add API2 params for this specific call
	params.Set("cpanel_jsonapi_apiversion", "2")
	params.Set("cpanel_jsonapi_module", "ZoneEdit")
	params.Set("cpanel_jsonapi_func", "fetchzone")

	body, err = c.doRequest(http.MethodGet, "/json-api/cpanel", params)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, NewAPIError("parse", "failed to parse zone response", err)
	}

	if resp.CPanelResult.Error != "" {
		return nil, NewAPIError("fetchzone", resp.CPanelResult.Error, nil)
	}

	if len(resp.CPanelResult.Data) == 0 || len(resp.CPanelResult.Data[0].Record) == 0 {
		return nil, NewAPIError("fetchzone", "no DNS records found", nil)
	}

	// Records are in data[0].record
	rawRecords := resp.CPanelResult.Data[0].Record
	records := make([]ZoneRecord, 0, len(rawRecords))

	for _, item := range rawRecords {
		record := ZoneRecord{}

		if v, ok := item["line"].(float64); ok {
			record.Line = int(v)
		}
		if v, ok := item["name"].(string); ok {
			record.Name = strings.TrimSuffix(v, ".")
		}
		if v, ok := item["type"].(string); ok {
			record.Type = v
		}
		if v, ok := item["ttl"].(float64); ok {
			record.TTL = int(v)
		}
		if v, ok := item["class"].(string); ok {
			record.Class = v
		}
		if v, ok := item["address"].(string); ok {
			record.Address = v
		}
		if v, ok := item["cname"].(string); ok {
			record.CName = strings.TrimSuffix(v, ".")
		}
		if v, ok := item["txtdata"].(string); ok {
			record.TXTData = v
		}
		if v, ok := item["exchange"].(string); ok {
			record.Exchange = v
		}
		if v, ok := item["preference"].(float64); ok {
			record.Priority = int(v)
		}

		// Skip raw/comment lines
		if record.Type == ":RAW" || record.Type == "$TTL" || record.Type == "" {
			continue
		}

		records = append(records, record)
	}

	return records, nil
}

// GetTXTRecords returns all TXT records for a domain
func (c *Client) GetTXTRecords(domain string) ([]TXTRecord, error) {
	records, err := c.GetZone(domain)
	if err != nil {
		return nil, err
	}

	txtRecords := make([]TXTRecord, 0)
	for _, r := range records {
		if r.Type == "TXT" {
			txtRecords = append(txtRecords, TXTRecord{
				Name:  r.Name,
				Value: r.TXTData,
				Line:  r.Line,
			})
		}
	}

	return txtRecords, nil
}

// GetARecords returns all A records for a domain
func (c *Client) GetARecords(domain string) ([]ARecord, error) {
	records, err := c.GetZone(domain)
	if err != nil {
		return nil, err
	}

	aRecords := make([]ARecord, 0)
	for _, r := range records {
		if r.Type == "A" {
			aRecords = append(aRecords, ARecord{
				Name:    r.Name,
				Address: r.Address,
				Line:    r.Line,
			})
		}
	}

	return aRecords, nil
}

// FindARecord finds an A record by name
// e.g., FindARecord("example.com", "*") finds "*.example.com"
func (c *Client) FindARecord(domain, name string) (*ARecord, error) {
	records, err := c.GetARecords(domain)
	if err != nil {
		return nil, err
	}

	// Build the full name we're looking for
	var fullName string
	if name == "*" {
		fullName = "*." + strings.ToLower(domain)
	} else if name == "" || name == "@" {
		fullName = strings.ToLower(domain)
	} else {
		fullName = strings.ToLower(name + "." + domain)
	}

	for _, r := range records {
		if strings.ToLower(r.Name) == fullName {
			return &r, nil
		}
	}

	return nil, nil // Not found, but not an error
}

// GetCNAMERecords returns all CNAME records for a domain
func (c *Client) GetCNAMERecords(domain string) ([]CNAMERecord, error) {
	records, err := c.GetZone(domain)
	if err != nil {
		return nil, err
	}

	cnameRecords := make([]CNAMERecord, 0)
	for _, r := range records {
		if r.Type == "CNAME" {
			cnameRecords = append(cnameRecords, CNAMERecord{
				Name:   r.Name,
				Target: r.CName,
				Line:   r.Line,
			})
		}
	}

	return cnameRecords, nil
}

// FindCNAMERecord finds a CNAME record by name
func (c *Client) FindCNAMERecord(domain, name string) (*CNAMERecord, error) {
	records, err := c.GetCNAMERecords(domain)
	if err != nil {
		return nil, err
	}

	fullName := strings.ToLower(name + "." + domain)

	for _, r := range records {
		if strings.ToLower(r.Name) == fullName {
			return &r, nil
		}
	}

	return nil, nil
}

// FindTXTRecord finds a TXT record by name prefix
// e.g., FindTXTRecord("example.com", "_guardbot-verify") finds "_guardbot-verify.example.com"
func (c *Client) FindTXTRecord(domain, namePrefix string) (*TXTRecord, error) {
	records, err := c.GetTXTRecords(domain)
	if err != nil {
		return nil, err
	}

	// Build the full name we're looking for
	fullName := strings.ToLower(namePrefix + "." + domain)

	for _, r := range records {
		if strings.ToLower(r.Name) == fullName {
			return &r, nil
		}
	}

	return nil, nil // Not found, but not an error
}

// AddTXTRecord adds a TXT record to the domain's DNS zone
func (c *Client) AddTXTRecord(domain, name, value string) error {
	// First check if record already exists
	existing, err := c.FindTXTRecord(domain, name)
	if err != nil {
		return err
	}
	if existing != nil {
		if existing.Value == value {
			return nil // Already exists with same value
		}
		return fmt.Errorf("%w: %s.%s already exists with value %q", ErrRecordExists, name, domain, existing.Value)
	}

	params := url.Values{}
	params.Set("cpanel_jsonapi_apiversion", "2")
	params.Set("cpanel_jsonapi_module", "ZoneEdit")
	params.Set("cpanel_jsonapi_func", "add_zone_record")
	params.Set("domain", domain)
	params.Set("name", name)
	params.Set("type", "TXT")
	params.Set("txtdata", value)
	params.Set("ttl", "300") // 5 minute TTL for quick propagation

	body, err := c.doRequest(http.MethodGet, "/json-api/cpanel", params)
	if err != nil {
		return err
	}

	// Parse response: {"cpanelresult":{"data":[{"result":{"status":1,"statusmsg":"","newserial":123}}],...}}
	var resp struct {
		CPanelResult struct {
			Data []struct {
				Result struct {
					Status    int    `json:"status"`
					StatusMsg string `json:"statusmsg"`
					NewSerial int    `json:"newserial"`
				} `json:"result"`
			} `json:"data"`
			Error string `json:"error,omitempty"`
		} `json:"cpanelresult"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return NewAPIError("parse", "failed to parse add_zone_record response", err)
	}

	if resp.CPanelResult.Error != "" {
		return NewAPIError("add_zone_record", resp.CPanelResult.Error, ErrZoneEditFailed)
	}

	if len(resp.CPanelResult.Data) == 0 {
		return NewAPIError("add_zone_record", "no response data", ErrZoneEditFailed)
	}

	result := resp.CPanelResult.Data[0].Result
	if result.Status != 1 {
		errMsg := result.StatusMsg
		if errMsg == "" {
			errMsg = "failed to add TXT record"
		}
		return NewAPIError("add_zone_record", errMsg, ErrZoneEditFailed)
	}

	return nil
}

// RemoveTXTRecord removes a TXT record from the domain's DNS zone
func (c *Client) RemoveTXTRecord(domain, name string) error {
	// Find the record to get its line number
	record, err := c.FindTXTRecord(domain, name)
	if err != nil {
		return err
	}
	if record == nil {
		return nil // Already doesn't exist
	}

	return c.removeRecordByLine(domain, record.Line)
}

// removeRecordByLine removes a DNS record by its line number
func (c *Client) removeRecordByLine(domain string, line int) error {
	params := url.Values{}
	params.Set("cpanel_jsonapi_apiversion", "2")
	params.Set("cpanel_jsonapi_module", "ZoneEdit")
	params.Set("cpanel_jsonapi_func", "remove_zone_record")
	params.Set("domain", domain)
	params.Set("line", fmt.Sprintf("%d", line))

	body, err := c.doRequest(http.MethodGet, "/json-api/cpanel", params)
	if err != nil {
		return err
	}

	var resp struct {
		CPanelResult struct {
			Data []struct {
				Result struct {
					Status    int    `json:"status"`
					StatusMsg string `json:"statusmsg"`
				} `json:"result"`
			} `json:"data"`
			Error string `json:"error,omitempty"`
		} `json:"cpanelresult"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return NewAPIError("parse", "failed to parse remove_zone_record response", err)
	}

	if resp.CPanelResult.Error != "" {
		return NewAPIError("remove_zone_record", resp.CPanelResult.Error, ErrZoneEditFailed)
	}

	if len(resp.CPanelResult.Data) > 0 && resp.CPanelResult.Data[0].Result.Status != 1 {
		errMsg := resp.CPanelResult.Data[0].Result.StatusMsg
		if errMsg == "" {
			errMsg = "failed to remove DNS record"
		}
		return NewAPIError("remove_zone_record", errMsg, ErrZoneEditFailed)
	}

	return nil
}

// AddARecord adds an A record to the domain's DNS zone
func (c *Client) AddARecord(domain, name, ip string) error {
	params := url.Values{}
	params.Set("cpanel_jsonapi_apiversion", "2")
	params.Set("cpanel_jsonapi_module", "ZoneEdit")
	params.Set("cpanel_jsonapi_func", "add_zone_record")
	params.Set("domain", domain)
	params.Set("name", name)
	params.Set("type", "A")
	params.Set("address", ip)
	params.Set("ttl", "300")

	body, err := c.doRequest(http.MethodGet, "/json-api/cpanel", params)
	if err != nil {
		return err
	}

	var resp struct {
		CPanelResult struct {
			Data []struct {
				Result struct {
					Status    int    `json:"status"`
					StatusMsg string `json:"statusmsg"`
				} `json:"result"`
			} `json:"data"`
			Error string `json:"error,omitempty"`
		} `json:"cpanelresult"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return NewAPIError("parse", "failed to parse add_zone_record response", err)
	}

	if resp.CPanelResult.Error != "" {
		return NewAPIError("add_zone_record", resp.CPanelResult.Error, ErrZoneEditFailed)
	}

	if len(resp.CPanelResult.Data) > 0 && resp.CPanelResult.Data[0].Result.Status != 1 {
		errMsg := resp.CPanelResult.Data[0].Result.StatusMsg
		if errMsg == "" {
			errMsg = "failed to add A record"
		}
		return NewAPIError("add_zone_record", errMsg, ErrZoneEditFailed)
	}

	return nil
}

// UpdateOrAddARecord updates an A record if it exists with different IP, or adds it if not
func (c *Client) UpdateOrAddARecord(domain, name, ip string) error {
	existing, err := c.FindARecord(domain, name)
	if err != nil {
		return err
	}

	if existing != nil {
		if existing.Address == ip {
			return nil // Already correct
		}
		// Remove old record first
		if err := c.removeRecordByLine(domain, existing.Line); err != nil {
			return fmt.Errorf("failed to remove old A record: %w", err)
		}
	}

	// Add new record
	return c.AddARecord(domain, name, ip)
}

// UpdateOrAddTXTRecord updates a TXT record if it exists, or adds it if not
func (c *Client) UpdateOrAddTXTRecord(domain, name, value string) error {
	existing, err := c.FindTXTRecord(domain, name)
	if err != nil {
		return err
	}

	if existing != nil {
		if existing.Value == value {
			return nil // Already correct
		}
		// Remove old record first
		if err := c.removeRecordByLine(domain, existing.Line); err != nil {
			return fmt.Errorf("failed to remove old TXT record: %w", err)
		}
	}

	// Add new record
	return c.AddTXTRecord(domain, name, value)
}

// AddVerificationTXT adds the GuardBot verification TXT record
// Record name: _guardbot-verify.domain.com
func (c *Client) AddVerificationTXT(domain, token string) error {
	// Get base domain for DNS zone
	baseDomain, err := c.GetBaseDomain(domain)
	if err != nil {
		return err
	}

	return c.UpdateOrAddTXTRecord(baseDomain, "_guardbot-verify", token)
}

// RemoveVerificationTXT removes the GuardBot verification TXT record
func (c *Client) RemoveVerificationTXT(domain string) error {
	baseDomain, err := c.GetBaseDomain(domain)
	if err != nil {
		return err
	}

	return c.RemoveTXTRecord(baseDomain, "_guardbot-verify")
}

// AddAcmeChallengeTXT adds an ACME challenge TXT record for SSL
// Record name: _acme-challenge.domain.com
func (c *Client) AddAcmeChallengeTXT(domain, token string) error {
	baseDomain, err := c.GetBaseDomain(domain)
	if err != nil {
		return err
	}

	return c.UpdateOrAddTXTRecord(baseDomain, "_acme-challenge", token)
}

// RemoveAcmeChallengeTXT removes the ACME challenge TXT record
func (c *Client) RemoveAcmeChallengeTXT(domain string) error {
	baseDomain, err := c.GetBaseDomain(domain)
	if err != nil {
		return err
	}

	return c.RemoveTXTRecord(baseDomain, "_acme-challenge")
}

// AddCNAMERecord adds a CNAME record to the domain's DNS zone
func (c *Client) AddCNAMERecord(domain, name, target string) error {
	params := url.Values{}
	params.Set("cpanel_jsonapi_apiversion", "2")
	params.Set("cpanel_jsonapi_module", "ZoneEdit")
	params.Set("cpanel_jsonapi_func", "add_zone_record")
	params.Set("domain", domain)
	params.Set("name", name)
	params.Set("type", "CNAME")
	params.Set("cname", target)
	params.Set("ttl", "300")

	body, err := c.doRequest(http.MethodGet, "/json-api/cpanel", params)
	if err != nil {
		return err
	}

	var resp struct {
		CPanelResult struct {
			Data []struct {
				Result struct {
					Status    int    `json:"status"`
					StatusMsg string `json:"statusmsg"`
				} `json:"result"`
			} `json:"data"`
			Error string `json:"error,omitempty"`
		} `json:"cpanelresult"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return NewAPIError("parse", "failed to parse add_zone_record response", err)
	}

	if resp.CPanelResult.Error != "" {
		return NewAPIError("add_zone_record", resp.CPanelResult.Error, ErrZoneEditFailed)
	}

	if len(resp.CPanelResult.Data) > 0 && resp.CPanelResult.Data[0].Result.Status != 1 {
		errMsg := resp.CPanelResult.Data[0].Result.StatusMsg
		if errMsg == "" {
			errMsg = "failed to add CNAME record"
		}
		return NewAPIError("add_zone_record", errMsg, ErrZoneEditFailed)
	}

	return nil
}

// UpdateOrAddCNAMERecord updates a CNAME record if it exists with different target, or adds it if not
func (c *Client) UpdateOrAddCNAMERecord(domain, name, target string) error {
	existing, err := c.FindCNAMERecord(domain, name)
	if err != nil {
		return err
	}

	if existing != nil {
		if existing.Target == target {
			return nil // Already correct
		}
		// Remove old record first
		if err := c.removeRecordByLine(domain, existing.Line); err != nil {
			return fmt.Errorf("failed to remove old CNAME record: %w", err)
		}
	}

	// Add new record
	return c.AddCNAMERecord(domain, name, target)
}

// DebugZone prints all DNS records for debugging
func (c *Client) DebugZone(domain string) (string, error) {
	records, err := c.GetZone(domain)
	if err != nil {
		return "", err
	}

	data, _ := json.MarshalIndent(records, "", "  ")
	return string(data), nil
}
