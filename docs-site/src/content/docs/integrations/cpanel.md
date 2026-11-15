---
title: How to Connect cPanel
description: Import domains from your cPanel hosting account and auto-configure DNS.
---

Connect your cPanel hosting account to import domains directly into GuardBot. Once imported, DNS is configured automatically.

## Get Your API Token

1. Log into your cPanel hosting account
2. Go to **Security** → **Manage API Tokens**
3. Click **Create** to make a new token
4. Name it `GuardBot` and set expiration to "Never expires"
5. Click **Create** and copy the token immediately (you won't see it again)

If you don't see the API Tokens option, your hosting provider may have disabled API access. Contact their support.

## Connect to GuardBot

1. In your GuardBot dashboard, go to **Domains** → **Connect cPanel**
2. Enter your connection details:

| Field | Value |
|-------|-------|
| **cPanel Host** | Your cPanel URL (e.g., `server.yourhost.com` or `yourhost.com:2083`) |
| **Username** | Your cPanel username |
| **API Token** | The token you created |

3. Click **Test Connection** to verify
4. If the test passes, click **Connect**

### If connection fails

- Double-check the hostname (try with and without port number)
- Verify the username matches your cPanel login exactly
- Make sure you copied the complete API token
- Confirm your hosting provider allows API access

## Import a Domain

1. Go to **Domains** → **cPanel** and click your connection
2. You'll see all domains in your cPanel account (main, addon, subdomains, parked)
3. Click **Import** next to the domain you want

A dialog appears with one checkbox:

**Auto-configure DNS**: When enabled, GuardBot sets up DNS records and SSL certificates automatically. Your domain is ready for bot protection within minutes.

Leave auto-configure enabled unless you manage DNS elsewhere (like Cloudflare).

4. Click **Import** to complete

## Update API Token

If you regenerate your API token in cPanel:

1. Go to **Domains** → **cPanel** and click your connection
2. Click **Edit**
3. Enter the new token
4. Test and save

## Disconnect cPanel

1. Click on your connection
2. Click **Disconnect**
3. Confirm

Your imported domains stay in GuardBot. Only the cPanel link is removed.

## Troubleshooting

**DNS not updating**: Changes can take 5-15 minutes to propagate globally.

**Permission denied**: The cPanel user must have DNS zone access for the domain. Check addon domain permissions.

**Domains not showing**: Only domains with DNS zones in cPanel appear. Domains using external DNS (like Cloudflare) won't show even if hosted on cPanel.

## Comparison

| Feature | cPanel | NameSilo |
|---------|--------|----------|
| **What it connects** | Hosting account | Domain registrar |
| **Domains shown** | Domains hosted on that cPanel | All registered domains |

You can connect both. Each provider shows only the domains it manages.
