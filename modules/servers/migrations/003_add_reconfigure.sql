-- Add server_reconfigurations table for tracking DNS reconfiguration jobs
CREATE TABLE IF NOT EXISTS server_reconfigurations (
    id VARCHAR(36) PRIMARY KEY,
    server_id VARCHAR(36) NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    current_step INTEGER NOT NULL DEFAULT 0,
    total_steps INTEGER NOT NULL DEFAULT 4,
    step_name TEXT,
    output TEXT,
    error TEXT,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_server_reconfigurations_server_id ON server_reconfigurations(server_id);
CREATE INDEX IF NOT EXISTS idx_server_reconfigurations_status ON server_reconfigurations(status);
