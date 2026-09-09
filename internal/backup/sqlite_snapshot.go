package backup

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	_ "github.com/glebarez/go-sqlite"
)

var sqliteSnapshotMu sync.Mutex

// CreateSQLiteSnapshot writes a transactionally consistent SQLite snapshot.
// VACUUM INTO reads through SQLite itself, so committed rows that are still in
// the WAL are included. The completed file is published atomically only after
// SQLite's integrity check succeeds.
func CreateSQLiteSnapshot(ctx context.Context, sourcePath, destinationPath string) error {
	sqliteSnapshotMu.Lock()
	defer sqliteSnapshotMu.Unlock()

	sourceAbs, err := filepath.Abs(strings.TrimSpace(sourcePath))
	if err != nil {
		return fmt.Errorf("resolve source path: %w", err)
	}
	destinationAbs, err := filepath.Abs(strings.TrimSpace(destinationPath))
	if err != nil {
		return fmt.Errorf("resolve destination path: %w", err)
	}
	if sourceAbs == destinationAbs {
		return fmt.Errorf("source and destination paths must differ")
	}
	if info, statErr := os.Stat(sourceAbs); statErr != nil {
		return fmt.Errorf("stat source database: %w", statErr)
	} else if !info.Mode().IsRegular() {
		return fmt.Errorf("source database is not a regular file")
	}

	backupDir := filepath.Dir(destinationAbs)
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return fmt.Errorf("create backup directory: %w", err)
	}
	if err := os.Chmod(backupDir, 0o700); err != nil {
		return fmt.Errorf("secure backup directory: %w", err)
	}

	tmp, err := os.CreateTemp(backupDir, ".vpanel-snapshot-*.db")
	if err != nil {
		return fmt.Errorf("create temporary snapshot: %w", err)
	}
	tmpPath := tmp.Name()
	if closeErr := tmp.Close(); closeErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temporary snapshot: %w", closeErr)
	}
	// VACUUM INTO requires a destination that does not yet exist.
	if err := os.Remove(tmpPath); err != nil {
		return fmt.Errorf("prepare temporary snapshot: %w", err)
	}
	defer os.Remove(tmpPath)

	sourceDB, err := sql.Open("sqlite", sqliteFileDSN(sourceAbs, false))
	if err != nil {
		return fmt.Errorf("open source database: %w", err)
	}
	sourceDB.SetMaxOpenConns(1)
	defer sourceDB.Close()

	if _, err := sourceDB.ExecContext(ctx, "VACUUM INTO "+quoteSQLiteString(tmpPath)); err != nil {
		return fmt.Errorf("create SQLite snapshot: %w", err)
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return fmt.Errorf("secure snapshot: %w", err)
	}
	if err := verifySQLiteSnapshot(ctx, tmpPath); err != nil {
		return err
	}

	snapshotFile, err := os.Open(tmpPath)
	if err != nil {
		return fmt.Errorf("open snapshot for sync: %w", err)
	}
	if err := snapshotFile.Sync(); err != nil {
		_ = snapshotFile.Close()
		return fmt.Errorf("sync snapshot: %w", err)
	}
	if err := snapshotFile.Close(); err != nil {
		return fmt.Errorf("close snapshot: %w", err)
	}

	if _, err := os.Stat(destinationAbs); err == nil {
		return fmt.Errorf("backup destination already exists")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat backup destination: %w", err)
	}
	if err := os.Rename(tmpPath, destinationAbs); err != nil {
		return fmt.Errorf("publish snapshot: %w", err)
	}
	dir, err := os.Open(backupDir)
	if err != nil {
		return fmt.Errorf("open backup directory for sync: %w", err)
	}
	syncErr := dir.Sync()
	closeErr := dir.Close()
	if syncErr != nil {
		return fmt.Errorf("sync backup directory: %w", syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close backup directory: %w", closeErr)
	}
	return nil
}

func verifySQLiteSnapshot(ctx context.Context, path string) error {
	db, err := sql.Open("sqlite", sqliteFileDSN(path, true))
	if err != nil {
		return fmt.Errorf("open snapshot for verification: %w", err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()

	var result string
	if err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result); err != nil {
		return fmt.Errorf("verify snapshot: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(result), "ok") {
		return fmt.Errorf("snapshot integrity check failed: %s", result)
	}
	return nil
}

func sqliteFileDSN(path string, readOnly bool) string {
	u := &url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	query := u.Query()
	query.Set("_pragma", "busy_timeout(10000)")
	if readOnly {
		query.Set("mode", "ro")
	}
	u.RawQuery = query.Encode()
	return u.String()
}

func quoteSQLiteString(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
