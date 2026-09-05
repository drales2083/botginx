// pkg/customizer/patterns.go
package customizer

import (
	"encoding/base64"
	"fmt"
)

// patternData holds pattern metadata and SVG generator
type patternData struct {
	name    string
	svgFunc func(color string) string
}

// patterns contains all 21 pattern definitions
var patterns = map[string]patternData{
	// =============================================================================
	// GEOMETRIC PATTERNS
	// =============================================================================

	"grid": {
		name: "Grid",
		svgFunc: func(color string) string {
			return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="40" height="40"><path d="M40 0v40H0" fill="none" stroke="%s" stroke-width="1"/></svg>`, color)
		},
	},
	"dotGrid": {
		name: "Dot Grid",
		svgFunc: func(color string) string {
			return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20"><circle cx="10" cy="10" r="1.5" fill="%s"/></svg>`, color)
		},
	},
	"polkaDots": {
		name: "Polka Dots",
		svgFunc: func(color string) string {
			return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="30" height="30"><circle cx="15" cy="15" r="4" fill="%s"/></svg>`, color)
		},
	},
	"diagonalLines": {
		name: "Diagonal Lines",
		svgFunc: func(color string) string {
			return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><path d="M-1 1l12 12M-1 11l12-12" stroke="%s" stroke-width="1"/></svg>`, color)
		},
	},
	"hexagons": {
		name: "Hexagons",
		svgFunc: func(color string) string {
			return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="28" height="49"><path d="M14 0l14 8v16l-14 8L0 24V8zm0 33l14 8v8H0v-8z" fill="none" stroke="%s" stroke-width="1"/></svg>`, color)
		},
	},
	"diamonds": {
		name: "Diamonds",
		svgFunc: func(color string) string {
			return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24"><path d="M12 0l12 12-12 12L0 12z" fill="none" stroke="%s" stroke-width="1"/></svg>`, color)
		},
	},
	"triangles": {
		name: "Triangles",
		svgFunc: func(color string) string {
			return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="36" height="32"><path d="M18 0l18 32H0zM36 32l18 32H18z" fill="none" stroke="%s" stroke-width="1"/></svg>`, color)
		},
	},
	"boxes": {
		name: "Boxes",
		svgFunc: func(color string) string {
			return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="48" height="48"><rect x="4" y="4" width="40" height="40" fill="none" stroke="%s" stroke-width="1"/><rect x="12" y="12" width="24" height="24" fill="none" stroke="%s" stroke-width="1"/></svg>`, color, color)
		},
	},

	// =============================================================================
	// LINE PATTERNS
	// =============================================================================

	"waves": {
		name: "Waves",
		svgFunc: func(color string) string {
			return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="40" height="20"><path d="M0 10c10 0 10-8 20-8s10 8 20 8 10-8 20-8" fill="none" stroke="%s" stroke-width="1"/></svg>`, color)
		},
	},
	"wiggle": {
		name: "Wiggle",
		svgFunc: func(color string) string {
			return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="52" height="26"><path d="M0 13c8.5 0 8.5-8 17-8s8.5 8 17 8 8.5-8 17-8" fill="none" stroke="%s" stroke-width="1.5"/></svg>`, color)
		},
	},
	"zigzag": {
		name: "Zigzag",
		svgFunc: func(color string) string {
			return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="40" height="20"><polyline points="0,20 10,0 20,20 30,0 40,20" fill="none" stroke="%s" stroke-width="1"/></svg>`, color)
		},
	},
	"chevrons": {
		name: "Chevrons",
		svgFunc: func(color string) string {
			return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="28" height="28"><path d="M0 0l14 14L28 0M0 14l14 14 14-14" fill="none" stroke="%s" stroke-width="1"/></svg>`, color)
		},
	},

	// =============================================================================
	// DECORATIVE PATTERNS
	// =============================================================================

	"plus": {
		name: "Plus",
		svgFunc: func(color string) string {
			return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20"><path d="M10 4v12M4 10h12" stroke="%s" stroke-width="1.5"/></svg>`, color)
		},
	},
	"crosses": {
		name: "Crosses",
		svgFunc: func(color string) string {
			return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20"><path d="M4 4l12 12M16 4L4 16" stroke="%s" stroke-width="1.5"/></svg>`, color)
		},
	},
	"stars": {
		name: "Stars",
		svgFunc: func(color string) string {
			return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32"><path d="M16 4l2.5 7.5H26l-6 4.5 2.5 7.5-6.5-5-6.5 5 2.5-7.5-6-4.5h7.5z" fill="none" stroke="%s" stroke-width="1"/></svg>`, color)
		},
	},
	"brickWall": {
		name: "Brick Wall",
		svgFunc: func(color string) string {
			return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="42" height="22"><path d="M0 0h42M0 11h42M0 22h42M0 0v11M21 0v11M42 0v11M10.5 11v11M31.5 11v11" fill="none" stroke="%s" stroke-width="1"/></svg>`, color)
		},
	},

	// =============================================================================
	// ORGANIC & ABSTRACT PATTERNS
	// =============================================================================

	"topography": {
		name: "Topography",
		svgFunc: func(color string) string {
			return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="80" height="80"><path d="M0 40c10-5 20 5 30 0s20-10 30 0 20 10 20 0M0 60c10-5 20 5 30 0s20-10 30 0 20 10 20 0M0 20c10-5 20 5 30 0s20-10 30 0 20 10 20 0" fill="none" stroke="%s" stroke-width="1"/></svg>`, color)
		},
	},
	"jigsaw": {
		name: "Jigsaw",
		svgFunc: func(color string) string {
			return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="40" height="40"><path d="M0 0h15c0 5 5 5 5 0h20M0 40h15c0-5 5-5 5 0h20M0 0v15c5 0 5 5 0 5v20M40 0v15c-5 0-5 5 0 5v20" fill="none" stroke="%s" stroke-width="1"/></svg>`, color)
		},
	},
	"bubbles": {
		name: "Bubbles",
		svgFunc: func(color string) string {
			return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="50" height="50"><circle cx="10" cy="10" r="6" fill="none" stroke="%s" stroke-width="1"/><circle cx="35" cy="25" r="8" fill="none" stroke="%s" stroke-width="1"/><circle cx="15" cy="40" r="4" fill="none" stroke="%s" stroke-width="1"/><circle cx="40" cy="45" r="3" fill="none" stroke="%s" stroke-width="1"/></svg>`, color, color, color, color)
		},
	},
	"signal": {
		name: "Signal",
		svgFunc: func(color string) string {
			return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="40" height="40"><path d="M20 20m-5 0a5 5 0 1 0 10 0 5 5 0 1 0-10 0M20 20m-10 0a10 10 0 1 0 20 0 10 10 0 1 0-20 0M20 20m-15 0a15 15 0 1 0 30 0 15 15 0 1 0-30 0" fill="none" stroke="%s" stroke-width="1"/></svg>`, color)
		},
	},
	"circuitBoard": {
		name: "Circuit Board",
		svgFunc: func(color string) string {
			return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="50" height="50"><path d="M0 25h15M25 0v15M35 25h15M25 35v15" stroke="%s" stroke-width="1"/><circle cx="25" cy="25" r="3" fill="none" stroke="%s" stroke-width="1"/><circle cx="15" cy="25" r="2" fill="%s"/><circle cx="25" cy="15" r="2" fill="%s"/><circle cx="35" cy="25" r="2" fill="%s"/><circle cx="25" cy="35" r="2" fill="%s"/></svg>`, color, color, color, color, color, color)
		},
	},
}

