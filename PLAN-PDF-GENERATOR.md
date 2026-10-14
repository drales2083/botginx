# Plan: Generate PDF Module

## Overview

New module in Traffic section for generating downloadable PDFs with blurred background images and centered CTA buttons. **No history/saving** — PDFs download instantly on generate.

## UI Design

Use the **customizer pattern** from `/user/redirectlinks/{id}/customize`:
- Split layout: controls sidebar (left) + live preview (right)
- Collapsible sections with consistent styling
- Real-time preview updates on any change
- Single "Generate PDF" button (no save)

## Features

### 1. Background Image
- **Two modes**: "Upload" or "Presets"
- **Upload mode**: drag & drop or click (max 5MB, JPEG/PNG, not saved)
- **Presets mode**: grid of default images we provide
- Preview thumbnail after selection
- Clear button to remove

#### Preset Images (stored in `web/static/pdf-backgrounds/`)
| Filename | Description |
|----------|-------------|
| gradient-blue.jpg | Blue/purple gradient |
| gradient-red.jpg | Red/orange gradient |
| gradient-green.jpg | Green/teal gradient |
| abstract-dark.jpg | Dark abstract pattern |
| abstract-light.jpg | Light abstract pattern |
| geometric.jpg | Geometric shapes |
| nature-blur.jpg | Blurred nature scene |
| city-blur.jpg | Blurred cityscape |

### 2. Blur Level
- Range slider: 0-20px
- Real-time preview of blur effect
- Default: 5px

### 3. Button Customization
| Control | Type | Default |
|---------|------|---------|
| Button Text | Text input | "Click Here" |
| Button Color | Color picker | #dc3545 (red) |
| Text Color | Color picker | #ffffff (white) |
| Border Radius | Range (0-50px) | 8px |
| Padding X | Range (10-80px) | 40px |
| Padding Y | Range (8-40px) | 16px |
| Font Size | Range (14-32px) | 18px |
| Shadow | Toggle | On |

### 4. Link Selection
- **Dropdown**: User's redirect links (from redirectlinks module)
- **OR text input**: Paste any custom URL
- Toggle between modes: "My Links" / "Custom URL"

### 5. PDF Settings
- Page size: A4 (default), Letter, Custom
- Orientation: Portrait / Landscape
- Margins: Range slider (0-50mm)

## File Structure

```
modules/pdfgenerator/
├── module.go
├── handlers/
│   └── handler.go
├── services/
│   └── service.go          # PDF generation logic
├── templates/
│   ├── index.html          # Main customizer UI
│   └── partials/
│       └── pdf-customizer.html
└── static/
    └── backgrounds/        # Preset background images
        ├── gradient-blue.jpg
        ├── gradient-red.jpg
        ├── gradient-green.jpg
        ├── abstract-dark.jpg
        ├── abstract-light.jpg
        ├── geometric.jpg
        ├── nature-blur.jpg
        └── city-blur.jpg
```

## Routes

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | /user/traffic/pdf | Index | Show customizer UI |
| GET | /user/traffic/pdf/api/links | GetLinks | List user's redirect links for dropdown |
| GET | /user/traffic/pdf/api/backgrounds | GetBackgrounds | List preset background images |
| POST | /user/traffic/pdf/api/preview | Preview | Generate preview image (base64) |
| POST | /user/traffic/pdf/api/generate | Generate | Generate & download PDF |

## Menu Integration

Add to Traffic section in sidebar:
```go
MenuItem{
    ID:    "traffic-pdf",
    Title: "Generate PDF",
    Icon:  "bi-file-earmark-pdf",
    Path:  "/user/traffic/pdf",
}
```

## Technical Implementation

### PDF Generation (service.go)

Use `github.com/jung-kurt/gofpdf` or `github.com/signintech/gopdf`:

```go
type PDFRequest struct {
    ImageDataURL string  `json:"imageDataUrl"`
    BlurLevel    int     `json:"blurLevel"`
    ButtonText   string  `json:"buttonText"`
    ButtonColor  string  `json:"buttonColor"`
    TextColor    string  `json:"textColor"`
    BorderRadius int     `json:"borderRadius"`
    PaddingX     int     `json:"paddingX"`
    PaddingY     int     `json:"paddingY"`
    FontSize     int     `json:"fontSize"`
    Shadow       bool    `json:"shadow"`
    LinkURL      string  `json:"linkUrl"`
    PageSize     string  `json:"pageSize"`     // a4, letter, custom
    Orientation  string  `json:"orientation"`  // portrait, landscape
    Margins      int     `json:"margins"`
}

func (s *Service) GeneratePDF(req PDFRequest) ([]byte, error) {
    // 1. Decode base64 image
    // 2. Apply blur using imaging library
    // 3. Create PDF with blurred background
    // 4. Add centered button as clickable link annotation
    // 5. Return PDF bytes
}
```

