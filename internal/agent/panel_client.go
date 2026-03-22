// Package agent provides the Node Agent functionality for V Panel.
package agent

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"v/internal/logger"
)

// PanelClientConfig holds configuration for the Panel client.
type PanelClientConfig struct {
	URL               string
	Token             string
	TLSSkipVerify     bool
	ConnectTimeout    time.Duration
	ReconnectInterval time.Duration
	MaxReconnectDelay time.Duration
}

// PanelClient handles communication with the Panel Server.
type PanelClient struct {
	mu         sync.RWMutex
	config     PanelClientConfig
	logger     logger.Logger
	httpClient *http.Client

	// Reconnection state
	reconnectDelay   time.Duration
	lastConnected    time.Time
	consecutiveFails int
}

// NewPanelClient creates a new Panel client.
func NewPanelClient(cfg PanelClientConfig, log logger.Logger) *PanelClient {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.TLSSkipVerify,
		},
	}

	httpClient := &http.Client{
		Timeout:   cfg.ConnectTimeout,
		Transport: transport,
	}

	return &PanelClient{
		config:         cfg,
		logger:         log,
		httpClient:     httpClient,
		reconnectDelay: cfg.ReconnectInterval,
	}
}

// Register registers the agent with the Panel Server.
func (c *PanelClient) Register(ctx context.Context, req *RegisterRequest) (*RegisterResponse, error) {
	url := fmt.Sprintf("%s/api/node/register", c.config.URL)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Node-Token", c.config.Token)

	resp, err := c.doRequest(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("registration failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result RegisterResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Handle successful connection
	c.handleConnectionSuccess()

	return &result, nil
}

// Heartbeat sends a heartbeat to the Panel Server.
func (c *PanelClient) Heartbeat(ctx context.Context, req *HeartbeatRequest) (*HeartbeatResponse, error) {
	url := fmt.Sprintf("%s/api/node/heartbeat", c.config.URL)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Node-Token", c.config.Token)

	resp, err := c.doRequest(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("heartbeat failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result HeartbeatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Handle successful connection
	c.handleConnectionSuccess()

	return &result, nil
}

// ReportCommandResult reports the result of a command execution to the Panel.
func (c *PanelClient) ReportCommandResult(ctx context.Context, result *CommandResult) error {
	url := fmt.Sprintf("%s/api/node/command/result", c.config.URL)

	body, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Node-Token", c.config.Token)

	resp, err := c.doRequest(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("report failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// SyncConfig fetches the latest configuration from the Panel.
func (c *PanelClient) SyncConfig(ctx context.Context, nodeID int64) (json.RawMessage, error) {
	url := fmt.Sprintf("%s/api/node/%d/config", c.config.URL, nodeID)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("X-Node-Token", c.config.Token)

	resp, err := c.doRequest(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("config sync failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse the response to extract the config field
	var response struct {
		Success bool   `json:"success"`
		Config  string `json:"config"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("config sync failed: %s", response.Message)
	}

	return json.RawMessage(response.Config), nil
}

// doRequest performs an HTTP request with retry logic.
func (c *PanelClient) doRequest(req *http.Request) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.handleConnectionError()
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// handleConnectionError handles connection errors and updates reconnect delay.
func (c *PanelClient) handleConnectionError() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.consecutiveFails++

	// Exponential backoff with jitter
	c.reconnectDelay = c.reconnectDelay * 2
	if c.reconnectDelay > c.config.MaxReconnectDelay {
		c.reconnectDelay = c.config.MaxReconnectDelay
	}

	c.logger.Warn("connection error, will retry",
		logger.F("consecutive_fails", c.consecutiveFails),
		logger.F("next_retry_delay", c.reconnectDelay.String()))
}

// handleConnectionSuccess handles successful connections.
func (c *PanelClient) handleConnectionSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.consecutiveFails = 0
	c.reconnectDelay = c.config.ReconnectInterval
	c.lastConnected = time.Now()
}

// GetReconnectDelay returns the current reconnect delay.
func (c *PanelClient) GetReconnectDelay() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.reconnectDelay
}

// IsConnected returns whether the client has recently connected successfully.
func (c *PanelClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// Consider connected if last successful connection was within 2 heartbeat intervals
	return time.Since(c.lastConnected) < 2*time.Minute
}

// ResetReconnectDelay resets the reconnect delay to the initial value.
func (c *PanelClient) ResetReconnectDelay() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reconnectDelay = c.config.ReconnectInterval
}

// GetConsecutiveFails returns the number of consecutive connection failures.
func (c *PanelClient) GetConsecutiveFails() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.consecutiveFails
}

// GetLastConnectedTime returns the time of the last successful connection.
func (c *PanelClient) GetLastConnectedTime() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastConnected
}

// ShouldReconnect returns whether the client should attempt to reconnect.
func (c *PanelClient) ShouldReconnect() bool {
	// Agent-side reconnects must be persistent. A temporary panel outage should
	// not permanently stop heartbeats or traffic reporting.
	return true
}

// WaitForReconnect waits for the reconnect delay before returning.
func (c *PanelClient) WaitForReconnect(ctx context.Context) error {
	c.mu.RLock()
	delay := c.reconnectDelay
	c.mu.RUnlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		return nil
	}
}
