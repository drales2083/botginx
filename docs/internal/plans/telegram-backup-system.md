# Telegram Backup System for Botginx

## Overview

Automated daily backup of botginx database and configuration files to Telegram, with chunking support for files >50MB and optional GPG encryption.

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        BACKUP MODULE                                │
│  modules/backup/ orchestrates the entire backup flow                │
├─────────────────────────────────────────────────────────────────────┤
│  Step 1: pg_dump (database) → db.dump                               │
│  Step 2: tar.gz (config files) → config.tar.gz                      │
│  Step 3: Optional GPG encryption → *.gpg                            │
│  Step 4: Write _manifest.json                                       │
│  Step 5: Chunk files >45MB into parts                               │
│  Step 6: Upload to Telegram channel(s)                              │
│  Step 7: Cleanup old backups (retention policy)                     │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Database Schema

```sql
-- modules/backup/migrations/001_create_tables.sql

CREATE TABLE backup_history (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    status TEXT NOT NULL DEFAULT 'pending',  -- pending, running, completed, failed
    started_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    
    -- Stats
    db_size_bytes BIGINT,
    config_size_bytes BIGINT,
    total_size_bytes BIGINT,
    chunk_count INT DEFAULT 0,
    
    -- Telegram upload result
    telegram_meta JSONB,  -- { channels: { "chatId": { messageIds: [], manifestId: 123 } } }
    
    -- Errors
    error_message TEXT,
    
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_backup_history_status ON backup_history(status);
CREATE INDEX idx_backup_history_started ON backup_history(started_at DESC);

-- Backup settings (admin-only)
CREATE TABLE backup_settings (
    id TEXT PRIMARY KEY DEFAULT 'default',
    enabled BOOLEAN DEFAULT FALSE,
    
    -- Schedule
    interval_hours INT DEFAULT 24,
    retention_count INT DEFAULT 7,  -- Keep last N backups
    
    -- Telegram
    telegram_bot_token_encrypted BYTEA,
    telegram_chat_ids TEXT[],  -- Array of chat IDs
    telegram_chunk_size_mb INT DEFAULT 45,
    telegram_send_notification BOOLEAN DEFAULT TRUE,
    
    -- Encryption
    gpg_enabled BOOLEAN DEFAULT FALSE,
    gpg_passphrase_encrypted BYTEA,
    
    -- What to backup
    include_database BOOLEAN DEFAULT TRUE,
    include_config BOOLEAN DEFAULT TRUE,
    config_paths TEXT[] DEFAULT ARRAY['.env', '.env.local'],
    
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Insert default settings
INSERT INTO backup_settings (id) VALUES ('default') ON CONFLICT DO NOTHING;
```

---

## Go Module Structure

```
modules/backup/
├── module.go                 # Module init, routes, cron jobs
├── models/
│   ├── history.go           # BackupHistory model
│   └── settings.go          # BackupSettings model
├── services/
│   ├── backup_service.go    # Main backup orchestration
│   ├── database.go          # pg_dump wrapper
│   ├── archiver.go          # tar.gz creation
│   ├── encryption.go        # GPG encryption
│   └── chunker.go           # File chunking for Telegram
├── telegram/
│   ├── client.go            # Telegram Bot API client
│   ├── upload.go            # Upload files with chunking
│   └── prune.go             # Delete old messages
├── handlers/
│   └── handler.go           # Admin UI handlers
├── templates/
│   ├── settings.html        # Backup settings page
│   └── history.html         # Backup history page
└── migrations/
    └── 001_create_tables.sql
```

---

## Core Implementation

### 1. Backup Service (orchestrates everything)

