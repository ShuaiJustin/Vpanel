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
	if err := db.AutoMigrate(&Proxy{}); err != nil {
		t.Fatalf("failed to migrate proxy table: %v", err)
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

func TestTrafficRepository_GetTrafficByUser_IgnoresUnknownProxyInProxyCount(t *testing.T) {
	db := setupTrafficRepositoryTestDB(t)
	repo := NewTrafficRepository(db)

	if err := db.Create(&User{
		ID:       9,
		Username: "proxy-count",
		Email:    "proxy-count@example.com",
	}).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	records := []*Traffic{
		{UserID: 9, ProxyID: 0, Upload: 100, Download: 200, RecordedAt: time.Date(2026, 3, 20, 8, 0, 0, 0, time.UTC)},
		{UserID: 9, ProxyID: 13, Upload: 300, Download: 400, RecordedAt: time.Date(2026, 3, 20, 9, 0, 0, 0, time.UTC)},
	}
	for _, record := range records {
		if err := db.Create(record).Error; err != nil {
			t.Fatalf("failed to seed traffic record: %v", err)
		}
	}

	stats, err := repo.GetTrafficByUser(
		context.Background(),
		time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 20, 23, 59, 59, 0, time.UTC),
		10,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("expected 1 user stat row, got %d", len(stats))
	}
	if stats[0].ProxyCount != 1 {
		t.Fatalf("expected proxy count 1, got %d", stats[0].ProxyCount)
	}
	if stats[0].Upload != 400 || stats[0].Download != 600 {
		t.Fatalf("unexpected totals: upload=%d download=%d", stats[0].Upload, stats[0].Download)
	}
}

func TestTrafficRepository_DeleteOlderThan(t *testing.T) {
	db := setupTrafficRepositoryTestDB(t)
	repo := NewTrafficRepository(db)

	records := []*Traffic{
		{UserID: 1, ProxyID: 1, Upload: 10, Download: 20, RecordedAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)},
		{UserID: 1, ProxyID: 1, Upload: 30, Download: 40, RecordedAt: time.Date(2026, 3, 21, 0, 0, 0, 0, time.UTC)},
	}
	for _, record := range records {
		if err := db.Create(record).Error; err != nil {
			t.Fatalf("failed to seed traffic record: %v", err)
		}
	}

	deleted, err := repo.DeleteOlderThan(context.Background(), time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 deleted row, got %d", deleted)
	}

	var remaining int64
	if err := db.Model(&Traffic{}).Count(&remaining).Error; err != nil {
		t.Fatalf("failed to count remaining rows: %v", err)
	}
	if remaining != 1 {
		t.Fatalf("expected 1 remaining row, got %d", remaining)
	}
}

func TestTrafficRepository_GetTrafficByUser_IncludesDeletedUsers(t *testing.T) {
	db := setupTrafficRepositoryTestDB(t)
	repo := NewTrafficRepository(db)

	record := &Traffic{
		UserID:     42,
		ProxyID:    1,
		Upload:     111,
		Download:   222,
		RecordedAt: time.Date(2026, 3, 21, 8, 0, 0, 0, time.UTC),
	}
	if err := db.Create(record).Error; err != nil {
		t.Fatalf("failed to seed traffic record: %v", err)
	}

	stats, err := repo.GetTrafficByUser(
		context.Background(),
		time.Date(2026, 3, 21, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 21, 23, 59, 59, 0, time.UTC),
		10,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("expected 1 user stat row, got %d", len(stats))
	}
	if stats[0].UserID != 42 {
		t.Fatalf("expected user id 42, got %d", stats[0].UserID)
	}
	if stats[0].Username != "deleted-user-42" {
		t.Fatalf("expected deleted-user-42 fallback, got %q", stats[0].Username)
	}
	if stats[0].Upload != 111 || stats[0].Download != 222 {
		t.Fatalf("unexpected totals: upload=%d download=%d", stats[0].Upload, stats[0].Download)
	}
}

func TestTrafficRepository_GetTrafficByProtocol_IncludesUnknownProxyTraffic(t *testing.T) {
	db := setupTrafficRepositoryTestDB(t)
	repo := NewTrafficRepository(db)

	records := []*Traffic{
		{UserID: 7, ProxyID: 0, Upload: 50, Download: 60, RecordedAt: time.Date(2026, 3, 21, 8, 0, 0, 0, time.UTC)},
		{UserID: 7, ProxyID: 999, Upload: 70, Download: 80, RecordedAt: time.Date(2026, 3, 21, 9, 0, 0, 0, time.UTC)},
	}
	for _, record := range records {
		if err := db.Create(record).Error; err != nil {
			t.Fatalf("failed to seed traffic record: %v", err)
		}
	}

	stats, err := repo.GetTrafficByProtocol(
		context.Background(),
		time.Date(2026, 3, 21, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 21, 23, 59, 59, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("expected 1 protocol row, got %d", len(stats))
	}
	if stats[0].Protocol != "unknown" {
		t.Fatalf("expected unknown protocol bucket, got %q", stats[0].Protocol)
	}
	if stats[0].Upload != 120 || stats[0].Download != 140 {
		t.Fatalf("unexpected totals: upload=%d download=%d", stats[0].Upload, stats[0].Download)
	}
	if stats[0].Count != 0 {
		t.Fatalf("expected unknown bucket count 0, got %d", stats[0].Count)
	}
}
