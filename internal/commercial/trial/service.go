// Package trial provides trial subscription management functionality.
package trial

import (
	"context"
	"errors"
	"math"
	"time"

	"v/internal/database/repository"
	"v/internal/logger"
)

// Common errors
var (
	ErrTrialNotFound      = errors.New("trial not found")
	ErrTrialAlreadyUsed   = errors.New("user has already used trial")
	ErrTrialExpired       = errors.New("trial has expired")
	ErrTrialNotActive     = errors.New("trial is not active")
	ErrTrialDisabled      = errors.New("trial feature is disabled")
	ErrEmailNotVerified   = errors.New("email verification required for trial")
	ErrActiveSubscription = errors.New("user already has an active subscription")
)

// Config holds trial configuration.
type Config struct {
	Enabled             bool     `json:"enabled"`
	Duration            int      `json:"duration"`      // days
	TrafficLimit        int64    `json:"traffic_limit"` // bytes
	RequireEmailVerify  bool     `json:"require_email_verify"`
	AutoActivate        bool     `json:"auto_activate"` // on registration
	FeatureRestrictions []string `json:"feature_restrictions"`
}

// DefaultConfig returns the default trial configuration.
func DefaultConfig() *Config {
	return &Config{
		Enabled:             true,
		Duration:            7,            // 7 days
		TrafficLimit:        107374182400, // 100 GB
		RequireEmailVerify:  false,
		AutoActivate:        true,
		FeatureRestrictions: []string{},
	}
}

// Trial represents a trial subscription.
type Trial struct {
	ID          int64      `json:"id"`
	UserID      int64      `json:"user_id"`
	Status      string     `json:"status"`
	StartAt     time.Time  `json:"start_at"`
	ExpireAt    time.Time  `json:"expire_at"`
	TrafficUsed int64      `json:"traffic_used"`
	ConvertedAt *time.Time `json:"converted_at"`
	CreatedAt   time.Time  `json:"created_at"`
	// Computed fields
	RemainingDays    int   `json:"remaining_days"`
	RemainingTraffic int64 `json:"remaining_traffic"`
	TrafficLimit     int64 `json:"traffic_limit"`
}

// TrialStats represents trial statistics.
type TrialStats struct {
	TotalTrials     int64   `json:"total_trials"`
	ActiveTrials    int64   `json:"active_trials"`
	ExpiredTrials   int64   `json:"expired_trials"`
	ConvertedTrials int64   `json:"converted_trials"`
	ConversionRate  float64 `json:"conversion_rate"`
}

// Service provides trial management operations.
type Service struct {
	trialRepo repository.TrialRepository
	userRepo  repository.UserRepository
	config    *Config
	logger    logger.Logger
}

// NewService creates a new trial service.
func NewService(trialRepo repository.TrialRepository, userRepo repository.UserRepository, log logger.Logger, config *Config) *Service {
	if config == nil {
		config = DefaultConfig()
	}
	return &Service{
		trialRepo: trialRepo,
		userRepo:  userRepo,
		config:    config,
		logger:    log,
	}
}

// GetConfig returns the current trial configuration.
func (s *Service) GetConfig() *Config {
	return s.config
}

// UpdateConfig updates the trial configuration.
func (s *Service) UpdateConfig(config *Config) {
	s.config = config
}

// ActivateTrial activates a trial for a user.
func (s *Service) ActivateTrial(ctx context.Context, userID int64) (*Trial, error) {
	if !s.config.Enabled {
		return nil, ErrTrialDisabled
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get user", logger.Err(err), logger.F("user_id", userID))
		return nil, err
	}
	if s.userHasActiveSubscription(user, time.Now()) {
		return nil, ErrActiveSubscription
	}

	// Check if user has already used trial
	exists, err := s.trialRepo.ExistsByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to check trial existence", logger.Err(err), logger.F("user_id", userID))
		return nil, err
	}
	if exists {
		return nil, ErrTrialAlreadyUsed
	}

	// Check email verification if required
	if s.config.RequireEmailVerify && !user.EmailVerified {
		return nil, ErrEmailNotVerified
	}

	// Create trial
	now := time.Now()
	expireAt := now.AddDate(0, 0, s.config.Duration)

	repoTrial := &repository.Trial{
		UserID:      userID,
		Status:      "active",
		StartAt:     now,
		ExpireAt:    expireAt,
		TrafficUsed: 0,
	}

	if err := s.trialRepo.Create(ctx, repoTrial); err != nil {
		s.logger.Error("Failed to create trial", logger.Err(err), logger.F("user_id", userID))
		return nil, err
	}

	s.logger.Info("Trial activated", logger.F("user_id", userID), logger.F("trial_id", repoTrial.ID))

	return s.toTrial(repoTrial), nil
}

// GetTrial retrieves a trial by user ID.
func (s *Service) GetTrial(ctx context.Context, userID int64) (*Trial, error) {
	repoTrial, err := s.trialRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, ErrTrialNotFound
	}
	repoTrial = s.normalizeTrialState(ctx, repoTrial)
	return s.toTrial(repoTrial), nil
}

