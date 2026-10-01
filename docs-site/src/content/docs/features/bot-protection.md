---
title: Bot Protection
description: Block scrapers, fake clicks, and unwanted traffic before it reaches your pages.
---

Bot protection runs on every click before the redirect happens. GuardBot checks the visitor against your rules and either lets them through or blocks them. You configure what gets blocked per link.

## How Detection Works

GuardBot examines each request using multiple signals:

**IP reputation** checks the visitor's IP against known bot networks, datacenter ranges, VPN providers, and Tor exit nodes. Bad IPs get flagged before anything else.

**User agent analysis** looks at the browser string. Known bot signatures (Googlebot, scrapers, headless browsers) get caught here.

**Behavioral signals** watch for patterns that humans don't produce. Requests with no JavaScript execution, missing headers, or impossible timing get flagged.

**Fingerprinting** (in challenge mode) collects browser characteristics to identify automation tools even when they try to look human.

## Protection Levels

Each link can run at a different protection level.

### Off

No protection. All traffic passes through and gets redirected. You still get analytics, but nothing gets blocked.

Use this when you're testing, or when you genuinely want all traffic including bots.

### Block Known Bots

Catches obvious bots: search engine crawlers, known scraper signatures, headless browser user agents.

Doesn't block datacenter IPs or VPNs. Real users on corporate networks or privacy tools still get through.

This is a good starting point. It stops the obvious stuff without risking false positives.

### Block Datacenter IPs

Adds IP-based blocking on top of user agent checks. Traffic from AWS, Google Cloud, DigitalOcean, and similar providers gets blocked.

Most real users browse from residential or mobile IPs. Traffic from datacenters is usually bots, scrapers, or competitors clicking your ads.

Exception: some corporate users browse through cloud-hosted proxies. If your audience includes enterprise users, test this carefully.

### Block VPNs and Proxies

Extends datacenter blocking to include commercial VPN services and known proxy networks.

This catches more bad traffic but also blocks privacy-conscious users. Works well for campaigns where you don't expect VPN usage.

### Challenge Mode

Instead of an instant redirect, visitors see a verification page first. The page runs JavaScript checks and collects fingerprint data. Visitors who pass get redirected. Those who fail get blocked.

Challenge mode catches sophisticated bots that mimic real browsers. It adds a small delay (under a second for real users), so use it when you need strict filtering and can accept the friction.

## Per-Link Settings

Open any link and go to its protection settings. You can toggle:

| Setting | What It Does |
|---------|--------------|
| Block Known Bots | User agent detection |
| Block Datacenter IPs | IP range blocking |
| Block VPNs | Commercial VPN blocking |
| Block Tor | Tor exit node blocking |
| Challenge Mode | JavaScript verification |
| Block Empty Referrers | Require a referrer header |

Mix and match based on what your traffic looks like.

## IP Whitelisting

Sometimes you need to let specific IPs through regardless of rules. Partners, affiliates, your own testing.

Go to **IP Lists** in the sidebar. Add IPs to your whitelist. Whitelisted IPs bypass all protection on all your links.

You can add single IPs or CIDR ranges like `192.168.1.0/24`.

## Viewing Blocked Traffic

The **Threat Log** shows every blocked request:

- Timestamp
- IP address
- Block reason (datacenter, bot signature, failed challenge, etc.)
- User agent
- Requested URL

Use this to verify protection is working and to catch false positives. If you see a legitimate IP getting blocked, add it to your whitelist.

## Tuning Your Settings

Start with light protection and tighten as needed.

1. Enable "Block Known Bots" first
2. Check your analytics and threat log after a day
3. If you're still seeing junk traffic, add datacenter blocking
4. If bots are sophisticated, enable challenge mode
5. Whitelist any legitimate IPs that got caught

The goal is blocking bad traffic without losing real visitors. Check your conversion rates after enabling each level. If they drop unexpectedly, you might be blocking real users.
