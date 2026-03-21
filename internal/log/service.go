// Package log provides logging service with database persistence.
package log

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"v/internal/database/repository"
	"v/internal/logger"
)

// Service provides logging operations with database persistence.
type Service struct {
	repo          repository.LogRepository
	logger        logger.Logger
	writer        *AsyncWriter
	retentionDays int
	mu            sync.RWMutex
}

const defaultRetentionDays = 30

// Config holds log service configuration.
type Config struct {
	DatabaseEnabled bool
	DatabaseLevel   string
	BufferSize      int
	BatchSize       int
	FlushInterval   time.Duration
	RetentionDays   int
}

// NewService creates a new log service.
func NewService(repo repository.LogRepository, log logger.Logger, cfg Config) *Service {
	if cfg.BufferSize == 0 {
		cfg.BufferSize = 1000
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 100
	}
	if cfg.FlushInterval == 0 {
		cfg.FlushInterval = 5 * time.Second
	}
	if cfg.RetentionDays <= 0 {
		cfg.RetentionDays = defaultRetentionDays
	}

	s := &Service{
		repo:          repo,
		logger:        log,
		retentionDays: cfg.RetentionDays,
	}

	s.writer = NewAsyncWriter(repo, log, cfg.BufferSize, cfg.BatchSize, cfg.FlushInterval)
	return s
}

// Log writes a log entry asynchronously.
func (s *Service) Log(ctx context.Context, level, message, source string, fields map[string]any) error {
	fieldsJSON := ""
	if len(fields) > 0 {
		data, _ := json.Marshal(fields)
		fieldsJSON = string(data)
	}

	log := &repository.Log{
		Level:     level,
		Message:   message,
		Source:    source,
		Fields:    fieldsJSON,
		CreatedAt: time.Now(),
	}

	// Extract common fields
	if userID, ok := fields["user_id"].(int64); ok {
		log.UserID = &userID
	}
	if ip, ok := fields["ip"].(string); ok {
		log.IP = ip
	}
	if ua, ok := fields["user_agent"].(string); ok {
		log.UserAgent = ua
	}
	if reqID, ok := fields["request_id"].(string); ok {
		log.RequestID = reqID
	}

	return s.writer.Write(log)
}

// Query retrieves logs with filtering and pagination.
func (s *Service) Query(ctx context.Context, filter *repository.LogFilter, limit, offset int) ([]*repository.Log, int64, error) {
	logs, err := s.repo.List(ctx, filter, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	count, err := s.repo.Count(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	return logs, count, nil
}

// GetByID retrieves a log entry by ID.
func (s *Service) GetByID(ctx context.Context, id int64) (*repository.Log, error) {
	return s.repo.GetByID(ctx, id)
}

// Delete deletes logs matching the filter.
func (s *Service) Delete(ctx context.Context, filter *repository.LogFilter) (int64, error) {
	return s.repo.DeleteByFilter(ctx, filter)
}

// Cleanup deletes logs older than retention period.
func (s *Service) Cleanup(ctx context.Context, retentionDays int) (int64, error) {
	if retentionDays < 1 {
		return 0, fmt.Errorf("retention days must be greater than 0")
	}

	before := time.Now().AddDate(0, 0, -retentionDays)
	deleted, err := s.repo.DeleteOlderThan(ctx, before)
	if err != nil {
		return 0, err
	}

	s.logger.Info("log cleanup completed", logger.F("deleted", deleted), logger.F("before", before))
	return deleted, nil
}

// Close gracefully shuts down the service.
func (s *Service) Close() error {
	return s.writer.Close()
}

// StartCleanupScheduler starts a background scheduler for log cleanup.
func (s *Service) StartCleanupScheduler(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if _, err := s.Cleanup(ctx, s.retentionDays); err != nil {
					s.logger.Error("scheduled cleanup failed", logger.F("error", err))
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}
