# How to Back Up to Telegram

This guide shows you how to back up your botginx database and configuration to a Telegram chat.

## Prerequisites

1. A Telegram bot (create via [@BotFather](https://t.me/BotFather))
2. The bot token (looks like `123456789:ABCdefGHI...`)
3. A chat with your bot (private chat, group, or channel)
4. The chat ID (use [@userinfobot](https://t.me/userinfobot) to find it)

## Step 1: Configure Backup Settings

1. Go to **Admin → Backup** (`/admin/backup`)
2. Enter your bot token
3. Enter chat ID(s) — comma-separate for multiple
4. Set chunk size (default 20MB, max 20MB for Telegram API)
5. Enable "Include Database" and/or "Include Config"
6. Set backup interval (hours) and retention count
7. Click **Save Settings**

## Step 2: Run Manual Backup

Click **Run Backup Now** on the settings page.

The system will:

1. Dump database using `pg_dump --format=custom`
2. Archive config files to `config.tar.gz`
3. Split large files into 20MB chunks
4. Compute SHA256 for each file/chunk
5. Upload all files to Telegram
6. Create `_manifest.json` with all metadata
7. Send completion notification

## What Gets Uploaded

```
📦 db.dump.part001          (if chunked)
📦 db.dump.part002
📦 db.dump.part003
📄 config.tar.gz            (if enabled)
📋 _manifest.json           (REQUIRED for restore)
✅ Backup Complete message
```

## Scheduled Backups

Set the backup interval in settings. The system runs automatically at the specified interval.

## Important: Save the Manifest

The `_manifest.json` file is **required** to restore. It contains:

- File checksums for verification
- Telegram `file_id` values for downloading
- Chunk ordering information

Save a copy somewhere safe — if you lose the manifest, you cannot restore from the backup.

## Chat ID Formats

| Chat Type | Format | Example |
|-----------|--------|---------|
| Private | Positive integer | `123456789` |
| Group | Negative integer | `-123456789` |
| Supergroup | Starts with -100 | `-1001234567890` |
| Channel | Starts with -100 | `-1001234567890` |
| Username | @name | `@mybackupchannel` |

## Troubleshooting

**Bot can't send messages:**
- Make sure the bot is added to the chat/channel
- Check the chat ID is correct
- Verify the bot has permission to send messages

**Large file fails:**
- Files over 20MB are automatically chunked
- If still failing, try reducing chunk size in settings

**Rate limit errors (429):**
- System automatically waits and retries
- Normal for large backups with many files

## Related Documentation

- [How to Restore from Telegram](restore-from-telegram.md)
- [Backup Manifest Reference](../reference/backup-manifest-format.md)
