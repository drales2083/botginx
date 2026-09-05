# Reusable Page Customizer Component

**Date:** 2026-09-05  
**Status:** Approved  
**Scope:** New `pkg/customizer` package for splash page customization

## Overview

A reusable Go package that generates customized splash/loading pages with:
- 100+ loading animations across 7 categories
- 21 background patterns
- 30+ Google Fonts across 7 style categories
- Image/logo upload (base64 data URLs, max 500KB)
- Full layout controls (content order, alignment, spacing)

Used by redirect links module now, and future downloads module (PHP challenge page generation).

## Package Structure

```
pkg/customizer/
├── embed.go                   # //go:embed directives for assets
├── assets/
│   ├── loaders.css            # All 100+ loader animations (~40KB)
│   ├── patterns.go            # Pattern SVG generators as Go funcs
│   └── fonts.json             # Font registry: name, weights, Google import URL
├── types.go                   # Customization struct
├── registry.go                # GetLoaders(), GetPatterns(), GetFonts()
├── generator.go               # GenerateHTML(opts)
├── preview.go                 # GeneratePreviewHTML(c)
└── template.go                # HTML template strings
```

## Data Model

```go
type Customization struct {
    // Background
    BgColor          string `json:"bgColor"`
    BgColorSecondary string `json:"bgColorSecondary"`
    GradientEnabled  bool   `json:"gradientEnabled"`
    Pattern          string `json:"pattern"`
    PatternColor     string `json:"patternColor"`
    PatternOpacity   int    `json:"patternOpacity"`

    // Loader
    Loader               string `json:"loader"`
    LoaderColorPrimary   string `json:"loaderColorPrimary"`
    LoaderColorSecondary string `json:"loaderColorSecondary"`

    // Text
    Heading           string `json:"heading"`
    HeadingVisible    bool   `json:"headingVisible"`
    Subheading        string `json:"subheading"`
    SubheadingVisible bool   `json:"subheadingVisible"`
    Font              string `json:"font"`
    FontWeight        int    `json:"fontWeight"`
    TextColor         string `json:"textColor"`
    TextSize          int    `json:"textSize"`
    TextShadow        bool   `json:"textShadow"`

    // Image/Logo
    ImageMode    string `json:"imageMode"`
    ImageDataURL string `json:"imageDataUrl"`
    ImageSize    int    `json:"imageSize"`
    ImageOverlay int    `json:"imageOverlay"`

    // Layout
    ContentOrder string `json:"contentOrder"`
    TextAlign    string `json:"textAlign"`
    VPos         int    `json:"vPos"`
    Gap          int    `json:"gap"`
    PageTitle    string `json:"pageTitle"`
}
```

## Registry System

### Loader Categories (100+ total)

| Category | Count | Examples |
|----------|-------|----------|
| Spinners & Rings | 23 | spinner-classic, ring-pulse, ring-orbit |
| Dots & Bouncing | 20 | dots-bounce, dots-fade, dots-wave |
| Bars & Waves | 13 | bars-scale, bars-audio, bars-equalizer |
| Shapes & Morphing | 17 | square-spin, cube-grid, blob-morph |
| Pulse & Glow | 13 | pulse-circle, glow-ring, neon-circle |
| Progress & Special | 11 | progress-bar, hourglass, orbit |
| Brand Inspired | 9 | google-dots, spotify-bars, apple-spinner |

### Patterns (21)

topography, jigsaw, wiggle, bubbles, signal, diagonalLines, polkaDots, hexagons, plus, circuitBoard, boxes, brickWall, diamonds, waves, chevrons, zigzag, triangles, stars, crosses, grid, dotGrid

### Font Categories (30+ total)

| Category | Fonts |
|----------|-------|
| Stylish Display | Playfair Display, Cinzel, Abril Fatface, Bodoni Moda, Cormorant Garamond |
| Bold Impact | Bebas Neue, Anton, Oswald, Righteous, Russo One |
| Script | Dancing Script, Pacifico, Great Vibes, Lobster |
| Tech | Orbitron, Audiowide, Rajdhani, Exo 2 |
| Modern | Montserrat, Poppins, Inter, Space Grotesk, DM Sans |
| Rounded | Quicksand, Comfortaa, Nunito |
| Monospace | JetBrains Mono, Fira Code |

