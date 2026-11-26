// Package migrations provides database migration management.
package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

//go:embed *.sql
var migrationFiles embed.FS

// Migration represents a database migration.
type Migration struct {
	ID        int64     `gorm:"primaryKey;autoIncrement"`
	Version   string    `gorm:"size:50;not null;uniqueIndex"`
	Name      string    `gorm:"size:255"`
	AppliedAt time.Time `gorm:"autoCreateTime"`
}

// TableName returns the table name for Migration.
func (Migration) TableName() string {
	return "migrations"
}

// Migrator handles database migrations.
type Migrator struct {
	db *gorm.DB
}

// NewMigrator creates a new migrator.
func NewMigrator(db *gorm.DB) *Migrator {
	return &Migrator{db: db}
}

// Init initializes the migrations table.
func (m *Migrator) Init(ctx context.Context) error {
	return m.db.WithContext(ctx).AutoMigrate(&Migration{})
}

// GetAppliedMigrations returns all applied migrations.
func (m *Migrator) GetAppliedMigrations(ctx context.Context) ([]Migration, error) {
	var migrations []Migration
	err := m.db.WithContext(ctx).Order("version ASC").Find(&migrations).Error
	return migrations, err
}

// GetPendingMigrations returns migrations that haven't been applied yet.
func (m *Migrator) GetPendingMigrations(ctx context.Context) ([]string, error) {
	// Get applied migrations
	applied, err := m.GetAppliedMigrations(ctx)
	if err != nil {
		return nil, err
	}

	appliedMap := make(map[string]bool)
	for _, mig := range applied {
		appliedMap[mig.Version] = true
	}

	// Get all migration files
	files, err := m.getMigrationFiles()
	if err != nil {
		return nil, err
	}

	// Find pending migrations
	var pending []string
	for _, file := range files {
		version := extractVersion(file)
		if !appliedMap[version] {
			pending = append(pending, file)
		}
	}

	return pending, nil
}

// getMigrationFiles returns all SQL migration files sorted by name.
func (m *Migrator) getMigrationFiles() ([]string, error) {
	var files []string

	err := fs.WalkDir(migrationFiles, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".sql") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(files)
	return files, nil
}

// extractVersion extracts the version from a migration filename.
// Expected format: 001_name.sql -> 001
func extractVersion(filename string) string {
	base := filepath.Base(filename)
	parts := strings.SplitN(base, "_", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return base
}

// extractName extracts the name from a migration filename.
// Expected format: 001_name.sql -> name
func extractName(filename string) string {
	base := filepath.Base(filename)
	base = strings.TrimSuffix(base, ".sql")
	parts := strings.SplitN(base, "_", 2)
	if len(parts) > 1 {
		return parts[1]
	}
	return base
}

// Migrate runs all pending migrations.
func (m *Migrator) Migrate(ctx context.Context) error {
	// Initialize migrations table
	if err := m.Init(ctx); err != nil {
		return fmt.Errorf("failed to initialize migrations table: %w", err)
	}

	// Get pending migrations
	pending, err := m.GetPendingMigrations(ctx)
	if err != nil {
		return fmt.Errorf("failed to get pending migrations: %w", err)
	}

	if len(pending) == 0 {
		return nil
	}

	// Apply each pending migration
	for _, file := range pending {
		if err := m.applyMigration(ctx, file); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", file, err)
		}
	}

	return nil
}

// applyMigration applies a single migration file.
func (m *Migrator) applyMigration(ctx context.Context, filename string) error {
	// Read migration file
	content, err := migrationFiles.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}

	version := extractVersion(filename)
	name := extractName(filename)

	// Execute migration in a transaction
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Execute SQL statements
		statements := splitStatements(string(content))
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" || strings.HasPrefix(stmt, "--") {
				continue
			}
			if err := tx.Exec(stmt).Error; err != nil {
				return fmt.Errorf("failed to execute statement: %w", err)
			}
		}

		// Record migration
		migration := Migration{
			Version:   version,
			Name:      name,
			AppliedAt: time.Now(),
		}
		if err := tx.Create(&migration).Error; err != nil {
			return fmt.Errorf("failed to record migration: %w", err)
		}

		return nil
	})
}

