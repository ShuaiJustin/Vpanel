// Package dispatcher runs background notification jobs (traffic warnings,
// expiry reminders) and exposes broadcast helpers (e.g. for announcements).
//
// The dispatcher reads per-user preferences from the User table:
//   - category flags: NotifyTrafficWarning, NotifyExpiryReminder, NotifyAnnouncements
//   - channel flags:  NotifyEmail, NotifyTelegram
//   - per-user last-sent timestamps used as cooldowns.
//
// It calls out to notification.Service for the actual SMTP / Telegram delivery.
package dispatcher

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"v/internal/database/repository"
	"v/internal/logger"
	"v/internal/notification"
)

const (
	// Tunables. The values are intentionally not exposed via config yet —
	// when product needs are clearer we can promote them. Keep them as
	// constants here so the behaviour is easy to find and reason about.
	trafficWarnThresholdPct = 80
	trafficWarnCooldown     = 24 * time.Hour
	expiryReminderWindow    = 7 * 24 * time.Hour
	expiryReminderCooldown  = 24 * time.Hour

	trafficTickInterval = time.Hour
	expiryTickInterval  = 24 * time.Hour

	// Page size when iterating all users. Keep generous since v.db is SQLite
	// and the user table is typically small. If it ever grows, we'll batch.
	userPageSize = 10000
)

// Dispatcher drives periodic notification jobs and fan-out broadcasts.
type Dispatcher struct {
	notif    *notification.Service
	userRepo repository.UserRepository
	log      logger.Logger
	now      func() time.Time // injectable for tests

	startOnce sync.Once
}

// New constructs a Dispatcher. nil dependencies are tolerated and turn the
// dispatcher into a no-op so callers don't need to guard the call site.
func New(notif *notification.Service, userRepo repository.UserRepository, log logger.Logger) *Dispatcher {
	return &Dispatcher{
		notif:    notif,
		userRepo: userRepo,
		log:      log,
		now:      time.Now,
	}
}

// Start spawns the background tickers. Safe to call once; subsequent calls
// are no-ops. Stops cleanly when ctx is canceled.
func (d *Dispatcher) Start(ctx context.Context) {
	if d == nil || d.notif == nil || d.userRepo == nil {
		return
	}
	d.startOnce.Do(func() {
		go d.runTrafficWarner(ctx)
		go d.runExpiryReminder(ctx)
		if d.log != nil {
			d.log.Info("notification dispatcher started",
				logger.F("traffic_threshold_pct", trafficWarnThresholdPct),
				logger.F("expiry_window_days", int(expiryReminderWindow/(24*time.Hour))),
			)
		}
	})
}

func (d *Dispatcher) runTrafficWarner(ctx context.Context) {
	ticker := time.NewTicker(trafficTickInterval)
	defer ticker.Stop()

	// Run once on startup so a freshly-deployed instance does not wait an
	// hour before its first sweep. Then tick on the interval.
	d.sweepTrafficWarnings(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.sweepTrafficWarnings(ctx)
		}
	}
}

func (d *Dispatcher) runExpiryReminder(ctx context.Context) {
	ticker := time.NewTicker(expiryTickInterval)
	defer ticker.Stop()

	d.sweepExpiryReminders(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.sweepExpiryReminders(ctx)
		}
	}
}

func (d *Dispatcher) sweepTrafficWarnings(ctx context.Context) {
	users, err := d.userRepo.List(ctx, userPageSize, 0)
	if err != nil {
		d.logWarn("traffic warning sweep: list users failed", err)
		return
	}

	now := d.now().UTC()
	sent := 0
	for _, u := range users {
		if u == nil || !u.Enabled || u.TrafficLimit <= 0 {
			continue
		}
		if !u.NotifyTrafficWarning {
			continue
		}
		usagePct := int((u.TrafficUsed * 100) / u.TrafficLimit)
		if usagePct < trafficWarnThresholdPct {
			continue
		}
		if u.LastTrafficWarningAt != nil && now.Sub(*u.LastTrafficWarningAt) < trafficWarnCooldown {
			continue
		}

		subj := "[V Panel] 流量使用提醒"
		body := fmt.Sprintf("您好 %s，\n\n您的流量已使用 %d%%（%s / %s），请关注用量，以免影响服务使用。\n\n— V Panel",
			displayName(u), usagePct, formatBytes(u.TrafficUsed), formatBytes(u.TrafficLimit))

		if d.deliver(u, subj, body) {
			stamp := now
			u.LastTrafficWarningAt = &stamp
			if err := d.userRepo.Update(ctx, u); err != nil {
				d.logWarn("traffic warning: update LastTrafficWarningAt failed", err)
			}
			sent++
		}
	}
	if sent > 0 && d.log != nil {
		d.log.Info("traffic warnings dispatched", logger.F("count", sent))
	}
}

