-- +goose Up
ALTER TABLE users ADD COLUMN IF NOT EXISTS telegram_default_domain_id TEXT REFERENCES domains(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS telegram_default_domain_id;
