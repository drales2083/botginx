// pkg/customizer/defaults.go
package customizer

func GetDefaultCustomization() Customization {
	return Customization{
		BgColor:              "#0a0a0a",
		BgColorSecondary:     "#1a1a1a",
		GradientEnabled:      false,
		Pattern:              "none",
		PatternColor:         "#333333",
		PatternOpacity:       100,
		Loader:               "dots-bounce",
		LoaderColorPrimary:   "#4f46e5",
		LoaderColorSecondary: "#e5e7eb",
		Heading:              "Please Wait",
		HeadingVisible:       true,
		Subheading:           "We're redirecting you...",
		SubheadingVisible:    true,
		Font:                 "Montserrat",
		FontWeight:           700,
		TextColor:            "#ffffff",
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
