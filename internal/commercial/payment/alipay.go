// Package payment provides payment gateway functionality.
package payment

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// AlipayGateway implements PaymentGateway for Alipay.
type AlipayGateway struct {
	config     *AlipayConfig
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	client     *http.Client
}

// AlipayConfig holds Alipay gateway configuration.
type AlipayConfig struct {
	AppID           string `json:"app_id"`
	PrivateKey      string `json:"private_key"`       // RSA private key (PEM format)
	AlipayPublicKey string `json:"alipay_public_key"` // Alipay public key (PEM format)
	NotifyURL       string `json:"notify_url"`
	ReturnURL       string `json:"return_url"`
	IsSandbox       bool   `json:"is_sandbox"`
}

// NewAlipayGateway creates a new Alipay gateway.
func NewAlipayGateway(config *AlipayConfig) (*AlipayGateway, error) {
	if config.AppID == "" {
		return nil, errors.New("app_id is required")
	}

	gateway := &AlipayGateway{
		config: config,
		client: &http.Client{Timeout: 30 * time.Second},
	}

	// Parse private key
	if config.PrivateKey != "" {
		privateKey, err := parsePrivateKey(config.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		gateway.privateKey = privateKey
	}

	// Parse Alipay public key
	if config.AlipayPublicKey != "" {
		publicKey, err := parsePublicKey(config.AlipayPublicKey)
		if err != nil {
			return nil, fmt.Errorf("failed to parse alipay public key: %w", err)
		}
		gateway.publicKey = publicKey
	}

	return gateway, nil
}

// Name returns the gateway name.
func (g *AlipayGateway) Name() string {
	return "alipay"
}

// CreatePayment creates a payment request.
func (g *AlipayGateway) CreatePayment(order *PaymentOrder) (*PaymentRequest, error) {
	if order == nil {
		return nil, errors.New("order is required")
	}

	// Build biz content
	bizContent := map[string]interface{}{
		"out_trade_no": order.OrderNo,
		"total_amount": fmt.Sprintf("%.2f", float64(order.Amount)/100),
		"subject":      order.Subject,
		"product_code": "FAST_INSTANT_TRADE_PAY",
	}

	bizContentJSON, err := json.Marshal(bizContent)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal biz content: %w", err)
	}

	// Build request params
	params := map[string]string{
		"app_id":      g.config.AppID,
		"method":      "alipay.trade.page.pay",
		"format":      "JSON",
		"charset":     "utf-8",
		"sign_type":   "RSA2",
		"timestamp":   time.Now().Format("2006-01-02 15:04:05"),
		"version":     "1.0",
		"notify_url":  g.config.NotifyURL,
		"return_url":  g.config.ReturnURL,
		"biz_content": string(bizContentJSON),
	}

	// Sign request
	sign, err := g.sign(params)
	if err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}
	params["sign"] = sign

	// Build payment URL
	baseURL := "https://openapi.alipay.com/gateway.do"
	if g.config.IsSandbox {
		baseURL = "https://openapi.alipaydev.com/gateway.do"
	}

	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}

	paymentURL := baseURL + "?" + values.Encode()

	return &PaymentRequest{
		PaymentURL: paymentURL,
		ExpireTime: time.Now().Add(15 * time.Minute),
		Extra: map[string]string{
			"method": "alipay.trade.page.pay",
		},
	}, nil
}

