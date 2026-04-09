// Package agent provides the Node Agent functionality for V Panel.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"v/internal/logger"
)

var userTrafficStatPattern = regexp.MustCompile(`^user>>>user-(\d+)-proxy-(\d+)>>>traffic>>>(uplink|downlink)$`)

const (
	TrafficCollectorStatusUnknown           = "unknown"
	TrafficCollectorStatusHealthyCollecting = "healthy_collecting"
	TrafficCollectorStatusHealthyIdle       = "healthy_idle"
	TrafficCollectorStatusCollectorError    = "collector_error"
)

// TrafficRecord represents per-user traffic collected on a node.
type TrafficRecord struct {
	UserID   int64  `json:"user_id"`
	ProxyID  *int64 `json:"proxy_id,omitempty"`
	Upload   int64  `json:"upload"`
	Download int64  `json:"download"`
}

// TrafficSnapshot stores a point-in-time view of Xray counters.
type TrafficSnapshot struct {
	counters map[string]int64
}

// TrafficCollectorStatus describes the agent's current traffic collection state.
type TrafficCollectorStatus struct {
	Status               string     `json:"status"`
	ConfiguredConfigPath string     `json:"configured_config_path,omitempty"`
	ResolvedConfigPath   string     `json:"resolved_config_path,omitempty"`
	CandidateConfigPaths []string   `json:"candidate_config_paths,omitempty"`
	APIPort              int        `json:"api_port,omitempty"`
	XrayRunning          bool       `json:"xray_running"`
	LastCollectionAt     *time.Time `json:"last_collection_at,omitempty"`
	LastSuccessAt        *time.Time `json:"last_success_at,omitempty"`
	LastError            string     `json:"last_error,omitempty"`
	LastErrorAt          *time.Time `json:"last_error_at,omitempty"`
	LastRecordCount      int        `json:"last_record_count"`
}

type trafficCollectionResolution struct {
	ConfigPath          string
	CandidateConfigPath []string
	APIPort             int
}

type combinedOutputRunner interface {
	CombinedOutput() ([]byte, error)
}

type execCombinedOutputCommand struct {
	cmd *exec.Cmd
}

func (c execCombinedOutputCommand) CombinedOutput() ([]byte, error) {
	return c.cmd.CombinedOutput()
}

type combinedOutputFunc func() ([]byte, error)

func (f combinedOutputFunc) CombinedOutput() ([]byte, error) {
	return f()
}

var execCommandContext = func(ctx context.Context, name string, arg ...string) combinedOutputRunner {
	return execCombinedOutputCommand{cmd: exec.CommandContext(ctx, name, arg...)}
}

type trafficReporter struct {
	binaryPath  string
	configPath  string
	logger      logger.Logger
	readFile    func(string) ([]byte, error)
	processList func() (string, error)

	mu            sync.Mutex
	lastCommitted map[string]int64
	status        *TrafficCollectorStatus
}

func (r *trafficReporter) ExportCommittedCounters() map[string]int64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	cloned := make(map[string]int64, len(r.lastCommitted))
	for name, value := range r.lastCommitted {
		cloned[name] = value
	}
	return cloned
}

func (r *trafficReporter) RestoreCommittedCounters(counters map[string]int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.lastCommitted = make(map[string]int64, len(counters))
	for name, value := range counters {
		r.lastCommitted[name] = value
	}
}

type xrayRuntimeConfig struct {
	API *struct {
		Tag string `json:"tag"`
	} `json:"api"`
	Inbounds []struct {
		Tag  string `json:"tag"`
		Port int    `json:"port"`
	} `json:"inbounds"`
}

type xrayStatsQueryResponse struct {
	Stat []xrayStatEntry `json:"stat"`
}

type xrayStatEntry struct {
	Name  string          `json:"name"`
	Value json.RawMessage `json:"value"`
}

type trafficAggregateKey struct {
	userID  int64
	proxyID int64
}

