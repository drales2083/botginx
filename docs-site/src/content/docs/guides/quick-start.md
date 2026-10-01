---
title: Quick Start
description: Create your first protected redirect link in under five minutes.
---

This guide gets you from zero to a working link with bot protection. Takes about five minutes.

## What You'll Need

- A GuardBot account (sign up at guardbot.sbs if you don't have one)
- A domain you control, or use a shared domain while testing

## Step 1: Add a Domain

You can skip this if you're using a shared domain for testing.

Go to **Domains** in the sidebar. Click **Add Domain**. Enter your domain name.

GuardBot shows you the DNS records to add. Point your domain's A record at the IP shown. If you want subdomains too, add a wildcard A record.

DNS propagation takes anywhere from a few minutes to a few hours. GuardBot checks automatically. Once it sees the right records, your domain goes active and gets an SSL certificate.

## Step 2: Create a Link

Go to **Redirect Links** in the sidebar. Click **New Link**.

Fill in:

- **Domain**: Pick from your verified domains
- **Path**: The URL path (like `/offer` or `/promo-summer`)
- **Destination**: Where visitors should end up

Leave bot protection off for now. You can turn it on after testing.

Click **Create**. Your link is live.

## Step 3: Test It

Open your new link in a browser. You should land on the destination URL.

Check your dashboard. The visit shows up in analytics within a few seconds. You'll see the country, device type, and whether it was flagged as a bot.

## Step 4: Enable Bot Protection

Go back to your link's settings. Turn on bot protection. You've got options:

- **Block known bots**: Catches scrapers and crawlers by user agent
- **Block datacenter IPs**: Stops traffic from hosting providers and VPNs
- **Challenge mode**: Shows a verification page before redirecting

Start with "Block known bots" and see how your traffic looks. Add more protection if you're still seeing junk.

## Step 5: Check Analytics

Click into your link to see visit data. You'll find:

- Total clicks and unique visitors
- Country breakdown
- Device split (desktop, mobile, tablet)
- Bot vs human ratio
- Timeline showing traffic over time

## What's Next

You've got a working link. Here's where to go from here:

- [Redirect Links](/features/redirect-links/) for link types, rotation, and advanced settings
- [Bot Protection](/features/bot-protection/) for fine-tuning your blocking rules
- [Analytics](/features/analytics/) for understanding your traffic data
- [API Reference](/api/overview/) if you want to automate link creation
