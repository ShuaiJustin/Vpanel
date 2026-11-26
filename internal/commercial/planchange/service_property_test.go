// Package planchange provides plan upgrade and downgrade functionality.
package planchange

import (
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: commercial-system, Property 16: Plan Change Proration
// Validates: Requirements 16.3
// *For any* plan upgrade, the prorated price SHALL equal (new_price - old_price) * (remaining_days / total_days).
func TestProperty_PlanChangeProration(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Create a service instance for testing (we only need the CalculateProration method)
	svc := &Service{}

	properties.Property("proration formula is correctly applied", prop.ForAll(
		func(oldPrice, newPrice int64, remainingDays, totalDays int) bool {
			// Skip invalid inputs
			if totalDays <= 0 {
				return true
			}
			if remainingDays < 0 || remainingDays > totalDays {
				return true
			}

			result := svc.CalculateProration(oldPrice, newPrice, remainingDays, totalDays)

			// Calculate expected result using the formula
			// Formula: (new_price - old_price) * (remaining_days / total_days)
			priceDiff := newPrice - oldPrice
			expected := (priceDiff * int64(remainingDays)) / int64(totalDays)

			return result == expected
		},
		gen.Int64Range(0, 100000),    // oldPrice: 0 to 1000.00
		gen.Int64Range(0, 100000),    // newPrice: 0 to 1000.00
		gen.IntRange(0, 365),         // remainingDays: 0 to 365
		gen.IntRange(1, 365),         // totalDays: 1 to 365
	))

	properties.Property("proration is zero when remaining days is zero", prop.ForAll(
		func(oldPrice, newPrice int64, totalDays int) bool {
			if totalDays <= 0 {
				return true
			}

			result := svc.CalculateProration(oldPrice, newPrice, 0, totalDays)
			return result == 0
		},
		gen.Int64Range(0, 100000),
		gen.Int64Range(0, 100000),
		gen.IntRange(1, 365),
	))

	properties.Property("proration equals price difference when remaining equals total days", prop.ForAll(
		func(oldPrice, newPrice int64, totalDays int) bool {
			if totalDays <= 0 {
				return true
			}

			result := svc.CalculateProration(oldPrice, newPrice, totalDays, totalDays)
			expected := newPrice - oldPrice

			return result == expected
		},
		gen.Int64Range(0, 100000),
		gen.Int64Range(0, 100000),
		gen.IntRange(1, 365),
	))

	properties.Property("proration is positive for upgrades (new > old)", prop.ForAll(
		func(oldPrice int64, priceDiff int64, remainingDays, totalDays int) bool {
			if totalDays <= 0 || remainingDays <= 0 || priceDiff <= 0 {
				return true
			}
			if remainingDays > totalDays {
				return true
			}

			newPrice := oldPrice + priceDiff
			result := svc.CalculateProration(oldPrice, newPrice, remainingDays, totalDays)

			return result >= 0
		},
		gen.Int64Range(0, 50000),
		gen.Int64Range(1, 50000),
		gen.IntRange(1, 365),
		gen.IntRange(1, 365),
	))

	properties.Property("proration is negative for downgrades (new < old)", prop.ForAll(
		func(newPrice int64, priceDiff int64, remainingDays, totalDays int) bool {
			if totalDays <= 0 || remainingDays <= 0 || priceDiff <= 0 {
				return true
			}
			if remainingDays > totalDays {
				return true
			}

			oldPrice := newPrice + priceDiff
			result := svc.CalculateProration(oldPrice, newPrice, remainingDays, totalDays)

			return result <= 0
		},
		gen.Int64Range(0, 50000),
		gen.Int64Range(1, 50000),
		gen.IntRange(1, 365),
		gen.IntRange(1, 365),
	))

	properties.Property("proration returns zero when totalDays is zero or negative", prop.ForAll(
		func(oldPrice, newPrice int64, remainingDays int) bool {
			result := svc.CalculateProration(oldPrice, newPrice, remainingDays, 0)
			return result == 0
		},
		gen.Int64Range(0, 100000),
		gen.Int64Range(0, 100000),
		gen.IntRange(0, 365),
	))

	properties.Property("proration increases with remaining days", prop.ForAll(
		func(oldPrice, newPrice int64, totalDays int) bool {
			if totalDays <= 2 {
				return true
			}
			// Skip when prices are the same (no proration needed)
			if oldPrice == newPrice {
				return true
			}

			// Test that more remaining days means larger absolute proration
			smallerDays := totalDays / 4
			largerDays := totalDays / 2
			
			if smallerDays == largerDays || smallerDays == 0 {
				return true
			}

			smallerResult := svc.CalculateProration(oldPrice, newPrice, smallerDays, totalDays)
			largerResult := svc.CalculateProration(oldPrice, newPrice, largerDays, totalDays)

			// For upgrades (positive proration), larger days should give larger result
			// For downgrades (negative proration), larger days should give more negative result
			if newPrice > oldPrice {
				// Upgrade: larger days should give larger positive result
				return largerResult >= smallerResult
			}
			// Downgrade: larger days should give more negative result
			return largerResult <= smallerResult
		},
		gen.Int64Range(1, 100000),
		gen.Int64Range(1, 100000),
		gen.IntRange(4, 365),
	))

	properties.TestingRun(t)
}

// Feature: commercial-system, Property 16: Plan Change Proration (Edge Cases)
// Validates: Requirements 16.3
func TestProperty_PlanChangeProrationEdgeCases(t *testing.T) {
	svc := &Service{}

	// Test specific edge cases
	testCases := []struct {
		name          string
		oldPrice      int64
		newPrice      int64
		remainingDays int
		totalDays     int
		expected      int64
	}{
		{
			name:          "same price",
			oldPrice:      10000,
			newPrice:      10000,
			remainingDays: 15,
			totalDays:     30,
			expected:      0,
		},
		{
			name:          "upgrade full period",
			oldPrice:      10000,
			newPrice:      20000,
			remainingDays: 30,
			totalDays:     30,
			expected:      10000,
		},
		{
			name:          "upgrade half period",
			oldPrice:      10000,
			newPrice:      20000,
			remainingDays: 15,
			totalDays:     30,
			expected:      5000,
		},
		{
			name:          "downgrade full period",
			oldPrice:      20000,
			newPrice:      10000,
			remainingDays: 30,
			totalDays:     30,
			expected:      -10000,
		},
		{
			name:          "no remaining days",
			oldPrice:      10000,
			newPrice:      20000,
			remainingDays: 0,
			totalDays:     30,
			expected:      0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := svc.CalculateProration(tc.oldPrice, tc.newPrice, tc.remainingDays, tc.totalDays)
			if result != tc.expected {
				t.Errorf("Expected %d, got %d", tc.expected, result)
			}
		})
	}
}
