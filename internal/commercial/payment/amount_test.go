package payment

import (
	"crypto/rand"
	"crypto/rsa"
	"net/url"
	"testing"
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
	gateway := &AlipayGateway{privateKey: key, publicKey: &key.PublicKey}
	for input, want := range map[string]int64{"1.15": 115, "2.30": 230, "19.99": 1999, "0.29": 29} {
		params := map[string]string{"out_trade_no": "ORD-signed", "trade_no": "PAY-signed", "trade_status": "TRADE_SUCCESS", "total_amount": input}
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
	}
}
