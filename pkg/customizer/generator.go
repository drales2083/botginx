// pkg/customizer/generator.go
package customizer

import (
	"fmt"
	"html"
	"strings"
)

// GenerateHTML produces a complete self-contained HTML page based on the options
func GenerateHTML(opts GenerateOptions) string {
	c := opts.Customization

	var b strings.Builder

	// Page title
	pageTitle := c.PageTitle
	if pageTitle == "" {
		pageTitle = "Loading..."
	}

	// Build the HTML
	b.WriteString("<!DOCTYPE html>\n")
	b.WriteString("<html lang=\"en\">\n")
	b.WriteString("<head>\n")
	b.WriteString("  <meta charset=\"UTF-8\">\n")
	b.WriteString("  <meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n")
	b.WriteString(fmt.Sprintf("  <title>%s</title>\n", html.EscapeString(pageTitle)))

	// Google Font link
	fontURL := buildFontURL(c.Font, c.FontWeight)
	if fontURL != "" {
		b.WriteString(fmt.Sprintf("  <link rel=\"preconnect\" href=\"https://fonts.googleapis.com\">\n"))
		b.WriteString(fmt.Sprintf("  <link rel=\"preconnect\" href=\"https://fonts.gstatic.com\" crossorigin>\n"))
		b.WriteString(fmt.Sprintf("  <link href=\"%s\" rel=\"stylesheet\">\n", fontURL))
	}

	// Inline CSS
	b.WriteString("  <style>\n")
	b.WriteString(buildCSS(c))
	b.WriteString("  </style>\n")
	b.WriteString("</head>\n")

	// Body
	b.WriteString("<body>\n")
	b.WriteString(buildBody(c))

	// Redirect script (if redirect URL is provided)
	if opts.RedirectURL != "" && opts.Delay > 0 {
		b.WriteString(buildRedirectScript(opts.RedirectURL, opts.Delay))
	}

	b.WriteString("</body>\n")
	b.WriteString("</html>\n")

	return b.String()
}

// buildFontURL constructs the Google Fonts URL for the given font
func buildFontURL(fontID string, weight int) string {
	if fontID == "" {
		return ""
	}

	// Find font definition
	for _, f := range fonts {
		if f.ID == fontID {
			// Determine which weights to load
			weights := []int{weight}
			// Always include the requested weight if available
			hasWeight := false
			for _, w := range f.Weights {
				if w == weight {
					hasWeight = true
					break
				}
			}
			if !hasWeight && len(f.Weights) > 0 {
				weights = []int{f.Weights[0]}
			}
			return GetFontImportURL(f.Name, weights)
		}
	}
	return ""
}

