package protection

// GetDefaultSettings returns sensible defaults for new links
func GetDefaultSettings() Settings {
	return Settings{
		CountryMode:      "all",
		CountryList:      []string{},
		ASNMode:          "all",
		ASNList:          []string{},
		DeviceMode:       "all",
		DeviceList:       []string{},
		BlockBots:        true,
		BlockTor:         true,
		BlockProxy:       true,
		BlockDatacenter:  true,
		BlockHeadless:    true,
		MinBehaviorScore: 50,
		RedirectOnBlock:  "",
	}
}
