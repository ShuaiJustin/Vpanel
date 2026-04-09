// Package agent provides the Node Agent functionality for V Panel.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"v/internal/logger"
)

// HealthServerConfig holds configuration for the health server.
type HealthServerConfig struct {
	Host string
	Port int
}

// HealthResponse represents the health check response.
type HealthResponse struct {
	Status    string                  `json:"status"`
	Timestamp time.Time               `json:"timestamp"`
	Xray      *XrayStatus             `json:"xray,omitempty"`
	Traffic   *TrafficCollectorStatus `json:"traffic,omitempty"`
	Metrics   *NodeMetrics            `json:"metrics,omitempty"`
	Agent     *AgentInfo              `json:"agent,omitempty"`
	Checks    []HealthCheck           `json:"checks,omitempty"`
}

// AgentInfo represents agent information.
type AgentInfo struct {
	Version    string `json:"version"`
	Registered bool   `json:"registered"`
	NodeID     int64  `json:"node_id,omitempty"`
	Uptime     string `json:"uptime"`
}

// HealthCheck represents a single health check result.
type HealthCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// AgentStatusProvider provides agent status information.
type AgentStatusProvider interface {
	IsRunning() bool
	IsRegistered() bool
	GetNodeID() int64
	GetXrayStatus() *XrayStatus
	GetTrafficCollectorStatus() *TrafficCollectorStatus
	GetMetrics() *NodeMetrics
}

// HealthServer provides health check endpoints for the agent.
type HealthServer struct {
	mu      sync.RWMutex
	config  HealthServerConfig
	logger  logger.Logger
	server  *http.Server
	agent   AgentStatusProvider
	running bool
}

// NewHealthServer creates a new health server.
func NewHealthServer(cfg HealthServerConfig, agent AgentStatusProvider, log logger.Logger) *HealthServer {
	return &HealthServer{
		config: cfg,
		logger: log,
		agent:  agent,
	}
}

// Start starts the health server.
func (s *HealthServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("health server is already running")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/health/live", s.handleLiveness)
	mux.HandleFunc("/health/ready", s.handleReadiness)
	mux.HandleFunc("/xray/status", s.handleXrayStatus)
	mux.HandleFunc("/traffic/status", s.handleTrafficStatus)
	mux.HandleFunc("/metrics", s.handleMetrics)

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	s.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		s.logger.Info("health server starting", logger.F("address", addr))
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("health server error", logger.F("error", err))
		}
	}()

	s.running = true
	return nil
}

// Stop stops the health server.
func (s *HealthServer) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown health server: %w", err)
	}

	s.running = false
	s.logger.Info("health server stopped")
	return nil
}

// handleHealth handles the /health endpoint.
func (s *HealthServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := &HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Checks:    []HealthCheck{},
	}

	// Check Xray status
	if s.agent != nil {
		xrayStatus := s.agent.GetXrayStatus()
		response.Xray = xrayStatus

		// Add Xray health check
		xrayCheck := HealthCheck{
			Name:   "xray",
			Status: "pass",
		}
		if !xrayStatus.Running {
			xrayCheck.Status = "fail"
			xrayCheck.Message = "Xray is not running"
			response.Status = "degraded"
		}
		response.Checks = append(response.Checks, xrayCheck)

		// Add registration check
		regCheck := HealthCheck{
			Name:   "registration",
			Status: "pass",
		}
		if !s.agent.IsRegistered() {
			regCheck.Status = "warn"
			regCheck.Message = "Agent is not registered with Panel"
		}
		response.Checks = append(response.Checks, regCheck)

		trafficStatus := s.agent.GetTrafficCollectorStatus()
		response.Traffic = trafficStatus

		trafficCheck := HealthCheck{
			Name:   "traffic_collector",
			Status: "pass",
		}
		if trafficStatus == nil {
			trafficCheck.Status = "warn"
			trafficCheck.Message = "Traffic collector status unavailable"
			if response.Status == "healthy" {
				response.Status = "degraded"
			}
		} else {
			switch trafficStatus.Status {
			case TrafficCollectorStatusCollectorError:
				trafficCheck.Status = "warn"
				if trafficStatus.LastError != "" {
					trafficCheck.Message = trafficStatus.LastError
				} else {
					trafficCheck.Message = "Traffic collector reported an error"
				}
				if response.Status == "healthy" {
					response.Status = "degraded"
				}
			case TrafficCollectorStatusHealthyCollecting:
				trafficCheck.Message = fmt.Sprintf("Collected %d traffic records during the last successful pass", trafficStatus.LastRecordCount)
			case TrafficCollectorStatusHealthyIdle:
				trafficCheck.Message = "Traffic collector is healthy but has no new traffic records"
			default:
				trafficCheck.Status = "warn"
				trafficCheck.Message = "Traffic collector status is unknown"
				if response.Status == "healthy" {
					response.Status = "degraded"
				}
			}
		}
		response.Checks = append(response.Checks, trafficCheck)

		// Add agent info
		response.Agent = &AgentInfo{
			Version:    "1.0.0",
			Registered: s.agent.IsRegistered(),
			NodeID:     s.agent.GetNodeID(),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleTrafficStatus handles the /traffic/status endpoint.
func (s *HealthServer) handleTrafficStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.agent == nil {
		http.Error(w, "Agent not available", http.StatusServiceUnavailable)
		return
	}

	status := s.agent.GetTrafficCollectorStatus()
	if status == nil {
		status = &TrafficCollectorStatus{Status: TrafficCollectorStatusUnknown}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleXrayStatus handles the /xray/status endpoint.
func (s *HealthServer) handleXrayStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.agent == nil {
		http.Error(w, "Agent not available", http.StatusServiceUnavailable)
		return
	}

	status := s.agent.GetXrayStatus()
	if !status.Running {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleMetrics handles the /metrics endpoint.
func (s *HealthServer) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.agent == nil {
		http.Error(w, "Agent not available", http.StatusServiceUnavailable)
		return
	}

	metrics := s.agent.GetMetrics()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

// handleLiveness handles the /health/live endpoint (Kubernetes liveness probe).
func (s *HealthServer) handleLiveness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Liveness check - is the agent process alive?
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"status":    "alive",
		"timestamp": time.Now(),
	})
}

// handleReadiness handles the /health/ready endpoint (Kubernetes readiness probe).
func (s *HealthServer) handleReadiness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ready := true
	reasons := []string{}

	if s.agent != nil {
		// Check if agent is running
		if !s.agent.IsRunning() {
			ready = false
			reasons = append(reasons, "agent not running")
		}

		// Check if registered with Panel
		if !s.agent.IsRegistered() {
			ready = false
			reasons = append(reasons, "not registered with panel")
		}

		// Check if Xray is running
		xrayStatus := s.agent.GetXrayStatus()
		if !xrayStatus.Running {
			ready = false
			reasons = append(reasons, "xray not running")
		}
	} else {
		ready = false
		reasons = append(reasons, "agent not initialized")
	}

	response := map[string]any{
		"ready":     ready,
		"timestamp": time.Now(),
	}

	if !ready {
		response["reasons"] = reasons
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(response)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// IsRunning returns whether the health server is running.
func (s *HealthServer) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}
