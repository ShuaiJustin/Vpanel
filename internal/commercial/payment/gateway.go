// Package payment provides payment gateway functionality.
package payment

import (
	"time"
)

var gatewayChinaTime = time.FixedZone("CST", 8*60*60)

// PaymentGateway defines the interface for payment gateways.
type PaymentGateway interface {
	// Name returns the gateway name.
	Name() string

	// CreatePayment creates a payment request.
	CreatePayment(order *PaymentOrder) (*PaymentRequest, error)

	// VerifyCallback verifies a payment callback.
	VerifyCallback(data []byte, signature string) (*PaymentResult, error)

	// QueryPayment queries the payment status.
	QueryPayment(paymentNo string) (*PaymentResult, error)

	// Refund processes a refund.
	Refund(paymentNo string, amount int64, reason string) (*RefundResult, error)
}

// PaymentOrder represents an order for payment.
type PaymentOrder struct {
	OrderNo     string    `json:"order_no"`
	Amount      int64     `json:"amount"`      // cents
	Subject     string    `json:"subject"`     // order subject/title
	Description string    `json:"description"` // order description
	ClientIP    string    `json:"client_ip"`   // client IP address
	NotifyURL   string    `json:"notify_url"`  // callback URL
	ReturnURL   string    `json:"return_url"`  // return URL after payment
	ExpiresAt   time.Time `json:"expires_at"`
}

// PaymentRequest represents a payment request response.
type PaymentRequest struct {
	PaymentURL string            `json:"payment_url"` // redirect URL
	QRCodeURL  string            `json:"qrcode_url"`  // QR code image URL
	QRCodeData string            `json:"qrcode_data"` // QR code raw data
	ExpireTime time.Time         `json:"expire_time"` // payment expiration time
	Extra      map[string]string `json:"extra"`       // extra data
}

// PaymentResult represents a payment result.
type PaymentResult struct {
	Success   bool      `json:"success"`
	OrderNo   string    `json:"order_no"`
	PaymentNo string    `json:"payment_no"` // external payment ID
	Amount    int64     `json:"amount"`     // cents
	PaidAt    time.Time `json:"paid_at"`
	Error     string    `json:"error"`
}

// RefundResult represents a refund result.
type RefundResult struct {
	Success  bool      `json:"success"`
	RefundNo string    `json:"refund_no"` // refund transaction ID
	Amount   int64     `json:"amount"`    // refunded amount in cents
	RefundAt time.Time `json:"refund_at"`
	Error    string    `json:"error"`
}

// GatewayConfig holds common gateway configuration.
type GatewayConfig struct {
	AppID      string `json:"app_id"`
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key"`
	NotifyURL  string `json:"notify_url"`
	ReturnURL  string `json:"return_url"`
	IsSandbox  bool   `json:"is_sandbox"`
}
