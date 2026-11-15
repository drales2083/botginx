-- +goose Up
ALTER TABLE users ADD COLUMN IF NOT EXISTS telegram_alert_dismissed BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS telegram_alert_dismissed;
