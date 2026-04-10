package ip

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type countingRoundTripper struct {
	requests atomic.Int32
	delay    time.Duration
}

func (t *countingRoundTripper) RoundTrip(_ *http.Request) (*http.Response, error) {
	t.requests.Add(1)
	if t.delay > 0 {
		time.Sleep(t.delay)
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(
			`{"success":true,"country":"China","country_code":"CN","region":"Shanghai","city":"Shanghai"}`,
		)),
	}, nil
}

func setupIPServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if err := db.AutoMigrate(&ActiveIP{}, &IPHistory{}, &GeoCache{}); err != nil {
		t.Fatalf("migrate tables: %v", err)
	}

	return db
}

func newTestIPService(t *testing.T, db *gorm.DB) *Service {
	t.Helper()

	service, err := NewService(db, &ServiceConfig{
		GeoConfig: &GeolocationConfig{
			DatabasePath: "",
			CacheTTL:     24 * time.Hour,
		},
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	return service
}

func waitFor(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("condition not satisfied within %s", timeout)
}

func TestServiceUserReadPathsEnrichResponseWithoutPersistingGeo(t *testing.T) {
	db := setupIPServiceTestDB(t)
	service := newTestIPService(t, db)

	now := time.Now()
	const userID uint = 9
	const testIP = "124.79.151.251"

	if err := db.Create(&GeoCache{
		IP:          testIP,
		Country:     "China",
		CountryCode: "CN",
		Region:      "Shanghai",
		City:        "Shanghai",
		CachedAt:    now,
	}).Error; err != nil {
		t.Fatalf("seed geo cache: %v", err)
	}

	if err := db.Create(&ActiveIP{
		UserID:     userID,
		IP:         testIP,
		UserAgent:  "ua-device",
		DeviceType: "desktop",
		LastActive: now,
		CreatedAt:  now,
	}).Error; err != nil {
		t.Fatalf("seed active ip: %v", err)
	}

	if err := db.Create(&IPHistory{
		UserID:     userID,
		IP:         testIP,
		UserAgent:  "ua-history",
		AccessType: AccessTypeProxy,
		CreatedAt:  now,
	}).Error; err != nil {
		t.Fatalf("seed ip history: %v", err)
	}

	onlineIPs, err := service.GetOnlineIPs(context.Background(), userID)
	if err != nil {
		t.Fatalf("get online ips: %v", err)
	}
	if len(onlineIPs) != 1 {
		t.Fatalf("unexpected online ip count: got %d want 1", len(onlineIPs))
	}
	if onlineIPs[0].Country != "China" || onlineIPs[0].City != "Shanghai" || onlineIPs[0].CountryCode != "CN" {
		t.Fatalf("unexpected enriched online ip: %+v", onlineIPs[0])
	}

	summaries, total, err := service.GetAggregatedIPHistory(context.Background(), userID, 10, 0)
	if err != nil {
		t.Fatalf("get aggregated history: %v", err)
	}
	if total != 1 || len(summaries) != 1 {
		t.Fatalf("unexpected history result: total=%d len=%d", total, len(summaries))
	}
	if summaries[0].Country != "China" || summaries[0].City != "Shanghai" || summaries[0].CountryCode != "CN" {
		t.Fatalf("unexpected enriched history summary: %+v", summaries[0])
	}

	var activeIP ActiveIP
	if err := db.Where("user_id = ? AND ip = ?", userID, testIP).Take(&activeIP).Error; err != nil {
		t.Fatalf("reload active ip: %v", err)
	}
	if activeIP.Country != "" || activeIP.City != "" {
		t.Fatalf("expected read path to avoid persisting active ip geo, got %+v", activeIP)
	}

	var historyRecord IPHistory
	if err := db.Where("user_id = ? AND ip = ?", userID, testIP).Take(&historyRecord).Error; err != nil {
		t.Fatalf("reload history: %v", err)
	}
	if historyRecord.Country != "" || historyRecord.City != "" {
		t.Fatalf("expected read path to avoid persisting history geo, got %+v", historyRecord)
	}
}

func TestServiceUserReadPathsSkipExternalGeoOnColdCache(t *testing.T) {
	db := setupIPServiceTestDB(t)
	service := newTestIPService(t, db)

	transport := &countingRoundTripper{}
	service.geoService.httpClient = &http.Client{
		Timeout:   time.Second,
		Transport: transport,
	}

	now := time.Now()
	const userID uint = 10
	const testIP = "124.79.151.251"

	if err := db.Create(&ActiveIP{
		UserID:     userID,
		IP:         testIP,
		UserAgent:  "ua-device",
		DeviceType: "desktop",
		LastActive: now,
		CreatedAt:  now,
	}).Error; err != nil {
		t.Fatalf("seed active ip: %v", err)
	}

	if err := db.Create(&IPHistory{
		UserID:     userID,
		IP:         testIP,
		UserAgent:  "ua-history",
		AccessType: AccessTypeProxy,
		CreatedAt:  now,
	}).Error; err != nil {
		t.Fatalf("seed ip history: %v", err)
	}

	onlineIPs, err := service.GetOnlineIPs(context.Background(), userID)
	if err != nil {
		t.Fatalf("get online ips: %v", err)
	}
	if len(onlineIPs) != 1 {
		t.Fatalf("unexpected online ip count: got %d want 1", len(onlineIPs))
	}

	summaries, total, err := service.GetAggregatedIPHistory(context.Background(), userID, 10, 0)
	if err != nil {
		t.Fatalf("get aggregated history: %v", err)
	}
	if total != 1 || len(summaries) != 1 {
		t.Fatalf("unexpected history result: total=%d len=%d", total, len(summaries))
	}

	if got := transport.requests.Load(); got != 0 {
		t.Fatalf("expected read paths to skip external geo lookup, got %d requests", got)
	}
}

func TestServiceGetOnlineIPsDoesNotCleanupActiveRowsOnReadPath(t *testing.T) {
	db := setupIPServiceTestDB(t)
	service := newTestIPService(t, db)

	now := time.Now()
	if err := db.Create(&ActiveIP{
		UserID:     12,
		IP:         "198.51.100.1",
		UserAgent:  "ua-current",
		DeviceType: "desktop",
		LastActive: now,
		CreatedAt:  now,
	}).Error; err != nil {
		t.Fatalf("seed active ip: %v", err)
	}
	if err := db.Create(&ActiveIP{
		UserID:     12,
		IP:         "198.51.100.2",
		UserAgent:  "ua-stale",
		DeviceType: "desktop",
		LastActive: now.Add(-24 * time.Hour),
		CreatedAt:  now.Add(-24 * time.Hour),
	}).Error; err != nil {
		t.Fatalf("seed stale active ip: %v", err)
	}

	onlineIPs, err := service.GetOnlineIPs(context.Background(), 12)
	if err != nil {
		t.Fatalf("get online ips: %v", err)
	}
	if len(onlineIPs) != 2 {
		t.Fatalf("expected read path to avoid cleanup side effects, got %d rows", len(onlineIPs))
	}

	var activeIPCount int64
	if err := db.Model(&ActiveIP{}).Where("user_id = ?", 12).Count(&activeIPCount).Error; err != nil {
		t.Fatalf("count active ips: %v", err)
	}
	if activeIPCount != 2 {
		t.Fatalf("expected read path to keep active rows untouched, got %d", activeIPCount)
	}
}

func TestServiceRecordActivityWarmsGeoCacheAsyncWithoutBlockingResponse(t *testing.T) {
	db := setupIPServiceTestDB(t)
	service := newTestIPService(t, db)

	transport := &countingRoundTripper{
		delay: 200 * time.Millisecond,
	}
	service.geoService.httpClient = &http.Client{
		Timeout:   time.Second,
		Transport: transport,
	}

	start := time.Now()
	if err := service.RecordActivity(context.Background(), 11, "124.79.151.251", "ua-1", AccessTypeAPI); err != nil {
		t.Fatalf("record activity #1: %v", err)
	}
	if err := service.RecordActivity(context.Background(), 11, "124.79.151.251", "ua-2", AccessTypeAPI); err != nil {
		t.Fatalf("record activity #2: %v", err)
	}
	if elapsed := time.Since(start); elapsed >= 150*time.Millisecond {
		t.Fatalf("expected record activity to return before external lookup finishes, took %s", elapsed)
	}

	waitFor(t, time.Second, func() bool {
		return transport.requests.Load() == 1
	})

	var cache GeoCache
	waitFor(t, time.Second, func() bool {
		return db.Where("ip = ?", "124.79.151.251").Take(&cache).Error == nil
	})

	if cache.Country != "China" || cache.City != "Shanghai" || cache.CountryCode != "CN" {
		t.Fatalf("unexpected cached geo info: %+v", cache)
	}
}