```go
// modules/backup/services/backup_service.go

package services

import (
    "context"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "time"
    
    "github.com/botginx/botginx/modules/backup/models"
    "github.com/botginx/botginx/modules/backup/telegram"
    "github.com/botginx/botginx/pkg/crypto"
)

type BackupService struct {
    db       *sqlx.DB
    settings *models.BackupSettings
    crypto   *crypto.Encryptor
    tempDir  string
}

func NewBackupService(db *sqlx.DB, crypto *crypto.Encryptor) *BackupService {
    return &BackupService{
        db:      db,
        crypto:  crypto,
        tempDir: os.TempDir(),
    }
}

// RunBackup executes a full backup
func (s *BackupService) RunBackup(ctx context.Context) (*models.BackupHistory, error) {
    // 1. Load settings
    settings, err := s.GetSettings()
    if err != nil {
        return nil, fmt.Errorf("load settings: %w", err)
    }
    if !settings.Enabled {
        return nil, fmt.Errorf("backup is disabled")
    }
    
    // 2. Create history record
    history := &models.BackupHistory{
        ID:        generateID(),
        Status:    "running",
        StartedAt: time.Now(),
    }
    if err := s.createHistory(history); err != nil {
        return nil, err
    }
    
    // 3. Create temp directory for this backup
    backupDir := filepath.Join(s.tempDir, "botginx-backup-"+history.ID)
    if err := os.MkdirAll(backupDir, 0700); err != nil {
        return s.failBackup(history, err)
    }
    defer os.RemoveAll(backupDir)
    
    var files []BackupFile
    
    // 4. Database dump
    if settings.IncludeDatabase {
        dbFile, err := s.dumpDatabase(ctx, backupDir)
        if err != nil {
            return s.failBackup(history, fmt.Errorf("database dump: %w", err))
        }
        history.DBSizeBytes = dbFile.Size
        files = append(files, dbFile)
    }
    
    // 5. Config archive
    if settings.IncludeConfig && len(settings.ConfigPaths) > 0 {
        configFile, err := s.archiveConfig(ctx, backupDir, settings.ConfigPaths)
        if err != nil {
            return s.failBackup(history, fmt.Errorf("config archive: %w", err))
        }
        history.ConfigSizeBytes = configFile.Size
        files = append(files, configFile)
    }
    
    // 6. GPG encryption (optional)
    if settings.GPGEnabled {
        passphrase, err := s.crypto.Decrypt(settings.GPGPassphraseEncrypted)
        if err != nil {
            return s.failBackup(history, fmt.Errorf("decrypt passphrase: %w", err))
        }
        files, err = s.encryptFiles(ctx, files, string(passphrase))
        if err != nil {
            return s.failBackup(history, fmt.Errorf("encryption: %w", err))
        }
    }
    
    // 7. Calculate total size and hashes
    var totalSize int64
    for i := range files {
        files[i].SHA256, _ = hashFile(files[i].Path)
        totalSize += files[i].Size
    }
    history.TotalSizeBytes = totalSize
    
    // 8. Upload to Telegram
    if len(settings.TelegramChatIDs) > 0 {
        botToken, err := s.crypto.Decrypt(settings.TelegramBotTokenEncrypted)
        if err != nil {
            return s.failBackup(history, fmt.Errorf("decrypt bot token: %w", err))
        }
        
        client := telegram.NewClient(string(botToken))
        result, err := client.UploadBackup(ctx, telegram.UploadOptions{
            Files:            files,
            ChatIDs:          settings.TelegramChatIDs,
            ChunkSizeMB:      settings.TelegramChunkSizeMB,
            BackupID:         history.ID,
            Timestamp:        history.StartedAt,
            SendNotification: settings.TelegramSendNotification,
        })
        if err != nil {
            return s.failBackup(history, fmt.Errorf("telegram upload: %w", err))
        }
        
        history.ChunkCount = result.ChunkCount
        history.TelegramMeta = result.ToJSON()
    }
    
    // 9. Mark completed
    history.Status = "completed"
    history.CompletedAt = timePtr(time.Now())
    if err := s.updateHistory(history); err != nil {
        return nil, err
    }
    
    // 10. Enforce retention (async)
    go s.EnforceRetention(context.Background())
    
    return history, nil
}

// BackupFile represents a file to be backed up
type BackupFile struct {
    Name   string
    Path   string
    Size   int64
    SHA256 string
}
```

### 2. Database Dump

```go
// modules/backup/services/database.go

package services

import (
    "context"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
)

// dumpDatabase runs pg_dump and returns the file info
func (s *BackupService) dumpDatabase(ctx context.Context, backupDir string) (BackupFile, error) {
    outputPath := filepath.Join(backupDir, "db.dump")
    
    // Get DATABASE_URL from environment
    dbURL := os.Getenv("DATABASE_URL")
    if dbURL == "" {
        return BackupFile{}, fmt.Errorf("DATABASE_URL not set")
    }
    
    // Run pg_dump
    cmd := exec.CommandContext(ctx, "pg_dump",
        "--dbname="+dbURL,
        "--format=custom",
        "--no-owner",
        "--no-acl",
        "--compress=6",
        // Exclude large log tables if any
        // "--exclude-table=audit_logs",
        "--file="+outputPath,
    )
    
    output, err := cmd.CombinedOutput()
    if err != nil {
        return BackupFile{}, fmt.Errorf("pg_dump failed: %w\n%s", err, string(output))
    }
    
    // Get file info
    info, err := os.Stat(outputPath)
    if err != nil {
        return BackupFile{}, err
    }
    
    return BackupFile{
        Name: "db.dump",
        Path: outputPath,
        Size: info.Size(),
    }, nil
}
```

