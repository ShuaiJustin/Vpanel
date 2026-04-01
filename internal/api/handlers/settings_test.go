package handlers

import "testing"

func TestShouldPersistPaymentSettings(t *testing.T) {
	t.Run("skip when request and store both lack payment fields", func(t *testing.T) {
		if shouldPersistPaymentSettings(&UpdateSettingsRequest{}, map[string]string{}) {
			t.Fatal("expected payment settings to be skipped")
		}
	})

	t.Run("persist when request includes payment field", func(t *testing.T) {
		enabled := true
		if !shouldPersistPaymentSettings(&UpdateSettingsRequest{PaymentAlipayEnabled: &enabled}, map[string]string{}) {
			t.Fatal("expected payment settings to be persisted when request touches them")
		}
	})

	t.Run("persist when store already contains payment field", func(t *testing.T) {
		if !shouldPersistPaymentSettings(&UpdateSettingsRequest{}, map[string]string{"payment_wechat_enabled": "true"}) {
			t.Fatal("expected existing persisted payment settings to be kept")
		}
	})
}
