// Package payment provides payment gateway functionality.
package payment

import (
	"context"
	"sync"
	"time"

	"v/internal/logger"
)

// RetryScheduler manages scheduled payment retry tasks.
type RetryScheduler struct {
	retryService *RetryService
	logger       logger.Logger
	interval     time.Duration
	stopCh       chan struct{}
	wg           sync.WaitGroup
	running      bool
	mu           sync.Mutex
}

// SchedulerConfig holds configuration for the retry scheduler.
type SchedulerConfig struct {
	Interval time.Duration // How often to check for pending retries (default: 5 minutes)
	Enabled  bool          // Whether the scheduler is enabled
}

// DefaultSchedulerConfig returns the default scheduler configuration.
func DefaultSchedulerConfig() *SchedulerConfig {
	return &SchedulerConfig{
		Interval: 5 * time.Minute,
		Enabled:  true,
	}
}

// NewRetryScheduler creates a new retry scheduler.
func NewRetryScheduler(retryService *RetryService, config *SchedulerConfig, log logger.Logger) *RetryScheduler {
	if config == nil {
		config = DefaultSchedulerConfig()
	}
	return &RetryScheduler{
		retryService: retryService,
		logger:       log,
		interval:     config.Interval,
		stopCh:       make(chan struct{}),
	}
}

// Start starts the retry scheduler.
func (s *RetryScheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.mu.Unlock()

	s.wg.Add(1)
	go s.run()

	s.logger.Info("Payment retry scheduler started",
		logger.F("interval", s.interval.String()))
}

// Stop stops the retry scheduler.
func (s *RetryScheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	close(s.stopCh)
	s.mu.Unlock()

	s.wg.Wait()
	s.logger.Info("Payment retry scheduler stopped")
}

// IsRunning returns whether the scheduler is running.
func (s *RetryScheduler) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// run is the main scheduler loop.
func (s *RetryScheduler) run() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Run immediately on start
	s.processRetries()

	for {
		select {
		case <-ticker.C:
			s.processRetries()
		case <-s.stopCh:
			return
		}
	}
}

// processRetries processes all pending payment retries.
func (s *RetryScheduler) processRetries() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	succeeded, failed, err := s.retryService.ProcessPendingRetries(ctx)
	if err != nil {
		s.logger.Error("Failed to process pending retries", logger.Err(err))
		return
	}

	if succeeded > 0 || failed > 0 {
		s.logger.Info("Processed payment retries",
			logger.F("succeeded", succeeded),
			logger.F("failed", failed))
	}
}

// TriggerNow triggers an immediate retry processing cycle.
func (s *RetryScheduler) TriggerNow() {
	go s.processRetries()
}

// GetStatus returns the current scheduler status.
func (s *RetryScheduler) GetStatus() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	return map[string]interface{}{
		"running":  s.running,
		"interval": s.interval.String(),
	}
}