### 3. Config Archiver

```go
// modules/backup/services/archiver.go

package services

import (
    "archive/tar"
    "compress/gzip"
    "context"
    "io"
    "os"
    "path/filepath"
)

// archiveConfig creates a tar.gz of config files
func (s *BackupService) archiveConfig(ctx context.Context, backupDir string, paths []string) (BackupFile, error) {
    outputPath := filepath.Join(backupDir, "config.tar.gz")
    
    file, err := os.Create(outputPath)
    if err != nil {
        return BackupFile{}, err
    }
    defer file.Close()
    
    gw := gzip.NewWriter(file)
    defer gw.Close()
    
    tw := tar.NewWriter(gw)
    defer tw.Close()
    
    // Add each config file
    for _, p := range paths {
        // Resolve path relative to app directory
        fullPath := filepath.Join(os.Getenv("APP_DIR"), p)
        if _, err := os.Stat(fullPath); os.IsNotExist(err) {
            continue // Skip missing files
        }
        
        if err := addFileToTar(tw, fullPath, p); err != nil {
            return BackupFile{}, err
        }
    }
    
    // Close writers to flush
    tw.Close()
    gw.Close()
    file.Close()
    
    // Get file info
    info, err := os.Stat(outputPath)
    if err != nil {
        return BackupFile{}, err
    }
    
    return BackupFile{
        Name: "config.tar.gz",
        Path: outputPath,
        Size: info.Size(),
    }, nil
}

func addFileToTar(tw *tar.Writer, filePath, name string) error {
    file, err := os.Open(filePath)
    if err != nil {
        return err
    }
    defer file.Close()
    
    info, err := file.Stat()
    if err != nil {
        return err
    }
    
    header, err := tar.FileInfoHeader(info, "")
    if err != nil {
        return err
    }
    header.Name = name
    
    if err := tw.WriteHeader(header); err != nil {
        return err
    }
    
    _, err = io.Copy(tw, file)
    return err
}
```

### 4. File Chunker (for Telegram's 50MB limit)

```go
// modules/backup/services/chunker.go

package services

import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "io"
    "os"
    "path/filepath"
)

const (
    DefaultChunkSizeMB = 45
    MaxChunkSizeMB     = 50  // Telegram limit
)

type ChunkInfo struct {
    Part   int    `json:"part"`
    Name   string `json:"name"`
    Path   string `json:"-"`
    Size   int64  `json:"size"`
    SHA256 string `json:"sha256"`
}

// ChunkFile splits a file into parts if it exceeds chunkSizeMB
func ChunkFile(filePath string, chunkSizeMB int) ([]ChunkInfo, error) {
    if chunkSizeMB <= 0 || chunkSizeMB > MaxChunkSizeMB {
        chunkSizeMB = DefaultChunkSizeMB
    }
    chunkSize := int64(chunkSizeMB) * 1024 * 1024
    
    info, err := os.Stat(filePath)
    if err != nil {
        return nil, err
    }
    
    // No chunking needed
    if info.Size() <= chunkSize {
        hash, _ := hashFile(filePath)
        return []ChunkInfo{{
            Part:   0, // 0 means not chunked
            Name:   filepath.Base(filePath),
            Path:   filePath,
            Size:   info.Size(),
            SHA256: hash,
        }}, nil
    }
    
    // Split into chunks
    file, err := os.Open(filePath)
    if err != nil {
        return nil, err
    }
    defer file.Close()
    
    dir := filepath.Dir(filePath)
    baseName := filepath.Base(filePath)
    
    var chunks []ChunkInfo
    buf := make([]byte, chunkSize)
    part := 1
    
    for {
        n, err := file.Read(buf)
        if n == 0 {
            break
        }
        
        chunkName := fmt.Sprintf("%s.part%03d", baseName, part)
        chunkPath := filepath.Join(dir, chunkName)
        
        if err := os.WriteFile(chunkPath, buf[:n], 0600); err != nil {
            return nil, err
        }
        
        hash, _ := hashFile(chunkPath)
        chunks = append(chunks, ChunkInfo{
            Part:   part,
            Name:   chunkName,
            Path:   chunkPath,
            Size:   int64(n),
            SHA256: hash,
        })
        
        part++
        
        if err == io.EOF {
            break
        }
        if err != nil {
            return nil, err
        }
    }
    
    return chunks, nil
}

func hashFile(path string) (string, error) {
    file, err := os.Open(path)
    if err != nil {
        return "", err
    }
    defer file.Close()
    
    h := sha256.New()
    if _, err := io.Copy(h, file); err != nil {
        return "", err
    }
    
    return hex.EncodeToString(h.Sum(nil)), nil
}
```

