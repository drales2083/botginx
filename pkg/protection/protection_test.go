package protection

import (
	"encoding/json"
	"testing"
)

func TestSettingsValidation(t *testing.T) {
	s := GetDefaultSettings()
	if err := s.Validate(); err != nil {
		t.Errorf("default settings should be valid: %v", err)
	}

	s.CountryMode = "invalid"
	if err := s.Validate(); err == nil {
		t.Error("invalid countryMode should fail validation")
	}

	s = GetDefaultSettings()
	s.DeviceMode = "whitelist"
	s.DeviceList = []string{"desktop", "invalid"}
	if err := s.Validate(); err == nil {
		t.Error("invalid device should fail validation")
	}

	s = GetDefaultSettings()
	s.MinBehaviorScore = 101
	if err := s.Validate(); err == nil {
		t.Error("score > 100 should fail validation")
	}
}

func TestSettingsJSON(t *testing.T) {
	s := GetDefaultSettings()
	s.CountryList = []string{"US", "CA"}
	s.BlockBots = true

	val, err := s.Value()
	if err != nil {
		t.Errorf("Value() failed: %v", err)
	}

	var s2 Settings
	if err := s2.Scan(val); err != nil {
		t.Errorf("Scan() failed: %v", err)
	}

	if s2.BlockBots != s.BlockBots {
		t.Error("round-trip failed for BlockBots")
	}
	if len(s2.CountryList) != 2 {
		t.Errorf("round-trip failed for CountryList: got %d, want 2", len(s2.CountryList))
	}
}

func TestSettingsJSONEmptySlices(t *testing.T) {
	s := Settings{
		CountryMode: "all",
		// Leave slices nil
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Errorf("Marshal failed: %v", err)
	}

	// Verify empty arrays are [] not null
	var m map[string]interface{}
	json.Unmarshal(data, &m)

	if m["countryList"] == nil {
		t.Error("countryList should be [] not null")
	}
}

func TestGetCountries(t *testing.T) {
	countries := GetCountries()
	if len(countries) == 0 {
		t.Error("GetCountries should return countries")
	}

	// Check US is in the list
	found := false
	for _, c := range countries {
		if c.Code == "US" {
			found = true
			break
		}
	}
	if !found {
		t.Error("US should be in countries list")
	}
}

func TestGetDefaultSettings(t *testing.T) {
	s := GetDefaultSettings()
	if s.CountryMode != "" {
		t.Errorf("default CountryMode should be empty (disabled), got %s", s.CountryMode)
	}
	if !s.BlockBots {
		t.Error("default BlockBots should be true")
	}
	if s.BlockHeadless {
		t.Error("default BlockHeadless should be false")
	}
	if s.BlockTor {
		t.Error("default BlockTor should be false")
	}
}
