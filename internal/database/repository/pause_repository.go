// Package repository provides data access implementations.
package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"v/pkg/errors"
)

// SubscriptionPause represents a subscription pause record in the database.
type SubscriptionPause struct {
	ID               int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID           int64      `gorm:"index;not null" json:"user_id"`
	PausedAt         time.Time  `gorm:"not null" json:"paused_at"`
	ResumedAt        *time.Time `gorm:"" json:"resumed_at"`
	RemainingDays    int        `gorm:"not null" json:"remaining_days"`
	RemainingTraffic int64      `gorm:"not null" json:"remaining_traffic"`
	AutoResumeAt     time.Time  `gorm:"not null;index" json:"auto_resume_at"`
	CreatedAt        time.Time  `gorm:"autoCreateTime" json:"created_at"`

	User *User `gorm:"foreignKey:UserID" json:"-"`
}

// TableName returns the table name for SubscriptionPause.
func (SubscriptionPause) TableName() string {
	return "subscription_pauses"
}

// PauseFilter represents filter options for listing pauses.
type PauseFilter struct {
	UserID         *int64
	ActiveOnly     bool
	AutoResumeBefore *time.Time
	Limit          int
	Offset         int
}

// PauseRepository defines the interface for subscription pause data access.
type PauseRepository interface {
	// Create creates a new pause record.
	Create(ctx context.Context, pause *SubscriptionPause) error

	// GetByID retrieves a pause by its ID.
	GetByID(ctx context.Context, id int64) (*SubscriptionPause, error)

	// GetActivePause retrieves the active (not resumed) pause for a user.
	GetActivePause(ctx context.Context, userID int64) (*SubscriptionPause, error)

	// GetByUserID retrieves all pauses for a user.
	GetByUserID(ctx context.Context, userID int64, limit, offset int) ([]*SubscriptionPause, int64, error)

	// Update updates an existing pause record.
	Update(ctx context.Context, pause *SubscriptionPause) error

	// CountPausesInPeriod counts the number of pauses for a user within a time period.
	CountPausesInPeriod(ctx context.Context, userID int64, start, end time.Time) (int, error)

	// GetPausesToAutoResume retrieves pauses that need to be auto-resumed.
	GetPausesToAutoResume(ctx context.Context, before time.Time) ([]*SubscriptionPause, error)

	// GetPauseStats retrieves pause statistics.
	GetPauseStats(ctx context.Context) (*PauseStats, error)
}

// PauseStats represents pause statistics.
type PauseStats struct {
	TotalPauses       int64 `json:"total_pauses"`
	ActivePauses      int64 `json:"active_pauses"`
	ResumedPauses     int64 `json:"resumed_pauses"`
	AvgPauseDuration  float64 `json:"avg_pause_duration_days"`
}

// pauseRepository implements PauseRepository.
type pauseRepository struct {
	db *gorm.DB
}

// NewPauseRepository creates a new pause repository.
func NewPauseRepository(db *gorm.DB) PauseRepository {
	return &pauseRepository{db: db}
}

// Create creates a new pause record.
func (r *pauseRepository) Create(ctx context.Context, pause *SubscriptionPause) error {
	result := r.db.WithContext(ctx).Create(pause)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to create pause", result.Error)
	}
	return nil
}

// GetByID retrieves a pause by its ID.
func (r *pauseRepository) GetByID(ctx context.Context, id int64) (*SubscriptionPause, error) {
	var pause SubscriptionPause
	result := r.db.WithContext(ctx).First(&pause, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.NewNotFoundError("pause", id)
		}
		return nil, errors.NewDatabaseError("failed to get pause", result.Error)
	}
	return &pause, nil
}

// GetActivePause retrieves the active (not resumed) pause for a user.
func (r *pauseRepository) GetActivePause(ctx context.Context, userID int64) (*SubscriptionPause, error) {
	var pause SubscriptionPause
	result := r.db.WithContext(ctx).
		Where("user_id = ? AND resumed_at IS NULL", userID).
		Order("paused_at DESC").
		First(&pause)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil // No active pause is not an error
		}
		return nil, errors.NewDatabaseError("failed to get active pause", result.Error)
	}
	return &pause, nil
}

// GetByUserID retrieves all pauses for a user.
func (r *pauseRepository) GetByUserID(ctx context.Context, userID int64, limit, offset int) ([]*SubscriptionPause, int64, error) {
	var pauses []*SubscriptionPause
	var total int64

	query := r.db.WithContext(ctx).Model(&SubscriptionPause{}).Where("user_id = ?", userID)

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errors.NewDatabaseError("failed to count pauses", err)
	}

	// Fetch results
	if err := query.Order("paused_at DESC").Limit(limit).Offset(offset).Find(&pauses).Error; err != nil {
		return nil, 0, errors.NewDatabaseError("failed to list pauses", err)
	}

	return pauses, total, nil
}

// Update updates an existing pause record.
func (r *pauseRepository) Update(ctx context.Context, pause *SubscriptionPause) error {
	result := r.db.WithContext(ctx).Save(pause)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to update pause", result.Error)
	}
	return nil
}

// CountPausesInPeriod counts the number of pauses for a user within a time period.
func (r *pauseRepository) CountPausesInPeriod(ctx context.Context, userID int64, start, end time.Time) (int, error) {
	var count int64
	result := r.db.WithContext(ctx).Model(&SubscriptionPause{}).
		Where("user_id = ? AND paused_at >= ? AND paused_at <= ?", userID, start, end).
		Count(&count)
	if result.Error != nil {
		return 0, errors.NewDatabaseError("failed to count pauses in period", result.Error)
	}
	return int(count), nil
}

// GetPausesToAutoResume retrieves pauses that need to be auto-resumed.
func (r *pauseRepository) GetPausesToAutoResume(ctx context.Context, before time.Time) ([]*SubscriptionPause, error) {
	var pauses []*SubscriptionPause
	result := r.db.WithContext(ctx).
		Where("resumed_at IS NULL AND auto_resume_at <= ?", before).
		Find(&pauses)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get pauses to auto-resume", result.Error)
	}
	return pauses, nil
}

// GetPauseStats retrieves pause statistics.
func (r *pauseRepository) GetPauseStats(ctx context.Context) (*PauseStats, error) {
	stats := &PauseStats{}

	// Total pauses
	if err := r.db.WithContext(ctx).Model(&SubscriptionPause{}).Count(&stats.TotalPauses).Error; err != nil {
		return nil, errors.NewDatabaseError("failed to count total pauses", err)
	}

	// Active pauses
	if err := r.db.WithContext(ctx).Model(&SubscriptionPause{}).
		Where("resumed_at IS NULL").
		Count(&stats.ActivePauses).Error; err != nil {
		return nil, errors.NewDatabaseError("failed to count active pauses", err)
	}

	// Resumed pauses
	stats.ResumedPauses = stats.TotalPauses - stats.ActivePauses

	// Average pause duration (for resumed pauses)
	var avgDuration struct {
		AvgDays float64
	}
	r.db.WithContext(ctx).Model(&SubscriptionPause{}).
		Select("AVG(JULIANDAY(resumed_at) - JULIANDAY(paused_at)) as avg_days").
		Where("resumed_at IS NOT NULL").
		Scan(&avgDuration)
	stats.AvgPauseDuration = avgDuration.AvgDays

	return stats, nil
}
