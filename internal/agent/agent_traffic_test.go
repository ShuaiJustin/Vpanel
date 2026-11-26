package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	"v/internal/logger"
)

type stubHeartbeatTrafficReporter struct {
	snapshot     *TrafficSnapshot
	records      []TrafficRecord
	committed    map[string]int64
	prepareCalls int
	commitCalls  int
}

func (s *stubHeartbeatTrafficReporter) PrepareDelta(ctx context.Context) (*TrafficSnapshot, []TrafficRecord, error) {
	s.prepareCalls++
	return s.snapshot, cloneTrafficRecords(s.records), nil
}

func (s *stubHeartbeatTrafficReporter) Commit(snapshot *TrafficSnapshot) {
	s.commitCalls++
	if snapshot != nil {
		s.committed = cloneCommittedCounters(snapshot.counters)
	}
}

func (s *stubHeartbeatTrafficReporter) ExportCommittedCounters() map[string]int64 {
	return cloneCommittedCounters(s.committed)
}

func (s *stubHeartbeatTrafficReporter) RestoreCommittedCounters(counters map[string]int64) {
	s.committed = cloneCommittedCounters(counters)
}

func TestAgentSendHeartbeatReusesPendingTrafficBatchUntilAcknowledged(t *testing.T) {
	const nodeID = int64(42)
	proxyID := int64(11)

	reporter := &stubHeartbeatTrafficReporter{
		snapshot: &TrafficSnapshot{
			counters: map[string]int64{
				"user>>>user-7-proxy-11>>>traffic>>>uplink":   100,
				"user>>>user-7-proxy-11>>>traffic>>>downlink": 200,
			},
		},
		records: []TrafficRecord{
			{UserID: 7, ProxyID: &proxyID, Upload: 100, Download: 200},
		},
	}

	var (
		mu                sync.Mutex
		registerCalls     int
		heartbeatRequests []HeartbeatRequest
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/node/register":
			mu.Lock()
			registerCalls++
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(RegisterResponse{
				Success: true,
				NodeID:  nodeID,
				Message: "ok",
			})
		case "/api/node/heartbeat":
			var req HeartbeatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed to decode heartbeat request: %v", err)
			}

			mu.Lock()
			heartbeatRequests = append(heartbeatRequests, req)
			callIndex := len(heartbeatRequests)
			mu.Unlock()

			w.Header().Set("Content-Type", "application/json")
			if callIndex == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(HeartbeatResponse{
					Success: false,
					Message: "temporary failure",
				})
				return
			}

			_ = json.NewEncoder(w).Encode(HeartbeatResponse{
				Success: true,
				Message: "ok",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	log := logger.NewNopLogger()
	cfg := &Config{
		Node: NodeConfig{
			Token: "node-token",
		},
		Panel: PanelConfig{
			URL:               server.URL,
			ConnectTimeout:    time.Second,
			ReconnectInterval: time.Millisecond,
			MaxReconnectDelay: time.Millisecond,
		},
	}

	agent := &Agent{
		config: cfg,
		logger: log,
		panelClient: NewPanelClient(PanelClientConfig{
			URL:               cfg.Panel.URL,
			TLSSkipVerify:     cfg.Panel.TLSSkipVerify,
			ConnectTimeout:    cfg.Panel.ConnectTimeout,
			ReconnectInterval: cfg.Panel.ReconnectInterval,
			MaxReconnectDelay: cfg.Panel.MaxReconnectDelay,
			Token:             cfg.Node.Token,
		}, log),
		metricsCollector: NewMetricsCollector(log),
		xrayManager:      &XrayManager{},
		trafficReporter:  reporter,
		registered:       true,
		nodeID:           nodeID,
		ctx:              context.Background(),
	}

	agent.sendHeartbeat()
	agent.sendHeartbeat()
	agent.sendHeartbeat()

	if reporter.prepareCalls != 1 {
		t.Fatalf("expected traffic delta to be prepared once, got %d", reporter.prepareCalls)
	}
	if reporter.commitCalls != 1 {
		t.Fatalf("expected traffic snapshot to commit once, got %d", reporter.commitCalls)
	}
	if pending := agent.loadPendingTrafficBatch(); pending != nil {
		t.Fatalf("expected pending traffic batch to be cleared after success, got %+v", pending)
	}

	mu.Lock()
	defer mu.Unlock()

	if registerCalls != 1 {
		t.Fatalf("expected one re-registration after failed heartbeat, got %d", registerCalls)
	}
	if len(heartbeatRequests) != 2 {
		t.Fatalf("expected two heartbeat attempts, got %d", len(heartbeatRequests))
	}
	if heartbeatRequests[0].TrafficBatchID == "" {
		t.Fatal("expected heartbeat traffic batch id to be present")
	}
	if heartbeatRequests[0].TrafficBatchID != heartbeatRequests[1].TrafficBatchID {
		t.Fatalf("expected retry heartbeat to reuse batch id, got %q and %q", heartbeatRequests[0].TrafficBatchID, heartbeatRequests[1].TrafficBatchID)
	}
	if !reflect.DeepEqual(heartbeatRequests[0].Traffic, heartbeatRequests[1].Traffic) {
		t.Fatalf("expected retry heartbeat to resend identical traffic batch, got %+v and %+v", heartbeatRequests[0].Traffic, heartbeatRequests[1].Traffic)
	}
}

func TestAgentPersistsTrafficStateAcrossRestart(t *testing.T) {
	tempDir := t.TempDir()
	proxyID := int64(17)

	reporter := &stubHeartbeatTrafficReporter{
		committed: map[string]int64{
			"user>>>user-7-proxy-17>>>traffic>>>uplink": 111,
		},
	}

	agent := &Agent{
		config: &Config{
			Node: NodeConfig{Token: "node-token"},
			Panel: PanelConfig{URL: "https://panel.example.com"},
			Xray: XrayConfig{BackupDir: tempDir, ConfigPath: tempDir + "/xray.json"},
		},
		logger:         logger.NewNopLogger(),
		trafficReporter: reporter,
		pendingTraffic: &pendingTrafficBatch{
			batchID: "batch-1",
			snapshot: &TrafficSnapshot{counters: map[string]int64{
				"user>>>user-7-proxy-17>>>traffic>>>uplink": 222,
			}},
			records: []TrafficRecord{{UserID: 7, ProxyID: &proxyID, Upload: 50, Download: 10}},
		},
	}

	agent.persistTrafficState()

	restoredReporter := &stubHeartbeatTrafficReporter{}
	restored := &Agent{
		config: &Config{
			Node: NodeConfig{Token: "node-token"},
			Panel: PanelConfig{URL: "https://panel.example.com"},
			Xray: XrayConfig{BackupDir: tempDir, ConfigPath: tempDir + "/xray.json"},
		},
		logger:         logger.NewNopLogger(),
		trafficReporter: restoredReporter,
	}

	restored.restoreTrafficState()

	if restored.pendingTraffic == nil {
		t.Fatal("expected pending traffic batch to be restored")
	}
	if restored.pendingTraffic.batchID != "batch-1" {
		t.Fatalf("expected restored batch id batch-1, got %q", restored.pendingTraffic.batchID)
	}
	if len(restored.pendingTraffic.records) != 1 || restored.pendingTraffic.records[0].Upload != 50 {
		t.Fatalf("expected restored records to match persisted state, got %+v", restored.pendingTraffic.records)
	}
	if restoredReporter.committed["user>>>user-7-proxy-17>>>traffic>>>uplink"] != 111 {
		t.Fatalf("expected committed counters to be restored, got %+v", restoredReporter.committed)
	}

	restored.acknowledgeHeartbeatTraffic("batch-1", restored.pendingTraffic.snapshot)
	if restored.pendingTraffic != nil {
		t.Fatal("expected pending traffic batch to be cleared after acknowledgement")
	}

	finalReporter := &stubHeartbeatTrafficReporter{}
	finalAgent := &Agent{
		config: &Config{
			Node: NodeConfig{Token: "node-token"},
			Panel: PanelConfig{URL: "https://panel.example.com"},
			Xray: XrayConfig{BackupDir: tempDir, ConfigPath: tempDir + "/xray.json"},
		},
		logger:         logger.NewNopLogger(),
		trafficReporter: finalReporter,
	}
	finalAgent.restoreTrafficState()
	if finalAgent.pendingTraffic != nil {
		t.Fatalf("expected no pending traffic after acknowledgement, got %+v", finalAgent.pendingTraffic)
	}
	if finalReporter.committed["user>>>user-7-proxy-17>>>traffic>>>uplink"] != 222 {
		t.Fatalf("expected committed counters to advance to acknowledged snapshot, got %+v", finalReporter.committed)
	}
}
