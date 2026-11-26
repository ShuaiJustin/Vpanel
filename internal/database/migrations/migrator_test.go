// Package migrations provides database migration management.
package migrations

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupTestDB creates a test database with prerequisite tables for multi-server management.
func setupTestDB(t *testing.T) *gorm.DB {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	prereqSQL := `
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username VARCHAR(50) NOT NULL UNIQUE,
			password VARCHAR(255) NOT NULL,
			email VARCHAR(100),
			role VARCHAR(20) DEFAULT 'user',
			enabled BOOLEAN DEFAULT 1,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS proxies (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			name VARCHAR(100) NOT NULL,
			protocol VARCHAR(20) NOT NULL,
			port INTEGER NOT NULL,
			settings TEXT,
			enabled BOOLEAN DEFAULT 1,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`
	if err := db.Exec(prereqSQL).Error; err != nil {
		t.Fatalf("Failed to create prerequisite tables: %v", err)
	}
	return db
}


func executeMigration(t *testing.T, db *gorm.DB) {
	content, err := migrationFiles.ReadFile("023_multi_server_management.sql")
	if err != nil {
		t.Fatalf("Failed to read migration file: %v", err)
	}
	statements := splitStatements(string(content))
	inDownSection := false
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		// Check if this statement contains the +migrate Down marker
		if strings.Contains(stmt, "+migrate Down") {
			inDownSection = true
			continue
		}
		if inDownSection {
			continue
		}
		// Remove comment lines from the statement to get the actual SQL
		cleanStmt := removeCommentLines(stmt)
		if cleanStmt == "" {
			continue
		}
		if err := db.Exec(cleanStmt).Error; err != nil {
			t.Fatalf("Failed to execute statement: %v\nStatement: %s", err, cleanStmt)
		}
	}
}

// removeCommentLines removes lines that start with -- from a SQL statement
func removeCommentLines(stmt string) string {
	lines := strings.Split(stmt, "\n")
	var cleanLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "--") {
			cleanLines = append(cleanLines, line)
		}
	}
	return strings.TrimSpace(strings.Join(cleanLines, "\n"))
}

func TestMultiServerManagementMigration(t *testing.T) {
	db := setupTestDB(t)
	executeMigration(t, db)

	tables := []string{"nodes", "node_groups", "node_group_members", "health_checks", "user_node_assignments", "node_traffic", "node_auth_failures"}
	for _, table := range tables {
		if !db.Migrator().HasTable(table) {
			t.Errorf("Table %s should exist after migration", table)
		}
	}

	nodeColumns := []string{"id", "name", "address", "port", "token", "status", "tags", "region", "weight", "max_users", "current_users", "latency", "last_seen_at", "sync_status", "synced_at", "ip_whitelist", "created_at", "updated_at"}
	for _, col := range nodeColumns {
		if !db.Migrator().HasColumn("nodes", col) {
			t.Errorf("Column nodes.%s should exist", col)
		}
	}

	groupColumns := []string{"id", "name", "description", "region", "strategy", "created_at", "updated_at"}
	for _, col := range groupColumns {
		if !db.Migrator().HasColumn("node_groups", col) {
			t.Errorf("Column node_groups.%s should exist", col)
		}
	}

	memberColumns := []string{"id", "node_id", "group_id", "created_at"}
	for _, col := range memberColumns {
		if !db.Migrator().HasColumn("node_group_members", col) {
			t.Errorf("Column node_group_members.%s should exist", col)
		}
	}

	healthColumns := []string{"id", "node_id", "status", "latency", "message", "tcp_ok", "api_ok", "xray_ok", "checked_at"}
	for _, col := range healthColumns {
		if !db.Migrator().HasColumn("health_checks", col) {
			t.Errorf("Column health_checks.%s should exist", col)
		}
	}

	assignmentColumns := []string{"id", "user_id", "node_id", "assigned_at", "updated_at"}
	for _, col := range assignmentColumns {
		if !db.Migrator().HasColumn("user_node_assignments", col) {
			t.Errorf("Column user_node_assignments.%s should exist", col)
		}
	}

	trafficColumns := []string{"id", "node_id", "user_id", "proxy_id", "upload", "download", "recorded_at"}
	for _, col := range trafficColumns {
		if !db.Migrator().HasColumn("node_traffic", col) {
			t.Errorf("Column node_traffic.%s should exist", col)
		}
	}

	authFailureColumns := []string{"id", "ip", "attempts", "blocked_until", "created_at", "updated_at"}
	for _, col := range authFailureColumns {
		if !db.Migrator().HasColumn("node_auth_failures", col) {
			t.Errorf("Column node_auth_failures.%s should exist", col)
		}
	}
	t.Log("Migration completed successfully - all tables and columns verified")
}


