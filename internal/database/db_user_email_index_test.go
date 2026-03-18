package database

import "testing"

func TestAutoMigrate_EnsuresUniqueUserEmailIndex(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Driver = "sqlite"
	cfg.DSN = "file::memory:?cache=shared"

	db, err := New(&cfg)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	if err := db.AutoMigrate(); err != nil {
		t.Fatalf("failed to auto migrate database: %v", err)
	}

	type indexRow struct {
		Name   string `gorm:"column:name"`
		Unique int    `gorm:"column:unique"`
	}

	indexes := make([]indexRow, 0)
	if err := db.DB().Raw(`PRAGMA index_list('users')`).Scan(&indexes).Error; err != nil {
		t.Fatalf("failed to inspect user indexes: %v", err)
	}

	hasUniqueEmailIndex := false
	for _, index := range indexes {
		if index.Name == "idx_users_email" {
			t.Fatalf("expected legacy idx_users_email to be removed")
		}
		if index.Name == "idx_users_email_unique" && index.Unique == 1 {
			hasUniqueEmailIndex = true
		}
	}

	if !hasUniqueEmailIndex {
		t.Fatalf("expected idx_users_email_unique to exist and be unique")
	}
}
