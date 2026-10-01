-- Admin activity logs for audit trail
CREATE TABLE IF NOT EXISTS admin_activity_logs (
    id VARCHAR(26) PRIMARY KEY,
    admin_id TEXT NOT NULL,
    admin_email TEXT NOT NULL,
    action VARCHAR(50) NOT NULL,
    target_type VARCHAR(20),
    target_id TEXT,
    target_name TEXT,
    details JSONB DEFAULT '{}',
    ip_address VARCHAR(45),
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_admin_logs_admin ON admin_activity_logs(admin_email);
CREATE INDEX IF NOT EXISTS idx_admin_logs_action ON admin_activity_logs(action);
CREATE INDEX IF NOT EXISTS idx_admin_logs_target ON admin_activity_logs(target_type, target_id);
CREATE INDEX IF NOT EXISTS idx_admin_logs_created ON admin_activity_logs(created_at DESC);
