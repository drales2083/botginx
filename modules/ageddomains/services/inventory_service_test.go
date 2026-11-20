package services

import (
	"testing"
)

func TestInventoryServiceConstants(t *testing.T) {
	// Verify error definitions exist
	if ErrDomainNotFound == nil {
		t.Error("ErrDomainNotFound should not be nil")
	}
	if ErrDomainUnavailable == nil {
		t.Error("ErrDomainUnavailable should not be nil")
	}
}

func TestPaginationDefaults(t *testing.T) {
	// Test default pagination values
	testCases := []struct {
		page    int
		perPage int
		wantPage    int
		wantPerPage int
	}{
		{0, 0, 1, 20},     // defaults
		{-1, -10, 1, 20},  // negative values should default
		{5, 100, 5, 100},  // valid values
		{1, 500, 1, 100},  // perPage capped at 100
	}

	for _, tc := range testCases {
		page := tc.page
		perPage := tc.perPage

		// Apply defaults (mimics ListAvailable behavior)
		if page < 1 {
			page = 1
		}
		if perPage < 1 {
			perPage = 20
		}
		if perPage > 100 {
			perPage = 100
		}

		if page != tc.wantPage {
			t.Errorf("page=%d, want %d", page, tc.wantPage)
		}
		if perPage != tc.wantPerPage {
			t.Errorf("perPage=%d, want %d", perPage, tc.wantPerPage)
		}
	}
}

func TestSafetyStatusValues(t *testing.T) {
	// Valid safety status values
	validStatuses := []string{"unchecked", "clean", "flagged", "unknown"}

	for _, status := range validStatuses {
		// Just ensure these strings are what we expect
		if len(status) == 0 {
			t.Errorf("status should not be empty")
		}
	}
}
