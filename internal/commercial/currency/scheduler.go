// Package currency provides multi-currency support functionality.
package currency

import (
	"context"
	"sync"
	"time"

	"v/internal/logger"
)

// Scheduler handles periodic exchange rate updates.
type Scheduler struct {
	service  *Service
	interval time.Duration
	logger   logger.Logger
	stopCh   chan struct{}
	wg       sync.WaitGroup
	running  bool
	mu       sync.Mutex
}

// SchedulerConfig holds scheduler configuration.
type SchedulerConfig struct {
	Interval time.Duration // Update interval (default: 1 hour)
	Enabled  bool          // Whether scheduler is enabled
}

// DefaultSchedulerConfig returns default scheduler configuration.
func DefaultSchedulerConfig() *SchedulerConfig {
	return &SchedulerConfig{
		Interval: time.Hour,
		Enabled:  true,
	}
}

// NewScheduler creates a new exchange rate update scheduler.
func NewScheduler(service *Service, config *SchedulerConfig, log logger.Logger) *Scheduler {
	if config == nil {
		config = DefaultSchedulerConfig()
	}

	return &Scheduler{
		service:  service,
		interval: config.Interval,
		logger:   log,
		stopCh:   make(chan struct{}),
	}
}

// Start starts the scheduler.
func (s *Scheduler) Start() {
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

	s.logger.Info("Exchange rate scheduler started", logger.F("interval", s.interval.String()))
}

// Stop stops the scheduler.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	close(s.stopCh)
	s.mu.Unlock()

	s.wg.Wait()
	s.logger.Info("Exchange rate scheduler stopped")
}

// IsRunning returns whether the scheduler is running.
func (s *Scheduler) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// run is the main scheduler loop.
func (s *Scheduler) run() {
	defer s.wg.Done()

	// Run immediately on start
	s.updateRates()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.updateRates()
		case <-s.stopCh:
			return
		}
	}
}

// updateRates performs the exchange rate update.
func (s *Scheduler) updateRates() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s.logger.Info("Updating exchange rates")

	if err := s.service.UpdateRates(ctx); err != nil {
		s.logger.Error("Failed to update exchange rates", logger.Err(err))
		return
	}

	s.logger.Info("Exchange rates updated successfully")
}

// TriggerUpdate manually triggers an exchange rate update.
func (s *Scheduler) TriggerUpdate() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return s.service.UpdateRates(ctx)
}

// SetInterval updates the scheduler interval.
func (s *Scheduler) SetInterval(interval time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if interval < time.Minute {
		interval = time.Minute // Minimum 1 minute
	}

	s.interval = interval
	s.logger.Info("Exchange rate scheduler interval updated", logger.F("interval", interval.String()))

	// Restart if running
	if s.running {
		close(s.stopCh)
		s.wg.Wait()
		s.stopCh = make(chan struct{})
		s.wg.Add(1)
		go s.run()
	}
}

// GetInterval returns the current scheduler interval.
func (s *Scheduler) GetInterval() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.interval
}

// GetLastUpdateTime returns the last update time from the service cache.
func (s *Scheduler) GetLastUpdateTime() time.Time {
	// This would need to be tracked in the service
	// For now, return zero time
	return time.Time{}
}
