---
title: How to Connect NameSilo
description: Import domains from your NameSilo registrar account and auto-configure DNS.
---

Connect your NameSilo account to import registered domains directly into GuardBot. Once imported, DNS is configured automatically.

## Get Your API Key

1. Log into [namesilo.com](https://www.namesilo.com)
2. Go to **Account** → **API Manager** (direct link: [namesilo.com/account/api-manager](https://www.namesilo.com/account/api-manager))
3. If you don't have an API key, click **Generate API Key** and enter your password to confirm
4. Copy the API key

## Connect to GuardBot

1. In your GuardBot dashboard, go to **Domains** → **Connect NameSilo**
2. Paste your NameSilo API key
3. Click **Connect**

If successful, your domains appear in a table.

### If connection fails

- Make sure you copied the full API key
- Check that your NameSilo account is active
- Verify API access is enabled on your account

## Import a Domain

1. Find the domain you want in the list
2. Click **Import**

A dialog appears with one checkbox:

**Auto-configure DNS records**: When enabled, GuardBot sets up DNS records automatically. Your domain is ready for bot protection within minutes.

3. Click **Import** to complete

## Refresh Domain List

Click **Refresh** to fetch the latest list from NameSilo. Do this after:

- Registering new domains
- Renewing domains
- Transferring domains to your account

## Disconnect NameSilo

1. Click **Disconnect**
2. Confirm in the dialog

Your imported domains stay in GuardBot. Only the API connection is removed.

## Manual DNS Setup

If you disabled auto-configure, set up DNS manually in NameSilo:

1. Log into NameSilo
2. Go to **Domain Manager** → select your domain → **DNS Records**
3. Add A records for root and `*` pointing to your GuardBot server IP

You can find the server IP in Admin → Servers.

## Troubleshooting

**Invalid API key**: Double-check you copied the complete key. API keys are long alphanumeric strings.

**Domains not appearing**: New or transferred domains may take a few minutes to appear. Click Refresh.

**DNS not working**: DNS can take 15-60 minutes to propagate globally.

## Comparison

| Feature | NameSilo | cPanel |
|---------|----------|--------|
| **What it connects** | Domain registrar | Hosting account |
| **Domains shown** | All registered domains | Domains hosted on that cPanel |

You can connect both. Each provider shows only the domains it manages.
