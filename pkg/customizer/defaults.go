// pkg/customizer/defaults.go
package customizer

func GetDefaultCustomization() Customization {
	return Customization{
		BgColor:              "#ffffff",
		BgColorSecondary:     "#f5f5f5",
		GradientEnabled:      false,
		Pattern:              "none",
		PatternColor:         "#e0e0e0",
		PatternOpacity:       100,
		Loader:               "none",
		LoaderColorPrimary:   "#4f46e5",
		LoaderColorSecondary: "#c7d2fe",
		Heading:              "Please Wait",
		HeadingVisible:       true,
		Subheading:           "We're redirecting you...",
		SubheadingVisible:    true,
		Font:                 "montserrat",
		FontWeight:           700,
		TextColor:            "#1a1a1a",
		TextSize:             36,
		TextShadow:           false,
		ImageMode:            "none",
		ImageSize:            150,
		ImageOverlay:         90,
		ContentOrder:         "loader-text",
		TextAlign:            "center",
		VPos:                 0,
		Gap:                  30,
		PageTitle:            "Loading...",
	}
}
