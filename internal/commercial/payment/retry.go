// Package payment provides payment gateway functionality.
package payment

import (
	"context"
	"errors"
	"sync"
	"time"

	"v/internal/database/repository"
	"v/internal/logger"
)

// Retry-related errors
var (
	ErrMaxRetriesExceeded = errors.New("maximum retry attempts exceeded")
	ErrOrderNotRetryable  = errors.New("order is not retryable")
	ErrRetryTooSoon       = errors.New("retry attempted too soon")
)

// RetryConfig holds configuration for payment retry.
type RetryConfig struct {
	MaxRetries     int             // Maximum number of retry attempts (default: 3)
	RetryIntervals []time.Duration // Intervals between retries (default: 1h, 4h, 24h)
	Enabled        bool            // Whether retry is enabled
}

// DefaultRetryConfig returns the default retry configuration.
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries: 3,
		RetryIntervals: []time.Duration{
			1 * time.Hour,
			4 * time.Hour,
			24 * time.Hour,
		},
		Enabled: true,
	}
}

// PaymentRetry represents a payment retry record.
type PaymentRetry struct {
	ID            int64      `json:"id"`
	OrderID       int64      `json:"order_id"`
	OrderNo       string     `json:"order_no"`
	AttemptCount  int        `json:"attempt_count"`
	LastAttemptAt *time.Time `json:"last_attempt_at"`
	NextRetryAt   *time.Time `json:"next_retry_at"`
	LastError     string     `json:"last_error"`
	Status        string     `json:"status"` // pending, retrying, exhausted, succeeded, cancelled
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// RetryStatus constants
const (
	RetryStatusPending   = "pending"
	RetryStatusRetrying  = "retrying"
	RetryStatusExhausted = "exhausted"
	RetryStatusSucceeded = "succeeded"
	RetryStatusCancelled = "cancelled"
)

// FailedPaymentStats represents statistics about failed payments.
type FailedPaymentStats struct {
	TotalFailed      int64            `json:"total_failed"`
	PendingRetry     int64            `json:"pending_retry"`
	RetryExhausted   int64            `json:"retry_exhausted"`
	RecoveredByRetry int64            `json:"recovered_by_retry"`
	FailureRate      float64          `json:"failure_rate"`
	RecoveryRate     float64          `json:"recovery_rate"`
	AvgRetryAttempts float64          `json:"avg_retry_attempts"`
	FailuresByMethod map[string]int64 `json:"failures_by_method"`
	FailuresByReason map[string]int64 `json:"failures_by_reason"`
}

// RetryService provides payment retry functionality.
type RetryService struct {
	orderRepo      repository.OrderRepository
	paymentService *Service
	config         *RetryConfig
	logger         logger.Logger
	mu             sync.Mutex

	// In-memory tracking of retry records (in production, use database)
	retryRecords map[int64]*PaymentRetry
}

// NewRetryService creates a new payment retry service.
func NewRetryService(
	orderRepo repository.OrderRepository,
	paymentService *Service,
	config *RetryConfig,
	log logger.Logger,
) *RetryService {
	if config == nil {
		config = DefaultRetryConfig()
	}
	return &RetryService{
		orderRepo:      orderRepo,
		paymentService: paymentService,
		config:         config,
		logger:         log,
		retryRecords:   make(map[int64]*PaymentRetry),
	}
}

// RecordFailure records a payment failure and schedules retry if applicable.
func (s *RetryService) RecordFailure(ctx context.Context, orderID int64, orderNo string, errorMsg string) (*PaymentRetry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	// Check if retry record exists
	retry, exists := s.retryRecords[orderID]
	if !exists {
		retry = &PaymentRetry{
			ID:           orderID, // Using orderID as retry ID for simplicity
			OrderID:      orderID,
			OrderNo:      orderNo,
			AttemptCount: 0,
			Status:       RetryStatusPending,
			CreatedAt:    now,
		}
		s.retryRecords[orderID] = retry
	}

	// Increment attempt count
	retry.AttemptCount++
	retry.LastAttemptAt = &now
	retry.LastError = errorMsg
	retry.UpdatedAt = now

	// Check if max retries exceeded
	if retry.AttemptCount >= s.config.MaxRetries {
		retry.Status = RetryStatusExhausted
		retry.NextRetryAt = nil
		s.logger.Warn("Payment retry exhausted",
			logger.F("orderID", orderID),
			logger.F("orderNo", orderNo),
			logger.F("attempts", retry.AttemptCount))
		return retry, ErrMaxRetriesExceeded
	}

	// Calculate next retry time
	intervalIndex := retry.AttemptCount - 1
	if intervalIndex >= len(s.config.RetryIntervals) {
		intervalIndex = len(s.config.RetryIntervals) - 1
	}
	nextRetry := now.Add(s.config.RetryIntervals[intervalIndex])
	retry.NextRetryAt = &nextRetry
	retry.Status = RetryStatusPending

	s.logger.Info("Payment failure recorded, retry scheduled",
		logger.F("orderID", orderID),
		logger.F("orderNo", orderNo),
		logger.F("attempt", retry.AttemptCount),
		logger.F("nextRetry", nextRetry))

	return retry, nil
}

// GetRetryRecord retrieves the retry record for an order.
func (s *RetryService) GetRetryRecord(orderID int64) (*PaymentRetry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	retry, exists := s.retryRecords[orderID]
	return retry, exists
}

// GetPendingRetries returns all orders pending retry.
func (s *RetryService) GetPendingRetries(ctx context.Context) ([]*PaymentRetry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	var pending []*PaymentRetry

	for _, retry := range s.retryRecords {
		if retry.Status == RetryStatusPending && retry.NextRetryAt != nil && retry.NextRetryAt.Before(now) {
			pending = append(pending, retry)
		}
	}

	return pending, nil
}

// ExecuteRetry attempts to retry a payment for an order.
func (s *RetryService) ExecuteRetry(ctx context.Context, orderID int64, paymentMethod string) error {
	s.mu.Lock()
	retry, exists := s.retryRecords[orderID]
	if !exists {
		s.mu.Unlock()
		return ErrOrderNotRetryable
	}

	if retry.Status != RetryStatusPending {
		s.mu.Unlock()
		return ErrOrderNotRetryable
	}

	// Check if it's time to retry
	if retry.NextRetryAt != nil && time.Now().Before(*retry.NextRetryAt) {
		s.mu.Unlock()
		return ErrRetryTooSoon
	}

	retry.Status = RetryStatusRetrying
	s.mu.Unlock()

	// Get order
	order, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		s.RecordFailure(ctx, orderID, retry.OrderNo, "order not found")
		return err
	}

	// Check if order is still pending
	if order.Status != repository.OrderStatusPending {
		s.mu.Lock()
		retry.Status = RetryStatusCancelled
		s.mu.Unlock()
		return ErrOrderNotRetryable
	}

	// Use provided payment method or fall back to original
	method := paymentMethod
	if method == "" {
		method = order.PaymentMethod
	}

	// Attempt payment
	_, err = s.paymentService.CreatePayment(ctx, order.OrderNo, method, "")
	if err != nil {
		s.RecordFailure(ctx, orderID, order.OrderNo, err.Error())
		return err
	}

	// Mark as succeeded (actual success will be confirmed via callback)
	s.mu.Lock()
	retry.Status = RetryStatusPending // Keep pending until callback confirms
	s.mu.Unlock()

	s.logger.Info("Payment retry initiated",
		logger.F("orderID", orderID),
		logger.F("orderNo", order.OrderNo),
		logger.F("method", method))

	return nil
}

