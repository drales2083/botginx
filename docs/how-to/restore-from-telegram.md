# How to Restore from Telegram

This guide shows you how to restore your botginx database and configuration from a Telegram backup.

## Prerequisites

1. Fresh server with botginx installed and running
2. PostgreSQL database created (empty)
3. The **same Telegram bot token** used for the backup
4. The `_manifest.json` file from your backup

## Step 1: Get the Manifest

Go to your Telegram backup chat and download the `_manifest.json` file:

- Click on it in Telegram Desktop → Save As
- Or forward it to yourself and download
- Or copy the raw JSON content

## Step 2: Configure Bot Token

Before restoring, ensure the **same bot token** is configured:

1. Go to **Admin → Backup** (`/admin/backup`)
2. Enter the bot token that was used for the backup
3. Click **Save Settings**

**Important:** The `file_id` in the manifest is tied to the specific bot. You must use the same bot to download the files.

## Step 3: Start Restore

1. Go to **Admin → Backup → Restore** (`/admin/backup/restore`)
2. Paste the `_manifest.json` content OR upload the file
3. Click **Validate Manifest**
4. Review backup details (date, size, files)
5. Click **Start Restore**

## Restore Process

The system automatically:

1. **Validates** — checks manifest has all required `file_id`s
2. **Downloads** — fetches each chunk from Telegram via `getFile` API
3. **Verifies chunks** — SHA256 check each downloaded chunk
4. **Merges** — combines chunks in correct order
5. **Verifies merged** — SHA256 check final files
6. **Restores database** — runs `pg_restore`
7. **Restores config** — extracts `config.tar.gz`

## Step 4: After Restore

1. Restart botginx service: `systemctl restart botginx`
2. Verify data in the admin panel
3. Re-configure any server-specific settings if needed

## Troubleshooting

**"Missing file_id" error:**
- Backup was created before restore support was added
- Solution: create a new backup with the current version

**"File not found" (404) error:**
- Files were deleted from Telegram or bot was removed
- Check if files still exist in the backup chat
- Ensure bot is still a member of the chat

**"Hash mismatch" error:**
- File was corrupted during download
- Try again (system has retry logic)
- If persistent, the file on Telegram may be corrupted

**"pg_restore failed" error:**
- Check PostgreSQL is running: `systemctl status postgresql`
- Check DATABASE_URL is set correctly
- Check database exists and user has permissions

**Rate limit (429) error:**
- System automatically waits and retries
- Normal for large restores — be patient

## Restore Checklist

- [ ] Fresh server with botginx running
- [ ] PostgreSQL database exists (empty)
- [ ] Same bot token configured
- [ ] Have `_manifest.json` file
- [ ] Restore completes without errors
- [ ] Restart botginx service
- [ ] Verify data in admin panel

## Related Documentation

- [How to Back Up to Telegram](backup-to-telegram.md)
- [Backup Manifest Reference](../reference/backup-manifest-format.md)
