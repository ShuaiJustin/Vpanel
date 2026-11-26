// Package pause provides subscription pause functionality.
package pause

import (
	"context"
	stderrors "errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"v/internal/database/repository"
	"v/internal/logger"
	"v/pkg/errors"
)

// Config holds pause service configuration.
type Config struct {
	Enabled        bool    `json:"enabled"`
	MaxDuration    int     `json:"max_duration"`     // days, default 30
	MaxPerCycle    int     `json:"max_per_cycle"`    // times per billing cycle, default 1
	AllowedPlanIDs []int64 `json:"allowed_plan_ids"` // empty = all plans
}

// DefaultConfig returns the default pause configuration.
func DefaultConfig() *Config {
	return &Config{
		Enabled:        true,
		MaxDuration:    30,
		MaxPerCycle:    1,
		AllowedPlanIDs: nil, // All plans allowed
	}
}

// PauseResult represents the result of a pause operation.
type PauseResult struct {
	Pause        *repository.SubscriptionPause `json:"pause"`
	AutoResumeAt time.Time                     `json:"auto_resume_at"`
	MaxDuration  int                           `json:"max_duration_days"`
}

// PauseStatus represents the current pause status for a user.
type PauseStatus struct {
	IsPaused          bool                          `json:"is_paused"`
	Pause             *repository.SubscriptionPause `json:"pause,omitempty"`
	CanPause          bool                          `json:"can_pause"`
	CannotPauseReason string                        `json:"cannot_pause_reason,omitempty"`
	RemainingPauses   int                           `json:"remaining_pauses"`
	MaxDuration       int                           `json:"max_duration_days"`
}

// Service provides subscription pause operations.
type Service struct {
	pauseRepo     repository.PauseRepository
	userRepo      repository.UserRepository
	config        *Config
	logger        logger.Logger
	accessChanged func(ctx context.Context, userID int64)
}

// NewService creates a new pause service.
func NewService(
	pauseRepo repository.PauseRepository,
	userRepo repository.UserRepository,
	logger logger.Logger,
	config *Config,
) *Service {
	if config == nil {
		config = DefaultConfig()
	}
	return &Service{
		pauseRepo: pauseRepo,
		userRepo:  userRepo,
		config:    config,
		logger:    logger,
	}
}

// WithAccessChangedHook is called after a pause or resume changed whether the
// user may connect, so the nodes serving that user can be re-synced.
func (s *Service) WithAccessChangedHook(hook func(ctx context.Context, userID int64)) *Service {
	s.accessChanged = hook
	return s
}

func (s *Service) notifyAccessChanged(ctx context.Context, userID int64) {
	if s.accessChanged != nil {
		s.accessChanged(ctx, userID)
	}
}

// GetPauseStatus returns the current pause status for a user.
func (s *Service) GetPauseStatus(ctx context.Context, userID int64) (*PauseStatus, error) {
	status := &PauseStatus{
		MaxDuration: s.config.MaxDuration,
	}

	// Check if pause feature is enabled
	if !s.config.Enabled {
		status.CanPause = false
		status.CannotPauseReason = "暂停功能未开启"
		return status, nil
	}

	// Get active pause
	activePause, err := s.pauseRepo.GetActivePause(ctx, userID)
	if err != nil {
		return nil, err
	}

	if activePause != nil {
		status.IsPaused = true
		status.Pause = activePause
		status.CanPause = false
		status.CannotPauseReason = "订阅已处于暂停状态"
		return status, nil
	}

	// Check if user can pause
	canPause, reason := s.CanPause(ctx, userID)
	status.CanPause = canPause
	status.CannotPauseReason = reason

	// Calculate remaining pauses in current cycle
	remainingPauses, _ := s.getRemainingPausesInCycle(ctx, userID)
	status.RemainingPauses = remainingPauses

	return status, nil
}

// CanPause checks if a user can pause their subscription.
func (s *Service) CanPause(ctx context.Context, userID int64) (bool, string) {
	// Check if pause feature is enabled
	if !s.config.Enabled {
		return false, "暂停功能未开启"
	}

	// Get user
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get user for pause check", logger.F("error", err), logger.F("user_id", userID))
		return false, "用户校验失败"
	}

	// Permanent subscriptions (no expiry, with a quota) are valid. An expiry in
	// the past, or an account that never had a plan, has nothing to freeze.
	if (user.ExpiresAt != nil && user.ExpiresAt.Before(time.Now())) || (user.ExpiresAt == nil && user.TrafficLimit <= 0) {
		return false, "当前无有效订阅"
	}

	// Check if already paused
	activePause, err := s.pauseRepo.GetActivePause(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to check active pause", logger.F("error", err), logger.F("user_id", userID))
		return false, "暂停状态检查失败"
	}
	if activePause != nil {
		return false, "订阅已处于暂停状态"
	}

	// Check pause frequency limit
	remainingPauses, err := s.getRemainingPausesInCycle(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to check pause frequency", logger.F("error", err), logger.F("user_id", userID))
		return false, "暂停次数检查失败"
	}
	if remainingPauses <= 0 {
		return false, fmt.Sprintf("当前计费周期最多可暂停 %d 次，已达到上限", s.config.MaxPerCycle)
	}

	return true, ""
}