// GetPatterns returns all available pattern definitions sorted for UI display
func GetPatterns() []PatternDef {
	// Return in a sensible order for the UI
	orderedIDs := []string{
		"grid", "dotGrid", "polkaDots", "diagonalLines",
		"hexagons", "diamonds", "triangles", "boxes",
		"waves", "wiggle", "zigzag", "chevrons",
		"plus", "crosses", "stars", "brickWall",
		"topography", "jigsaw", "bubbles", "signal", "circuitBoard",
	}

	result := make([]PatternDef, 0, len(orderedIDs))
	for _, id := range orderedIDs {
		if p, ok := patterns[id]; ok {
			result = append(result, PatternDef{
				ID:   id,
				Name: p.name,
			})
		}
	}
	return result
}

// GetPatternSVG returns the SVG pattern as a data URL
// id: pattern identifier
// color: hex color (e.g., "#333333")
// opacity: 0-100 percentage
func GetPatternSVG(id, color string, opacity int) string {
	p, ok := patterns[id]
	if !ok || id == "none" {
		return ""
	}

	// Clamp opacity to valid range
	if opacity < 0 {
		opacity = 0
	}
	if opacity > 100 {
		opacity = 100
	}

	// Convert opacity to hex alpha
	alpha := fmt.Sprintf("%02x", (opacity*255)/100)
	colorWithAlpha := color + alpha

	svg := p.svgFunc(colorWithAlpha)
	encoded := base64.StdEncoding.EncodeToString([]byte(svg))
	return fmt.Sprintf("data:image/svg+xml;base64,%s", encoded)
}
