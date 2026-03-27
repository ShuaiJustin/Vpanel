package notification

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"strings"
	"sync"
	"time"
)

// NotificationType represents the type of notification
type NotificationType string

const (
	NotificationNewDevice        NotificationType = "new_device"
	NotificationIPLimitReached   NotificationType = "ip_limit_reached"
	NotificationSuspiciousIP     NotificationType = "suspicious_ip"
	NotificationDeviceKicked     NotificationType = "device_kicked"
	NotificationAutoBlacklisted  NotificationType = "auto_blacklisted"
	NotificationNodeStatusChange NotificationType = "node_status_change"
	NotificationNodeTrafficAlert NotificationType = "node_traffic_alert"
)

// NotificationChannel represents the notification channel
type NotificationChannel string

const (
	ChannelEmail    NotificationChannel = "email"
	ChannelTelegram NotificationChannel = "telegram"
)

// NotificationConfig holds the notification configuration
type NotificationConfig struct {
	// Email settings
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string
	AdminEmail   string
	SiteName     string

	// Telegram settings
	TelegramBotToken string
	TelegramChatID   string

	// Notification preferences
	EnabledTypes    map[NotificationType]bool
	EnabledChannels map[NotificationChannel]bool
}

// IPNotificationData contains data for IP-related notifications
type IPNotificationData struct {
	UserID       uint
	Username     string
	Email        string
	IP           string
	Country      string
	City         string
	DeviceInfo   string
	Reason       string
	CurrentCount int
	MaxCount     int
	Timestamp    time.Time
}

// NodeStatusChangeData contains data for node status change notifications
type NodeStatusChangeData struct {
	NodeID    int64
	NodeName  string
	OldStatus string
	NewStatus string
	Reason    string
	Timestamp time.Time
}

// NodeTrafficAlertData contains data for node traffic alert notifications.
type NodeTrafficAlertData struct {
	NodeID           int64
	NodeName         string
	Level            string
	TrafficTotal     int64
	TrafficLimit     int64
	UsagePercent     float64
	ThresholdPercent float64
	Timestamp        time.Time
}

// Service handles sending notifications
type Service struct {
	config *NotificationConfig
	mu     sync.RWMutex
	client *http.Client
}