### 5. Telegram Client

```go
// modules/backup/telegram/client.go

package telegram

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "mime/multipart"
    "net/http"
    "os"
    "path/filepath"
    "time"
)

const BaseURL = "https://api.telegram.org/bot"

type Client struct {
    token  string
    http   *http.Client
}

func NewClient(token string) *Client {
    return &Client{
        token: token,
        http: &http.Client{
            Timeout: 5 * time.Minute, // Large file uploads
        },
    }
}

// SendDocument uploads a file to a chat
func (c *Client) SendDocument(ctx context.Context, chatID string, filePath string, caption string) (*Message, error) {
    file, err := os.Open(filePath)
    if err != nil {
        return nil, err
    }
    defer file.Close()
    
    body := &bytes.Buffer{}
    writer := multipart.NewWriter(body)
    
    // Add chat_id
    writer.WriteField("chat_id", chatID)
    
    // Add caption if provided
    if caption != "" {
        writer.WriteField("caption", caption)
    }
    
    // Add document
    part, err := writer.CreateFormFile("document", filepath.Base(filePath))
    if err != nil {
        return nil, err
    }
    if _, err := io.Copy(part, file); err != nil {
        return nil, err
    }
    
    writer.Close()
    
    req, err := http.NewRequestWithContext(ctx, "POST", 
        BaseURL+c.token+"/sendDocument", body)
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", writer.FormDataContentType())
    
    resp, err := c.http.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var result struct {
        OK          bool    `json:"ok"`
        Result      Message `json:"result"`
        Description string  `json:"description"`
        ErrorCode   int     `json:"error_code"`
        Parameters  struct {
            RetryAfter int `json:"retry_after"`
        } `json:"parameters"`
    }
    
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }
    
    if !result.OK {
        // Handle rate limiting
        if result.ErrorCode == 429 && result.Parameters.RetryAfter > 0 {
            time.Sleep(time.Duration(result.Parameters.RetryAfter) * time.Second)
            return c.SendDocument(ctx, chatID, filePath, caption) // Retry
        }
        return nil, fmt.Errorf("telegram error %d: %s", result.ErrorCode, result.Description)
    }
    
    return &result.Result, nil
}

// SendMessage sends a text message
func (c *Client) SendMessage(ctx context.Context, chatID string, text string, parseMode string) (*Message, error) {
    payload := map[string]interface{}{
        "chat_id":                  chatID,
        "text":                     text,
        "parse_mode":               parseMode,
        "disable_web_page_preview": true,
    }
    
    body, _ := json.Marshal(payload)
    
    req, err := http.NewRequestWithContext(ctx, "POST",
        BaseURL+c.token+"/sendMessage", bytes.NewReader(body))
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", "application/json")
    
    resp, err := c.http.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var result struct {
        OK     bool    `json:"ok"`
        Result Message `json:"result"`
    }
    
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }
    
    return &result.Result, nil
}

// DeleteMessages deletes multiple messages (max 100)
func (c *Client) DeleteMessages(ctx context.Context, chatID string, messageIDs []int) error {
    if len(messageIDs) == 0 {
        return nil
    }
    if len(messageIDs) > 100 {
        // Batch into groups of 100
        for i := 0; i < len(messageIDs); i += 100 {
            end := i + 100
            if end > len(messageIDs) {
                end = len(messageIDs)
            }
            if err := c.DeleteMessages(ctx, chatID, messageIDs[i:end]); err != nil {
                return err
            }
        }
        return nil
    }
    
    payload := map[string]interface{}{
        "chat_id":     chatID,
        "message_ids": messageIDs,
    }
    
    body, _ := json.Marshal(payload)
    
    req, err := http.NewRequestWithContext(ctx, "POST",
        BaseURL+c.token+"/deleteMessages", bytes.NewReader(body))
    if err != nil {
        return err
    }
    req.Header.Set("Content-Type", "application/json")
    
    resp, err := c.http.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    return nil
}

type Message struct {
    MessageID int `json:"message_id"`
}
```

