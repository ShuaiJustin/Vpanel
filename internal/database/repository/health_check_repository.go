// Package repository provides data access implementations.
package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"v/pkg/errors"
)

// HealthCheck represents a health check record for a node.
type HealthCheck struct {
	ID        int64     `gorm:"primaryKey;autoIncrement"`
	NodeID    int64     `gorm:"index;not null"`
	Status    string    `gorm:"size:32"` // success, failed
	Latency   int       `gorm:""`        // milliseconds
	Message   string    `gorm:"size:512"`
	TCPOk     bool      `gorm:"default:false"`
	APIOk     bool      `gorm:"default:false"`
	XrayOk    bool      `gorm:"default:false"`
	CheckedAt time.Time `gorm:"index"`

	Node *Node `gorm:"foreignKey:NodeID"`
}

// TableName returns the table name for HealthCheck.
func (HealthCheck) TableName() string {
	return "health_checks"
}

// HealthCheckStatus constants
const (
	HealthCheckStatusSuccess = "success"
	HealthCheckStatusFailed  = "failed"
)

// HealthCheckRepository defines the interface for health check data access.
type HealthCheckRepository interface {
	// CRUD operations
	Create(ctx context.Context, check *HealthCheck) error
	GetByID(ctx context.Context, id int64) (*HealthCheck, error)
	Delete(ctx context.Context, id int64) error

	// Query operations
	GetByNodeID(ctx context.Context, nodeID int64, limit int) ([]*HealthCheck, error)
	GetLatestByNodeID(ctx context.Context, nodeID int64) (*HealthCheck, error)
	GetByDateRange(ctx context.Context, nodeID int64, start, end time.Time) ([]*HealthCheck, error)
	GetRecentFailures(ctx context.Context, nodeID int64, count int) ([]*HealthCheck, error)
	GetRecentSuccesses(ctx context.Context, nodeID int64, count int) ([]*HealthCheck, error)

	// Statistics
	CountByStatus(ctx context.Context, nodeID int64, status string, since time.Time) (int64, error)
	GetAverageLatency(ctx context.Context, nodeID int64, since time.Time) (float64, error)
	GetConsecutiveFailures(ctx context.Context, nodeID int64) (int, error)
	GetConsecutiveSuccesses(ctx context.Context, nodeID int64) (int, error)

	// Cleanup
	DeleteOlderThan(ctx context.Context, before time.Time) (int64, error)
	DeleteByNodeID(ctx context.Context, nodeID int64) error
}

// healthCheckRepository implements HealthCheckRepository.
type healthCheckRepository struct {
	db *gorm.DB
}

// NewHealthCheckRepository creates a new health check repository.
func NewHealthCheckRepository(db *gorm.DB) HealthCheckRepository {
	return &healthCheckRepository{db: db}
}

// Create creates a new health check record.
func (r *healthCheckRepository) Create(ctx context.Context, check *HealthCheck) error {
	result := r.db.WithContext(ctx).Create(check)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to create health check", result.Error)
	}
	return nil
}

// GetByID retrieves a health check by ID.
func (r *healthCheckRepository) GetByID(ctx context.Context, id int64) (*HealthCheck, error) {
	var check HealthCheck
	result := r.db.WithContext(ctx).First(&check, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.NewNotFoundError("health check", id)
		}
		return nil, errors.NewDatabaseError("failed to get health check", result.Error)
	}
	return &check, nil
}

// Delete deletes a health check by ID.
func (r *healthCheckRepository) Delete(ctx context.Context, id int64) error {
	result := r.db.WithContext(ctx).Delete(&HealthCheck{}, id)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to delete health check", result.Error)
	}
	return nil
}

// GetByNodeID retrieves health checks for a node with limit.
func (r *healthCheckRepository) GetByNodeID(ctx context.Context, nodeID int64, limit int) ([]*HealthCheck, error) {
	var checks []*HealthCheck
	query := r.db.WithContext(ctx).
		Where("node_id = ?", nodeID).
		Order("checked_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	result := query.Find(&checks)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get health checks by node ID", result.Error)
	}
	return checks, nil
}

// GetLatestByNodeID retrieves the most recent health check for a node.
func (r *healthCheckRepository) GetLatestByNodeID(ctx context.Context, nodeID int64) (*HealthCheck, error) {
	var check HealthCheck
	result := r.db.WithContext(ctx).
		Where("node_id = ?", nodeID).
		Order("checked_at DESC").
		First(&check)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil // No health checks yet
		}
		return nil, errors.NewDatabaseError("failed to get latest health check", result.Error)
	}
	return &check, nil
}

