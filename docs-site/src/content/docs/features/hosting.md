---
title: Hosting
description: Deploy landing pages on infrastructure that handles abuse reports and stays online.
---

GuardBot Hosting gives you web hosting built for pages that attract complaints. Affiliate landing pages, aggressive marketing, anything where uptime matters more than a polite terms of service.

## What You Get

**Panel access** to manage files, databases, email, and domains. You get cPanel or CloudPanel depending on the server.

**Automatic SSL** via Let's Encrypt. Add a domain, point DNS, get a certificate. No manual setup.

**Bot protection** on every domain. Same detection that powers your redirect links, built into the hosting.

**Analytics** for hosted domains. See visitors, countries, devices, and bot traffic without adding tracking scripts.

## Buying a Hosting Package

Go to **Hosting** in the sidebar. Click **Buy Hosting**.

Pick a package based on storage and bandwidth. Complete payment using your account balance (top up via crypto if needed).

After purchase, your account provisions automatically. Takes a few minutes. You'll see your panel URL and credentials once it's ready.

## Accessing Your Panel

Click **Show Credentials** on your hosting account card. You'll see:

- Panel URL
- Username
- Password

Log into the panel to manage files, create email accounts, add databases, and configure settings.

## Adding Domains

### From the Hosting Panel

Use your panel's domain management to add domains. The exact steps depend on whether you have cPanel or CloudPanel.

### DNS Configuration

Point your domain at the hosting server IP:

```
Type: A
Name: @
Value: [Server IP from your account]
TTL: 300
```

For wildcard subdomains:

```
Type: A  
Name: *
Value: [Server IP]
TTL: 300
```

DNS propagation takes minutes to hours. Once GuardBot sees the records, SSL gets issued automatically.

## File Management

Access files through:

**File Manager** in your hosting panel. Web-based, works for quick edits and uploads.

**FTP/SFTP** using credentials from your panel. Better for bulk uploads or syncing from your local machine.

**SSH** if your package includes shell access. Full command line control.

## Bot Protection on Hosted Domains

Every domain on your hosting account gets bot protection automatically. Configure it per domain:

1. Go to your domain in GuardBot (not the hosting panel)
2. Open protection settings
3. Toggle the rules you want

Traffic stats show up in GuardBot analytics alongside your redirect link data.

## Backups

Your hosting panel may include backup tools. Use them.

GuardBot doesn't manage backups for you. Download copies of important files and databases regularly. If something breaks, you're responsible for restoring it.

## Renewals

Hosting packages bill monthly against your account balance. Make sure you've got funds to cover renewal. If your balance runs dry, the account gets suspended until you top up.

Check **Payments** to see your balance and add funds.
