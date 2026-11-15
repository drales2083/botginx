---
title: How to Connect Telegram
description: Create your own Telegram bot, connect it to GuardBot, and manage links, stats and notifications from chat.
---

This guide shows you how to connect Telegram to GuardBot. You create your own bot with @BotFather, connect it to your account, and then manage links, check stats, and get notifications without opening the dashboard.

## Connect Your Account

Each user runs their own bot. GuardBot does not use a shared bot.

1. Open [@BotFather](https://t.me/BotFather) in Telegram
2. Send `/newbot` and follow the prompts to pick a name and username
3. Copy the bot token BotFather gives you
4. In GuardBot, go to **Settings** → **Telegram**
5. Paste the token and click **Connect**
6. Open a chat with your new bot and send `/start`

The first `/start` links that chat to your account. After that, the bot only responds to you.

## Using the Bot

The bot works like a small app. Each command opens a screen, and navigation happens with inline buttons. The bot edits its message in place instead of posting a new one for every reply, so the chat stays readable.

| Command | What it opens |
|---------|---------------|
| `/start` | Main menu |
| `/help` | Available commands and tips |
| `/stats` | Quick overview of your account |
| `/links` | Your recent links |
| `/newlink` | Create a new protected link |
| `/balance` | Your current balance |
| `/settings` | Notification settings |

Your own messages are removed from the chat as the bot handles them, so the conversation stays tidy.

### Quick Links

To protect a URL, paste it or forward it to the bot. GuardBot creates a protected link right away and replies with the new link.

You can also use `/newlink` with a URL, for example:

```
/newlink https://my-landing-page.com/offer
```

Send `/newlink` on its own to get a prompt for the URL.

### Check Stats

Send `/stats` to see a quick summary of your account. For detailed analytics, use the dashboard or API.

## Configure Notifications

You can change notification settings in either place. Both update the same settings.

**In Telegram:** send `/settings` to the bot and tap a setting to turn it on or off.

**In the dashboard:** go to **Settings** → **Telegram**.

| Notification | Description |
|--------------|-------------|
| **Bot attack alerts** | When blocked traffic spikes |
| **Traffic milestones** | When a link passes a click threshold |
| **SSL expiring** | Before domain certificates need renewal |
| **Low balance** | When your balance runs low |

For traffic milestones, set the threshold (for example, "notify after 100 visits") in the dashboard or in the bot's notification settings.

## Disconnect Your Account

Go to **Settings** → **Telegram** in GuardBot and click **Disconnect**. The bot stops responding to your commands immediately.

## Troubleshooting

**Invalid token**: Check that you copied the full token from @BotFather. It includes a number, a colon, and a second part, for example `123456:ABC-DEF...`. Copy it again from BotFather if needed. Use `/token` in BotFather to view or regenerate it.

**Bot not responding**: Make sure you sent `/start` to the bot after connecting. Only the chat where you sent `/start` is accepted, so use the same chat every time.

**Commands not working**: Verify the bot is connected in **Settings** → **Telegram**. If it is not, paste the token again and click **Connect**. Lapsed subscriptions may also limit bot access.

**Not getting notifications**: Verify the notification is enabled in **Settings** → **Telegram** or with `/settings`. Check your Telegram notification settings too, and make sure the bot chat isn't muted.

**Wrong bot connected**: Disconnect from **Settings** → **Telegram**, then connect the correct bot token.

## Security

Only your own Telegram chat with your bot can control your GuardBot account. Keep your bot token private. Anyone with the token can control your bot.

Don't share your bot token. Don't click links from strangers claiming to be GuardBot. The real bot never asks for your password or API keys.
