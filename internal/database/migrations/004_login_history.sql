-- Migration: 004_login_history
-- Description: Create login_history table for tracking login attempts

-- +migrate Up
CREATE TABLE IF NOT EXISTS login_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    ip VARCHAR(50),
    user_agent VARCHAR(255),
    success BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_login_history_user_id ON login_history(user_id);
CREATE INDEX IF NOT EXISTS idx_login_history_created_at ON login_history(created_at);

-- +migrate Down
DROP INDEX IF EXISTS idx_login_history_created_at;
DROP INDEX IF EXISTS idx_login_history_user_id;
DROP TABLE IF EXISTS login_history;
