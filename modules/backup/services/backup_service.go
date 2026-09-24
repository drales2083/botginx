package services

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/botginx/botginx/modules/backup/models"
	"github.com/botginx/botginx/modules/backup/telegram"
	"github.com/botginx/botginx/modules/backup/types"
	"github.com/botginx/botginx/pkg/crypto"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type BackupService struct {
	db      *sqlx.DB
	tempDir string
	appDir  string
}

func NewBackupService(db *sqlx.DB, appDir string) *BackupService {
	return &BackupService{
		db:      db,
		tempDir: os.TempDir(),
		appDir:  appDir,
	}
}

func (s *BackupService) GetSettings() (*models.BackupSettings, error) {
	var settings models.BackupSettings
	err := s.db.Get(&settings, `SELECT * FROM backup_settings WHERE id = 'default'`)
	if err != nil {
		return nil, err
	}
	return &settings, nil
}

func (s *BackupService) UpdateSettings(input models.UpdateSettingsInput) error {
	settings, err := s.GetSettings()
	if err != nil {
		return err
	}

	if input.BackupName != nil {
		settings.BackupName = *input.BackupName
	}
	if input.Enabled != nil {
		settings.Enabled = *input.Enabled
	}
	if input.IntervalHours != nil {
		settings.IntervalHours = *input.IntervalHours
	}
	if input.RetentionCount != nil {
		settings.RetentionCount = *input.RetentionCount
	}
	if input.TelegramBotToken != nil && *input.TelegramBotToken != "" {
		encrypted, err := crypto.Encrypt(*input.TelegramBotToken)
		if err != nil {
			return err
		}
		encryptedBytes, _ := base64.StdEncoding.DecodeString(encrypted)
		settings.TelegramBotTokenEncrypted = encryptedBytes
	}
	if input.TelegramChatIDs != nil {
		ids := strings.Split(*input.TelegramChatIDs, ",")
		var cleaned []string
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if id != "" {
				cleaned = append(cleaned, id)
			}
		}
		settings.TelegramChatIDs = cleaned
	}
	if input.TelegramChunkSizeMB != nil {
		settings.TelegramChunkSizeMB = *input.TelegramChunkSizeMB
	}
	if input.TelegramSendNotification != nil {
		settings.TelegramSendNotification = *input.TelegramSendNotification
	}
	if input.IncludeDatabase != nil {
		settings.IncludeDatabase = *input.IncludeDatabase
	}
	if input.IncludeConfig != nil {
		settings.IncludeConfig = *input.IncludeConfig
	}

	_, err = s.db.Exec(`
		UPDATE backup_settings SET
			backup_name = $1,
			enabled = $2,
			interval_hours = $3,
			retention_count = $4,
			telegram_bot_token_encrypted = $5,
			telegram_chat_ids = $6,
			telegram_chunk_size_mb = $7,
			telegram_send_notification = $8,
			include_database = $9,
			include_config = $10,
			updated_at = NOW()
		WHERE id = 'default'
	`,
		settings.BackupName,
		settings.Enabled,
		settings.IntervalHours,
		settings.RetentionCount,
		settings.TelegramBotTokenEncrypted,
		pq.Array(settings.TelegramChatIDs),
		settings.TelegramChunkSizeMB,
		settings.TelegramSendNotification,
		settings.IncludeDatabase,
		settings.IncludeConfig,
	)
	return err
}

func (s *BackupService) RunBackup(ctx context.Context) (*models.BackupHistory, error) {
	settings, err := s.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("load settings: %w", err)
	}

	history := &models.BackupHistory{
		ID:        generateID(),
		Status:    "running",
		StartedAt: time.Now(),
	}
	if err := s.createHistory(history); err != nil {
		return nil, err
	}

	backupDir := filepath.Join(s.tempDir, "botginx-backup-"+history.ID)
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return s.failBackup(history, err)
	}
	defer os.RemoveAll(backupDir)

	var files []types.BackupFile

	if settings.IncludeDatabase {
		dbFile, err := s.dumpDatabase(ctx, backupDir)
		if err != nil {
			return s.failBackup(history, fmt.Errorf("database dump: %w", err))
		}
		history.DBSizeBytes = dbFile.Size
		files = append(files, dbFile)
	}

	if settings.IncludeConfig && len(settings.ConfigPaths) > 0 {
		configFile, err := s.archiveConfig(ctx, backupDir, settings.ConfigPaths)
		if err != nil {
			return s.failBackup(history, fmt.Errorf("config archive: %w", err))
		}
		history.ConfigSizeBytes = configFile.Size
		files = append(files, configFile)
	}

	var totalSize int64
	for i := range files {
		files[i].SHA256, _ = types.HashFile(files[i].Path)
		totalSize += files[i].Size
	}
	history.TotalSizeBytes = totalSize

	if len(settings.TelegramChatIDs) > 0 && len(settings.TelegramBotTokenEncrypted) > 0 {
		encodedToken := base64.StdEncoding.EncodeToString(settings.TelegramBotTokenEncrypted)
		botToken, err := crypto.Decrypt(encodedToken)
		if err != nil {
			return s.failBackup(history, fmt.Errorf("decrypt bot token: %w", err))
		}

		client := telegram.NewClient(botToken)
		result, err := client.UploadBackup(ctx, telegram.UploadOptions{
			Files:            files,
			ChatIDs:          settings.TelegramChatIDs,
			ChunkSizeMB:      settings.TelegramChunkSizeMB,
			BackupID:         history.ID,
			BackupName:       settings.BackupName,
			Timestamp:        history.StartedAt,
			SendNotification: settings.TelegramSendNotification,
		})
		if err != nil {
			return s.failBackup(history, fmt.Errorf("telegram upload: %w", err))
		}

		history.ChunkCount = result.ChunkCount
		history.TelegramMeta = models.JSONB(result.ToJSON())
	}

	history.Status = "completed"
	now := time.Now()
	history.CompletedAt = &now
	if err := s.updateHistory(history); err != nil {
		return nil, err
	}

	go s.EnforceRetention(context.Background())

	return history, nil
}

