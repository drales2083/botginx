---
title: Telegram Bot
description: Create links, check stats, and get notifications through Telegram.
---

The GuardBot Telegram bot lets you manage your account without opening the dashboard. Create links, check analytics, get alerts when things happen.

## Connecting Your Account

### Step 1: Find the Bot

Go to **Settings** in your GuardBot dashboard. Look for the Telegram section. You'll see a link to start a chat with the bot.

Or search for the bot directly in Telegram using the username shown in settings.

### Step 2: Start the Conversation

Send `/start` to the bot. It responds with a welcome message and asks you to link your account.

### Step 3: Get Your Verification Code

Back in GuardBot settings, click **Connect Telegram**. Copy the verification code that appears.

### Step 4: Send the Code

Paste the code into your Telegram chat with the bot. It verifies your identity and links your account.

Done. The bot now knows who you are.

## Commands

| Command | What It Does |
|---------|--------------|
| `/start` | Initialize or restart the bot |
| `/help` | Show available commands |
| `/stats` | Quick overview of your account stats |
| `/newlink [url]` | Create a new redirect link |
| `/links` | List your recent links |
| `/balance` | Check your account balance |

### Creating Links

Send `/newlink` followed by the destination URL:

```
/newlink https://my-landing-page.com/offer
```

The bot creates the link on your default domain and sends back the URL. Quick way to make links without logging in.

### Checking Stats

Send `/stats` to get a summary:

- Total clicks today
- Active links count
- Recent bot blocks

For detailed analytics, use the dashboard or API.

## Notifications

The bot can alert you when things happen:

**Traffic alerts**: Get notified when a link passes a click threshold. Useful for knowing when a campaign takes off.

**Bot attacks**: Alerts when blocked traffic spikes. Someone might be hitting your links.

**SSL expiring**: Warnings before your domain certificates need renewal.

**Support replies**: Know immediately when an admin responds to your ticket.

**Balance warnings**: Alerts when your balance runs low.

### Configuring Notifications

Go to **Settings** → **Telegram** in your dashboard. Toggle which notifications you want. Set thresholds where applicable (like "notify after 100 visits").

## Security

Only your linked Telegram account can control your GuardBot. Commands are authenticated using your Telegram user ID.

Don't share your verification code. Don't click links from strangers claiming to be GuardBot. The real bot never asks for your password or API keys.

You can unlink your Telegram anytime from Settings. After unlinking, the bot stops responding to your commands.

## Troubleshooting

**Bot doesn't respond**: Check if the bot is online (Telegram shows "last seen" status). Try `/start` again. If still nothing, relink your account.

**Commands fail**: Make sure your account is active. Lapsed subscriptions may limit bot access.

**Not getting notifications**: Verify notifications are enabled in Settings. Check your Telegram notification settings too. Make sure the bot chat isn't muted.

**Wrong account linked**: Unlink from Settings, then link again with the correct Telegram account.
