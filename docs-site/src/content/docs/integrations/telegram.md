---
title: How to Connect Telegram
description: Manage links, check stats, and get notifications through Telegram.
---

This guide shows you how to connect your Telegram account to GuardBot. Once connected, you can create links, check analytics, and receive notifications without opening the dashboard.

## Connect Your Account

1. Go to **Settings** in your GuardBot dashboard
2. Find the Telegram section and click the link to start a chat with the bot (or search for the bot username shown)
3. Send `/start` to the bot
4. Back in GuardBot settings, click **Connect Telegram** and copy the verification code
5. Paste the code into your Telegram chat with the bot

The bot verifies your identity and links your account.

## Available Commands

| Command | Description |
|---------|-------------|
| `/start` | Initialize or restart the bot |
| `/help` | Show available commands |
| `/stats` | Quick overview of your account stats |
| `/newlink [url]` | Create a new redirect link |
| `/links` | List your recent links |
| `/balance` | Check your account balance |

### Create a Link

Send `/newlink` followed by the destination URL:

```
/newlink https://my-landing-page.com/offer
```

The bot creates the link on your default domain and sends back the URL.

### Check Stats

Send `/stats` to get:

- Total clicks today
- Active links count
- Recent bot blocks

For detailed analytics, use the dashboard or API.

## Configure Notifications

Go to **Settings** → **Telegram** in your dashboard to toggle notifications:

| Notification | Description |
|--------------|-------------|
| **Traffic alerts** | When a link passes a click threshold |
| **Bot attacks** | When blocked traffic spikes |
| **SSL expiring** | Before domain certificates need renewal |
| **Support replies** | When an admin responds to your ticket |
| **Balance warnings** | When your balance runs low |

Set thresholds where applicable (like "notify after 100 visits").

## Unlink Your Account

Go to **Settings** in GuardBot and click **Disconnect Telegram**. The bot stops responding to your commands immediately.

## Troubleshooting

**Bot doesn't respond**: Check if the bot is online (Telegram shows "last seen" status). Try `/start` again. If still nothing, relink your account.

**Commands fail**: Make sure your account is active. Lapsed subscriptions may limit bot access.

**Not getting notifications**: Verify notifications are enabled in Settings. Check your Telegram notification settings too. Make sure the bot chat isn't muted.

**Wrong account linked**: Unlink from Settings, then link again with the correct Telegram account.

## Security

Only your linked Telegram account can control your GuardBot. Commands are authenticated using your Telegram user ID.

Don't share your verification code. Don't click links from strangers claiming to be GuardBot. The real bot never asks for your password or API keys.
