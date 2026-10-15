# Backup Manifest Reference

Technical reference for the Telegram backup manifest format, chunking, and Telegram API methods.

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

## Field Reference

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

## Chunking

Files over 20MB are split into chunks for Telegram's `getFile` API limit.

| Setting | Value |
|---------|-------|
| Default chunk size | 20MB |
| Maximum chunk size | 20MB (Telegram API limit) |
| Minimum chunk size | 5MB (configurable) |

Files ≤ chunk size are uploaded as-is. Larger files split into `filename.partNNN` chunks.

## Database Backup

Uses PostgreSQL custom format with compression:

```bash
# Backup
pg_dump --format=custom --no-owner --no-acl --compress=6

# Restore
pg_restore --no-owner --no-acl --single-transaction
```

## Config Backup

Archives specified paths (default: `.env`, `.env.local`) into `config.tar.gz`.

## Telegram API Methods

| Operation | API Method |
|-----------|------------|
| Upload file | `sendDocument` |
| Get file info | `getFile` |
| Download file | `https://api.telegram.org/file/bot<token>/<file_path>` |

## Security

1. **Bot token** is encrypted in database using AES-256-GCM
2. **Backup files** are NOT encrypted on Telegram (use private chat)
3. **file_id** is tied to your bot — others cannot download your files
4. **Manifest** contains no sensitive data, just file metadata

## Related Documentation

- [How to Back Up to Telegram](../how-to/backup-to-telegram.md)
- [How to Restore from Telegram](../how-to/restore-from-telegram.md)