func TestMultiServerManagementTableOperations(t *testing.T) {
	db := setupTestDB(t)
	executeMigration(t, db)

	result := db.Exec(`INSERT INTO nodes (name, address, port, token, status, region, weight) VALUES ('test-node', '192.168.1.1', 18443, 'test-token-123', 'online', 'us-west', 1)`)
	if result.Error != nil {
		t.Fatalf("Failed to insert node: %v", result.Error)
	}

	var nodeCount int64
	db.Raw("SELECT COUNT(*) FROM nodes WHERE name = 'test-node'").Scan(&nodeCount)
	if nodeCount != 1 {
		t.Errorf("Expected 1 node, got %d", nodeCount)
	}

	result = db.Exec(`INSERT INTO node_groups (name, description, region, strategy) VALUES ('test-group', 'Test group description', 'us-west', 'round-robin')`)
	if result.Error != nil {
		t.Fatalf("Failed to insert node group: %v", result.Error)
	}

	result = db.Exec(`INSERT INTO node_group_members (node_id, group_id) VALUES (1, 1)`)
	if result.Error != nil {
		t.Fatalf("Failed to insert node group member: %v", result.Error)
	}

	result = db.Exec(`INSERT INTO health_checks (node_id, status, latency, message, tcp_ok, api_ok, xray_ok) VALUES (1, 'success', 50, 'All checks passed', 1, 1, 1)`)
	if result.Error != nil {
		t.Fatalf("Failed to insert health check: %v", result.Error)
	}

	result = db.Exec(`INSERT INTO node_traffic (node_id, user_id, upload, download) VALUES (1, 1, 1024, 2048)`)
	if result.Error != nil {
		t.Fatalf("Failed to insert node traffic: %v", result.Error)
	}

	result = db.Exec(`INSERT INTO node_auth_failures (ip, attempts) VALUES ('192.168.1.100', 3)`)
	if result.Error != nil {
		t.Fatalf("Failed to insert node auth failure: %v", result.Error)
	}
	t.Log("All table operations completed successfully")
}

func TestMultiServerManagementIndexes(t *testing.T) {
	db := setupTestDB(t)
	executeMigration(t, db)

	type IndexInfo struct {
		Name string
	}
	var indexes []IndexInfo
	db.Raw("SELECT name FROM sqlite_master WHERE type='index' AND tbl_name IN ('nodes', 'node_groups', 'node_group_members', 'health_checks', 'user_node_assignments', 'node_traffic', 'node_auth_failures')").Scan(&indexes)

	expectedIndexes := map[string]bool{
		"idx_nodes_status": false, "idx_nodes_region": false, "idx_node_group_member": false,
		"idx_node_group_members_group": false, "idx_health_checks_node": false, "idx_health_checks_checked_at": false,
		"idx_user_node_assignments_node": false, "idx_node_traffic_node": false, "idx_node_traffic_user": false,
		"idx_node_auth_failures_ip": false,
	}

	for _, idx := range indexes {
		if _, ok := expectedIndexes[idx.Name]; ok {
			expectedIndexes[idx.Name] = true
		}
	}

	for name, found := range expectedIndexes {
		if !found {
			t.Errorf("Expected index %s to exist", name)
		}
	}
	t.Logf("Found %d indexes on multi-server management tables", len(indexes))
}
