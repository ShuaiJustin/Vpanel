// Package payment provides payment gateway functionality.
package payment

import (
	"context"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"v/internal/logger"
)

// Feature: commercial-system, Property 17: Payment Retry Limit
// Validates: Requirements 17.2
// *For any* order, the number of payment retry attempts SHALL not exceed the configured max_retries.
func TestProperty_PaymentRetryLimit(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("retry stops accepting failures after max_retries", prop.ForAll(
		func(maxRetries int, attemptCount int) bool {
			if maxRetries < 1 {
				maxRetries = 1
			}
			if attemptCount < 1 {
				attemptCount = 1
			}

			config := &RetryConfig{
				MaxRetries:     maxRetries,
				RetryIntervals: DefaultRetryConfig().RetryIntervals,
				Enabled:        true,
			}

			svc := NewRetryService(nil, nil, config, logger.NewNopLogger())
			ctx := context.Background()

			orderID := int64(1)
			orderNo := "ORD-TEST-001"

			// Simulate multiple failures
			var lastErr error
			exhaustedAt := -1
			for i := 0; i < attemptCount; i++ {
				_, lastErr = svc.RecordFailure(ctx, orderID, orderNo, "test error")
				if lastErr == ErrMaxRetriesExceeded && exhaustedAt == -1 {
					exhaustedAt = i + 1
				}
			}

			// Get retry record
			retry, exists := svc.GetRetryRecord(orderID)
			if !exists {
				return false
			}

			// Once exhausted, status should remain exhausted
			if exhaustedAt > 0 {
				if retry.Status != RetryStatusExhausted {
					t.Logf("Expected exhausted status after reaching max, got %s", retry.Status)
					return false
				}
				// Exhaustion should happen at maxRetries
				if exhaustedAt != maxRetries {
					t.Logf("Expected exhaustion at %d, got %d", maxRetries, exhaustedAt)
					return false
				}
			}

			// If we haven't reached max, status should be pending
			if attemptCount < maxRetries {
				if retry.Status != RetryStatusPending {
					t.Logf("Expected pending status before max, got %s", retry.Status)
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 10),  // maxRetries
		gen.IntRange(1, 20),  // attemptCount
	))

	properties.Property("CanRetry returns false when max retries reached", prop.ForAll(
		func(maxRetries int) bool {
			if maxRetries < 1 {
				maxRetries = 1
			}

			config := &RetryConfig{
				MaxRetries:     maxRetries,
				RetryIntervals: DefaultRetryConfig().RetryIntervals,
				Enabled:        true,
			}

			svc := NewRetryService(nil, nil, config, logger.NewNopLogger())
			ctx := context.Background()

			orderID := int64(1)
			orderNo := "ORD-TEST-001"

			// Initially should be able to retry
			if !svc.CanRetry(orderID) {
				return false
			}

			// Record failures up to max
			for i := 0; i < maxRetries; i++ {
				svc.RecordFailure(ctx, orderID, orderNo, "test error")
			}

			// After max retries, should not be able to retry
			return !svc.CanRetry(orderID)
		},
		gen.IntRange(1, 10),
	))

	properties.Property("retry status transitions correctly", prop.ForAll(
		func(maxRetries int) bool {
			if maxRetries < 2 {
				maxRetries = 2
			}

			config := &RetryConfig{
				MaxRetries:     maxRetries,
				RetryIntervals: DefaultRetryConfig().RetryIntervals,
				Enabled:        true,
			}

			svc := NewRetryService(nil, nil, config, logger.NewNopLogger())
			ctx := context.Background()

			orderID := int64(1)
			orderNo := "ORD-TEST-001"

			// First failure - should be pending
			svc.RecordFailure(ctx, orderID, orderNo, "error 1")
			retry, _ := svc.GetRetryRecord(orderID)
			if retry.Status != RetryStatusPending {
				t.Logf("Expected pending after first failure, got %s", retry.Status)
				return false
			}

			// Continue until exhausted
			for i := 1; i < maxRetries; i++ {
				svc.RecordFailure(ctx, orderID, orderNo, "error")
			}

			retry, _ = svc.GetRetryRecord(orderID)
			if retry.Status != RetryStatusExhausted {
				t.Logf("Expected exhausted after max failures, got %s", retry.Status)
				return false
			}

			return true
		},
		gen.IntRange(2, 10),
	))

	properties.Property("next retry time is scheduled correctly", prop.ForAll(
		func(maxRetries int) bool {
			if maxRetries < 3 {
				maxRetries = 3
			}

			config := &RetryConfig{
				MaxRetries:     maxRetries,
				RetryIntervals: DefaultRetryConfig().RetryIntervals,
				Enabled:        true,
			}

			svc := NewRetryService(nil, nil, config, logger.NewNopLogger())
			ctx := context.Background()

			orderID := int64(1)
			orderNo := "ORD-TEST-001"

			// First failure
			svc.RecordFailure(ctx, orderID, orderNo, "error 1")
			retry, _ := svc.GetRetryRecord(orderID)

			// Should have next retry scheduled
			if retry.NextRetryAt == nil {
				t.Log("NextRetryAt should be set after first failure")
				return false
			}

			// After exhaustion, next retry should be nil
			for i := 1; i < maxRetries; i++ {
				svc.RecordFailure(ctx, orderID, orderNo, "error")
			}

			retry, _ = svc.GetRetryRecord(orderID)
			if retry.NextRetryAt != nil {
				t.Log("NextRetryAt should be nil after exhaustion")
				return false
			}

			return true
		},
		gen.IntRange(3, 10),
	))

	properties.Property("marking succeeded stops further retries", prop.ForAll(
		func(maxRetries int, failuresBeforeSuccess int) bool {
			if maxRetries < 3 {
				maxRetries = 3
			}
			if failuresBeforeSuccess < 1 {
				failuresBeforeSuccess = 1
			}
			if failuresBeforeSuccess >= maxRetries {
				failuresBeforeSuccess = maxRetries - 1
			}

			config := &RetryConfig{
				MaxRetries:     maxRetries,
				RetryIntervals: DefaultRetryConfig().RetryIntervals,
				Enabled:        true,
			}

			svc := NewRetryService(nil, nil, config, logger.NewNopLogger())
			ctx := context.Background()

			orderID := int64(1)
			orderNo := "ORD-TEST-001"

			// Record some failures
			for i := 0; i < failuresBeforeSuccess; i++ {
				svc.RecordFailure(ctx, orderID, orderNo, "error")
			}

			// Mark as succeeded
			svc.MarkSucceeded(orderID)

			retry, _ := svc.GetRetryRecord(orderID)
			if retry.Status != RetryStatusSucceeded {
				t.Logf("Expected succeeded status, got %s", retry.Status)
				return false
			}

			return true
		},
		gen.IntRange(3, 10),
		gen.IntRange(1, 5),
	))

	properties.TestingRun(t)
}

// Feature: commercial-system, Property 17: Payment Retry Limit (Cancellation)
// Validates: Requirements 17.2
func TestProperty_PaymentRetryCancellation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("cancelled retry cannot be retried", prop.ForAll(
		func(maxRetries int, failuresBeforeCancel int) bool {
			if maxRetries < 3 {
				maxRetries = 3
			}
			if failuresBeforeCancel < 1 {
				failuresBeforeCancel = 1
			}
			if failuresBeforeCancel >= maxRetries {
				failuresBeforeCancel = maxRetries - 1
			}

			config := &RetryConfig{
				MaxRetries:     maxRetries,
				RetryIntervals: DefaultRetryConfig().RetryIntervals,
				Enabled:        true,
			}

			svc := NewRetryService(nil, nil, config, logger.NewNopLogger())
			ctx := context.Background()

			orderID := int64(1)
			orderNo := "ORD-TEST-001"

			// Record some failures
			for i := 0; i < failuresBeforeCancel; i++ {
				svc.RecordFailure(ctx, orderID, orderNo, "error")
			}

			// Cancel retry
			svc.CancelRetry(orderID)

			retry, _ := svc.GetRetryRecord(orderID)
			if retry.Status != RetryStatusCancelled {
				t.Logf("Expected cancelled status, got %s", retry.Status)
				return false
			}

			return true
		},
		gen.IntRange(3, 10),
		gen.IntRange(1, 5),
	))

	properties.TestingRun(t)
}
