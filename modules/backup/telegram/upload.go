package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/botginx/botginx/modules/backup/types"
)

var chunkPattern = regexp.MustCompile(`\.part\d{3}$`)

type UploadOptions struct {
	Files            []types.BackupFile
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
	MessageIDs []int  `json:"messageIds"`
	ManifestID int    `json:"manifestId"`
	Error      string `json:"error,omitempty"`
}

func (r *UploadResult) ToJSON() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"channels": r.Channels,
	})
	return data
}

type Manifest struct {
	Version   int         `json:"version"`
	BackupID  string      `json:"backup_id"`
	Timestamp string      `json:"timestamp"`
	Host      string      `json:"host"`
	Files     []FileEntry `json:"files"`
}

type FileEntry struct {
	Name     string        `json:"name"`
	Size     int64         `json:"size"`
	SHA256   string        `json:"sha256"`
	Chunked  bool          `json:"chunked"`
	Chunks   []ChunkEntry  `json:"chunks,omitempty"`
	Telegram *TelegramInfo `json:"telegram,omitempty"`
}

type ChunkEntry struct {
	Part     int           `json:"part"`
	Name     string        `json:"name"`
	Size     int64         `json:"size"`
	SHA256   string        `json:"sha256"`
	Telegram *TelegramInfo `json:"telegram,omitempty"`
}

type TelegramInfo struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
}

type uploadItem struct {
	path    string
	caption string
}

func (c *Client) UploadBackup(ctx context.Context, opts UploadOptions) (*UploadResult, error) {
	result := &UploadResult{
		Channels: make(map[string]ChannelResult),
	}

	if len(opts.Files) == 0 || len(opts.ChatIDs) == 0 {
		return result, nil
	}

	manifest := Manifest{
		Version:   1,
		BackupID:  opts.BackupID,
		Timestamp: opts.Timestamp.Format(time.RFC3339),
		Host:      getHostname(),
		Files:     make([]FileEntry, 0),
	}

	// Build file entries and upload queue
	type queueItem struct {
		path      string
		caption   string
		fileIdx   int // index in manifest.Files
		chunkIdx  int // index in Chunks array, -1 if not chunked
	}
	var uploadQueue []queueItem

	for fileIdx, file := range opts.Files {
		chunks, err := types.ChunkFile(file.Path, opts.ChunkSizeMB)
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
			for chunkIdx, chunk := range chunks {
				entry.Chunks = append(entry.Chunks, ChunkEntry{
					Part:   chunk.Part,
					Name:   chunk.Name,
					Size:   chunk.Size,
					SHA256: chunk.SHA256,
				})
				uploadQueue = append(uploadQueue, queueItem{
					path:     chunk.Path,
					caption:  fmt.Sprintf("📦 %s (%s)", file.Name, chunk.Name),
					fileIdx:  fileIdx,
					chunkIdx: chunkIdx,
				})
				result.ChunkCount++
			}
		} else {
			uploadQueue = append(uploadQueue, queueItem{
				path:     file.Path,
				caption:  fmt.Sprintf("📄 %s", file.Name),
				fileIdx:  fileIdx,
				chunkIdx: -1,
			})
		}

		result.TotalBytes += file.Size
		manifest.Files = append(manifest.Files, entry)
	}

	// Upload to first chat ID, capturing file_ids
	firstChatID := opts.ChatIDs[0]
	firstResult := ChannelResult{}

	for _, item := range uploadQueue {
		msg, err := c.SendDocument(ctx, firstChatID, item.path, item.caption)
		if err != nil {
			firstResult.Error = err.Error()
			break
		}
		firstResult.MessageIDs = append(firstResult.MessageIDs, msg.MessageID)

		// Capture file_id and store in manifest
		if msg.Document != nil {
			telegramInfo := &TelegramInfo{
				FileID:       msg.Document.FileID,
				FileUniqueID: msg.Document.FileUniqueID,
			}
			if item.chunkIdx >= 0 {
				manifest.Files[item.fileIdx].Chunks[item.chunkIdx].Telegram = telegramInfo
			} else {
				manifest.Files[item.fileIdx].Telegram = telegramInfo
			}
		}
	}

	// Write manifest WITH file_ids
	manifestPath := filepath.Join(filepath.Dir(opts.Files[0].Path), "_manifest.json")
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(manifestPath, manifestData, 0600); err != nil {
		return nil, err
	}

	// Upload manifest to first chat
	if firstResult.Error == "" {
		msg, err := c.SendDocument(ctx, firstChatID, manifestPath, "📋 _manifest.json")
		if err != nil {
			firstResult.Error = err.Error()
		} else {
			firstResult.MessageIDs = append(firstResult.MessageIDs, msg.MessageID)
			firstResult.ManifestID = msg.MessageID
		}
	}

	// Send notification to first chat
	if opts.SendNotification && firstResult.Error == "" {
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
		c.SendMessage(ctx, firstChatID, notification, "HTML")
	}

	result.Channels[firstChatID] = firstResult

	// Upload to additional chat IDs (file_ids already captured, just re-upload)
	for _, chatID := range opts.ChatIDs[1:] {
		chanResult := ChannelResult{}

		for _, item := range uploadQueue {
			msg, err := c.SendDocument(ctx, chatID, item.path, item.caption)
			if err != nil {
				chanResult.Error = err.Error()
				break
			}
			chanResult.MessageIDs = append(chanResult.MessageIDs, msg.MessageID)
		}

		// Upload manifest
		if chanResult.Error == "" {
			msg, err := c.SendDocument(ctx, chatID, manifestPath, "📋 _manifest.json")
			if err != nil {
				chanResult.Error = err.Error()
			} else {
				chanResult.MessageIDs = append(chanResult.MessageIDs, msg.MessageID)
				chanResult.ManifestID = msg.MessageID
			}
		}

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

	// Cleanup chunk files
	for _, item := range uploadQueue {
		base := filepath.Base(item.path)
		if chunkPattern.MatchString(base) {
			os.Remove(item.path)
		}
	}

	return result, nil
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
