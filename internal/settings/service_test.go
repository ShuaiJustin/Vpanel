// Package settings provides system settings management.
package settings

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSettingsRepository is a mock implementation of SettingsRepository for testing.
type mockSettingsRepository struct {
	settings map[string]string
}

func newMockSettingsRepository() *mockSettingsRepository {
	return &mockSettingsRepository{
		settings: make(map[string]string),
	}
}

func (m *mockSettingsRepository) Get(ctx context.Context, key string) (string, error) {
	return m.settings[key], nil
}

func (m *mockSettingsRepository) GetAll(ctx context.Context) (map[string]string, error) {
	result := make(map[string]string, len(m.settings))
	for k, v := range m.settings {
		result[k] = v
	}
	return result, nil
}

func (m *mockSettingsRepository) Set(ctx context.Context, key, value string) error {
	m.settings[key] = value
	return nil
}

func (m *mockSettingsRepository) SetMultiple(ctx context.Context, settings map[string]string) error {
	for k, v := range settings {
		m.settings[k] = v
	}
	return nil
}

func (m *mockSettingsRepository) Delete(ctx context.Context, key string) error {
	delete(m.settings, key)
	return nil
}

func (m *mockSettingsRepository) Backup(ctx context.Context) ([]byte, error) {
	return json.Marshal(m.settings)
}

func (m *mockSettingsRepository) Restore(ctx context.Context, data []byte) error {
	var settings map[string]string
	if err := json.Unmarshal(data, &settings); err != nil {
		return err
	}
	m.settings = settings
	return nil
}

// Feature: project-optimization, Property 27: Settings Persistence
// Validates: Requirements 18.3
// *For any* settings update, the new values SHALL be persisted to the database,
// and subsequent reads SHALL return the updated values.
func TestSettingsPersistence_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("Single setting persistence", prop.ForAll(
		func(key, value string) bool {
			if key == "" {
				return true // Skip empty keys
			}

			repo := newMockSettingsRepository()
			service := NewService(repo)
			ctx := context.Background()

			// Set the value
			err := service.Set(ctx, key, value)
			if err != nil {
				return false
			}

			// Read it back
			readValue, err := service.Get(ctx, key)
			if err != nil {
				return false
			}

			return readValue == value
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString(),
	))

	properties.Property("Multiple settings persistence", prop.ForAll(
		func(settings map[string]string) bool {
			// Filter out empty keys
			filtered := make(map[string]string)
			for k, v := range settings {
				if k != "" {
					filtered[k] = v
				}
			}
			if len(filtered) == 0 {
				return true
			}

			repo := newMockSettingsRepository()
			service := NewService(repo)
			ctx := context.Background()

			// Set multiple values
			err := service.SetMultiple(ctx, filtered)
			if err != nil {
				return false
			}

			// Read all back
			readSettings, err := service.GetAll(ctx)
			if err != nil {
				return false
			}

			// Verify all values match
			for k, v := range filtered {
				if readSettings[k] != v {
					return false
				}
			}

			return true
		},
		gen.MapOf(gen.AlphaString(), gen.AlphaString()),
	))

	properties.Property("Backup and restore preserves settings", prop.ForAll(
		func(settings map[string]string) bool {
			// Filter out empty keys
			filtered := make(map[string]string)
			for k, v := range settings {
				if k != "" {
					filtered[k] = v
				}
			}

			repo := newMockSettingsRepository()
			service := NewService(repo)
			ctx := context.Background()

			// Set initial values
			if len(filtered) > 0 {
				err := service.SetMultiple(ctx, filtered)
				if err != nil {
					return false
				}
			}

			// Create backup
			backup, err := service.Backup(ctx)
			if err != nil {
				return false
			}

			// Clear settings
			repo.settings = make(map[string]string)

			// Restore from backup
			err = service.Restore(ctx, backup)
			if err != nil {
				return false
			}

			// Verify all values match
			readSettings, err := service.GetAll(ctx)
			if err != nil {
				return false
			}

			if len(readSettings) != len(filtered) {
				return false
			}

			for k, v := range filtered {
				if readSettings[k] != v {
					return false
				}
			}

			return true
		},
		gen.MapOf(gen.AlphaString(), gen.AlphaString()),
	))

	properties.TestingRun(t)
}

// Unit tests for specific edge cases

