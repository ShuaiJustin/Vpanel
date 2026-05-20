// Package backup runs a periodic SQLite snapshot of the v.db file plus
// retention pruning. Production deployments should rely on this rather than
// the manual "备份数据库" button, which is fine for ad-hoc snapshots but
// won't help when an admin forgets to click it before disaster strikes.
//
// The implementation matches the manual handler in handlers/settings.go:
// it copies the database file byte-for-byte. With SQLite in WAL mode (the
// default), this is safe enough for the kind of workloads V Panel handles
// (single-host, modest write concurrency). For higher durability requirements
// users should layer Litestream or the SQLite backup API on top.
package backup

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"v/internal/logger"
)

// Scheduler periodically snapshots a SQLite database to a sibling backups/
// directory and prunes old snapshots.
type Scheduler struct {
	dbPath        string
	retentionDays int
	hourLocal     int // 0-23, e.g. 3 = "around 3am local time"
	logger        logger.Logger
}

// New constructs a Scheduler. retentionDays <= 0 disables pruning.
// hourLocal picks when the daily run fires (rounded to the next occurrence
// after Start is called).
func New(dbPath string, retentionDays, hourLocal int, log logger.Logger) *Scheduler {
	if hourLocal < 0 || hourLocal > 23 {
		hourLocal = 3
	}
	return &Scheduler{
		dbPath:        dbPath,
		retentionDays: retentionDays,
		hourLocal:     hourLocal,
		logger:        log,
	}
}

// Start spawns a goroutine that runs RunOnce daily at the configured hour.
// Stops cleanly when ctx is canceled. Safe to call once.
func (s *Scheduler) Start(ctx context.Context) {
	if s == nil || strings.TrimSpace(s.dbPath) == "" {
		return
	}
	go s.loop(ctx)
}

func (s *Scheduler) loop(ctx context.Context) {
	if s.logger != nil {
		s.logger.Info("backup scheduler started",
			logger.F("db_path", s.dbPath),
			logger.F("retention_days", s.retentionDays),
			logger.F("hour_local", s.hourLocal),
		)
	}
	for {
		next := nextRunAt(time.Now(), s.hourLocal)
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(next)):
		}

		path, err := s.RunOnce(ctx)
		if err != nil {
			if s.logger != nil {
				s.logger.Error("scheduled backup failed", logger.F("error", err))
			}
			continue
		}
		if s.logger != nil {
			s.logger.Info("scheduled backup written", logger.F("path", path))
		}
		if pruned, err := s.Prune(); err != nil {
			if s.logger != nil {
				s.logger.Warn("backup prune failed", logger.F("error", err))
			}
		} else if pruned > 0 && s.logger != nil {
			s.logger.Info("backup retention pruned old snapshots", logger.F("count", pruned))
		}
	}
}

// RunOnce takes a single snapshot now. Exposed so an admin endpoint or
// startup hook can trigger an immediate backup. Returns the path written.
func (s *Scheduler) RunOnce(ctx context.Context) (string, error) {
	if _, err := os.Stat(s.dbPath); err != nil {
		return "", fmt.Errorf("stat db file: %w", err)
	}
	backupDir := filepath.Join(filepath.Dir(s.dbPath), "backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}

	stamp := time.Now().Format("20060102_150405")
	out := filepath.Join(backupDir, fmt.Sprintf("vpanel_db_%s.db", stamp))

	src, err := os.Open(s.dbPath)
	if err != nil {
		return "", fmt.Errorf("open source: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(out)
	if err != nil {
		return "", fmt.Errorf("create dest: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		_ = os.Remove(out)
		return "", fmt.Errorf("copy: %w", err)
	}
	return out, nil
}

// Prune deletes snapshots older than retentionDays. Returns the count
// removed. A retention of 0 or negative is treated as "keep everything".
func (s *Scheduler) Prune() (int, error) {
	if s.retentionDays <= 0 {
		return 0, nil
	}
	backupDir := filepath.Join(filepath.Dir(s.dbPath), "backups")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	cutoff := time.Now().AddDate(0, 0, -s.retentionDays)
	removed := 0

	// Sort by name so deletions are deterministic in logs; the naming
	// format is timestamp-prefixed so name order matches creation order.
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "vpanel_db_") || !strings.HasSuffix(name, ".db") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		if err := os.Remove(filepath.Join(backupDir, name)); err == nil {
			removed++
		}
	}
	return removed, nil
}

// nextRunAt returns the next time on or after `from` whose local clock is
// at hourLocal:00:00. Always returns a time strictly in the future so an
// inflight loop iteration doesn't end up firing immediately again.
func nextRunAt(from time.Time, hourLocal int) time.Time {
	loc := time.Local
	t := time.Date(from.Year(), from.Month(), from.Day(), hourLocal, 0, 0, 0, loc)
	if !t.After(from) {
		t = t.Add(24 * time.Hour)
	}
	return t
}
