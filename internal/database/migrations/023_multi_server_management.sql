-- Multi-Server Management System Migration
-- Creates tables for node management, health checks, load balancing, and traffic statistics

-- +migrate Up

-- Nodes table: stores remote Xray node server information
CREATE TABLE IF NOT EXISTS nodes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(128) NOT NULL,
    address VARCHAR(256) NOT NULL,
    port INTEGER DEFAULT 18443,
    token VARCHAR(64) UNIQUE,
    status VARCHAR(32) DEFAULT 'offline',
    tags TEXT,
    region VARCHAR(64),
    weight INTEGER DEFAULT 1,
    max_users INTEGER DEFAULT 0,
    current_users INTEGER DEFAULT 0,
    latency INTEGER DEFAULT 0,
    last_seen_at TIMESTAMP,
    sync_status VARCHAR(32) DEFAULT 'pending',
    synced_at TIMESTAMP,
    ip_whitelist TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for nodes table
CREATE INDEX IF NOT EXISTS idx_nodes_status ON nodes(status);
CREATE INDEX IF NOT EXISTS idx_nodes_region ON nodes(region);

-- Node groups table: organizes nodes by region or purpose
CREATE TABLE IF NOT EXISTS node_groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(64) NOT NULL,
    description VARCHAR(256),
    region VARCHAR(64),
    strategy VARCHAR(32) DEFAULT 'round-robin',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Node group members table: many-to-many relationship between nodes and groups
CREATE TABLE IF NOT EXISTS node_group_members (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id INTEGER NOT NULL,
    group_id INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE,
    FOREIGN KEY (group_id) REFERENCES node_groups(id) ON DELETE CASCADE
);

-- Unique index to prevent duplicate node-group assignments
CREATE UNIQUE INDEX IF NOT EXISTS idx_node_group_member ON node_group_members(node_id, group_id);
CREATE INDEX IF NOT EXISTS idx_node_group_members_group ON node_group_members(group_id);

-- Health checks table: stores health check history for nodes
CREATE TABLE IF NOT EXISTS health_checks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id INTEGER NOT NULL,
    status VARCHAR(32),
    latency INTEGER,
    message VARCHAR(512),
    tcp_ok BOOLEAN DEFAULT 0,
    api_ok BOOLEAN DEFAULT 0,
    xray_ok BOOLEAN DEFAULT 0,
    checked_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE
);

-- Indexes for health checks
CREATE INDEX IF NOT EXISTS idx_health_checks_node ON health_checks(node_id);
CREATE INDEX IF NOT EXISTS idx_health_checks_checked_at ON health_checks(checked_at);
CREATE INDEX IF NOT EXISTS idx_health_checks_node_checked ON health_checks(node_id, checked_at);

-- User node assignments table: tracks which node a user is assigned to
CREATE TABLE IF NOT EXISTS user_node_assignments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL UNIQUE,
    node_id INTEGER NOT NULL,
    assigned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE SET NULL
);

-- Index for user node assignments
CREATE INDEX IF NOT EXISTS idx_user_node_assignments_node ON user_node_assignments(node_id);

-- Node traffic table: stores per-node traffic statistics
CREATE TABLE IF NOT EXISTS node_traffic (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    proxy_id INTEGER,
    upload INTEGER DEFAULT 0,
    download INTEGER DEFAULT 0,
    recorded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for node traffic
CREATE INDEX IF NOT EXISTS idx_node_traffic_node ON node_traffic(node_id);
CREATE INDEX IF NOT EXISTS idx_node_traffic_user ON node_traffic(user_id);
CREATE INDEX IF NOT EXISTS idx_node_traffic_proxy ON node_traffic(proxy_id);
CREATE INDEX IF NOT EXISTS idx_node_traffic_recorded ON node_traffic(recorded_at);
CREATE INDEX IF NOT EXISTS idx_node_traffic_node_recorded ON node_traffic(node_id, recorded_at);
CREATE INDEX IF NOT EXISTS idx_node_traffic_user_recorded ON node_traffic(user_id, recorded_at);

-- Node auth failures table: tracks authentication failures for rate limiting
CREATE TABLE IF NOT EXISTS node_auth_failures (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ip VARCHAR(45) NOT NULL,
    attempts INTEGER DEFAULT 1,
    blocked_until TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Index for node auth failures
CREATE INDEX IF NOT EXISTS idx_node_auth_failures_ip ON node_auth_failures(ip);

-- +migrate Down

DROP INDEX IF EXISTS idx_node_auth_failures_ip;
DROP TABLE IF EXISTS node_auth_failures;

DROP INDEX IF EXISTS idx_node_traffic_user_recorded;
DROP INDEX IF EXISTS idx_node_traffic_node_recorded;
DROP INDEX IF EXISTS idx_node_traffic_recorded;
DROP INDEX IF EXISTS idx_node_traffic_proxy;
DROP INDEX IF EXISTS idx_node_traffic_user;
DROP INDEX IF EXISTS idx_node_traffic_node;
DROP TABLE IF EXISTS node_traffic;

DROP INDEX IF EXISTS idx_user_node_assignments_node;
DROP TABLE IF EXISTS user_node_assignments;

DROP INDEX IF EXISTS idx_health_checks_node_checked;
DROP INDEX IF EXISTS idx_health_checks_checked_at;
DROP INDEX IF EXISTS idx_health_checks_node;
DROP TABLE IF EXISTS health_checks;

DROP INDEX IF EXISTS idx_node_group_members_group;
DROP INDEX IF EXISTS idx_node_group_member;
DROP TABLE IF EXISTS node_group_members;

DROP TABLE IF EXISTS node_groups;

DROP INDEX IF EXISTS idx_nodes_region;
DROP INDEX IF EXISTS idx_nodes_status;
DROP TABLE IF EXISTS nodes;
