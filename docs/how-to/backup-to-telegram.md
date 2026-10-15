# How to Back Up to Telegram

This guide shows you how to back up botginx's database and config to a Telegram chat. For the manifest layout see the [backup manifest reference](../reference/backup-manifest-format.md); to recover, see [How to restore from Telegram](restore-from-telegram.md).

## Prerequisites

1. Create a Telegram bot via [@BotFather](https://t.me/BotFather)
2. Get the bot token (looks like `123456789:ABCdefGHI...`)
3. Start a chat with your bot OR add bot to a group/channel
4. Get the chat ID (use [@userinfobot](https://t.me/userinfobot) or similar)

## Configure Backup

1. Go to **Admin → Backup** (`/admin/backup`)
2. Enter your bot token
3. Enter chat ID(s) - comma-separate multiple
4. Set chunk size (default 20MB, max 20MB for Telegram API)
5. Enable "Include Database" and/or "Include Config"
6. Set backup interval (hours) and retention count
7. Click **Save Settings**

## Run Manual Backup

Click **Run Backup Now** on the settings page. The system will:

1. Dump database using `pg_dump --format=custom`
2. Archive config files to `config.tar.gz`
3. Split large files into 20MB chunks
4. Compute SHA256 for each file/chunk
5. Upload all files to Telegram
6. Capture `file_id` from each upload
7. Create `_manifest.json` with all metadata
8. Upload manifest to Telegram
9. Send completion notification

## What Gets Uploaded

```
📦 db.dump.part001          (if chunked)
📦 db.dump.part002
📦 db.dump.part003
📄 config.tar.gz            (if enabled)
📋 _manifest.json           (REQUIRED for restore)
✅ Backup Complete message
```

---

## Checklist


- [ ] Bot token configured
- [ ] Chat ID configured
- [ ] Bot can send messages to chat
- [ ] "Include Database" enabled
- [ ] Backup runs successfully
- [ ] Save `_manifest.json` somewhere safe
