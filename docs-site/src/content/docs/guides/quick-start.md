---
title: Getting Started
description: Create your first protected redirect link in five minutes.
---

In this tutorial, we'll create a redirect link with bot protection and watch the analytics come in. By the end, you'll have a working link that blocks bots and tracks every click.

## What You'll Need

- A GuardBot account (sign up at guardbot.sbs if you don't have one)
- A domain you control, or use a shared domain while testing

## Add Your Domain

First, we need a domain to host your links on. If you're using a shared domain for testing, skip ahead to creating a link.

Go to **Domains** in the sidebar and click **Add Domain**. Enter your domain name.

GuardBot shows you the DNS records to add. Point your domain's A record at the IP shown. If you want subdomains too, add a wildcard A record.

DNS propagation takes anywhere from a few minutes to a few hours. GuardBot checks automatically. Once it sees the right records, your domain goes active and gets an SSL certificate.

You'll see a green "Verified" badge when your domain is ready.

## Create Your First Link

Now let's create a redirect link.

Go to **Redirect Links** in the sidebar and click **New Link**.

Fill in these fields:

- **Domain**: Pick from your verified domains
- **Path**: Enter `/offer` (or any path you like)
- **Destination**: Enter the URL where visitors should end up

Leave bot protection off for now. We'll enable it after testing.

Click **Create**. Your link is now live.

## Test the Link

Open your new link in a browser. You should land on the destination URL you entered.

Now check your dashboard. Within a few seconds, you'll see your visit appear in the analytics. Notice it shows your country, device type, and that it passed as human traffic.

Try visiting the link from your phone too. You'll see that visit logged separately with "mobile" as the device type.

## Enable Bot Protection

Now let's add protection. Go back to your link's settings by clicking on it in the list.

Find the protection options and enable **Block Known Bots**. This catches scrapers and crawlers by their user agent signatures.

Save the changes. Protection is now active.

## Watch It Work

Visit your link again from your browser. You'll still get through because you're a real human.

To see the protection in action, check the **Threat Log** in your dashboard over the next few days. As bots discover your link, you'll see them getting blocked with reasons like "datacenter IP" or "bot signature detected."

## What You've Built

You now have a redirect link that:

- Forwards human visitors to your destination
- Blocks known bot signatures
- Logs every visit with country, device, and bot classification
- Shows you exactly what's happening in real-time analytics

## What's Next

Now that you have a working link, explore these topics:

- [About Redirect Links](/features/redirect-links/) to understand rotation, HTML content, and advanced options
- [About Bot Protection](/features/bot-protection/) to learn how detection works and tune your settings
- [About Analytics](/features/analytics/) to understand your traffic data
- [API Overview](/api/overview/) if you want to automate link creation
