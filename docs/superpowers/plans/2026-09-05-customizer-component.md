# Customizer Component Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build reusable `pkg/customizer` package with 100+ loaders, 21 patterns, 30+ fonts for splash page generation.

**Architecture:** Go package with embedded assets. Registry exposes available options, generator produces HTML. UI template partial shared across modules.

**Tech Stack:** Go 1.21+, embed directive, html/template

**Spec:** `docs/superpowers/specs/2026-09-05-customizer-component-design.md`

## Global Constraints

- All assets embedded via `//go:embed`
- Generated HTML must be self-contained (no external JS)
- Google Fonts loaded via `<link>` tag
- Base64 images max 500KB
- Backwards compatible with existing customization JSON

---

### Task 1: Package Scaffold & Types

**Files:**
- Create: `pkg/customizer/types.go`
- Create: `pkg/customizer/defaults.go`

**Interfaces:**
- Produces: `Customization` struct, `GetDefaultCustomization() Customization`

- [ ] **Step 1: Create types.go**

```go
// pkg/customizer/types.go
package customizer

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

type LoaderDef struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

type PatternDef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type FontDef struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Weights  []int  `json:"weights"`
}

type GenerateOptions struct {
	Customization Customization
	RedirectURL   string
	Delay         int
	Mode          string
}
```

- [ ] **Step 2: Create defaults.go**

```go
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
```

- [ ] **Step 3: Verify build**

Run: `go build ./pkg/customizer/...`

- [ ] **Step 4: Commit**

```bash
git add pkg/customizer/
git commit -m "feat(customizer): add types and defaults"
```

---

### Task 2: Loader Registry (100+ animations)

**Files:**
- Create: `pkg/customizer/loaders.go`

**Interfaces:**
- Produces: `GetLoaders() []LoaderDef`, `GetLoaderCSS(id string) string`, `GetLoaderHTML(id string) string`

- [ ] **Step 1: Create loaders.go with all 106 loaders**

Create file with loader definitions and CSS/HTML generators. File will be ~2000 lines containing all animations from user's reference HTML.

Categories: Spinners & Rings (23), Dots & Bouncing (20), Bars & Waves (13), Shapes & Morphing (17), Pulse & Glow (13), Progress & Special (11), Brand Inspired (9).

- [ ] **Step 2: Verify build**

Run: `go build ./pkg/customizer/...`

- [ ] **Step 3: Commit**

```bash
git add pkg/customizer/loaders.go
git commit -m "feat(customizer): add 106 loader animations"
```

---

### Task 3: Pattern Registry (21 patterns)

**Files:**
- Create: `pkg/customizer/patterns.go`

**Interfaces:**
- Produces: `GetPatterns() []PatternDef`, `GetPatternSVG(id, color string, opacity int) string`

- [ ] **Step 1: Create patterns.go with all 21 patterns**

SVG generators for: topography, jigsaw, wiggle, bubbles, signal, diagonalLines, polkaDots, hexagons, plus, circuitBoard, boxes, brickWall, diamonds, waves, chevrons, zigzag, triangles, stars, crosses, grid, dotGrid.

- [ ] **Step 2: Verify build**

Run: `go build ./pkg/customizer/...`

- [ ] **Step 3: Commit**

```bash
git add pkg/customizer/patterns.go
git commit -m "feat(customizer): add 21 background patterns"
```

---

### Task 4: Font Registry (30+ fonts)

**Files:**
- Create: `pkg/customizer/fonts.go`

**Interfaces:**
- Produces: `GetFonts() []FontDef`, `GetFontImportURL(name string, weights []int) string`

- [ ] **Step 1: Create fonts.go with all 30 fonts**

Categories: Stylish Display (5), Bold Impact (5), Script (4), Tech (4), Modern (5), Rounded (3), Monospace (2).

- [ ] **Step 2: Verify build**

Run: `go build ./pkg/customizer/...`

