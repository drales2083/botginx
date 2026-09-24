package models

import (
	"strings"
	"time"

	"github.com/lib/pq"
)

type BackupSettings struct {
	ID                        string         `db:"id" json:"id"`
	BackupName                string         `db:"backup_name" json:"backupName"`
	Enabled                   bool           `db:"enabled" json:"enabled"`
	IntervalHours             int            `db:"interval_hours" json:"intervalHours"`
	RetentionCount            int            `db:"retention_count" json:"retentionCount"`
	TelegramBotTokenEncrypted []byte         `db:"telegram_bot_token_encrypted" json:"-"`
	TelegramChatIDs           pq.StringArray `db:"telegram_chat_ids" json:"telegramChatIds"`
	TelegramChunkSizeMB       int            `db:"telegram_chunk_size_mb" json:"telegramChunkSizeMb"`
	TelegramSendNotification  bool           `db:"telegram_send_notification" json:"telegramSendNotification"`
	IncludeDatabase           bool           `db:"include_database" json:"includeDatabase"`
	IncludeConfig             bool           `db:"include_config" json:"includeConfig"`
	ConfigPaths               pq.StringArray `db:"config_paths" json:"configPaths"`
	UpdatedAt                 time.Time      `db:"updated_at" json:"updatedAt"`
}

func (s *BackupSettings) HasBotToken() bool {
	return len(s.TelegramBotTokenEncrypted) > 0
}

func (s *BackupSettings) TelegramChatIDsStr() string {
	return strings.Join(s.TelegramChatIDs, ", ")
}

type UpdateSettingsInput struct {
	BackupName               *string `json:"backupName"`
	Enabled                  *bool   `json:"enabled"`
	IntervalHours            *int    `json:"intervalHours"`
	RetentionCount           *int    `json:"retentionCount"`
	TelegramBotToken         *string `json:"telegramBotToken"`
	TelegramChatIDs          *string `json:"telegramChatIds"`
	TelegramChunkSizeMB      *int    `json:"telegramChunkSizeMb"`
	TelegramSendNotification *bool   `json:"telegramSendNotification"`
	IncludeDatabase          *bool   `json:"includeDatabase"`
	IncludeConfig            *bool   `json:"includeConfig"`
}