// MarkSucceeded marks a retry as succeeded (called when payment callback succeeds).
func (s *RetryService) MarkSucceeded(orderID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if retry, exists := s.retryRecords[orderID]; exists {
		retry.Status = RetryStatusSucceeded
		retry.UpdatedAt = time.Now()
		s.logger.Info("Payment retry succeeded",
			logger.F("orderID", orderID),
			logger.F("attempts", retry.AttemptCount))
	}
}

// CancelRetry cancels pending retries for an order.
func (s *RetryService) CancelRetry(orderID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if retry, exists := s.retryRecords[orderID]; exists {
		retry.Status = RetryStatusCancelled
		retry.UpdatedAt = time.Now()
	}
}

// ProcessPendingRetries processes all pending retries that are due.
func (s *RetryService) ProcessPendingRetries(ctx context.Context) (int, int, error) {
	pending, err := s.GetPendingRetries(ctx)
	if err != nil {
		return 0, 0, err
	}

	var succeeded, failed int
	for _, retry := range pending {
		// Get order to determine payment method
		order, err := s.orderRepo.GetByID(ctx, retry.OrderID)
		if err != nil {
			failed++
			continue
		}

		err = s.ExecuteRetry(ctx, retry.OrderID, order.PaymentMethod)
		if err != nil {
			failed++
			s.logger.Warn("Retry failed",
				logger.F("orderID", retry.OrderID),
				logger.Err(err))
		} else {
			succeeded++
		}
	}

	if succeeded > 0 || failed > 0 {
		s.logger.Info("Processed pending retries",
			logger.F("succeeded", succeeded),
			logger.F("failed", failed))
	}

	return succeeded, failed, nil
}

