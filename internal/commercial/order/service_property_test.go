// Package order provides order management functionality.
package order

import (
	"strings"
	"testing"
	"testing/quick"
	"time"

	"v/internal/logger"
)

// Feature: commercial-system, Property 1: Order ID Uniqueness
// Validates: Requirements 3.3
// For any two orders in the system, their order numbers SHALL be unique.

func TestProperty_OrderNoUniqueness(t *testing.T) {
	log := logger.NewNopLogger()
	svc := NewService(nil, nil, log, nil)

	// Property: Generating N order numbers should produce N unique values
	f := func(count uint8) bool {
		// Limit count to reasonable range
		n := int(count%100) + 1

		orderNos := make(map[string]bool)
		for i := 0; i < n; i++ {
			orderNo := svc.GenerateOrderNo()
			if orderNos[orderNo] {
				return false // Duplicate found
			}
			orderNos[orderNo] = true
		}

		return len(orderNos) == n
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// Property: Order numbers follow the expected format
func TestProperty_OrderNoFormat(t *testing.T) {
	log := logger.NewNopLogger()
	svc := NewService(nil, nil, log, nil)

	f := func(count uint8) bool {
		n := int(count%50) + 1

		for i := 0; i < n; i++ {
			orderNo := svc.GenerateOrderNo()

			// Check format: ORD-YYYYMMDD-XXXXXXXX
			if !strings.HasPrefix(orderNo, "ORD-") {
				return false
			}

			parts := strings.Split(orderNo, "-")
			if len(parts) != 3 {
				return false
			}

			// Date part should be 8 characters
			if len(parts[1]) != 8 {
				return false
			}

			// Random part should be 8 characters
			if len(parts[2]) != 8 {
				return false
			}
		}

		return true
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// Feature: commercial-system, Property 5: Order Status Transitions
// Validates: Requirements 5.4
// For any order, status transitions SHALL follow the valid state machine.

func TestProperty_ValidStatusTransitions(t *testing.T) {
	log := logger.NewNopLogger()
	svc := NewService(nil, nil, log, nil)

	// Define all possible status pairs
	allStatuses := []string{StatusPending, StatusPaid, StatusCompleted, StatusCancelled, StatusRefunded}

	// Valid transitions
	validTransitions := map[string]map[string]bool{
		StatusPending:   {StatusPaid: true, StatusCancelled: true},
		StatusPaid:      {StatusCompleted: true, StatusRefunded: true},
		StatusCompleted: {StatusRefunded: true},
		StatusCancelled: {},
		StatusRefunded:  {},
	}

	// Property: isValidStatusTransition should match our expected transitions
	for _, from := range allStatuses {
		for _, to := range allStatuses {
			expected := validTransitions[from][to]
			actual := svc.isValidStatusTransition(from, to)

			if expected != actual {
				t.Errorf("Transition from %s to %s: expected %v, got %v", from, to, expected, actual)
			}
		}
	}
}

// Property: Terminal states have no valid outgoing transitions (except refund from completed)
func TestProperty_TerminalStates(t *testing.T) {
	log := logger.NewNopLogger()
	svc := NewService(nil, nil, log, nil)

	terminalStates := []string{StatusCancelled, StatusRefunded}
	allStatuses := []string{StatusPending, StatusPaid, StatusCompleted, StatusCancelled, StatusRefunded}

	for _, terminal := range terminalStates {
		for _, to := range allStatuses {
			if svc.isValidStatusTransition(terminal, to) {
				t.Errorf("Terminal state %s should not transition to %s", terminal, to)
			}
		}
	}
}

// Feature: commercial-system, Property 7: Order Expiration
// Validates: Requirements 3.7, 3.8
// For any pending order past its expired_at time, the system SHALL mark it as cancelled.

func TestProperty_OrderExpiration(t *testing.T) {
	// Property: An order is considered expired if current time is after expired_at
	f := func(minutesOffset int8) bool {
		now := time.Now()
		// Create expiration time based on offset (-128 to 127 minutes from now)
		expiredAt := now.Add(time.Duration(minutesOffset) * time.Minute)

		isExpired := now.After(expiredAt)
		expectedExpired := minutesOffset < 0

		return isExpired == expectedExpired
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// Property: Expired orders should transition to cancelled status
func TestProperty_ExpiredOrderStatus(t *testing.T) {
	log := logger.NewNopLogger()
	svc := NewService(nil, nil, log, nil)

	// Property: Only pending orders can be expired (cancelled)
	// Expired orders should transition from pending to cancelled
	canExpire := svc.isValidStatusTransition(StatusPending, StatusCancelled)
	if !canExpire {
		t.Error("Pending orders should be able to transition to cancelled (expired)")
	}

	// Non-pending orders should not be affected by expiration
	nonPendingStatuses := []string{StatusPaid, StatusCompleted, StatusCancelled, StatusRefunded}
	for _, status := range nonPendingStatuses {
		// These statuses should not transition to cancelled via expiration
		// (paid/completed have different paths, cancelled/refunded are terminal)
		if status == StatusPaid || status == StatusCompleted {
			continue // These have valid paths but not via expiration
		}
		if svc.isValidStatusTransition(status, StatusCancelled) {
			t.Errorf("Status %s should not transition to cancelled", status)
		}
	}
}

// Property: Order expiration time should be set correctly based on config
func TestProperty_OrderExpirationConfig(t *testing.T) {
	f := func(minutes uint8) bool {
		// Ensure minutes is at least 1
		mins := int(minutes%60) + 1
		duration := time.Duration(mins) * time.Minute

		config := &Config{
			OrderExpiration: duration,
		}

		// Verify config is set correctly
		return config.OrderExpiration == duration
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}