// splitStatements splits SQL content into individual statements.
func splitStatements(content string) []string {
	// Simple split by semicolon - handles most cases
	// For complex migrations, consider using a proper SQL parser
	var statements []string
	var current strings.Builder
	inString := false
	stringChar := byte(0)

	for i := 0; i < len(content); i++ {
		c := content[i]

		if inString {
			current.WriteByte(c)
			if c == stringChar && (i+1 >= len(content) || content[i+1] != stringChar) {
				inString = false
			}
			continue
		}

		if c == '\'' || c == '"' {
			inString = true
			stringChar = c
			current.WriteByte(c)
			continue
		}

		if c == ';' {
			stmt := strings.TrimSpace(current.String())
			if stmt != "" {
				statements = append(statements, stmt)
			}
			current.Reset()
			continue
		}

		current.WriteByte(c)
	}

	// Add any remaining content
	stmt := strings.TrimSpace(current.String())
	if stmt != "" {
		statements = append(statements, stmt)
	}

	return statements
}

// MigrateUp runs a specific number of pending migrations.
func (m *Migrator) MigrateUp(ctx context.Context, steps int) error {
	if err := m.Init(ctx); err != nil {
		return fmt.Errorf("failed to initialize migrations table: %w", err)
	}

	pending, err := m.GetPendingMigrations(ctx)
	if err != nil {
		return fmt.Errorf("failed to get pending migrations: %w", err)
	}

	if steps > len(pending) {
		steps = len(pending)
	}

	for i := 0; i < steps; i++ {
		if err := m.applyMigration(ctx, pending[i]); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", pending[i], err)
		}
	}

	return nil
}

// Version returns the current migration version.
func (m *Migrator) Version(ctx context.Context) (string, error) {
	var migration Migration
	err := m.db.WithContext(ctx).Order("version DESC").First(&migration).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", nil
		}
		return "", err
	}
	return migration.Version, nil
}

// Status returns the migration status.
type MigrationStatus struct {
	Version   string    `json:"version"`
	Name      string    `json:"name"`
	Applied   bool      `json:"applied"`
	AppliedAt time.Time `json:"applied_at,omitempty"`
}

// Status returns the status of all migrations.
func (m *Migrator) Status(ctx context.Context) ([]MigrationStatus, error) {
	if err := m.Init(ctx); err != nil {
		return nil, err
	}

	// Get applied migrations
	applied, err := m.GetAppliedMigrations(ctx)
	if err != nil {
		return nil, err
	}

	appliedMap := make(map[string]Migration)
	for _, mig := range applied {
		appliedMap[mig.Version] = mig
	}

	// Get all migration files
	files, err := m.getMigrationFiles()
	if err != nil {
		return nil, err
	}

	var statuses []MigrationStatus
	for _, file := range files {
		version := extractVersion(file)
		name := extractName(file)

		status := MigrationStatus{
			Version: version,
			Name:    name,
			Applied: false,
		}

		if mig, ok := appliedMap[version]; ok {
			status.Applied = true
			status.AppliedAt = mig.AppliedAt
		}

		statuses = append(statuses, status)
	}

	return statuses, nil
}

// Reset drops all tables and re-runs migrations.
// WARNING: This will delete all data!
func (m *Migrator) Reset(ctx context.Context) error {
	// Get underlying SQL DB
	sqlDB, err := m.db.DB()
	if err != nil {
		return err
	}

	// Get all tables
	tables, err := m.getTables(ctx, sqlDB)
	if err != nil {
		return err
	}

	// Drop all tables
	for _, table := range tables {
		if err := m.db.WithContext(ctx).Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", table)).Error; err != nil {
			return fmt.Errorf("failed to drop table %s: %w", table, err)
		}
	}

	// Re-run migrations
	return m.Migrate(ctx)
}

// getTables returns all table names in the database.
func (m *Migrator) getTables(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}

	return tables, rows.Err()
}
