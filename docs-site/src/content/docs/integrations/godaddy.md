---
title: How to Connect GoDaddy
description: Import domains from your GoDaddy account and auto-configure DNS for bot protection.
---

This guide shows you how to connect your GoDaddy account to GuardBot. Once connected, you can import domains and let GuardBot configure DNS, SSL, and nginx automatically.

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

1. In your GuardBot dashboard, go to **Domains** → **GoDaddy**
2. Paste your API Key
3. Paste your API Secret
4. Click **Connect**

GuardBot tests the connection by fetching your domain list. If it works, your domains appear in a table.

### If connection fails

- Check that you selected **Production**, not OTE (test environment)
- Make sure you copied the full key and secret without extra spaces
- Try regenerating the API key if it was created a while ago

## Import a Domain

1. Your GoDaddy domains appear with their status (Active, Cancelled, etc.)
2. Click **Import** next to the domain you want

A dialog appears with one checkbox:

**Auto-configure DNS and SSL**: When enabled, GuardBot:

- Creates `A @` and `A *` records pointing to the deploy server
- Adds a verification TXT record
- Sets up ACME-DNS delegation for SSL
- Generates a wildcard SSL certificate
- Creates nginx config on the deploy server

This replaces existing A records for root and wildcard. Other records stay untouched.

3. Click **Import** to start

The import runs in the background. DNS setup takes a few seconds. SSL generation takes 2-3 minutes because it waits for DNS propagation.

## What Happens During Import

GuardBot runs these steps in order:

1. **DNS records** — A records for `@` and `*` point to your deploy VPS
2. **Verification TXT** — `_guardbot-verify` proves domain ownership
3. **ACME-DNS CNAME** — `_acme-challenge` delegates to acme-dns for instant SSL renewals
4. **Wait 90 seconds** — DNS needs time to propagate
5. **SSL certificate** — Wildcard cert for `*.yourdomain.com` via Let's Encrypt
6. **nginx config** — HTTPS proxy config on the deploy server

If acme-dns isn't available, GuardBot falls back to a slower method that adds TXT records directly through GoDaddy's API. This takes longer (3-5 minutes) but still works.

## Reconfigure DNS

If your deploy server IP changes:

1. Find the domain in the GoDaddy page
2. Click **Reconfigure**

This updates the A records to point to the new IP. You can also trigger this for all GoDaddy domains at once from Admin → Servers.

## Refresh Domain List

Click **Refresh** to fetch the latest domains from GoDaddy. Do this after:

- Buying new domains
- Transferring domains into your account
- Domain status changes

## Disconnect GoDaddy

1. Click **Disconnect** in the card header
2. Confirm in the dialog

Your imported domains stay in GuardBot. Only the API connection is removed. You won't be able to reconfigure DNS until you reconnect.

## Manual DNS Setup

If you disabled auto-configure, or if automatic setup failed, configure DNS manually:

1. Log into [GoDaddy DNS Management](https://dcc.godaddy.com/manage/dns)
2. Select your domain
3. Delete existing A records for `@` and `*`
4. Add new A records:
   - Name: `@` → Value: your server IP → TTL: 600
   - Name: `*` → Value: your server IP → TTL: 600

:::note
GoDaddy's minimum TTL is 600 seconds (10 minutes). Lower values get rounded up.
:::

## Rate Limits

GoDaddy limits API requests to 60 per minute. GuardBot handles this automatically. If you're importing many domains, they'll process one per second.

## Troubleshooting

**"Invalid credentials"**: Double-check both key and secret. Make sure you're using Production keys, not OTE.

**Domains not appearing**: Only domains registered in your account appear. Domains you manage for others (via delegation) won't show.

**DNS not updating**: GoDaddy DNS can take 10-30 minutes to propagate. Check with `dig yourdomain.com A` to see current records.

**SSL failed**: Check the domain's status in your GuardBot domains list. If it shows an SSL error, the log will say why. Common causes: DNS not propagated yet, rate limited by Let's Encrypt, or firewall blocking port 80.

**nginx config failed**: The deploy server needs to be reachable via SSH. Check Admin → Servers to verify the server status is "ready".

## Security Notes

- API credentials are encrypted at rest with AES-256-GCM
- Credentials are stored per-user, not shared
- GoDaddy API keys can be revoked anytime from the Developer Portal
- Only connect accounts you control

## GoDaddy vs Cloudflare vs NameSilo

| Feature | GoDaddy | Cloudflare | NameSilo |
|---------|---------|------------|----------|
| **What it connects** | Registrar + DNS | DNS provider | Registrar |
| **Domains shown** | Your registered domains | Zones you've added | Your registered domains |
| **DNS control** | Full zone management | Full zone management | Registrar-level |
| **SSL setup** | Auto via acme-dns or TXT | Auto via acme-dns | Auto via acme-dns |

You can use all of them. Connect each for the domains it manages.
