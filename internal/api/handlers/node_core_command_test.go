package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"v/internal/database/repository"
	"v/internal/logger"
)

func TestOperatorCommandsSurvivePanelRestart(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&repository.Node{}, &repository.NodeOperation{}))
	require.NoError(t, db.Create(&repository.Node{ID: 3, Name: "test", Address: "localhost", Token: "test"}).Error)
	start := func() *NodeRecoveryTracker { return NewNodeRecoveryTracker(logger.NewNopLogger()).WithStorage(db) }
	tracker := start()
	cmd, ok := tracker.QueueAgentUpdateCommand(3, "admin", "update", "test-release")
	require.True(t, ok)
	tracker = start()
	require.Equal(t, "queued", tracker.GetRecentRecoveryEvents(3)[0].Status)
	tracker.ReportAgentVersion(3, "old", "old-process")
	require.Equal(t, []Command{cmd}, tracker.GetPendingCommands(3))
	tracker = start()
	require.Equal(t, "dispatched", tracker.GetRecentRecoveryEvents(3)[0].Status)
	require.Empty(t, tracker.GetPendingCommands(3))
	tracker.ReportAgentVersion(3, "test-release", "old-process")
	require.Equal(t, "dispatched", tracker.GetRecentRecoveryEvents(3)[0].Status)
	tracker.ReportAgentVersion(3, "test-release", "new-process")
	require.Equal(t, "success", start().GetRecentRecoveryEvents(3)[0].Status)
}

type mockNodeXrayStatusUpdater struct {
	calls []mockNodeXrayStatusCall
}

type mockNodeXrayStatusCall struct {
	nodeID      int64
	xrayRunning bool
	xrayVersion string
}

func TestAgentUpdateWaitsForTargetVersionRegistration(t *testing.T) {
	tracker := NewNodeRecoveryTracker(logger.NewNopLogger())
	tracker.ReportAgentVersion(3, "new-release", "prior-process")
	command, queued := tracker.QueueAgentUpdateCommand(3, "admin", "update", "new-release")
	if !queued || len(tracker.GetPendingCommands(3)) != 1 {
		t.Fatal("expected Agent update to be dispatched")
	}
	tracker.CompleteInflightCommand(3, command.ID, true, "Agent update scheduled", nil)
	tracker.ReportAgentVersion(3, "new-release", "")
	tracker.ReportAgentVersion(3, "new-release", "prior-process")
	if got := tracker.GetRecentRecoveryEvents(3)[0].Status; got != "dispatched" {
		t.Fatalf("update before target registration has status %q", got)
	}
	if _, queued := tracker.QueueXrayInstallVersionCommand(3, "admin", "switch", "26.11.9"); queued {
		t.Fatal("Xray switch queued during Agent update")
	}
	tracker.ReportAgentVersion(3, "new-release", "new-process")
	if got := tracker.GetRecentRecoveryEvents(3)[0].Status; got != "success" {
		t.Fatalf("update after target registration has status %q", got)
	}
	if _, queued := tracker.QueueXrayInstallVersionCommand(3, "admin", "switch", "26.11.9"); !queued {
		t.Fatal("expected Xray switch to be available after Agent update")
	}
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
	tracker.pendingSince[first.ID] = time.Now().Add(-nodeCommandPendingTTL - time.Second)
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

func TestNodeRecoveryTracker_QueuesFollowUpConfigSyncWhileInflight(t *testing.T) {
	tracker := NewNodeRecoveryTracker(logger.NewNopLogger())

	first, queued := tracker.QueueConfigSyncCommandDetailed(23, "node_heartbeat", "initial sync")
	if !queued {
		t.Fatal("expected initial config sync to be queued")
	}
	dispatched := tracker.GetPendingCommands(23)
	if len(dispatched) != 1 || dispatched[0].ID != first.ID {
		t.Fatalf("expected initial config sync to be dispatched, got %#v", dispatched)
	}

	followUp, queued := tracker.QueueConfigSyncCommandDetailed(23, "entitlement_auto_provision", "proxy created during sync")
	if !queued {
		t.Fatal("expected a follow-up config sync to be queued while the first is inflight")
	}
	if followUp.ID == first.ID {
		t.Fatal("expected follow-up config sync to have a new id")
	}
	if _, queuedAgain := tracker.QueueConfigSyncCommandDetailed(23, "entitlement_auto_provision", "another proxy created during sync"); queuedAgain {
		t.Fatal("expected additional config changes to coalesce into the pending follow-up sync")
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
	entry := tracker.inflightCommands[first.ID]
	entry.DispatchedAt = time.Now().Add(-nodeCommandInflightTTL - time.Second)
	tracker.inflightCommands[first.ID] = entry
	tracker.lastQueuedCommands[3][commandTypeXrayRestart] = time.Now().Add(-nodeCommandInflightTTL - time.Second)
	tracker.mu.Unlock()

	if _, queuedAfterExpiry := tracker.QueueXrayRestartCommand(3, "admin", "manual restart"); !queuedAfterExpiry {
		t.Fatal("expected stale inflight restart command to expire")
	}
}

func TestNodeRecoveryTracker_ExpiresXrayInstallAndRejectsWrongNodeResult(t *testing.T) {
	tracker := NewNodeRecoveryTracker(logger.NewNopLogger())
	first, queued := tracker.QueueXrayInstallVersionCommand(3, "admin", "switch", "26.10.31")
	if !queued || len(tracker.GetPendingCommands(3)) != 1 {
		t.Fatal("expected the install to be dispatched")
	}
	if kind, err := tracker.CompleteInflightCommand(4, first.ID, true, "wrong node", nil); kind != "" || err != nil {
		t.Fatal("a different node completed the installation")
	}
	entry, ok := tracker.inflightCommands[first.ID]
	if !ok {
		t.Fatal("a different node removed the installation")
	}
	entry.DispatchedAt = time.Now().Add(-xrayInstallInflightTTL - time.Second)
	tracker.inflightCommands[first.ID] = entry
	tracker.lastQueuedCommands[3][commandTypeXrayInstallVersion] = entry.DispatchedAt
	if got := tracker.GetRecentRecoveryEvents(3)[0].Status; got != "expired" {
		t.Fatalf("install state = %q, want expired", got)
	}
	if _, queued := tracker.QueueXrayInstallVersionCommand(3, "admin", "retry", "26.10.31"); !queued {
		t.Fatal("expected a retry after the dispatched command expired")
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
	err = updateNodeXrayStatusFromCommandResult(context.Background(), updater, 7, commandTypeXrayInstallVersion, true, map[string]any{
		"running": true, "version": "Xray 26.11.9",
	})
	if err != nil || len(updater.calls) != 3 || updater.calls[2].xrayVersion != "Xray 26.11.9" {
		t.Fatalf("version switch result was not recorded: %+v, %v", updater.calls, err)
	}
}
