package handlers

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"v/internal/database/repository"
)

type aggregateTrafficSpy struct {
	repository.TrafficRepository
	calls atomic.Int32
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
