-- Migration: 016_user_portal_fields
-- Description: Add User Portal fields to users table

-- Add new columns to users table for User Portal
ALTER TABLE users ADD COLUMN email_verified BOOLEAN DEFAULT FALSE;
ALTER TABLE users ADD COLUMN email_verified_at DATETIME;
ALTER TABLE users ADD COLUMN two_factor_enabled BOOLEAN DEFAULT FALSE;
ALTER TABLE users ADD COLUMN last_login_at DATETIME;
ALTER TABLE users ADD COLUMN last_login_ip VARCHAR(45);
ALTER TABLE users ADD COLUMN avatar_url VARCHAR(512);
ALTER TABLE users ADD COLUMN display_name VARCHAR(64);
ALTER TABLE users ADD COLUMN telegram_id BIGINT;
ALTER TABLE users ADD COLUMN notify_email BOOLEAN DEFAULT TRUE;
ALTER TABLE users ADD COLUMN notify_telegram BOOLEAN DEFAULT FALSE;
ALTER TABLE users ADD COLUMN theme VARCHAR(16) DEFAULT 'auto';
ALTER TABLE users ADD COLUMN language VARCHAR(8) DEFAULT 'zh-CN';
ALTER TABLE users ADD COLUMN invited_by INTEGER;

-- Create index for telegram_id
CREATE INDEX IF NOT EXISTS idx_users_telegram_id ON users(telegram_id);
CREATE INDEX IF NOT EXISTS idx_users_invited_by ON users(invited_by);