// GetByDateRange retrieves health checks for a node within a date range.
func (r *healthCheckRepository) GetByDateRange(ctx context.Context, nodeID int64, start, end time.Time) ([]*HealthCheck, error) {
	var checks []*HealthCheck
	result := r.db.WithContext(ctx).
		Where("node_id = ? AND checked_at >= ? AND checked_at <= ?", nodeID, start, end).
		Order("checked_at DESC").
		Find(&checks)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get health checks by date range", result.Error)
	}
	return checks, nil
}

// GetRecentFailures retrieves recent failed health checks for a node.
func (r *healthCheckRepository) GetRecentFailures(ctx context.Context, nodeID int64, count int) ([]*HealthCheck, error) {
	var checks []*HealthCheck
	result := r.db.WithContext(ctx).
		Where("node_id = ? AND status = ?", nodeID, HealthCheckStatusFailed).
		Order("checked_at DESC").
		Limit(count).
		Find(&checks)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get recent failures", result.Error)
	}
	return checks, nil
}

// GetRecentSuccesses retrieves recent successful health checks for a node.
func (r *healthCheckRepository) GetRecentSuccesses(ctx context.Context, nodeID int64, count int) ([]*HealthCheck, error) {
	var checks []*HealthCheck
	result := r.db.WithContext(ctx).
		Where("node_id = ? AND status = ?", nodeID, HealthCheckStatusSuccess).
		Order("checked_at DESC").
		Limit(count).
		Find(&checks)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get recent successes", result.Error)
	}
	return checks, nil
}


// CountByStatus counts health checks by status since a given time.
func (r *healthCheckRepository) CountByStatus(ctx context.Context, nodeID int64, status string, since time.Time) (int64, error) {
	var count int64
	result := r.db.WithContext(ctx).
		Model(&HealthCheck{}).
		Where("node_id = ? AND status = ? AND checked_at >= ?", nodeID, status, since).
		Count(&count)
	if result.Error != nil {
		return 0, errors.NewDatabaseError("failed to count health checks by status", result.Error)
	}
	return count, nil
}

// GetAverageLatency calculates the average latency for a node since a given time.
func (r *healthCheckRepository) GetAverageLatency(ctx context.Context, nodeID int64, since time.Time) (float64, error) {
	var avg float64
	result := r.db.WithContext(ctx).
		Model(&HealthCheck{}).
		Select("COALESCE(AVG(latency), 0)").
		Where("node_id = ? AND status = ? AND checked_at >= ?", nodeID, HealthCheckStatusSuccess, since).
		Scan(&avg)
	if result.Error != nil {
		return 0, errors.NewDatabaseError("failed to get average latency", result.Error)
	}
	return avg, nil
}

// GetConsecutiveFailures returns the number of consecutive failures for a node.
func (r *healthCheckRepository) GetConsecutiveFailures(ctx context.Context, nodeID int64) (int, error) {
	var checks []*HealthCheck
	result := r.db.WithContext(ctx).
		Where("node_id = ?", nodeID).
		Order("checked_at DESC").
		Limit(100). // Reasonable limit
		Find(&checks)
	if result.Error != nil {
		return 0, errors.NewDatabaseError("failed to get health checks", result.Error)
	}

	count := 0
	for _, check := range checks {
		if check.Status == HealthCheckStatusFailed {
			count++
		} else {
			break
		}
	}
	return count, nil
}

// GetConsecutiveSuccesses returns the number of consecutive successes for a node.
func (r *healthCheckRepository) GetConsecutiveSuccesses(ctx context.Context, nodeID int64) (int, error) {
	var checks []*HealthCheck
	result := r.db.WithContext(ctx).
		Where("node_id = ?", nodeID).
		Order("checked_at DESC").
		Limit(100). // Reasonable limit
		Find(&checks)
	if result.Error != nil {
		return 0, errors.NewDatabaseError("failed to get health checks", result.Error)
	}

	count := 0
	for _, check := range checks {
		if check.Status == HealthCheckStatusSuccess {
			count++
		} else {
			break
		}
	}
	return count, nil
}

// DeleteOlderThan deletes health checks older than the specified time.
func (r *healthCheckRepository) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("checked_at < ?", before).
		Delete(&HealthCheck{})
	if result.Error != nil {
		return 0, errors.NewDatabaseError("failed to delete old health checks", result.Error)
	}
	return result.RowsAffected, nil
}

// DeleteByNodeID deletes all health checks for a node.
func (r *healthCheckRepository) DeleteByNodeID(ctx context.Context, nodeID int64) error {
	result := r.db.WithContext(ctx).
		Where("node_id = ?", nodeID).
		Delete(&HealthCheck{})
	if result.Error != nil {
		return errors.NewDatabaseError("failed to delete health checks by node ID", result.Error)
	}
	return nil
}
