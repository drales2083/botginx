# Telegram Backup & Restore System

Complete guide for backing up and restoring botginx using Telegram as storage.

## Overview

The backup system uploads database dumps and config files to Telegram, storing them as documents in any chat (private, group, or channel). Each backup includes a `_manifest.json` file containing SHA256 checksums and Telegram `file_id`s needed for restore.

### Key Features

- **20MB chunk limit** - Files split automatically for Telegram's `getFile` API
- **SHA256 verification** - Every chunk and merged file verified
- **Any chat type** - Works with private chats, groups, or channels
- **Fresh server restore** - Works on brand new server with empty database
- **Scheduled backups** - Automatic backups at configurable intervals

---

## Backup

### Prerequisites

1. Create a Telegram bot via [@BotFather](https://t.me/BotFather)
2. Get the bot token (looks like `123456789:ABCdefGHI...`)
3. Start a chat with your bot OR add bot to a group/channel
4. Get the chat ID (use [@userinfobot](https://t.me/userinfobot) or similar)

### Configure Backup

1. Go to **Admin → Backup** (`/admin/backup`)
2. Enter your bot token
3. Enter chat ID(s) - comma-separate multiple
4. Set chunk size (default 20MB, max 20MB for Telegram API)
5. Enable "Include Database" and/or "Include Config"
6. Set backup interval (hours) and retention count
7. Click **Save Settings**

### Run Manual Backup

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

### What Gets Uploaded

```
📦 db.dump.part001          (if chunked)
📦 db.dump.part002
📦 db.dump.part003
📄 config.tar.gz            (if enabled)
📋 _manifest.json           (REQUIRED for restore)
✅ Backup Complete message
```

---

## Restore

### Prerequisites

1. Fresh server with botginx installed and running
2. PostgreSQL database created (empty)
3. Same Telegram bot token configured in botginx
4. The `_manifest.json` file from your backup

### Step-by-Step Restore

#### 1. Get the Manifest

Go to your Telegram backup chat and download the `_manifest.json` file. You can:
- Click on it in Telegram Desktop → Save As
- Forward it to yourself and download
- Copy the raw JSON content

#### 2. Configure Bot Token

Before restoring, ensure the **same bot token** is configured:

1. Go to **Admin → Backup** (`/admin/backup`)
2. Enter the bot token that was used for the backup
3. Click **Save Settings**

> **Important**: The `file_id` in the manifest is tied to the specific bot. You must use the same bot to download the files.

#### 3. Start Restore

1. Go to **Admin → Backup → Restore** (`/admin/backup/restore`)
2. Paste the `_manifest.json` content OR upload the file
3. Click **Validate Manifest**
4. Review backup details (date, size, files)
5. Click **Start Restore**

#### 4. Restore Process

The system will automatically:

1. **Validate** - Check manifest has all required `file_id`s
2. **Download** - Fetch each chunk from Telegram via `getFile` API
3. **Verify chunks** - SHA256 check each downloaded chunk
4. **Merge** - Combine chunks in correct order
5. **Verify merged** - SHA256 check final files
6. **Restore database** - Run `pg_restore` 
7. **Restore config** - Extract `config.tar.gz`

#### 5. After Restore

- Restart botginx service: `systemctl restart botginx`
- Verify data in the admin panel
- Re-configure any server-specific settings (if needed)

---

## Manifest Format

The `_manifest.json` file contains everything needed for restore:

```json
{
  "version": 1,
  "backup_id": "1695847293847562000",
  "timestamp": "2024-09-28T10:30:00Z",
  "host": "guardbot.sbs",
  "files": [
    {
      "name": "db.dump",
      "size": 52428800,
      "sha256": "a1b2c3d4e5f6...",
      "chunked": true,
      "chunks": [
        {
          "part": 1,
          "name": "db.dump.part001",
          "size": 20971520,
          "sha256": "1a2b3c4d5e6f...",
          "telegram": {
            "file_id": "BQACAgIAAxkBAAI...",
            "file_unique_id": "AgADAgAT..."
          }
        },
        {
          "part": 2,
          "name": "db.dump.part002",
          "size": 20971520,
          "sha256": "2b3c4d5e6f7g...",
          "telegram": {
            "file_id": "BQACAgIAAxkBAAJ...",
            "file_unique_id": "AgADAgAU..."
          }
        },
        {
          "part": 3,
          "name": "db.dump.part003",
          "size": 10485760,
          "sha256": "3c4d5e6f7g8h...",
          "telegram": {
            "file_id": "BQACAgIAAxkBAAK...",
            "file_unique_id": "AgADAgAV..."
          }
        }
      ]
    },
    {
      "name": "config.tar.gz",
      "size": 4096,
      "sha256": "9z8y7x6w5v4u...",
      "chunked": false,
      "telegram": {
        "file_id": "BQACAgIAAxkBAAL...",
        "file_unique_id": "AgADAgAW..."
      }
    }
  ]
}
```

### Key Fields

| Field | Description |
|-------|-------------|
| `version` | Manifest format version (always 1) |
| `backup_id` | Unique ID for this backup |
| `timestamp` | When backup was created (ISO 8601) |
| `host` | Server hostname at backup time |
| `files[].name` | Original filename |
| `files[].size` | Total size in bytes |
| `files[].sha256` | SHA256 of complete file |
| `files[].chunked` | Whether file was split |
| `files[].telegram.file_id` | Telegram file ID for download |
| `chunks[].part` | Chunk order (1, 2, 3...) |
| `chunks[].sha256` | SHA256 of this chunk |

---

## Troubleshooting

### "Missing file_id" Error

**Cause**: Backup was created before restore support was added.

**Solution**: Create a new backup with the current version.

### "File not found" (404) Error

**Cause**: Files were deleted from Telegram or bot was removed.

**Solution**: 
- Check if files still exist in the backup chat
- Ensure bot is still a member of the chat
- Files may have been auto-deleted (Telegram can delete large files after time)

### "Hash mismatch" Error

**Cause**: File was corrupted during download.

**Solution**:
- Check internet connection
- Try again (system has retry logic)
- If persistent, the file on Telegram may be corrupted

### "pg_restore failed" Error

**Cause**: Database restore error.

**Possible solutions**:
- Check PostgreSQL is running: `systemctl status postgresql`
- Check DATABASE_URL is set correctly
- Check database exists and user has permissions
- Review error message for specific issue

### Rate Limit (429) Error

**Cause**: Too many Telegram API requests.

**Solution**: System automatically waits and retries. For large backups, this is normal.

---

## Technical Details

### Chunk Size

- **Default**: 20MB
- **Maximum**: 20MB (Telegram `getFile` API limit)
- **Configurable**: Yes, in settings (5-20MB)

Files ≤ chunk size are uploaded as-is. Larger files split into `filename.partNNN` chunks.

### Database Backup

Uses PostgreSQL custom format with compression:

```bash
pg_dump --format=custom --no-owner --no-acl --compress=6
```

Restore uses:

```bash
pg_restore --no-owner --no-acl --single-transaction
```

### Config Backup

Archives specified paths (default: `.env`, `.env.local`) into `config.tar.gz`.

### Telegram API Methods

| Operation | API Method |
|-----------|------------|
| Upload file | `sendDocument` |
| Get file info | `getFile` |
| Download file | `https://api.telegram.org/file/bot<token>/<file_path>` |

### Chat ID Formats

| Chat Type | Format | Example |
|-----------|--------|---------|
| Private | Positive integer | `123456789` |
| Group | Negative integer | `-123456789` |
| Supergroup | Starts with -100 | `-1001234567890` |
| Channel | Starts with -100 | `-1001234567890` |
| Username | @name | `@mybackupchannel` |

---

## Security Notes

1. **Bot token** is encrypted in database using AES-256-GCM
2. **Backup files** are NOT encrypted on Telegram (consider private chat)
3. **file_id** is tied to your bot - others cannot download your files
4. **Manifest** contains no sensitive data, just file metadata

---

## Quick Reference

### Backup Checklist

- [ ] Bot token configured
- [ ] Chat ID configured
- [ ] Bot can send messages to chat
- [ ] "Include Database" enabled
- [ ] Backup runs successfully
- [ ] Save `_manifest.json` somewhere safe

### Restore Checklist

- [ ] Fresh server with botginx running
- [ ] PostgreSQL database exists (empty)
- [ ] Same bot token configured
- [ ] Have `_manifest.json` file
- [ ] Restore completes without errors
- [ ] Restart botginx service
- [ ] Verify data in admin panel
