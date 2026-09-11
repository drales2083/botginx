package protection

// GetDefaultSettings returns sensible defaults for new links.
// Philosophy: challenge over block - Tor/VPN/datacenter users see challenge, not block.
func GetDefaultSettings() Settings {
	return Settings{
		Template:         "cloudflare",
		ThemeMode:        "auto",
		CountryMode:      "",
		CountryList:      []string{},
		ASNMode:          "",
		ASNList:          []string{},
		DeviceMode:       "",
		DeviceList:       []string{},
		BlockBots:        true,
		BlockTor:         false,
		BlockProxy:       false,
		BlockDatacenter:  false,
		BlockHeadless:    false,
		MinBehaviorScore: 0,
		RedirectOnBlock:  "",
	}
}
