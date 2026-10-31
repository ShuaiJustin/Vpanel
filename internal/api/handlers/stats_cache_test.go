package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"v/internal/cache"
	"v/internal/database/repository"
	"v/internal/logger"
)

type aggregateTrafficSpy struct {
	repository.TrafficRepository
	calls atomic.Int32
}

type detailedTrafficSpy struct {
	repository.TrafficRepository
	totalCalls    atomic.Int32
	userCalls     atomic.Int32
	timelineCalls atomic.Int32
}

func (s *detailedTrafficSpy) GetTotalTrafficByPeriod(ctx context.Context, start, end time.Time) (int64, int64, error) {
	s.totalCalls.Add(1)
	return s.TrafficRepository.GetTotalTrafficByPeriod(ctx, start, end)
}

func (s *detailedTrafficSpy) GetTrafficByUser(ctx context.Context, start, end time.Time, limit int) ([]*repository.UserTrafficStats, error) {
	s.userCalls.Add(1)
	return s.TrafficRepository.GetTrafficByUser(ctx, start, end, limit)
}

func (s *detailedTrafficSpy) GetTrafficTimeline(ctx context.Context, start, end time.Time, interval string) ([]*repository.TrafficTimelinePoint, error) {
	s.timelineCalls.Add(1)
	return s.TrafficRepository.GetTrafficTimeline(ctx, start, end, interval)
}

func TestDetailedDashboardSkipsUnusedQueriesAndCoalescesCacheMisses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupStatsTestDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.Create(&repository.User{ID: 1, Username: "stats-user"}).Error)
	require.NoError(t, db.Create(&repository.Traffic{UserID: 1, Upload: 12, Download: 34, RecordedAt: time.Now()}).Error)
	repos := repository.NewRepositories(db)
	spy := &detailedTrafficSpy{TrafficRepository: repos.Traffic}
	repos.Traffic = spy
	c := cache.NewMemoryCache(cache.DefaultConfig())
	t.Cleanup(func() { _ = c.Close() })
	h := NewStatsHandler(logger.NewNopLogger(), repos, c)
	router := gin.New()
	router.GET("/stats/detailed", h.GetDetailedStats)
	request := func(query string) DetailedStats {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/stats/detailed?period=month"+query, nil))
		require.Equal(t, http.StatusOK, w.Code)
		var body struct {
			Data DetailedStats `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		return body.Data
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data := request("&view=dashboard")
			require.Equal(t, int64(46), data.TotalTraffic)
			require.Empty(t, data.ByUser)
			require.Empty(t, data.Timeline)
		}()
	}
	wg.Wait()
	require.Equal(t, int32(1), spy.totalCalls.Load())
	require.Zero(t, spy.userCalls.Load())
	require.Zero(t, spy.timelineCalls.Load())
	summary := request("&view=dashboard")
	full := request("")
	require.Equal(t, summary.TotalTraffic, full.TotalTraffic)
	require.Equal(t, summary.ByProtocol, full.ByProtocol)
	require.NotEmpty(t, full.ByUser)
	require.NotEmpty(t, full.Timeline)
	require.Equal(t, int32(2), spy.totalCalls.Load())
	request("")
	require.Equal(t, int32(2), spy.totalCalls.Load())
	require.Equal(t, int32(1), spy.userCalls.Load())
	require.Equal(t, int32(1), spy.timelineCalls.Load())
}

func (s *aggregateTrafficSpy) GetTotalTraffic(context.Context) (int64, int64, error) {
	s.calls.Add(1)
	return 12, 34, nil
}

func TestDashboardHistoricalTotalCacheCoalescesMisses(t *testing.T) {
	spy := &aggregateTrafficSpy{}
	h := &StatsHandler{repos: &repository.Repositories{Traffic: spy}}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			up, down, err := h.dashboardTraffic(context.Background())
			require.NoError(t, err)
			require.Equal(t, int64(12), up)
			require.Equal(t, int64(34), down)
		}()
	}
	wg.Wait()
	require.Equal(t, int32(1), spy.calls.Load())
}
