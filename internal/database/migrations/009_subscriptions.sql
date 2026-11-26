-- Migration: 009_subscriptions
-- Description: Create subscriptions table for user subscription links

-- Create subscriptions table
CREATE TABLE IF NOT EXISTS subscriptions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL UNIQUE,
    token VARCHAR(64) NOT NULL UNIQUE,
    short_code VARCHAR(16) UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_access_at DATETIME,
    access_count INTEGER NOT NULL DEFAULT 0,
    last_ip VARCHAR(45),
    last_ua VARCHAR(256),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Create indexes for fast lookups
CREATE INDEX IF NOT EXISTS idx_subscriptions_token ON subscriptions(token);
CREATE INDEX IF NOT EXISTS idx_subscriptions_short_code ON subscriptions(short_code);
CREATE INDEX IF NOT EXISTS idx_subscriptions_user_id ON subscriptions(user_id);
