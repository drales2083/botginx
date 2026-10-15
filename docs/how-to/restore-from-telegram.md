# How to Restore from Telegram

This guide shows you how to restore botginx onto a fresh server from a Telegram backup. You need the `_manifest.json` produced by [the backup](backup-to-telegram.md).

## Prerequisites

1. Fresh server with botginx installed and running
2. PostgreSQL database created (empty)
3. Same Telegram bot token configured in botginx
4. The `_manifest.json` file from your backup

## Step-by-Step Restore

### 1. Get the Manifest

Go to your Telegram backup chat and download the `_manifest.json` file. You can:
- Click on it in Telegram Desktop → Save As
- Forward it to yourself and download
- Copy the raw JSON content

### 2. Configure Bot Token

Before restoring, ensure the **same bot token** is configured:

1. Go to **Admin → Backup** (`/admin/backup`)
2. Enter the bot token that was used for the backup
3. Click **Save Settings**

> **Important**: The `file_id` in the manifest is tied to the specific bot. You must use the same bot to download the files.

### 3. Start Restore

1. Go to **Admin → Backup → Restore** (`/admin/backup/restore`)
2. Paste the `_manifest.json` content OR upload the file
3. Click **Validate Manifest**
4. Review backup details (date, size, files)
5. Click **Start Restore**

### 4. Restore Process

The system will automatically:

1. **Validate** - Check manifest has all required `file_id`s
2. **Download** - Fetch each chunk from Telegram via `getFile` API
3. **Verify chunks** - SHA256 check each downloaded chunk
4. **Merge** - Combine chunks in correct order
5. **Verify merged** - SHA256 check final files
6. **Restore database** - Run `pg_restore` 
7. **Restore config** - Extract `config.tar.gz`

### 5. After Restore

- Restart botginx service: `systemctl restart botginx`
- Verify data in the admin panel
- Re-configure any server-specific settings (if needed)

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

## Checklist


- [ ] Fresh server with botginx running
- [ ] PostgreSQL database exists (empty)
- [ ] Same bot token configured
- [ ] Have `_manifest.json` file
- [ ] Restore completes without errors
- [ ] Restart botginx service
- [ ] Verify data in admin panel
