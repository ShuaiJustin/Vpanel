package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupTrafficRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	originalLocal := time.Local
	t.Cleanup(func() {
		time.Local = originalLocal
	})
	t.Setenv("TZ", "UTC")
	time.Local = time.UTC
	_ = os.Setenv("TZ", "UTC")

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	if err := db.AutoMigrate(&Traffic{}); err != nil {
		t.Fatalf("failed to migrate traffic table: %v", err)
	}
	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatalf("failed to migrate user table: %v", err)
	}

	return db
}

func seedTrafficRecords(t *testing.T, db *gorm.DB) {
	t.Helper()

	if err := db.Create(&User{
		ID:       7,
		Username: "tester",
		Email:    "tester@example.com",
	}).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	records := []*Traffic{
		{UserID: 7, ProxyID: 11, Upload: 100, Download: 200, RecordedAt: time.Date(2026, 3, 18, 8, 15, 0, 0, time.UTC)},
		{UserID: 7, ProxyID: 11, Upload: 300, Download: 400, RecordedAt: time.Date(2026, 3, 18, 18, 30, 0, 0, time.UTC)},
		{UserID: 7, ProxyID: 12, Upload: 500, Download: 600, RecordedAt: time.Date(2026, 3, 19, 9, 0, 0, 0, time.UTC)},
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

func TestTrafficRepository_GetTrafficByUser_SQLite(t *testing.T) {
	db := setupTrafficRepositoryTestDB(t)
	seedTrafficRecords(t, db)
	repo := NewTrafficRepository(db)

	stats, err := repo.GetTrafficByUser(
		context.Background(),
		time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 19, 23, 59, 59, 0, time.UTC),
		10,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(stats) != 1 {
		t.Fatalf("expected 1 user stat row, got %d", len(stats))
	}

	if stats[0].UserID != 7 {
		t.Fatalf("expected user id 7, got %d", stats[0].UserID)
	}

	if stats[0].Upload != 900 || stats[0].Download != 1200 {
		t.Fatalf("unexpected totals: upload=%d download=%d", stats[0].Upload, stats[0].Download)
	}

	if stats[0].ProxyCount != 2 {
		t.Fatalf("expected proxy count 2, got %d", stats[0].ProxyCount)
	}

	if stats[0].LastActive == nil || stats[0].LastActive.IsZero() {
		t.Fatal("expected last active timestamp to be populated")
	}
}