// VerifyCallback verifies a payment callback.
func (g *AlipayGateway) VerifyCallback(data []byte, signature string) (*PaymentResult, error) {
	// Parse callback data
	values, err := url.ParseQuery(string(data))
	if err != nil {
		return nil, fmt.Errorf("failed to parse callback data: %w", err)
	}

	// Get signature from callback
	callbackSign := values.Get("sign")
	if callbackSign == "" && signature != "" {
		callbackSign = signature
	}
	if callbackSign == "" {
		return nil, errors.New("missing callback signature")
	}

	// Remove sign and sign_type from params for verification
	params := make(map[string]string)
	for k := range values {
		if k != "sign" && k != "sign_type" {
			params[k] = values.Get(k)
		}
	}

	// Verify signature
	if g.publicKey == nil {
		return nil, errors.New("alipay public key not configured")
	}
	if err := g.verify(params, callbackSign); err != nil {
		return nil, fmt.Errorf("signature verification failed: %w", err)
	}

	// Check trade status
	tradeStatus := values.Get("trade_status")
	if tradeStatus != "TRADE_SUCCESS" && tradeStatus != "TRADE_FINISHED" {
		return &PaymentResult{
			Success: false,
			OrderNo: values.Get("out_trade_no"),
			Error:   fmt.Sprintf("trade status: %s", tradeStatus),
		}, nil
	}

	// Parse amount
	var amount int64
	if amountStr := values.Get("total_amount"); amountStr != "" {
		var amountFloat float64
		fmt.Sscanf(amountStr, "%f", &amountFloat)
		amount = int64(amountFloat * 100)
	}

	// Parse paid time
	paidAt := time.Now()
	if gmtPayment := values.Get("gmt_payment"); gmtPayment != "" {
		if t, err := time.Parse("2006-01-02 15:04:05", gmtPayment); err == nil {
			paidAt = t
		}
	}

	return &PaymentResult{
		Success:   true,
		OrderNo:   values.Get("out_trade_no"),
		PaymentNo: values.Get("trade_no"),
		Amount:    amount,
		PaidAt:    paidAt,
	}, nil
}

