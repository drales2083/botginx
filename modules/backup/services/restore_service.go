package services

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/botginx/botginx/modules/backup/telegram"
	"github.com/botginx/botginx/modules/backup/types"
	"github.com/botginx/botginx/pkg/crypto"
)

type RestoreManifest struct {
	Version   int                  `json:"version"`
	BackupID  string               `json:"backup_id"`
	Timestamp string               `json:"timestamp"`
	Host      string               `json:"host"`
	Files     []RestoreFileEntry   `json:"files"`
}

type RestoreFileEntry struct {
	Name     string              `json:"name"`
	Size     int64               `json:"size"`
	SHA256   string              `json:"sha256"`
	Chunked  bool                `json:"chunked"`
	Chunks   []RestoreChunkEntry `json:"chunks,omitempty"`
	Telegram *RestoreTelegramInfo `json:"telegram,omitempty"`
}

type RestoreChunkEntry struct {
	Part     int                  `json:"part"`
	Name     string               `json:"name"`
	Size     int64                `json:"size"`
	SHA256   string               `json:"sha256"`
	Telegram *RestoreTelegramInfo `json:"telegram,omitempty"`
}

type RestoreTelegramInfo struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
}

type RestoreProgress struct {
	Phase           string  `json:"phase"`
	TotalFiles      int     `json:"totalFiles"`
	CompletedFiles  int     `json:"completedFiles"`
	CurrentFile     string  `json:"currentFile"`
	TotalBytes      int64   `json:"totalBytes"`
	DownloadedBytes int64   `json:"downloadedBytes"`
	Percentage      int     `json:"percentage"`
	Error           string  `json:"error,omitempty"`
}

type RestoreResult struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	FilesRestored int   `json:"filesRestored"`
}

func (s *BackupService) ValidateManifest(manifest *RestoreManifest) error {
	if manifest.Version != 1 {
		return fmt.Errorf("unsupported manifest version: %d", manifest.Version)
	}

	if len(manifest.Files) == 0 {
		return fmt.Errorf("no files in manifest")
	}

	for _, file := range manifest.Files {
		if file.Chunked {
			for _, chunk := range file.Chunks {
				if chunk.Telegram == nil || chunk.Telegram.FileID == "" {
					return fmt.Errorf("missing file_id for chunk: %s (backup may be too old)", chunk.Name)
				}
			}
		} else {
			if file.Telegram == nil || file.Telegram.FileID == "" {
				return fmt.Errorf("missing file_id for file: %s (backup may be too old)", file.Name)
			}
		}
	}

	return nil
}

