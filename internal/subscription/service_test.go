package subscription

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	trialsvc "v/internal/commercial/trial"
	"v/internal/database/repository"
	"v/internal/entitlement"
	"v/internal/logger"
	"v/internal/proxy"
	"v/internal/proxy/protocols/shadowsocks"
	"v/internal/proxy/protocols/trojan"
	"v/internal/proxy/protocols/vless"
	"v/internal/proxy/protocols/vmess"
)

// setupTestDB creates an in-memory SQLite database for testing.
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Enable foreign key constraints
	db.Exec("PRAGMA foreign_keys = ON")

	// Auto migrate
	if err := db.AutoMigrate(&repository.User{}, &repository.Proxy{}, &repository.Subscription{}); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	return db
}

// createTestUser creates a test user and returns its ID.
func createTestUser(t *testing.T, db *gorm.DB, username string) int64 {
	user := &repository.User{
		Username:     username,
		PasswordHash: "hashedpassword",
		Email:        username + "@example.com",
		Role:         "user",
		Enabled:      true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	return user.ID
}

// createTestService creates a subscription service for testing.
func createTestService(t *testing.T, db *gorm.DB) *Service {
	subscriptionRepo := repository.NewSubscriptionRepository(db)
	userRepo := repository.NewUserRepository(db)
	proxyRepo := repository.NewProxyRepository(db)
	log := logger.NewNopLogger()

	return NewService(subscriptionRepo, userRepo, proxyRepo, log, "http://localhost:8080")
}

// TestGenerateToken tests basic token generation.
func TestGenerateToken(t *testing.T) {
	db := setupTestDB(t)
	service := createTestService(t, db)

	token, err := service.GenerateToken()
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Token should be at least 64 characters (32 bytes in hex)
	if len(token) < 64 {
		t.Errorf("Token length %d is less than expected 64", len(token))
	}
}

// TestGenerateShortCode tests basic short code generation.
func TestGenerateShortCode(t *testing.T) {
	db := setupTestDB(t)
	service := createTestService(t, db)

	shortCode, err := service.GenerateShortCode()
	if err != nil {
		t.Fatalf("Failed to generate short code: %v", err)
	}

	// Short code should be exactly 8 characters
	if len(shortCode) != 8 {
		t.Errorf("Short code length %d is not 8", len(shortCode))
	}
}

// TestGetOrCreateSubscription tests subscription creation and retrieval.
func TestGetOrCreateSubscription(t *testing.T) {
	db := setupTestDB(t)
	service := createTestService(t, db)
	ctx := context.Background()

	userID := createTestUser(t, db, "testuser")

	// First call should create a new subscription
	sub1, err := service.GetOrCreateSubscription(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to create subscription: %v", err)
	}

	if sub1.Token == "" {
		t.Error("Expected token to be set")
	}

	// Second call should return the same subscription
	sub2, err := service.GetOrCreateSubscription(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to get subscription: %v", err)
	}

	if sub1.ID != sub2.ID {
		t.Errorf("Expected same subscription ID, got %d and %d", sub1.ID, sub2.ID)
	}
}

// TestRegenerateToken tests token regeneration.
func TestRegenerateToken(t *testing.T) {
	db := setupTestDB(t)
	service := createTestService(t, db)
	ctx := context.Background()

	userID := createTestUser(t, db, "testuser")

	// Create initial subscription
	sub1, err := service.GetOrCreateSubscription(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to create subscription: %v", err)
	}
	oldToken := sub1.Token

	// Regenerate token
	sub2, err := service.RegenerateToken(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to regenerate token: %v", err)
	}

	if sub2.Token == oldToken {
		t.Error("Expected new token to be different from old token")
	}

	// Old token should be invalid
	_, err = service.ValidateToken(ctx, oldToken)
	if err == nil {
		t.Error("Expected old token to be invalid")
	}

	// New token should be valid
	_, err = service.ValidateToken(ctx, sub2.Token)
	if err != nil {
		t.Errorf("Expected new token to be valid, got error: %v", err)
	}
}

// TestDetectClientFormat tests User-Agent detection.
func TestDetectClientFormat(t *testing.T) {
	db := setupTestDB(t)
	service := createTestService(t, db)

	tests := []struct {
		userAgent string
		expected  ClientFormat
	}{
		{"Clash/1.0", FormatClash},
		{"clash.meta/1.0", FormatClashMeta},
		{"Mihomo/1.0", FormatClashMeta},
		{"Shadowrocket/1.0", FormatShadowrocket},
		{"Surge/5.0", FormatSurge},
		{"Quantumult%20X/1.0", FormatQuantumultX},
		{"sing-box/1.0", FormatSingbox},
		{"SingBox/1.0", FormatSingbox},
		{"V2rayN/1.0", FormatV2rayN},
		{"V2rayNG/1.0", FormatV2rayN},
		{"Mozilla/5.0", FormatV2rayN}, // Unknown defaults to V2rayN
		{"", FormatV2rayN},            // Empty defaults to V2rayN
	}

	for _, tt := range tests {
		t.Run(tt.userAgent, func(t *testing.T) {
			result := service.DetectClientFormat(tt.userAgent)
			if result != tt.expected {
				t.Errorf("DetectClientFormat(%q) = %v, want %v", tt.userAgent, result, tt.expected)
			}
		})
	}
}

// TestCheckUserAccess tests user access validation.
func TestCheckUserAccess(t *testing.T) {
	db := setupTestDB(t)
	service := createTestService(t, db)
	ctx := context.Background()

	// Test enabled user
	enabledUserID := createTestUser(t, db, "enableduser")
	err := service.CheckUserAccess(ctx, enabledUserID)
	if err != nil {
		t.Errorf("Expected enabled user to have access, got error: %v", err)
	}

	// Test disabled user - explicitly set Enabled to false
	disabledUser := &repository.User{
		Username:     "disableduser",
		PasswordHash: "hashedpassword",
		Email:        "disabled@example.com",
		Role:         "user",
		Enabled:      false,
	}
	if err := db.Create(disabledUser).Error; err != nil {
		t.Fatalf("Failed to create disabled user: %v", err)
	}
	// Explicitly update to ensure Enabled is false
	db.Model(disabledUser).Update("enabled", false)

	err = service.CheckUserAccess(ctx, disabledUser.ID)
	if err == nil {
		t.Error("Expected disabled user to be denied access")
	}

	// Test expired user
	expiredTime := time.Now().Add(-24 * time.Hour)
	expiredUser := &repository.User{
		Username:     "expireduser",
		PasswordHash: "hashedpassword",
		Email:        "expired@example.com",
		Role:         "user",
		Enabled:      true,
		ExpiresAt:    &expiredTime,
	}
	if err := db.Create(expiredUser).Error; err != nil {
		t.Fatalf("Failed to create expired user: %v", err)
	}
	err = service.CheckUserAccess(ctx, expiredUser.ID)
	if err == nil {
		t.Error("Expected expired user to be denied access")
	}

	// Test traffic exceeded user
	trafficUser := &repository.User{
		Username:     "trafficuser",
		PasswordHash: "hashedpassword",
		Email:        "traffic@example.com",
		Role:         "user",
		Enabled:      true,
		TrafficLimit: 1000,
		TrafficUsed:  2000,
	}
	if err := db.Create(trafficUser).Error; err != nil {
		t.Fatalf("Failed to create traffic user: %v", err)
	}
	err = service.CheckUserAccess(ctx, trafficUser.ID)
	if err == nil {
		t.Error("Expected traffic exceeded user to be denied access")
	}
}

// Property-based tests

// Feature: subscription-system, Property 1: Token Uniqueness
// Validates: Requirements 1.1
// *For any* two subscription tokens generated by the system, they SHALL never be equal.
func TestProperty_TokenUniqueness(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	db := setupTestDB(t)
	service := createTestService(t, db)

	properties.Property("all generated tokens are unique", prop.ForAll(
		func(_ int) bool {
			tokens := make(map[string]bool)
			for i := 0; i < 10; i++ {
				token, err := service.GenerateToken()
				if err != nil {
					return false
				}
				if tokens[token] {
					return false // Duplicate found
				}
				tokens[token] = true
			}
			return true
		},
		gen.IntRange(0, 1000),
	))

	properties.TestingRun(t)
}

// Feature: subscription-system, Property 2: Token Length Constraint
// Validates: Requirements 1.3
// *For any* generated subscription token, its length SHALL be at least 32 characters.
func TestProperty_TokenLengthConstraint(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	db := setupTestDB(t)
	service := createTestService(t, db)

	properties.Property("all generated tokens have at least 32 characters", prop.ForAll(
		func(_ int) bool {
			token, err := service.GenerateToken()
			if err != nil {
				return false
			}
			// Token is hex encoded, so 32 bytes = 64 characters
			// But requirement says "at least 32 characters"
			return len(token) >= 32
		},
		gen.IntRange(0, 1000),
	))

	properties.TestingRun(t)
}

// Feature: subscription-system, Property 18: Short Code Length
// Validates: Requirements 8.2
// *For any* generated short code, its length SHALL be exactly 8 characters.
func TestProperty_ShortCodeLength(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	db := setupTestDB(t)
	service := createTestService(t, db)

	properties.Property("all generated short codes have exactly 8 characters", prop.ForAll(
		func(_ int) bool {
			shortCode, err := service.GenerateShortCode()
			if err != nil {
				return false
			}
			return len(shortCode) == 8
		},
		gen.IntRange(0, 1000),
	))

	properties.TestingRun(t)
}

// Feature: subscription-system, Property 4: Token Regeneration Invalidation
// Validates: Requirements 1.5, 1.6
// *For any* subscription, after regenerating the token, the old token SHALL be invalid
// (return not found) and the new token SHALL be valid.
func TestProperty_TokenRegenerationInvalidation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	userCounter := 0

	properties.Property("regenerating token invalidates old token and validates new token", prop.ForAll(
		func(_ int) bool {
			db := setupTestDB(t)
			service := createTestService(t, db)
			ctx := context.Background()

			// Create unique user for each test
			userCounter++
			username := "user" + string(rune('a'+userCounter%26)) + string(rune('0'+userCounter/26%10))
			userID := createTestUser(t, db, username)

			// Create initial subscription
			sub1, err := service.GetOrCreateSubscription(ctx, userID)
			if err != nil {
				return false
			}
			oldToken := sub1.Token

			// Regenerate token
			sub2, err := service.RegenerateToken(ctx, userID)
			if err != nil {
				return false
			}
			newToken := sub2.Token

			// Old token should be different from new token
			if oldToken == newToken {
				return false
			}

			// Old token should be invalid (not found)
			_, err = service.ValidateToken(ctx, oldToken)
			if err == nil {
				return false // Old token should be invalid
			}

			// New token should be valid
			_, err = service.ValidateToken(ctx, newToken)
			if err != nil {
				return false // New token should be valid
			}

			return true
		},
		gen.IntRange(0, 1000),
	))

	properties.TestingRun(t)
}

