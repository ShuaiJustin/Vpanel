package repository

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupTrafficRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	if err := db.AutoMigrate(&Traffic{}); err != nil {
		t.Fatalf("failed to migrate traffic table: %v", err)
	}

	return db
}

func seedTrafficRecords(t *testing.T, db *gorm.DB) {
	t.Helper()

	records := []*Traffic{
		{UserID: 7, Upload: 100, Download: 200, RecordedAt: time.Date(2026, 3, 18, 8, 15, 0, 0, time.UTC)},
		{UserID: 7, Upload: 300, Download: 400, RecordedAt: time.Date(2026, 3, 18, 18, 30, 0, 0, time.UTC)},
		{UserID: 7, Upload: 500, Download: 600, RecordedAt: time.Date(2026, 3, 19, 9, 0, 0, 0, time.UTC)},
	}

	for _, record := range records {
		if err := db.Create(record).Error; err != nil {
			t.Fatalf("failed to seed traffic record: %v", err)
		}
	}
}

func TestTrafficRepository_GetTrafficTimeline_SQLite(t *testing.T) {
	db := setupTrafficRepositoryTestDB(t)
	seedTrafficRecords(t, db)
	repo := NewTrafficRepository(db)

	points, err := repo.GetTrafficTimeline(context.Background(),
		time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 19, 23, 59, 59, 0, time.UTC),
		"day",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(points) != 2 {
		t.Fatalf("expected 2 timeline points, got %d", len(points))
	}

	if points[0].Upload != 400 || points[0].Download != 600 {
		t.Fatalf("unexpected first point totals: upload=%d download=%d", points[0].Upload, points[0].Download)
	}

	if points[1].Upload != 500 || points[1].Download != 600 {
		t.Fatalf("unexpected second point totals: upload=%d download=%d", points[1].Upload, points[1].Download)
	}
}

func TestTrafficRepository_GetTrafficTimelineByUser_SQLite(t *testing.T) {
	db := setupTrafficRepositoryTestDB(t)
	seedTrafficRecords(t, db)
	repo := NewTrafficRepository(db)

	points, err := repo.GetTrafficTimelineByUser(context.Background(),
		7,
		time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 19, 23, 59, 59, 0, time.UTC),
		"day",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(points) != 2 {
		t.Fatalf("expected 2 timeline points, got %d", len(points))
	}

	if points[0].Upload != 400 || points[0].Download != 600 {
		t.Fatalf("unexpected first point totals: upload=%d download=%d", points[0].Upload, points[0].Download)
	}

	if points[1].Upload != 500 || points[1].Download != 600 {
		t.Fatalf("unexpected second point totals: upload=%d download=%d", points[1].Upload, points[1].Download)
	}
}
