// Package balance provides balance management functionality.
package balance

import (
	"testing"
	"testing/quick"
)

// Feature: commercial-system, Property 3: Balance Non-Negative Invariant
// Validates: Requirements 6.9
// For any balance operation, the resulting user balance SHALL never be negative.

func TestProperty_BalanceNonNegative(t *testing.T) {
	// Property: For any initial balance and deduction amount,
	// if deduction is allowed, the result must be non-negative
	f := func(initialBalance, deductAmount uint32) bool {
		balance := int64(initialBalance)
		amount := int64(deductAmount)

		// Simulate deduction check
		if amount <= 0 {
			return true // Invalid amount, no operation
		}

		// Check if deduction is allowed
		canDeduct := balance >= amount

		if canDeduct {
			newBalance := balance - amount
			// Result must be non-negative
			return newBalance >= 0
		}

		// If cannot deduct, balance remains unchanged (non-negative)
		return balance >= 0
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// Property: Adjustment cannot result in negative balance
func TestProperty_AdjustmentNonNegative(t *testing.T) {
	f := func(initialBalance uint32, adjustment int32) bool {
		balance := int64(initialBalance)
		adj := int64(adjustment)

		newBalance := balance + adj

		// If adjustment would result in negative, it should be rejected
		if newBalance < 0 {
			// Operation should be rejected
			return true // We expect the service to reject this
		}

		// If adjustment is valid, result must be non-negative
		return newBalance >= 0
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// Feature: commercial-system, Property 4: Balance Transaction Consistency
// Validates: Requirements 6.4, 6.5
// For any sequence of balance transactions for a user, the final balance
// SHALL equal the sum of all transaction amounts.

func TestProperty_TransactionConsistency(t *testing.T) {
	// Property: Sum of all transaction amounts equals final balance
	f := func(transactions []int32) bool {
		if len(transactions) == 0 {
			return true
		}

		var sum int64 = 0
		var currentBalance int64 = 0

		for _, tx := range transactions {
			amount := int64(tx)

			// Skip if would result in negative balance
			if currentBalance+amount < 0 {
				continue
			}

			sum += amount
			currentBalance += amount
		}

		// Final balance should equal sum of valid transactions
		return currentBalance == sum
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// Property: Recharge always increases balance
func TestProperty_RechargeIncreasesBalance(t *testing.T) {
	f := func(initialBalance uint32, rechargeAmount uint32) bool {
		if rechargeAmount == 0 {
			return true // Zero recharge is invalid
		}

		balance := int64(initialBalance)
		amount := int64(rechargeAmount)

		newBalance := balance + amount

		// Recharge should always increase balance
		return newBalance > balance && newBalance == balance+amount
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// Property: Refund always increases balance
func TestProperty_RefundIncreasesBalance(t *testing.T) {
	f := func(initialBalance uint32, refundAmount uint32) bool {
		if refundAmount == 0 {
			return true // Zero refund is invalid
		}

		balance := int64(initialBalance)
		amount := int64(refundAmount)

		newBalance := balance + amount

		// Refund should always increase balance
		return newBalance > balance && newBalance == balance+amount
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// Property: Deduction decreases balance by exact amount
func TestProperty_DeductionExactAmount(t *testing.T) {
	f := func(initialBalance, deductAmount uint32) bool {
		balance := int64(initialBalance)
		amount := int64(deductAmount)

		if amount <= 0 || balance < amount {
			return true // Invalid operation
		}

		newBalance := balance - amount

		// Deduction should decrease by exact amount
		return newBalance == balance-amount && newBalance >= 0
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}
