package customizer

import (
	"fmt"
	"strconv"
	"strings"
)

var fonts = []FontDef{
	// Stylish Display (5)
	{ID: "playfair-display", Name: "Playfair Display", Category: "Stylish Display", Weights: []int{400, 600, 700, 800}},
	{ID: "cinzel", Name: "Cinzel", Category: "Stylish Display", Weights: []int{400, 600, 700}},
	{ID: "abril-fatface", Name: "Abril Fatface", Category: "Stylish Display", Weights: []int{400}},
	{ID: "bodoni-moda", Name: "Bodoni Moda", Category: "Stylish Display", Weights: []int{400, 600, 700}},
	{ID: "cormorant-garamond", Name: "Cormorant Garamond", Category: "Stylish Display", Weights: []int{400, 600, 700}},

	// Bold Impact (5)
	{ID: "bebas-neue", Name: "Bebas Neue", Category: "Bold Impact", Weights: []int{400}},
	{ID: "anton", Name: "Anton", Category: "Bold Impact", Weights: []int{400}},
	{ID: "oswald", Name: "Oswald", Category: "Bold Impact", Weights: []int{400, 600, 700}},
	{ID: "righteous", Name: "Righteous", Category: "Bold Impact", Weights: []int{400}},
	{ID: "russo-one", Name: "Russo One", Category: "Bold Impact", Weights: []int{400}},

	// Script (4)
	{ID: "dancing-script", Name: "Dancing Script", Category: "Script", Weights: []int{400, 600, 700}},
	{ID: "pacifico", Name: "Pacifico", Category: "Script", Weights: []int{400}},
	{ID: "great-vibes", Name: "Great Vibes", Category: "Script", Weights: []int{400}},
	{ID: "lobster", Name: "Lobster", Category: "Script", Weights: []int{400}},

	// Tech (4)
	{ID: "orbitron", Name: "Orbitron", Category: "Tech", Weights: []int{400, 600, 700, 800}},
	{ID: "audiowide", Name: "Audiowide", Category: "Tech", Weights: []int{400}},
	{ID: "rajdhani", Name: "Rajdhani", Category: "Tech", Weights: []int{400, 600, 700}},
	{ID: "exo-2", Name: "Exo 2", Category: "Tech", Weights: []int{400, 600, 700, 800}},

	// Modern (5)
	{ID: "montserrat", Name: "Montserrat", Category: "Modern", Weights: []int{400, 600, 700, 800}},
	{ID: "poppins", Name: "Poppins", Category: "Modern", Weights: []int{400, 600, 700, 800}},
	{ID: "inter", Name: "Inter", Category: "Modern", Weights: []int{400, 600, 700, 800}},
	{ID: "space-grotesk", Name: "Space Grotesk", Category: "Modern", Weights: []int{400, 600, 700}},
	{ID: "dm-sans", Name: "DM Sans", Category: "Modern", Weights: []int{400, 600, 700}},

	// Rounded (3)
	{ID: "quicksand", Name: "Quicksand", Category: "Rounded", Weights: []int{400, 600, 700}},
	{ID: "comfortaa", Name: "Comfortaa", Category: "Rounded", Weights: []int{400, 600, 700}},
	{ID: "nunito", Name: "Nunito", Category: "Rounded", Weights: []int{400, 600, 700, 800}},

	// Monospace (2)
	{ID: "jetbrains-mono", Name: "JetBrains Mono", Category: "Monospace", Weights: []int{400, 600, 700}},
	{ID: "fira-code", Name: "Fira Code", Category: "Monospace", Weights: []int{400, 600, 700}},
}

// GetFonts returns all available font definitions grouped by category
func GetFonts() []FontDef {
	return fonts
}

// GetFontImportURL returns the Google Fonts CSS import URL for a font with specific weights
func GetFontImportURL(name string, weights []int) string {
	// Convert "Playfair Display" to "Playfair+Display"
	urlName := strings.ReplaceAll(name, " ", "+")

	// Format weights as "400;600;700"
	weightStrs := make([]string, len(weights))
	for i, w := range weights {
		weightStrs[i] = strconv.Itoa(w)
	}

	return fmt.Sprintf("https://fonts.googleapis.com/css2?family=%s:wght@%s&display=swap",
		urlName, strings.Join(weightStrs, ";"))
}
