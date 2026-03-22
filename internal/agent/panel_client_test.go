package agent

import (
	"testing"
	"time"

	"v/internal/logger"
)

func TestPanelClientShouldReconnectAfterRepeatedFailures(t *testing.T) {
	client := NewPanelClient(PanelClientConfig{
		URL:               "http://127.0.0.1:8080",
		Token:             "test-token",
		ConnectTimeout:    time.Second,
		ReconnectInterval: time.Second,
		MaxReconnectDelay: 30 * time.Second,
	}, logger.New(logger.Config{
		Level:  "error",
		Format: "json",
		Output: "stdout",
	}))

	for i := 0; i < 20; i++ {
		client.handleConnectionError()
	}

	if !client.ShouldReconnect() {
		t.Fatal("expected client to keep reconnecting after repeated failures")
	}
}

func TestPanelClientReconnectDelayRemainsCapped(t *testing.T) {
	client := NewPanelClient(PanelClientConfig{
		URL:               "http://127.0.0.1:8080",
		Token:             "test-token",
		ConnectTimeout:    time.Second,
		ReconnectInterval: time.Second,
		MaxReconnectDelay: 8 * time.Second,
	}, logger.New(logger.Config{
		Level:  "error",
		Format: "json",
		Output: "stdout",
	}))

	for i := 0; i < 10; i++ {
		client.handleConnectionError()
	}

	if got := client.GetReconnectDelay(); got != 8*time.Second {
		t.Fatalf("expected reconnect delay to cap at 8s, got %s", got)
	}
}