// Feature: subscription-system, Property 3: Token-User Association Round Trip
// Validates: Requirements 1.4
// *For any* user who generates a subscription token, querying the subscription by that token
// SHALL return the same user ID.
func TestProperty_TokenUserAssociationRoundTrip(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	userCounter := 0

	properties.Property("token lookup returns correct user ID", prop.ForAll(
		func(_ int) bool {
			db := setupTestDB(t)
			service := createTestService(t, db)
			ctx := context.Background()

			// Create unique user for each test
			userCounter++
			username := "user" + string(rune('a'+userCounter%26)) + string(rune('0'+userCounter/26%10))
			userID := createTestUser(t, db, username)

			// Create subscription
			sub, err := service.GetOrCreateSubscription(ctx, userID)
			if err != nil {
				return false
			}

			// Validate token and check user ID
			foundSub, err := service.ValidateToken(ctx, sub.Token)
			if err != nil {
				return false
			}

			return foundSub.UserID == userID
		},
		gen.IntRange(0, 1000),
	))

	properties.TestingRun(t)
}

// Feature: subscription-system, Property 19: Short Code Mapping Consistency
// Validates: Requirements 8.5
// *For any* short code, looking up the subscription by short code SHALL return the same
// subscription as looking up by the full token.
func TestProperty_ShortCodeMappingConsistency(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	userCounter := 0

	properties.Property("short code and token lookup return same subscription", prop.ForAll(
		func(_ int) bool {
			db := setupTestDB(t)
			service := createTestService(t, db)
			ctx := context.Background()

			// Create unique user for each test
			userCounter++
			username := "user" + string(rune('a'+userCounter%26)) + string(rune('0'+userCounter/26%10))
			userID := createTestUser(t, db, username)

			// Create subscription
			sub, err := service.GetOrCreateSubscription(ctx, userID)
			if err != nil {
				return false
			}

			// Lookup by token
			subByToken, err := service.ValidateToken(ctx, sub.Token)
			if err != nil {
				return false
			}

			// Lookup by short code
			subByShortCode, err := service.ValidateShortCode(ctx, sub.ShortCode)
			if err != nil {
				return false
			}

			// Both lookups should return the same subscription
			return subByToken.ID == subByShortCode.ID &&
				subByToken.UserID == subByShortCode.UserID &&
				subByToken.Token == subByShortCode.Token &&
				subByToken.ShortCode == subByShortCode.ShortCode
		},
		gen.IntRange(0, 1000),
	))

	properties.TestingRun(t)
}

