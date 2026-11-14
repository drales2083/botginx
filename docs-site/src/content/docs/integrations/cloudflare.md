---
title: How to Connect Cloudflare
description: Import domains from your Cloudflare account and auto-configure DNS for bot protection.
---

This guide shows you how to connect your Cloudflare account to GuardBot. Once connected, you can import domains directly and let GuardBot configure DNS automatically.

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

1. In your GuardBot dashboard, go to **Domains** → **Cloudflare**
2. Enter your Cloudflare account email
3. Paste your Global API Key
4. Click **Connect**

GuardBot tests the connection by fetching your zone list.

### If connection fails

- Make sure you're using the **Global API Key**, not an API Token
- Verify the email matches your Cloudflare account
- Check that your API key hasn't been regenerated

## Import a Domain

1. Your Cloudflare zones appear in a table showing domain name, status, and whether it's already imported
2. Click **Import** next to the domain you want

A dialog appears with DNS options:

**Auto-configure DNS records**: When enabled, GuardBot creates:

- `A @ → [server-ip]` — Root domain points to deploy server
- `A * → [server-ip]` — Wildcard for all subdomains
- `CNAME _acme-challenge → [acme-dns]` — For SSL certificate provisioning

All records are created with:
- **Proxied: OFF** — Traffic goes directly to GuardBot (required for bot protection)
- **TTL: Auto** — Cloudflare manages the TTL

This replaces any existing A records for root and wildcard. Other records (MX, TXT, etc.) remain unchanged.

3. Click **Import** to complete

The domain is ready immediately. DNS changes typically propagate within minutes.

## Reconfigure DNS

If you need to update DNS records for a domain (e.g., after server changes):

1. Find the domain in your list
2. Click **Reconfigure** next to it

This updates the A records to point to the current GuardBot server.

## Refresh Domain List

Click **Refresh** to fetch the latest zones from Cloudflare. Useful after:

- Adding new domains to Cloudflare
- Changing domain status
- Transferring domains between accounts

## Disconnect Cloudflare

1. Click **Disconnect** in the header
2. Confirm in the dialog

Your imported domains remain in GuardBot. Only the API connection is removed.

## Manual DNS Setup

If you disabled auto-configure, set up DNS manually:

1. Log into Cloudflare Dashboard
2. Select your domain
3. Go to **DNS** → **Records**
4. Add or update A records:
   - Name: `@` → Content: your server IP → Proxy: OFF
   - Name: `*` → Content: your server IP → Proxy: OFF

:::important
**Proxy must be OFF** for bot protection to work. Cloudflare's proxy would interfere with visitor fingerprinting.
:::

## Troubleshooting

**Invalid credentials**: Make sure you're using the Global API Key (not an API Token) and the correct email.

**Domains not appearing**: Only active zones appear. Pending or moved domains won't show until fully active.

**DNS not working**: Cloudflare DNS updates are fast, but ISPs may cache records. Wait 15-30 minutes.

**Import errors**: "Domain already imported" means it's already in your GuardBot account. "DNS configuration failed" means the domain was imported but auto-configure failed—try configuring DNS manually.

**Orange cloud still on**: If the proxy icon is orange in Cloudflare, click it to turn it grey (DNS only). Bot protection requires direct traffic.

## Security Notes

- Your API key is encrypted at rest using AES-256-GCM
- Global API keys have full account access—only connect accounts you control
- You can regenerate your API key in Cloudflare anytime (you'll need to reconnect in GuardBot)

## Cloudflare vs NameSilo vs cPanel

| Feature | Cloudflare | NameSilo | cPanel |
|---------|------------|----------|--------|
| **What it connects** | DNS provider | Domain registrar | Hosting account |
| **Domains shown** | Zones in Cloudflare | Registered domains | Hosted domains |
| **DNS control** | Full zone management | Registrar-level | Zone files only |
| **Extra features** | Fast propagation | Domain management | Hosting integration |

You can use all three. Connect each for the domains it manages.
