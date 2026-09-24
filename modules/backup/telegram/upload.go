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

type uploadItem struct {
	path    string
	caption string
}

func (c *Client) UploadBackup(ctx context.Context, opts UploadOptions) (*UploadResult, error) {
	result := &UploadResult{
		Channels: make(map[string]ChannelResult),
	}

	if len(opts.Files) == 0 {
		return result, nil
	}

	manifest := Manifest{
		Version:   1,
		BackupID:  opts.BackupID,
		Timestamp: opts.Timestamp.Format(time.RFC3339),
		Host:      getHostname(),
		Files:     make([]FileEntry, 0),
	}

	var uploadQueue []uploadItem

	for _, file := range opts.Files {
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

	manifestPath := filepath.Join(filepath.Dir(opts.Files[0].Path), "_manifest.json")
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(manifestPath, manifestData, 0600); err != nil {
		return nil, err
	}
	uploadQueue = append(uploadQueue, uploadItem{
		path:    manifestPath,
		caption: "📋 _manifest.json",
	})

	for _, chatID := range opts.ChatIDs {
		chanResult := ChannelResult{}

		for _, item := range uploadQueue {
			msg, err := c.SendDocument(ctx, chatID, item.path, item.caption)
			if err != nil {
				chanResult.Error = err.Error()
				break
			}
			chanResult.MessageIDs = append(chanResult.MessageIDs, msg.MessageID)

			if filepath.Base(item.path) == "_manifest.json" {
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
