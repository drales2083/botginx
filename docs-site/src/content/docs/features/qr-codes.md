---
title: About QR Codes
description: When and why to use QR codes for your links.
---

QR codes turn your links into scannable images. Print them on flyers, business cards, product packaging, or anywhere physical. Scans get tracked the same way clicks do.

## Why QR Codes

Digital links work for digital channels. But physical materials need a bridge to the online world. A QR code provides that bridge without asking people to type URLs.

**Print marketing** puts your link on flyers, posters, and direct mail. People scan instead of typing.

**Product packaging** links to instructions, warranty registration, or promotional offers. The physical product becomes a gateway to your digital content.

**Business cards** replace long URLs with clean, scannable codes. Professional and functional.

**Event materials** on conference badges, signage, and handouts. One scan beats hunting for a URL.

**Retail displays** link to product info, reviews, or purchase pages. Convert browsing interest into online engagement.

## Link First, QR Second

The best practice: create a redirect link first, then make a QR code pointing at it.

Why? Printed materials are permanent. Your destinations aren't. When your landing page moves or your campaign ends, update the redirect link. The printed QR code keeps working.

Also, redirect links give you bot protection and detailed analytics. A QR code pointing directly at an external URL just sends people there without any tracking.

## Error Correction

QR codes have built-in redundancy. Higher error correction means the code still works even if part of it is obscured or damaged.

**Level L (7%)** handles minor damage. Best for clean digital displays.

**Level M (15%)** handles moderate damage. Standard for most printing.

**Level Q (25%)** handles significant damage. Good for outdoor use or rough handling.

**Level H (30%)** handles heavy damage. Required when overlaying logos on the code.

Higher correction means more redundancy, which means the code needs more squares. The tradeoff is that heavily corrected codes are slightly harder to scan at small sizes.

## Customization Considerations

You can change the foreground and background colors, but stick to high contrast. Dark on light works best. Low-contrast combinations look good but scan poorly.

Keep it simple for reliability. Fancy designs with gradients or complex patterns often fail to scan. A basic black-and-white code scans reliably every time.

## Size Guidelines

QR codes need minimum size to scan reliably. The general rule: 10:1 ratio of scanning distance to code size. A 1-inch code scans from about 10 inches away. A tiny code on a billboard won't work.

Leave white space (the quiet zone) around the code. Crowding it against other design elements breaks scanning.

## Format Choice

**PNG** gives you a raster image at your chosen size. Works for web, documents, and standard printing.

**SVG** gives you a vector that scales to any size without pixelation. Use for large format printing or when you need flexibility.

## Tracking Scans

When your QR code points to a GuardBot redirect link, scans appear in that link's analytics. You see total scans, geographic breakdown, device types (mostly mobile for QR), and time patterns.

Create separate redirect links for each QR placement to track them independently. One link for your conference flyer, another for your product box. Then you know which placement drives traffic.

## Testing

Always test before printing. Scan the code with multiple phones and apps. Make sure it resolves correctly at the size you'll print. Once it's on 10,000 flyers, you can't fix it.

## Related Topics

- [Getting Started](/guides/quick-start/) for creating your first link
- [About Redirect Links](/features/redirect-links/) for understanding how redirects work
- [QR Codes API Reference](/api/qrcodes/) for generating codes programmatically
