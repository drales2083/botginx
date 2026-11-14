# How to Connect Cloudflare and Import Domains

This guide shows you how to connect your Cloudflare account to GuardBot, import your domains, and auto-configure DNS for bot protection.

## Prerequisites

- A Cloudflare account with at least one domain
- Your Cloudflare Global API Key
- Admin access to GuardBot

## Step 1: Get Your Cloudflare Global API Key

1. Go to [Cloudflare Dashboard](https://dash.cloudflare.com)
2. Click your profile icon → **My Profile**
3. Select **API Tokens** from the left sidebar
4. Scroll to **API Keys** section
5. Click **View** next to **Global API Key**
6. Enter your password and copy the key

> **Note:** Keep this key secure. It provides full access to your Cloudflare account.

## Step 2: Connect Cloudflare to GuardBot

1. In GuardBot, go to **Domains → Cloudflare** in the sidebar
2. Enter your Cloudflare account email
3. Paste your Global API Key
4. Click **Connect**

GuardBot validates your credentials against the Cloudflare API. If successful, you'll see your domains listed.

## Step 3: Import a Domain

1. Find the domain you want to protect in the list
2. Click **Import**
3. In the modal:
   - **Auto-configure DNS** is enabled by default (recommended)
   - This creates the required A records pointing to GuardBot
4. Click **Import**

The domain is added to GuardBot and DNS is configured automatically.

## Step 4: Verify DNS Configuration

After import, the DNS records are created:

| Record | Name | Value |
|--------|------|-------|
| A | @ (root) | GuardBot server IP |
| A | * (wildcard) | GuardBot server IP |
| CNAME | _acme-challenge | acme-dns delegation |

DNS propagation takes up to 15 minutes. SSL certificate generation starts automatically once DNS is verified.

## Managing Imported Domains

### Reconfigure DNS

If the GuardBot server IP changes, update all DNS records:

1. Click **Reconfigure** next to the domain, or
2. Use the unified **Update All Domain DNS** button on the deploy server page (updates NameSilo, cPanel, and Cloudflare domains together)

### View DNS Records

Click the domain row to view current DNS records directly from Cloudflare.

### Disconnect Cloudflare

1. Click **Disconnect** in the page header
2. Confirm in the modal

Your imported domains remain in GuardBot. Only the API connection is removed.

## Troubleshooting

### "Invalid credentials" error

- Verify you're using the **Global API Key**, not an API Token
- Confirm the email matches your Cloudflare account
- Check the API key hasn't been regenerated

### Domain not appearing in list

- The domain must be active in Cloudflare (not pending or moved)
- Refresh the page to reload domains from the API

### DNS changes not taking effect

- Wait 15 minutes for propagation
- Check Cloudflare dashboard to confirm records were created
- Ensure Cloudflare proxy (orange cloud) is disabled for A records

## Related

- [Cloudflare Integration Reference](../reference/cloudflare-integration.md) — API details and configuration options
- [About DNS Auto-Configuration](../explanation/acme-dns-delegation.md) — How SSL certificate provisioning works
