// Package repository provides data access interfaces and implementations.
package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// Trial represents a trial subscription in the database.
type Trial struct {
	ID          int64      `gorm:"primaryKey;autoIncrement"`
	UserID      int64      `gorm:"uniqueIndex;not null"`
	Status      string     `gorm:"size:32;default:active;index"`
	StartAt     time.Time  `gorm:"not null"`
	ExpireAt    time.Time  `gorm:"not null;index"`
	TrafficUsed int64      `gorm:"default:0"`
	ConvertedAt *time.Time `gorm:""`
	CreatedAt   time.Time  `gorm:"autoCreateTime"`
}

// TableName returns the table name for Trial.
func (Trial) TableName() string {
	return "trials"
}

// TrialRepository defines the interface for trial data access.
type TrialRepository interface {
	Create(ctx context.Context, trial *Trial) error
	GetByID(ctx context.Context, id int64) (*Trial, error)
	GetByUserID(ctx context.Context, userID int64) (*Trial, error)
	Update(ctx context.Context, trial *Trial) error
	UpdateStatus(ctx context.Context, id int64, status string) error
	UpdateTrafficUsed(ctx context.Context, id int64, trafficUsed int64) error
	MarkConverted(ctx context.Context, userID int64) error
	ListExpired(ctx context.Context) ([]*Trial, error)
	ListActive(ctx context.Context) ([]*Trial, error)
	CountByStatus(ctx context.Context, status string) (int64, error)
	CountConverted(ctx context.Context) (int64, error)
	CountTotal(ctx context.Context) (int64, error)
	ExistsByUserID(ctx context.Context, userID int64) (bool, error)
}

// trialRepository implements TrialRepository.
type trialRepository struct {
	db *gorm.DB
}

// NewTrialRepository creates a new trial repository.
func NewTrialRepository(db *gorm.DB) TrialRepository {
	return &trialRepository{db: db}
}

// Create creates a new trial.
func (r *trialRepository) Create(ctx context.Context, trial *Trial) error {
	return r.db.WithContext(ctx).Create(trial).Error
}

// GetByID retrieves a trial by ID.
func (r *trialRepository) GetByID(ctx context.Context, id int64) (*Trial, error) {
	var trial Trial
	err := r.db.WithContext(ctx).First(&trial, id).Error
	if err != nil {
		return nil, err
	}
	return &trial, nil
}

// GetByUserID retrieves a trial by user ID.
func (r *trialRepository) GetByUserID(ctx context.Context, userID int64) (*Trial, error) {
	var trial Trial
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&trial).Error
	if err != nil {
		return nil, err
	}
	return &trial, nil
}

// Update updates a trial.
func (r *trialRepository) Update(ctx context.Context, trial *Trial) error {
	return r.db.WithContext(ctx).Save(trial).Error
}

// UpdateStatus updates the status of a trial.
func (r *trialRepository) UpdateStatus(ctx context.Context, id int64, status string) error {
	return r.db.WithContext(ctx).
		Model(&Trial{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// UpdateTrafficUsed updates the traffic used for a trial.
func (r *trialRepository) UpdateTrafficUsed(ctx context.Context, id int64, trafficUsed int64) error {
	return r.db.WithContext(ctx).
		Model(&Trial{}).
		Where("id = ?", id).
		Update("traffic_used", trafficUsed).Error
}

// MarkConverted marks a trial as converted.
func (r *trialRepository) MarkConverted(ctx context.Context, userID int64) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&Trial{}).
		Where("user_id = ?", userID).
		Updates(map[string]interface{}{
			"status":       "converted",
			"converted_at": now,
		}).Error
}

// ListExpired lists all expired trials that are still marked as active.
func (r *trialRepository) ListExpired(ctx context.Context) ([]*Trial, error) {
	var trials []*Trial
	err := r.db.WithContext(ctx).
		Where("status = ? AND expire_at < ?", "active", time.Now()).
		Find(&trials).Error
	return trials, err
}

// ListActive lists all active trials.
func (r *trialRepository) ListActive(ctx context.Context) ([]*Trial, error) {
	var trials []*Trial
	err := r.db.WithContext(ctx).
		Where("status = ?", "active").
		Find(&trials).Error
	return trials, err
}

// CountByStatus counts trials by status.
func (r *trialRepository) CountByStatus(ctx context.Context, status string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&Trial{}).
		Where("status = ?", status).
		Count(&count).Error
	return count, err
}

// CountConverted counts converted trials.
func (r *trialRepository) CountConverted(ctx context.Context) (int64, error) {
	return r.CountByStatus(ctx, "converted")
}

// CountTotal counts all trials.
func (r *trialRepository) CountTotal(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&Trial{}).Count(&count).Error
	return count, err
}

// ExistsByUserID checks if a trial exists for a user.
func (r *trialRepository) ExistsByUserID(ctx context.Context, userID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&Trial{}).
		Where("user_id = ?", userID).
		Count(&count).Error
	return count > 0, err
}