// NewService creates a new notification service
func NewService(config *NotificationConfig) *Service {
	return &Service{
		config: config,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// UpdateConfig updates the notification configuration
func (s *Service) UpdateConfig(config *NotificationConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = config
}

// CanSendEmail returns whether SMTP is configured well enough to send email.
func (s *Service) CanSendEmail() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.config != nil &&
		strings.TrimSpace(s.config.SMTPHost) != "" &&
		s.config.SMTPPort > 0 &&
		strings.TrimSpace(s.config.SMTPUser) != "" &&
		strings.TrimSpace(s.config.SMTPPassword) != ""
}

// SendEmail sends a plain text email using the current SMTP configuration.
func (s *Service) SendEmail(to, subject, body string) error {
	if strings.TrimSpace(to) == "" {
		return fmt.Errorf("recipient email is required")
	}
	if !s.CanSendEmail() {
		return fmt.Errorf("SMTP not configured")
	}
	return s.sendEmail(to, subject, body)
}

// NotifyNewDevice sends notification when a new device connects
func (s *Service) NotifyNewDevice(data IPNotificationData) error {
	if !s.isEnabled(NotificationNewDevice) {
		return nil
	}

	subject := "新设备连接通知"
	message := fmt.Sprintf(
		"用户 %s 有新设备连接\n\nIP: %s\n位置: %s %s\n设备: %s\n时间: %s",
		data.Username,
		data.IP,
		data.Country,
		data.City,
		data.DeviceInfo,
		data.Timestamp.Format("2006-01-02 15:04:05"),
	)

	return s.send(data.Email, subject, message)
}

// NotifyIPLimitReached sends notification when IP limit is reached
func (s *Service) NotifyIPLimitReached(data IPNotificationData) error {
	if !s.isEnabled(NotificationIPLimitReached) {
		return nil
	}

	subject := "设备数量已达上限"
	message := fmt.Sprintf(
		"用户 %s 的设备数量已达上限\n\n当前设备数: %d\n最大设备数: %d\n\n新连接 IP: %s\n位置: %s %s\n\n请断开其他设备后重试",
		data.Username,
		data.CurrentCount,
		data.MaxCount,
		data.IP,
		data.Country,
		data.City,
	)

	return s.send(data.Email, subject, message)
}

// NotifySuspiciousActivity sends notification for suspicious IP activity
func (s *Service) NotifySuspiciousActivity(data IPNotificationData) error {
	if !s.isEnabled(NotificationSuspiciousIP) {
		return nil
	}

	subject := "⚠️ 可疑活动告警"
	message := fmt.Sprintf(
		"检测到可疑活动\n\n用户: %s\nIP: %s\n位置: %s %s\n原因: %s\n时间: %s",
		data.Username,
		data.IP,
		data.Country,
		data.City,
		data.Reason,
		data.Timestamp.Format("2006-01-02 15:04:05"),
	)

	// Suspicious activity notifications go to admin
	return s.sendToAdmin(subject, message)
}

// NotifyDeviceKicked sends notification when a device is kicked
func (s *Service) NotifyDeviceKicked(data IPNotificationData) error {
	if !s.isEnabled(NotificationDeviceKicked) {
		return nil
	}

	subject := "设备已被踢出"
	message := fmt.Sprintf(
		"您的设备已被踢出\n\nIP: %s\n位置: %s %s\n原因: %s\n时间: %s\n\n如非本人操作，请检查账号安全",
		data.IP,
		data.Country,
		data.City,
		data.Reason,
		data.Timestamp.Format("2006-01-02 15:04:05"),
	)

	return s.send(data.Email, subject, message)
}

// NotifyAutoBlacklisted sends notification when an IP is auto-blacklisted
func (s *Service) NotifyAutoBlacklisted(data IPNotificationData) error {
	if !s.isEnabled(NotificationAutoBlacklisted) {
		return nil
	}

	subject := "IP 已被自动封禁"
	message := fmt.Sprintf(
		"IP 已被自动加入黑名单\n\nIP: %s\n位置: %s %s\n原因: %s\n时间: %s",
		data.IP,
		data.Country,
		data.City,
		data.Reason,
		data.Timestamp.Format("2006-01-02 15:04:05"),
	)

	return s.sendToAdmin(subject, message)
}

// NotifyNodeStatusChange sends notification when a node status changes
func (s *Service) NotifyNodeStatusChange(data NodeStatusChangeData) error {
	if !s.isEnabled(NotificationNodeStatusChange) {
		return nil
	}

	var emoji string
	var statusText string
	switch data.NewStatus {
	case "online":
		emoji = "✅"
		statusText = "恢复正常"
	case "unhealthy":
		emoji = "⚠️"
		statusText = "不健康"
	case "offline":
		emoji = "❌"
		statusText = "离线"
	default:
		emoji = "ℹ️"
		statusText = data.NewStatus
	}

	subject := fmt.Sprintf("%s 节点状态变更: %s", emoji, data.NodeName)
	message := fmt.Sprintf(
		"节点状态已变更\n\n节点ID: %d\n节点名称: %s\n原状态: %s\n新状态: %s\n原因: %s\n时间: %s",
		data.NodeID,
		data.NodeName,
		data.OldStatus,
		statusText,
		data.Reason,
		data.Timestamp.Format("2006-01-02 15:04:05"),
	)

	return s.sendToAdmin(subject, message)
}

// NotifyNodeTrafficAlert sends notification when node traffic reaches a configured threshold.
func (s *Service) NotifyNodeTrafficAlert(data NodeTrafficAlertData) error {
	if !s.isEnabled(NotificationNodeTrafficAlert) {
		return nil
	}

	var emoji string
	var levelText string
	switch data.Level {
	case "limit":
		emoji = "🚫"
		levelText = "达到硬流量上限"
	case "threshold":
		emoji = "⚠️"
		levelText = "达到流量告警阈值"
	default:
		emoji = "ℹ️"
		levelText = data.Level
	}

	subject := fmt.Sprintf("%s 节点流量告警: %s", emoji, data.NodeName)
	message := fmt.Sprintf(
		"节点流量告警\n\n节点ID: %d\n节点名称: %s\n告警级别: %s\n当前使用率: %.2f%%\n告警阈值: %.2f%%\n累计流量: %d\n流量上限: %d\n时间: %s",
		data.NodeID,
		data.NodeName,
		levelText,
		data.UsagePercent,
		data.ThresholdPercent,
		data.TrafficTotal,
		data.TrafficLimit,
		data.Timestamp.Format("2006-01-02 15:04:05"),
	)

	return s.sendToAdmin(subject, message)
}

// isEnabled checks if a notification type is enabled
func (s *Service) isEnabled(notifType NotificationType) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.config == nil || s.config.EnabledTypes == nil {
		return false
	}
	return s.config.EnabledTypes[notifType]
}

