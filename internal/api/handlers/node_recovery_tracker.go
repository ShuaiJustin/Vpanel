package handlers

import (
	"fmt"
	"sync"
	"time"

	"v/internal/logger"
)

const (
	commandTypeXrayStart          = "xray_start"
	commandTypeXrayRestart        = "xray_restart"
	commandTypeXrayStatus         = "xray_status"
	commandTypeConfigSync         = "config_sync"
	commandTypeXrayInstallVersion = "xray_install_version"
	xrayRecoveryCommandCooldown   = 20 * time.Second
	nodeCommandPendingTTL         = 2 * time.Minute
	nodeCommandInflightTTL        = 10 * time.Minute
	maxNodeRecoveryEvents         = 12
)

type NodeRecoveryEvent struct {
	CommandID   string `json:"command_id"`
	CommandType string `json:"command_type"`
	Source      string `json:"source"`
	Reason      string `json:"reason,omitempty"`
	Status      string `json:"status"`
	Message     string `json:"message,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type inflightNodeCommand struct {
	NodeID  int64
	Command Command
}

type NodeRecoveryTracker struct {
	logger             logger.Logger
	mu                 sync.Mutex
	pendingCommands    map[int64][]Command
	inflightCommands   map[string]inflightNodeCommand
	lastQueuedCommands map[int64]map[string]time.Time
	recentEvents       map[int64][]NodeRecoveryEvent
}

func NewNodeRecoveryTracker(log logger.Logger) *NodeRecoveryTracker {
	return &NodeRecoveryTracker{
		logger:             log,
		pendingCommands:    make(map[int64][]Command),
		inflightCommands:   make(map[string]inflightNodeCommand),
		lastQueuedCommands: make(map[int64]map[string]time.Time),
		recentEvents:       make(map[int64][]NodeRecoveryEvent),
	}
}

func (t *NodeRecoveryTracker) QueueXrayRecoveryCommand(nodeID int64, source, reason string) bool {
	if t == nil || nodeID <= 0 {
		return false
	}
	_, queued := t.queueCommand(nodeID, commandTypeXrayStart, source, reason, map[string]any{
		"reason":    reason,
		"queued_at": time.Now().Unix(),
		"source":    source,
	})
	return queued
}

func (t *NodeRecoveryTracker) QueueConfigSyncCommand(nodeID int64, source, reason string) bool {
	if t == nil || nodeID <= 0 {
		return false
	}
	// config_sync should not carry recovery metadata payload. The agent treats
	// payload as xray config and may overwrite config.json if non-config data is sent.
	_, queued := t.queueCommand(nodeID, commandTypeConfigSync, source, reason, nil)
	return queued
}

func (t *NodeRecoveryTracker) QueueXrayStartCommand(nodeID int64, source, reason string) (Command, bool) {
	if t == nil || nodeID <= 0 {
		return Command{}, false
	}
	return t.queueCommand(nodeID, commandTypeXrayStart, source, reason, nil)
}

func (t *NodeRecoveryTracker) QueueXrayRestartCommand(nodeID int64, source, reason string) (Command, bool) {
	if t == nil || nodeID <= 0 {
		return Command{}, false
	}
	return t.queueCommand(nodeID, commandTypeXrayRestart, source, reason, nil)
}

func (t *NodeRecoveryTracker) QueueXrayStatusCommand(nodeID int64, source, reason string) (Command, bool) {
	if t == nil || nodeID <= 0 {
		return Command{}, false
	}
	return t.queueCommand(nodeID, commandTypeXrayStatus, source, reason, nil)
}

func (t *NodeRecoveryTracker) QueueConfigSyncCommandDetailed(nodeID int64, source, reason string) (Command, bool) {
	if t == nil || nodeID <= 0 {
		return Command{}, false
	}
	return t.queueCommand(nodeID, commandTypeConfigSync, source, reason, nil)
}

func (t *NodeRecoveryTracker) QueueXrayInstallVersionCommand(nodeID int64, source, reason, version string) (Command, bool) {
	if t == nil || nodeID <= 0 {
		return Command{}, false
	}
	return t.queueCommand(nodeID, commandTypeXrayInstallVersion, source, reason, map[string]any{
		"version": version,
	})
}

func (t *NodeRecoveryTracker) queueCommand(nodeID int64, commandType, source, reason string, payload any) (Command, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.pruneStaleCommandsLocked(nodeID)
	if t.hasPendingOrInflightCommandLocked(nodeID, commandType) {
		return Command{}, false
	}

	nodeCooldowns, ok := t.lastQueuedCommands[nodeID]
	if !ok {
		nodeCooldowns = make(map[string]time.Time)
		t.lastQueuedCommands[nodeID] = nodeCooldowns
	}
	if queuedAt, ok := nodeCooldowns[commandType]; ok && time.Since(queuedAt) < xrayRecoveryCommandCooldown {
		return Command{}, false
	}

	now := time.Now().Format(time.RFC3339)
	cmd := Command{
		ID:      fmt.Sprintf("%s-%d-%d", commandType, nodeID, time.Now().UnixNano()),
		Type:    commandType,
		Payload: payload,
	}
	t.pendingCommands[nodeID] = append(t.pendingCommands[nodeID], cmd)
	nodeCooldowns[commandType] = time.Now()
	t.appendEventLocked(nodeID, NodeRecoveryEvent{
		CommandID:   cmd.ID,
		CommandType: commandType,
		Source:      source,
		Reason:      reason,
		Status:      "queued",
		Message:     "已加入命令队列，等待节点心跳领取",
		CreatedAt:   now,
		UpdatedAt:   now,
	})

	t.logger.Info("Queued node command",
		logger.F("node_id", nodeID),
		logger.F("command_id", cmd.ID),
		logger.F("command_type", commandType),
		logger.F("source", source))
	return cmd, true
}

func (t *NodeRecoveryTracker) GetPendingCommands(nodeID int64) []Command {
	if t == nil || nodeID <= 0 {
		return []Command{}
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.pruneStaleCommandsLocked(nodeID)
	queued := t.pendingCommands[nodeID]
	if len(queued) == 0 {
		return []Command{}
	}

	commands := make([]Command, len(queued))
	copy(commands, queued)
	for _, cmd := range commands {
		t.inflightCommands[cmd.ID] = inflightNodeCommand{NodeID: nodeID, Command: cmd}
		t.updateEventStatusLocked(nodeID, cmd.ID, "dispatched", "已通过心跳下发到节点")
	}
	delete(t.pendingCommands, nodeID)
	return commands
}

func (t *NodeRecoveryTracker) CompleteInflightCommand(nodeID int64, commandID string, success bool, message string) string {
	if t == nil || commandID == "" {
		return ""
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	entry, ok := t.inflightCommands[commandID]
	if ok {
		delete(t.inflightCommands, commandID)
		if nodeID <= 0 {
			nodeID = entry.NodeID
		}
		status := "failed"
		if success {
			status = "success"
		}
		t.updateEventStatusLocked(nodeID, commandID, status, message)
		return entry.Command.Type
	}
	return ""
}

func (t *NodeRecoveryTracker) MarkCommandRecovered(nodeID int64, commandType, message string) {
	if t == nil || nodeID <= 0 || commandType == "" {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	queued := t.pendingCommands[nodeID]
	if len(queued) > 0 {
		filtered := queued[:0]
		for _, cmd := range queued {
			if cmd.Type == commandType {
				t.updateEventStatusLocked(nodeID, cmd.ID, "success", message)
				continue
			}
			filtered = append(filtered, cmd)
		}
		if len(filtered) == 0 {
			delete(t.pendingCommands, nodeID)
		} else {
			t.pendingCommands[nodeID] = filtered
		}
	}

	for commandID, entry := range t.inflightCommands {
		if entry.NodeID == nodeID && entry.Command.Type == commandType {
			delete(t.inflightCommands, commandID)
			t.updateEventStatusLocked(nodeID, commandID, "success", message)
		}
	}

	for index := range t.recentEvents[nodeID] {
		event := &t.recentEvents[nodeID][index]
		if event.CommandType != commandType {
			continue
		}
		if event.Status == "queued" || event.Status == "dispatched" {
			event.Status = "success"
			event.Message = message
			event.UpdatedAt = time.Now().Format(time.RFC3339)
		}
	}
}

func (t *NodeRecoveryTracker) GetRecentRecoveryEvents(nodeID int64) []NodeRecoveryEvent {
	if t == nil || nodeID <= 0 {
		return []NodeRecoveryEvent{}
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	events := t.recentEvents[nodeID]
	if len(events) == 0 {
		return []NodeRecoveryEvent{}
	}
	copied := make([]NodeRecoveryEvent, len(events))
	copy(copied, events)
	return copied
}

func (t *NodeRecoveryTracker) hasPendingOrInflightCommandLocked(nodeID int64, commandType string) bool {
	for _, cmd := range t.pendingCommands[nodeID] {
		if cmd.Type == commandType {
			return true
		}
	}
	for _, entry := range t.inflightCommands {
		if entry.NodeID == nodeID && entry.Command.Type == commandType {
			return true
		}
	}
	return false
}

func (t *NodeRecoveryTracker) pruneStaleCommandsLocked(nodeID int64) {
	if nodeID <= 0 {
		return
	}

	now := time.Now()
	if queued := t.pendingCommands[nodeID]; len(queued) > 0 {
		filtered := queued[:0]
		for _, cmd := range queued {
			if t.commandEventOlderThanLocked(nodeID, cmd.ID, now, nodeCommandPendingTTL) {
				t.updateEventStatusLocked(nodeID, cmd.ID, "expired", "命令等待节点心跳领取超时，已从队列移除；请先确认节点 Agent 已注册")
				continue
			}
			filtered = append(filtered, cmd)
		}
		if len(filtered) == 0 {
			delete(t.pendingCommands, nodeID)
		} else {
			t.pendingCommands[nodeID] = filtered
		}
	}

	for commandID, entry := range t.inflightCommands {
		if entry.NodeID != nodeID || entry.Command.Type == commandTypeXrayInstallVersion {
			continue
		}
		if t.commandEventOlderThanLocked(nodeID, commandID, now, nodeCommandInflightTTL) {
			delete(t.inflightCommands, commandID)
			t.updateEventStatusLocked(nodeID, commandID, "expired", "命令下发后长时间未收到结果，已允许重新下发")
		}
	}
}

func (t *NodeRecoveryTracker) commandEventOlderThanLocked(nodeID int64, commandID string, now time.Time, ttl time.Duration) bool {
	if ttl <= 0 {
		return false
	}
	for index := range t.recentEvents[nodeID] {
		event := t.recentEvents[nodeID][index]
		if event.CommandID != commandID {
			continue
		}
		if event.Status != "queued" && event.Status != "dispatched" {
			return false
		}
		createdAt, err := time.Parse(time.RFC3339, event.CreatedAt)
		if err != nil {
			return false
		}
		return now.Sub(createdAt) > ttl
	}
	return false
}

func (t *NodeRecoveryTracker) appendEventLocked(nodeID int64, event NodeRecoveryEvent) {
	events := append([]NodeRecoveryEvent{event}, t.recentEvents[nodeID]...)
	if len(events) > maxNodeRecoveryEvents {
		events = events[:maxNodeRecoveryEvents]
	}
	t.recentEvents[nodeID] = events
}

func (t *NodeRecoveryTracker) updateEventStatusLocked(nodeID int64, commandID, status, message string) {
	for index := range t.recentEvents[nodeID] {
		event := &t.recentEvents[nodeID][index]
		if event.CommandID != commandID {
			continue
		}
		event.Status = status
		if message != "" {
			event.Message = message
		}
		event.UpdatedAt = time.Now().Format(time.RFC3339)
		return
	}
}
