package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"v/internal/ip"
	"v/internal/logger"
)

func setupIPRestrictionHandlerTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if err := db.AutoMigrate(&ip.ActiveIP{}, &ip.IPBlacklist{}, &ip.IPWhitelist{}, &ip.IPHistory{}, &ip.FailedAttempt{}); err != nil {
		t.Fatalf("migrate ip restriction tables: %v", err)
	}

	return db
}

func TestIPRestrictionHandlerGetStats_UsesLiveDataAndCleansStaleRecords(t *testing.T) {
	db := setupIPRestrictionHandlerTestDB(t)
	ipService, err := ip.NewService(db, nil)
	if err != nil {
		t.Fatalf("create ip service: %v", err)
	}

	handler := NewIPRestrictionHandler(logger.NewNopLogger(), ipService)
	router := gin.New()
	router.GET("/stats", handler.GetStats)

	now := time.Now()
	staleActiveAt := now.Add(-15 * time.Minute)
	freshActiveAt := now.Add(-2 * time.Minute)
	expiredBlacklistAt := now.Add(-1 * time.Hour)
	activeBlacklistAt := now.Add(1 * time.Hour)

	records := []interface{}{
		&ip.ActiveIP{
			UserID:     1,
			IP:         "1.1.1.1",
			UserAgent:  "ua-japan",
			DeviceType: "desktop",
			Country:    "Japan",
			City:       "Tokyo",
			LastActive: freshActiveAt,
			CreatedAt:  freshActiveAt,
		},
		&ip.ActiveIP{
			UserID:     2,
			IP:         "2.2.2.2",
			UserAgent:  "ua-stale",
			DeviceType: "mobile",
			Country:    "United States",
			City:       "San Jose",
			LastActive: staleActiveAt,
			CreatedAt:  staleActiveAt,
		},
		&ip.IPBlacklist{
			IP:        "3.3.3.3",
			Reason:    "manual block",
			ExpiresAt: &activeBlacklistAt,
			CreatedAt: now,
		},
		&ip.IPBlacklist{
			IP:        "4.4.4.4",
			Reason:    "expired block",
			ExpiresAt: &expiredBlacklistAt,
			CreatedAt: now.Add(-2 * time.Hour),
		},
		&ip.IPWhitelist{
			IP:        "5.5.5.5",
			CreatedBy: 1,
			CreatedAt: now,
			UpdatedAt: now,
		},
		&ip.FailedAttempt{IP: "6.6.6.6", Reason: "ip limit", CreatedAt: now},
		&ip.FailedAttempt{IP: "7.7.7.7", Reason: "blacklist", CreatedAt: now},
		&ip.FailedAttempt{IP: "8.8.8.8", Reason: "old failure", CreatedAt: now.Add(-24 * time.Hour)},
		&ip.IPHistory{
			UserID:       1,
			IP:           "9.9.9.9",
			UserAgent:    "ua-suspicious",
			AccessType:   ip.AccessTypeProxy,
			Country:      "Japan",
			City:         "Tokyo",
			IsSuspicious: true,
			CreatedAt:    now,
		},
		&ip.IPHistory{
			UserID:       1,
			IP:           "9.9.9.10",
			UserAgent:    "ua-normal",
			AccessType:   ip.AccessTypeAPI,
			Country:      "Japan",
			City:         "Tokyo",
			IsSuspicious: false,
			CreatedAt:    now,
		},
	}

	for _, record := range records {
		if err := db.Create(record).Error; err != nil {
			t.Fatalf("insert record %T: %v", record, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response struct {
		Code int `json:"code"`
		Data struct {
			TotalActiveIPs   int64 `json:"total_active_ips"`
			TotalBlacklisted int64 `json:"total_blacklisted"`
			TotalWhitelisted int64 `json:"total_whitelisted"`
			ActiveUsers      int64 `json:"active_users"`
			BlockedToday     int64 `json:"blocked_today"`
			SuspiciousCount  int64 `json:"suspicious_count"`
			CountryStats     []struct {
				Country string `json:"country"`
				Count   int64  `json:"count"`
			} `json:"country_stats"`
		} `json:"data"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != 200 {
		t.Fatalf("unexpected code: got %d want 200", response.Code)
	}
	if response.Data.TotalActiveIPs != 1 {
		t.Fatalf("unexpected total active IPs: got %d want 1", response.Data.TotalActiveIPs)
	}
	if response.Data.TotalBlacklisted != 1 {
		t.Fatalf("unexpected total blacklisted: got %d want 1", response.Data.TotalBlacklisted)
	}
	if response.Data.TotalWhitelisted != 1 {
		t.Fatalf("unexpected total whitelisted: got %d want 1", response.Data.TotalWhitelisted)
	}
	if response.Data.ActiveUsers != 1 {
		t.Fatalf("unexpected active users: got %d want 1", response.Data.ActiveUsers)
	}
	if response.Data.BlockedToday != 2 {
		t.Fatalf("unexpected blocked today: got %d want 2", response.Data.BlockedToday)
	}
	if response.Data.SuspiciousCount != 1 {
		t.Fatalf("unexpected suspicious count: got %d want 1", response.Data.SuspiciousCount)
	}
	if len(response.Data.CountryStats) != 1 {
		t.Fatalf("unexpected country stats length: got %d want 1", len(response.Data.CountryStats))
	}
	if response.Data.CountryStats[0].Country != "Japan" || response.Data.CountryStats[0].Count != 1 {
		t.Fatalf("unexpected country stats entry: %+v", response.Data.CountryStats[0])
	}

	var activeIPCount int64
	if err := db.Model(&ip.ActiveIP{}).Count(&activeIPCount).Error; err != nil {
		t.Fatalf("count active IPs after cleanup: %v", err)
	}
	if activeIPCount != 1 {
		t.Fatalf("expected stale active IP cleanup to leave 1 row, got %d", activeIPCount)
	}

	var blacklistCount int64
	if err := db.Model(&ip.IPBlacklist{}).Count(&blacklistCount).Error; err != nil {
		t.Fatalf("count blacklist after cleanup: %v", err)
	}
	if blacklistCount != 1 {
		t.Fatalf("expected expired blacklist cleanup to leave 1 row, got %d", blacklistCount)
	}
}
