package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"v/internal/database/repository"
)

// Feature: project-optimization, Property 20: Statistics Accuracy
// *For any* dashboard statistics query, the returned values SHALL match the actual
// counts and sums from the database (total_users = COUNT(users), total_proxies = COUNT(proxies),
// total_traffic = SUM(traffic.upload + traffic.download)).
// **Validates: Requirements 20.1, 20.2, 20.3, 20.4**

// setupStatsTestDB creates a test database with all required tables.
func setupStatsTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Create tables
	err = db.AutoMigrate(&repository.User{}, &repository.Proxy{}, &repository.Traffic{})
	require.NoError(t, err)

	return db
}

// TestStatisticsAccuracy_TotalUsers tests that total_users matches COUNT(users).
func TestStatisticsAccuracy_TotalUsers(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("total_users equals COUNT(users)", prop.ForAll(
		func(userCount int) bool {
			if userCount < 0 || userCount > 50 {
				return true // Skip invalid counts
			}

			// Create fresh database for each test
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			if err != nil {
				return false
			}
			db.AutoMigrate(&repository.User{})

			userRepo := repository.NewUserRepository(db)
			ctx := context.Background()

			// Create users
			for i := 0; i < userCount; i++ {
				user := &repository.User{
					Username:     generateUsername(i),
					PasswordHash: "hash",
					Email:        generateEmail(i),
					Role:         "user",
					Enabled:      true,
				}
				err := userRepo.Create(ctx, user)
				if err != nil {
					return false
				}
			}

			// Get count from repository
			count, err := userRepo.Count(ctx)
			if err != nil {
				return false
			}

			return count == int64(userCount)
		},
		gen.IntRange(0, 50),
	))

	properties.TestingRun(t)
}

// TestStatisticsAccuracy_ActiveUsers tests that active_users matches enabled and non-expired users.
func TestStatisticsAccuracy_ActiveUsers(t *testing.T) {
	// Simple unit test first
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	db.AutoMigrate(&repository.User{})

	userRepo := repository.NewUserRepository(db)
	ctx := context.Background()

	// Create 1 user first (will be enabled by default)
	user := &repository.User{
		Username:     "disabled_user",
		PasswordHash: "hash",
		Email:        "disabled@test.com",
		Role:         "user",
	}
	err = userRepo.Create(ctx, user)
	require.NoError(t, err)

	// Now disable the user
	user.Enabled = false
	err = db.Model(user).Update("enabled", false).Error
	require.NoError(t, err)

	// Check what's in the database
	var users []repository.User
	db.Find(&users)
	t.Logf("Users in DB: %d", len(users))
	for _, u := range users {
		t.Logf("User: %s, Enabled: %v, ExpiresAt: %v", u.Username, u.Enabled, u.ExpiresAt)
	}

	// Get active count - should be 0
	activeCount, err := userRepo.CountActive(ctx)
	require.NoError(t, err)
	t.Logf("CountActive result: %d", activeCount)
	assert.Equal(t, int64(0), activeCount, "Active count should be 0 for disabled user")

	// Get total count - should be 1
	totalCount, err := userRepo.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), totalCount, "Total count should be 1")
}

// TestStatisticsAccuracy_TotalProxies tests that total_proxies matches COUNT(proxies).
func TestStatisticsAccuracy_TotalProxies(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("total_proxies equals COUNT(proxies)", prop.ForAll(
		func(proxyCount int) bool {
			if proxyCount < 0 || proxyCount > 50 {
				return true
			}

			// Create fresh database for each test
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			if err != nil {
				return false
			}
			db.AutoMigrate(&repository.Proxy{})

			proxyRepo := repository.NewProxyRepository(db)
			ctx := context.Background()

			// Create proxies
			for i := 0; i < proxyCount; i++ {
				proxy := &repository.Proxy{
					UserID:   1,
					Name:     generateProxyName(i),
					Protocol: "vmess",
					Port:     10000 + i,
					Enabled:  true,
				}
				err := proxyRepo.Create(ctx, proxy)
				if err != nil {
					return false
				}
			}

			// Get count
			count, err := proxyRepo.Count(ctx)
			if err != nil {
				return false
			}

			return count == int64(proxyCount)
		},
		gen.IntRange(0, 50),
	))

	properties.TestingRun(t)
}

