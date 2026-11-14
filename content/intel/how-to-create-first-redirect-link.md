---
title: How to Create Your First Redirect Link
slug: how-to-create-first-redirect-link
date: 2026-11-14
author: GuardBot Team
category: tutorials
tags:
  - getting-started
  - redirect-links
  - bot-protection
excerpt: Step-by-step guide to creating a protected redirect link with bot detection and analytics. Takes about 5 minutes.
featured: true
---

In this tutorial, you'll create a redirect link with bot protection and watch the analytics come in. By the end, you'll have a working link that blocks bots and tracks every click.

## What You'll Need

- A GuardBot account ([sign up free](https://guardbot.sbs/auth/signup) if you don't have one)
- A domain you control, or use a shared domain while testing

## Add Your Domain

First, you need a domain to host your links on. If you're testing with a shared domain, skip to the next step.

1. Go to **Domains** in the sidebar
2. Click **Add Domain**
3. Enter your domain name

GuardBot shows you the DNS records to add. Point your domain's A record at the IP shown. For subdomains, add a wildcard A record too.

DNS propagation takes a few minutes to a few hours. GuardBot checks automatically and adds an SSL certificate when ready.

You'll see a green **Verified** badge when your domain is active.

## Create the Redirect Link

Now let's create your first protected link.

1. Go to **Redirect Links** in the sidebar
2. Click **New Link**
3. Fill in these fields:
   - **Domain**: Pick from your verified domains
   - **Path**: Enter `/offer` (or any path you like)  
   - **Destination**: Enter the URL where visitors should land

Leave bot protection off for now—we'll enable it after testing.

Click **Create**. Your link is now live.

## Test Your Link

Open your new link in a browser. You should land on the destination URL.

Check your dashboard. Within seconds, you'll see your visit in the analytics showing:
- Your country
- Device type
- "Human" classification

Try visiting from your phone too. You'll see it logged separately as "mobile."

## Enable Bot Protection

Now let's add protection.

1. Click on your link in the list to open settings
2. Find the protection options
3. Enable **Block Known Bots**

This catches scrapers and crawlers by their signatures. Save the changes—protection is now active.

## Watch It Work

Visit your link again from your browser. You'll still get through because you're a real human.

Check the **Threat Log** in your dashboard over the next few days. As bots discover your link, you'll see them blocked with reasons like:
- "Datacenter IP"
- "Bot signature detected"
- "Headless browser"

## What You've Built

Your redirect link now:

- Forwards human visitors to your destination
- Blocks known bot signatures
- Logs every visit with country, device, and classification
- Shows real-time analytics

## Next Steps

Now that you have a working link, explore:

- [About Redirect Links](https://docs.guardbot.sbs/features/redirect-links/) - rotation, HTML content, advanced options
- [About Bot Protection](https://docs.guardbot.sbs/features/bot-protection/) - how detection works
- [API Overview](https://docs.guardbot.sbs/api/overview/) - automate link creation