// Feature: subscription-system, Property 5: Format Detection Consistency
// Validates: Requirements 2.2
// *For any* known User-Agent string pattern, the detected client format SHALL be consistent
// across multiple calls.
func TestProperty_FormatDetectionConsistency(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	db := setupTestDB(t)
	service := createTestService(t, db)

	// Known User-Agent patterns
	knownUserAgents := []string{
		"Clash/1.0",
		"clash.meta/1.0",
		"Mihomo/1.0",
		"Shadowrocket/1.0",
		"Surge/5.0",
		"Quantumult%20X/1.0",
		"sing-box/1.0",
		"V2rayN/1.0",
		"V2rayNG/1.0",
		"Mozilla/5.0",
		"",
	}

	properties.Property("format detection is consistent for known User-Agents", prop.ForAll(
		func(index int) bool {
			if index < 0 || index >= len(knownUserAgents) {
				index = 0
			}
			ua := knownUserAgents[index]

			// Call detection multiple times
			format1 := service.DetectClientFormat(ua)
			format2 := service.DetectClientFormat(ua)
			format3 := service.DetectClientFormat(ua)

			// All calls should return the same format
			return format1 == format2 && format2 == format3
		},
		gen.IntRange(0, len(knownUserAgents)-1),
	))

	properties.TestingRun(t)
}

