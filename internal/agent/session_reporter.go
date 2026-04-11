package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"v/internal/logger"
)

const (
	defaultProxySessionLookback = 10 * time.Minute
	maxInitialAccessLogScan     = 4 << 20
)

var (
	xrayAccessTimestampPattern = regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}(?:\.\d+)?)`)
	xrayAccessEmailPattern     = regexp.MustCompile(`user-(\d+)-proxy-(\d+)`)
	xrayAccessFromPattern      = regexp.MustCompile(`from\s+\[?([0-9a-fA-F:.]+)\]?:\d+`)
	xrayAccessProtoPattern     = regexp.MustCompile(`(?:tcp|udp):\[?([0-9a-fA-F:.]+)\]?:\d+`)
)

// ProxySessionRecord represents a recent proxy session observed on the node.
type ProxySessionRecord struct {
	UserID     int64  `json:"user_id"`
	ProxyID    int64  `json:"proxy_id"`
	IP         string `json:"ip"`
	LastSeen   int64  `json:"last_seen"`
	DeviceInfo string `json:"device_info,omitempty"`
}

type accessSessionKey struct {
	userID  int64
	proxyID int64
	ip      string
}

type accessSessionState struct {
	record   ProxySessionRecord
	lastSeen time.Time
}

type xrayAccessLogConfig struct {
	Log *struct {
		Access string `json:"access"`
	} `json:"log"`
}

type sessionReporter struct {
	configPath string
	logger     logger.Logger
	readFile   func(string) ([]byte, error)
	openFile   func(string) (*os.File, error)
	lookback   time.Duration

	mu              sync.Mutex
	resolvedLogPath string
	lastOffset      int64
	partialLine     string
	recentSessions  map[accessSessionKey]accessSessionState
}

func newSessionReporter(cfg XrayConfig, log logger.Logger) *sessionReporter {
	return &sessionReporter{
		configPath:     normalizeTrafficConfigPath(cfg.ConfigPath),
		logger:         log,
		readFile:       os.ReadFile,
		openFile:       os.Open,
		lookback:       defaultProxySessionLookback,
		recentSessions: make(map[accessSessionKey]accessSessionState),
	}
}

func (r *sessionReporter) CollectRecentSessions(ctx context.Context) ([]ProxySessionRecord, error) {
	_ = ctx

	r.mu.Lock()
	defer r.mu.Unlock()

	logPath, err := r.resolveAccessLogPath()
	if err != nil {
		r.pruneLocked(time.Now().UTC())
		return nil, err
	}

	if logPath != r.resolvedLogPath {
		r.resolvedLogPath = logPath
		r.lastOffset = 0
		r.partialLine = ""
		r.recentSessions = make(map[accessSessionKey]accessSessionState)
	}

	if err := r.ingestLocked(logPath); err != nil {
		r.pruneLocked(time.Now().UTC())
		return nil, err
	}

	now := time.Now().UTC()
	r.pruneLocked(now)

	records := make([]ProxySessionRecord, 0, len(r.recentSessions))
	for _, session := range r.recentSessions {
		records = append(records, session.record)
	}

	sort.Slice(records, func(i, j int) bool {
		if records[i].LastSeen != records[j].LastSeen {
			return records[i].LastSeen > records[j].LastSeen
		}
		if records[i].UserID != records[j].UserID {
			return records[i].UserID < records[j].UserID
		}
		if records[i].ProxyID != records[j].ProxyID {
			return records[i].ProxyID < records[j].ProxyID
		}
		return records[i].IP < records[j].IP
	})

	return records, nil
}

func (r *sessionReporter) resolveAccessLogPath() (string, error) {
	reader := r.readFile
	if reader == nil {
		reader = os.ReadFile
	}

	data, err := reader(r.configPath)
	if err != nil {
		return "", fmt.Errorf("read xray config failed: %w", err)
	}

	var cfg xrayAccessLogConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("parse xray config failed: %w", err)
	}

	if cfg.Log == nil || strings.TrimSpace(cfg.Log.Access) == "" {
		return "", fmt.Errorf("xray access log path is not configured")
	}

	resolved := strings.TrimSpace(cfg.Log.Access)
	if filepath.IsAbs(resolved) {
		return filepath.Clean(resolved), nil
	}

	baseDir := filepath.Dir(r.configPath)
	if strings.TrimSpace(baseDir) == "" || baseDir == "." {
		return filepath.Clean(resolved), nil
	}
	return filepath.Clean(filepath.Join(baseDir, resolved)), nil
}

func (r *sessionReporter) ingestLocked(logPath string) error {
	openFn := r.openFile
	if openFn == nil {
		openFn = os.Open
	}

	file, err := openFn(logPath)
	if err != nil {
		return fmt.Errorf("open xray access log failed: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat xray access log failed: %w", err)
	}

	startOffset := r.lastOffset
	initialScan := false
	if startOffset == 0 && info.Size() > maxInitialAccessLogScan {
		startOffset = info.Size() - maxInitialAccessLogScan
		initialScan = true
	}
	if startOffset > info.Size() {
		startOffset = 0
		r.partialLine = ""
	}

	if _, err := file.Seek(startOffset, io.SeekStart); err != nil {
		return fmt.Errorf("seek xray access log failed: %w", err)
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("read xray access log failed: %w", err)
	}
	r.lastOffset = info.Size()

	if len(data) == 0 {
		return nil
	}
	if initialScan && startOffset > 0 {
		if idx := strings.IndexByte(string(data), '\n'); idx >= 0 {
			data = data[idx+1:]
		} else {
			r.partialLine = ""
			return nil
		}
	}

	content := r.partialLine + string(data)
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return nil
	}

	if !strings.HasSuffix(content, "\n") {
		r.partialLine = lines[len(lines)-1]
		lines = lines[:len(lines)-1]
	} else {
		r.partialLine = ""
	}

	for _, line := range lines {
		record, seenAt, ok := parseXrayAccessLogLine(line)
		if !ok {
			continue
		}
		if seenAt.IsZero() {
			seenAt = time.Now().UTC()
		}

		record.LastSeen = seenAt.Unix()
		key := accessSessionKey{
			userID:  record.UserID,
			proxyID: record.ProxyID,
			ip:      record.IP,
		}
		existing, exists := r.recentSessions[key]
		if !exists || seenAt.After(existing.lastSeen) {
			r.recentSessions[key] = accessSessionState{
				record:   record,
				lastSeen: seenAt,
			}
		}
	}

	return nil
}

func (r *sessionReporter) pruneLocked(now time.Time) {
	cutoff := now.Add(-r.lookback)
	for key, session := range r.recentSessions {
		if session.lastSeen.Before(cutoff) {
			delete(r.recentSessions, key)
		}
	}
}

func parseXrayAccessLogLine(line string) (ProxySessionRecord, time.Time, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return ProxySessionRecord{}, time.Time{}, false
	}

	emailMatch := xrayAccessEmailPattern.FindStringSubmatch(trimmed)
	if len(emailMatch) != 3 {
		return ProxySessionRecord{}, time.Time{}, false
	}

	userID, err := strconv.ParseInt(emailMatch[1], 10, 64)
	if err != nil || userID <= 0 {
		return ProxySessionRecord{}, time.Time{}, false
	}
	proxyID, err := strconv.ParseInt(emailMatch[2], 10, 64)
	if err != nil || proxyID <= 0 {
		return ProxySessionRecord{}, time.Time{}, false
	}

	var seenAt time.Time
	if tsMatch := xrayAccessTimestampPattern.FindStringSubmatch(trimmed); len(tsMatch) == 2 {
		for _, layout := range []string{"2006/01/02 15:04:05.999999999", "2006/01/02 15:04:05"} {
			if parsed, parseErr := time.ParseInLocation(layout, tsMatch[1], time.Local); parseErr == nil {
				seenAt = parsed.UTC()
				break
			}
		}
	}

	sourceSection := trimmed
	if acceptedIdx := strings.Index(sourceSection, " accepted "); acceptedIdx > 0 {
		sourceSection = sourceSection[:acceptedIdx]
	}

	ip := ""
	if fromMatch := xrayAccessFromPattern.FindStringSubmatch(sourceSection); len(fromMatch) == 2 {
		ip = normalizeAccessLogIP(fromMatch[1])
	}
	if ip == "" {
		matches := xrayAccessProtoPattern.FindAllStringSubmatch(sourceSection, -1)
		for _, match := range matches {
			if len(match) != 2 {
				continue
			}
			ip = normalizeAccessLogIP(match[1])
			if ip != "" {
				break
			}
		}
	}
	if ip == "" {
		return ProxySessionRecord{}, time.Time{}, false
	}

	return ProxySessionRecord{
		UserID:     userID,
		ProxyID:    proxyID,
		IP:         ip,
		DeviceInfo: fmt.Sprintf("Proxy #%d connection", proxyID),
	}, seenAt, true
}

func normalizeAccessLogIP(candidate string) string {
	ip := net.ParseIP(strings.Trim(strings.TrimSpace(candidate), "[]"))
	if ip == nil {
		return ""
	}
	return ip.String()
}
