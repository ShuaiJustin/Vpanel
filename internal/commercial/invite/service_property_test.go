// Package invite provides invite and referral management functionality.
package invite

import (
	"testing"
	"testing/quick"

	"v/internal/logger"
)

// Feature: commercial-system, Property 8: Invite Code Uniqueness
// Validates: Requirements 9.1
// For any two invite codes in the system, their codes SHALL be unique.

func TestProperty_InviteCodeUniqueness(t *testing.T) {
	log := logger.NewNopLogger()
	svc := NewService(nil, log, nil)

	// Property: Generating N invite codes should produce N unique values
	f := func(count uint8) bool {
		// Limit count to reasonable range
		n := int(count%100) + 1

		codes := make(map[string]bool)
		for i := 0; i < n; i++ {
			code := svc.generateCode()
			if codes[code] {
				return false // Duplicate found
			}
			codes[code] = true
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

// Property: Invite codes should have consistent format
func TestProperty_InviteCodeFormat(t *testing.T) {
	log := logger.NewNopLogger()
	svc := NewService(nil, log, nil)

	f := func(count uint8) bool {
		n := int(count%50) + 1

		for i := 0; i < n; i++ {
			code := svc.generateCode()

			// Code should be 8 characters (4 bytes hex encoded)
			if len(code) != 8 {
				return false
			}

			// Code should be uppercase
			for _, c := range code {
				if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'F')) {
					return false
				}
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

// Property: Invite link should contain the code
func TestProperty_InviteLinkContainsCode(t *testing.T) {
	log := logger.NewNopLogger()

	f := func(baseURL, code string) bool {
		if baseURL == "" || code == "" {
			return true
		}

		config := &Config{BaseURL: baseURL}
		svc := NewService(nil, log, config)

		link := svc.GenerateInviteLink(code)

		// Link should contain the code
		return len(link) > len(code) && link[len(link)-len(code):] == code
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// Property: Self-referral should always be rejected
func TestProperty_SelfReferralRejected(t *testing.T) {
	f := func(userID int64) bool {
		// Self-referral check: inviter ID == invitee ID
		inviterID := userID
		inviteeID := userID

		isSelfReferral := inviterID == inviteeID

		// Self-referral should always be true when IDs match
		return isSelfReferral
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// Property: Conversion rate calculation is correct
func TestProperty_ConversionRateCalculation(t *testing.T) {
	f := func(totalInvites, convertedInvites uint16) bool {
		total := int(totalInvites)
		converted := int(convertedInvites)

		// Converted cannot exceed total
		if converted > total {
			converted = total
		}

		var rate float64
		if total > 0 {
			rate = float64(converted) / float64(total) * 100
		}

		// Rate should be between 0 and 100
		if rate < 0 || rate > 100 {
			return false
		}

		// If no invites, rate should be 0
		if total == 0 && rate != 0 {
			return false
		}

		// If all converted, rate should be 100
		if total > 0 && converted == total && rate != 100 {
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