// TestStatisticsAccuracy_ActiveProxies tests that active_proxies matches enabled proxies.
func TestStatisticsAccuracy_ActiveProxies(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("active_proxies equals COUNT of enabled proxies", prop.ForAll(
		func(enabledCount, disabledCount int) bool {
			// Ensure valid counts
			if enabledCount < 0 {
				enabledCount = 0
			}
			if disabledCount < 0 {
				disabledCount = 0
			}
			if enabledCount+disabledCount > 30 {
				return true // Skip large tests
			}

			// Create fresh database for each test
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			if err != nil {
				return false
			}
			db.AutoMigrate(&repository.Proxy{})

			proxyRepo := repository.NewProxyRepository(db)
			ctx := context.Background()

			idx := 0
			// Create enabled proxies (default is enabled, so just create)
			for i := 0; i < enabledCount; i++ {
				proxy := &repository.Proxy{
					UserID:   1,
					Name:     generateProxyName(idx),
					Protocol: "vmess",
					Port:     10000 + idx,
					Enabled:  true,
				}
				if err := proxyRepo.Create(ctx, proxy); err != nil {
					return false
				}
				idx++
			}

			// Create disabled proxies - create first, then update to disabled
			// (GORM's default:true tag causes Enabled:false to be ignored on create)
			for i := 0; i < disabledCount; i++ {
				proxy := &repository.Proxy{
					UserID:   1,
					Name:     generateProxyName(idx),
					Protocol: "vmess",
					Port:     10000 + idx,
				}
				if err := proxyRepo.Create(ctx, proxy); err != nil {
					return false
				}
				// Update to disabled after creation
				if err := db.Model(proxy).Update("enabled", false).Error; err != nil {
					return false
				}
				idx++
			}

			// Get enabled count
			activeCount, err := proxyRepo.CountEnabled(ctx)
			if err != nil {
				return false
			}

			return activeCount == int64(enabledCount)
		},
		gen.IntRange(0, 15),
		gen.IntRange(0, 15),
	))

	properties.TestingRun(t)
}

// TestStatisticsAccuracy_TotalTraffic tests that total_traffic matches SUM(upload + download).
func TestStatisticsAccuracy_TotalTraffic(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("total_traffic equals SUM(upload + download)", prop.ForAll(
		func(records []trafficRecord) bool {
			if len(records) > 50 {
				return true
			}

			// Create fresh database for each test
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			if err != nil {
				return false
			}
			db.AutoMigrate(&repository.Traffic{})

			trafficRepo := repository.NewTrafficRepository(db)
			ctx := context.Background()

			var expectedUpload, expectedDownload int64

			// Create traffic records
			for i, rec := range records {
				traffic := &repository.Traffic{
					UserID:     1,
					ProxyID:    1,
					Upload:     rec.upload,
					Download:   rec.download,
					RecordedAt: time.Now().Add(time.Duration(-i) * time.Hour),
				}
				err := trafficRepo.Create(ctx, traffic)
				if err != nil {
					return false
				}
				expectedUpload += rec.upload
				expectedDownload += rec.download
			}

			// Get total traffic
			upload, download, err := trafficRepo.GetTotalTraffic(ctx)
			if err != nil {
				return false
			}

			return upload == expectedUpload && download == expectedDownload
		},
		gen.SliceOf(genTrafficRecord()),
	))

	properties.TestingRun(t)
}

// trafficRecord represents a traffic record for testing.
type trafficRecord struct {
	upload   int64
	download int64
}

// genTrafficRecord generates random traffic records.
func genTrafficRecord() gopter.Gen {
	return gopter.CombineGens(
		gen.Int64Range(0, 1000000),
		gen.Int64Range(0, 1000000),
	).Map(func(vals []interface{}) trafficRecord {
		return trafficRecord{
			upload:   vals[0].(int64),
			download: vals[1].(int64),
		}
	})
}

// Helper functions
func generateUsername(i int) string {
	return "user" + string(rune('a'+i%26)) + string(rune('0'+i/26))
}

func generateEmail(i int) string {
	return generateUsername(i) + "@test.com"
}

func generateProxyName(i int) string {
	return "proxy" + string(rune('a'+i%26)) + string(rune('0'+i/26))
}

