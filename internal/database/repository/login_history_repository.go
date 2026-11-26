// Package repository provides data access interfaces and implementations.
package repository

import (
	"context"

	"gorm.io/gorm"
)

// loginHistoryRepository implements LoginHistoryRepository using GORM.
type loginHistoryRepository struct {
	db *gorm.DB
}

// NewLoginHistoryRepository creates a new LoginHistoryRepository.
func NewLoginHistoryRepository(db *gorm.DB) LoginHistoryRepository {
	return &loginHistoryRepository{db: db}
}

// Create creates a new login history record.
func (r *loginHistoryRepository) Create(ctx context.Context, history *LoginHistory) error {
	return r.db.WithContext(ctx).Create(history).Error
}

// GetByUserID returns login history for a user with pagination.
func (r *loginHistoryRepository) GetByUserID(ctx context.Context, userID int64, limit, offset int) ([]*LoginHistory, error) {
	var histories []*LoginHistory
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&histories).Error
	return histories, err
}

// DeleteByUserID deletes all login history for a user.
func (r *loginHistoryRepository) DeleteByUserID(ctx context.Context, userID int64) error {
	return r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&LoginHistory{}).Error
}

// Count returns the total number of login history records for a user.
func (r *loginHistoryRepository) Count(ctx context.Context, userID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&LoginHistory{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}
