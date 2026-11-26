// Package node provides node management functionality.
package node

import (
	"testing"
	"testing/quick"

	"v/internal/database/repository"
)

// Feature: multi-server-management, Property 5: Health Status Transition
// Validates: Requirements 2.4, 2.5
// For any node, when consecutive health checks fail (exceeding threshold), the node status
// SHALL transition to unhealthy. When consecutive checks succeed (exceeding recovery threshold),
// status SHALL transition to healthy.

// mockHealthCheckState simulates the health checker state for property testing
type mockHealthCheckState struct {
	consecutiveFailures  int
	consecutiveSuccesses int
	currentStatus        string
	unhealthyThreshold   int
	healthyThreshold     int
}

func newMockHealthCheckState(unhealthyThreshold, healthyThreshold int) *mockHealthCheckState {
	return &mockHealthCheckState{
		consecutiveFailures:  0,
		consecutiveSuccesses: 0,
		currentStatus:        repository.NodeStatusOnline,
		unhealthyThreshold:   unhealthyThreshold,
		healthyThreshold:     healthyThreshold,
	}
}

// recordSuccess records a successful health check
func (s *mockHealthCheckState) recordSuccess() string {
	s.consecutiveFailures = 0
	s.consecutiveSuccesses++

	// Transition from unhealthy to online if threshold met
	if s.currentStatus == repository.NodeStatusUnhealthy &&
		s.consecutiveSuccesses >= s.healthyThreshold {
		s.currentStatus = repository.NodeStatusOnline
	}

	return s.currentStatus
}

// recordFailure records a failed health check
func (s *mockHealthCheckState) recordFailure() string {
	s.consecutiveSuccesses = 0
	s.consecutiveFailures++

	// Transition from online to unhealthy if threshold met
	if s.currentStatus == repository.NodeStatusOnline &&
		s.consecutiveFailures >= s.unhealthyThreshold {
		s.currentStatus = repository.NodeStatusUnhealthy
	}

	return s.currentStatus
}

