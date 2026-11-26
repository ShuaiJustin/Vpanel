// Package plan provides plan management functionality.
package plan

import (
	"testing"
	"testing/quick"
)

// Feature: commercial-system, Property 11: Plan Price Per Month Calculation
// Validates: Requirements 2.4
// For any plan, the monthly price SHALL equal (price / duration) * 30, rounded appropriately.

func TestProperty_MonthlyPriceCalculation(t *testing.T) {
	service := &Service{}

	// Property: For any plan with positive duration, monthly price = (price * 30) / duration
	f := func(price int64, duration int) bool {
		// Constrain inputs to valid ranges
		if price < 0 || price > 1000000000 { // max 10M in cents
			return true // skip invalid inputs
		}
		if duration <= 0 || duration > 3650 { // max 10 years
			return true // skip invalid inputs
		}

		plan := &Plan{
			Price:    price,
			Duration: duration,
		}

		monthlyPrice := service.CalculateMonthlyPrice(plan)
		expected := (price * 30) / int64(duration)

		return monthlyPrice == expected
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// Property: Zero or negative duration returns zero monthly price
func TestProperty_ZeroDurationReturnsZero(t *testing.T) {
	service := &Service{}

	f := func(price int64, duration int) bool {
		if price < 0 {
			return true // skip negative prices
		}
		if duration > 0 {
			return true // skip positive durations
		}

		plan := &Plan{
			Price:    price,
			Duration: duration,
		}

		monthlyPrice := service.CalculateMonthlyPrice(plan)
		return monthlyPrice == 0
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// Property: Monthly price is proportional to price
func TestProperty_MonthlyPriceProportionalToPrice(t *testing.T) {
	service := &Service{}

	f := func(price1, price2 uint32, duration uint16) bool {
		// Constrain inputs to avoid overflow
		if duration == 0 {
			return true
		}
		// Use smaller values to avoid overflow
		p1 := int64(price1 % 100000000) // max 1M in cents
		p2 := int64(price2 % 100000000)
		d := int(duration%3650) + 1 // 1-3650 days

		plan1 := &Plan{Price: p1, Duration: d}
		plan2 := &Plan{Price: p2, Duration: d}

		monthly1 := service.CalculateMonthlyPrice(plan1)
		monthly2 := service.CalculateMonthlyPrice(plan2)

		// If price1 > price2, then monthly1 >= monthly2 (accounting for integer division)
		if p1 > p2 {
			return monthly1 >= monthly2
		}
		if p1 < p2 {
			return monthly1 <= monthly2
		}
		return monthly1 == monthly2
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// Property: Monthly price is inversely proportional to duration
func TestProperty_MonthlyPriceInverselyProportionalToDuration(t *testing.T) {
	service := &Service{}

	f := func(price uint32, duration1, duration2 uint16) bool {
		// Constrain inputs to avoid overflow
		if duration1 == 0 || duration2 == 0 {
			return true
		}
		// Use smaller values to avoid overflow
		p := int64(price % 100000000) // max 1M in cents
		if p == 0 {
			return true
		}
		d1 := int(duration1%3650) + 1 // 1-3650 days
		d2 := int(duration2%3650) + 1

		plan1 := &Plan{Price: p, Duration: d1}
		plan2 := &Plan{Price: p, Duration: d2}

		monthly1 := service.CalculateMonthlyPrice(plan1)
		monthly2 := service.CalculateMonthlyPrice(plan2)

		// If duration1 > duration2, then monthly1 <= monthly2 (longer duration = cheaper per month)
		if d1 > d2 {
			return monthly1 <= monthly2
		}
		if d1 < d2 {
			return monthly1 >= monthly2
		}
		return monthly1 == monthly2
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}
