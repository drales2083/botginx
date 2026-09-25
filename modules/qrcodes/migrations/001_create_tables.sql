CREATE TABLE IF NOT EXISTS qr_codes (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL DEFAULT '',
    url TEXT NOT NULL,
    redirect_link_id TEXT REFERENCES redirect_links(id) ON DELETE SET NULL,
    fg_color VARCHAR(7) NOT NULL DEFAULT '#000000',
    bg_color VARCHAR(7) NOT NULL DEFAULT '#FFFFFF',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_qr_codes_user_id ON qr_codes(user_id);
