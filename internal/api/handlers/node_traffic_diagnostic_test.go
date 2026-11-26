package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"v/internal/agent"
	"v/internal/database/repository"
	"v/internal/logger"
	"v/internal/node"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newNodeTrafficDiagnosticHandler(
	t *testing.T,
	httpClient *http.Client,
	address string,
	port int,
) *NodeHandler {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&repository.Node{}); err != nil {
		t.Fatalf("migrate nodes: %v", err)
	}

	nodeRepo := repository.NewNodeRepository(db)
	if err := db.Create(&repository.Node{
		ID:      1,
		Name:    "node-1",
		Address: address,
		Port:    port,
		Token:   "node-token",
		Status:  repository.NodeStatusOnline,
	}).Error; err != nil {
		t.Fatalf("seed node: %v", err)
	}

	handler := NewNodeHandler(
		node.NewService(nodeRepo, nil, nil, logger.NewNopLogger()),
		nil,
		nil,
		nil,
		logger.NewNopLogger(),
	)
	handler.httpClient = httpClient
	return handler
}

func parseServerHostPort(t *testing.T, rawURL string) (string, int) {
	t.Helper()

	parsed, err := neturl.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	host := parsed.Hostname()
	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	return host, port
}

func TestNodeHandlerGetTrafficDiagnosticClassifiesCollectorStates(t *testing.T) {
	tests := []struct {
		name                string
		client              *http.Client
		serverHandler       http.HandlerFunc
		wantDiagnostic      string
		wantMessageContains string
	}{
		{
			name: "agent unreachable",
			client: &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return nil, errors.New("dial tcp timeout")
				}),
			},
			wantDiagnostic:      "agent_unreachable",
			wantMessageContains: "无法连接到节点 Agent",
		},
		{
			name: "agent missing traffic endpoint",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				http.NotFound(w, r)
			},
			wantDiagnostic:      "agent_no_traffic_endpoint",
			wantMessageContains: "/traffic/status",
		},
		{
			name: "xray not running",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(agent.TrafficCollectorStatus{
					Status:      agent.TrafficCollectorStatusHealthyIdle,
					XrayRunning: false,
				})
			},
			wantDiagnostic:      "xray_not_running",
			wantMessageContains: "Xray 当前未运行",
		},
		{
			name: "collector error",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(agent.TrafficCollectorStatus{
					Status:      agent.TrafficCollectorStatusCollectorError,
					XrayRunning: true,
					LastError:   "query xray stats failed",
				})
			},
			wantDiagnostic:      agent.TrafficCollectorStatusCollectorError,
			wantMessageContains: "query xray stats failed",
		},
		{
			name: "healthy collecting with config mismatch guidance",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(agent.TrafficCollectorStatus{
					Status:               agent.TrafficCollectorStatusHealthyCollecting,
					ConfiguredConfigPath: "/etc/xray/config.json",
					ResolvedConfigPath:   "/usr/local/etc/xray/config.json",
					CandidateConfigPaths: []string{"/etc/xray/config.json", "/usr/local/etc/xray/config.json"},
					APIPort:              62001,
					XrayRunning:          true,
					LastRecordCount:      4,
				})
			},
			wantDiagnostic:      agent.TrafficCollectorStatusHealthyCollecting,
			wantMessageContains: "路径不一致",
		},
		{
			name: "healthy idle",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(agent.TrafficCollectorStatus{
					Status:               agent.TrafficCollectorStatusHealthyIdle,
					ConfiguredConfigPath: "/usr/local/etc/xray/config.json",
					ResolvedConfigPath:   "/usr/local/etc/xray/config.json",
					XrayRunning:          true,
					LastRecordCount:      0,
				})
			},
			wantDiagnostic:      agent.TrafficCollectorStatusHealthyIdle,
			wantMessageContains: "没有新的用户流量记录",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.client
			address := "127.0.0.1"
			port := 18443
			var server *httptest.Server

			if tt.serverHandler != nil {
				server = httptest.NewServer(tt.serverHandler)
				defer server.Close()
				address, port = parseServerHostPort(t, server.URL)
				if client == nil {
					client = server.Client()
				}
			}

			handler := newNodeTrafficDiagnosticHandler(t, client, address, port)
			router := gin.New()
			router.GET("/api/admin/nodes/:id/traffic-diagnostic", handler.GetTrafficDiagnostic)

			req := httptest.NewRequest(http.MethodGet, "/api/admin/nodes/1/traffic-diagnostic", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
			}

			var response TrafficDiagnosticResponse
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if response.DiagnosticStatus != tt.wantDiagnostic {
				t.Fatalf("expected diagnostic %q, got %q", tt.wantDiagnostic, response.DiagnosticStatus)
			}
			if tt.wantMessageContains != "" && !strings.Contains(response.Message, tt.wantMessageContains) {
				t.Fatalf("expected message %q to contain %q", response.Message, tt.wantMessageContains)
			}
		})
	}
}