### 6. Upload Orchestration

```go
// modules/backup/telegram/upload.go

package telegram

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "time"
    
    "github.com/botginx/botginx/modules/backup/services"
)

type UploadOptions struct {
    Files            []services.BackupFile
    ChatIDs          []string
    ChunkSizeMB      int
    BackupID         string
    Timestamp        time.Time
    SendNotification bool
}

type UploadResult struct {
    TotalBytes int64
    ChunkCount int
    Channels   map[string]ChannelResult
}

type ChannelResult struct {
    MessageIDs []int `json:"messageIds"`
    ManifestID int   `json:"manifestId"`
    Error      string `json:"error,omitempty"`
}

func (r *UploadResult) ToJSON() json.RawMessage {
    data, _ := json.Marshal(map[string]interface{}{
        "channels": r.Channels,
    })
    return data
}

// UploadBackup uploads all backup files to Telegram
func (c *Client) UploadBackup(ctx context.Context, opts UploadOptions) (*UploadResult, error) {
    result := &UploadResult{
        Channels: make(map[string]ChannelResult),
    }
    
    // Build manifest
    manifest := Manifest{
        Version:   1,
        BackupID:  opts.BackupID,
        Timestamp: opts.Timestamp.Format(time.RFC3339),
        Host:      getHostname(),
        Files:     make([]FileEntry, 0),
    }
    
    // Process each file - chunk if needed
    var uploadQueue []uploadItem
    
    for _, file := range opts.Files {
        chunks, err := services.ChunkFile(file.Path, opts.ChunkSizeMB)
        if err != nil {
            return nil, fmt.Errorf("chunk %s: %w", file.Name, err)
        }
        
        entry := FileEntry{
            Name:    file.Name,
            Size:    file.Size,
            SHA256:  file.SHA256,
            Chunked: len(chunks) > 1,
        }
        
        if len(chunks) > 1 {
            for _, chunk := range chunks {
                entry.Chunks = append(entry.Chunks, ChunkEntry{
                    Part:   chunk.Part,
                    Name:   chunk.Name,
                    Size:   chunk.Size,
                    SHA256: chunk.SHA256,
                })
                uploadQueue = append(uploadQueue, uploadItem{
                    path:    chunk.Path,
                    caption: fmt.Sprintf("📦 %s (%s)", file.Name, chunk.Name),
                })
                result.ChunkCount++
            }
        } else {
            uploadQueue = append(uploadQueue, uploadItem{
                path:    file.Path,
                caption: fmt.Sprintf("📄 %s", file.Name),
            })
        }
        
        result.TotalBytes += file.Size
        manifest.Files = append(manifest.Files, entry)
    }
    
    // Write manifest to temp file
    manifestPath := filepath.Join(filepath.Dir(opts.Files[0].Path), "_manifest.json")
    manifestData, _ := json.MarshalIndent(manifest, "", "  ")
    if err := os.WriteFile(manifestPath, manifestData, 0600); err != nil {
        return nil, err
    }
    uploadQueue = append(uploadQueue, uploadItem{
        path:    manifestPath,
        caption: "📋 _manifest.json",
    })
    
    // Upload to each channel
    for _, chatID := range opts.ChatIDs {
        chanResult := ChannelResult{}
        
        for _, item := range uploadQueue {
            msg, err := c.SendDocument(ctx, chatID, item.path, item.caption)
            if err != nil {
                chanResult.Error = err.Error()
                break
            }
            chanResult.MessageIDs = append(chanResult.MessageIDs, msg.MessageID)
            
            // Track manifest message ID
            if filepath.Base(item.path) == "_manifest.json" {
                chanResult.ManifestID = msg.MessageID
            }
        }
        
        // Send notification
        if opts.SendNotification && chanResult.Error == "" {
            notification := fmt.Sprintf(
                "✅ <b>Backup Complete</b>\n\n"+
                "📅 %s\n"+
                "📁 %d file(s) (%d chunks)\n"+
                "💾 %s",
                opts.Timestamp.Format("2006-01-02 15:04:05"),
                len(opts.Files),
                result.ChunkCount,
                formatBytes(result.TotalBytes),
            )
            c.SendMessage(ctx, chatID, notification, "HTML")
        }
        
        result.Channels[chatID] = chanResult
    }
    
    return result, nil
}

type uploadItem struct {
    path    string
    caption string
}

type Manifest struct {
    Version   int         `json:"version"`
    BackupID  string      `json:"backup_id"`
    Timestamp string      `json:"timestamp"`
    Host      string      `json:"host"`
    Files     []FileEntry `json:"files"`
}

type FileEntry struct {
    Name    string       `json:"name"`
    Size    int64        `json:"size"`
    SHA256  string       `json:"sha256"`
    Chunked bool         `json:"chunked"`
    Chunks  []ChunkEntry `json:"chunks,omitempty"`
}

type ChunkEntry struct {
    Part   int    `json:"part"`
    Name   string `json:"name"`
    Size   int64  `json:"size"`
    SHA256 string `json:"sha256"`
}

func getHostname() string {
    h, _ := os.Hostname()
    return h
}

func formatBytes(b int64) string {
    const unit = 1024
    if b < unit {
        return fmt.Sprintf("%d B", b)
    }
    div, exp := int64(unit), 0
    for n := b / unit; n >= unit; n /= unit {
        div *= unit
        exp++
    }
    return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
```

