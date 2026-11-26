// Package giftcard provides gift card management functionality.
package giftcard

import (
	"context"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"v/internal/commercial/balance"
	"v/internal/database/repository"
	"v/internal/logger"
)

// setupTestDB creates an in-memory SQLite database for testing.
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Create tables
	err = db.AutoMigrate(
		&repository.GiftCard{},
		&repository.User{},
		&repository.BalanceTransaction{},
	)
	if err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	return db
}

// createTestUser creates a test user in the database.
func createTestUser(db *gorm.DB, userID int64) error {
	user := &repository.User{
		ID:           userID,
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: "hashedpassword",
	}
	return db.Create(user).Error
}

// Feature: commercial-system, Property 20: Gift Card Redemption
// Validates: Requirements 20.6
// *For any* gift card redemption, the user's balance SHALL increase by exactly the gift card value,
// and the gift card status SHALL change to redeemed.
func TestProperty_GiftCardRedemption(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("redemption increases balance by exact value", prop.ForAll(
		func(giftCardValue int64, userID int64) bool {
			if giftCardValue <= 0 {
				giftCardValue = 100
			}
			if userID <= 0 {
				userID = 1
			}

			db := setupTestDB(t)
			giftCardRepo := repository.NewGiftCardRepository(db)
			balanceRepo := repository.NewBalanceRepository(db)
			log := logger.NewNopLogger()

			// Create test user
			if err := createTestUser(db, userID); err != nil {
				t.Logf("Failed to create user: %v", err)
				return false
			}

			// Create balance service (only 2 args)
			balanceSvc := balance.NewService(balanceRepo, log)

			// Create gift card service
			svc := NewService(giftCardRepo, balanceSvc, log)
			ctx := context.Background()

			// Get initial balance
			initialBalance, err := balanceSvc.GetBalance(ctx, userID)
			if err != nil {
				t.Logf("Failed to get initial balance: %v", err)
				return false
			}

			// Create a gift card
			adminID := int64(999)
			createTestUser(db, adminID) // Create admin user
			req := &CreateBatchRequest{
				Count: 1,
				Value: giftCardValue,
			}
			cards, _, err := svc.CreateBatch(ctx, req, adminID)
			if err != nil {
				t.Logf("Failed to create gift card: %v", err)
				return false
			}

			// Redeem the gift card
			_, err = svc.Redeem(ctx, cards[0].Code, userID)
			if err != nil {
				t.Logf("Failed to redeem gift card: %v", err)
				return false
			}

			// Get final balance
			finalBalance, err := balanceSvc.GetBalance(ctx, userID)
			if err != nil {
				t.Logf("Failed to get final balance: %v", err)
				return false
			}

			// Verify balance increased by exact gift card value
			expectedBalance := initialBalance + giftCardValue
			if finalBalance != expectedBalance {
				t.Logf("Balance mismatch: expected %d, got %d", expectedBalance, finalBalance)
				return false
			}

			return true
		},
		gen.Int64Range(100, 100000),  // giftCardValue: 1.00 to 1000.00
		gen.Int64Range(1, 1000),      // userID
	))

	properties.Property("redemption changes status to redeemed", prop.ForAll(
		func(giftCardValue int64, userID int64) bool {
			if giftCardValue <= 0 {
				giftCardValue = 100
			}
			if userID <= 0 {
				userID = 1
			}

			db := setupTestDB(t)
			giftCardRepo := repository.NewGiftCardRepository(db)
			balanceRepo := repository.NewBalanceRepository(db)
			log := logger.NewNopLogger()

			// Create test user
			if err := createTestUser(db, userID); err != nil {
				return false
			}

			// Create balance service
			balanceSvc := balance.NewService(balanceRepo, log)

			// Create gift card service
			svc := NewService(giftCardRepo, balanceSvc, log)
			ctx := context.Background()

			// Create a gift card
			adminID := int64(999)
			createTestUser(db, adminID)
			req := &CreateBatchRequest{
				Count: 1,
				Value: giftCardValue,
			}
			cards, _, err := svc.CreateBatch(ctx, req, adminID)
			if err != nil {
				return false
			}

			// Verify initial status is active
			if cards[0].Status != StatusActive {
				t.Logf("Initial status should be active, got %s", cards[0].Status)
				return false
			}

			// Redeem the gift card
			redeemedCard, err := svc.Redeem(ctx, cards[0].Code, userID)
			if err != nil {
				return false
			}

			// Verify status changed to redeemed
			if redeemedCard.Status != StatusRedeemed {
				t.Logf("Status should be redeemed, got %s", redeemedCard.Status)
				return false
			}

			return true
		},
		gen.Int64Range(100, 100000),
		gen.Int64Range(1, 1000),
	))

	properties.TestingRun(t)
}

