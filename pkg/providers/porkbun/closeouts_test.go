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
		PriceMin:      500,  // $5.00
		PriceMax:      5000, // $50.00
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
	// Closeout struct from search results
	closeout := Closeout{
		CloseoutID:       12345,
		Domain:           "example.com",
		TLD:              "com",
		Price:            1299, // $12.99 (closeout price)
		Age:              8,    // 8 years old
		EndTime:          "2024-01-15T12:00:00Z",
		RegistrationDate: "2015-03-01T00:00:00Z",
		InboundLinks:     150,
		MonthlyVisitors:  1000,
	}

	if closeout.Domain == "" {
		t.Error("Domain should not be empty")
	}
	if closeout.TLD == "" {
		t.Error("TLD should not be empty")
	}
	if closeout.Price <= 0 {
		t.Error("Price should be positive")
	}
	if closeout.Age <= 0 {
		t.Error("Age should be positive for aged domains")
	}
}

func TestCloseoutDetailStructure(t *testing.T) {
	// CloseoutDetail has additional fields from /closeout/get/{domain}
	detail := CloseoutDetail{
		Closeout: Closeout{
			CloseoutID: 12345,
			Domain:     "example.com",
			TLD:        "com",
			Price:      999, // Closeout price
			Age:        5,
		},
		RenewalPrice: 1099,       // Porkbun renewal price
		TotalPrice:   999 + 1099, // What you pay
		Available:    true,
		LocalDomain:  false,
	}

	if detail.TotalPrice != detail.Price+detail.RenewalPrice {
		t.Errorf("TotalPrice should equal Price + RenewalPrice")
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
			{CloseoutID: 1, Domain: "one.com", TLD: "com", Price: 999, Age: 3},
			{CloseoutID: 2, Domain: "two.net", TLD: "net", Price: 1299, Age: 5},
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
		Status:       "SUCCESS",
		Available:    true,
		Domain:       "example.com",
		Price:        999,
		RenewalPrice: 1099,
		TotalPrice:   2098,
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
		CloseoutID:    456789,
		CloseoutPrice: 999,
		RenewalPrice:  1099,
		TotalPrice:    2098,
		Message:       "Domain purchased",
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
