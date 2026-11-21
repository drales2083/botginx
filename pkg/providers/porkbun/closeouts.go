package porkbun

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

// Closeout represents a closeout domain listing from search results
type Closeout struct {
	CloseoutID           int    `json:"closeoutId"`
	Domain               string `json:"domain"`
	TLD                  string `json:"tld"`
	Price                int    `json:"price"`                // Closeout price in cents
	Age                  int    `json:"age"`                  // Domain age in years
	RegistrationDate     string `json:"registrationDate"`     // Original registration (ISO 8601)
	EndTime              string `json:"endTime"`              // When listing ends (ISO 8601)
	NameLength           int    `json:"nameLength"`           // SLD character count
	IsIDN                bool   `json:"isIdn"`                // Internationalized domain
	EstibotAppraisal     int    `json:"estibotAppraisal"`     // Third-party appraisal (cents)
	MonthlyVisitors      int    `json:"monthlyVisitors"`      // Traffic stat
	InboundLinks         int    `json:"inboundLinks"`         // Backlink count
	ExpiredRevenue       int    `json:"expiredRevenue"`       // Historical revenue
	OtherTLDCount        int    `json:"otherTldCount"`        // Same name in other TLDs
	ProviderRenewalPrice int    `json:"providerRenewalPrice"` // Provider's renewal figure
}

// CloseoutDetail has additional fields from /closeout/get/{domain}
type CloseoutDetail struct {
	Closeout
	RenewalPrice int  `json:"renewalPrice"` // Porkbun renewal price
	TotalPrice   int  `json:"totalPrice"`   // price + renewalPrice (what you pay)
	LocalDomain  bool `json:"localDomain"`  // Already at Porkbun
	Available    bool `json:"available"`    // Still available
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
// Uses GET with header authentication per Porkbun API docs
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
		q.Set("limit", "100")
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
	Status   string         `json:"status"`
	Closeout CloseoutDetail `json:"closeout,omitempty"`
	// Flat fields also present at root level
	Domain       string `json:"domain,omitempty"`
	Available    bool   `json:"available"`
	TotalPrice   int    `json:"totalPrice,omitempty"`
	RenewalPrice int    `json:"renewalPrice,omitempty"`
	Price        int    `json:"price,omitempty"`
}

// GetCloseout returns details for a specific closeout domain
// Uses GET with header authentication
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
	CloseoutID    int    `json:"closeoutId"`
	CloseoutPrice int    `json:"closeoutPrice"`
	RenewalPrice  int    `json:"renewalPrice"`
	TotalPrice    int    `json:"totalPrice"`
	Message       string `json:"message"`
}

// BuyCloseout purchases a closeout domain
// cost must be the exact totalPrice in cents from GetCloseout
// Uses POST with body authentication
func (c *Client) BuyCloseout(ctx context.Context, domain string, costCents int) (*BuyCloseoutResponse, error) {
	path := fmt.Sprintf("/closeout/buy/%s", domain)

	body, err := c.doPost(ctx, path, map[string]interface{}{
		"cost": costCents,
	})
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

// ListAgedDomains returns all closeout domains matching criteria
// Fetches all pages for the given TLDs
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
			if start+resp.Count >= resp.TotalAvailable || len(resp.Closeouts) == 0 {
				break
			}
			start += resp.Count
		}
	}

	return allDomains, nil
}