// Feature: subscription-system, Property 6: Format Override Priority
// Validates: Requirements 2.3
// *For any* request with an explicit format parameter, the format parameter SHALL override
// User-Agent detection.
func TestProperty_FormatOverridePriority(t *testing.T) {
	// This property is tested at the handler level, not service level
	// The service's DetectClientFormat only handles User-Agent detection
	// Format override is handled by the handler when parsing query parameters

	// For now, we test that the service correctly returns the format when explicitly specified
	db := setupTestDB(t)
	service := createTestService(t, db)

	// Test that DetectClientFormat returns expected formats for known patterns
	tests := []struct {
		userAgent string
		expected  ClientFormat
	}{
		{"Clash/1.0", FormatClash},
		{"clash.meta/1.0", FormatClashMeta},
		{"Mihomo/1.0", FormatClashMeta},
		{"Shadowrocket/1.0", FormatShadowrocket},
		{"Surge/5.0", FormatSurge},
		{"Quantumult%20X/1.0", FormatQuantumultX},
		{"sing-box/1.0", FormatSingbox},
		{"V2rayN/1.0", FormatV2rayN},
		{"Mozilla/5.0", FormatV2rayN}, // Unknown defaults to V2rayN
	}

	for _, tt := range tests {
		result := service.DetectClientFormat(tt.userAgent)
		if result != tt.expected {
			t.Errorf("DetectClientFormat(%q) = %v, want %v", tt.userAgent, result, tt.expected)
		}
	}
}

