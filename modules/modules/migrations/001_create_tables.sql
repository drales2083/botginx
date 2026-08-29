-- Module states table

CREATE TABLE IF NOT EXISTS module_states (
    id              TEXT PRIMARY KEY,
    module_id       TEXT NOT NULL UNIQUE,
    name            TEXT NOT NULL,
    description     TEXT,
    version         TEXT DEFAULT '1.0.0',
    is_enabled      BOOLEAN DEFAULT TRUE,
    is_core         BOOLEAN DEFAULT FALSE,
    settings        TEXT,
    created_at      TIMESTAMP DEFAULT NOW(),
    updated_at      TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_module_states_module_id ON module_states(module_id);
CREATE INDEX IF NOT EXISTS idx_module_states_enabled ON module_states(is_enabled);
