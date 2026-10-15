# QR Codes Module Plan

## Overview

QR code generator that allows users to create downloadable QR codes for any URL or their existing redirect links. QR codes are saved to a library for future access.

## User Flow

1. User navigates to `/user/qrcodes`
2. Clicks "Create QR Code"
3. Either:
   - Pastes any external URL, OR
   - Selects from dropdown of their redirect links
4. Optionally sets title for the QR
5. Customizes colors (foreground/background)
6. Previews QR code in real-time
7. Downloads as PNG or SVG
8. QR is saved to their library

## Features

### Core
- Generate QR code from any URL
- Generate QR code from existing redirect link (pre-fills URL)
- Real-time preview as user types/customizes
- Download PNG (multiple sizes: 200px, 400px, 800px, 1200px)
- Download SVG (vector, scalable)
- Save to library for later access

### Customization
- Foreground color (default: black)
- Background color (default: white)
- Size presets for download
- Title/label for organization

### Library
- List all saved QR codes
- Search/filter by title
- Re-download anytime
- Delete QR codes
- View original URL

## Pages

| Route | Title | Description |
|-------|-------|-------------|
| `/user/qrcodes` | QR Codes | Library of saved QR codes |
| `/user/qrcodes/new` | Create QR Code | Generate new QR code |
| `/user/qrcodes/{id}` | QR Details | View, edit, re-download |

## Database

### Table: `qr_codes`

```sql
CREATE TABLE IF NOT EXISTS qr_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL DEFAULT '',
    url TEXT NOT NULL,
    redirect_link_id UUID REFERENCES redirect_links(id) ON DELETE SET NULL,
    fg_color VARCHAR(7) NOT NULL DEFAULT '#000000',
    bg_color VARCHAR(7) NOT NULL DEFAULT '#FFFFFF',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_qr_codes_user_id ON qr_codes(user_id);
```

## Module Structure

```
modules/qrcodes/
├── module.go              # Module registration
├── handlers/
│   └── handler.go         # HTTP handlers
├── models/
│   └── qrcode.go          # QRCode struct
├── services/
│   └── service.go         # Business logic, QR generation
├── templates/
│   ├── list.html          # QR library
│   ├── new.html           # Create QR form
│   └── show.html          # QR details
└── migrations/
    └── 001_create_tables.sql
```

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/user/qrcodes` | List page |
| GET | `/user/qrcodes/new` | Create page |
| GET | `/user/qrcodes/{id}` | Detail page |
| GET | `/user/qrcodes/api/` | List QR codes JSON |
| POST | `/user/qrcodes/api/` | Create QR code |
| GET | `/user/qrcodes/api/{id}` | Get QR code JSON |
| PUT | `/user/qrcodes/api/{id}` | Update QR code |
| DELETE | `/user/qrcodes/api/{id}` | Delete QR code |
| GET | `/user/qrcodes/api/{id}/download` | Download QR image |
| POST | `/user/qrcodes/api/preview` | Generate preview (no save) |

## Dependencies

Go library for QR generation:
- `github.com/skip2/go-qrcode` - PNG generation
- For SVG: generate manually or use template

## UI Components

### List Page (`/user/qrcodes`)
- Grid of QR code cards (similar to redirect links)
- Each card shows: QR preview, title, URL (truncated), created date
- Actions: View, Download, Delete
- Empty state with CTA to create first QR

### Create Page (`/user/qrcodes/new`)
- Input method toggle: "Paste URL" | "Select Link"
- URL input field OR redirect link dropdown
- Title input (optional)
- Color pickers: foreground, background
- Live QR preview (updates as user types)
- Size selector for download
- Format selector: PNG | SVG
- Download button
- Save to library checkbox (default: checked)

### Detail Page (`/user/qrcodes/{id}`)
- Large QR code display
- URL display with copy button
- Edit title, colors
- Download buttons (PNG sizes, SVG)
- Delete button
- If linked to redirect: link to analytics

## Menu Item

```go
{
    Title:   "QR Codes",
    Icon:    "bi-qr-code",
    Path:    "/user/qrcodes",
    Order:   7,
    Section: module.MenuSectionUser,
    Group:   "Links",
}
```

## Implementation Order

1. Create module structure and register
2. Database migration
3. Models and service (QR generation)
4. List page with empty state
5. Create page with preview
6. API endpoints
7. Detail page
8. Download functionality (PNG/SVG)
9. Polish UI, add to dashboard tile

## QR Generation Logic

```go
// Generate PNG
func GeneratePNG(url string, size int, fgColor, bgColor string) ([]byte, error) {
    qr, err := qrcode.New(url, qrcode.Medium)
    if err != nil {
        return nil, err
    }
    qr.ForegroundColor = parseColor(fgColor)
    qr.BackgroundColor = parseColor(bgColor)
    return qr.PNG(size)
}

// Generate SVG (template-based)
func GenerateSVG(url string, fgColor, bgColor string) (string, error) {
    // Use qrcode library to get the bit matrix
    // Render as SVG using template
}
```

## Color Presets (Optional Enhancement)

Offer quick color presets:
- Classic (black on white)
- Inverted (white on black)
- Brand colors (from user settings if available)
- Dark mode friendly

## Error Handling

- Invalid URL: Show validation error
- URL too long (QR has limits ~2000 chars): Show error
- Invalid colors: Default to black/white
- Generation failure: Show error toast

## Security

- URLs are not validated for content (user responsibility)
- Rate limit QR generation (prevent abuse)
- Sanitize title input
- QR codes are private (only owner can access)
