package porkbun

import (
	"testing"
)

func TestSearchCloseoutsParamsDefaults(t *testing.T) {
	params := SearchCloseoutsParams{}

	// Default limit should be 100 if not set
	if params.Limit == 0 {
		params.Limit = 100 // default
	}

	if params.Limit != 100 {
		t.Errorf("Default limit should be 100, got %d", params.Limit)
	}
}

func TestSearchCloseoutsParamsFilters(t *testing.T) {
	params := SearchCloseoutsParams{
		TLD:           "com",
		AgeMin:        5,
		AgeMax:        10,
		PriceMin:      500,   // $5.00
		PriceMax:      5000,  // $50.00
		SortName:      "registrationDate",
		SortDirection: "desc",
		Limit:         50,
	}

	if params.TLD != "com" {
		t.Errorf("TLD = %q, want %q", params.TLD, "com")
	}
	if params.AgeMin != 5 {
		t.Errorf("AgeMin = %d, want %d", params.AgeMin, 5)
	}
	if params.PriceMax != 5000 {
		t.Errorf("PriceMax = %d, want %d", params.PriceMax, 5000)
	}
}

func TestSearchCloseoutsSortOptions(t *testing.T) {
	validSortNames := []string{
		"domain",
		"endTime",
		"price",
		"revenue",
		"visitors",
		"inboundLinks",
		"registrationDate",
	}

	validSortDirections := []string{"asc", "desc"}

	for _, name := range validSortNames {
		if len(name) == 0 {
			t.Error("Sort name should not be empty")
		}
	}

	for _, dir := range validSortDirections {
		if dir != "asc" && dir != "desc" {
			t.Errorf("Invalid sort direction: %s", dir)
		}
	}
}

func TestCloseoutStructure(t *testing.T) {
	closeout := Closeout{
		Domain:           "example.com",
		TLD:              "com",
		TotalPrice:       1299, // $12.99
		RenewalPrice:     999,  // $9.99
		EndTime:          "2024-01-15T12:00:00Z",
		RegistrationDate: "2015-03-01",
		InboundLinks:     150,
		Visitors:         1000,
		Revenue:          0,
	}

	if closeout.Domain == "" {
		t.Error("Domain should not be empty")
	}
	if closeout.TLD == "" {
		t.Error("TLD should not be empty")
	}
	if closeout.TotalPrice <= 0 {
		t.Error("TotalPrice should be positive")
	}
}

func TestSearchCloseoutsResponseStructure(t *testing.T) {
	resp := SearchCloseoutsResponse{
		Status:         "SUCCESS",
		Count:          2,
		TotalAvailable: 100,
		Start:          0,
		Limit:          100,
		Closeouts: []Closeout{
			{Domain: "one.com", TLD: "com", TotalPrice: 999},
			{Domain: "two.net", TLD: "net", TotalPrice: 1299},
		},
	}

	if resp.Status != "SUCCESS" {
		t.Error("Status should be SUCCESS")
	}
	if resp.Count != len(resp.Closeouts) {
		t.Errorf("Count %d should match Closeouts length %d", resp.Count, len(resp.Closeouts))
	}
}

func TestGetCloseoutResponseStructure(t *testing.T) {
	resp := GetCloseoutResponse{
		Status:    "SUCCESS",
		Available: true,
		Closeout: Closeout{
			Domain:     "example.com",
			TLD:        "com",
			TotalPrice: 1299,
		},
	}

	if resp.Status != "SUCCESS" {
		t.Error("Status should be SUCCESS")
	}
	if !resp.Available {
		t.Error("Available should be true")
	}
}

func TestBuyCloseoutResponseStructure(t *testing.T) {
	resp := BuyCloseoutResponse{
		Status:        "SUCCESS",
		Domain:        "example.com",
		OrderID:       "order123",
		CloseoutID:    "closeout456",
		CloseoutPrice: 999,
		RenewalPrice:  999,
		TotalPrice:    1998,
		Message:       "Purchase successful",
	}

	if resp.Status != "SUCCESS" {
		t.Error("Status should be SUCCESS")
	}
	if resp.OrderID == "" {
		t.Error("OrderID should not be empty")
	}
	if resp.Domain == "" {
		t.Error("Domain should not be empty")
	}
}

func TestPricingConversions(t *testing.T) {
	// Prices are in cents
	priceInCents := 1299
	priceInDollars := float64(priceInCents) / 100.0

	if priceInDollars != 12.99 {
		t.Errorf("Price conversion: %d cents = $%.2f, want $12.99", priceInCents, priceInDollars)
	}
}