---

## Admin UI

### Settings Page

```html
<!-- modules/backup/templates/settings.html -->
{{define "content"}}
<div class="card">
    <div class="card-header">
        <h3 class="card-title">Backup Settings</h3>
    </div>
    <div class="card-body">
        <form id="backup-settings-form">
            <!-- Enable/Disable -->
            <div class="mb-3">
                <div class="form-check form-switch">
                    <input class="form-check-input" type="checkbox" id="enabled" {{if .Settings.Enabled}}checked{{end}}>
                    <label class="form-check-label" for="enabled">Enable Automatic Backups</label>
                </div>
            </div>

            <hr>
            <h5>Schedule</h5>
            
            <div class="row mb-3">
                <div class="col-md-6">
                    <label class="form-label">Backup Interval (hours)</label>
                    <input type="number" class="form-control" id="intervalHours" 
                           value="{{.Settings.IntervalHours}}" min="1" max="168">
                </div>
                <div class="col-md-6">
                    <label class="form-label">Keep Last N Backups</label>
                    <input type="number" class="form-control" id="retentionCount" 
                           value="{{.Settings.RetentionCount}}" min="1" max="30">
                </div>
            </div>

            <hr>
            <h5>Telegram</h5>
            
            <div class="mb-3">
                <label class="form-label">Bot Token</label>
                <input type="password" class="form-control" id="telegramBotToken" 
                       placeholder="{{if .Settings.HasBotToken}}••••••••••••{{else}}Enter bot token{{end}}">
                <small class="text-muted">Get from @BotFather on Telegram</small>
            </div>
            
            <div class="mb-3">
                <label class="form-label">Chat IDs</label>
                <input type="text" class="form-control" id="telegramChatIds" 
                       value="{{.Settings.TelegramChatIDsStr}}" 
                       placeholder="-1001234567890, -1009876543210">
                <small class="text-muted">Comma-separated channel/group IDs. Add bot as admin to the channel.</small>
            </div>
            
            <div class="row mb-3">
                <div class="col-md-6">
                    <label class="form-label">Chunk Size (MB)</label>
                    <input type="number" class="form-control" id="chunkSizeMb" 
                           value="{{.Settings.TelegramChunkSizeMB}}" min="10" max="50">
                    <small class="text-muted">Max 50MB (Telegram limit)</small>
                </div>
                <div class="col-md-6">
                    <div class="form-check form-switch mt-4">
                        <input class="form-check-input" type="checkbox" id="sendNotification" 
                               {{if .Settings.TelegramSendNotification}}checked{{end}}>
                        <label class="form-check-label" for="sendNotification">Send completion notification</label>
                    </div>
                </div>
            </div>

            <hr>
            <h5>Encryption (Optional)</h5>
            
            <div class="mb-3">
                <div class="form-check form-switch">
                    <input class="form-check-input" type="checkbox" id="gpgEnabled" 
                           {{if .Settings.GPGEnabled}}checked{{end}}>
                    <label class="form-check-label" for="gpgEnabled">Encrypt backups with GPG</label>
                </div>
            </div>
            
            <div class="mb-3" id="gpgPassphraseGroup" style="{{if not .Settings.GPGEnabled}}display:none{{end}}">
                <label class="form-label">Passphrase</label>
                <input type="password" class="form-control" id="gpgPassphrase" 
                       placeholder="{{if .Settings.HasGPGPassphrase}}••••••••••••{{else}}Enter passphrase{{end}}">
                <small class="text-muted">Used to encrypt/decrypt backup files</small>
            </div>

            <hr>
            <h5>What to Backup</h5>
            
            <div class="mb-3">
                <div class="form-check">
                    <input class="form-check-input" type="checkbox" id="includeDatabase" 
                           {{if .Settings.IncludeDatabase}}checked{{end}}>
                    <label class="form-check-label" for="includeDatabase">Database (pg_dump)</label>
                </div>
                <div class="form-check">
                    <input class="form-check-input" type="checkbox" id="includeConfig" 
                           {{if .Settings.IncludeConfig}}checked{{end}}>
                    <label class="form-check-label" for="includeConfig">Configuration files</label>
                </div>
            </div>

            <div class="d-flex gap-2">
                <button type="submit" class="btn btn-primary">Save Settings</button>
                <button type="button" class="btn btn-outline-secondary" id="btn-test">
                    <i class="bi bi-play-fill me-1"></i>Run Backup Now
                </button>
            </div>
        </form>
    </div>
</div>
{{end}}
```

