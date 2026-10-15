---
title: About Bot Protection
description: How GuardBot detects and blocks unwanted traffic.
---

Bot protection runs on every click before the redirect happens. GuardBot checks the visitor against your rules and either lets them through or blocks them. You configure what gets blocked per link.

## Why Block Bots

Not all traffic is created equal. Bots inflate your analytics, drain your ad budgets, and can abuse your landing pages. They include:

**Scrapers** that harvest content, prices, or competitor intelligence. They hit your links repeatedly, looking for changes.

**Click fraud** from competitors or fraud networks trying to exhaust your ad spend. Each fake click costs you money.

**Crawlers** from search engines and indexing services. Sometimes useful, sometimes just noise in your data.

**Automation tools** running in datacenters, probing your infrastructure or testing stolen credentials.

By filtering this traffic at the redirect layer, bots never reach your actual landing pages. Your analytics stay clean. Your conversions stay real.

## How Detection Works

GuardBot examines each request using multiple signals:

**IP reputation** checks the visitor's IP against known bot networks, datacenter ranges, VPN providers, and Tor exit nodes. Bad IPs get flagged before anything else runs.

**User agent analysis** looks at the browser string. Known bot signatures—Googlebot, scrapers, headless browsers like Puppeteer—get caught here.

**Behavioral signals** watch for patterns humans don't produce. Requests with no JavaScript execution, missing standard headers, or impossible timing get flagged.

**Fingerprinting** (in challenge mode) collects browser characteristics to identify automation tools even when they try to look human.

No single signal is definitive. GuardBot combines them to make decisions. A request from a datacenter IP with a headless browser user agent is almost certainly a bot. A residential IP with a normal Chrome user agent is probably human.

## Protection Levels

Each link can run at a different level:

**Off** lets all traffic through. You still get analytics, but nothing gets blocked. Use this when testing or when you genuinely want all traffic.

**Block Known Bots** catches obvious bots: search engine crawlers, known scraper signatures, headless browser user agents. Real users on corporate networks or VPNs still get through. Good starting point with minimal false positives.

**Block Datacenter IPs** adds IP-based blocking. Traffic from AWS, Google Cloud, DigitalOcean, and similar providers gets blocked. Most real users browse from residential or mobile IPs. Traffic from datacenters is usually bots.

**Block VPNs and Proxies** extends datacenter blocking to commercial VPN services and proxy networks. Catches more bad traffic but also blocks privacy-conscious users.

**Challenge Mode** shows a verification page before redirecting. The page runs JavaScript checks and collects fingerprint data. Catches sophisticated bots that mimic real browsers. Adds a small delay (under a second for real users).

## Finding the Right Balance

Stricter protection catches more bots but risks blocking real users. The right level depends on your traffic:

If you're running ads to a general audience, start with "Block Known Bots" and add datacenter blocking if you're still seeing junk.

If your audience is enterprise users on corporate networks, be careful with IP-based blocking. Some legitimate traffic comes through cloud-hosted proxies.

If you're getting sophisticated bot attacks, enable challenge mode. The extra friction is worth it for clean data.

Watch your conversion rates after enabling protection. If they drop unexpectedly, you might be blocking real users. Check the threat log to see what's getting caught.

## IP Whitelisting

Some IPs need to bypass protection regardless of rules. Partners, affiliates, your own testing. Add them to your whitelist, and they'll pass through without checks.

## Viewing Blocked Traffic

The threat log shows every blocked request: timestamp, IP, block reason, user agent, and requested URL. Use it to verify protection is working and catch false positives. If you see a legitimate IP getting blocked, add it to your whitelist.

## Related Topics

- [Getting Started](/guides/quick-start/) for enabling protection on a link
- [IP Lists API Reference](/api/iplists/) for managing whitelists via API
