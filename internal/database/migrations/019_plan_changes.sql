-- Migration: 019_plan_changes.sql
-- Description: Add pending_downgrades table for plan change functionality

-- Pending downgrades table
CREATE TABLE IF NOT EXISTS pending_downgrades (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL UNIQUE,
    current_plan_id INTEGER NOT NULL,
    new_plan_id INTEGER NOT NULL,
    effective_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (current_plan_id) REFERENCES commercial_plans(id),
    FOREIGN KEY (new_plan_id) REFERENCES commercial_plans(id)
);

CREATE INDEX IF NOT EXISTS idx_pending_downgrades_user ON pending_downgrades(user_id);
CREATE INDEX IF NOT EXISTS idx_pending_downgrades_effective ON pending_downgrades(effective_at);