// GetTrialByID retrieves a trial by ID.
func (s *Service) GetTrialByID(ctx context.Context, id int64) (*Trial, error) {
	repoTrial, err := s.trialRepo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrTrialNotFound
	}
	repoTrial = s.normalizeTrialState(ctx, repoTrial)
	return s.toTrial(repoTrial), nil
}

// HasUsedTrial checks if a user has already used their trial.
func (s *Service) HasUsedTrial(ctx context.Context, userID int64) bool {
	exists, err := s.trialRepo.ExistsByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to check trial existence", logger.Err(err), logger.F("user_id", userID))
		return false
	}
	return exists
}

// IsTrialActive checks if a user has an active trial.
func (s *Service) IsTrialActive(ctx context.Context, userID int64) bool {
	trial, err := s.trialRepo.GetByUserID(ctx, userID)
	if err != nil {
		return false
	}
	return trial.Status == "active" && time.Now().Before(trial.ExpireAt)
}

// UpdateTrafficUsed updates the traffic used for a trial.
func (s *Service) UpdateTrafficUsed(ctx context.Context, userID int64, trafficUsed int64) error {
	trial, err := s.trialRepo.GetByUserID(ctx, userID)
	if err != nil {
		return ErrTrialNotFound
	}

	if trial.Status != "active" {
		return ErrTrialNotActive
	}

	if err := s.trialRepo.UpdateTrafficUsed(ctx, trial.ID, trafficUsed); err != nil {
		s.logger.Error("Failed to update trial traffic", logger.Err(err), logger.F("trial_id", trial.ID))
		return err
	}

	return nil
}

// MarkConverted marks a trial as converted (user purchased a plan).
func (s *Service) MarkConverted(ctx context.Context, userID int64) error {
	trial, err := s.trialRepo.GetByUserID(ctx, userID)
	if err != nil {
		return ErrTrialNotFound
	}

	if trial.Status == "converted" {
		return nil // Already converted
	}

	if err := s.trialRepo.MarkConverted(ctx, userID); err != nil {
		s.logger.Error("Failed to mark trial as converted", logger.Err(err), logger.F("user_id", userID))
		return err
	}

	s.logger.Info("Trial marked as converted", logger.F("user_id", userID), logger.F("trial_id", trial.ID))
	return nil
}

// ExpireTrials expires all trials that have passed their expiration date.
// This should be called by a cron job.
func (s *Service) ExpireTrials(ctx context.Context) (int, error) {
	expiredTrials, err := s.trialRepo.ListExpired(ctx)
	if err != nil {
		s.logger.Error("Failed to list expired trials", logger.Err(err))
		return 0, err
	}

	count := 0
	for _, trial := range expiredTrials {
		if err := s.trialRepo.UpdateStatus(ctx, trial.ID, "expired"); err != nil {
			s.logger.Error("Failed to expire trial", logger.Err(err), logger.F("trial_id", trial.ID))
			continue
		}
		count++
	}

	if count > 0 {
		s.logger.Info("Expired trials", logger.F("count", count))
	}

	return count, nil
}

// GetStats returns trial statistics.
func (s *Service) GetStats(ctx context.Context) (*TrialStats, error) {
	total, err := s.trialRepo.CountTotal(ctx)
	if err != nil {
		s.logger.Error("Failed to count total trials", logger.Err(err))
		return nil, err
	}

	active, err := s.trialRepo.CountByStatus(ctx, "active")
	if err != nil {
		s.logger.Error("Failed to count active trials", logger.Err(err))
		return nil, err
	}

	expired, err := s.trialRepo.CountByStatus(ctx, "expired")
	if err != nil {
		s.logger.Error("Failed to count expired trials", logger.Err(err))
		return nil, err
	}

	converted, err := s.trialRepo.CountConverted(ctx)
	if err != nil {
		s.logger.Error("Failed to count converted trials", logger.Err(err))
		return nil, err
	}

	var conversionRate float64
	if total > 0 {
		conversionRate = float64(converted) / float64(total) * 100
	}

	return &TrialStats{
		TotalTrials:     total,
		ActiveTrials:    active,
		ExpiredTrials:   expired,
		ConvertedTrials: converted,
		ConversionRate:  conversionRate,
	}, nil
}

// GetConversionRate returns the trial to paid conversion rate.
func (s *Service) GetConversionRate(ctx context.Context) (float64, error) {
	stats, err := s.GetStats(ctx)
	if err != nil {
		return 0, err
	}
	return stats.ConversionRate, nil
}

// CanActivateTrial checks if a user can activate a trial.
func (s *Service) CanActivateTrial(ctx context.Context, userID int64) (bool, string) {
	if !s.config.Enabled {
		return false, "Trial feature is disabled"
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return false, "Failed to get user information"
	}
	if s.userHasActiveSubscription(user, time.Now()) {
		return false, "当前已有有效订阅，无需试用"
	}

	exists, err := s.trialRepo.ExistsByUserID(ctx, userID)
	if err != nil {
		return false, "Failed to check trial status"
	}
	if exists {
		return false, "You have already used your trial"
	}

	if s.config.RequireEmailVerify && !user.EmailVerified {
		return false, "Email verification required"
	}

	return true, ""
}