func TestSettingsService_DefaultSettings(t *testing.T) {
	repo := newMockSettingsRepository()
	service := NewService(repo)
	ctx := context.Background()

	settings, err := service.GetSystemSettings(ctx)
	require.NoError(t, err)

	// Check default values
	assert.Equal(t, "V Panel", settings.SiteName)
	assert.Equal(t, false, settings.AllowRegistration)
	assert.Equal(t, 30, settings.DefaultExpiryDays)
	assert.Equal(t, 1440, settings.SessionTimeout)
	assert.False(t, settings.EnableIPWhitelist)
	assert.False(t, settings.EnableLoginLock)
	assert.Equal(t, 5, settings.MaxLoginAttempts)
	assert.Equal(t, 10, settings.LockDuration)
	assert.Equal(t, true, settings.RateLimitEnabled)
}

func TestSettingsService_UpdateSystemSettings(t *testing.T) {
	repo := newMockSettingsRepository()
	service := NewService(repo)
	ctx := context.Background()

	// Update settings
	newSettings := &SystemSettings{
		SiteName:          "My Panel",
		SiteDescription:   "Custom description",
		AllowRegistration: true,
		DefaultExpiryDays: 60,
		RateLimitEnabled:  false,
	}

	err := service.UpdateSystemSettings(ctx, newSettings)
	require.NoError(t, err)

	// Read back
	readSettings, err := service.GetSystemSettings(ctx)
	require.NoError(t, err)

	assert.Equal(t, "My Panel", readSettings.SiteName)
	assert.Equal(t, "Custom description", readSettings.SiteDescription)
	assert.Equal(t, true, readSettings.AllowRegistration)
	assert.Equal(t, 60, readSettings.DefaultExpiryDays)
	assert.Equal(t, false, readSettings.RateLimitEnabled)
}

func TestSettingsService_PaymentSettingsPersistence(t *testing.T) {
	repo := newMockSettingsRepository()
	service := NewService(repo)
	ctx := context.Background()

	newSettings := &SystemSettings{
		PaymentAlipayEnabled:    true,
		PaymentAlipayAppID:      "alipay-app",
		PaymentAlipayPrivateKey: "alipay-private-key",
		PaymentAlipayPublicKey:  "alipay-public-key",
		PaymentAlipayNotifyURL:  "https://panel.example.com/api/payments/callback/alipay",
		PaymentAlipayReturnURL:  "https://panel.example.com/user/orders",
		PaymentAlipaySandbox:    true,
		PaymentWeChatEnabled:    true,
		PaymentWeChatAppID:      "wechat-app",
		PaymentWeChatMchID:      "wechat-mch",
		PaymentWeChatAPIKey:     "wechat-api-key",
		PaymentWeChatNotifyURL:  "https://panel.example.com/api/payments/callback/wechat",
		PaymentWeChatSandbox:    true,
	}

	err := service.UpdateSystemSettings(ctx, newSettings)
	require.NoError(t, err)

	readSettings, err := service.GetSystemSettings(ctx)
	require.NoError(t, err)

	assert.True(t, readSettings.PaymentAlipayEnabled)
	assert.Equal(t, "alipay-app", readSettings.PaymentAlipayAppID)
	assert.Equal(t, "alipay-private-key", readSettings.PaymentAlipayPrivateKey)
	assert.True(t, readSettings.PaymentAlipayPrivateKeyConfigured)
	assert.Equal(t, "alipay-public-key", readSettings.PaymentAlipayPublicKey)
	assert.Equal(t, "https://panel.example.com/api/payments/callback/alipay", readSettings.PaymentAlipayNotifyURL)
	assert.Equal(t, "https://panel.example.com/user/orders", readSettings.PaymentAlipayReturnURL)
	assert.True(t, readSettings.PaymentAlipaySandbox)
	assert.True(t, readSettings.PaymentWeChatEnabled)
	assert.Equal(t, "wechat-app", readSettings.PaymentWeChatAppID)
	assert.Equal(t, "wechat-mch", readSettings.PaymentWeChatMchID)
	assert.Equal(t, "wechat-api-key", readSettings.PaymentWeChatAPIKey)
	assert.True(t, readSettings.PaymentWeChatAPIKeyConfigured)
	assert.Equal(t, "https://panel.example.com/api/payments/callback/wechat", readSettings.PaymentWeChatNotifyURL)
	assert.True(t, readSettings.PaymentWeChatSandbox)
}

