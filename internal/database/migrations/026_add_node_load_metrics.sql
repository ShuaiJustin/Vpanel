-- Add load metrics fields to nodes table
-- Adds CPU, memory, disk usage, network speed, and related fields

-- +migrate Up

-- Add load metrics columns
ALTER TABLE nodes ADD COLUMN cpu_usage REAL DEFAULT 0;
ALTER TABLE nodes ADD COLUMN memory_usage REAL DEFAULT 0;
ALTER TABLE nodes ADD COLUMN disk_usage REAL DEFAULT 0;
ALTER TABLE nodes ADD COLUMN net_speed INTEGER DEFAULT 0;

-- Add traffic statistics columns
ALTER TABLE nodes ADD COLUMN traffic_up INTEGER DEFAULT 0;
ALTER TABLE nodes ADD COLUMN traffic_down INTEGER DEFAULT 0;
ALTER TABLE nodes ADD COLUMN traffic_total INTEGER DEFAULT 0;
ALTER TABLE nodes ADD COLUMN traffic_limit INTEGER DEFAULT 0;
ALTER TABLE nodes ADD COLUMN traffic_reset_at TIMESTAMP;

-- Add speed limit column
ALTER TABLE nodes ADD COLUMN speed_limit INTEGER DEFAULT 0;

-- Add protocol support column
ALTER TABLE nodes ADD COLUMN protocols TEXT;

-- Add TLS configuration columns
ALTER TABLE nodes ADD COLUMN tls_enabled BOOLEAN DEFAULT 0;
ALTER TABLE nodes ADD COLUMN tls_domain VARCHAR(256);
ALTER TABLE nodes ADD COLUMN tls_cert_path VARCHAR(512);
ALTER TABLE nodes ADD COLUMN tls_key_path VARCHAR(512);

-- Add node group ID column
ALTER TABLE nodes ADD COLUMN group_id INTEGER;

-- Add priority and sort columns
ALTER TABLE nodes ADD COLUMN priority INTEGER DEFAULT 0;
ALTER TABLE nodes ADD COLUMN sort INTEGER DEFAULT 0;

-- Add alert threshold columns
ALTER TABLE nodes ADD COLUMN alert_traffic_threshold REAL DEFAULT 80;
ALTER TABLE nodes ADD COLUMN alert_cpu_threshold REAL DEFAULT 80;
ALTER TABLE nodes ADD COLUMN alert_memory_threshold REAL DEFAULT 80;

-- Add description and remarks columns
ALTER TABLE nodes ADD COLUMN description TEXT;
ALTER TABLE nodes ADD COLUMN remarks TEXT;

-- Add Xray status columns
ALTER TABLE nodes ADD COLUMN xray_running BOOLEAN DEFAULT 0;
ALTER TABLE nodes ADD COLUMN xray_version VARCHAR(64);

-- Create index for group_id
CREATE INDEX IF NOT EXISTS idx_nodes_group_id ON nodes(group_id);

-- +migrate Down

-- Drop index
DROP INDEX IF EXISTS idx_nodes_group_id;

-- Remove columns (SQLite doesn't support DROP COLUMN directly, so we would need to recreate the table)
-- For simplicity, we'll leave the columns in place during rollback
-- In production, you would need to recreate the table without these columns