// send sends notification through all enabled channels
func (s *Service) send(email, subject, message string) error {
	s.mu.RLock()
	config := s.config
	s.mu.RUnlock()

	if config == nil {
		return nil
	}

	var errs []string

	// Send email if enabled
	if config.EnabledChannels[ChannelEmail] && email != "" {
		if err := s.sendEmail(email, subject, message); err != nil {
			errs = append(errs, fmt.Sprintf("email: %v", err))
		}
	}

	// Send Telegram if enabled
	if config.EnabledChannels[ChannelTelegram] {
		if err := s.sendTelegram(subject + "\n\n" + message); err != nil {
			errs = append(errs, fmt.Sprintf("telegram: %v", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("notification errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// sendToAdmin sends notification to admin only
func (s *Service) sendToAdmin(subject, message string) error {
	s.mu.RLock()
	config := s.config
	s.mu.RUnlock()

	if config == nil {
		return nil
	}

	var errs []string

	adminEmail := strings.TrimSpace(config.AdminEmail)
	if adminEmail == "" {
		adminEmail = strings.TrimSpace(config.SMTPUser)
	}

	if config.EnabledChannels[ChannelEmail] && adminEmail != "" {
		if err := s.sendEmail(adminEmail, subject, message); err != nil {
			errs = append(errs, fmt.Sprintf("email: %v", err))
		}
	}

	if config.EnabledChannels[ChannelTelegram] {
		if err := s.sendTelegram("🔔 " + subject + "\n\n" + message); err != nil {
			errs = append(errs, fmt.Sprintf("telegram: %v", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("notification errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// sendEmail sends an email notification
func (s *Service) sendEmail(to, subject, body string) error {
	s.mu.RLock()
	config := s.config
	s.mu.RUnlock()

	if config.SMTPHost == "" || config.SMTPUser == "" {
		return fmt.Errorf("SMTP not configured")
	}

	msg, envelopeFrom, rcptTo, err := buildSMTPMessage(config, to, subject, body)
	if err != nil {
		return err
	}

	client, closeFn, err := newSMTPClient(config)
	if err != nil {
		return err
	}
	defer closeFn()

	if config.SMTPUser != "" && config.SMTPPassword != "" {
		auth := smtp.PlainAuth("", config.SMTPUser, config.SMTPPassword, config.SMTPHost)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth failed: %w", err)
		}
	}

	if err := client.Mail(envelopeFrom); err != nil {
		return fmt.Errorf("SMTP MAIL FROM failed: %w", err)
	}
	if err := client.Rcpt(rcptTo); err != nil {
		return fmt.Errorf("SMTP RCPT TO failed: %w", err)
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA failed: %w", err)
	}
	if _, err := writer.Write(msg); err != nil {
		_ = writer.Close()
		return fmt.Errorf("SMTP message write failed: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("SMTP message close failed: %w", err)
	}

	if err := client.Quit(); err != nil {
		return fmt.Errorf("SMTP QUIT failed: %w", err)
	}

	return nil
}

func buildSMTPMessage(config *NotificationConfig, to, subject, body string) ([]byte, string, string, error) {
	if config == nil {
		return nil, "", "", fmt.Errorf("notification config is nil")
	}

	from := strings.TrimSpace(config.SMTPFrom)
	if from == "" {
		from = strings.TrimSpace(config.SMTPUser)
	}
	if from == "" {
		return nil, "", "", fmt.Errorf("SMTP sender address is empty")
	}

	fromAddress, err := normalizeEmailAddress(from)
	if err != nil {
		return nil, "", "", fmt.Errorf("invalid sender address: %w", err)
	}

	toAddress, err := normalizeEmailAddress(to)
	if err != nil {
		return nil, "", "", fmt.Errorf("invalid recipient address: %w", err)
	}

	messageIDDomain := emailDomain(fromAddress)
	if messageIDDomain == "" {
		messageIDDomain = strings.Trim(strings.TrimSpace(config.SMTPHost), "[]")
	}
	if messageIDDomain == "" {
		messageIDDomain = "localhost"
	}

	fromHeader := (&mail.Address{
		Name:    strings.TrimSpace(config.SiteName),
		Address: fromAddress,
	}).String()
	if strings.TrimSpace(config.SiteName) == "" {
		fromHeader = (&mail.Address{Address: fromAddress}).String()
	}

	headers := []string{
		"From: " + fromHeader,
		"To: " + (&mail.Address{Address: toAddress}).String(),
		"Subject: " + mime.QEncoding.Encode("UTF-8", subject),
		"Date: " + time.Now().UTC().Format(time.RFC1123Z),
		fmt.Sprintf("Message-ID: <%d@%s>", time.Now().UTC().UnixNano(), messageIDDomain),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: quoted-printable",
	}

	var msg bytes.Buffer
	for _, header := range headers {
		msg.WriteString(header)
		msg.WriteString("\r\n")
	}
	msg.WriteString("\r\n")

	qpWriter := quotedprintable.NewWriter(&msg)
	if _, err := io.WriteString(qpWriter, body); err != nil {
		return nil, "", "", fmt.Errorf("failed to encode email body: %w", err)
	}
	if err := qpWriter.Close(); err != nil {
		return nil, "", "", fmt.Errorf("failed to finalize email body: %w", err)
	}

	return msg.Bytes(), fromAddress, toAddress, nil
}

func normalizeEmailAddress(rawAddress string) (string, error) {
	address, err := mail.ParseAddress(strings.TrimSpace(rawAddress))
	if err != nil {
		return "", err
	}

	return address.Address, nil
}

func emailDomain(address string) string {
	parts := strings.Split(strings.TrimSpace(address), "@")
	if len(parts) != 2 {
		return ""
	}

	return strings.TrimSpace(parts[1])
}

func newSMTPClient(config *NotificationConfig) (*smtp.Client, func(), error) {
	addr := fmt.Sprintf("%s:%d", config.SMTPHost, config.SMTPPort)
	dialer := &net.Dialer{Timeout: 15 * time.Second}

	if config.SMTPPort == 465 {
		conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
			ServerName: config.SMTPHost,
			MinVersion: tls.VersionTLS12,
		})
		if err != nil {
			return nil, func() {}, fmt.Errorf("SMTP TLS dial failed: %w", err)
		}
		if err := conn.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
			_ = conn.Close()
			return nil, func() {}, fmt.Errorf("SMTP deadline setup failed: %w", err)
		}
		client, err := smtp.NewClient(conn, config.SMTPHost)
		if err != nil {
			_ = conn.Close()
			return nil, func() {}, fmt.Errorf("SMTP client init failed: %w", err)
		}
		return client, func() {
			_ = client.Close()
		}, nil
	}

	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, func() {}, fmt.Errorf("SMTP dial failed: %w", err)
	}
	if err := conn.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		_ = conn.Close()
		return nil, func() {}, fmt.Errorf("SMTP deadline setup failed: %w", err)
	}

	client, err := smtp.NewClient(conn, config.SMTPHost)
	if err != nil {
		_ = conn.Close()
		return nil, func() {}, fmt.Errorf("SMTP client init failed: %w", err)
	}

	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{
			ServerName: config.SMTPHost,
			MinVersion: tls.VersionTLS12,
		}); err != nil {
			_ = client.Close()
			return nil, func() {}, fmt.Errorf("SMTP STARTTLS failed: %w", err)
		}
	}

	return client, func() {
		_ = client.Close()
	}, nil
}

// sendTelegram sends a Telegram notification
func (s *Service) sendTelegram(message string) error {
	s.mu.RLock()
	config := s.config
	s.mu.RUnlock()

	if config.TelegramBotToken == "" || config.TelegramChatID == "" {
		return fmt.Errorf("Telegram not configured")
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", config.TelegramBotToken)

	payload := map[string]interface{}{
		"chat_id":    config.TelegramChatID,
		"text":       message,
		"parse_mode": "HTML",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := s.client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned status %d", resp.StatusCode)
	}

	return nil
}