// getRemainingPausesInCycle calculates remaining pauses in the current billing cycle.
func (s *Service) getRemainingPausesInCycle(ctx context.Context, userID int64) (int, error) {
	// Get user to determine billing cycle
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return 0, err
	}

	// Same 30-day window as the atomic check in PauseSubscription.
	cycleStart := time.Now().AddDate(0, 0, -30)
	if cycleStart.Before(user.CreatedAt) {
		cycleStart = user.CreatedAt
	}

	// Count pauses in this cycle
	pauseCount, err := s.pauseRepo.CountPausesInPeriod(ctx, userID, cycleStart, time.Now())
	if err != nil {
		return 0, err
	}

	remaining := s.config.MaxPerCycle - pauseCount
	if remaining < 0 {
		remaining = 0
	}

	return remaining, nil
}

// Pause pauses a user's subscription.
func (s *Service) Pause(ctx context.Context, userID int64) (*PauseResult, error) {
	// Check if user can pause
	canPause, reason := s.CanPause(ctx, userID)
	if !canPause {
		return nil, errors.NewValidationError("cannot_pause", reason)
	}

	now := time.Now()
	autoResumeAt := now.AddDate(0, 0, s.config.MaxDuration)
	pause := &repository.SubscriptionPause{UserID: userID, PausedAt: now, AutoResumeAt: autoResumeAt}

	// The pause record and the end of access (expires_at = pause time) are
	// written together; a pause without the freeze would hand out the frozen
	// days a second time on resume.
	atomicRepo, ok := s.pauseRepo.(repository.AtomicPauseRepository)
	if !ok {
		return nil, fmt.Errorf("atomic pause repository is required")
	}
	if err := atomicRepo.PauseSubscription(ctx, pause, s.config.MaxPerCycle); err != nil {
		if stderrors.Is(err, repository.ErrCommercialState) {
			return nil, errors.NewValidationError("cannot_pause", "订阅状态或暂停次数已变化，请刷新后重试")
		}
		s.logger.Error("Failed to pause subscription", logger.F("error", err), logger.F("user_id", userID))
		return nil, errors.NewDatabaseError("failed to pause subscription", err)
	}
	s.notifyAccessChanged(ctx, userID)

	s.logger.Info("Subscription paused",
		logger.F("user_id", userID),
		logger.F("remaining_days", pause.RemainingDays),
		logger.F("remaining_traffic", pause.RemainingTraffic),
		logger.F("auto_resume_at", autoResumeAt),
	)

	return &PauseResult{
		Pause:        pause,
		AutoResumeAt: autoResumeAt,
		MaxDuration:  s.config.MaxDuration,
	}, nil
}

// Resume resumes a user's paused subscription.
func (s *Service) Resume(ctx context.Context, userID int64) error {
	atomicRepo, ok := s.pauseRepo.(repository.AtomicPauseRepository)
	if !ok {
		return fmt.Errorf("atomic pause repository is required")
	}
	pause, newExpiresAt, err := atomicRepo.ResumeSubscription(ctx, userID, time.Now())
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		return errors.NewValidationError("not_paused", "Subscription is not paused")
	}
	if err != nil {
		return errors.NewDatabaseError("failed to resume subscription", err)
	}

	s.notifyAccessChanged(ctx, userID)

	s.logger.Info("Subscription resumed",
		logger.F("user_id", userID),
		logger.F("new_expires_at", newExpiresAt),
		logger.F("remaining_days", pause.RemainingDays),
	)

	return nil
}

// GetActivePause returns the active pause for a user.
func (s *Service) GetActivePause(ctx context.Context, userID int64) (*repository.SubscriptionPause, error) {
	return s.pauseRepo.GetActivePause(ctx, userID)
}

// GetPauseHistory returns the pause history for a user.
func (s *Service) GetPauseHistory(ctx context.Context, userID int64, page, pageSize int) ([]*repository.SubscriptionPause, int64, error) {
	offset := (page - 1) * pageSize
	return s.pauseRepo.GetByUserID(ctx, userID, pageSize, offset)
}

// AutoResumePaused resumes all pauses that have reached their auto-resume time.
// This should be called by a cron job.
func (s *Service) AutoResumePaused(ctx context.Context) (int, error) {
	now := time.Now()
	pauses, err := s.pauseRepo.GetPausesToAutoResume(ctx, now)
	if err != nil {
		return 0, err
	}

	resumed := 0
	for _, pause := range pauses {
		if err := s.Resume(ctx, pause.UserID); err != nil {
			s.logger.Error("Failed to auto-resume subscription",
				logger.F("error", err),
				logger.F("user_id", pause.UserID),
				logger.F("pause_id", pause.ID),
			)
			continue
		}
		resumed++
	}

	if resumed > 0 {
		s.logger.Info("Auto-resumed subscriptions", logger.F("count", resumed))
	}

	return resumed, nil
}

// GetPauseStats returns pause statistics.
func (s *Service) GetPauseStats(ctx context.Context) (*repository.PauseStats, error) {
	return s.pauseRepo.GetPauseStats(ctx)
}