### Registry Functions

```go
func GetLoaders() []LoaderDef
func GetPatterns() []PatternDef
func GetFonts() []FontDef
func GetDefaultCustomization() Customization
```

## Generator

### HTML Generation

```go
type GenerateOptions struct {
    Customization Customization
    RedirectURL   string
    Delay         int
    Mode          string // "redirect" or "challenge"
}

func GenerateHTML(opts GenerateOptions) string
```

**Output characteristics:**
- Self-contained HTML (no external JS dependencies)
- Google Font loaded via `<link>` tag
- Only includes CSS for the selected loader
- Pattern SVG inlined as background-image data URL
- Image embedded as base64 data URL

### Preview Generation

```go
func GeneratePreviewHTML(c Customization) string
```

Same visual output, no redirect script.

## Preview API

```
POST /api/customizer/preview
Content-Type: application/json

{
    "bgColor": "#0a0a0a",
    "loader": "dots-bounce",
    ...
}

Response: text/html (preview page)
```

**UI flow:**
1. User changes control
2. JS debounces (100ms) and POSTs to preview endpoint
3. Response HTML replaces iframe srcdoc
4. Preview updates in ~50ms

## UI Template Partial

Located at `web/templates/partials/customizer.html`:

```html
{{define "customizer"}}
<div class="row g-2" id="customizationPanel">
    <!-- Left Panel - Controls (col-lg-4) -->
    <div class="col-lg-4">
        <div class="cust-panel">
            <!-- Background Section -->
            <!-- Animation Section -->
            <!-- Text Content Section -->
            <!-- Image/Logo Section -->
            <!-- Layout Section -->
            <!-- Page Title Section -->
        </div>
    </div>
    
    <!-- Right Panel - Preview (col-lg-8) -->
    <div class="col-lg-8">
        <div class="preview-frame">
            <iframe id="previewFrame"></iframe>
        </div>
    </div>
</div>

<style>/* .cust-panel, .cust-section styles */</style>
<script>/* initCustomizer(existing, previewUrl, saveUrl) */</script>
{{end}}
```

**Usage:**
```html
{{template "customizer" .}}
<script>
    initCustomizer(
        {{.ExistingData | json}},
        '/api/customizer/preview',
        '/user/redirectlinks/api/{{.Link.ID}}/customization'
    );
</script>
```

## Integration: Redirect Links Module

### Files Modified

1. **`modules/redirectlinks/models/redirect_link.go`**
   - Remove `Customization` struct
   - Import `pkg/customizer`

2. **`modules/redirectlinks/handlers/handler.go`**
   - Remove hardcoded loader CSS (~100 lines)
   - Use `customizer.GenerateHTML(opts)`
   - Add preview API handler

3. **`modules/redirectlinks/templates/customize.html`**
   - Replace with `{{template "customizer" .}}`

4. **`modules/redirectlinks/routes.go`**
   - Add `/api/customizer/preview` route

### Migration

- Existing database JSON compatible with new struct
- Missing fields filled from `GetDefaultCustomization()`
- No data migration required

## Future: Downloads Module

When downloads module is built:

```go
type PHPGenerateOptions struct {
    Customization Customization
    APIEndpoint   string
    APIKey        string
    ProtectedURL  string
}

func GeneratePHP(opts PHPGenerateOptions) string
```

Shares all visual customization. Different output format and challenge logic.

## Implementation Tasks

1. Create `pkg/customizer/` package structure
2. Implement `types.go` with Customization struct
3. Implement `registry.go` with all loaders/patterns/fonts
4. Create `assets/loaders.css` with all 100+ animations
5. Create `assets/patterns.go` with SVG generators
6. Create `assets/fonts.json` with font metadata
7. Implement `generator.go` for HTML output
8. Implement `preview.go` for preview HTML
9. Create `web/templates/partials/customizer.html`
10. Refactor redirect links to use new package
11. Add preview API endpoint
12. Test all loaders/patterns/fonts render correctly
