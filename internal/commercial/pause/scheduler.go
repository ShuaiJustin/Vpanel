// Package pause provides subscription pause functionality.
package pause

import (
	"context"
	"sync"
	"time"

	"v/internal/logger"
)

// Scheduler manages scheduled auto-resume tasks.
type Scheduler struct {
	pauseService *Service
	logger       logger.Logger
	interval     time.Duration
	stopCh       chan struct{}
	wg           sync.WaitGroup
	running      bool
	mu           sync.Mutex
}

// SchedulerConfig holds configuration for the pause scheduler.
type SchedulerConfig struct {
	Interval time.Duration // How often to check for pauses to auto-resume (default: 1 hour)
	Enabled  bool          // Whether the scheduler is enabled
}

// DefaultSchedulerConfig returns the default scheduler configuration.
func DefaultSchedulerConfig() *SchedulerConfig {
	return &SchedulerConfig{
		Interval: 1 * time.Hour,
		Enabled:  true,
	}
}

// NewScheduler creates a new pause scheduler.
func NewScheduler(pauseService *Service, config *SchedulerConfig, log logger.Logger) *Scheduler {
	if config == nil {
		config = DefaultSchedulerConfig()
	}
	return &Scheduler{
		pauseService: pauseService,
		logger:       log,
		interval:     config.Interval,
		stopCh:       make(chan struct{}),
	}
}

// Start starts the pause scheduler.
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

	s.logger.Info("Pause auto-resume scheduler started",
		logger.F("interval", s.interval.String()))
}

// Stop stops the pause scheduler.
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
	s.logger.Info("Pause auto-resume scheduler stopped")
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

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Run immediately on start
	s.processAutoResume()

	for {
		select {
		case <-ticker.C:
			s.processAutoResume()
		case <-s.stopCh:
			return
		}
	}
}

// processAutoResume processes all pauses that need to be auto-resumed.
func (s *Scheduler) processAutoResume() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	resumed, err := s.pauseService.AutoResumePaused(ctx)
	if err != nil {
		s.logger.Error("Failed to process auto-resume", logger.Err(err))
		return
	}

	if resumed > 0 {
		s.logger.Info("Auto-resumed paused subscriptions",
			logger.F("count", resumed))
	}
}

// TriggerNow triggers an immediate auto-resume processing cycle.
func (s *Scheduler) TriggerNow() {
	go s.processAutoResume()
}

// GetStatus returns the current scheduler status.
func (s *Scheduler) GetStatus() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	return map[string]interface{}{
		"running":  s.running,
		"interval": s.interval.String(),
	}
}
