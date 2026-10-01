---
title: QR Codes
description: Generate trackable QR codes for print, packaging, and physical marketing.
---

QR codes turn your links into scannable images. Print them on flyers, business cards, product packaging, or anywhere physical. Scans get tracked the same way clicks do.

## Creating a QR Code

Go to **QR Codes** in the sidebar. Click **Create QR Code**.

Enter:

- **Title** for your reference (like "Conference Flyer" or "Product Insert")
- **URL** the code points to

The URL can be a GuardBot redirect link or any external URL. Using a redirect link gives you bot protection and the ability to change the destination later.

## Customizing Appearance

You can change:

| Option | What It Does |
|--------|--------------|
| Foreground Color | The dark squares (default: black) |
| Background Color | The light areas (default: white) |
| Size | Pixel dimensions for download |
| Error Correction | How much damage the code can survive |

### Error Correction Levels

QR codes have built-in redundancy. Higher correction means the code still works even if part of it is obscured or damaged.

| Level | Recovers From | Best For |
|-------|---------------|----------|
| L (7%) | Minor damage | Clean digital displays |
| M (15%) | Moderate damage | Standard printing |
| Q (25%) | Significant damage | Outdoor or rough handling |
| H (30%) | Heavy damage | Logos overlaid on the code |

If you're adding a logo in the center of the QR code, use H level. Otherwise M works for most printed materials.

## Downloading

Click **Download** and pick a format:

**PNG** gives you a raster image at your chosen size. Good for web, documents, or standard printing.

**SVG** gives you a vector. Scales to any size without pixelation. Use this for large format printing or when you need to resize later.

## Linking to Redirect Links

The best practice: create a redirect link first, then make a QR code pointing at it.

Why? You can change where the redirect link goes without reprinting the QR code. Printed materials are permanent. Your destinations aren't.

Also, redirect links give you bot protection and detailed analytics. A QR code pointing at an external URL just sends people there directly.

## Tracking Scans

If your QR code points at a GuardBot redirect link, scans show up in that link's analytics. You'll see:

- Total scans
- Geographic breakdown
- Device types (mostly mobile for QR)
- Time patterns

Create separate redirect links for each QR placement to track them independently. One link for your conference flyer, another for your product box. Then you know which placement drives traffic.

## Use Cases

**Print marketing**: Flyers, posters, direct mail. Give people a quick way to your landing page.

**Product packaging**: Link to instructions, warranty registration, or promotional offers.

**Business cards**: Replace a long URL with a clean code.

**Event materials**: Conference badges, signage, handouts.

**Retail displays**: Link to product info, reviews, or purchase pages.

## Best Practices

**Test before printing**. Scan the code with multiple phones. Make sure it resolves correctly.

**Leave quiet space**. QR codes need white space around them to scan reliably. Don't crowd them against other elements.

**Size appropriately**. Bigger codes scan from farther away. A tiny code on a billboard won't work. General rule: 10:1 ratio of scanning distance to code size.

**Use high contrast**. Dark foreground on light background works best. Avoid low-contrast color combinations.

**Don't over-customize**. Fancy designs look good but scan poorly. Keep it simple for reliability.