var excludedTables = []string{
	"visits",
	"short_link_clicks",
	"visitor_sessions",
	"hosting_visits",
	"conversions",
	"balance_transactions",
	"crypto_transactions",
	"backup_history",
}

func (s *BackupService) dumpDatabase(ctx context.Context, backupDir string) (types.BackupFile, error) {
	outputPath := filepath.Join(backupDir, "db.dump")

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return types.BackupFile{}, fmt.Errorf("DATABASE_URL not set")
	}

	args := []string{
		"--dbname=" + dbURL,
		"--format=custom",
		"--no-owner",
		"--no-acl",
		"--compress=6",
		"--file=" + outputPath,
	}
	for _, table := range excludedTables {
		args = append(args, "--exclude-table="+table)
	}

	cmd := exec.CommandContext(ctx, "pg_dump", args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return types.BackupFile{}, fmt.Errorf("pg_dump failed: %w\n%s", err, string(output))
	}

	info, err := os.Stat(outputPath)
	if err != nil {
		return types.BackupFile{}, err
	}

	return types.BackupFile{
		Name: "db.dump",
		Path: outputPath,
		Size: info.Size(),
	}, nil
}

func (s *BackupService) archiveConfig(ctx context.Context, backupDir string, paths []string) (types.BackupFile, error) {
	outputPath := filepath.Join(backupDir, "config.tar.gz")

	file, err := os.Create(outputPath)
	if err != nil {
		return types.BackupFile{}, err
	}
	defer file.Close()

	gw := gzip.NewWriter(file)
	tw := tar.NewWriter(gw)

	for _, p := range paths {
		fullPath := filepath.Join(s.appDir, p)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			continue
		}

		if err := addFileToTar(tw, fullPath, p); err != nil {
			tw.Close()
			gw.Close()
			return types.BackupFile{}, err
		}
	}

	tw.Close()
	gw.Close()
	file.Close()

	info, err := os.Stat(outputPath)
	if err != nil {
		return types.BackupFile{}, err
	}

	return types.BackupFile{
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

func (s *BackupService) createHistory(h *models.BackupHistory) error {
	_, err := s.db.Exec(`
		INSERT INTO backup_history (id, status, started_at)
		VALUES ($1, $2, $3)
	`, h.ID, h.Status, h.StartedAt)
	return err
}

func (s *BackupService) updateHistory(h *models.BackupHistory) error {
	_, err := s.db.Exec(`
		UPDATE backup_history SET
			status = $1,
			completed_at = $2,
			db_size_bytes = $3,
			config_size_bytes = $4,
			total_size_bytes = $5,
			chunk_count = $6,
			telegram_meta = $7,
			error_message = $8
		WHERE id = $9
	`,
		h.Status,
		h.CompletedAt,
		h.DBSizeBytes,
		h.ConfigSizeBytes,
		h.TotalSizeBytes,
		h.ChunkCount,
		h.TelegramMeta,
		h.ErrorMessage,
		h.ID,
	)
	return err
}

func (s *BackupService) failBackup(h *models.BackupHistory, err error) (*models.BackupHistory, error) {
	h.Status = "failed"
	errMsg := err.Error()
	h.ErrorMessage = &errMsg
	now := time.Now()
	h.CompletedAt = &now
	s.updateHistory(h)
	return h, err
}

func (s *BackupService) GetHistory(limit int) ([]models.BackupHistory, error) {
	if limit <= 0 {
		limit = 20
	}
	var history []models.BackupHistory
	err := s.db.Select(&history, `
		SELECT * FROM backup_history
		ORDER BY started_at DESC
		LIMIT $1
	`, limit)
	return history, err
}

func (s *BackupService) EnforceRetention(ctx context.Context) error {
	settings, err := s.GetSettings()
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		DELETE FROM backup_history
		WHERE id NOT IN (
			SELECT id FROM backup_history
			ORDER BY started_at DESC
			LIMIT $1
		)
	`, settings.RetentionCount)
	return err
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