func TestSettingsService_UpdateSystemSettingsWithOptions_SkipsPaymentSettings(t *testing.T) {
	repo := newMockSettingsRepository()
	service := NewService(repo)
	ctx := context.Background()

	err := service.UpdateSystemSettingsWithOptions(ctx, &SystemSettings{
		SiteName:             "Only Base Settings",
		PaymentAlipayEnabled: true,
		PaymentAlipayAppID:   "should-not-persist",
		PaymentWeChatEnabled: true,
		PaymentWeChatAppID:   "should-not-persist",
		PaymentWeChatMchID:   "should-not-persist",
		PaymentWeChatAPIKey:  "should-not-persist",
	}, UpdateOptions{IncludePaymentSettings: false})
	require.NoError(t, err)

	allSettings, err := service.GetAll(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Only Base Settings", allSettings["site_name"])
	assert.NotContains(t, allSettings, "payment_alipay_enabled")
	assert.NotContains(t, allSettings, "payment_alipay_app_id")
	assert.NotContains(t, allSettings, "payment_wechat_enabled")
	assert.NotContains(t, allSettings, "payment_wechat_app_id")
	assert.NotContains(t, allSettings, "payment_wechat_mch_id")
	assert.NotContains(t, allSettings, "payment_wechat_api_key")
}

func TestSettingsService_SecuritySettingsPersistence(t *testing.T) {
	repo := newMockSettingsRepository()
	service := NewService(repo)
	ctx := context.Background()

	newSettings := &SystemSettings{
		SessionTimeout:    180,
		EnableIPWhitelist: true,
		IPWhitelist:       "192.168.1.10\n10.0.0.0/24",
		EnableLoginLock:   true,
		MaxLoginAttempts:  4,
		LockDuration:      15,
	}

	err := service.UpdateSystemSettings(ctx, newSettings)
	require.NoError(t, err)

	readSettings, err := service.GetSystemSettings(ctx)
	require.NoError(t, err)

	assert.Equal(t, 180, readSettings.SessionTimeout)
	assert.True(t, readSettings.EnableIPWhitelist)
	assert.Equal(t, "192.168.1.10\n10.0.0.0/24", readSettings.IPWhitelist)
	assert.True(t, readSettings.EnableLoginLock)
	assert.Equal(t, 4, readSettings.MaxLoginAttempts)
	assert.Equal(t, 15, readSettings.LockDuration)
}

func TestSettingsService_SMTPSettingsPersistence(t *testing.T) {
	repo := newMockSettingsRepository()
	service := NewService(repo)
	ctx := context.Background()

	newSettings := &SystemSettings{
		SMTPHost:       "smtp.example.com",
		SMTPPort:       587,
		SMTPUser:       "mailer@example.com",
		SMTPFrom:       "noreply@example.com",
		SMTPAlertEmail: "ops@example.com",
		SMTPPassword:   "super-secret-password",
	}

	err := service.UpdateSystemSettings(ctx, newSettings)
	require.NoError(t, err)

	readSettings, err := service.GetSystemSettings(ctx)
	require.NoError(t, err)

	assert.Equal(t, "smtp.example.com", readSettings.SMTPHost)
	assert.Equal(t, 587, readSettings.SMTPPort)
	assert.Equal(t, "mailer@example.com", readSettings.SMTPUser)
	assert.Equal(t, "noreply@example.com", readSettings.SMTPFrom)
	assert.Equal(t, "ops@example.com", readSettings.SMTPAlertEmail)
	assert.Equal(t, "super-secret-password", readSettings.SMTPPassword)
	assert.True(t, readSettings.SMTPPasswordConfigured)
}

func TestSettingsService_CacheInvalidation(t *testing.T) {
	repo := newMockSettingsRepository()
	service := NewService(repo)
	ctx := context.Background()

	// Get initial settings (populates cache)
	settings1, err := service.GetSystemSettings(ctx)
	require.NoError(t, err)
	assert.Equal(t, "V Panel", settings1.SiteName)

	// Update directly in repo (simulating external change)
	repo.settings["site_name"] = "Updated Panel"

	// Cache should still return old value
	settings2, err := service.GetSystemSettings(ctx)
	require.NoError(t, err)
	assert.Equal(t, "V Panel", settings2.SiteName)

	// Invalidate cache
	service.InvalidateCache()

	// Now should return new value
	settings3, err := service.GetSystemSettings(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Updated Panel", settings3.SiteName)
}