// GrantTrial grants a trial to a specific user (admin function).
// This bypasses the normal checks and allows granting trial even if already used.
func (s *Service) GrantTrial(ctx context.Context, userID int64, durationDays int) (*Trial, error) {
	// Check if user already has an active trial
	existingTrial, err := s.trialRepo.GetByUserID(ctx, userID)
	if err == nil && existingTrial.Status == "active" {
		// Extend the existing trial
		newExpireAt := existingTrial.ExpireAt.AddDate(0, 0, durationDays)
		existingTrial.ExpireAt = newExpireAt
		if err := s.trialRepo.Update(ctx, existingTrial); err != nil {
			s.logger.Error("Failed to extend trial", logger.Err(err), logger.F("trial_id", existingTrial.ID))
			return nil, err
		}
		s.logger.Info("Trial extended", logger.F("user_id", userID), logger.F("trial_id", existingTrial.ID), logger.F("days", durationDays))
		return s.toTrial(existingTrial), nil
	}

	// Create new trial
	now := time.Now()
	expireAt := now.AddDate(0, 0, durationDays)

	repoTrial := &repository.Trial{
		UserID:      userID,
		Status:      "active",
		StartAt:     now,
		ExpireAt:    expireAt,
		TrafficUsed: 0,
	}

	if err := s.trialRepo.Create(ctx, repoTrial); err != nil {
		s.logger.Error("Failed to grant trial", logger.Err(err), logger.F("user_id", userID))
		return nil, err
	}

	s.logger.Info("Trial granted", logger.F("user_id", userID), logger.F("trial_id", repoTrial.ID), logger.F("days", durationDays))
	return s.toTrial(repoTrial), nil
}

// toTrial converts a repository trial to a service trial.
func (s *Service) normalizeTrialState(ctx context.Context, repoTrial *repository.Trial) *repository.Trial {
	if repoTrial == nil || repoTrial.Status != "active" {
		return repoTrial
	}

	now := time.Now()
	if user, err := s.userRepo.GetByID(ctx, repoTrial.UserID); err == nil {
		if s.userHasActiveSubscription(user, now) {
			if err := s.trialRepo.MarkConverted(ctx, repoTrial.UserID); err != nil {
				s.logger.Warn("failed to mark active trial converted during status normalization", logger.Err(err), logger.F("trial_id", repoTrial.ID), logger.F("user_id", repoTrial.UserID))
			}
			repoTrial.Status = "converted"
			convertedAt := now
			repoTrial.ConvertedAt = &convertedAt
			return repoTrial
		}
	} else {
		s.logger.Warn("failed to load user during trial normalization", logger.Err(err), logger.F("trial_id", repoTrial.ID), logger.F("user_id", repoTrial.UserID))
	}

	if !now.After(repoTrial.ExpireAt) {
		return repoTrial
	}

	if err := s.trialRepo.UpdateStatus(ctx, repoTrial.ID, "expired"); err != nil {
		s.logger.Warn("failed to mark expired trial during status normalization", logger.Err(err), logger.F("trial_id", repoTrial.ID), logger.F("user_id", repoTrial.UserID))
	}
	repoTrial.Status = "expired"
	return repoTrial
}

func (s *Service) userHasActiveSubscription(user *repository.User, now time.Time) bool {
	if user == nil || !user.Enabled {
		return false
	}
	if user.ExpiresAt != nil && !now.After(*user.ExpiresAt) {
		return true
	}
	if user.ExpiresAt == nil && user.TrafficLimit > 0 {
		return true
	}
	return false
}

func (s *Service) toTrial(rt *repository.Trial) *Trial {
	trial := &Trial{
		ID:           rt.ID,
		UserID:       rt.UserID,
		Status:       rt.Status,
		StartAt:      rt.StartAt,
		ExpireAt:     rt.ExpireAt,
		TrafficUsed:  rt.TrafficUsed,
		ConvertedAt:  rt.ConvertedAt,
		CreatedAt:    rt.CreatedAt,
		TrafficLimit: s.config.TrafficLimit,
	}

	// Calculate remaining days
	if rt.Status == "active" && time.Now().Before(rt.ExpireAt) {
		remaining := rt.ExpireAt.Sub(time.Now())
		trial.RemainingDays = int(math.Ceil(remaining.Hours() / 24))
		if trial.RemainingDays < 1 {
			trial.RemainingDays = 1
		}
	}

	// Calculate remaining traffic
	trial.RemainingTraffic = s.config.TrafficLimit - rt.TrafficUsed
	if trial.RemainingTraffic < 0 {
		trial.RemainingTraffic = 0
	}

	return trial
}
