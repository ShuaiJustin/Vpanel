-- Migration: 008_logs_enhancement
-- Description: Enhance logs table with request_id, fields, and optimized indexes

-- +migrate Up

-- Note: Columns may already exist from GORM AutoMigrate
-- SQLite doesn't support IF NOT EXISTS in ALTER TABLE ADD COLUMN

-- Create composite indexes for common queries (only for existing columns)
CREATE INDEX IF NOT EXISTS idx_logs_level_created ON logs(level, created_at);
CREATE INDEX IF NOT EXISTS idx_logs_user_created ON logs(user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_logs_request_id ON logs(request_id);

-- +migrate Down
DROP INDEX IF EXISTS idx_logs_request_id;
DROP INDEX IF EXISTS idx_logs_user_created;
DROP INDEX IF EXISTS idx_logs_level_created;
