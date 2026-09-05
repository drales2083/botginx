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
		BlockTor:         false,
		BlockProxy:       false,
		BlockDatacenter:  false,
		BlockHeadless:    true,
		MinBehaviorScore: 0,
		RedirectOnBlock:  "",
	}
}