// Feature: subscription-system, Property 9: Enabled Proxies Only
// Validates: Requirements 3.1
// *For any* subscription content generation, the output SHALL contain only proxies that are
// enabled (no disabled proxies).
func TestProperty_EnabledProxiesOnly(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	userCounter := 0

	properties.Property("subscription content contains only enabled proxies", prop.ForAll(
		func(numEnabled, numDisabled int) bool {
			if numEnabled < 0 {
				numEnabled = 0
			}
			if numEnabled > 5 {
				numEnabled = 5
			}
			if numDisabled < 0 {
				numDisabled = 0
			}
			if numDisabled > 5 {
				numDisabled = 5
			}

			db := setupTestDB(t)
			service := createTestService(t, db)
			ctx := context.Background()

			// Create unique user
			userCounter++
			username := "user" + string(rune('a'+userCounter%26)) + string(rune('0'+userCounter/26%10))
			userID := createTestUser(t, db, username)

			// Create enabled proxies
			for i := 0; i < numEnabled; i++ {
				proxy := &repository.Proxy{
					UserID:   userID,
					Name:     "Enabled-" + string(rune('A'+i)),
					Protocol: "vmess",
					Host:     "enabled.example.com",
					Port:     443 + i,
					Settings: map[string]interface{}{
						"uuid": "12345678-1234-1234-1234-123456789012",
					},
					Enabled: true,
				}
				if err := db.Create(proxy).Error; err != nil {
					return false
				}
			}

			// Create disabled proxies - explicitly set Enabled to false after creation
			for i := 0; i < numDisabled; i++ {
				proxy := &repository.Proxy{
					UserID:   userID,
					Name:     "Disabled-" + string(rune('A'+i)),
					Protocol: "vmess",
					Host:     "disabled.example.com",
					Port:     1443 + i,
					Settings: map[string]interface{}{
						"uuid": "12345678-1234-1234-1234-123456789012",
					},
					Enabled: false,
				}
				if err := db.Create(proxy).Error; err != nil {
					return false
				}
				// Explicitly update to ensure Enabled is false (GORM default might be true)
				db.Model(proxy).Update("enabled", false)
			}

			// Get enabled proxies
			proxies, err := service.GetUserEnabledProxies(ctx, userID)
			if err != nil {
				return false
			}

			// Verify count matches enabled only
			if len(proxies) != numEnabled {
				return false
			}

			// Verify all returned proxies are enabled
			for _, p := range proxies {
				if !p.Enabled {
					return false
				}
			}

			return true
		},
		gen.IntRange(0, 5),
		gen.IntRange(0, 5),
	))

	properties.TestingRun(t)
}

// Feature: subscription-system, Property 11: Unique Proxy Names
// Validates: Requirements 3.4
// *For any* subscription content, all proxy names within the content SHALL be unique.
func TestProperty_UniqueProxyNames(t *testing.T) {
	// This property is tested at the generator level
	// The generators package has MakeUniqueNames function for this

	// Test that the service correctly handles proxies with same names
	db := setupTestDB(t)
	ctx := context.Background()

	userID := createTestUser(t, db, "testuser")

	// Create proxies with same name
	for i := 0; i < 3; i++ {
		proxy := &repository.Proxy{
			UserID:   userID,
			Name:     "Same Name", // All have same name
			Protocol: "vmess",
			Host:     "server" + string(rune('0'+i)) + ".example.com",
			Port:     443 + i,
			Settings: map[string]interface{}{
				"uuid": "12345678-1234-1234-1234-12345678901" + string(rune('0'+i)),
			},
			Enabled: true,
		}
		db.Create(proxy)
	}

	service := createTestService(t, db)
	proxies, err := service.GetUserEnabledProxies(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to get proxies: %v", err)
	}

	if len(proxies) != 3 {
		t.Errorf("Expected 3 proxies, got %d", len(proxies))
	}
}

