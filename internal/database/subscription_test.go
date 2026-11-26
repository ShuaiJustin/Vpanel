package database

import (
	"path/filepath"
	"testing"
	"time"
)

// TestSubscriptionModel tests the Subscription model structure and table name.
func TestSubscriptionModel_TableName(t *testing.T) {
	sub := Subscription{}
	if sub.TableName() != "subscriptions" {
		t.Errorf("Expected table name 'subscriptions', got '%s'", sub.TableName())
	}
}

// TestSubscriptionModel_Fields tests that all required fields are present.
func TestSubscriptionModel_Fields(t *testing.T) {
	now := time.Now()
	sub := Subscription{
		ID:           1,
		UserID:       100,
		Token:        "test-token-12345678901234567890123456",
		ShortCode:    "abc12345",
		CreatedAt:    now,
		UpdatedAt:    now,
		LastAccessAt: &now,
		AccessCount:  10,
		LastIP:       "192.168.1.1",
		LastUA:       "Mozilla/5.0",
	}

	if sub.ID != 1 {
		t.Errorf("Expected ID 1, got %d", sub.ID)
	}
	if sub.UserID != 100 {
		t.Errorf("Expected UserID 100, got %d", sub.UserID)
	}
	if sub.Token != "test-token-12345678901234567890123456" {
		t.Errorf("Expected Token 'test-token-12345678901234567890123456', got '%s'", sub.Token)
	}
	if sub.ShortCode != "abc12345" {
		t.Errorf("Expected ShortCode 'abc12345', got '%s'", sub.ShortCode)
	}
	if sub.AccessCount != 10 {
		t.Errorf("Expected AccessCount 10, got %d", sub.AccessCount)
	}
	if sub.LastIP != "192.168.1.1" {
		t.Errorf("Expected LastIP '192.168.1.1', got '%s'", sub.LastIP)
	}
	if sub.LastUA != "Mozilla/5.0" {
		t.Errorf("Expected LastUA 'Mozilla/5.0', got '%s'", sub.LastUA)
	}
}

// TestSubscriptionModel_NullableFields tests that nullable fields work correctly.
func TestSubscriptionModel_NullableFields(t *testing.T) {
	sub := Subscription{
		ID:           1,
		UserID:       100,
		Token:        "test-token",
		LastAccessAt: nil, // Should be nullable
	}

	if sub.LastAccessAt != nil {
		t.Error("LastAccessAt should be nil")
	}
}

// TestSubscriptionModel_CRUD tests basic CRUD operations with the model.
func TestSubscriptionModel_CRUD(t *testing.T) {
	// Create a temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &Config{
		Driver:             "sqlite",
		DSN:                dbPath,
		MaxRetries:         3,
		RetryInterval:      10 * time.Millisecond,
		SlowQueryThreshold: 200 * time.Millisecond,
	}

	db, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Auto-migrate the Subscription model
	if err := db.DB().AutoMigrate(&Subscription{}); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	// Create
	sub := &Subscription{
		UserID:      1,
		Token:       "test-token-12345678901234567890123456",
		ShortCode:   "abc12345",
		AccessCount: 0,
	}

	if err := db.DB().Create(sub).Error; err != nil {
		t.Fatalf("Failed to create subscription: %v", err)
	}

	if sub.ID == 0 {
		t.Error("Expected ID to be set after create")
	}

	// Read
	var found Subscription
	if err := db.DB().First(&found, sub.ID).Error; err != nil {
		t.Fatalf("Failed to find subscription: %v", err)
	}

	if found.Token != sub.Token {
		t.Errorf("Expected Token '%s', got '%s'", sub.Token, found.Token)
	}

	// Update
	found.AccessCount = 5
	now := time.Now()
	found.LastAccessAt = &now
	found.LastIP = "10.0.0.1"

	if err := db.DB().Save(&found).Error; err != nil {
		t.Fatalf("Failed to update subscription: %v", err)
	}

	// Verify update
	var updated Subscription
	if err := db.DB().First(&updated, sub.ID).Error; err != nil {
		t.Fatalf("Failed to find updated subscription: %v", err)
	}

	if updated.AccessCount != 5 {
		t.Errorf("Expected AccessCount 5, got %d", updated.AccessCount)
	}
	if updated.LastIP != "10.0.0.1" {
		t.Errorf("Expected LastIP '10.0.0.1', got '%s'", updated.LastIP)
	}

	// Delete
	if err := db.DB().Delete(&updated).Error; err != nil {
		t.Fatalf("Failed to delete subscription: %v", err)
	}

	// Verify delete
	var deleted Subscription
	result := db.DB().First(&deleted, sub.ID)
	if result.Error == nil {
		t.Error("Expected error when finding deleted subscription")
	}
}