func newTrafficReporter(cfg XrayConfig, log logger.Logger) *trafficReporter {
	configPath := normalizeTrafficConfigPath(cfg.ConfigPath)
	return &trafficReporter{
		binaryPath:    cfg.BinaryPath,
		configPath:    configPath,
		logger:        log,
		readFile:      os.ReadFile,
		processList:   defaultXrayProcessList,
		lastCommitted: make(map[string]int64),
		status: &TrafficCollectorStatus{
			Status:               TrafficCollectorStatusUnknown,
			ConfiguredConfigPath: configPath,
		},
	}
}

func (r *trafficReporter) PrepareDelta(ctx context.Context) (*TrafficSnapshot, []TrafficRecord, error) {
	collectedAt := time.Now().UTC()
	currentCounters, resolution, err := r.queryCounters(ctx)
	if err != nil {
		r.recordCollectionFailure(resolution, collectedAt, err)
		return nil, nil, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	aggregated := make(map[trafficAggregateKey]*TrafficRecord)
	for name, currentValue := range currentCounters {
		userID, proxyID, direction, ok := parseUserTrafficStatName(name)
		if !ok {
			continue
		}

		previousValue := r.lastCommitted[name]
		delta := currentValue - previousValue
		if delta < 0 {
			delta = currentValue
		}
		if delta == 0 {
			continue
		}

		key := trafficAggregateKey{userID: userID, proxyID: proxyID}
		record, exists := aggregated[key]
		if !exists {
			proxyIDCopy := proxyID
			record = &TrafficRecord{
				UserID:  userID,
				ProxyID: &proxyIDCopy,
			}
			aggregated[key] = record
		}

		if direction == "uplink" {
			record.Upload += delta
		} else {
			record.Download += delta
		}
	}

	records := make([]TrafficRecord, 0, len(aggregated))
	for _, record := range aggregated {
		if record.Upload == 0 && record.Download == 0 {
			continue
		}
		records = append(records, *record)
	}

	r.applyCollectionSuccessLocked(resolution, collectedAt, len(records))

	return &TrafficSnapshot{counters: currentCounters}, records, nil
}

func (r *trafficReporter) Commit(snapshot *TrafficSnapshot) {
	if snapshot == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.lastCommitted = make(map[string]int64, len(snapshot.counters))
	for name, value := range snapshot.counters {
		r.lastCommitted[name] = value
	}
}

func (r *trafficReporter) GetCollectorStatus() *TrafficCollectorStatus {
	r.mu.Lock()
	defer r.mu.Unlock()

	return cloneTrafficCollectorStatus(r.status)
}

func (r *trafficReporter) queryCounters(ctx context.Context) (map[string]int64, *trafficCollectionResolution, error) {
	resolution, err := r.resolveAPIPort()
	if err != nil {
		return nil, resolution, err
	}

	output, err := execCommandContext(
		ctx,
		r.binaryPath,
		"api",
		"statsquery",
		fmt.Sprintf("--server=127.0.0.1:%d", resolution.APIPort),
	).CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if trimmed == "" {
			trimmed = err.Error()
		}
		return nil, resolution, fmt.Errorf("query xray stats failed: %s", trimmed)
	}

	var response xrayStatsQueryResponse
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, resolution, fmt.Errorf("parse xray stats failed: %w", err)
	}

	counters := make(map[string]int64, len(response.Stat))
	for _, entry := range response.Stat {
		value, err := parseXrayStatValue(entry.Value)
		if err != nil {
			r.logger.Warn("skip xray stat with invalid value",
				logger.F("name", entry.Name),
				logger.F("error", err.Error()))
			continue
		}
		counters[entry.Name] = value
	}

	return counters, resolution, nil
}

