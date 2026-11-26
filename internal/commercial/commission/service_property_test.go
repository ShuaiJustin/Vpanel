// Package commission provides commission management functionality.
package commission

import (
	"testing"
	"testing/quick"

	"v/internal/logger"
)

// Feature: commercial-system, Property 9: Commission Calculation
// Validates: Requirements 10.1
// For any order with a referrer, the commission amount SHALL equal
// order amount multiplied by commission rate.

func TestProperty_CommissionCalculation(t *testing.T) {
	log := logger.NewNopLogger()

	f := func(orderAmount uint32, ratePercent uint8) bool {
		amount := int64(orderAmount)
		rate := float64(ratePercent%100) / 100 // 0-99%

		config := &Config{
			Enabled: true,
			Rate:    rate,
		}
		svc := NewService(nil, nil, log, config)

		commission, err := svc.Calculate(amount, 1)
		if err != nil {
			return false
		}

		// Expected commission
		expected := int64(float64(amount) * rate)

		return commission == expected
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// Property: Commission should be zero when disabled
func TestProperty_CommissionDisabled(t *testing.T) {
	log := logger.NewNopLogger()

	f := func(orderAmount uint32) bool {
		amount := int64(orderAmount)

		config := &Config{
			Enabled: false,
			Rate:    0.1,
		}
		svc := NewService(nil, nil, log, config)

		commission, err := svc.Calculate(amount, 1)
		if err != nil {
			return false
		}

		// Commission should be zero when disabled
		return commission == 0
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// Property: Commission should be zero for zero or negative order amount
func TestProperty_CommissionZeroForInvalidAmount(t *testing.T) {
	log := logger.NewNopLogger()

	config := &Config{
		Enabled: true,
		Rate:    0.1,
	}
	svc := NewService(nil, nil, log, config)

	// Zero amount
	commission, _ := svc.Calculate(0, 1)
	if commission != 0 {
		t.Error("Commission should be zero for zero order amount")
	}

	// Negative amount
	commission, _ = svc.Calculate(-100, 1)
	if commission != 0 {
		t.Error("Commission should be zero for negative order amount")
	}
}

// Property: Commission is proportional to order amount
func TestProperty_CommissionProportional(t *testing.T) {
	log := logger.NewNopLogger()

	f := func(amount1, amount2 uint16) bool {
		a1 := int64(amount1) + 1 // Ensure positive
		a2 := int64(amount2) + 1

		config := &Config{
			Enabled: true,
			Rate:    0.1,
		}
		svc := NewService(nil, nil, log, config)

		c1, _ := svc.Calculate(a1, 1)
		c2, _ := svc.Calculate(a2, 1)

		// If a1 > a2, then c1 should be >= c2
		if a1 > a2 {
			return c1 >= c2
		}
		if a1 < a2 {
			return c1 <= c2
		}
		return c1 == c2
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// Property: Commission rate affects result proportionally
func TestProperty_CommissionRateProportional(t *testing.T) {
	log := logger.NewNopLogger()

	f := func(orderAmount uint32, rate1, rate2 uint8) bool {
		amount := int64(orderAmount)
		if amount == 0 {
			return true
		}

		r1 := float64(rate1%100) / 100
		r2 := float64(rate2%100) / 100

		config1 := &Config{Enabled: true, Rate: r1}
		config2 := &Config{Enabled: true, Rate: r2}

		svc1 := NewService(nil, nil, log, config1)
		svc2 := NewService(nil, nil, log, config2)

		c1, _ := svc1.Calculate(amount, 1)
		c2, _ := svc2.Calculate(amount, 1)

		// Higher rate should result in higher or equal commission
		if r1 > r2 {
			return c1 >= c2
		}
		if r1 < r2 {
			return c1 <= c2
		}
		return c1 == c2
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// Property: Default config has valid values
func TestProperty_DefaultConfigValid(t *testing.T) {
	config := DefaultConfig()

	// Rate should be between 0 and 1
	if config.Rate < 0 || config.Rate > 1 {
		t.Error("Default rate should be between 0 and 1")
	}

	// Settlement delay should be positive
	if config.SettlementDelay <= 0 {
		t.Error("Default settlement delay should be positive")
	}

	// Max level should be at least 1
	if config.MaxLevel < 1 {
		t.Error("Default max level should be at least 1")
	}
}