// TestStatisticsAccuracy_ProtocolCounts tests protocol count accuracy.
func TestStatisticsAccuracy_ProtocolCounts(t *testing.T) {
	db := setupStatsTestDB(t)
	proxyRepo := repository.NewProxyRepository(db)
	ctx := context.Background()

	// Create proxies with different protocols
	protocols := map[string]int{
		"vmess":       3,
		"vless":       2,
		"trojan":      4,
		"shadowsocks": 1,
	}

	port := 10000
	for protocol, count := range protocols {
		for i := 0; i < count; i++ {
			proxy := &repository.Proxy{
				UserID:   1,
				Name:     protocol + string(rune('0'+i)),
				Protocol: protocol,
				Port:     port,
				Enabled:  true,
			}
			err := proxyRepo.Create(ctx, proxy)
			require.NoError(t, err)
			port++
		}
	}

	// Get protocol counts
	counts, err := proxyRepo.CountByProtocol(ctx)
	require.NoError(t, err)

	// Verify counts
	countMap := make(map[string]int64)
	for _, c := range counts {
		countMap[c.Protocol] = c.Count
	}

	for protocol, expectedCount := range protocols {
		assert.Equal(t, int64(expectedCount), countMap[protocol], "Protocol %s count mismatch", protocol)
	}
}

// TestStatisticsAccuracy_TrafficByPeriod tests traffic filtering by period.
func TestStatisticsAccuracy_TrafficByPeriod(t *testing.T) {
	db := setupStatsTestDB(t)
	trafficRepo := repository.NewTrafficRepository(db)
	ctx := context.Background()

	now := time.Now()

	// Create traffic records at different times
	// Today's traffic
	todayTraffic := &repository.Traffic{
		UserID:     1,
		ProxyID:    1,
		Upload:     1000,
		Download:   2000,
		RecordedAt: now.Add(-1 * time.Hour),
	}
	require.NoError(t, trafficRepo.Create(ctx, todayTraffic))

	// Yesterday's traffic
	yesterdayTraffic := &repository.Traffic{
		UserID:     1,
		ProxyID:    1,
		Upload:     500,
		Download:   1000,
		RecordedAt: now.Add(-25 * time.Hour),
	}
	require.NoError(t, trafficRepo.Create(ctx, yesterdayTraffic))

	// Last week's traffic
	lastWeekTraffic := &repository.Traffic{
		UserID:     1,
		ProxyID:    1,
		Upload:     200,
		Download:   400,
		RecordedAt: now.Add(-8 * 24 * time.Hour),
	}
	require.NoError(t, trafficRepo.Create(ctx, lastWeekTraffic))

	// Test today's period
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	upload, download, err := trafficRepo.GetTotalTrafficByPeriod(ctx, todayStart, now)
	require.NoError(t, err)
	assert.Equal(t, int64(1000), upload, "Today's upload mismatch")
	assert.Equal(t, int64(2000), download, "Today's download mismatch")

	// Test week period (should include today and yesterday)
	weekStart := now.Add(-7 * 24 * time.Hour)
	upload, download, err = trafficRepo.GetTotalTrafficByPeriod(ctx, weekStart, now)
	require.NoError(t, err)
	assert.Equal(t, int64(1500), upload, "Week's upload mismatch")
	assert.Equal(t, int64(3000), download, "Week's download mismatch")
}

// Feature: project-optimization, Property 21: Traffic Period Filtering
// *For any* traffic query with a time period filter, the returned traffic SHALL only
// include records within the specified time range.
// **Validates: Requirements 20.7**

// TestTrafficPeriodFiltering_TodayFilter tests that today filter only includes today's traffic.
func TestTrafficPeriodFiltering_TodayFilter(t *testing.T) {
	db := setupStatsTestDB(t)
	trafficRepo := repository.NewTrafficRepository(db)
	ctx := context.Background()

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Create today's traffic
	todayTraffic := &repository.Traffic{
		UserID:     1,
		ProxyID:    1,
		Upload:     1000,
		Download:   2000,
		RecordedAt: now.Add(-1 * time.Hour),
	}
	require.NoError(t, trafficRepo.Create(ctx, todayTraffic))

	// Create yesterday's traffic
	yesterdayTraffic := &repository.Traffic{
		UserID:     1,
		ProxyID:    1,
		Upload:     500,
		Download:   1000,
		RecordedAt: now.Add(-25 * time.Hour),
	}
	require.NoError(t, trafficRepo.Create(ctx, yesterdayTraffic))

	// Query today's traffic
	upload, download, err := trafficRepo.GetTotalTrafficByPeriod(ctx, todayStart, now)
	require.NoError(t, err)

	// Should only include today's traffic
	assert.Equal(t, int64(1000), upload, "Today's upload should be 1000")
	assert.Equal(t, int64(2000), download, "Today's download should be 2000")
}

