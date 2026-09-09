package notification

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"v/internal/logger"
)

// Outbox jobs contain no SMTP credentials. A job is persisted before delivery;
// successful jobs remain for 180 days to deduplicate certificate lifecycle events.
type deliveryJob struct {
	Channel     NotificationChannel `json:"channel"`
	Recipient   string              `json:"recipient,omitempty"`
	Subject     string              `json:"subject"`
	Body        string              `json:"body"`
	Attempts    int                 `json:"attempts"`
	LastError   string              `json:"last_error,omitempty"`
	NextAttempt time.Time           `json:"next_attempt"`
	DeliveredAt *time.Time          `json:"delivered_at,omitempty"`
	CancelledAt *time.Time          `json:"cancelled_at,omitempty"`
}

var errChannelDisabled = errors.New("notification channel disabled")

// ConfigureOutbox must be called before the service starts, with a directory on
// the application's persistent data volume. Failure must not be silently ignored.
func (s *Service) ConfigureOutbox(dir string) error {
	if !filepath.IsAbs(dir) || filepath.Clean(dir) == string(filepath.Separator) {
		return fmt.Errorf("notification outbox requires an absolute data directory")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("outbox must be a real directory")
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return err
	}
	s.outboxDir = dir
	return nil
}

// StartOutbox retries persisted jobs immediately and then once a minute. The
// caller owns cancellation. SMTP acceptance provides at-least-once delivery:
// a crash between delivery and persistence can cause one duplicate message.
func (s *Service) StartOutbox(ctx context.Context, log logger.Logger) {
	if s.outboxDir == "" {
		return
	}
	s.outboxOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for {
				if err := s.processOutbox(ctx, time.Now(), s.deliverJob); err != nil && ctx.Err() == nil && log != nil {
					log.Error("Notification outbox delivery failed; retry retained", logger.Err(err))
				}
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
			}
		}()
	})
}

func (s *Service) enqueueAdmin(key, subject, body string) error {
	s.mu.RLock()
	cfg := s.config
	if cfg == nil {
		s.mu.RUnlock()
		return nil
	}
	email := strings.TrimSpace(cfg.AdminEmail)
	if email == "" {
		email = strings.TrimSpace(cfg.SMTPUser)
	}
	jobs := []deliveryJob{}
	if cfg.EnabledChannels[ChannelEmail] && email != "" {
		jobs = append(jobs, deliveryJob{Channel: ChannelEmail, Recipient: email, Subject: subject, Body: body})
	}
	if cfg.EnabledChannels[ChannelTelegram] {
		jobs = append(jobs, deliveryJob{Channel: ChannelTelegram, Recipient: cfg.TelegramChatID, Subject: subject, Body: body})
	}
	s.mu.RUnlock()
	if key == "" {
		key = subject + "\n" + body
	}
	s.outboxMu.Lock()
	defer s.outboxMu.Unlock()
	for _, job := range jobs {
		digest := sha256.Sum256([]byte(string(job.Channel) + "\n" + key))
		path := filepath.Join(s.outboxDir, fmt.Sprintf("%x.json", digest))
		if _, err := os.Stat(path); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return err
		}
		job.NextAttempt = time.Now()
		if err := writeDeliveryJob(path, &job); err != nil {
			return fmt.Errorf("persist notification: %w", err)
		}
	}
	return nil
}

func writeDeliveryJob(path string, job *deliveryJob) error {
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".delivery-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func (s *Service) processOutbox(ctx context.Context, now time.Time, send func(*deliveryJob) error) error {
	entries, err := os.ReadDir(s.outboxDir)
	if err != nil {
		return err
	}
	var lastErr error
	processed := 0
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(s.outboxDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			lastErr = err
			continue
		}
		var job deliveryJob
		if err := json.Unmarshal(data, &job); err != nil {
			lastErr = fmt.Errorf("invalid notification outbox record")
			continue
		}
		completedAt := job.DeliveredAt
		if job.CancelledAt != nil {
			completedAt = job.CancelledAt
		}
		if completedAt != nil {
			if now.Sub(*completedAt) > 180*24*time.Hour {
				s.outboxMu.Lock()
				err := os.Remove(path)
				s.outboxMu.Unlock()
				if err != nil {
					lastErr = err
				}
			}
			continue
		}
		if now.Before(job.NextAttempt) || processed >= 20 {
			continue
		}
		processed++
		if err := send(&job); errors.Is(err, errChannelDisabled) {
			job.CancelledAt = &now
			job.LastError = "notification channel disabled"
		} else if err != nil {
			lastErr = fmt.Errorf("%s delivery failed; retained for retry", job.Channel)
			job.LastError = err.Error()
			if len(job.LastError) > 1024 {
				job.LastError = job.LastError[:1024]
			}
			job.Attempts++
			shift := min(job.Attempts-1, 6)
			job.NextAttempt = now.Add(time.Minute * time.Duration(1<<shift))
		} else {
			job.DeliveredAt = &now
			job.LastError = ""
		}
		s.outboxMu.Lock()
		err = writeDeliveryJob(path, &job)
		s.outboxMu.Unlock()
		if err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (s *Service) deliverJob(job *deliveryJob) error {
	s.mu.RLock()
	enabled := s.config != nil && s.config.EnabledChannels[job.Channel]
	s.mu.RUnlock()
	if !enabled {
		return errChannelDisabled
	}
	switch job.Channel {
	case ChannelEmail:
		return s.sendEmail(job.Recipient, job.Subject, job.Body)
	case ChannelTelegram:
		return s.SendTelegramTo(job.Recipient, "🔔 "+job.Subject+"\n\n"+job.Body)
	default:
		return fmt.Errorf("unknown notification channel")
	}
}
