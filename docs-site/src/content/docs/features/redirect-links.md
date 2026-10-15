---
title: About Redirect Links
description: Why redirect links matter and how they work.
---

A redirect link is a URL on your domain that forwards visitors somewhere else. You control the path, the destination, and what happens in between.

## Why Use Redirect Links

Your traffic sources (ads, emails, social posts) point at your redirect link. The link points at your landing page. This separation gives you control:

**Change destinations without updating sources.** When your landing page moves or you want to test a new version, update the redirect. Your ads, emails, and posts keep working.

**A/B test with rotation.** Send different visitors to different pages. Measure which converts better. No code changes needed.

**Block bots before they reach your page.** Bot protection runs at the redirect layer. Bad traffic never touches your landing page.

**Track clicks with full visitor data.** Every click is logged with country, device, referrer, and bot classification. You see exactly what's happening.

**Use clean branded URLs.** Instead of long affiliate links with tracking parameters, use `yourdomain.com/offer`. Looks better, builds trust.

## How Redirects Work

When a visitor clicks your link:

1. Their browser requests `yourdomain.com/offer`
2. GuardBot receives the request
3. Bot protection rules are evaluated (if enabled)
4. If the visitor passes, GuardBot returns a 302 redirect to the destination
5. The browser follows the redirect to the final URL
6. The visit is logged regardless of outcome

The whole process takes milliseconds. Visitors don't notice any delay.

## Link Types

**Standard redirect** is one link to one destination. Simple and direct.

**Rotation** sends visitors to different destinations. Sequential rotation cycles through URLs in order. Random rotation picks one each time. Weighted rotation lets you control the split (70% to page A, 30% to page B).

**HTML content** serves a page directly instead of redirecting. Useful for bridge pages, compliance disclosures, or simple landing pages that need to live on your domain.

## Query Parameter Passthrough

When enabled, query strings from the original URL pass through to the destination.

A visitor clicking `yourdomain.com/offer?utm_source=facebook` lands on `landing-page.com/signup?utm_source=facebook`. This preserves tracking parameters, affiliate IDs, or any data you need downstream.

## Path Structure

Paths can include letters, numbers, hyphens, and forward slashes. Use paths like `/offer`, `/summer-sale-2024`, or `/campaigns/facebook/ad1` to organize your links logically.

## Related Topics

- [Getting Started](/guides/quick-start/) for creating your first link
- [About Bot Protection](/features/bot-protection/) for protecting your links
- [Links API Reference](/api/links/) for automation
