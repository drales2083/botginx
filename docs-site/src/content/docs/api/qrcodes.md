---
title: QR Codes
description: Generate and manage QR codes for your links.
---

## List QR Codes

```http
GET /api/v1/qrcodes
```

**Scope:** `qrcodes:read`

```bash
curl "https://guardbot.sbs/api/v1/qrcodes" \
  -H "Authorization: Bearer YOUR_API_KEY"
```

---

## Create QR Code

```http
POST /api/v1/qrcodes
```

**Scope:** `qrcodes:write`

```bash
curl -X POST "https://guardbot.sbs/api/v1/qrcodes" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "My QR Code",
    "url": "https://example.com/promo",
    "fgColor": "#000000",
    "bgColor": "#FFFFFF"
  }'
```

### Request Body

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `title` | string | Yes | Display title for the QR code |
| `url` | string | Yes | URL the QR code points to |
| `redirectLinkId` | string | No | Associate with a redirect link |
| `fgColor` | string | No | Foreground color (default: #000000) |
| `bgColor` | string | No | Background color (default: #FFFFFF) |

---

## Download QR Code

```http
GET /api/v1/qrcodes/{id}/download
```

**Scope:** `qrcodes:read`

Download QR code image:

```bash
curl "https://guardbot.sbs/api/v1/qrcodes/qr_abc123/download?format=png" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -o qrcode.png
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `format` | string | png | `png` or `svg` |
