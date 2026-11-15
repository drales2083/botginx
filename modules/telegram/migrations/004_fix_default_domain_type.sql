-- +goose Up
-- Fix column type: UUID → TEXT (domains.id is TEXT)
ALTER TABLE users DROP COLUMN IF EXISTS telegram_default_domain_id;
ALTER TABLE users ADD COLUMN telegram_default_domain_id TEXT REFERENCES domains(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS telegram_default_domain_id;
ALTER TABLE users ADD COLUMN telegram_default_domain_id UUID REFERENCES domains(id) ON DELETE SET NULL;