- [ ] **Step 3: Commit**

```bash
git add pkg/customizer/fonts.go
git commit -m "feat(customizer): add 30 Google Fonts"
```

---

### Task 5: HTML Generator

**Files:**
- Create: `pkg/customizer/generator.go`

**Interfaces:**
- Consumes: `GetLoaderCSS`, `GetLoaderHTML`, `GetPatternSVG`, `GetFontImportURL`
- Produces: `GenerateHTML(opts GenerateOptions) string`

- [ ] **Step 1: Create generator.go**

Generates complete self-contained HTML with:
- Google Font link tag
- Background (solid or gradient)
- Pattern overlay (inline SVG)
- Background image (if set)
- Loader animation (selected CSS only)
- Text content
- Redirect script

- [ ] **Step 2: Verify build**

Run: `go build ./pkg/customizer/...`

- [ ] **Step 3: Commit**

```bash
git add pkg/customizer/generator.go
git commit -m "feat(customizer): add HTML generator"
```

---

### Task 6: Preview Generator

**Files:**
- Create: `pkg/customizer/preview.go`

**Interfaces:**
- Consumes: `GenerateHTML`
- Produces: `GeneratePreviewHTML(c Customization) string`

- [ ] **Step 1: Create preview.go**

Same as full HTML but no redirect script.

- [ ] **Step 2: Commit**

```bash
git add pkg/customizer/preview.go
git commit -m "feat(customizer): add preview generator"
```

---

### Task 7: UI Template Partial

**Files:**
- Create: `web/templates/partials/customizer.html`

**Interfaces:**
- Produces: `{{template "customizer" .}}` partial with all controls

- [ ] **Step 1: Create customizer.html**

Full UI with:
- Background section (colors, gradient, pattern)
- Animation section (loader dropdown with 106 options)
- Text section (heading, subheading, font, size, color)
- Image section (none/logo/background, upload, size)
- Layout section (content order, alignment, spacing)
- Preview iframe
- JS: `initCustomizer(existing, previewUrl, saveUrl)`

- [ ] **Step 2: Commit**

```bash
git add web/templates/partials/customizer.html
git commit -m "feat(customizer): add UI template partial"
```

---

### Task 8: Integrate with Redirect Links

**Files:**
- Modify: `modules/redirectlinks/models/redirect_link.go`
- Modify: `modules/redirectlinks/handlers/handler.go`
- Modify: `modules/redirectlinks/templates/customize.html`
- Modify: `modules/redirectlinks/routes.go`

**Interfaces:**
- Consumes: `pkg/customizer` package

- [ ] **Step 1: Update models to import customizer**

Remove local `Customization` struct, use `customizer.Customization`.

- [ ] **Step 2: Update handler to use generator**

Replace hardcoded loader CSS with `customizer.GenerateHTML()`.
Add preview API handler.

- [ ] **Step 3: Update customize.html**

Replace basic form with `{{template "customizer" .}}`.

- [ ] **Step 4: Add preview route**

Add `POST /api/customizer/preview` route.

- [ ] **Step 5: Test end-to-end**

Run server, test customizer UI, verify preview updates and save works.

- [ ] **Step 6: Commit**

```bash
git add modules/redirectlinks/ web/templates/partials/
git commit -m "feat(redirectlinks): integrate customizer component"
```

---

### Task 9: Final Testing & Deploy

- [ ] **Step 1: Test all loaders render**
- [ ] **Step 2: Test all patterns render**
- [ ] **Step 3: Test all fonts load**
- [ ] **Step 4: Test image upload (logo and background modes)**
- [ ] **Step 5: Test layout options**
- [ ] **Step 6: Verify existing links still work (backwards compatibility)**
- [ ] **Step 7: Push and tag release**

```bash
git push origin main
git tag v1.9.0 -m "feat: reusable customizer component"
git push origin v1.9.0
```
