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

	if err := db.AutoMigrate(&ip.ActiveIP{}, &ip.IPBlacklist{}, &ip.IPWhitelist{}, &ip.IPHistory{}, &ip.FailedAttempt{}, &ip.GeoCache{}); err != nil {
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

func TestIPRestrictionHandlerGetAllIPHistory_EnrichesMissingGeo(t *testing.T) {
	db := setupIPRestrictionHandlerTestDB(t)
	ipService, err := ip.NewService(db, &ip.ServiceConfig{GeoConfig: &ip.GeolocationConfig{DatabasePath: "", CacheTTL: 24 * time.Hour}})
	if err != nil {
		t.Fatalf("create ip service: %v", err)
	}

	handler := NewIPRestrictionHandler(logger.NewNopLogger(), ipService)
	router := gin.New()
	router.GET("/history", handler.GetAllIPHistory)

	now := time.Now()
	if err := db.Create(&ip.GeoCache{IP: "124.79.151.251", Country: "China", CountryCode: "CN", Region: "Shanghai", City: "Shanghai", CachedAt: now}).Error; err != nil {
		t.Fatalf("seed geo cache: %v", err)
	}
	if err := db.Create(&ip.IPHistory{UserID: 42, IP: "124.79.151.251", UserAgent: "ua", AccessType: ip.AccessTypeProxy, CreatedAt: now}).Error; err != nil {
		t.Fatalf("seed history: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/history?user_id=42", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response struct {
		Code int `json:"code"`
		Data []struct {
			IP          string `json:"ip"`
			Country     string `json:"country"`
			CountryCode string `json:"country_code"`
			City        string `json:"city"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Code != 200 || len(response.Data) != 1 {
		t.Fatalf("unexpected response: %+v", response)
	}
	if response.Data[0].Country != "China" || response.Data[0].City != "Shanghai" || response.Data[0].CountryCode != "CN" {
		t.Fatalf("unexpected enriched data: %+v", response.Data[0])
	}
}

func TestIPRestrictionHandlerUserReadEndpoints_Return200WithColdGeoCache(t *testing.T) {
	db := setupIPRestrictionHandlerTestDB(t)
	ipService, err := ip.NewService(db, &ip.ServiceConfig{GeoConfig: &ip.GeolocationConfig{DatabasePath: "", CacheTTL: 24 * time.Hour}})
	if err != nil {
		t.Fatalf("create ip service: %v", err)
	}

	handler := NewIPRestrictionHandler(logger.NewNopLogger(), ipService)
	router := gin.New()
	router.GET("/devices", func(c *gin.Context) {
		c.Set("user_id", int64(42))
		handler.GetUserDevices(c)
	})
	router.GET("/ip-history", func(c *gin.Context) {
		c.Set("user_id", int64(42))
		handler.GetUserIPHistory(c)
	})

	now := time.Now()
	if err := db.Create(&ip.ActiveIP{
		UserID:     42,
		IP:         "124.79.151.251",
		UserAgent:  "ua-device",
		DeviceType: "desktop",
		LastActive: now,
		CreatedAt:  now,
	}).Error; err != nil {
		t.Fatalf("seed active ip: %v", err)
	}
	if err := db.Create(&ip.IPHistory{
		UserID:     42,
		IP:         "124.79.151.251",
		UserAgent:  "ua-history",
		AccessType: ip.AccessTypeProxy,
		CreatedAt:  now,
	}).Error; err != nil {
		t.Fatalf("seed history: %v", err)
	}

	deviceReq := httptest.NewRequest(http.MethodGet, "/devices", nil)
	deviceRes := httptest.NewRecorder()
	router.ServeHTTP(deviceRes, deviceReq)

	if deviceRes.Code != http.StatusOK {
		t.Fatalf("expected device status 200, got %d: %s", deviceRes.Code, deviceRes.Body.String())
	}

	var devicePayload struct {
		Code int `json:"code"`
		Data struct {
			Devices []struct {
				IP      string `json:"ip"`
				Country string `json:"country"`
				City    string `json:"city"`
			} `json:"devices"`
		} `json:"data"`
	}
	if err := json.Unmarshal(deviceRes.Body.Bytes(), &devicePayload); err != nil {
		t.Fatalf("decode devices response: %v", err)
	}
	if devicePayload.Code != 200 || len(devicePayload.Data.Devices) != 1 {
		t.Fatalf("unexpected devices response: %+v", devicePayload)
	}
	if devicePayload.Data.Devices[0].IP != "124.79.151.251" {
		t.Fatalf("unexpected device payload: %+v", devicePayload.Data.Devices[0])
	}

	historyReq := httptest.NewRequest(http.MethodGet, "/ip-history?limit=10&offset=0", nil)
	historyRes := httptest.NewRecorder()
	router.ServeHTTP(historyRes, historyReq)

	if historyRes.Code != http.StatusOK {
		t.Fatalf("expected history status 200, got %d: %s", historyRes.Code, historyRes.Body.String())
	}

	var historyPayload struct {
		Code int `json:"code"`
		Data struct {
			List []struct {
				IP      string `json:"ip"`
				Country string `json:"country"`
				City    string `json:"city"`
			} `json:"list"`
			Total int64 `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(historyRes.Body.Bytes(), &historyPayload); err != nil {
		t.Fatalf("decode history response: %v", err)
	}
	if historyPayload.Code != 200 || historyPayload.Data.Total != 1 || len(historyPayload.Data.List) != 1 {
		t.Fatalf("unexpected history response: %+v", historyPayload)
	}
	if historyPayload.Data.List[0].IP != "124.79.151.251" {
		t.Fatalf("unexpected history payload: %+v", historyPayload.Data.List[0])
	}
}
