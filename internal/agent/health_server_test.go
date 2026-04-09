package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"v/internal/logger"
)

type stubHealthAgentStatusProvider struct {
	running       bool
	registered    bool
	nodeID        int64
	xrayStatus    *XrayStatus
	trafficStatus *TrafficCollectorStatus
	metrics       *NodeMetrics
}

func (s *stubHealthAgentStatusProvider) IsRunning() bool {
	return s.running
}

func (s *stubHealthAgentStatusProvider) IsRegistered() bool {
	return s.registered
}

func (s *stubHealthAgentStatusProvider) GetNodeID() int64 {
	return s.nodeID
}

func (s *stubHealthAgentStatusProvider) GetXrayStatus() *XrayStatus {
	return s.xrayStatus
}

func (s *stubHealthAgentStatusProvider) GetTrafficCollectorStatus() *TrafficCollectorStatus {
	return s.trafficStatus
}

func (s *stubHealthAgentStatusProvider) GetMetrics() *NodeMetrics {
	return s.metrics
}

func TestHealthServerHandleHealthIncludesTrafficCollectorStatus(t *testing.T) {
	now := time.Now().UTC()
	server := NewHealthServer(HealthServerConfig{}, &stubHealthAgentStatusProvider{
		running:    true,
		registered: true,
		nodeID:     9,
		xrayStatus: &XrayStatus{Running: true, Version: "1.8.0"},
		trafficStatus: &TrafficCollectorStatus{
			Status:           TrafficCollectorStatusHealthyIdle,
			LastCollectionAt: &now,
			LastSuccessAt:    &now,
			LastRecordCount:  0,
			XrayRunning:      true,
		},
		metrics: &NodeMetrics{},
	}, logger.NewNopLogger())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	server.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var response HealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode health response: %v", err)
	}

	if response.Traffic == nil {
		t.Fatal("expected traffic status in health response")
	}
	if response.Traffic.Status != TrafficCollectorStatusHealthyIdle {
		t.Fatalf("expected healthy idle traffic status, got %q", response.Traffic.Status)
	}

	foundTrafficCheck := false
	for _, check := range response.Checks {
		if check.Name != "traffic_collector" {
			continue
		}
		foundTrafficCheck = true
		if check.Status != "pass" {
			t.Fatalf("expected pass status for idle traffic collector, got %q", check.Status)
		}
	}
	if !foundTrafficCheck {
		t.Fatal("expected traffic_collector health check")
	}
}

func TestHealthServerHandleTrafficStatusReturnsCollectorPayload(t *testing.T) {
	now := time.Now().UTC()
	server := NewHealthServer(HealthServerConfig{}, &stubHealthAgentStatusProvider{
		running:    true,
		registered: true,
		nodeID:     3,
		xrayStatus: &XrayStatus{Running: true},
		trafficStatus: &TrafficCollectorStatus{
			Status:               TrafficCollectorStatusHealthyCollecting,
			ConfiguredConfigPath: "/etc/xray/config.json",
			ResolvedConfigPath:   "/usr/local/etc/xray/config.json",
			CandidateConfigPaths: []string{"/etc/xray/config.json", "/usr/local/etc/xray/config.json"},
			APIPort:              62001,
			LastCollectionAt:     &now,
			LastSuccessAt:        &now,
			LastRecordCount:      5,
			XrayRunning:          true,
		},
	}, logger.NewNopLogger())

	req := httptest.NewRequest(http.MethodGet, "/traffic/status", nil)
	w := httptest.NewRecorder()

	server.handleTrafficStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var response TrafficCollectorStatus
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode traffic status response: %v", err)
	}

	if response.Status != TrafficCollectorStatusHealthyCollecting {
		t.Fatalf("expected healthy collecting status, got %q", response.Status)
	}
	if response.APIPort != 62001 {
		t.Fatalf("expected api port 62001, got %d", response.APIPort)
	}
	if len(response.CandidateConfigPaths) != 2 {
		t.Fatalf("expected candidate config paths, got %+v", response.CandidateConfigPaths)
	}
}
