package notification

import (
	"strings"
	"testing"
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
