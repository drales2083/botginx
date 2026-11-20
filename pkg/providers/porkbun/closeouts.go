package porkbun

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

// Closeout represents a closeout domain listing
type Closeout struct {
	Domain           string `json:"domain"`
	TLD              string `json:"tld"`
	TotalPrice       int    `json:"totalPrice"`       // Price in cents
	RenewalPrice     int    `json:"renewalPrice"`     // Renewal price in cents
	EndTime          string `json:"endTime"`          // When closeout expires
	RegistrationDate string `json:"registrationDate"` // Original registration date
	InboundLinks     int    `json:"inboundLinks"`
	Visitors         int    `json:"visitors"`
	Revenue          int    `json:"revenue"`
}

// SearchCloseoutsParams contains parameters for searching closeouts
type SearchCloseoutsParams struct {
	Query         string // Keyword search
	TLD           string // Filter by TLD (e.g., "com", "net")
	NameLength    int    // Exact SLD length
	AgeMin        int    // Minimum years
	AgeMax        int    // Maximum years
	PriceMin      int    // Minimum price in cents
	PriceMax      int    // Maximum price in cents
	SortName      string // domain, endTime, price, revenue, visitors, inboundLinks, registrationDate
	SortDirection string // asc, desc
	Start         int    // Pagination offset
	Limit         int    // Results per page (1-500, default 100)
}

// SearchCloseoutsResponse is the response from searching closeouts
type SearchCloseoutsResponse struct {
	Status         string     `json:"status"`
	Count          int        `json:"count"`
	TotalAvailable int        `json:"totalAvailable"`
	Start          int        `json:"start"`
	Limit          int        `json:"limit"`
	Closeouts      []Closeout `json:"closeouts"`
}

// SearchCloseouts searches for available closeout domains
func (c *Client) SearchCloseouts(ctx context.Context, params SearchCloseoutsParams) (*SearchCloseoutsResponse, error) {
	// Build query string
	q := url.Values{}
	if params.Query != "" {
		q.Set("query", params.Query)
	}
	if params.TLD != "" {
		q.Set("tld", params.TLD)
	}
	if params.NameLength > 0 {
		q.Set("nameLength", strconv.Itoa(params.NameLength))
	}
	if params.AgeMin > 0 {
		q.Set("ageMin", strconv.Itoa(params.AgeMin))
	}
	if params.AgeMax > 0 {
		q.Set("ageMax", strconv.Itoa(params.AgeMax))
	}
	if params.PriceMin > 0 {
		q.Set("priceMin", strconv.Itoa(params.PriceMin))
	}
	if params.PriceMax > 0 {
		q.Set("priceMax", strconv.Itoa(params.PriceMax))
	}
	if params.SortName != "" {
		q.Set("sortName", params.SortName)
	}
	if params.SortDirection != "" {
		q.Set("sortDirection", params.SortDirection)
	}
	if params.Start > 0 {
		q.Set("start", strconv.Itoa(params.Start))
	}
	if params.Limit > 0 {
		q.Set("limit", strconv.Itoa(params.Limit))
	} else {
		q.Set("limit", "100") // Default
	}

	path := "/closeout/search"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}

	body, err := c.doGet(ctx, path)
	if err != nil {
		return nil, err
	}

	_, err = checkResponse(body)
	if err != nil {
		return nil, err
	}

	var resp SearchCloseoutsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing closeouts response: %w", err)
	}

	return &resp, nil
}

// GetCloseoutResponse is the response from getting a single closeout
type GetCloseoutResponse struct {
	Status    string   `json:"status"`
	Available bool     `json:"available"`
	Closeout  Closeout `json:"closeout"`
}

// GetCloseout returns details for a specific closeout domain
func (c *Client) GetCloseout(ctx context.Context, domain string) (*GetCloseoutResponse, error) {
	path := fmt.Sprintf("/closeout/get/%s", domain)

	body, err := c.doGet(ctx, path)
	if err != nil {
		return nil, err
	}

	_, err = checkResponse(body)
	if err != nil {
		return nil, err
	}

	var resp GetCloseoutResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing closeout response: %w", err)
	}

	return &resp, nil
}

// BuyCloseoutResponse is the response from buying a closeout
type BuyCloseoutResponse struct {
	Status        string `json:"status"`
	Domain        string `json:"domain"`
	OrderID       string `json:"orderId"`
	CloseoutID    string `json:"closeoutId"`
	CloseoutPrice int    `json:"closeoutPrice"`
	RenewalPrice  int    `json:"renewalPrice"`
	TotalPrice    int    `json:"totalPrice"`
	Message       string `json:"message"`
}

// BuyCloseout purchases a closeout domain
// cost must be the exact totalPrice in cents from GetCloseout
func (c *Client) BuyCloseout(ctx context.Context, domain string, costCents int) (*BuyCloseoutResponse, error) {
	path := fmt.Sprintf("/closeout/buy/%s", domain)

	reqBody := map[string]interface{}{
		"cost": costCents,
	}

	body, err := c.doRequest(ctx, "POST", path, reqBody)
	if err != nil {
		return nil, err
	}

	_, err = checkResponse(body)
	if err != nil {
		return nil, err
	}

	var resp BuyCloseoutResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing buy response: %w", err)
	}

	return &resp, nil
}

// ListAgedDomains returns all closeout domains for inventory (implements DomainProvider interface)
// This fetches all pages of closeouts matching the given criteria
func (c *Client) ListAgedDomains(ctx context.Context, minAge int, maxPriceCents int, tlds []string) ([]Closeout, error) {
	var allDomains []Closeout

	for _, tld := range tlds {
		params := SearchCloseoutsParams{
			TLD:           tld,
			AgeMin:        minAge,
			PriceMax:      maxPriceCents,
			SortName:      "registrationDate",
			SortDirection: "desc",
			Limit:         500,
		}

		start := 0
		for {
			params.Start = start
			resp, err := c.SearchCloseouts(ctx, params)
			if err != nil {
				return nil, fmt.Errorf("searching closeouts for TLD %s: %w", tld, err)
			}

			allDomains = append(allDomains, resp.Closeouts...)

			// Check if more pages
			if start+resp.Count >= resp.TotalAvailable {
				break
			}
			start += resp.Count
		}
	}

	return allDomains, nil
}
