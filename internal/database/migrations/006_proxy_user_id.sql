-- Migration 006: Ensure proxy user_id field exists
-- This migration ensures the user_id column exists in the proxies table
-- and adds necessary indexes for user-based proxy queries.

-- Note: The user_id column should already exist from the initial schema.
-- This migration adds additional indexes for better query performance.

-- Add composite index for user_id and enabled (for filtering user's enabled proxies)
CREATE INDEX IF NOT EXISTS idx_proxies_user_enabled ON proxies(user_id, enabled);

-- Add composite index for user_id and protocol (for filtering user's proxies by protocol)
CREATE INDEX IF NOT EXISTS idx_proxies_user_protocol ON proxies(user_id, protocol);
