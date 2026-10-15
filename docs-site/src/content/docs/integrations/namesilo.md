---
title: NameSilo Integration
description: Connect your NameSilo domain registrar account to import domains and auto-configure DNS.
---

Connect your NameSilo account to GuardBot. Import your registered domains directly, configure DNS with one click, and manage everything from one dashboard.

## Why Connect NameSilo?

When you connect NameSilo, GuardBot can:

- **Import domains** directly from your registrar account
- **Auto-configure DNS** records for your domains
- **Set up A records** for root and wildcard subdomains
- **View domain status** including expiration dates
- **Manage DNS** without logging into NameSilo separately

Perfect for users who register domains at NameSilo and want seamless GuardBot integration.

## Getting Your API Key

NameSilo uses API keys for third-party access.

### Step 1: Log Into NameSilo

Go to [namesilo.com](https://www.namesilo.com) and sign in to your account.

### Step 2: Open API Manager

Navigate to **Account** → **API Manager**.

Direct link: [namesilo.com/account/api-manager](https://www.namesilo.com/account/api-manager)

### Step 3: Generate API Key

If you don't have an API key yet:

1. Click **Generate API Key**
2. Enter your account password to confirm
3. Copy the generated key

If you already have a key, it's displayed on this page. Copy it.

### Step 4: Store Securely

Save the API key somewhere safe. You'll need it to connect GuardBot.

## Connecting to GuardBot

### Step 1: Go to NameSilo Settings

In your GuardBot dashboard, navigate to **Domains** → **Connect NameSilo**.

### Step 2: Enter Your API Key

Paste your NameSilo API key into the field.

### Step 3: Connect

Click **Connect**. GuardBot tests the connection by fetching your domain list.

If connection fails:
- Make sure you copied the full API key
- Check that your NameSilo account is active
- Verify API access is enabled on your account

### Step 4: Done

Once connected, you'll see all domains in your NameSilo account listed.

## Importing Domains

After connecting, import domains to GuardBot.

### Step 1: View Your Domains

Your NameSilo domains appear in a table showing:

| Column | Information |
|--------|-------------|
| **Domain** | The domain name |
| **Expires** | Expiration date with warnings for soon-expiring domains |
| **Status** | Active, Expired, or other status |
| **Imported** | Whether it's already in GuardBot |

### Step 2: Import a Domain

Click **Import** next to the domain you want to add.

### Step 3: Configure DNS (Optional)

A dialog appears with DNS options:

**Auto-configure DNS records**: When enabled, GuardBot creates:

- `A @ → [server-ip]` — Root domain points to deploy server
- `A * → [server-ip]` — Wildcard for all subdomains

This replaces any existing A records for root and wildcard. Other records (MX, TXT, CNAME) remain unchanged.

Leave this enabled unless you manage DNS separately.

### Step 4: Complete Import

Click **Import**. GuardBot:

1. Adds the domain to your account
2. Configures DNS via NameSilo API (if enabled)
3. Sets up the domain for redirect links

The domain is ready to use immediately. DNS changes may take up to 15 minutes to propagate globally.

## Managing Your Connection

### View Connection Status

Go to **Domains** → **Connect NameSilo**. The page shows:

- Connection status (connected/disconnected)
- When you connected
- List of domains with import status

### Refresh Domain List

Click **Refresh** to fetch the latest domain list from NameSilo. Useful after:

- Registering new domains
- Renewing domains
- Transferring domains to your account

### Disconnect

To remove the NameSilo connection:

1. Click **Disconnect**
2. Confirm in the dialog

Your imported domains remain in GuardBot. Only the API connection is removed. You won't be able to import new domains or auto-configure DNS until you reconnect.

## DNS Management

### What Gets Configured

When you import with auto-configure enabled:

| Record | Host | Value |
|--------|------|-------|
| A | @ (root) | Deploy server IP |
| A | * (wildcard) | Deploy server IP |

This enables:
- `yourdomain.com` → Your deploy server
- `anything.yourdomain.com` → Your deploy server

### Existing Records

GuardBot removes existing A records for @ and * before adding new ones. Other record types are preserved:

- MX records (email) — untouched
- TXT records (SPF, DKIM, verification) — untouched
- CNAME records — untouched
- NS records — untouched

### Manual DNS Setup

If you disabled auto-configure, set up DNS manually:

1. Log into NameSilo
2. Go to **Domain Manager** → select your domain → **DNS Records**
3. Add/update A records:
   - Host: leave blank (for root) → Value: your server IP
   - Host: `*` → Value: your server IP

### Re-configure DNS

To update DNS for an already-imported domain:

1. Go to the domain's settings page in GuardBot
2. Or delete and re-import the domain with auto-configure enabled

## Domain Status

### Expiration Warnings

Domains expiring within 30 days show a warning badge. Renew at NameSilo to keep them active.

GuardBot doesn't automatically renew domains — that's between you and NameSilo.

### Status Types

| Status | Meaning |
|--------|---------|
| **Active** | Domain is registered and working |
| **Expired** | Domain has expired, may still be recoverable |
| **Locked** | Transfer lock enabled (normal security feature) |
| **Private** | WHOIS privacy enabled |

## Troubleshooting

### Connection Fails

**"Invalid API key"**: Double-check you copied the complete key from NameSilo. API keys are long alphanumeric strings.

**"Connection test failed"**: NameSilo API might be temporarily unavailable. Wait a few minutes and try again.

**"Account not found"**: Verify your NameSilo account is active and in good standing.

### Domains Not Appearing

**Just registered**: New domains may take a few minutes to appear in API results. Click Refresh.

**Transferred domains**: Domains transferred to your account appear after the transfer completes fully.

**Expired domains**: Expired domains still appear but may have limited functionality.

### DNS Not Working

**Changes not propagating**: DNS can take 15-60 minutes to propagate globally. Some ISPs cache longer.

**Wrong IP**: Verify your deploy server IP is correct in GuardBot admin settings.

**NameSilo propagation**: NameSilo's DNS typically updates within seconds, but global propagation takes longer.

### Import Errors

**"Domain already imported"**: The domain is already in your GuardBot account. Check your domain list.

**"DNS configuration failed"**: The domain was imported but DNS auto-configure failed. Check the error message and try configuring DNS manually.

## Security Notes

- Your API key is stored encrypted in GuardBot's database
- The key is never displayed after you enter it
- API keys have full account access — only connect accounts you control
- You can regenerate your API key in NameSilo anytime (you'll need to reconnect in GuardBot)

## Rate Limits

NameSilo's API has rate limits. GuardBot handles this automatically:

- Requests are spaced to respect limits
- Large domain lists may take a few seconds to load
- Import operations include built-in delays

You shouldn't encounter rate limit errors under normal use.

## Comparison: NameSilo vs cPanel

| Feature | NameSilo | cPanel |
|---------|----------|--------|
| **What it connects** | Domain registrar | Hosting account |
| **Domains shown** | All registered domains | Domains hosted on that cPanel |
| **DNS control** | Full (registrar-level) | Zone files only |
| **Use case** | Domains registered at NameSilo | Domains hosted on cPanel servers |

You can use both. Connect NameSilo for domains you register there, and cPanel for domains hosted elsewhere.
