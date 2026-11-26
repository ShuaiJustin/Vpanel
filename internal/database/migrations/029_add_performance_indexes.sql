-- Migration: Add performance indexes for search and query optimization
-- Version: 029
-- Requirements: 1.4, 1.5, 2.4, 2.5, 3.15
-- Related to: response-loading-performance-check bugfix spec

-- +migrate Up
-- Users table search indexes
-- These indexes support LIKE queries on username and email fields
-- Removing LOWER/TRIM from WHERE clauses allows these indexes to be used
CREATE INDEX IF NOT EXISTS idx_users_username_search ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_email_search ON users(email);

-- Node group members composite index
-- Supports efficient JOIN queries when filtering nodes by group_id
CREATE INDEX IF NOT EXISTS idx_node_group_members_lookup ON node_group_members(group_id, node_id);

-- Traffic table composite index
-- Supports efficient queries for user traffic statistics
CREATE INDEX IF NOT EXISTS idx_traffic_user_created ON traffic(user_id, created_at);

-- +migrate Down
-- Drop performance indexes
DROP INDEX IF EXISTS idx_users_username_search;
DROP INDEX IF EXISTS idx_users_email_search;
DROP INDEX IF EXISTS idx_node_group_members_lookup;
DROP INDEX IF EXISTS idx_traffic_user_created;
