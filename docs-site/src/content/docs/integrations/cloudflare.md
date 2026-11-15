---
title: How to Connect Cloudflare
description: Import domains from your Cloudflare account and auto-configure DNS for bot protection.
---

Connect your Cloudflare account to import domains directly into GuardBot. Once imported, DNS and SSL are configured automatically.

## Get Your Global API Key

1. Log into [Cloudflare Dashboard](https://dash.cloudflare.com)
2. Click your profile icon → **My Profile**
3. Go to **API Tokens** in the sidebar
4. Scroll to **API Keys** section
5. Click **View** next to **Global API Key**
6. Enter your password and copy the key

:::caution
Keep this key secure. The Global API Key has full access to your Cloudflare account.
:::

## Connect to GuardBot

1. In your GuardBot dashboard, go to **Domains** → **Connect Cloudflare**
2. Enter your Cloudflare account email
3. Paste your Global API Key
4. Click **Connect**

If successful, your domains appear in a table.

### If connection fails

- Make sure you're using the **Global API Key**, not an API Token
- Verify the email matches your Cloudflare account
- Check that your API key hasn't been regenerated

## Import a Domain

1. Find the domain you want in the list
2. Click **Import**

A dialog appears with one checkbox:

**Auto-configure DNS records**: When enabled, GuardBot sets up DNS records and SSL certificates automatically. Your domain is ready for bot protection within minutes.

:::important
**Proxy must be OFF** for bot protection to work. GuardBot automatically disables Cloudflare's proxy (orange cloud) when importing.
:::

3. Click **Import** to complete

## Reconfigure DNS

If you need to update DNS settings for an imported domain:

1. Find the domain in the Cloudflare page
2. Click **Reconfigure**

This refreshes the DNS configuration. Useful after changing server settings in Admin → Servers.

## Refresh Domain List

Click **Refresh** to fetch the latest zones from Cloudflare. Do this after:

- Adding new domains to Cloudflare
- Changing domain status
- Transferring domains between accounts

## Disconnect Cloudflare

1. Click **Disconnect** in the header
2. Confirm in the dialog

Your imported domains stay in GuardBot. Only the API connection is removed.

## Manual DNS Setup

If you disabled auto-configure, set up DNS manually in Cloudflare:

1. Log into Cloudflare Dashboard
2. Select your domain
3. Go to **DNS** → **Records**
4. Add A records for `@` and `*` pointing to your GuardBot server IP
5. Make sure the proxy is **OFF** (grey cloud, not orange)

You can find the server IP in Admin → Servers.

## Troubleshooting

**Invalid credentials**: Make sure you're using the Global API Key (not an API Token) and the correct email.

**Domains not appearing**: Only active zones appear. Pending or moved domains won't show until fully active.

**DNS not working**: DNS updates can take 15-30 minutes to propagate globally.

**Orange cloud still on**: If the proxy icon is orange in Cloudflare, click it to turn it grey. Bot protection requires direct traffic.

## Comparison

| Feature | Cloudflare | NameSilo | cPanel |
|---------|------------|----------|--------|
| **What it connects** | DNS provider | Domain registrar | Hosting account |
| **Domains shown** | Zones in Cloudflare | Registered domains | Hosted domains |

You can connect all of them. Each provider shows only the domains it manages.
