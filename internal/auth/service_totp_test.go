package auth

import (
	"testing"
	"time"
)

func TestGenerateTOTPCodeMatchesRFCVector(t *testing.T) {
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	code, err := generateTOTPCode(secret, time.Unix(59, 0).UTC())
	if err != nil {
		t.Fatalf("generateTOTPCode returned error: %v", err)
	}

	if code != "287082" {
		t.Fatalf("expected RFC6238-derived code 287082, got %s", code)
	}
}

func TestVerifyTOTPAtTime(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP"
	at := time.Unix(1700000000, 0).UTC()

	code, err := generateTOTPCode(secret, at)
	if err != nil {
		t.Fatalf("generateTOTPCode returned error: %v", err)
	}

	if !verifyTOTPAtTime(secret, code, at) {
		t.Fatalf("expected generated code %s to verify successfully", code)
	}

	if verifyTOTPAtTime(secret, "ABCDEF", at) {
		t.Fatalf("expected non-numeric code to be rejected")
	}

	if verifyTOTPAtTime(secret, code, at.Add(2*time.Minute)) {
		t.Fatalf("expected stale invalid code to be rejected")
	}
}
