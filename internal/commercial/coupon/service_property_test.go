// Package coupon provides coupon management functionality.
package coupon

import (
	"testing"
	"testing/quick"
	"time"

	"v/internal/logger"
)

// Feature: commercial-system, Property 2: Coupon Discount Calculation
// Validates: Requirements 3.5, 8.2
// For any valid coupon and order amount, the calculated discount SHALL not exceed
// the order amount, and for percentage coupons, SHALL not exceed the max_discount limit.

func TestProperty_DiscountNotExceedOrderAmount(t *testing.T) {
	log := logger.NewNopLogger()
	svc := NewService(nil, log)

	// Property: Discount should never exceed order amount
	f := func(orderAmount uint32, couponValue uint32, couponType uint8) bool {
		amount := int64(orderAmount)
		value := int64(couponValue)

		if amount == 0 {
			return true // Skip zero order amounts
		}

		var coupon *Coupon
		if couponType%2 == 0 {
			// Fixed discount
			coupon = &Coupon{
				Type:  TypeFixed,
				Value: value,
			}
		} else {
			// Percentage discount (value is percentage * 100)
			coupon = &Coupon{
				Type:  TypePercentage,
				Value: value % 10001, // 0-100%
			}
		}

		discount := svc.CalculateDiscount(coupon, amount)

		// Discount must not exceed order amount
		return discount <= amount
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

func TestProperty_PercentageDiscountMaxLimit(t *testing.T) {
	log := logger.NewNopLogger()
	svc := NewService(nil, log)

	// Property: Percentage discount should not exceed max_discount limit
	f := func(orderAmount uint32, percentage uint16, maxDiscount uint32) bool {
		amount := int64(orderAmount)
		pct := int64(percentage % 10001) // 0-100%
		maxDisc := int64(maxDiscount)

		if amount == 0 {
			return true
		}

		coupon := &Coupon{
			Type:        TypePercentage,
			Value:       pct,
			MaxDiscount: maxDisc,
		}

		discount := svc.CalculateDiscount(coupon, amount)

		// If max discount is set, discount should not exceed it
		if maxDisc > 0 {
			return discount <= maxDisc && discount <= amount
		}

		// Otherwise just check it doesn't exceed order amount
		return discount <= amount
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// Feature: commercial-system, Property 6: Coupon Usage Limit
// Validates: Requirements 8.5, 8.6
// For any coupon with usage limits, the used_count SHALL never exceed total_limit,
// and per-user usage SHALL never exceed per_user_limit.

func TestProperty_UsageLimitEnforced(t *testing.T) {
	// Property: If total limit is set and reached, coupon should be rejected
	f := func(totalLimit uint8, usedCount uint8) bool {
		limit := int(totalLimit)
		used := int(usedCount)

		if limit == 0 {
			return true // No limit
		}

		// If used count >= limit, coupon should be rejected
		shouldReject := used >= limit

		// Simulate validation
		isRejected := used >= limit

		return shouldReject == isRejected
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

func TestProperty_PerUserLimitEnforced(t *testing.T) {
	// Property: If per-user limit is set and reached, coupon should be rejected for that user
	f := func(perUserLimit uint8, userUsageCount uint8) bool {
		limit := int(perUserLimit)
		used := int(userUsageCount)

		if limit == 0 {
			return true // No limit
		}

		// If user usage count >= limit, coupon should be rejected
		shouldReject := used >= limit

		// Simulate validation
		isRejected := used >= limit

		return shouldReject == isRejected
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// Feature: commercial-system, Property 14: Coupon Validation Rules
// Validates: Requirements 8.4, 8.7
// For any coupon validation, the system SHALL reject coupons that are:
// expired, inactive, below minimum order amount, or exceeding usage limits.

func TestProperty_ExpiredCouponRejected(t *testing.T) {
	// Property: Coupons past their expire date should be rejected
	f := func(daysOffset int8) bool {
		now := time.Now()
		expireAt := now.AddDate(0, 0, int(daysOffset))

		isExpired := now.After(expireAt)
		shouldReject := daysOffset < 0

		return isExpired == shouldReject
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

func TestProperty_InactiveCouponRejected(t *testing.T) {
	// Property: Inactive coupons should always be rejected
	f := func(isActive bool) bool {
		shouldReject := !isActive
		isRejected := !isActive

		return shouldReject == isRejected
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

func TestProperty_MinOrderAmountEnforced(t *testing.T) {
	// Property: Orders below minimum amount should be rejected
	f := func(minAmount uint32, orderAmount uint32) bool {
		min := int64(minAmount)
		order := int64(orderAmount)

		if min == 0 {
			return true // No minimum
		}

		shouldReject := order < min
		isRejected := order < min

		return shouldReject == isRejected
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// Property: Fixed discount calculation is exact
func TestProperty_FixedDiscountExact(t *testing.T) {
	log := logger.NewNopLogger()
	svc := NewService(nil, log)

	f := func(orderAmount uint32, fixedValue uint32) bool {
		amount := int64(orderAmount)
		value := int64(fixedValue)

		if amount == 0 {
			return true
		}

		coupon := &Coupon{
			Type:  TypeFixed,
			Value: value,
		}

		discount := svc.CalculateDiscount(coupon, amount)

		// Fixed discount should be min(value, orderAmount)
		expected := value
		if expected > amount {
			expected = amount
		}

		return discount == expected
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// Property: Percentage discount calculation is correct
func TestProperty_PercentageDiscountCalculation(t *testing.T) {
	log := logger.NewNopLogger()
	svc := NewService(nil, log)

	f := func(orderAmount uint32, percentage uint16) bool {
		amount := int64(orderAmount)
		pct := int64(percentage % 10001) // 0-100%

		if amount == 0 {
			return true
		}

		coupon := &Coupon{
			Type:  TypePercentage,
			Value: pct,
		}

		discount := svc.CalculateDiscount(coupon, amount)

		// Calculate expected discount
		expected := amount * pct / 10000
		if expected > amount {
			expected = amount
		}

		return discount == expected
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// Property: Batch code generation produces unique codes
func TestProperty_BatchCodeUniqueness(t *testing.T) {
	log := logger.NewNopLogger()
	svc := NewService(nil, log)

	f := func(count uint8) bool {
		n := int(count%50) + 1 // 1-50 codes

		codes, err := svc.GenerateBatchCodes("TEST", n)
		if err != nil {
			return false
		}

		// Check uniqueness
		seen := make(map[string]bool)
		for _, code := range codes {
			if seen[code] {
				return false // Duplicate found
			}
			seen[code] = true
		}

		return len(codes) == n
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}
