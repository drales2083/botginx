# Telegram Backup Manifest Format

Technical reference for the Telegram backup system. Backups upload database dumps and config files as chat documents, plus a `_manifest.json` with SHA256 checksums and Telegram `file_id`s.

## Features

- **20MB chunk limit** - Files split automatically for Telegram's `getFile` API
- **SHA256 verification** - Every chunk and merged file verified
- **Any chat type** - Works with private chats, groups, or channels
- **Fresh server restore** - Works on brand new server with empty database
- **Scheduled backups** - Automatic backups at configurable intervals

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
