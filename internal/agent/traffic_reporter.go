// Package agent provides the Node Agent functionality for V Panel.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"v/internal/logger"
)

var userTrafficStatPattern = regexp.MustCompile(`^user>>>user-(\d+)-proxy-(\d+)>>>traffic>>>(uplink|downlink)$`)

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

type trafficReporter struct {
	binaryPath string
	configPath string
	logger     logger.Logger

	mu            sync.Mutex
	lastCommitted map[string]int64
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
	return &trafficReporter{
		binaryPath:    cfg.BinaryPath,
		configPath:    cfg.ConfigPath,
		logger:        log,
		lastCommitted: make(map[string]int64),
	}
}

func (r *trafficReporter) PrepareDelta(ctx context.Context) (*TrafficSnapshot, []TrafficRecord, error) {
	currentCounters, err := r.queryCounters(ctx)
	if err != nil {
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

func (r *trafficReporter) queryCounters(ctx context.Context) (map[string]int64, error) {
	apiPort, err := r.resolveAPIPort()
	if err != nil {
		return nil, err
	}

	command := exec.CommandContext(
		ctx,
		r.binaryPath,
		"api",
		"statsquery",
		fmt.Sprintf("--server=127.0.0.1:%d", apiPort),
	)
	output, err := command.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if trimmed == "" {
			trimmed = err.Error()
		}
		return nil, fmt.Errorf("query xray stats failed: %s", trimmed)
	}

	var response xrayStatsQueryResponse
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, fmt.Errorf("parse xray stats failed: %w", err)
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

	return counters, nil
}

func (r *trafficReporter) resolveAPIPort() (int, error) {
	data, err := os.ReadFile(r.configPath)
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

	return 0, fmt.Errorf("xray api inbound not found in %s", r.configPath)
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
