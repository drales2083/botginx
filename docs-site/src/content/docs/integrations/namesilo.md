---
title: How to Connect NameSilo
description: Import domains from your NameSilo registrar account and auto-configure DNS.
---

This guide shows you how to connect your NameSilo account to GuardBot. Once connected, you can import registered domains directly and let GuardBot configure DNS automatically.

## Get Your API Key

1. Log into [namesilo.com](https://www.namesilo.com)
2. Go to **Account** → **API Manager** (direct link: [namesilo.com/account/api-manager](https://www.namesilo.com/account/api-manager))
3. If you don't have an API key, click **Generate API Key** and enter your password to confirm
4. Copy the API key

## Connect to GuardBot

1. In your GuardBot dashboard, go to **Domains** → **Connect NameSilo**
2. Paste your NameSilo API key
3. Click **Connect**

GuardBot tests the connection by fetching your domain list.

### If connection fails

- Make sure you copied the full API key
- Check that your NameSilo account is active
- Verify API access is enabled on your account

## Import a Domain

1. Your NameSilo domains appear in a table showing domain name, expiration date, status, and whether it's already imported
2. Click **Import** next to the domain you want

A dialog appears with DNS options:

**Auto-configure DNS records**: When enabled, GuardBot creates:

- `A @ → [server-ip]` — Root domain points to deploy server
- `A * → [server-ip]` — Wildcard for all subdomains

This replaces any existing A records for root and wildcard. Other records (MX, TXT, CNAME) remain unchanged.

3. Click **Import** to complete

The domain is ready immediately. DNS changes may take up to 15 minutes to propagate globally.

## Refresh Domain List

Click **Refresh** to fetch the latest list from NameSilo. Useful after:

- Registering new domains
- Renewing domains
- Transferring domains to your account

## Disconnect NameSilo

1. Click **Disconnect**
2. Confirm in the dialog

Your imported domains remain in GuardBot. Only the API connection is removed.

## Manual DNS Setup

If you disabled auto-configure, set up DNS manually:

1. Log into NameSilo
2. Go to **Domain Manager** → select your domain → **DNS Records**
3. Add or update A records:
   - Host: (leave blank for root) → Value: your server IP
   - Host: `*` → Value: your server IP

## Troubleshooting

**Invalid API key**: Double-check you copied the complete key. API keys are long alphanumeric strings.

**Domains not appearing**: New or transferred domains may take a few minutes to appear. Click Refresh.

**DNS not working**: DNS can take 15-60 minutes to propagate globally. NameSilo's DNS typically updates within seconds, but ISPs may cache longer.

**Import errors**: "Domain already imported" means it's already in your GuardBot account. "DNS configuration failed" means the domain was imported but auto-configure failed—try configuring DNS manually.

## Security Notes

- Your API key is stored encrypted
- API keys have full account access—only connect accounts you control
- You can regenerate your API key in NameSilo anytime (you'll need to reconnect in GuardBot)

## NameSilo vs cPanel

| Feature | NameSilo | cPanel |
|---------|----------|--------|
| **What it connects** | Domain registrar | Hosting account |
| **Domains shown** | All registered domains | Domains hosted on that cPanel |
| **DNS control** | Full (registrar-level) | Zone files only |

You can use both. Connect NameSilo for domains you register there, and cPanel for domains hosted elsewhere.
