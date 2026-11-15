---
title: How to Connect GoDaddy
description: Import domains from your GoDaddy account and auto-configure DNS for bot protection.
---

Connect your GoDaddy account to import domains directly into GuardBot. Once imported, DNS and SSL are configured automatically.

## Get Your API Credentials

1. Go to [GoDaddy Developer Portal](https://developer.godaddy.com/keys)
2. Sign in with your GoDaddy account
3. Click **Create New API Key**
4. Choose **Production** environment
5. Copy both the **Key** and **Secret**

:::caution
The secret is shown only once. Save it somewhere secure before closing the page.
:::

## Connect to GuardBot

1. In your GuardBot dashboard, go to **Domains** → **Connect GoDaddy**
2. Paste your API Key
3. Paste your API Secret
4. Click **Connect**

If successful, your domains appear in a table.

### If connection fails

- Check that you selected **Production**, not OTE (test environment)
- Make sure you copied the full key and secret without extra spaces
- Try regenerating the API key if it was created a while ago

## Import a Domain

1. Find the domain you want in the list
2. Click **Import**

A dialog appears with one checkbox:

**Auto-configure DNS and SSL**: When enabled, GuardBot sets up everything for you. DNS records are created, SSL certificates are generated, and your domain is ready for bot protection within a few minutes.

3. Click **Import** to start

The setup runs in the background. You can close the page and come back later.

## Reconfigure DNS

If you need to update DNS settings for an imported domain:

1. Find the domain in the GoDaddy page
2. Click **Reconfigure**

This refreshes the DNS configuration. Useful after changing server settings in Admin → Servers.

## Refresh Domain List

Click **Refresh** to fetch the latest domains from GoDaddy. Do this after:

- Buying new domains
- Transferring domains into your account
- Domain status changes

## Disconnect GoDaddy

1. Click **Disconnect** in the card header
2. Confirm in the dialog

Your imported domains stay in GuardBot. Only the API connection is removed.

## Manual DNS Setup

If you disabled auto-configure, you can set up DNS manually in your GoDaddy account:

1. Log into [GoDaddy DNS Management](https://dcc.godaddy.com/manage/dns)
2. Select your domain
3. Add A records for `@` and `*` pointing to your GuardBot server IP

You can find the server IP in Admin → Servers.

## Troubleshooting

**"Invalid credentials"**: Double-check both key and secret. Make sure you're using Production keys, not OTE.

**Domains not appearing**: Only domains registered in your account appear. Domains you manage for others won't show.

**DNS not updating**: DNS changes can take 10-30 minutes to propagate globally.

**SSL not working**: If SSL setup fails, try clicking **Reconfigure** after a few minutes. DNS propagation delays are the most common cause.

## Comparison

| Feature | GoDaddy | Cloudflare | NameSilo |
|---------|---------|------------|----------|
| **What it connects** | Registrar + DNS | DNS provider | Registrar |
| **Domains shown** | Your registered domains | Zones you've added | Your registered domains |

You can connect all of them. Each provider shows only the domains it manages.