// TestSubscriptionModel_UniqueConstraints tests unique constraints.
func TestSubscriptionModel_UniqueConstraints(t *testing.T) {
	// Create a temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &Config{
		Driver:             "sqlite",
		DSN:                dbPath,
		MaxRetries:         3,
		RetryInterval:      10 * time.Millisecond,
		SlowQueryThreshold: 200 * time.Millisecond,
	}

	db, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Auto-migrate the Subscription model
	if err := db.DB().AutoMigrate(&Subscription{}); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	// Create first subscription
	sub1 := &Subscription{
		UserID:    1,
		Token:     "unique-token-1234567890123456789012",
		ShortCode: "short001",
	}

	if err := db.DB().Create(sub1).Error; err != nil {
		t.Fatalf("Failed to create first subscription: %v", err)
	}

	// Try to create subscription with same UserID (should fail)
	sub2 := &Subscription{
		UserID:    1, // Same UserID
		Token:     "different-token-12345678901234567890",
		ShortCode: "short002",
	}

	err = db.DB().Create(sub2).Error
	if err == nil {
		t.Error("Expected error when creating subscription with duplicate UserID")
	}

	// Try to create subscription with same Token (should fail)
	sub3 := &Subscription{
		UserID:    2,
		Token:     "unique-token-1234567890123456789012", // Same Token
		ShortCode: "short003",
	}

	err = db.DB().Create(sub3).Error
	if err == nil {
		t.Error("Expected error when creating subscription with duplicate Token")
	}

	// Try to create subscription with same ShortCode (should fail)
	sub4 := &Subscription{
		UserID:    3,
		Token:     "another-token-12345678901234567890",
		ShortCode: "short001", // Same ShortCode
	}

	err = db.DB().Create(sub4).Error
	if err == nil {
		t.Error("Expected error when creating subscription with duplicate ShortCode")
	}
}

// TestSubscriptionModel_TokenByUserID tests finding subscription by user ID.
func TestSubscriptionModel_TokenByUserID(t *testing.T) {
	// Create a temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &Config{
		Driver:             "sqlite",
		DSN:                dbPath,
		MaxRetries:         3,
		RetryInterval:      10 * time.Millisecond,
		SlowQueryThreshold: 200 * time.Millisecond,
	}

	db, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Auto-migrate the Subscription model
	if err := db.DB().AutoMigrate(&Subscription{}); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	// Create subscription
	sub := &Subscription{
		UserID:    42,
		Token:     "user42-token-123456789012345678901234",
		ShortCode: "u42short",
	}

	if err := db.DB().Create(sub).Error; err != nil {
		t.Fatalf("Failed to create subscription: %v", err)
	}

	// Find by UserID
	var found Subscription
	if err := db.DB().Where("user_id = ?", 42).First(&found).Error; err != nil {
		t.Fatalf("Failed to find subscription by UserID: %v", err)
	}

	if found.Token != sub.Token {
		t.Errorf("Expected Token '%s', got '%s'", sub.Token, found.Token)
	}

	// Find by Token
	var foundByToken Subscription
	if err := db.DB().Where("token = ?", sub.Token).First(&foundByToken).Error; err != nil {
		t.Fatalf("Failed to find subscription by Token: %v", err)
	}

	if foundByToken.UserID != 42 {
		t.Errorf("Expected UserID 42, got %d", foundByToken.UserID)
	}

	// Find by ShortCode
	var foundByShortCode Subscription
	if err := db.DB().Where("short_code = ?", "u42short").First(&foundByShortCode).Error; err != nil {
		t.Fatalf("Failed to find subscription by ShortCode: %v", err)
	}

	if foundByShortCode.UserID != 42 {
		t.Errorf("Expected UserID 42, got %d", foundByShortCode.UserID)
	}
}