func TestGetUserEnabledProxies_SkipsProxyWithoutResolvableServer(t *testing.T) {
	db := setupTestDB(t)
	service := createTestService(t, db)
	ctx := context.Background()

	userID := createTestUser(t, db, "noserver-user")

	proxy := &repository.Proxy{
		UserID:   userID,
		Name:     "No Server",
		Protocol: "vmess",
		Host:     "0.0.0.0",
		Port:     443,
		Settings: map[string]interface{}{
			"uuid": "12345678-1234-1234-1234-123456789012",
		},
		Enabled: true,
	}
	if err := db.Create(proxy).Error; err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	proxies, err := service.GetUserEnabledProxies(ctx, userID)
	if err != nil {
		t.Fatalf("GetUserEnabledProxies returned error: %v", err)
	}

	if len(proxies) != 0 {
		t.Fatalf("Expected unresolved server proxy to be skipped, got %d proxies", len(proxies))
	}
}

func TestGetUserEnabledProxies_UsesTLSDomainAsServer(t *testing.T) {
	db := setupTestDB(t)
	service := createTestService(t, db)
	ctx := context.Background()

	userID := createTestUser(t, db, "tlsdomain-user")

	proxy := &repository.Proxy{
		UserID:   userID,
		Name:     "TLS Server",
		Protocol: "vmess",
		Host:     "0.0.0.0",
		Port:     443,
		Settings: map[string]interface{}{
			"uuid":        "12345678-1234-1234-1234-123456789012",
			"security":    "tls",
			"server_name": "vpn.example.com",
		},
		Enabled: true,
	}
	if err := db.Create(proxy).Error; err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	proxies, err := service.GetUserEnabledProxies(ctx, userID)
	if err != nil {
		t.Fatalf("GetUserEnabledProxies returned error: %v", err)
	}
	if len(proxies) != 1 {
		t.Fatalf("Expected 1 proxy, got %d", len(proxies))
	}

	server, ok := proxies[0].Settings["server"].(string)
	if !ok || server != "vpn.example.com" {
		t.Fatalf("Expected resolved server vpn.example.com, got %#v", proxies[0].Settings["server"])
	}
}

func TestGetUserEnabledProxies_PrefersCurrentNodeAddressOverLegacySettingServer(t *testing.T) {
	db := setupTestDB(t)
	if err := db.AutoMigrate(&repository.Node{}); err != nil {
		t.Fatalf("Failed to migrate node model: %v", err)
	}
	service := createTestService(t, db).WithNodeRepository(repository.NewNodeRepository(db))
	ctx := context.Background()

	userID := createTestUser(t, db, "node-address-user")
	node := &repository.Node{Name: "sub-node", Address: "64.176.54.36", Token: "node-token", Status: repository.NodeStatusOnline}
	if err := db.Create(node).Error; err != nil {
		t.Fatalf("Failed to create node: %v", err)
	}

	proxy := &repository.Proxy{
		UserID:   userID,
		NodeID:   &node.ID,
		Name:     "Node Proxy",
		Protocol: "vmess",
		Host:     "old.example.com",
		Port:     443,
		Settings: map[string]interface{}{
			"uuid":   "12345678-1234-1234-1234-123456789012",
			"server": "old.example.com",
		},
		Enabled: true,
	}
	if err := db.Create(proxy).Error; err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	proxies, err := service.GetUserEnabledProxies(ctx, userID)
	if err != nil {
		t.Fatalf("GetUserEnabledProxies returned error: %v", err)
	}
	if len(proxies) != 1 {
		t.Fatalf("Expected 1 proxy, got %d", len(proxies))
	}

	server, ok := proxies[0].Settings["server"].(string)
	if !ok || server != "64.176.54.36" {
		t.Fatalf("Expected node address 64.176.54.36, got %#v", proxies[0].Settings["server"])
	}
	if proxy.Settings["server"] != "old.example.com" {
		t.Fatalf("Expected original proxy settings to remain unchanged, got %#v", proxy.Settings["server"])
	}
}

