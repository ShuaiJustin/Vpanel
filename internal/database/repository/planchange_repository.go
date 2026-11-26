// Package repository provides data access implementations.
package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"v/pkg/errors"
)

// PendingDowngrade represents a scheduled plan downgrade in the database.
type PendingDowngrade struct {
	ID            int64     `gorm:"primaryKey;autoIncrement"`
	UserID        int64     `gorm:"uniqueIndex;not null"`
	CurrentPlanID int64     `gorm:"not null"`
	NewPlanID     int64     `gorm:"not null"`
	EffectiveAt   time.Time `gorm:"not null;index"`
	CreatedAt     time.Time `gorm:"autoCreateTime"`

	// Relations
	User        *User           `gorm:"foreignKey:UserID"`
	CurrentPlan *CommercialPlan `gorm:"foreignKey:CurrentPlanID"`
	NewPlan     *CommercialPlan `gorm:"foreignKey:NewPlanID"`
}

// TableName returns the table name for PendingDowngrade.
func (PendingDowngrade) TableName() string {
	return "pending_downgrades"
}

// PlanChangeRepository defines the interface for plan change data access.
type PlanChangeRepository interface {
	// CreatePendingDowngrade creates a new pending downgrade record.
	CreatePendingDowngrade(ctx context.Context, downgrade *PendingDowngrade) error

	// GetPendingDowngradeByUserID retrieves a pending downgrade by user ID.
	GetPendingDowngradeByUserID(ctx context.Context, userID int64) (*PendingDowngrade, error)

	// DeletePendingDowngrade deletes a pending downgrade by ID.
	DeletePendingDowngrade(ctx context.Context, id int64) error

	// DeletePendingDowngradeByUserID deletes a pending downgrade by user ID.
	DeletePendingDowngradeByUserID(ctx context.Context, userID int64) error

	// ListDueDowngrades lists all pending downgrades that are due for execution.
	ListDueDowngrades(ctx context.Context) ([]*PendingDowngrade, error)

	// ListAllPendingDowngrades lists all pending downgrades with pagination.
	ListAllPendingDowngrades(ctx context.Context, limit, offset int) ([]*PendingDowngrade, int64, error)
}

// planChangeRepository implements PlanChangeRepository.
type planChangeRepository struct {
	db *gorm.DB
}

// NewPlanChangeRepository creates a new plan change repository.
func NewPlanChangeRepository(db *gorm.DB) PlanChangeRepository {
	return &planChangeRepository{db: db}
}

// CreatePendingDowngrade creates a new pending downgrade record.
func (r *planChangeRepository) CreatePendingDowngrade(ctx context.Context, downgrade *PendingDowngrade) error {
	result := r.db.WithContext(ctx).Create(downgrade)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to create pending downgrade", result.Error)
	}
	return nil
}

// GetPendingDowngradeByUserID retrieves a pending downgrade by user ID.
func (r *planChangeRepository) GetPendingDowngradeByUserID(ctx context.Context, userID int64) (*PendingDowngrade, error) {
	var downgrade PendingDowngrade
	result := r.db.WithContext(ctx).
		Preload("CurrentPlan").
		Preload("NewPlan").
		Where("user_id = ?", userID).
		First(&downgrade)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.NewNotFoundError("pending downgrade", userID)
		}
		return nil, errors.NewDatabaseError("failed to get pending downgrade", result.Error)
	}
	return &downgrade, nil
}

// DeletePendingDowngrade deletes a pending downgrade by ID.
func (r *planChangeRepository) DeletePendingDowngrade(ctx context.Context, id int64) error {
	result := r.db.WithContext(ctx).Delete(&PendingDowngrade{}, id)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to delete pending downgrade", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("pending downgrade", id)
	}
	return nil
}

// DeletePendingDowngradeByUserID deletes a pending downgrade by user ID.
func (r *planChangeRepository) DeletePendingDowngradeByUserID(ctx context.Context, userID int64) error {
	result := r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&PendingDowngrade{})
	if result.Error != nil {
		return errors.NewDatabaseError("failed to delete pending downgrade by user ID", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("pending downgrade", userID)
	}
	return nil
}

// ListDueDowngrades lists all pending downgrades that are due for execution.
func (r *planChangeRepository) ListDueDowngrades(ctx context.Context) ([]*PendingDowngrade, error) {
	var downgrades []*PendingDowngrade
	result := r.db.WithContext(ctx).
		Preload("User").
		Preload("CurrentPlan").
		Preload("NewPlan").
		Where("effective_at <= ?", time.Now()).
		Find(&downgrades)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to list due downgrades", result.Error)
	}
	return downgrades, nil
}

// ListAllPendingDowngrades lists all pending downgrades with pagination.
func (r *planChangeRepository) ListAllPendingDowngrades(ctx context.Context, limit, offset int) ([]*PendingDowngrade, int64, error) {
	var downgrades []*PendingDowngrade
	var total int64

	// Count total
	if err := r.db.WithContext(ctx).Model(&PendingDowngrade{}).Count(&total).Error; err != nil {
		return nil, 0, errors.NewDatabaseError("failed to count pending downgrades", err)
	}

	// Fetch with pagination
	result := r.db.WithContext(ctx).
		Preload("User").
		Preload("CurrentPlan").
		Preload("NewPlan").
		Order("effective_at ASC").
		Limit(limit).
		Offset(offset).
		Find(&downgrades)
	if result.Error != nil {
		return nil, 0, errors.NewDatabaseError("failed to list pending downgrades", result.Error)
	}

	return downgrades, total, nil
}
