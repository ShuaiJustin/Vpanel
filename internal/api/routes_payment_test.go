package api

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"v/internal/commercial/payment"
	"v/internal/config"
	"v/internal/logger"
)

func TestRegisterConfiguredPaymentGatewaysRegistersEnabledGateways(t *testing.T) {
	privateKey, publicKey := testPaymentKeyPair(t)
	log := logger.NewNopLogger()
	router := &Router{
		config: &config.Config{
			Server: config.ServerConfig{BaseURL: "https://panel.example.com"},
			Payment: config.PaymentConfig{
				Alipay: config.PaymentAlipayConfig{
					Enabled:         true,
					AppID:           "app-id",
					PrivateKey:      privateKey,
					AlipayPublicKey: publicKey,
				},
				WeChat: config.PaymentWeChatConfig{
					Enabled: true,
					AppID:   "wx-app-id",
					MchID:   "merchant-id",
					APIKey:  "secret",
				},
			},
		},
		logger: log,
	}

	paymentService := payment.NewService(nil, log)
	router.registerConfiguredPaymentGateways(paymentService)

	methods := paymentService.ListGateways()
	if !containsString(methods, "alipay") {
		t.Fatalf("expected alipay gateway to be registered, got %v", methods)
	}
	if !containsString(methods, "wechat") {
		t.Fatalf("expected wechat gateway to be registered, got %v", methods)
	}
}

func TestRegisterConfiguredPaymentGatewaysSkipsIncompleteConfig(t *testing.T) {
	log := logger.NewNopLogger()
	router := &Router{
		config: &config.Config{
			Server: config.ServerConfig{BaseURL: "https://panel.example.com"},
			Payment: config.PaymentConfig{
				Alipay: config.PaymentAlipayConfig{
					Enabled: true,
					AppID:   "app-id",
				},
			},
		},
		logger: log,
	}

	paymentService := payment.NewService(nil, log)
	router.registerConfiguredPaymentGateways(paymentService)

	methods := paymentService.ListGateways()
	if containsString(methods, "alipay") {
		t.Fatalf("expected incomplete alipay config to be skipped, got %v", methods)
	}
}

func testPaymentKeyPair(t *testing.T) (string, string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	return string(privateKeyPEM), string(publicKeyPEM)
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
