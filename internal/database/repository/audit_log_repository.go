// Package repository provides data access implementations.
package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"v/pkg/errors"
)

// auditLogRepository implements AuditLogRepository.
type auditLogRepository struct {
	db *gorm.DB
}

// NewAuditLogRepository creates a new audit log repository.
func NewAuditLogRepository(db *gorm.DB) AuditLogRepository {
	return &auditLogRepository{db: db}
}

// Create creates a new audit log entry.
func (r *auditLogRepository) Create(ctx context.Context, log *AuditLog) error {
	result := r.db.WithContext(ctx).Create(log)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to create audit log", result.Error)
	}
	return nil
}

// List retrieves audit logs with pagination.
func (r *auditLogRepository) List(ctx context.Context, limit, offset int) ([]*AuditLog, error) {
	var logs []*AuditLog
	result := r.db.WithContext(ctx).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&logs)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to list audit logs", result.Error)
	}
	return logs, nil
}

// GetByUserID retrieves audit logs by user ID.
func (r *auditLogRepository) GetByUserID(ctx context.Context, userID int64, limit, offset int) ([]*AuditLog, error) {
	var logs []*AuditLog
	result := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&logs)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get audit logs by user", result.Error)
	}
	return logs, nil
}

// GetByAction retrieves audit logs by action.
func (r *auditLogRepository) GetByAction(ctx context.Context, action string, limit, offset int) ([]*AuditLog, error) {
	var logs []*AuditLog
	result := r.db.WithContext(ctx).
		Where("action = ?", action).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&logs)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get audit logs by action", result.Error)
	}
	return logs, nil
}

// GetByResourceType retrieves audit logs by resource type.
func (r *auditLogRepository) GetByResourceType(ctx context.Context, resourceType string, limit, offset int) ([]*AuditLog, error) {
	var logs []*AuditLog
	result := r.db.WithContext(ctx).
		Where("resource_type = ?", resourceType).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&logs)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get audit logs by resource type", result.Error)
	}
	return logs, nil
}

// GetByDateRange retrieves audit logs within a date range.
func (r *auditLogRepository) GetByDateRange(ctx context.Context, start, end time.Time, limit, offset int) ([]*AuditLog, error) {
	var logs []*AuditLog
	result := r.db.WithContext(ctx).
		Where("created_at BETWEEN ? AND ?", start, end).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&logs)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get audit logs by date range", result.Error)
	}
	return logs, nil
}

// Count returns the total number of audit logs.
func (r *auditLogRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	result := r.db.WithContext(ctx).Model(&AuditLog{}).Count(&count)
	if result.Error != nil {
		return 0, errors.NewDatabaseError("failed to count audit logs", result.Error)
	}
	return count, nil
}

// DeleteOlderThan deletes audit logs older than the specified time.
func (r *auditLogRepository) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("created_at < ?", before).
		Delete(&AuditLog{})
	if result.Error != nil {
		return 0, errors.NewDatabaseError("failed to delete old audit logs", result.Error)
	}
	return result.RowsAffected, nil
}