// GetFailedPaymentStats returns statistics about failed payments.
func (s *RetryService) GetFailedPaymentStats(ctx context.Context) (*FailedPaymentStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stats := &FailedPaymentStats{
		FailuresByMethod: make(map[string]int64),
		FailuresByReason: make(map[string]int64),
	}

	var totalAttempts int64
	for _, retry := range s.retryRecords {
		stats.TotalFailed++
		totalAttempts += int64(retry.AttemptCount)

		switch retry.Status {
		case RetryStatusPending, RetryStatusRetrying:
			stats.PendingRetry++
		case RetryStatusExhausted:
			stats.RetryExhausted++
		case RetryStatusSucceeded:
			stats.RecoveredByRetry++
		}

		// Track failure reasons
		if retry.LastError != "" {
			stats.FailuresByReason[retry.LastError]++
		}
	}

	// Calculate rates
	if stats.TotalFailed > 0 {
		stats.AvgRetryAttempts = float64(totalAttempts) / float64(stats.TotalFailed)
		stats.RecoveryRate = float64(stats.RecoveredByRetry) / float64(stats.TotalFailed) * 100
	}

	// Get total orders for failure rate calculation
	totalOrders, err := s.orderRepo.Count(ctx)
	if err == nil && totalOrders > 0 {
		stats.FailureRate = float64(stats.TotalFailed) / float64(totalOrders) * 100
	}

	return stats, nil
}

// SwitchPaymentMethod allows switching payment method for a failed order.
func (s *RetryService) SwitchPaymentMethod(ctx context.Context, orderID int64, newMethod string) error {
	// Get order
	order, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return ErrOrderNotFound
	}

	// Check if order is still pending
	if order.Status != repository.OrderStatusPending {
		return ErrOrderNotRetryable
	}

	// Verify the new payment method is available
	_, err = s.paymentService.GetGateway(newMethod)
	if err != nil {
		return err
	}

	// Update order payment method
	order.PaymentMethod = newMethod
	if err := s.orderRepo.Update(ctx, order); err != nil {
		return err
	}

	s.logger.Info("Payment method switched",
		logger.F("orderID", orderID),
		logger.F("newMethod", newMethod))

	return nil
}

// CanRetry checks if an order can be retried.
func (s *RetryService) CanRetry(orderID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	retry, exists := s.retryRecords[orderID]
	if !exists {
		return true // No retry record means it can be retried
	}

	return retry.Status == RetryStatusPending && retry.AttemptCount < s.config.MaxRetries
}

// GetRetryInfo returns retry information for an order.
func (s *RetryService) GetRetryInfo(orderID int64) *PaymentRetry {
	s.mu.Lock()
	defer s.mu.Unlock()

	if retry, exists := s.retryRecords[orderID]; exists {
		// Return a copy to avoid race conditions
		copy := *retry
		return &copy
	}
	return nil
}
