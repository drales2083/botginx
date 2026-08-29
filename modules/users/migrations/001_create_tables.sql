-- Users module - extends auth users table with role
-- The base users table is created by auth module
-- This migration adds role column if not exists

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name = 'users' AND column_name = 'role') THEN
        ALTER TABLE users ADD COLUMN role TEXT DEFAULT 'user';
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