### Image Processing

Use `github.com/disintegration/imaging` for blur:

```go
import "github.com/disintegration/imaging"

func applyBlur(img image.Image, sigma float64) image.Image {
    return imaging.Blur(img, sigma)
}
```

### Button as PDF Link Annotation

PDF link annotations make the button clickable:

```go
// Using gofpdf
pdf.SetLink(linkID, 0, -1)
pdf.LinkString(x, y, w, h, linkURL)
```

### Preview Generation (handler.go)

For live preview, generate a PNG preview (not full PDF):

```go
func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
    var req PDFRequest
    json.NewDecoder(r.Body).Decode(&req)
    
    // Generate preview image with blur + button overlay
    previewImg := h.service.GeneratePreview(req)
    
    // Return as base64 data URL
    var buf bytes.Buffer
    png.Encode(&buf, previewImg)
    dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
    
    json.NewEncoder(w).Encode(map[string]string{"preview": dataURL})
}
```

## Template Structure (index.html)

```html
{{define "content"}}
<div class="cust-app" id="pdf-customizer-root">
  <!-- Left: Controls -->
  <aside class="cust-controls">
    <div class="cust-controls-body">
      
      <!-- Background Section -->
      <div class="cust-section" data-section="background">
        <div class="cust-section-header">
          <span><i class="bi bi-image me-2"></i>Background</span>
          <i class="bi bi-chevron-down"></i>
        </div>
        <div class="cust-section-body">
          <!-- Upload area -->
          <!-- Blur slider -->
        </div>
      </div>

      <!-- Button Section -->
      <div class="cust-section" data-section="button">
        <div class="cust-section-header">
          <span><i class="bi bi-hand-index me-2"></i>Button</span>
          <i class="bi bi-chevron-down"></i>
        </div>
        <div class="cust-section-body">
          <!-- Text, colors, sizing -->
        </div>
      </div>

      <!-- Link Section -->
      <div class="cust-section" data-section="link">
        <div class="cust-section-header">
          <span><i class="bi bi-link-45deg me-2"></i>Destination</span>
          <i class="bi bi-chevron-down"></i>
        </div>
        <div class="cust-section-body">
          <!-- Toggle: My Links / Custom URL -->
          <!-- Dropdown or text input -->
        </div>
      </div>

      <!-- PDF Settings Section -->
      <div class="cust-section" data-section="settings">
        <div class="cust-section-header">
          <span><i class="bi bi-file-earmark me-2"></i>PDF Settings</span>
          <i class="bi bi-chevron-down"></i>
        </div>
        <div class="cust-section-body">
          <!-- Page size, orientation, margins -->
        </div>
      </div>

    </div>

    <!-- Actions footer -->
    <div class="cust-actions">
      <button class="btn btn-outline-secondary btn-sm" onclick="resetSettings()">
        <i class="bi bi-arrow-counterclockwise me-1"></i>Reset
      </button>
      <button class="btn btn-primary btn-sm" id="generate-btn" onclick="generatePDF()">
        <i class="bi bi-download me-1"></i>Generate PDF
      </button>
    </div>
  </aside>

  <!-- Right: Preview -->
  <section class="cust-preview">
    <header class="cust-preview-header">
      <i class="bi bi-eye"></i>
      <span class="title">Live Preview</span>
    </header>
    <div class="cust-preview-frame">
      <img id="preview-img" src="" style="max-width:100%;max-height:100%;">
    </div>
  </section>
</div>
{{end}}
```

## Dependencies

Add to go.mod:
```
github.com/jung-kurt/gofpdf v1.16.2
github.com/disintegration/imaging v1.6.2
```

## User Flow

1. User navigates to Traffic → Generate PDF
2. Uploads background image
3. Adjusts blur level (sees live preview)
4. Customizes button appearance
5. Selects destination link (dropdown) or pastes custom URL
6. Adjusts PDF settings if needed
7. Clicks "Generate PDF" → instant download

## Edge Cases

| Case | Handling |
|------|----------|
| No image uploaded | Show placeholder, disable generate |
| No link selected | Show warning, disable generate |
| Large image (>5MB) | Client-side validation, toast error |
| Invalid URL | Validate on generate, toast error |
| Image decode failure | Server returns error, toast message |

## No Database

This module requires **no database tables**. Everything is:
- User input → server processing → instant download
- No history, no templates, no saved PDFs

## Implementation Order

1. Create module skeleton with routes
2. Build customizer UI (copy pattern from redirectlinks)
3. Implement preview endpoint (PNG with blur + button overlay)
4. Implement PDF generation with clickable link annotation
5. Add to Traffic menu
6. Test with various image sizes and button configs

## Estimated Effort

- Module setup + routes: 30 min
- UI (customizer template): 2 hours
- Preview service: 1 hour
- PDF generation: 2 hours
- Testing + polish: 1 hour

**Total: ~6 hours**
