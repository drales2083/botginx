---
title: Introduction
description: GuardBot protects your redirect links from bots and tracks every click.
---

GuardBot sits between your traffic sources and your landing pages. When someone clicks your link, GuardBot checks if they're human, logs the visit, then sends them through. Bots get blocked. Humans get redirected. You get data on both.

## Why This Exists

Affiliate links, ad campaigns, and landing pages attract bots. Scrapers crawl them. Competitors click them. Fraud networks abuse them. You end up paying for fake traffic or losing commissions to invalid clicks.

GuardBot stops that. Every link you create runs through bot detection before the redirect happens. You control the rules. Block datacenter IPs, challenge suspicious browsers, or let everything through. Your call.

## What You Can Do

**Create redirect links** with custom paths on your own domains. Point `yourdomain.com/offer` at any URL. Change the destination anytime without touching your traffic sources.

**Block bots** using behavioral detection, IP reputation, and fingerprinting. Configure protection per link. Some links need strict filtering. Others don't.

**Track visits** with country, device, referrer, and bot/human classification. Export the data or pull it through the API.

**Host landing pages** on infrastructure that stays up when things get hostile. Built-in SSL, automatic DNS setup, bot protection included.

**Generate QR codes** for any link. Print them, embed them, track scans the same way you track clicks.

**Manage everything via API** or Telegram bot. Create links, check stats, update settings without opening the dashboard.

## How It Works

You add a domain to GuardBot and point its DNS at our servers. Then you create links on that domain. Each link has a destination URL, optional bot protection settings, and analytics.

When traffic hits the link:

1. GuardBot checks the request against your protection rules
2. If the visitor passes, they're redirected to the destination
3. If they fail, they get blocked or challenged
4. Either way, the visit is logged

You see all of this in your dashboard. Clicks, blocks, countries, devices, bots caught.

## Getting Started

The fastest path: create your first link and watch it work.

Go to [Quick Start](/guides/quick-start/) for the steps.
