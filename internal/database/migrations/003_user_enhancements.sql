-- Migration: User model enhancements
-- Version: 003
-- Requirements: 17.6, 17.7, 17.8

-- Note: Columns may already exist from GORM AutoMigrate
-- SQLite doesn't support IF NOT EXISTS in ALTER TABLE ADD COLUMN
-- So we just create indexes (which support IF NOT EXISTS)

-- Add indexes for columns (columns should exist from GORM AutoMigrate)
CREATE INDEX IF NOT EXISTS idx_users_expires_at ON users(expires_at);
CREATE INDEX IF NOT EXISTS idx_users_traffic_limit ON users(traffic_limit);