// Feature: commercial-system, Property 20: Gift Card Redemption (Cannot Redeem Twice)
// Validates: Requirements 20.6
func TestProperty_GiftCardCannotRedeemTwice(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("redeemed gift card cannot be redeemed again", prop.ForAll(
		func(giftCardValue int64, userID1, userID2 int64) bool {
			if giftCardValue <= 0 {
				giftCardValue = 100
			}
			if userID1 <= 0 {
				userID1 = 1
			}
			if userID2 <= 0 || userID2 == userID1 {
				userID2 = userID1 + 1
			}

			db := setupTestDB(t)
			giftCardRepo := repository.NewGiftCardRepository(db)
			balanceRepo := repository.NewBalanceRepository(db)
			log := logger.NewNopLogger()

			// Create test users
			createTestUser(db, userID1)
			createTestUser(db, userID2)

			// Create balance service
			balanceSvc := balance.NewService(balanceRepo, log)

			// Create gift card service
			svc := NewService(giftCardRepo, balanceSvc, log)
			ctx := context.Background()

			// Create a gift card
			adminID := int64(999)
			createTestUser(db, adminID)
			req := &CreateBatchRequest{
				Count: 1,
				Value: giftCardValue,
			}
			cards, _, err := svc.CreateBatch(ctx, req, adminID)
			if err != nil {
				return false
			}

			// First redemption should succeed
			_, err = svc.Redeem(ctx, cards[0].Code, userID1)
			if err != nil {
				t.Logf("First redemption failed: %v", err)
				return false
			}

			// Second redemption should fail
			_, err = svc.Redeem(ctx, cards[0].Code, userID2)
			if err != ErrGiftCardAlreadyUsed {
				t.Logf("Expected ErrGiftCardAlreadyUsed, got: %v", err)
				return false
			}

			return true
		},
		gen.Int64Range(100, 100000),
		gen.Int64Range(1, 500),
		gen.Int64Range(501, 1000),
	))

	properties.TestingRun(t)
}

// Feature: commercial-system, Property 20: Gift Card Redemption (Expiration)
// Validates: Requirements 20.6
func TestProperty_GiftCardExpiration(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("expired gift card cannot be redeemed", prop.ForAll(
		func(giftCardValue int64, userID int64) bool {
			if giftCardValue <= 0 {
				giftCardValue = 100
			}
			if userID <= 0 {
				userID = 1
			}

			db := setupTestDB(t)
			giftCardRepo := repository.NewGiftCardRepository(db)
			balanceRepo := repository.NewBalanceRepository(db)
			log := logger.NewNopLogger()

			// Create test user
			createTestUser(db, userID)

			// Create balance service
			balanceSvc := balance.NewService(balanceRepo, log)

			// Create gift card service
			svc := NewService(giftCardRepo, balanceSvc, log)
			ctx := context.Background()

			// Create an expired gift card directly in database
			expiredAt := time.Now().AddDate(0, 0, -1) // Expired yesterday
			expiredCard := &repository.GiftCard{
				Code:      "EXPIRED-TEST-CODE",
				Value:     giftCardValue,
				Status:    StatusActive, // Still marked active but expired
				ExpiresAt: &expiredAt,
			}
			if err := db.Create(expiredCard).Error; err != nil {
				return false
			}

			// Attempt to redeem should fail
			_, err := svc.Redeem(ctx, expiredCard.Code, userID)
			if err != ErrGiftCardExpired {
				t.Logf("Expected ErrGiftCardExpired, got: %v", err)
				return false
			}

			return true
		},
		gen.Int64Range(100, 100000),
		gen.Int64Range(1, 1000),
	))

	properties.TestingRun(t)
}

// Feature: commercial-system, Property 20: Gift Card Redemption (Code Uniqueness)
// Validates: Requirements 20.6
func TestProperty_GiftCardCodeUniqueness(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("batch creation generates unique codes", prop.ForAll(
		func(batchSize int) bool {
			if batchSize < 2 {
				batchSize = 2
			}
			if batchSize > 100 {
				batchSize = 100
			}

			db := setupTestDB(t)
			giftCardRepo := repository.NewGiftCardRepository(db)
			balanceRepo := repository.NewBalanceRepository(db)
			log := logger.NewNopLogger()

			// Create balance service
			balanceSvc := balance.NewService(balanceRepo, log)

			// Create gift card service
			svc := NewService(giftCardRepo, balanceSvc, log)
			ctx := context.Background()

			// Create admin user
			adminID := int64(999)
			createTestUser(db, adminID)

			// Create batch
			req := &CreateBatchRequest{
				Count: batchSize,
				Value: 1000,
			}
			cards, _, err := svc.CreateBatch(ctx, req, adminID)
			if err != nil {
				t.Logf("Failed to create batch: %v", err)
				return false
			}

			// Verify all codes are unique
			codeSet := make(map[string]bool)
			for _, card := range cards {
				if codeSet[card.Code] {
					t.Logf("Duplicate code found: %s", card.Code)
					return false
				}
				codeSet[card.Code] = true
			}

			return len(codeSet) == batchSize
		},
		gen.IntRange(2, 100),
	))

	properties.TestingRun(t)
}
