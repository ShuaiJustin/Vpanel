// Package migrator copies all GORM-managed table rows from a source database
// to a target database. It's used by the admin "switch database" workflow.
//
// Important caveats:
//   - The migrator runs AutoMigrate on the target before copying, so the
//     target schema matches what the running binary expects.
//   - IDs are preserved (we insert rows with their existing primary key) so
//     foreign keys remain intact. On PostgreSQL the sequence does NOT auto-
//     advance to match these IDs — we call setval() per table afterwards.
//   - Migration is one-way and not transactional across tables: if it fails
//     mid-way the target DB is left in a partial state. Callers should warn
//     the user and recommend they take a backup first.
//   - Cutover is manual: after a successful migration the operator must
//     update their config (V_DB_DRIVER / V_DB_DSN) and restart
//     the process to start using the target DB. The migrator does NOT
//     switch the running process.
package migrator

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// TableResult is the per-table outcome of a migration.
type TableResult struct {
	Table   string `json:"table"`
	Rows    int64  `json:"rows"`
	Skipped bool   `json:"skipped"`
	Error   string `json:"error,omitempty"`
}

// Report summarizes a migration run.
type Report struct {
	Tables     []TableResult `json:"tables"`
	TotalRows  int64         `json:"total_rows"`
	TableCount int           `json:"table_count"`
}

// Migrate copies every model in `models` from src to tgt. The target schema is
// (re-)applied via AutoMigrate before copying. Returns a per-table report.
// On the first hard failure that prevents proceeding (target migrate fails),
// returns an error. Per-table copy errors are recorded in the report but do
// not abort the rest of the migration.
func Migrate(ctx context.Context, src, tgt *gorm.DB, models []any, batchSize int) (*Report, error) {
	if src == nil || tgt == nil {
		return nil, fmt.Errorf("source and target db must be non-nil")
	}
	if batchSize <= 0 {
		batchSize = 500
	}

	// Apply schema on the target. Without this, INSERTs fail because tables
	// don't exist yet on a fresh target DB.
	if err := tgt.AutoMigrate(models...); err != nil {
		return nil, fmt.Errorf("auto-migrate target: %w", err)
	}

	report := &Report{Tables: make([]TableResult, 0, len(models))}

	for _, model := range models {
		stmt := &gorm.Statement{DB: tgt}
		if err := stmt.Parse(model); err != nil {
			report.Tables = append(report.Tables, TableResult{
				Table: fmt.Sprintf("%T", model),
				Error: fmt.Sprintf("parse model: %v", err),
			})
			continue
		}
		table := stmt.Schema.Table

		result := copyTable(ctx, src, tgt, table, batchSize)
		report.Tables = append(report.Tables, result)
		report.TotalRows += result.Rows
		report.TableCount++
	}

	// PostgreSQL's serial sequences don't advance when we insert explicit IDs,
	// so the next default-value INSERT after migration would collide. Fix the
	// sequences. SQLite and MySQL auto-increment automatically track max(id).
	if tgt.Dialector.Name() == "postgres" {
		fixPostgresSequences(ctx, tgt, models, report)
	}

	return report, nil
}

func copyTable(ctx context.Context, src, tgt *gorm.DB, table string, batchSize int) TableResult {
	res := TableResult{Table: table}

	// Pull rows as raw maps to side-step typed struct constraints (and avoid
	// having to know every concrete model type here).
	var total int64
	if err := src.WithContext(ctx).Table(table).Count(&total).Error; err != nil {
		res.Error = fmt.Sprintf("count source: %v", err)
		return res
	}
	if total == 0 {
		res.Skipped = true
		return res
	}

	// Truncate the target table first so retries are idempotent. If the target
	// table has no rows yet (fresh migrate) this is a no-op.
	if err := tgt.WithContext(ctx).Exec("DELETE FROM " + quoteIdent(tgt, table)).Error; err != nil {
		res.Error = fmt.Sprintf("clear target: %v", err)
		return res
	}

	for offset := int64(0); offset < total; offset += int64(batchSize) {
		var batch []map[string]any
		if err := src.WithContext(ctx).
			Table(table).
			Limit(batchSize).
			Offset(int(offset)).
			Find(&batch).Error; err != nil {
			res.Error = fmt.Sprintf("read batch at offset %d: %v", offset, err)
			return res
		}
		if len(batch) == 0 {
			break
		}

		if err := tgt.WithContext(ctx).Table(table).Create(&batch).Error; err != nil {
			res.Error = fmt.Sprintf("insert batch at offset %d: %v", offset, err)
			return res
		}
		res.Rows += int64(len(batch))
	}
	return res
}

// quoteIdent returns a backend-appropriate quoted identifier. SQLite and
// PostgreSQL accept double quotes; MySQL prefers backticks.
func quoteIdent(db *gorm.DB, name string) string {
	if db.Dialector.Name() == "mysql" {
		return "`" + strings.ReplaceAll(name, "`", "``") + "`"
	}
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// fixPostgresSequences calls setval() on every <table>_id_seq so subsequent
// default-value INSERTs don't collide with the explicit IDs we copied over.
// Tables without a serial id column are silently skipped (the query errors
// and we ignore it).
func fixPostgresSequences(ctx context.Context, tgt *gorm.DB, models []any, report *Report) {
	for i := range report.Tables {
		tr := &report.Tables[i]
		if tr.Skipped || tr.Error != "" {
			continue
		}
		seq := tr.Table + "_id_seq"
		// COALESCE handles empty tables; false on the third arg means "next
		// nextval() returns the value you passed, not value+1".
		_ = tgt.WithContext(ctx).Exec(fmt.Sprintf(
			"SELECT setval(%s, COALESCE((SELECT MAX(id) FROM %s), 1), true)",
			"'"+seq+"'", quoteIdent(tgt, tr.Table),
		)).Error
	}
}
