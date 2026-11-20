-- Provisioning jobs track multi-step server setup
CREATE TABLE IF NOT EXISTS server_provisions (
    id              VARCHAR(36) PRIMARY KEY,
    server_id       VARCHAR(36) NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    current_step    INT DEFAULT 0,
    total_steps     INT DEFAULT 5,
    step_name       VARCHAR(100),
    output          TEXT,
    error           TEXT,
    acme_domain     VARCHAR(255),
    started_at      TIMESTAMP,
    completed_at    TIMESTAMP,
    created_at      TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_server_provisions_server_id ON server_provisions(server_id);
CREATE INDEX IF NOT EXISTS idx_server_provisions_status ON server_provisions(status);

-- Add provision_id to servers for tracking current/last provision
ALTER TABLE servers ADD COLUMN IF NOT EXISTS provision_id VARCHAR(36);
