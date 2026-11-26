package payment

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/url"
	"testing"
	"time"
)

func TestParseCentsExact(t *testing.T) {
	for input, want := range map[string]int64{"1.15": 115, "2.30": 230, "19.99": 1999, "0.29": 29, "9": 900, "9.9": 990, "0.00": 0, "92233720368547758.07": 9223372036854775807} {
		got, err := parseCents(input)
		if err != nil || got != want {
			t.Errorf("parseCents(%q) = %d, %v; want %d", input, got, err, want)
		}
		if got, err := parseCents(formatCents(want)); err != nil || got != want {
			t.Errorf("round trip %d = %d, %v", want, got, err)
		}
	}
	for _, input := range []string{"", "-1", "+1", " 1.15", "1.", ".29", "1.234", "1e2", "NaN", "92233720368547758.08"} {
		if _, err := parseCents(input); err == nil {
			t.Errorf("accepted invalid amount %q", input)
		}
	}
}

func TestAlipaySignedCallbackExactCents(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	gateway := &AlipayGateway{config: &AlipayConfig{}, privateKey: key, publicKey: &key.PublicKey}
	deadline := time.Date(2026, 11, 11, 2, 30, 0, 0, time.UTC)
	request, err := gateway.CreatePayment(&PaymentOrder{OrderNo: "ORD-signed", Amount: 115, Subject: "Plan", ExpiresAt: deadline})
	if err != nil {
		t.Fatal(err)
	}
	checkoutURL, err := url.Parse(request.PaymentURL)
	if err != nil {
		t.Fatal(err)
	}
	var biz map[string]any
	if err := json.Unmarshal([]byte(checkoutURL.Query().Get("biz_content")), &biz); err != nil {
		t.Fatal(err)
	}
	if !request.ExpireTime.Equal(deadline) || biz["time_expire"] != "2026-11-11 10:30:00" {
		t.Fatalf("checkout uses a different cutoff: %s, %v", request.ExpireTime, biz["time_expire"])
	}
	for input, want := range map[string]int64{"1.15": 115, "2.30": 230, "19.99": 1999, "0.29": 29} {
		params := map[string]string{"out_trade_no": "ORD-signed", "trade_no": "PAY-signed", "trade_status": "TRADE_SUCCESS", "total_amount": input, "gmt_payment": "2026-11-11 10:15:00"}
		sign, err := gateway.sign(params)
		if err != nil {
			t.Fatal(err)
		}
		values := url.Values{}
		for k, v := range params {
			values.Set(k, v)
		}
		values.Set("sign", sign)
		result, err := gateway.VerifyCallback([]byte(values.Encode()), "")
		if err != nil || !result.Success || result.Amount != want {
			t.Fatalf("signed callback %s: %+v %v", input, result, err)
		}
		if !result.PaidAt.Equal(time.Date(2026, 11, 11, 2, 15, 0, 0, time.UTC)) {
			t.Fatalf("wrong payment instant: %s", result.PaidAt)
		}
	}
}

func TestWeChatSignedCallbackTimeZone(t *testing.T) {
	gateway, err := NewWeChatGateway(&WeChatConfig{AppID: "wx-app", MchID: "mch-id", APIKey: "test"})
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{
		"appid": "wx-app", "mch_id": "mch-id", "out_trade_no": "ORD-signed", "transaction_id": "PAY-signed",
		"return_code": "SUCCESS", "result_code": "SUCCESS", "total_fee": "100", "time_end": "20261111101500",
	}
	values["sign"] = gateway.sign(values)
	result, err := gateway.VerifyCallback(gateway.buildXML(values), "")
	if err != nil {
		t.Fatal(err)
	}
	if !result.PaidAt.Equal(time.Date(2026, 11, 11, 2, 15, 0, 0, time.UTC)) {
		t.Fatalf("wrong payment instant: %s", result.PaidAt)
	}
}
