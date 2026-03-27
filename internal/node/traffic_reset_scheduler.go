package node

import (
	"context"
	"sync"
	"time"

	"v/internal/logger"
)

// TrafficResetSchedulerConfig controls periodic node traffic reset checks.
type TrafficResetSchedulerConfig struct {
	Interval time.Duration
}

// DefaultTrafficResetSchedulerConfig returns the default scheduler config.
func DefaultTrafficResetSchedulerConfig() *TrafficResetSchedulerConfig {
	return &TrafficResetSchedulerConfig{Interval: time.Hour}
}

// TrafficResetScheduler periodically resets node traffic counters when their monthly cycle is due.
type TrafficResetScheduler struct {
	config         *TrafficResetSchedulerConfig
	trafficService *TrafficService
	logger         logger.Logger

	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	running   bool
	runningMu sync.Mutex
}

// NewTrafficResetScheduler creates a new traffic reset scheduler.
func NewTrafficResetScheduler(cfg *TrafficResetSchedulerConfig, trafficService *TrafficService, log logger.Logger) *TrafficResetScheduler {
	if cfg == nil {
		cfg = DefaultTrafficResetSchedulerConfig()
	}
	if cfg.Interval <= 0 {
		cfg.Interval = time.Hour
	}
	return &TrafficResetScheduler{
		config:         cfg,
		trafficService: trafficService,
		logger:         log,
	}
}

// Start starts the scheduler loop.
func (s *TrafficResetScheduler) Start(ctx context.Context) error {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	if s.running {
		return nil
	}

	s.ctx, s.cancel = context.WithCancel(ctx)
	s.running = true
	s.wg.Add(1)
	go s.runLoop()

	s.logger.Info("node traffic reset scheduler started",
		logger.F("interval", s.config.Interval.String()))
	return nil
}

// Stop stops the scheduler loop.
func (s *TrafficResetScheduler) Stop(ctx context.Context) error {
	s.runningMu.Lock()
	if !s.running {
		s.runningMu.Unlock()
		return nil
	}
	s.cancel()
	s.running = false
	s.runningMu.Unlock()

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("node traffic reset scheduler stopped")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *TrafficResetScheduler) runLoop() {
	defer s.wg.Done()

	s.runOnce()

	ticker := time.NewTicker(s.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.runOnce()
		}
	}
}

func (s *TrafficResetScheduler) runOnce() {
	if s == nil || s.trafficService == nil {
		return
	}

	resetNodeIDs, err := s.trafficService.ProcessMonthlyTrafficResets(s.ctx, time.Now())
	if err != nil {
		s.logger.Warn("node traffic reset scheduler run failed", logger.Err(err))
		return
	}
	if len(resetNodeIDs) == 0 {
		return
	}

	s.logger.Info("node traffic reset scheduler processed due nodes",
		logger.F("count", len(resetNodeIDs)),
		logger.F("node_ids", resetNodeIDs))
}