// TestProperty_ConsecutiveFailuresTransitionToUnhealthy tests that consecutive failures
// cause a transition to unhealthy status.
func TestProperty_ConsecutiveFailuresTransitionToUnhealthy(t *testing.T) {
	// Property: For any unhealthy threshold N, after N consecutive failures,
	// a node should transition from online to unhealthy
	f := func(threshold uint8) bool {
		// Constrain threshold to reasonable range (1-10)
		n := int(threshold%10) + 1

		state := newMockHealthCheckState(n, 2)

		// Record N-1 failures - should still be online
		for i := 0; i < n-1; i++ {
			status := state.recordFailure()
			if status != repository.NodeStatusOnline {
				t.Logf("Node transitioned too early at failure %d (threshold=%d)", i+1, n)
				return false
			}
		}

		// Record Nth failure - should transition to unhealthy
		status := state.recordFailure()
		if status != repository.NodeStatusUnhealthy {
			t.Logf("Node did not transition to unhealthy after %d failures", n)
			return false
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

// TestProperty_ConsecutiveSuccessesTransitionToHealthy tests that consecutive successes
// cause a transition from unhealthy to healthy status.
func TestProperty_ConsecutiveSuccessesTransitionToHealthy(t *testing.T) {
	// Property: For any healthy threshold N, after N consecutive successes,
	// a node should transition from unhealthy to online
	f := func(threshold uint8) bool {
		// Constrain threshold to reasonable range (1-10)
		n := int(threshold%10) + 1

		state := newMockHealthCheckState(3, n)
		// Start in unhealthy state
		state.currentStatus = repository.NodeStatusUnhealthy

		// Record N-1 successes - should still be unhealthy
		for i := 0; i < n-1; i++ {
			status := state.recordSuccess()
			if status != repository.NodeStatusUnhealthy {
				t.Logf("Node transitioned too early at success %d (threshold=%d)", i+1, n)
				return false
			}
		}

		// Record Nth success - should transition to online
		status := state.recordSuccess()
		if status != repository.NodeStatusOnline {
			t.Logf("Node did not transition to online after %d successes", n)
			return false
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

// TestProperty_SuccessResetsFailureCounter tests that a success resets the failure counter.
func TestProperty_SuccessResetsFailureCounter(t *testing.T) {
	// Property: A successful health check should reset the consecutive failure counter
	f := func(failures uint8) bool {
		// Constrain failures to less than threshold
		n := int(failures%5) + 1
		threshold := n + 2 // Ensure threshold is higher

		state := newMockHealthCheckState(threshold, 2)

		// Record some failures
		for i := 0; i < n; i++ {
			state.recordFailure()
		}

		// Verify failures were recorded
		if state.consecutiveFailures != n {
			t.Logf("Expected %d failures, got %d", n, state.consecutiveFailures)
			return false
		}

		// Record a success
		state.recordSuccess()

		// Failure counter should be reset
		if state.consecutiveFailures != 0 {
			t.Logf("Failure counter not reset after success: %d", state.consecutiveFailures)
			return false
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

// TestProperty_FailureResetsSuccessCounter tests that a failure resets the success counter.
func TestProperty_FailureResetsSuccessCounter(t *testing.T) {
	// Property: A failed health check should reset the consecutive success counter
	f := func(successes uint8) bool {
		// Constrain successes to less than threshold
		n := int(successes%5) + 1
		threshold := n + 2 // Ensure threshold is higher

		state := newMockHealthCheckState(3, threshold)
		state.currentStatus = repository.NodeStatusUnhealthy

		// Record some successes
		for i := 0; i < n; i++ {
			state.recordSuccess()
		}

		// Verify successes were recorded
		if state.consecutiveSuccesses != n {
			t.Logf("Expected %d successes, got %d", n, state.consecutiveSuccesses)
			return false
		}

		// Record a failure
		state.recordFailure()

		// Success counter should be reset
		if state.consecutiveSuccesses != 0 {
			t.Logf("Success counter not reset after failure: %d", state.consecutiveSuccesses)
			return false
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

// TestProperty_OnlineNodeStaysOnlineWithSuccesses tests that an online node stays online
// with continuous successes.
func TestProperty_OnlineNodeStaysOnlineWithSuccesses(t *testing.T) {
	// Property: An online node should remain online with any number of consecutive successes
	f := func(successes uint8) bool {
		n := int(successes%100) + 1

		state := newMockHealthCheckState(3, 2)
		// Start online
		state.currentStatus = repository.NodeStatusOnline

		// Record many successes
		for i := 0; i < n; i++ {
			status := state.recordSuccess()
			if status != repository.NodeStatusOnline {
				t.Logf("Online node changed status after %d successes", i+1)
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

// TestProperty_UnhealthyNodeStaysUnhealthyWithFailures tests that an unhealthy node stays
// unhealthy with continuous failures.
func TestProperty_UnhealthyNodeStaysUnhealthyWithFailures(t *testing.T) {
	// Property: An unhealthy node should remain unhealthy with any number of consecutive failures
	f := func(failures uint8) bool {
		n := int(failures%100) + 1

		state := newMockHealthCheckState(3, 2)
		// Start unhealthy
		state.currentStatus = repository.NodeStatusUnhealthy

		// Record many failures
		for i := 0; i < n; i++ {
			status := state.recordFailure()
			if status != repository.NodeStatusUnhealthy {
				t.Logf("Unhealthy node changed status after %d failures", i+1)
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

// TestProperty_StatusTransitionSequence tests a sequence of health checks with mixed results.
func TestProperty_StatusTransitionSequence(t *testing.T) {
	// Property: Status transitions should follow the threshold rules regardless of sequence
	f := func(sequence []bool) bool {
		if len(sequence) == 0 {
			return true
		}

		// Limit sequence length
		if len(sequence) > 50 {
			sequence = sequence[:50]
		}

		unhealthyThreshold := 3
		healthyThreshold := 2

		state := newMockHealthCheckState(unhealthyThreshold, healthyThreshold)

		for _, success := range sequence {
			if success {
				state.recordSuccess()
			} else {
				state.recordFailure()
			}

			// Verify invariants
			// 1. Consecutive counters should never both be non-zero
			if state.consecutiveFailures > 0 && state.consecutiveSuccesses > 0 {
				t.Log("Both counters are non-zero")
				return false
			}

			// 2. Status should be valid
			if state.currentStatus != repository.NodeStatusOnline &&
				state.currentStatus != repository.NodeStatusUnhealthy {
				t.Logf("Invalid status: %s", state.currentStatus)
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

// TestProperty_ThresholdBoundary tests behavior at exact threshold boundaries.
func TestProperty_ThresholdBoundary(t *testing.T) {
	// Property: Transition should happen exactly at threshold, not before or after
	f := func(unhealthyThreshold, healthyThreshold uint8) bool {
		ut := int(unhealthyThreshold%10) + 1
		ht := int(healthyThreshold%10) + 1

		// Test unhealthy transition
		state1 := newMockHealthCheckState(ut, ht)
		for i := 0; i < ut-1; i++ {
			state1.recordFailure()
		}
		if state1.currentStatus != repository.NodeStatusOnline {
			t.Logf("Transitioned before threshold: failures=%d, threshold=%d", ut-1, ut)
			return false
		}
		state1.recordFailure()
		if state1.currentStatus != repository.NodeStatusUnhealthy {
			t.Logf("Did not transition at threshold: failures=%d, threshold=%d", ut, ut)
			return false
		}

		// Test healthy transition
		state2 := newMockHealthCheckState(ut, ht)
		state2.currentStatus = repository.NodeStatusUnhealthy
		for i := 0; i < ht-1; i++ {
			state2.recordSuccess()
		}
		if state2.currentStatus != repository.NodeStatusUnhealthy {
			t.Logf("Transitioned before threshold: successes=%d, threshold=%d", ht-1, ht)
			return false
		}
		state2.recordSuccess()
		if state2.currentStatus != repository.NodeStatusOnline {
			t.Logf("Did not transition at threshold: successes=%d, threshold=%d", ht, ht)
			return false
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
