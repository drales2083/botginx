CREATE TABLE IF NOT EXISTS tracking_pixels (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    label        text NOT NULL,
    domain       text NOT NULL,
    format       text NOT NULL DEFAULT 'gif',
    token        text NOT NULL UNIQUE,
    verified     boolean NOT NULL DEFAULT false,
    last_open_at timestamptz,
    created_at   timestamptz NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_tracking_pixels_user ON tracking_pixels(user_id);

CREATE TABLE IF NOT EXISTS tracking_pixel_opens (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    pixel_id       uuid NOT NULL REFERENCES tracking_pixels(id) ON DELETE CASCADE,
    opened_at      timestamptz NOT NULL DEFAULT NOW(),
    ip_hash        text,
    user_agent     text,
    classification text NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_tracking_pixel_opens_pixel ON tracking_pixel_opens(pixel_id);
CREATE INDEX IF NOT EXISTS idx_tracking_pixel_opens_opened_at ON tracking_pixel_opens(opened_at);
