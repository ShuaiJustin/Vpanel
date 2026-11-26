package ip

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type selectQueryCounter struct {
	gormlogger.Interface
	selects int
}

func newSelectQueryCounter() *selectQueryCounter {
	return &selectQueryCounter{
		Interface: gormlogger.Default.LogMode(gormlogger.Silent),
	}
}

func (l *selectQueryCounter) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	l.Interface = l.Interface.LogMode(level)
	return l
}

func (l *selectQueryCounter) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	sql, rows := fc()
	if len(sql) >= len("SELECT") && sql[:len("SELECT")] == "SELECT" {
		l.selects++
	}
	l.Interface.Trace(ctx, begin, func() (string, int64) {
		return sql, rows
	}, err)
}

func TestGetAggregatedIPHistorySQLite(t *testing.T) {
	queryCounter := newSelectQueryCounter()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: queryCounter,
	})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if err := db.AutoMigrate(&IPHistory{}); err != nil {
		t.Fatalf("migrate ip history: %v", err)
	}

	loc := time.FixedZone("CST", 8*60*60)
	firstSeen := time.Date(2026, 3, 22, 21, 0, 0, 123456789, loc)
	secondSeen := firstSeen.Add(5 * time.Minute)
	thirdSeen := secondSeen.Add(10 * time.Minute)

	records := []*IPHistory{
		{
			UserID:     7,
			IP:         "1.1.1.1",
			UserAgent:  "ua-1",
			AccessType: AccessTypeProxy,
			Country:    "Japan",
			City:       "Tokyo",
			CreatedAt:  firstSeen,
		},
		{
			UserID:     7,
			IP:         "1.1.1.1",
			UserAgent:  "ua-2",
			AccessType: AccessTypeProxy,
			Country:    "Japan",
			City:       "Osaka",
			CreatedAt:  secondSeen,
		},
		{
			UserID:     7,
			IP:         "2.2.2.2",
			UserAgent:  "ua-3",
			AccessType: AccessTypeSubscription,
			Country:    "United States",
			City:       "San Jose",
			CreatedAt:  thirdSeen,
		},
	}

	for _, record := range records {
		if err := db.Create(record).Error; err != nil {
			t.Fatalf("create ip history record: %v", err)
		}
	}

	tracker := NewTracker(db)
	queryCounter.selects = 0
	summaries, total, err := tracker.GetAggregatedIPHistory(context.Background(), 7, 10, 0)
	if err != nil {
		t.Fatalf("get aggregated ip history: %v", err)
	}

	if total != 2 {
		t.Fatalf("unexpected distinct total: got %d want 2", total)
	}
	if len(summaries) != 2 {
		t.Fatalf("unexpected summary length: got %d want 2", len(summaries))
	}

	if summaries[0].IP != "2.2.2.2" {
		t.Fatalf("unexpected first summary ip: got %s want 2.2.2.2", summaries[0].IP)
	}
	if summaries[0].AccessCount != 1 {
		t.Fatalf("unexpected first summary access count: got %d want 1", summaries[0].AccessCount)
	}
	if !summaries[0].FirstSeen.UTC().Equal(thirdSeen.UTC()) || !summaries[0].LastSeen.UTC().Equal(thirdSeen.UTC()) {
		t.Fatalf("unexpected first summary timestamps: got first=%s last=%s want=%s", summaries[0].FirstSeen.UTC(), summaries[0].LastSeen.UTC(), thirdSeen.UTC())
	}
	if summaries[0].Country != "United States" || summaries[0].City != "San Jose" {
		t.Fatalf("unexpected latest location for second ip: got %s/%s", summaries[0].Country, summaries[0].City)
	}

	if summaries[1].IP != "1.1.1.1" {
		t.Fatalf("unexpected second summary ip: got %s want 1.1.1.1", summaries[1].IP)
	}
	if summaries[1].AccessCount != 2 {
		t.Fatalf("unexpected second summary access count: got %d want 2", summaries[1].AccessCount)
	}
	if !summaries[1].FirstSeen.UTC().Equal(firstSeen.UTC()) || !summaries[1].LastSeen.UTC().Equal(secondSeen.UTC()) {
		t.Fatalf("unexpected second summary timestamps: got first=%s last=%s want first=%s last=%s", summaries[1].FirstSeen.UTC(), summaries[1].LastSeen.UTC(), firstSeen.UTC(), secondSeen.UTC())
	}
	if summaries[1].Country != "Japan" || summaries[1].City != "Osaka" {
		t.Fatalf("unexpected latest location for first ip: got %s/%s", summaries[1].Country, summaries[1].City)
	}
	if queryCounter.selects != 3 {
		t.Fatalf("expected fixed select count for aggregated history, got %d", queryCounter.selects)
	}
}

func TestGetAggregatedIPHistorySQLiteQueryCountStaysFixedAcrossMoreIPs(t *testing.T) {
	queryCounter := newSelectQueryCounter()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: queryCounter,
	})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if err := db.AutoMigrate(&IPHistory{}); err != nil {
		t.Fatalf("migrate ip history: %v", err)
	}

	baseTime := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 12; i++ {
		recordTime := baseTime.Add(time.Duration(i) * time.Minute)
		ipAddr := fmt.Sprintf("10.0.0.%d", i+1)
		for version := 0; version < 2; version++ {
			if err := db.Create(&IPHistory{
				UserID:     18,
				IP:         ipAddr,
				UserAgent:  fmt.Sprintf("ua-%d-%d", i, version),
				AccessType: AccessTypeProxy,
				Country:    "Country",
				City:       fmt.Sprintf("City-%d", version),
				CreatedAt:  recordTime.Add(time.Duration(version) * time.Second),
			}).Error; err != nil {
				t.Fatalf("create ip history record: %v", err)
			}
		}
	}

	tracker := NewTracker(db)
	queryCounter.selects = 0
	if _, _, err := tracker.GetAggregatedIPHistory(context.Background(), 18, 10, 0); err != nil {
		t.Fatalf("get aggregated ip history: %v", err)
	}

	if queryCounter.selects != 3 {
		t.Fatalf("expected fixed select count for paged aggregated history, got %d", queryCounter.selects)
	}
}
