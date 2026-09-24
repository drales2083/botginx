package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

type BackupHistory struct {
	ID              string     `db:"id" json:"id"`
	Status          string     `db:"status" json:"status"`
	StartedAt       time.Time  `db:"started_at" json:"startedAt"`
	CompletedAt     *time.Time `db:"completed_at" json:"completedAt"`
	DBSizeBytes     int64      `db:"db_size_bytes" json:"dbSizeBytes"`
	ConfigSizeBytes int64      `db:"config_size_bytes" json:"configSizeBytes"`
	TotalSizeBytes  int64      `db:"total_size_bytes" json:"totalSizeBytes"`
	ChunkCount      int        `db:"chunk_count" json:"chunkCount"`
	TelegramMeta    JSONB      `db:"telegram_meta" json:"telegramMeta"`
	ErrorMessage    *string    `db:"error_message" json:"errorMessage"`
	CreatedAt       time.Time  `db:"created_at" json:"createdAt"`
}

type JSONB json.RawMessage

func (j JSONB) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return []byte(j), nil
}

func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	*j = bytes
	return nil
}

func (j JSONB) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return j, nil
}

func (j *JSONB) UnmarshalJSON(data []byte) error {
	*j = data
	return nil
}
