-- Trials Migration
-- Creates table for trial subscriptions

-- Trials table
CREATE TABLE IF NOT EXISTS trials (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL UNIQUE,
    status VARCHAR(32) DEFAULT 'active',
    start_at DATETIME NOT NULL,
    expire_at DATETIME NOT NULL,
    traffic_used BIGINT DEFAULT 0,
    converted_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_trials_user ON trials(user_id);
CREATE INDEX IF NOT EXISTS idx_trials_status ON trials(status);
CREATE INDEX IF NOT EXISTS idx_trials_expire ON trials(expire_at);
