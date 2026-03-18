// Package payment provides payment gateway functionality.
package payment

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// WeChatGateway implements PaymentGateway for WeChat Pay.
type WeChatGateway struct {
	config *WeChatConfig
	client *http.Client
}

// WeChatConfig holds WeChat Pay gateway configuration.
type WeChatConfig struct {
	AppID     string `json:"app_id"`
	MchID     string `json:"mch_id"`  // Merchant ID
	APIKey    string `json:"api_key"` // API Key for signing
	NotifyURL string `json:"notify_url"`
	IsSandbox bool   `json:"is_sandbox"`
}

// NewWeChatGateway creates a new WeChat Pay gateway.
func NewWeChatGateway(config *WeChatConfig) (*WeChatGateway, error) {
	if config.AppID == "" {
		return nil, errors.New("app_id is required")
	}
	if config.MchID == "" {
		return nil, errors.New("mch_id is required")
	}
	if config.APIKey == "" {
		return nil, errors.New("api_key is required")
	}

	return &WeChatGateway{
		config: config,
		client: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Name returns the gateway name.
func (g *WeChatGateway) Name() string {
	return "wechat"
}

// CreatePayment creates a payment request.
func (g *WeChatGateway) CreatePayment(order *PaymentOrder) (*PaymentRequest, error) {
	if order == nil {
		return nil, errors.New("order is required")
	}

	nonceStr := generateNonceStr()

	params := map[string]string{
		"appid":            g.config.AppID,
		"mch_id":           g.config.MchID,
		"nonce_str":        nonceStr,
		"body":             order.Subject,
		"out_trade_no":     order.OrderNo,
		"total_fee":        fmt.Sprintf("%d", order.Amount),
		"spbill_create_ip": order.ClientIP,
		"notify_url":       g.config.NotifyURL,
		"trade_type":       "NATIVE", // QR code payment
	}

	// Sign request
	sign := g.sign(params)
	params["sign"] = sign

	// Build XML request
	xmlData := g.buildXML(params)

	// Make request
	baseURL := "https://api.mch.weixin.qq.com/pay/unifiedorder"
	if g.config.IsSandbox {
		baseURL = "https://api.mch.weixin.qq.com/sandboxnew/pay/unifiedorder"
	}

	resp, err := g.client.Post(baseURL, "application/xml", bytes.NewReader(xmlData))
	if err != nil {
		return nil, fmt.Errorf("failed to create payment: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response
	result, err := g.parseXML(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result["return_code"] != "SUCCESS" {
		return nil, fmt.Errorf("payment failed: %s", result["return_msg"])
	}

	if result["result_code"] != "SUCCESS" {
		return nil, fmt.Errorf("payment failed: %s", result["err_code_des"])
	}

	codeURL := result["code_url"]
	if codeURL == "" {
		return nil, errors.New("no code_url in response")
	}

	return &PaymentRequest{
		QRCodeData: codeURL,
		ExpireTime: time.Now().Add(2 * time.Hour),
		Extra: map[string]string{
			"prepay_id": result["prepay_id"],
		},
	}, nil
}

// VerifyCallback verifies a payment callback.
func (g *WeChatGateway) VerifyCallback(data []byte, signature string) (*PaymentResult, error) {
	// Parse XML callback
	result, err := g.parseXML(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse callback: %w", err)
	}

	// Verify signature
	callbackSign := strings.TrimSpace(result["sign"])
	if callbackSign == "" {
		return nil, errors.New("missing callback signature")
	}
	delete(result, "sign")

	expectedSign := g.sign(result)
	if callbackSign != expectedSign {
		return nil, errors.New("signature verification failed")
	}

	// Check result
	if result["return_code"] != "SUCCESS" || result["result_code"] != "SUCCESS" {
		return &PaymentResult{
			Success: false,
			OrderNo: result["out_trade_no"],
			Error:   result["err_code_des"],
		}, nil
	}

	// Parse amount
	var amount int64
	if feeStr := result["total_fee"]; feeStr != "" {
		fmt.Sscanf(feeStr, "%d", &amount)
	}

	// Parse paid time
	paidAt := time.Now()
	if timeEnd := result["time_end"]; timeEnd != "" {
		if t, err := time.Parse("20060102150405", timeEnd); err == nil {
			paidAt = t
		}
	}

	return &PaymentResult{
		Success:   true,
		OrderNo:   result["out_trade_no"],
		PaymentNo: result["transaction_id"],
		Amount:    amount,
		PaidAt:    paidAt,
	}, nil
}

// QueryPayment queries the payment status.
func (g *WeChatGateway) QueryPayment(paymentNo string) (*PaymentResult, error) {
	nonceStr := generateNonceStr()

	params := map[string]string{
		"appid":          g.config.AppID,
		"mch_id":         g.config.MchID,
		"transaction_id": paymentNo,
		"nonce_str":      nonceStr,
	}

	sign := g.sign(params)
	params["sign"] = sign

	xmlData := g.buildXML(params)

	baseURL := "https://api.mch.weixin.qq.com/pay/orderquery"
	if g.config.IsSandbox {
		baseURL = "https://api.mch.weixin.qq.com/sandboxnew/pay/orderquery"
	}

	resp, err := g.client.Post(baseURL, "application/xml", bytes.NewReader(xmlData))
	if err != nil {
		return nil, fmt.Errorf("failed to query payment: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	result, err := g.parseXML(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result["return_code"] != "SUCCESS" {
		return &PaymentResult{
			Success: false,
			Error:   result["return_msg"],
		}, nil
	}

	tradeState := result["trade_state"]
	success := tradeState == "SUCCESS"

	var amount int64
	if feeStr := result["total_fee"]; feeStr != "" {
		fmt.Sscanf(feeStr, "%d", &amount)
	}

	return &PaymentResult{
		Success:   success,
		OrderNo:   result["out_trade_no"],
		PaymentNo: result["transaction_id"],
		Amount:    amount,
		PaidAt:    time.Now(),
	}, nil
}

// Refund processes a refund.
func (g *WeChatGateway) Refund(paymentNo string, amount int64, reason string) (*RefundResult, error) {
	nonceStr := generateNonceStr()
	refundNo := fmt.Sprintf("RF%s%d", time.Now().Format("20060102150405"), time.Now().UnixNano()%10000)

	params := map[string]string{
		"appid":          g.config.AppID,
		"mch_id":         g.config.MchID,
		"nonce_str":      nonceStr,
		"transaction_id": paymentNo,
		"out_refund_no":  refundNo,
		"total_fee":      fmt.Sprintf("%d", amount), // Should be original order amount
		"refund_fee":     fmt.Sprintf("%d", amount),
		"refund_desc":    reason,
	}

	sign := g.sign(params)
	params["sign"] = sign

	xmlData := g.buildXML(params)

	baseURL := "https://api.mch.weixin.qq.com/secapi/pay/refund"
	if g.config.IsSandbox {
		baseURL = "https://api.mch.weixin.qq.com/sandboxnew/secapi/pay/refund"
	}

	// Note: Refund API requires client certificate, simplified here
	resp, err := g.client.Post(baseURL, "application/xml", bytes.NewReader(xmlData))
	if err != nil {
		return nil, fmt.Errorf("failed to process refund: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	result, err := g.parseXML(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result["return_code"] != "SUCCESS" || result["result_code"] != "SUCCESS" {
		errMsg := result["return_msg"]
		if result["err_code_des"] != "" {
			errMsg = result["err_code_des"]
		}
		return &RefundResult{
			Success: false,
			Error:   errMsg,
		}, nil
	}

	var refundAmount int64
	if feeStr := result["refund_fee"]; feeStr != "" {
		fmt.Sscanf(feeStr, "%d", &refundAmount)
	}

	return &RefundResult{
		Success:  true,
		RefundNo: result["refund_id"],
		Amount:   refundAmount,
		RefundAt: time.Now(),
	}, nil
}

// sign signs the request parameters using MD5.
func (g *WeChatGateway) sign(params map[string]string) string {
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
	signStr := strings.Join(pairs, "&") + "&key=" + g.config.APIKey

	// MD5 hash
	hash := md5.Sum([]byte(signStr))
	return strings.ToUpper(hex.EncodeToString(hash[:]))
}

// signHMAC signs using HMAC-SHA256.
func (g *WeChatGateway) signHMAC(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var pairs []string
	for _, k := range keys {
		if v := params[k]; v != "" {
			pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
		}
	}
	signStr := strings.Join(pairs, "&") + "&key=" + g.config.APIKey

	h := hmac.New(sha256.New, []byte(g.config.APIKey))
	h.Write([]byte(signStr))
	return strings.ToUpper(hex.EncodeToString(h.Sum(nil)))
}

// buildXML builds XML from params.
func (g *WeChatGateway) buildXML(params map[string]string) []byte {
	var buf bytes.Buffer
	buf.WriteString("<xml>")
	for k, v := range params {
		buf.WriteString(fmt.Sprintf("<%s><![CDATA[%s]]></%s>", k, v, k))
	}
	buf.WriteString("</xml>")
	return buf.Bytes()
}

// parseXML parses XML response to map.
func (g *WeChatGateway) parseXML(data []byte) (map[string]string, error) {
	result := make(map[string]string)

	decoder := xml.NewDecoder(bytes.NewReader(data))
	var currentKey string

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local != "xml" {
				currentKey = t.Name.Local
			}
		case xml.CharData:
			if currentKey != "" {
				result[currentKey] = strings.TrimSpace(string(t))
			}
		case xml.EndElement:
			currentKey = ""
		}
	}

	return result, nil
}

// generateNonceStr generates a random nonce string.
func generateNonceStr() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
