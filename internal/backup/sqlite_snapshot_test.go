package backup

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateSQLiteSnapshotIncludesCommittedWALRows(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.db")
	destination := filepath.Join(dir, "backups", "snapshot.db")

	db, err := sql.Open("sqlite", sqliteFileDSN(source, false))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; CREATE TABLE entries (id INTEGER PRIMARY KEY, value TEXT);`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO entries(value) VALUES ('committed-in-wal')`); err != nil {
		t.Fatal(err)
	}

	if err := CreateSQLiteSnapshot(context.Background(), source, destination); err != nil {
		t.Fatal(err)
	}

	backupDB, err := sql.Open("sqlite", sqliteFileDSN(destination, true))
	if err != nil {
		t.Fatal(err)
	}
	defer backupDB.Close()
	var value string
	if err := backupDB.QueryRow(`SELECT value FROM entries WHERE id = 1`).Scan(&value); err != nil {
		t.Fatal(err)
	}
	if value != "committed-in-wal" {
		t.Fatalf("unexpected snapshot row %q", value)
	}
	info, err := os.Stat(destination)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("snapshot permissions = %o, want 600", got)
	}
}

func TestCreateSQLiteSnapshotDoesNotOverwriteDestination(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.db")
	destination := filepath.Join(dir, "snapshot.db")

	db, err := sql.Open("sqlite", sqliteFileDSN(source, false))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE entries (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := CreateSQLiteSnapshot(context.Background(), source, destination); err == nil {
		t.Fatal("expected an existing-destination error")
	}
	contents, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "keep" {
		t.Fatalf("destination was overwritten: %q", contents)
	}
}