func (d *Dispatcher) sweepExpiryReminders(ctx context.Context) {
	users, err := d.userRepo.List(ctx, userPageSize, 0)
	if err != nil {
		d.logWarn("expiry reminder sweep: list users failed", err)
		return
	}

	now := d.now().UTC()
	sent := 0
	for _, u := range users {
		if u == nil || !u.Enabled || u.ExpiresAt == nil {
			continue
		}
		if !u.NotifyExpiryReminder {
			continue
		}
		remaining := u.ExpiresAt.Sub(now)
		if remaining <= 0 || remaining > expiryReminderWindow {
			continue
		}
		if u.LastExpiryReminderAt != nil && now.Sub(*u.LastExpiryReminderAt) < expiryReminderCooldown {
			continue
		}

		days := int(remaining / (24 * time.Hour))
		if days < 0 {
			days = 0
		}
		subj := "[V Panel] 账户即将到期"
		body := fmt.Sprintf("您好 %s，\n\n您的账户将在 %d 天后到期（%s），请及时续费以免中断服务。\n\n— V Panel",
			displayName(u), days, u.ExpiresAt.Format("2006-01-02"))

		if d.deliver(u, subj, body) {
			stamp := now
			u.LastExpiryReminderAt = &stamp
			if err := d.userRepo.Update(ctx, u); err != nil {
				d.logWarn("expiry reminder: update LastExpiryReminderAt failed", err)
			}
			sent++
		}
	}
	if sent > 0 && d.log != nil {
		d.log.Info("expiry reminders dispatched", logger.F("count", sent))
	}
}

// BroadcastAnnouncement fans out an announcement to every enabled user that
// has NotifyAnnouncements set and at least one usable channel. Intended to
// be called from an admin "create announcement" handler. Runs asynchronously
// so it never blocks the request.
func (d *Dispatcher) BroadcastAnnouncement(ctx context.Context, title, body string) {
	if d == nil || d.notif == nil || d.userRepo == nil {
		return
	}
	go d.broadcastAnnouncement(ctx, title, body)
}

func (d *Dispatcher) broadcastAnnouncement(ctx context.Context, title, body string) {
	users, err := d.userRepo.List(ctx, userPageSize, 0)
	if err != nil {
		d.logWarn("announcement broadcast: list users failed", err)
		return
	}
	subj := "[V Panel] " + title
	full := fmt.Sprintf("%s\n\n— V Panel", body)
	sent := 0
	for _, u := range users {
		if u == nil || !u.Enabled || !u.NotifyAnnouncements {
			continue
		}
		if d.deliver(u, subj, full) {
			sent++
		}
	}
	if d.log != nil {
		d.log.Info("announcement broadcast complete", logger.F("count", sent))
	}
}

// deliver attempts to send the message via every channel the user has both
// enabled AND configured. Returns true if at least one channel succeeded.
func (d *Dispatcher) deliver(u *repository.User, subject, body string) bool {
	delivered := false

	if u.NotifyEmail && strings.TrimSpace(u.Email) != "" && d.notif.CanSendEmail() {
		if err := d.notif.SendEmail(u.Email, subject, body); err != nil {
			d.logWarn("send email failed (user "+u.Username+")", err)
		} else {
			delivered = true
		}
	}

	if u.NotifyTelegram && strings.TrimSpace(u.TelegramID) != "" && d.notif.CanSendTelegram() {
		// Telegram payload uses subject + blank line + body so the user
		// sees the category at a glance in their chat.
		msg := subject + "\n\n" + body
		if err := d.notif.SendTelegramTo(u.TelegramID, msg); err != nil {
			d.logWarn("send telegram failed (user "+u.Username+")", err)
		} else {
			delivered = true
		}
	}

	return delivered
}

func (d *Dispatcher) logWarn(msg string, err error) {
	if d.log == nil {
		return
	}
	d.log.Warn(msg, logger.F("error", err))
}

func displayName(u *repository.User) string {
	if strings.TrimSpace(u.DisplayName) != "" {
		return u.DisplayName
	}
	return u.Username
}

// formatBytes renders a byte count using binary units (KiB / MiB / GiB / TiB).
// Kept here instead of pulling in humanize to avoid a new dependency.
func formatBytes(n int64) string {
	const k = 1024
	if n < k {
		return fmt.Sprintf("%dB", n)
	}
	units := []string{"KiB", "MiB", "GiB", "TiB", "PiB"}
	value := float64(n) / k
	i := 0
	for value >= k && i < len(units)-1 {
		value /= k
		i++
	}
	return fmt.Sprintf("%.2f%s", value, units[i])
}
