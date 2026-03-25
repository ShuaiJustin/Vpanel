package ip

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestServiceEnrichesMissingGeoForDevicesAndHistory(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if err := db.AutoMigrate(&ActiveIP{}, &IPHistory{}, &GeoCache{}); err != nil {
		t.Fatalf("migrate tables: %v", err)
	}

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

	service, err := NewService(db, &ServiceConfig{
		GeoConfig: &GeolocationConfig{
			DatabasePath: "",
			CacheTTL:     24 * time.Hour,
		},
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
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
	if activeIP.Country != "China" || activeIP.City != "Shanghai" {
		t.Fatalf("unexpected persisted active ip location: %+v", activeIP)
	}

	var historyRecord IPHistory
	if err := db.Where("user_id = ? AND ip = ?", userID, testIP).Take(&historyRecord).Error; err != nil {
		t.Fatalf("reload history: %v", err)
	}
	if historyRecord.Country != "China" || historyRecord.City != "Shanghai" {
		t.Fatalf("unexpected persisted history location: %+v", historyRecord)
	}
}
