-- Migration: 021_subscription_pauses
-- Description: Add subscription pauses table for pause/resume functionality

-- Subscription pauses table
CREATE TABLE IF NOT EXISTS subscription_pauses (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    paused_at DATETIME NOT NULL,
    resumed_at DATETIME,
    remaining_days INTEGER NOT NULL,
    remaining_traffic BIGINT NOT NULL,
    auto_resume_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

-- Indexes for subscription_pauses
CREATE INDEX IF NOT EXISTS idx_subscription_pauses_user ON subscription_pauses(user_id, paused_at DESC);
CREATE INDEX IF NOT EXISTS idx_subscription_pauses_auto_resume ON subscription_pauses(resumed_at, auto_resume_at);
