-- Performance optimization indexes for Vpanel
-- Created: 2026-04-17
-- Purpose: Fix 2294ms latency issue and improve query performance

-- Index for node name lookups (used in duplicate checks and searches)
CREATE INDEX IF NOT EXISTS idx_nodes_name_lower ON nodes(LOWER(name));

-- Composite index for address+port duplicate checks
CREATE INDEX IF NOT EXISTS idx_nodes_address_port ON nodes(LOWER(address), port);

-- Composite index for node_traffic queries by node and time range
CREATE INDEX IF NOT EXISTS idx_node_traffic_node_recorded ON node_traffic(node_id, recorded_at DESC);

-- Composite index for node_traffic queries by user and time range
CREATE INDEX IF NOT EXISTS idx_node_traffic_user_recorded ON node_traffic(user_id, recorded_at DESC);

-- Index for time-range queries on node_traffic
CREATE INDEX IF NOT EXISTS idx_node_traffic_recorded ON node_traffic(recorded_at DESC);

-- Index for node group membership queries
CREATE INDEX IF NOT EXISTS idx_node_group_members_node ON node_group_members(node_id);
CREATE INDEX IF NOT EXISTS idx_node_group_members_group ON node_group_members(group_id);

-- Composite index for efficient group traffic aggregation
CREATE INDEX IF NOT EXISTS idx_node_traffic_node_time ON node_traffic(node_id, recorded_at DESC, upload, download);

-- Index for user assignment queries
CREATE INDEX IF NOT EXISTS idx_user_node_assignments_node ON user_node_assignments(node_id);
CREATE INDEX IF NOT EXISTS idx_user_node_assignments_user ON user_node_assignments(user_id);

-- Index for proxy queries by node
CREATE INDEX IF NOT EXISTS idx_proxies_node ON proxies(node_id) WHERE node_id IS NOT NULL;

-- Index for certificate queries
CREATE INDEX IF NOT EXISTS idx_nodes_certificate ON nodes(certificate_id) WHERE certificate_id IS NOT NULL;

-- Analyze tables to update statistics
ANALYZE nodes;
ANALYZE node_traffic;
ANALYZE node_group_members;
ANALYZE user_node_assignments;
ANALYZE proxies;
