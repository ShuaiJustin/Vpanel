// Package repository provides data access interfaces and implementations.
package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// Log represents a system log entry in the database.
type Log struct {
	ID        int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Level     string    `json:"level" gorm:"size:10;index"`
	Message   string    `json:"message" gorm:"type:text"`
	Source    string    `json:"source" gorm:"column:module;size:50;index:idx_logs_module"` // Maps to 'module' column in existing DB
	UserID    *int64    `json:"user_id" gorm:"index"`
	Username  string    `json:"username" gorm:"size:50"`
	IP        string    `json:"ip" gorm:"size:50"`
	UserAgent string    `json:"user_agent" gorm:"size:255"`
	RequestID string    `json:"request_id" gorm:"size:100;index"`
	Fields    string    `json:"fields" gorm:"column:details;type:text"` // Maps to 'details' column in existing DB
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the table name for Log.
func (Log) TableName() string {
	return "logs"
}

// LogFilter defines filtering options for log queries.
type LogFilter struct {
	Level     string     // Filter by exact log level
	MinLevel  string     // Filter by minimum log level (severity)
	Source    string     // Filter by source component
	UserID    *int64     // Filter by user ID
	StartTime *time.Time // Filter by start time
	EndTime   *time.Time // Filter by end time
	Keyword   string     // Search keyword in message
	RequestID string     // Filter by request ID
}

// LogRepository defines the interface for log data access.
type LogRepository interface {
	// Create creates a single log entry
	Create(ctx context.Context, log *Log) error

	// CreateBatch creates multiple log entries in a single transaction
	CreateBatch(ctx context.Context, logs []*Log) error

	// GetByID retrieves a log entry by ID
	GetByID(ctx context.Context, id int64) (*Log, error)

	// List retrieves logs with pagination and filtering
	List(ctx context.Context, filter *LogFilter, limit, offset int) ([]*Log, error)

	// Count returns the total count of logs matching the filter
	Count(ctx context.Context, filter *LogFilter) (int64, error)

	// DeleteOlderThan deletes logs older than the specified time
	DeleteOlderThan(ctx context.Context, before time.Time) (int64, error)

	// DeleteByFilter deletes logs matching the filter
	DeleteByFilter(ctx context.Context, filter *LogFilter) (int64, error)
}

// logRepository implements LogRepository using GORM.
type logRepository struct {
	db *gorm.DB
}

// NewLogRepository creates a new log repository.
func NewLogRepository(db *gorm.DB) LogRepository {
	return &logRepository{db: db}
}

// Create creates a single log entry.
func (r *logRepository) Create(ctx context.Context, log *Log) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// CreateBatch creates multiple log entries in a single transaction.
func (r *logRepository) CreateBatch(ctx context.Context, logs []*Log) error {
	if len(logs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(logs, 100).Error
}

// GetByID retrieves a log entry by ID.
func (r *logRepository) GetByID(ctx context.Context, id int64) (*Log, error) {
	var log Log
	err := r.db.WithContext(ctx).First(&log, id).Error
	if err != nil {
		return nil, err
	}
	return &log, nil
}

// List retrieves logs with pagination and filtering.
func (r *logRepository) List(ctx context.Context, filter *LogFilter, limit, offset int) ([]*Log, error) {
	var logs []*Log
	query := r.applyFilter(r.db.WithContext(ctx), filter)
	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&logs).Error
	if err != nil {
		return nil, err
	}
	return logs, nil
}

// Count returns the total count of logs matching the filter.
func (r *logRepository) Count(ctx context.Context, filter *LogFilter) (int64, error) {
	var count int64
	query := r.applyFilter(r.db.WithContext(ctx).Model(&Log{}), filter)
	err := query.Count(&count).Error
	return count, err
}

// DeleteOlderThan deletes logs older than the specified time.
func (r *logRepository) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	result := r.db.WithContext(ctx).Where("created_at < ?", before).Delete(&Log{})
	return result.RowsAffected, result.Error
}

// DeleteByFilter deletes logs matching the filter.
func (r *logRepository) DeleteByFilter(ctx context.Context, filter *LogFilter) (int64, error) {
	query := r.applyFilter(r.db.WithContext(ctx), filter)
	result := query.Delete(&Log{})
	return result.RowsAffected, result.Error
}

// applyFilter applies the filter to the query.
func (r *logRepository) applyFilter(query *gorm.DB, filter *LogFilter) *gorm.DB {
	if filter == nil {
		return query
	}

	if filter.Level != "" {
		query = query.Where("level = ?", filter.Level)
	}

	if filter.MinLevel != "" {
		levels := getLevelsAtOrAbove(filter.MinLevel)
		if len(levels) > 0 {
			query = query.Where("level IN ?", levels)
		}
	}

	if filter.Source != "" {
		// 'module' is the actual column name in the database
		query = query.Where("module = ?", filter.Source)
	}

	if filter.UserID != nil {
		query = query.Where("user_id = ?", *filter.UserID)
	}

	if filter.StartTime != nil {
		query = query.Where("created_at >= ?", *filter.StartTime)
	}

	if filter.EndTime != nil {
		query = query.Where("created_at <= ?", *filter.EndTime)
	}

	if filter.Keyword != "" {
		query = query.Where("message LIKE ?", "%"+filter.Keyword+"%")
	}

	if filter.RequestID != "" {
		query = query.Where("request_id = ?", filter.RequestID)
	}

	return query
}

// levelSeverity maps log levels to their severity (higher = more severe).
var levelSeverity = map[string]int{
	"debug": 1,
	"info":  2,
	"warn":  3,
	"error": 4,
	"fatal": 5,
}

// getLevelsAtOrAbove returns all log levels at or above the given level.
func getLevelsAtOrAbove(minLevel string) []string {
	minSeverity, ok := levelSeverity[minLevel]
	if !ok {
		return nil
	}

	var levels []string
	for level, severity := range levelSeverity {
		if severity >= minSeverity {
			levels = append(levels, level)
		}
	}
	return levels
}
