---
title: About Analytics
description: Understanding your traffic data and what gets tracked.
---

Every click through GuardBot gets logged. You see where your traffic comes from, what devices they use, and how much of it is bots.

## What Gets Tracked

Every request logs:

- **Timestamp** of when the click happened
- **IP address** of the visitor (hashed in exports for privacy)
- **Country** derived from IP geolocation
- **Device type** (desktop, mobile, tablet)
- **Browser and OS** from the user agent
- **Referrer URL** showing where the click originated
- **Bot detection result** (passed, blocked, challenged)
- **Protection rule** that triggered, if blocked

GuardBot doesn't track across sites or build user profiles. Each visit is logged independently. When a visitor leaves, that's it.

## Dashboard Metrics

The main dashboard shows aggregate stats across all your links:

**Total Visits** is your cumulative click count since you started.

**Today's Visits** gives you a quick pulse check on current traffic.

**Unique Visitors** deduplicates by IP to show actual people rather than repeat clicks.

**Bot Rate** shows what percentage of traffic gets blocked or flagged. High bot rates mean either your protection is working or you've got a traffic quality problem worth investigating.

These update in near real-time. A click shows up within seconds.

## Per-Link Analytics

Click into any link to see its specific data:

**Traffic timeline** shows visits by hour, day, or week. Spot patterns like weekend spikes or campaign end dates.

**Geographic breakdown** ranks visitors by country. Check if your targeting works. If you're running US-only ads and half your traffic is from Russia, something's wrong.

**Device split** shows desktop vs mobile vs tablet. If 90% is mobile, make sure your landing pages work on phones.

**Bot vs human** breaks down traffic that passed protection vs traffic that got blocked.

**Referrers** show where clicks originate: direct visits, social platforms, search engines, or specific URLs.

## Date Filtering

Narrow your view with date filters:

- Today
- Yesterday
- Last 7 days
- Last 30 days
- Custom range

All metrics recalculate based on your selection.

## Exporting Data

Export analytics as CSV for reporting, deeper analysis, or importing into other tools. Exports include timestamp, country, device type, referrer, and bot status.

IP addresses are hashed in exports to protect visitor privacy while still letting you identify patterns.

## Real-Time Monitoring

The antibot dashboard shows live traffic: requests per minute, active blocked IPs, recent threat log entries. Useful for watching a campaign launch or investigating a spike as it happens.

## Data Retention

Analytics data stays available indefinitely on active accounts. If your subscription lapses, data may be purged after 30 days. Export anything important before letting your subscription expire.

## Related Topics

- [Analytics API Reference](/api/analytics/) for pulling data programmatically
- [About Bot Protection](/features/bot-protection/) for understanding the bot/human classification
