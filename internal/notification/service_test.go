package notification

import (
	"strings"
	"testing"
	"time"
)

func TestBuildSMTPMessage_UsesRFCCompliantHeaders(t *testing.T) {
	config := &NotificationConfig{
		SMTPHost: "smtp.exmail.qq.com",
		SMTPFrom: "system@shcrystal.com",
		SiteName: "V Panel",
	}

	message, fromAddress, toAddress, err := buildSMTPMessage(
		config,
		"user@example.com",
		"请验证您的邮箱",
		"欢迎注册 V Panel。",
	)
	if err != nil {
		t.Fatalf("buildSMTPMessage returned error: %v", err)
	}

	raw := string(message)

	if fromAddress != "system@shcrystal.com" {
		t.Fatalf("unexpected from address: %s", fromAddress)
	}
	if toAddress != "user@example.com" {
		t.Fatalf("unexpected recipient address: %s", toAddress)
	}
	if !strings.Contains(raw, "From: \"V Panel\" <system@shcrystal.com>\r\n") {
		t.Fatalf("missing formatted From header: %q", raw)
	}
	if !strings.Contains(raw, "To: <user@example.com>\r\n") {
		t.Fatalf("missing To header: %q", raw)
	}
	if !strings.Contains(raw, "Subject: =?UTF-8?") {
		t.Fatalf("subject is not MIME encoded: %q", raw)
	}
	if !strings.Contains(raw, "Content-Transfer-Encoding: quoted-printable\r\n") {
		t.Fatalf("missing transfer encoding header: %q", raw)
	}
	if !strings.Contains(raw, "Message-ID: <") || !strings.Contains(raw, "@shcrystal.com>") {
		t.Fatalf("missing message id header: %q", raw)
	}
}

func TestBuildCertificateAlertContentIncludesFailureAndNode(t *testing.T) {
	subject, message := buildCertificateAlertContent(CertificateAlertData{
		CertificateID: 5,
		Domain:        "*.example.com",
		Level:         "renewal_failed",
		Reason:        "ACME 签发状态缺失",
		NodeID:        19,
		NodeName:      "edge-hk",
		Timestamp:     time.Date(2026, 8, 24, 10, 30, 0, 0, time.Local),
	})

	if !strings.Contains(subject, "证书续期失败") || !strings.Contains(subject, "*.example.com") {
		t.Fatalf("unexpected certificate alert subject: %q", subject)
	}
	for _, expected := range []string{"证书ID: 5", "edge-hk (#19)", "ACME 签发状态缺失"} {
		if !strings.Contains(message, expected) {
			t.Fatalf("certificate alert message missing %q: %q", expected, message)
		}
	}
}
