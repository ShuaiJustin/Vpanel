package handlers

import (
	"context"
	"testing"
	"time"

	"v/internal/logger"
)

type mockNodeXrayStatusUpdater struct {
	calls []mockNodeXrayStatusCall
}

type mockNodeXrayStatusCall struct {
	nodeID      int64
	xrayRunning bool
	xrayVersion string
}

func (m *mockNodeXrayStatusUpdater) UpdateXrayStatus(_ context.Context, id int64, xrayRunning bool, xrayVersion string) error {
	m.calls = append(m.calls, mockNodeXrayStatusCall{
		nodeID:      id,
		xrayRunning: xrayRunning,
		xrayVersion: xrayVersion,
	})
	return nil
}

func TestNodeRecoveryTracker_QueueXrayRestartCommand(t *testing.T) {
	tracker := NewNodeRecoveryTracker(logger.NewDefault())

	cmd, queued := tracker.QueueXrayRestartCommand(3, "admin", "manual restart")
	if !queued {
		t.Fatal("expected restart command to be queued")
	}
	if cmd.Type != commandTypeXrayRestart {
		t.Fatalf("expected command type %q, got %q", commandTypeXrayRestart, cmd.Type)
	}
	if cmd.ID == "" {
		t.Fatal("expected queued command to have an id")
	}

	if _, queuedAgain := tracker.QueueXrayRestartCommand(3, "admin", "manual restart"); queuedAgain {
		t.Fatal("expected duplicate restart command to be rejected while pending")
	}
}

func TestNodeRecoveryTracker_ExpiresStalePendingCommand(t *testing.T) {
	tracker := NewNodeRecoveryTracker(logger.NewNopLogger())

	first, queued := tracker.QueueConfigSyncCommandDetailed(3, "admin", "manual sync")
	if !queued {
		t.Fatal("expected first config sync command to be queued")
	}
	if _, queuedAgain := tracker.QueueConfigSyncCommandDetailed(3, "admin", "manual sync"); queuedAgain {
		t.Fatal("expected duplicate config sync command to be rejected while pending")
	}

	tracker.mu.Lock()
	tracker.recentEvents[3][0].CreatedAt = time.Now().Add(-nodeCommandPendingTTL - time.Second).Format(time.RFC3339)
	tracker.lastQueuedCommands[3][commandTypeConfigSync] = time.Now().Add(-nodeCommandPendingTTL - time.Second)
	tracker.mu.Unlock()

	second, queuedAfterExpiry := tracker.QueueConfigSyncCommandDetailed(3, "admin", "manual sync")
	if !queuedAfterExpiry {
		t.Fatal("expected stale pending config sync command to expire")
	}
	if second.ID == first.ID {
		t.Fatal("expected replacement command to get a new id")
	}

	events := tracker.GetRecentRecoveryEvents(3)
	foundExpired := false
	for _, event := range events {
		if event.CommandID == first.ID && event.Status == "expired" {
			foundExpired = true
			break
		}
	}
	if !foundExpired {
		t.Fatalf("expected first command to be marked expired, got %#v", events)
	}
}

func TestNodeRecoveryTracker_ExpiresStaleInflightCommand(t *testing.T) {
	tracker := NewNodeRecoveryTracker(logger.NewNopLogger())

	first, queued := tracker.QueueXrayRestartCommand(3, "admin", "manual restart")
	if !queued {
		t.Fatal("expected first restart command to be queued")
	}
	pending := tracker.GetPendingCommands(3)
	if len(pending) != 1 || pending[0].ID != first.ID {
		t.Fatalf("expected first command to be dispatched, got %#v", pending)
	}
	if _, queuedAgain := tracker.QueueXrayRestartCommand(3, "admin", "manual restart"); queuedAgain {
		t.Fatal("expected duplicate restart command to be rejected while inflight")
	}

	tracker.mu.Lock()
	tracker.recentEvents[3][0].CreatedAt = time.Now().Add(-nodeCommandInflightTTL - time.Second).Format(time.RFC3339)
	tracker.lastQueuedCommands[3][commandTypeXrayRestart] = time.Now().Add(-nodeCommandInflightTTL - time.Second)
	tracker.mu.Unlock()

	if _, queuedAfterExpiry := tracker.QueueXrayRestartCommand(3, "admin", "manual restart"); !queuedAfterExpiry {
		t.Fatal("expected stale inflight restart command to expire")
	}
}

func TestUpdateNodeXrayStatusFromCommandResult(t *testing.T) {
	updater := &mockNodeXrayStatusUpdater{}

	err := updateNodeXrayStatusFromCommandResult(context.Background(), updater, 7, commandTypeXrayRestart, true, map[string]any{
		"running": true,
		"version": "Xray 26.2.4",
	})
	if err != nil {
		t.Fatalf("unexpected error updating xray status from restart result: %v", err)
	}

	if len(updater.calls) != 1 {
		t.Fatalf("expected 1 status update call, got %d", len(updater.calls))
	}
	if !updater.calls[0].xrayRunning || updater.calls[0].xrayVersion != "Xray 26.2.4" {
		t.Fatalf("unexpected restart status payload: %+v", updater.calls[0])
	}

	err = updateNodeXrayStatusFromCommandResult(context.Background(), updater, 7, "xray_stop", true, nil)
	if err != nil {
		t.Fatalf("unexpected error updating xray status from stop result: %v", err)
	}

	if len(updater.calls) != 2 {
		t.Fatalf("expected 2 status update calls, got %d", len(updater.calls))
	}
	if updater.calls[1].xrayRunning || updater.calls[1].xrayVersion != "" {
		t.Fatalf("unexpected stop status payload: %+v", updater.calls[1])
	}
}
