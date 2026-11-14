package cloudflare

import (
	"context"
	"encoding/json"
	"fmt"
)

// Zone represents a Cloudflare zone (domain)
type Zone struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Status          string   `json:"status"` // active, pending, initializing, moved, deleted
	Paused          bool     `json:"paused"`
	Type            string   `json:"type"` // full, partial
	NameServers     []string `json:"name_servers"`
	OriginalNS      []string `json:"original_name_servers"`
	ActivatedOn     string   `json:"activated_on"`
	ModifiedOn      string   `json:"modified_on"`
	DevelopmentMode int      `json:"development_mode"`
}

// zonesResponse is the response from listing zones
type zonesResponse struct {
	Result     []Zone `json:"result"`
	ResultInfo struct {
		Page       int `json:"page"`
		PerPage    int `json:"per_page"`
		TotalPages int `json:"total_pages"`
		Count      int `json:"count"`
		TotalCount int `json:"total_count"`
	} `json:"result_info"`
}

// ListZones returns all zones in the account
func (c *Client) ListZones(ctx context.Context) ([]Zone, error) {
	var allZones []Zone
	page := 1
	perPage := 50

	for {
		path := fmt.Sprintf("/zones?page=%d&per_page=%d", page, perPage)
		body, err := c.doRequest(ctx, "GET", path, nil)
		if err != nil {
			return nil, err
		}

		_, err = checkResponse(body)
		if err != nil {
			return nil, err
		}

		var resp zonesResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("parsing zones response: %w", err)
		}

		allZones = append(allZones, resp.Result...)

		// Check if more pages
		if page >= resp.ResultInfo.TotalPages {
			break
		}
		page++
	}

	return allZones, nil
}

// GetZone returns details for a specific zone
func (c *Client) GetZone(ctx context.Context, zoneID string) (*Zone, error) {
	path := fmt.Sprintf("/zones/%s", zoneID)
	body, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	_, err = checkResponse(body)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Result Zone `json:"result"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing zone response: %w", err)
	}

	return &resp.Result, nil
}

// VerifyCredentials checks if the API credentials are valid by listing zones
func (c *Client) VerifyCredentials(ctx context.Context) error {
	path := "/zones?page=1&per_page=1"
	body, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return err
	}

	_, err = checkResponse(body)
	return err
}
