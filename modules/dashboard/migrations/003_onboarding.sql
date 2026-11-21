-- Onboarding checklist tracking
ALTER TABLE users ADD COLUMN IF NOT EXISTS onboarding_dismissed_at TIMESTAMP;
ALTER TABLE users ADD COLUMN IF NOT EXISTS bot_settings_visited_at TIMESTAMP;