// QueryPayment queries the payment status.
func (g *AlipayGateway) QueryPayment(paymentNo string) (*PaymentResult, error) {
	bizContent := map[string]interface{}{
		"trade_no": paymentNo,
	}

	bizContentJSON, err := json.Marshal(bizContent)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal biz content: %w", err)
	}

	params := map[string]string{
		"app_id":      g.config.AppID,
		"method":      "alipay.trade.query",
		"format":      "JSON",
		"charset":     "utf-8",
		"sign_type":   "RSA2",
		"timestamp":   time.Now().Format("2006-01-02 15:04:05"),
		"version":     "1.0",
		"biz_content": string(bizContentJSON),
	}

	sign, err := g.sign(params)
	if err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}
	params["sign"] = sign

	// Make request
	baseURL := "https://openapi.alipay.com/gateway.do"
	if g.config.IsSandbox {
		baseURL = "https://openapi.alipaydev.com/gateway.do"
	}

	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}

	resp, err := g.client.PostForm(baseURL, values)
	if err != nil {
		return nil, fmt.Errorf("failed to query payment: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response
	var result struct {
		AlipayTradeQueryResponse struct {
			Code        string `json:"code"`
			Msg         string `json:"msg"`
			TradeNo     string `json:"trade_no"`
			OutTradeNo  string `json:"out_trade_no"`
			TradeStatus string `json:"trade_status"`
			TotalAmount string `json:"total_amount"`
		} `json:"alipay_trade_query_response"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	queryResp := result.AlipayTradeQueryResponse
	if queryResp.Code != "10000" {
		return &PaymentResult{
			Success: false,
			Error:   queryResp.Msg,
		}, nil
	}

	success := queryResp.TradeStatus == "TRADE_SUCCESS" || queryResp.TradeStatus == "TRADE_FINISHED"

	var amount int64
	if queryResp.TotalAmount != "" {
		var amountFloat float64
		fmt.Sscanf(queryResp.TotalAmount, "%f", &amountFloat)
		amount = int64(amountFloat * 100)
	}

	return &PaymentResult{
		Success:   success,
		OrderNo:   queryResp.OutTradeNo,
		PaymentNo: queryResp.TradeNo,
		Amount:    amount,
		PaidAt:    time.Now(),
	}, nil
}

// Refund processes a refund.
func (g *AlipayGateway) Refund(paymentNo string, amount int64, reason string) (*RefundResult, error) {
	refundNo := fmt.Sprintf("RF%s%d", time.Now().Format("20060102150405"), time.Now().UnixNano()%10000)

	bizContent := map[string]interface{}{
		"trade_no":       paymentNo,
		"refund_amount":  fmt.Sprintf("%.2f", float64(amount)/100),
		"refund_reason":  reason,
		"out_request_no": refundNo,
	}

	bizContentJSON, err := json.Marshal(bizContent)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal biz content: %w", err)
	}

	params := map[string]string{
		"app_id":      g.config.AppID,
		"method":      "alipay.trade.refund",
		"format":      "JSON",
		"charset":     "utf-8",
		"sign_type":   "RSA2",
		"timestamp":   time.Now().Format("2006-01-02 15:04:05"),
		"version":     "1.0",
		"biz_content": string(bizContentJSON),
	}

	sign, err := g.sign(params)
	if err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}
	params["sign"] = sign

	baseURL := "https://openapi.alipay.com/gateway.do"
	if g.config.IsSandbox {
		baseURL = "https://openapi.alipaydev.com/gateway.do"
	}

	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}

	resp, err := g.client.PostForm(baseURL, values)
	if err != nil {
		return nil, fmt.Errorf("failed to process refund: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		AlipayTradeRefundResponse struct {
			Code       string `json:"code"`
			Msg        string `json:"msg"`
			TradeNo    string `json:"trade_no"`
			RefundFee  string `json:"refund_fee"`
			FundChange string `json:"fund_change"`
		} `json:"alipay_trade_refund_response"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	refundResp := result.AlipayTradeRefundResponse
	if refundResp.Code != "10000" {
		return &RefundResult{
			Success: false,
			Error:   refundResp.Msg,
		}, nil
	}

	var refundAmount int64
	if refundResp.RefundFee != "" {
		var amountFloat float64
		fmt.Sscanf(refundResp.RefundFee, "%f", &amountFloat)
		refundAmount = int64(amountFloat * 100)
	}

	return &RefundResult{
		Success:  true,
		RefundNo: refundNo,
		Amount:   refundAmount,
		RefundAt: time.Now(),
	}, nil
}

// sign signs the request parameters.
func (g *AlipayGateway) sign(params map[string]string) (string, error) {
	if g.privateKey == nil {
		return "", errors.New("private key not configured")
	}

	// Sort keys
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build sign string
	var pairs []string
	for _, k := range keys {
		if v := params[k]; v != "" {
			pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
		}
	}
	signStr := strings.Join(pairs, "&")

	// Sign with RSA2 (SHA256)
	hash := sha256.Sum256([]byte(signStr))
	signature, err := rsa.SignPKCS1v15(rand.Reader, g.privateKey, crypto.SHA256, hash[:])
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(signature), nil
}

// verify verifies the callback signature.
func (g *AlipayGateway) verify(params map[string]string, signature string) error {
	if g.publicKey == nil {
		return errors.New("public key not configured")
	}

	// Sort keys
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build sign string
	var pairs []string
	for _, k := range keys {
		if v := params[k]; v != "" {
			pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
		}
	}
	signStr := strings.Join(pairs, "&")

	// Decode signature
	sig, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return err
	}

	// Verify with RSA2 (SHA256)
	hash := sha256.Sum256([]byte(signStr))
	return rsa.VerifyPKCS1v15(g.publicKey, crypto.SHA256, hash[:], sig)
}

// parsePrivateKey parses a PEM-encoded RSA private key.
func parsePrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS1
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
	}

	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("not an RSA private key")
	}

	return rsaKey, nil
}

// parsePublicKey parses a PEM-encoded RSA public key.
func parsePublicKey(pemStr string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	rsaKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("not an RSA public key")
	}

	return rsaKey, nil
}