---

## Cron Job Registration

```go
// modules/backup/module.go (partial)

func (m *Module) CronJobs() []module.CronJob {
    return []module.CronJob{
        {
            ID:       "backup-run",
            Interval: m.getBackupInterval(),
            Handler: func(ctx context.Context) error {
                settings, _ := m.service.GetSettings()
                if !settings.Enabled {
                    return nil
                }
                _, err := m.service.RunBackup(ctx)
                return err
            },
        },
        {
            ID:       "backup-retention",
            Interval: 24 * time.Hour,
            Handler: func(ctx context.Context) error {
                return m.service.EnforceRetention(ctx)
            },
        },
    }
}
```

---

## Restore Instructions

After downloading backup files from Telegram:

```bash
# 1. If chunked, reassemble
cat db.dump.part* > db.dump

# 2. Verify checksums (from _manifest.json)
sha256sum -c <<< "abc123... db.dump"

# 3. If encrypted, decrypt
gpg --decrypt db.dump.gpg > db.dump
gpg --decrypt config.tar.gz.gpg > config.tar.gz

# 4. Restore database
pg_restore \
  --dbname=postgresql://botginx:PASSWORD@localhost/botginx \
  --clean \
  --if-exists \
  db.dump

# 5. Restore config files
tar xzf config.tar.gz -C /etc/botginx/

# 6. Restart service
systemctl restart botginx
```

---

## Environment Variables

```bash
# Required for backup
DATABASE_URL=postgresql://botginx:pass@localhost/botginx

# For encryption key (same as cPanel integration)
BACKUP_ENCRYPTION_KEY=<64-char-hex>  # Generate: openssl rand -hex 32
```

---

## Security Considerations

1. **Bot token and passphrase are encrypted** at rest using AES-256-GCM
2. **Backups can be GPG-encrypted** before upload
3. **Bot must be admin** in the Telegram channel (can't read history, only post)
4. **Rate limiting handled** automatically with retry
5. **Temp files deleted** after upload

---

## File Summary

| File | Purpose |
|------|---------|
| `modules/backup/module.go` | Module init, routes, cron |
| `modules/backup/services/backup_service.go` | Main orchestration |
| `modules/backup/services/database.go` | pg_dump wrapper |
| `modules/backup/services/archiver.go` | tar.gz creation |
| `modules/backup/services/chunker.go` | File chunking |
| `modules/backup/telegram/client.go` | Bot API client |
| `modules/backup/telegram/upload.go` | Upload with manifest |
| `modules/backup/handlers/handler.go` | Admin UI |
| `modules/backup/templates/*.html` | Settings & history UI |

---

## Implementation Order

1. **Phase 1: Core** - Database schema, models, encryption
2. **Phase 2: pg_dump** - Database backup service
3. **Phase 3: Telegram** - Client and upload with chunking
4. **Phase 4: Admin UI** - Settings page
5. **Phase 5: Cron** - Scheduled backups
6. **Phase 6: Retention** - Cleanup old backups
7. **Phase 7: Restore docs** - Document restore process
