---
title: Analytics
description: Track clicks, visitors, countries, devices, and bot detection rates.
---

Every click through GuardBot gets logged. You see where your traffic comes from, what devices they use, and how much of it is bots.

## Dashboard Overview

The main dashboard shows aggregate stats across all your links:

- **Total Visits** since you started
- **Today's Visits** for quick pulse checks
- **Unique Visitors** deduplicated by IP
- **Bot Rate** as a percentage of total traffic

These update in near real-time. A click shows up within seconds.

## Per-Link Analytics

Click into any link to see its specific data:

### Traffic Over Time

A chart showing visits by hour, day, or week. Switch between views to spot patterns. Maybe your traffic spikes on weekends. Maybe it died after a campaign ended. The timeline tells you.

### Geographic Breakdown

Where your visitors come from, ranked by country. Useful for checking if your targeting works. If you're running US-only ads and half your traffic is from Russia, something's wrong.

### Device Split

Desktop vs mobile vs tablet. Tells you how people access your links. If 90% is mobile, make sure your landing pages work on phones.

### Bot vs Human

A breakdown of traffic that passed protection vs traffic that got blocked. High bot rates mean either your protection is working, or you've got a traffic quality problem worth investigating.

### Referrers

Where clicks originate. Direct visits, social platforms, search engines, or specific URLs. Helps you understand which traffic sources actually send people.

## Filtering Data

Use date filters to narrow your view:

- Today
- Yesterday  
- Last 7 days
- Last 30 days
- Custom range

The numbers recalculate based on your selection.

## Exporting Data

Click **Export** to download analytics as CSV. The export includes:

- Timestamp
- IP (hashed for privacy)
- Country
- Device type
- Referrer
- Bot status (passed, blocked, challenged)

Use exports for reporting, deeper analysis, or importing into other tools.

## Real-Time Monitoring

The **Antibot Dashboard** shows live traffic:

- Requests per minute
- Active blocked IPs
- Recent threat log entries

Good for watching a campaign launch or investigating a traffic spike as it happens.

## API Access

Pull analytics programmatically through the API:

```bash
curl "https://guardbot.sbs/api/v1/analytics/links/lnk_abc123" \
  -H "Authorization: Bearer YOUR_API_KEY"
```

Returns JSON with the same data you see in the dashboard. Useful for building custom dashboards or automated reporting.

See [Analytics API](/api/analytics/) for full endpoint documentation.

## What Gets Tracked

Every request logs:

- Timestamp
- IP address
- Country (from IP geolocation)
- Device type
- Browser and OS
- Referrer URL
- Bot detection result
- Protection rule that triggered (if blocked)

GuardBot doesn't track across sites or build user profiles. Each visit is logged independently. When a visitor leaves, that's it.

## Data Retention

Analytics data stays available indefinitely on active accounts. If your account lapses, data may be purged after 30 days.

Export anything important before letting your subscription expire.