// buildCSS generates the complete CSS for the page
func buildCSS(c Customization) string {
	var b strings.Builder

	// CSS Variables for loader colors
	b.WriteString("    :root {\n")
	b.WriteString(fmt.Sprintf("      --primary: %s;\n", c.LoaderColorPrimary))
	b.WriteString(fmt.Sprintf("      --secondary: %s;\n", c.LoaderColorSecondary))
	b.WriteString("    }\n\n")

	// Reset and base styles
	b.WriteString("    * { margin: 0; padding: 0; box-sizing: border-box; }\n\n")

	// Body styles with background
	b.WriteString("    body {\n")
	b.WriteString("      min-height: 100vh;\n")
	b.WriteString("      display: flex;\n")
	b.WriteString("      flex-direction: column;\n")
	b.WriteString("      align-items: center;\n")

	// Vertical position
	vPos := c.VPos
	if vPos == 0 {
		vPos = 50 // default center
	}
	if vPos <= 33 {
		b.WriteString("      justify-content: flex-start;\n")
		b.WriteString(fmt.Sprintf("      padding-top: %d%%;\n", vPos))
	} else if vPos >= 67 {
		b.WriteString("      justify-content: flex-end;\n")
		b.WriteString(fmt.Sprintf("      padding-bottom: %d%%;\n", 100-vPos))
	} else {
		b.WriteString("      justify-content: center;\n")
	}

	// Background (solid or gradient)
	if c.GradientEnabled && c.BgColorSecondary != "" {
		b.WriteString(fmt.Sprintf("      background: linear-gradient(135deg, %s 0%%, %s 100%%);\n", c.BgColor, c.BgColorSecondary))
	} else {
		b.WriteString(fmt.Sprintf("      background: %s;\n", c.BgColor))
	}

	// Font family
	fontFamily := getFontFamily(c.Font)
	if fontFamily != "" {
		b.WriteString(fmt.Sprintf("      font-family: '%s', sans-serif;\n", fontFamily))
	}

	b.WriteString("      overflow: hidden;\n")
	b.WriteString("      position: relative;\n")
	b.WriteString("    }\n\n")

	// Pattern overlay
	if c.Pattern != "" && c.Pattern != "none" {
		patternSVG := GetPatternSVG(c.Pattern, c.PatternColor, c.PatternOpacity)
		if patternSVG != "" {
			b.WriteString("    body::before {\n")
			b.WriteString("      content: '';\n")
			b.WriteString("      position: absolute;\n")
			b.WriteString("      inset: 0;\n")
			b.WriteString(fmt.Sprintf("      background-image: url(\"%s\");\n", patternSVG))
			b.WriteString("      background-repeat: repeat;\n")
			b.WriteString("      pointer-events: none;\n")
			b.WriteString("      z-index: 0;\n")
			b.WriteString("    }\n\n")
		}
	}

	// Background image overlay
	if c.ImageMode == "background" && c.ImageDataURL != "" {
		b.WriteString("    body::after {\n")
		b.WriteString("      content: '';\n")
		b.WriteString("      position: absolute;\n")
		b.WriteString("      inset: 0;\n")
		b.WriteString(fmt.Sprintf("      background-image: url(\"%s\");\n", c.ImageDataURL))
		b.WriteString("      background-size: cover;\n")
		b.WriteString("      background-position: center;\n")
		if c.ImageOverlay > 0 {
			overlayOpacity := float64(c.ImageOverlay) / 100.0
			b.WriteString(fmt.Sprintf("      opacity: %.2f;\n", 1.0-overlayOpacity))
		}
		b.WriteString("      pointer-events: none;\n")
		b.WriteString("      z-index: 0;\n")
		b.WriteString("    }\n\n")
	}

	// Content container
	b.WriteString("    .content {\n")
	b.WriteString("      display: flex;\n")
	b.WriteString("      flex-direction: column;\n")
	b.WriteString("      align-items: center;\n")
	gap := c.Gap
	if gap == 0 {
		gap = 20 // default gap
	}
	b.WriteString(fmt.Sprintf("      gap: %dpx;\n", gap))
	b.WriteString("      z-index: 1;\n")
	b.WriteString("      position: relative;\n")

	// Text alignment
	textAlign := c.TextAlign
	if textAlign == "" {
		textAlign = "center"
	}
	b.WriteString(fmt.Sprintf("      text-align: %s;\n", textAlign))
	b.WriteString("    }\n\n")

	// Logo/image styles
	if c.ImageMode == "logo" && c.ImageDataURL != "" {
		imageSize := c.ImageSize
		if imageSize == 0 {
			imageSize = 100
		}
		b.WriteString("    .logo {\n")
		b.WriteString(fmt.Sprintf("      width: %dpx;\n", imageSize))
		b.WriteString(fmt.Sprintf("      height: %dpx;\n", imageSize))
		b.WriteString("      object-fit: contain;\n")
		b.WriteString("    }\n\n")
	}

	// Text styles
	textSize := c.TextSize
	if textSize == 0 {
		textSize = 24
	}
	fontWeight := c.FontWeight
	if fontWeight == 0 {
		fontWeight = 400
	}

	b.WriteString("    .heading {\n")
	b.WriteString(fmt.Sprintf("      font-size: %dpx;\n", textSize))
	b.WriteString(fmt.Sprintf("      font-weight: %d;\n", fontWeight))
	b.WriteString(fmt.Sprintf("      color: %s;\n", c.TextColor))
	if c.TextShadow {
		b.WriteString("      text-shadow: 0 2px 4px rgba(0,0,0,0.3);\n")
	}
	b.WriteString("    }\n\n")

	b.WriteString("    .subheading {\n")
	subSize := textSize * 60 / 100
	if subSize < 12 {
		subSize = 12
	}
	b.WriteString(fmt.Sprintf("      font-size: %dpx;\n", subSize))
	b.WriteString(fmt.Sprintf("      font-weight: %d;\n", max(fontWeight-200, 300)))
	b.WriteString(fmt.Sprintf("      color: %s;\n", c.TextColor))
	b.WriteString("      opacity: 0.8;\n")
	if c.TextShadow {
		b.WriteString("      text-shadow: 0 1px 2px rgba(0,0,0,0.2);\n")
	}
	b.WriteString("    }\n\n")

	// Loader styles
	b.WriteString("    .loader-container {\n")
	b.WriteString("      display: flex;\n")
	b.WriteString("      justify-content: center;\n")
	b.WriteString("      align-items: center;\n")
	b.WriteString("    }\n\n")

	// Include selected loader CSS
	if c.Loader != "" {
		loaderCSS := GetLoaderCSS(c.Loader)
		if loaderCSS != "" {
			b.WriteString("    ")
			b.WriteString(strings.ReplaceAll(loaderCSS, "\n", "\n    "))
			b.WriteString("\n")
		}
	}

	return b.String()
}