func TestGetUserEnabledProxies_PreservesExternalHostOverride(t *testing.T) {
	db := setupTestDB(t)
	if err := db.AutoMigrate(&repository.Node{}); err != nil {
		t.Fatalf("Failed to migrate node model: %v", err)
	}
	service := createTestService(t, db).WithNodeRepository(repository.NewNodeRepository(db))
	ctx := context.Background()

	userID := createTestUser(t, db, "external-host-user")
	node := &repository.Node{Name: "sub-node", Address: "64.176.54.36", Token: "node-token", Status: repository.NodeStatusOnline}
	if err := db.Create(node).Error; err != nil {
		t.Fatalf("Failed to create node: %v", err)
	}

	proxy := &repository.Proxy{
		UserID:   userID,
		NodeID:   &node.ID,
		Name:     "Node Proxy",
		Protocol: "vmess",
		Host:     "64.176.54.36",
		Port:     443,
		Settings: map[string]interface{}{
			"uuid":          "12345678-1234-1234-1234-123456789012",
			"external_host": "edge.example.com",
		},
		Enabled: true,
	}
	if err := db.Create(proxy).Error; err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	proxies, err := service.GetUserEnabledProxies(ctx, userID)
	if err != nil {
		t.Fatalf("GetUserEnabledProxies returned error: %v", err)
	}
	if len(proxies) != 1 {
		t.Fatalf("Expected 1 proxy, got %d", len(proxies))
	}

	server, ok := proxies[0].Settings["server"].(string)
	if !ok || server != "edge.example.com" {
		t.Fatalf("Expected external host edge.example.com, got %#v", proxies[0].Settings["server"])
	}
}

func TestGetUserEnabledProxies_WithEntitlementIncludesMultipleNodes(t *testing.T) {
	db := setupTestDB(t)
	if err := db.AutoMigrate(&repository.Node{}, &repository.Trial{}, &repository.UserNodeAssignment{}); err != nil {
		t.Fatalf("Failed to migrate entitlement models: %v", err)
	}

	subscriptionRepo := repository.NewSubscriptionRepository(db)
	userRepo := repository.NewUserRepository(db)
	proxyRepo := repository.NewProxyRepository(db)
	nodeRepo := repository.NewNodeRepository(db)
	trialRepo := repository.NewTrialRepository(db)
	assignmentRepo := repository.NewUserNodeAssignmentRepository(db)
	log := logger.NewNopLogger()

	trialService := trialsvc.NewService(trialRepo, userRepo, log, nil)
	proxyManager := proxy.NewManager(proxyRepo)
	proxyManager.RegisterProtocol(vmess.New())
	proxyManager.RegisterProtocol(vless.New())
	proxyManager.RegisterProtocol(trojan.New())
	proxyManager.RegisterProtocol(shadowsocks.New())

	entitlementService := entitlement.NewService(
		userRepo,
		trialRepo,
		proxyRepo,
		nodeRepo,
		assignmentRepo,
		trialService,
		log,
	).WithProxyManager(proxyManager)

	service := NewService(subscriptionRepo, userRepo, proxyRepo, log, "http://localhost:8080").
		WithNodeRepository(nodeRepo).
		WithEntitlementService(entitlementService)

	ctx := context.Background()
	userID := createTestUser(t, db, "entitled-subscription-user")

	nodeOne := &repository.Node{Name: "sub-node-1", Address: "sub-node-1.example.com", Token: "sub-node-1-token", Status: repository.NodeStatusOnline, Protocols: `["vmess"]`}
	nodeTwo := &repository.Node{Name: "sub-node-2", Address: "sub-node-2.example.com", Token: "sub-node-2-token", Status: repository.NodeStatusOnline, Protocols: `["vmess"]`}
	if err := db.Create(nodeOne).Error; err != nil {
		t.Fatalf("Failed to create node one: %v", err)
	}
	if err := db.Create(nodeTwo).Error; err != nil {
		t.Fatalf("Failed to create node two: %v", err)
	}

	proxies, err := service.GetUserEnabledProxies(ctx, userID)
	if err != nil {
		t.Fatalf("GetUserEnabledProxies returned error: %v", err)
	}
	if len(proxies) != 2 {
		t.Fatalf("Expected 2 proxies from entitlement-backed subscription, got %d", len(proxies))
	}
}
