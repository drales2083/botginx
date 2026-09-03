-- Add pass_params column for passing query parameters to destination
ALTER TABLE redirect_links ADD COLUMN IF NOT EXISTS pass_params BOOLEAN NOT NULL DEFAULT FALSE;
