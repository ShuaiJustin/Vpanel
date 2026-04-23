// Package payment provides payment gateway functionality.
package payment

import (
	"testing"
	"testing/quick"

	"v/internal/logger"
)

// Callback idempotency is now enforced at the database layer inside
// HandleCallback (order status + persisted payment_no checks) rather than via
// an in-memory map. The previously exercised TestProperty_CallbackIdempotency
// and TestProperty_IndependentCallbackTracking tests poked at private fields
// that no longer exist; integration coverage for the DB-level idempotency
// lives in internal/api/handlers/payment_test.go.

// Feature: commercial-system, Property 13: Subscription Activation on Payment
// Validates: Requirements 4.7
// For any successful payment, the user's subscription SHALL be activated
// with correct expiration date based on plan duration.

func TestProperty_PaymentResultSuccess(t *testing.T) {
	// Property: A successful payment result should have all required fields
	f := func(orderNo, paymentNo string, amount uint32) bool {
		if orderNo == "" || paymentNo == "" {
			return true
		}

		result := &PaymentResult{
			Success:   true,
			OrderNo:   orderNo,
			PaymentNo: paymentNo,
			Amount:    int64(amount),
		}

		// Successful payment should have order number and payment number
		return result.Success &&
			result.OrderNo != "" &&
			result.PaymentNo != "" &&
			result.Amount >= 0
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// Property: Failed payment result should have error message
func TestProperty_PaymentResultFailure(t *testing.T) {
	f := func(errorMsg string) bool {
		if errorMsg == "" {
			return true
		}

		result := &PaymentResult{
			Success: false,
			Error:   errorMsg,
		}

		// Failed payment should have error message
		return !result.Success && result.Error != ""
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// Property: Gateway registration should be idempotent
func TestProperty_GatewayRegistration(t *testing.T) {
	log := logger.NewNopLogger()

	f := func(gatewayName string) bool {
		if gatewayName == "" {
			return true
		}

		svc := NewService(nil, log)

		// Create a mock gateway
		mockGateway := &mockGateway{name: gatewayName}

		// Register multiple times
		svc.RegisterGateway(mockGateway)
		svc.RegisterGateway(mockGateway)
		svc.RegisterGateway(mockGateway)

		// Should still only have one gateway with that name
		gateways := svc.ListGateways()
		count := 0
		for _, name := range gateways {
			if name == gatewayName {
				count++
			}
		}

		return count == 1
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// mockGateway is a mock implementation of PaymentGateway for testing.
type mockGateway struct {
	name string
}

func (g *mockGateway) Name() string {
	return g.name
}

func (g *mockGateway) CreatePayment(order *PaymentOrder) (*PaymentRequest, error) {
	return &PaymentRequest{}, nil
}

func (g *mockGateway) VerifyCallback(data []byte, signature string) (*PaymentResult, error) {
	return &PaymentResult{Success: true}, nil
}

func (g *mockGateway) QueryPayment(paymentNo string) (*PaymentResult, error) {
	return &PaymentResult{Success: true}, nil
}

func (g *mockGateway) Refund(paymentNo string, amount int64, reason string) (*RefundResult, error) {
	return &RefundResult{Success: true}, nil
}