// TestTrafficPeriodFiltering_WeekFilter tests that week filter includes last 7 days.
func TestTrafficPeriodFiltering_WeekFilter(t *testing.T) {
	db := setupStatsTestDB(t)
	trafficRepo := repository.NewTrafficRepository(db)
	ctx := context.Background()

	now := time.Now()
	weekStart := now.AddDate(0, 0, -7)

	// Create traffic within the week
	for i := 0; i < 5; i++ {
		traffic := &repository.Traffic{
			UserID:     1,
			ProxyID:    1,
			Upload:     100,
			Download:   200,
			RecordedAt: now.Add(time.Duration(-i*24) * time.Hour),
		}
		require.NoError(t, trafficRepo.Create(ctx, traffic))
	}

	// Create traffic outside the week (8 days ago)
	oldTraffic := &repository.Traffic{
		UserID:     1,
		ProxyID:    1,
		Upload:     9999,
		Download:   9999,
		RecordedAt: now.Add(-8 * 24 * time.Hour),
	}
	require.NoError(t, trafficRepo.Create(ctx, oldTraffic))

	// Query week's traffic
	upload, download, err := trafficRepo.GetTotalTrafficByPeriod(ctx, weekStart, now)
	require.NoError(t, err)

	// Should only include 5 records within the week
	assert.Equal(t, int64(500), upload, "Week's upload should be 500")
	assert.Equal(t, int64(1000), download, "Week's download should be 1000")
}

// TestTrafficPeriodFiltering_CustomRange tests custom date range filtering.
func TestTrafficPeriodFiltering_CustomRange(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("custom range only includes traffic within specified dates", prop.ForAll(
		func(daysAgo int, rangeDays int) bool {
			if daysAgo < 1 || daysAgo > 30 || rangeDays < 1 || rangeDays > 10 {
				return true // Skip invalid ranges
			}

			// Create fresh database
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			if err != nil {
				return false
			}
			db.AutoMigrate(&repository.Traffic{})

			trafficRepo := repository.NewTrafficRepository(db)
			ctx := context.Background()

			now := time.Now()
			rangeStart := now.AddDate(0, 0, -daysAgo)
			rangeEnd := rangeStart.AddDate(0, 0, rangeDays)

			// Create traffic within range
			inRangeUpload := int64(0)
			inRangeDownload := int64(0)
			for i := 0; i < rangeDays; i++ {
				traffic := &repository.Traffic{
					UserID:     1,
					ProxyID:    1,
					Upload:     100,
					Download:   200,
					RecordedAt: rangeStart.Add(time.Duration(i*12) * time.Hour),
				}
				if err := trafficRepo.Create(ctx, traffic); err != nil {
					return false
				}
				inRangeUpload += 100
				inRangeDownload += 200
			}

			// Create traffic outside range (before)
			beforeTraffic := &repository.Traffic{
				UserID:     1,
				ProxyID:    1,
				Upload:     9999,
				Download:   9999,
				RecordedAt: rangeStart.Add(-24 * time.Hour),
			}
			trafficRepo.Create(ctx, beforeTraffic)

			// Create traffic outside range (after)
			afterTraffic := &repository.Traffic{
				UserID:     1,
				ProxyID:    1,
				Upload:     8888,
				Download:   8888,
				RecordedAt: rangeEnd.Add(24 * time.Hour),
			}
			trafficRepo.Create(ctx, afterTraffic)

			// Query custom range
			upload, download, err := trafficRepo.GetTotalTrafficByPeriod(ctx, rangeStart, rangeEnd)
			if err != nil {
				return false
			}

			return upload == inRangeUpload && download == inRangeDownload
		},
		gen.IntRange(1, 30),
		gen.IntRange(1, 10),
	))

	properties.TestingRun(t)
}

// TestTrafficPeriodFiltering_EmptyRange tests that empty range returns zero traffic.
func TestTrafficPeriodFiltering_EmptyRange(t *testing.T) {
	db := setupStatsTestDB(t)
	trafficRepo := repository.NewTrafficRepository(db)
	ctx := context.Background()

	now := time.Now()

	// Create some traffic
	traffic := &repository.Traffic{
		UserID:     1,
		ProxyID:    1,
		Upload:     1000,
		Download:   2000,
		RecordedAt: now,
	}
	require.NoError(t, trafficRepo.Create(ctx, traffic))

	// Query a range with no traffic (far in the past)
	farPast := now.AddDate(-10, 0, 0)
	upload, download, err := trafficRepo.GetTotalTrafficByPeriod(ctx, farPast, farPast.Add(24*time.Hour))
	require.NoError(t, err)

	assert.Equal(t, int64(0), upload, "Empty range should have 0 upload")
	assert.Equal(t, int64(0), download, "Empty range should have 0 download")
}