func (r *trafficReporter) resolveAPIPort() (*trafficCollectionResolution, error) {
	resolution := &trafficCollectionResolution{
		ConfigPath:          "",
		CandidateConfigPath: r.candidateConfigPaths(),
	}

	var attemptErrors []string
	for _, candidate := range resolution.CandidateConfigPath {
		port, err := r.resolveAPIPortFromConfig(candidate)
		if err == nil {
			resolution.ConfigPath = candidate
			resolution.APIPort = port
			return resolution, nil
		}
		attemptErrors = append(attemptErrors, fmt.Sprintf("%s: %v", candidate, err))
	}

	if len(attemptErrors) == 0 {
		return resolution, fmt.Errorf("xray api inbound not found: no config paths available")
	}

	return resolution, fmt.Errorf("xray api inbound not found; attempted %s", strings.Join(attemptErrors, "; "))
}

func (r *trafficReporter) resolveAPIPortFromConfig(configPath string) (int, error) {
	reader := r.readFile
	if reader == nil {
		reader = os.ReadFile
	}

	data, err := reader(configPath)
	if err != nil {
		return 0, fmt.Errorf("read xray config failed: %w", err)
	}

	var cfg xrayRuntimeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return 0, fmt.Errorf("parse xray config failed: %w", err)
	}

	apiTag := "api"
	if cfg.API != nil && strings.TrimSpace(cfg.API.Tag) != "" {
		apiTag = strings.TrimSpace(cfg.API.Tag)
	}

	for _, inbound := range cfg.Inbounds {
		if strings.TrimSpace(inbound.Tag) == apiTag && inbound.Port > 0 {
			return inbound.Port, nil
		}
	}

	return 0, fmt.Errorf("xray api inbound not found")
}

func (r *trafficReporter) candidateConfigPaths() []string {
	candidates := make([]string, 0, 4)
	seen := make(map[string]struct{})

	addCandidate := func(path string) {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			return
		}
		cleaned := filepath.Clean(trimmed)
		if cleaned == "." {
			return
		}
		if _, exists := seen[cleaned]; exists {
			return
		}
		seen[cleaned] = struct{}{}
		candidates = append(candidates, cleaned)
	}

	addCandidate(r.configPath)
	for _, path := range r.runningXrayConfigPaths() {
		addCandidate(path)
	}
	addCandidate("/etc/xray/config.json")
	addCandidate("/usr/local/etc/xray/config.json")

	return candidates
}

func (r *trafficReporter) runningXrayConfigPaths() []string {
	processList := r.processList
	if processList == nil {
		processList = defaultXrayProcessList
	}

	output, err := processList()
	if err != nil || strings.TrimSpace(output) == "" {
		return nil
	}

	paths := make([]string, 0)
	seen := make(map[string]struct{})
	for _, line := range strings.Split(output, "\n") {
		_, args, ok := parseXrayProcessLine(line)
		if !ok {
			continue
		}
		configPath := extractXrayConfigPathFromArgs(args)
		if configPath == "" {
			continue
		}
		if _, exists := seen[configPath]; exists {
			continue
		}
		seen[configPath] = struct{}{}
		paths = append(paths, configPath)
	}

	return paths
}

func extractXrayConfigPathFromArgs(args string) string {
	fields := strings.Fields(strings.TrimSpace(args))
	for i := 0; i < len(fields); i++ {
		field := strings.TrimSpace(fields[i])
		switch {
		case field == "-config" || field == "-c":
			if i+1 >= len(fields) {
				return ""
			}
			return filepath.Clean(strings.TrimSpace(fields[i+1]))
		case strings.HasPrefix(field, "-config="):
			return filepath.Clean(strings.TrimSpace(strings.TrimPrefix(field, "-config=")))
		case strings.HasPrefix(field, "-c="):
			return filepath.Clean(strings.TrimSpace(strings.TrimPrefix(field, "-c=")))
		}
	}
	return ""
}