func (s *BackupService) RunRestore(ctx context.Context, manifest *RestoreManifest, onProgress func(RestoreProgress)) (*RestoreResult, error) {
	progress := RestoreProgress{
		Phase:      "validating",
		TotalFiles: len(manifest.Files),
	}

	for _, f := range manifest.Files {
		if f.Chunked {
			progress.TotalBytes += f.Size
		} else {
			progress.TotalBytes += f.Size
		}
	}

	if onProgress != nil {
		onProgress(progress)
	}

	// Get settings for bot token
	settings, err := s.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("load settings: %w", err)
	}

	if len(settings.TelegramBotTokenEncrypted) == 0 {
		return nil, fmt.Errorf("telegram bot token not configured")
	}

	// Decrypt bot token
	botToken, err := s.decryptBotToken(settings.TelegramBotTokenEncrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt bot token: %w", err)
	}

	client := telegram.NewClient(botToken)

	// Create temp directory for downloads
	restoreDir := filepath.Join(s.tempDir, "botginx-restore-"+manifest.BackupID)
	if err := os.MkdirAll(restoreDir, 0700); err != nil {
		return nil, fmt.Errorf("create restore dir: %w", err)
	}
	defer os.RemoveAll(restoreDir)

	// Download all files
	progress.Phase = "downloading"
	if onProgress != nil {
		onProgress(progress)
	}

	for fileIdx, file := range manifest.Files {
		if file.Chunked {
			// Download chunks
			for _, chunk := range file.Chunks {
				progress.CurrentFile = chunk.Name
				if onProgress != nil {
					onProgress(progress)
				}

				chunkPath := filepath.Join(restoreDir, chunk.Name)
				if err := client.DownloadFile(ctx, chunk.Telegram.FileID, chunkPath); err != nil {
					return nil, fmt.Errorf("download %s: %w", chunk.Name, err)
				}

				// Verify chunk hash
				hash, err := types.HashFile(chunkPath)
				if err != nil {
					return nil, fmt.Errorf("hash %s: %w", chunk.Name, err)
				}
				if hash != chunk.SHA256 {
					return nil, fmt.Errorf("chunk %s corrupted: hash mismatch", chunk.Name)
				}

				progress.DownloadedBytes += chunk.Size
				progress.Percentage = int(float64(progress.DownloadedBytes) / float64(progress.TotalBytes) * 100)
				if onProgress != nil {
					onProgress(progress)
				}
			}
		} else {
			// Download single file
			progress.CurrentFile = file.Name
			if onProgress != nil {
				onProgress(progress)
			}

			filePath := filepath.Join(restoreDir, file.Name)
			if err := client.DownloadFile(ctx, file.Telegram.FileID, filePath); err != nil {
				return nil, fmt.Errorf("download %s: %w", file.Name, err)
			}

			// Verify hash
			hash, err := types.HashFile(filePath)
			if err != nil {
				return nil, fmt.Errorf("hash %s: %w", file.Name, err)
			}
			if hash != file.SHA256 {
				return nil, fmt.Errorf("file %s corrupted: hash mismatch", file.Name)
			}

			progress.DownloadedBytes += file.Size
			progress.Percentage = int(float64(progress.DownloadedBytes) / float64(progress.TotalBytes) * 100)
		}

		progress.CompletedFiles = fileIdx + 1
		if onProgress != nil {
			onProgress(progress)
		}
	}

	// Merge chunks
	progress.Phase = "merging"
	if onProgress != nil {
		onProgress(progress)
	}

	for _, file := range manifest.Files {
		if !file.Chunked {
			continue
		}

		progress.CurrentFile = file.Name
		if onProgress != nil {
			onProgress(progress)
		}

		outputPath := filepath.Join(restoreDir, file.Name)
		outFile, err := os.Create(outputPath)
		if err != nil {
			return nil, fmt.Errorf("create %s: %w", file.Name, err)
		}

		// Sort chunks by part number
		sortedChunks := make([]RestoreChunkEntry, len(file.Chunks))
		copy(sortedChunks, file.Chunks)
		sort.Slice(sortedChunks, func(i, j int) bool {
			return sortedChunks[i].Part < sortedChunks[j].Part
		})

		for _, chunk := range sortedChunks {
			chunkPath := filepath.Join(restoreDir, chunk.Name)
			chunkData, err := os.ReadFile(chunkPath)
			if err != nil {
				outFile.Close()
				return nil, fmt.Errorf("read chunk %s: %w", chunk.Name, err)
			}
			outFile.Write(chunkData)
			os.Remove(chunkPath) // Clean up chunk
		}
		outFile.Close()

		// Verify merged file
		hash, err := types.HashFile(outputPath)
		if err != nil {
			return nil, fmt.Errorf("hash merged %s: %w", file.Name, err)
		}
		if hash != file.SHA256 {
			return nil, fmt.Errorf("merged file %s corrupted: hash mismatch", file.Name)
		}
	}

	// Restore database if db.dump exists
	progress.Phase = "restoring"
	if onProgress != nil {
		onProgress(progress)
	}

	dbDumpPath := filepath.Join(restoreDir, "db.dump")
	if _, err := os.Stat(dbDumpPath); err == nil {
		progress.CurrentFile = "database"
		if onProgress != nil {
			onProgress(progress)
		}

		// Create pre-restore backup
		preRestorePath := filepath.Join(s.tempDir, fmt.Sprintf("pre-restore-%d.dump", time.Now().Unix()))
		if err := s.createPreRestoreBackup(ctx, preRestorePath); err != nil {
			// Non-fatal, continue with restore
			fmt.Printf("Warning: could not create pre-restore backup: %v\n", err)
		}

		if err := s.restoreDatabase(ctx, dbDumpPath); err != nil {
			return nil, fmt.Errorf("restore database: %w", err)
		}
	}

	// Restore config if config.tar.gz exists
	configPath := filepath.Join(restoreDir, "config.tar.gz")
	if _, err := os.Stat(configPath); err == nil {
		progress.CurrentFile = "config"
		if onProgress != nil {
			onProgress(progress)
		}

		if err := s.restoreConfig(ctx, configPath); err != nil {
			return nil, fmt.Errorf("restore config: %w", err)
		}
	}

	progress.Phase = "completed"
	progress.Percentage = 100
	if onProgress != nil {
		onProgress(progress)
	}

	return &RestoreResult{
		Success:       true,
		Message:       "Restore completed successfully",
		FilesRestored: len(manifest.Files),
	}, nil
}

func (s *BackupService) decryptBotToken(encrypted []byte) (string, error) {
	encodedToken := base64.StdEncoding.EncodeToString(encrypted)
	return crypto.Decrypt(encodedToken)
}

func (s *BackupService) createPreRestoreBackup(ctx context.Context, outputPath string) error {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return fmt.Errorf("DATABASE_URL not set")
	}

	cmd := exec.CommandContext(ctx, "pg_dump",
		"--dbname="+dbURL,
		"--format=custom",
		"--no-owner",
		"--no-acl",
		"--compress=6",
		"--file="+outputPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pg_dump failed: %w\n%s", err, string(output))
	}

	return nil
}

func (s *BackupService) restoreDatabase(ctx context.Context, dumpPath string) error {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return fmt.Errorf("DATABASE_URL not set")
	}

	// Try restore without --clean first (fresh database scenario)
	// If that fails with "already exists" errors, retry with --clean
	cmd := exec.CommandContext(ctx, "pg_restore",
		"--dbname="+dbURL,
		"--no-owner",
		"--no-acl",
		"--single-transaction",
		dumpPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if it's "already exists" error - retry with --clean
		outputStr := string(output)
		if strings.Contains(outputStr, "already exists") || strings.Contains(outputStr, "duplicate key") {
			cmd2 := exec.CommandContext(ctx, "pg_restore",
				"--dbname="+dbURL,
				"--clean",
				"--if-exists",
				"--no-owner",
				"--no-acl",
				"--single-transaction",
				dumpPath,
			)
			output2, err2 := cmd2.CombinedOutput()
			if err2 != nil {
				return fmt.Errorf("pg_restore with --clean failed: %w\n%s", err2, string(output2))
			}
			return nil
		}
		return fmt.Errorf("pg_restore failed: %w\n%s", err, outputStr)
	}

	return nil
}


func (s *BackupService) restoreConfig(ctx context.Context, tarPath string) error {
	cmd := exec.CommandContext(ctx, "tar",
		"-xzf", tarPath,
		"-C", s.appDir,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tar extract failed: %w\n%s", err, string(output))
	}

	return nil
}