// getFontFamily returns the display name for a font ID
func getFontFamily(fontID string) string {
	for _, f := range fonts {
		if f.ID == fontID {
			return f.Name
		}
	}
	return ""
}

// buildBody generates the body content HTML
func buildBody(c Customization) string {
	var b strings.Builder

	// Content container
	b.WriteString("  <div class=\"content\">\n")

	// Determine content order
	order := c.ContentOrder
	if order == "" {
		order = "loader-text" // default order
	}

	// Parse the order
	parts := strings.Split(order, "-")
	for _, part := range parts {
		switch part {
		case "loader":
			if c.Loader != "" {
				loaderHTML := GetLoaderHTML(c.Loader)
				if loaderHTML != "" {
					b.WriteString("    <div class=\"loader-container\">\n")
					b.WriteString(fmt.Sprintf("      %s\n", loaderHTML))
					b.WriteString("    </div>\n")
				}
			}
		case "text":
			// Heading
			if c.HeadingVisible && c.Heading != "" {
				b.WriteString(fmt.Sprintf("    <h1 class=\"heading\">%s</h1>\n", html.EscapeString(c.Heading)))
			}
			// Subheading
			if c.SubheadingVisible && c.Subheading != "" {
				b.WriteString(fmt.Sprintf("    <p class=\"subheading\">%s</p>\n", html.EscapeString(c.Subheading)))
			}
		case "logo", "image":
			if c.ImageMode == "logo" && c.ImageDataURL != "" {
				b.WriteString(fmt.Sprintf("    <img class=\"logo\" src=\"%s\" alt=\"Logo\">\n", c.ImageDataURL))
			}
		}
	}

	// Handle cases where image/logo might not be in the order string
	if c.ImageMode == "logo" && c.ImageDataURL != "" && !strings.Contains(order, "logo") && !strings.Contains(order, "image") {
		b.WriteString(fmt.Sprintf("    <img class=\"logo\" src=\"%s\" alt=\"Logo\">\n", c.ImageDataURL))
	}

	b.WriteString("  </div>\n")

	return b.String()
}

// buildRedirectScript generates the JavaScript for redirect
func buildRedirectScript(url string, delaySeconds int) string {
	var b strings.Builder

	b.WriteString("  <script>\n")
	b.WriteString(fmt.Sprintf("    setTimeout(function() {\n"))
	b.WriteString(fmt.Sprintf("      window.location.href = %q;\n", url))
	b.WriteString(fmt.Sprintf("    }, %d);\n", delaySeconds*1000))
	b.WriteString("  </script>\n")

	return b.String()
}