func defaultXrayProcessList() (string, error) {
	output, err := exec.Command("ps", "-eo", "pid=", "-o", "comm=", "-o", "args=").Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func (r *trafficReporter) recordCollectionFailure(resolution *trafficCollectionResolution, collectedAt time.Time, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	status := r.ensureStatusLocked()
	status.Status = TrafficCollectorStatusCollectorError
	status.ConfiguredConfigPath = normalizeTrafficConfigPath(r.configPath)
	status.CandidateConfigPaths = cloneStringSlice(nil)
	status.ResolvedConfigPath = ""
	status.APIPort = 0
	status.LastCollectionAt = cloneTimePtr(&collectedAt)
	status.LastRecordCount = 0
	if err != nil {
		status.LastError = err.Error()
		status.LastErrorAt = cloneTimePtr(&collectedAt)
	}
	if resolution != nil {
		status.CandidateConfigPaths = cloneStringSlice(resolution.CandidateConfigPath)
		status.ResolvedConfigPath = normalizeTrafficConfigPath(resolution.ConfigPath)
		status.APIPort = resolution.APIPort
	}
}

func (r *trafficReporter) applyCollectionSuccessLocked(resolution *trafficCollectionResolution, collectedAt time.Time, recordCount int) {
	status := r.ensureStatusLocked()
	if recordCount > 0 {
		status.Status = TrafficCollectorStatusHealthyCollecting
	} else {
		status.Status = TrafficCollectorStatusHealthyIdle
	}
	status.ConfiguredConfigPath = normalizeTrafficConfigPath(r.configPath)
	status.ResolvedConfigPath = ""
	status.CandidateConfigPaths = cloneStringSlice(nil)
	status.APIPort = 0
	if resolution != nil {
		status.ResolvedConfigPath = normalizeTrafficConfigPath(resolution.ConfigPath)
		status.CandidateConfigPaths = cloneStringSlice(resolution.CandidateConfigPath)
		status.APIPort = resolution.APIPort
	}
	status.LastCollectionAt = cloneTimePtr(&collectedAt)
	status.LastSuccessAt = cloneTimePtr(&collectedAt)
	status.LastError = ""
	status.LastErrorAt = nil
	status.LastRecordCount = recordCount
}

func (r *trafficReporter) ensureStatusLocked() *TrafficCollectorStatus {
	if r.status == nil {
		r.status = &TrafficCollectorStatus{
			Status:               TrafficCollectorStatusUnknown,
			ConfiguredConfigPath: normalizeTrafficConfigPath(r.configPath),
		}
	}
	return r.status
}

func normalizeTrafficConfigPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	cleaned := filepath.Clean(trimmed)
	if cleaned == "." {
		return ""
	}
	return cleaned
}

func cloneTrafficCollectorStatus(status *TrafficCollectorStatus) *TrafficCollectorStatus {
	if status == nil {
		return nil
	}

	cloned := *status
	cloned.CandidateConfigPaths = cloneStringSlice(status.CandidateConfigPaths)
	cloned.LastCollectionAt = cloneTimePtr(status.LastCollectionAt)
	cloned.LastSuccessAt = cloneTimePtr(status.LastSuccessAt)
	cloned.LastErrorAt = cloneTimePtr(status.LastErrorAt)
	return &cloned
}

func cloneStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}

func cloneTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func parseUserTrafficStatName(name string) (int64, int64, string, bool) {
	matches := userTrafficStatPattern.FindStringSubmatch(strings.TrimSpace(name))
	if len(matches) != 4 {
		return 0, 0, "", false
	}

	userID, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0, 0, "", false
	}
	proxyID, err := strconv.ParseInt(matches[2], 10, 64)
	if err != nil {
		return 0, 0, "", false
	}

	return userID, proxyID, matches[3], true
}

func parseXrayStatValue(raw json.RawMessage) (int64, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return 0, nil
	}

	if strings.HasPrefix(trimmed, "\"") {
		var stringValue string
		if err := json.Unmarshal(raw, &stringValue); err != nil {
			return 0, err
		}
		if strings.TrimSpace(stringValue) == "" {
			return 0, nil
		}
		return strconv.ParseInt(stringValue, 10, 64)
	}

	return strconv.ParseInt(trimmed, 10, 64)
}
