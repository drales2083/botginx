---
title: Redirect Links
description: Create trackable redirect links with custom paths, rotation, and protection.
---

A redirect link is a URL on your domain that forwards visitors somewhere else. You control the path, the destination, and what happens in between.

## Why Use Redirect Links

Your traffic sources (ads, emails, social posts) point at your redirect link. The link points at your landing page. This separation gives you control:

- Change the destination without updating your traffic sources
- A/B test by rotating between multiple destinations
- Block bots before they reach your page
- Track clicks with full visitor data
- Use clean branded URLs instead of long affiliate links

## Link Types

### Standard Redirect

One link, one destination. Visitor clicks, gets redirected, done.

```
yourdomain.com/offer → landing-page.com/signup
```

Use this when you've got a single destination and just need tracking or protection.

### Rotation

One link, multiple destinations. GuardBot cycles through them.

**Sequential rotation** sends visitors to destinations in order. First click goes to URL 1, second click to URL 2, and so on. Useful for distributing traffic evenly across pages.

**Random rotation** picks a destination at random each time. Good for A/B testing when you don't need strict distribution.

**Weighted rotation** lets you set percentages. Send 70% of traffic to your main page, 30% to a variant.

### HTML Content

Instead of redirecting, serve HTML directly. The link becomes a page.

Use this for simple landing pages, bridge pages, or compliance pages that need to live on your domain.

## Creating a Link

Go to **Redirect Links** and click **New Link**.

Required fields:

| Field | What It Does |
|-------|--------------|
| Domain | Which of your domains hosts the link |
| Destination | Where visitors go (URL or multiple URLs for rotation) |

Optional fields:

| Field | What It Does |
|-------|--------------|
| Path | The URL path like `/promo`. Auto-generated if blank |
| Subdomain | Put the link on a subdomain like `go.yourdomain.com` |
| Bot Protection | Enable blocking rules for this link |
| Pass Parameters | Forward query strings to the destination |

Click **Create**. The link goes live immediately.

## Editing Links

Click any link to open its settings. You can change:

- Destination URL (or URLs for rotation)
- Path and subdomain
- Protection settings
- Active/inactive status

Changes take effect within seconds. No DNS changes, no propagation wait.

## Path Rules

Paths can include letters, numbers, hyphens, and forward slashes.

Valid paths:
- `/offer`
- `/summer-sale-2024`
- `/campaigns/facebook/ad1`

Invalid paths:
- Spaces
- Special characters except hyphens and slashes
- Starting with a slash is optional (we add it)

## Query Parameter Passthrough

Turn on **Pass Parameters** to forward query strings to your destination.

If someone visits `yourdomain.com/offer?utm_source=facebook`, they land on `landing-page.com/signup?utm_source=facebook`.

Useful for preserving tracking parameters, affiliate IDs, or any data you need downstream.

## Bulk Operations

Select multiple links to:

- Enable or disable them
- Delete them
- Export to CSV

Use the checkbox column in the link list, then pick an action from the toolbar.
