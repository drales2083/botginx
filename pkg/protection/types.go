// Package protection provides shared protection settings types and UI components
// for bot filtering, geo filtering, and device filtering across all modules.
package protection

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// Settings holds all protection/filtering configuration
type Settings struct {
	// Challenge page appearance
	Template  string `json:"template"`  // "cloudflare", "humancheck", "humansecurity", "slidepuzzle", "smartpuzzle", "simple", "lobby"
	ThemeMode string `json:"themeMode"` // "auto", "light", "dark"

	// Geographic filtering
	CountryMode string   `json:"countryMode"` // "", "whitelist", "blacklist" (empty = disabled)
	CountryList []string `json:"countryList"` // ISO 3166-1 alpha-2 codes

	// ASN filtering
	ASNMode string   `json:"asnMode"` // "", "whitelist", "blacklist" (empty = disabled)
	ASNList []string `json:"asnList"` // ASN numbers as strings (e.g., "AS12345")

	// Device filtering
	DeviceMode string   `json:"deviceMode"` // "", "whitelist", "blacklist" (empty = disabled)
	DeviceList []string `json:"deviceList"` // "desktop", "mobile", "tablet"

	// Bot detection toggles
	BlockBots       bool `json:"blockBots"`
	BlockTor        bool `json:"blockTor"`
	BlockProxy      bool `json:"blockProxy"`
	BlockDatacenter bool `json:"blockDatacenter"`
	BlockHeadless   bool `json:"blockHeadless"`

	// Behavior scoring
	MinBehaviorScore int `json:"minBehaviorScore"` // 0-100, 0 = disabled

	// Block action
	RedirectOnBlock string `json:"redirectOnBlock"` // URL to redirect blocked visitors, empty = show block page
}

// Validate checks settings are valid
func (s *Settings) Validate() error {
	validModes := map[string]bool{"all": true, "whitelist": true, "blacklist": true, "": true}

	if !validModes[s.CountryMode] {
		return fmt.Errorf("invalid countryMode: %s", s.CountryMode)
	}
	if !validModes[s.ASNMode] {
		return fmt.Errorf("invalid asnMode: %s", s.ASNMode)
	}
	if !validModes[s.DeviceMode] {
		return fmt.Errorf("invalid deviceMode: %s", s.DeviceMode)
	}

	if s.MinBehaviorScore < 0 || s.MinBehaviorScore > 100 {
		return fmt.Errorf("minBehaviorScore must be 0-100, got %d", s.MinBehaviorScore)
	}

	for _, d := range s.DeviceList {
		if d != "desktop" && d != "mobile" && d != "tablet" {
			return fmt.Errorf("invalid device: %s", d)
		}
	}

	return nil
}

// Value implements driver.Valuer for database storage
func (s Settings) Value() (driver.Value, error) {
	return json.Marshal(s)
}

// Scan implements sql.Scanner for database retrieval
func (s *Settings) Scan(value interface{}) error {
	if value == nil {
		*s = Settings{}
		return nil
	}

	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("cannot scan %T into Settings", value)
	}

	return json.Unmarshal(data, s)
}

// MarshalJSON ensures empty slices are marshaled as [] not null
func (s Settings) MarshalJSON() ([]byte, error) {
	type Alias Settings
	a := Alias(s)
	if a.CountryList == nil {
		a.CountryList = []string{}
	}
	if a.ASNList == nil {
		a.ASNList = []string{}
	}
	if a.DeviceList == nil {
		a.DeviceList = []string{}
	}
	return json.Marshal(a)
}
