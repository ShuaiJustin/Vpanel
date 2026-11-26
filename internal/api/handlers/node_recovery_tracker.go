package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"gorm.io/gorm"

	"v/internal/database/repository"
	"v/internal/logger"
)

var errSaveNodeOperation = errors.New("failed to save node operation")

const (
	commandTypeXrayStart          = "xray_start"
	commandTypeXrayRestart        = "xray_restart"
	commandTypeXrayStatus         = "xray_status"
	commandTypeConfigSync         = "config_sync"
	commandTypeXrayInstallVersion = "xray_install_version"
	commandTypeAgentUpdate        = "agent_update"
	xrayRecoveryCommandCooldown   = 20 * time.Second
	nodeCommandPendingTTL         = 2 * time.Minute
	nodeCommandInflightTTL        = 10 * time.Minute
	xrayInstallInflightTTL        = 15 * time.Minute
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
	NodeID        int64
	Command       Command
	DispatchedAt  time.Time
	AgentInstance string
}

type NodeRecoveryTracker struct {
	logger             logger.Logger
	mu                 sync.Mutex
	pendingCommands    map[int64][]Command
	inflightCommands   map[string]inflightNodeCommand
	lastQueuedCommands map[int64]map[string]time.Time
	recentEvents       map[int64][]NodeRecoveryEvent
	agentVersions      map[int64]string
	agentInstances     map[int64]string
	pendingSince       map[string]time.Time
	savedActive        map[string]bool
	storage            *gorm.DB
	restoreFailed      bool
}

func NewNodeRecoveryTracker(log logger.Logger) *NodeRecoveryTracker {
	return &NodeRecoveryTracker{
		logger:             log,
		pendingCommands:    make(map[int64][]Command),
		inflightCommands:   make(map[string]inflightNodeCommand),
		lastQueuedCommands: make(map[int64]map[string]time.Time),
		recentEvents:       make(map[int64][]NodeRecoveryEvent),
		agentVersions:      make(map[int64]string),
		agentInstances:     make(map[int64]string),
		pendingSince:       make(map[string]time.Time),
		savedActive:        make(map[string]bool),
	}
}

// WithStorage restores operator commands before the router accepts requests.
func (t *NodeRecoveryTracker) WithStorage(db *gorm.DB) *NodeRecoveryTracker {
	t.storage = db
	var rows []repository.NodeOperation
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.WithContext(ctx).Joins("JOIN nodes ON nodes.id = node_operations.node_id").
		Order("node_operations.created_at, node_operations.command_id").Find(&rows).Error; err != nil {
		t.restoreFailed = true
		t.logger.Error("Restore node operations", logger.Err(err))
		return t
	}
	for _, row := range rows {
		cmd := Command{ID: row.CommandID, Type: row.CommandType}
		if row.Payload != "" && row.Payload != "null" {
			var payload map[string]any
			if err := json.Unmarshal([]byte(row.Payload), &payload); err != nil {
				t.restoreFailed = true
				t.logger.Error("Restore node operation payload", logger.F("command_id", row.CommandID), logger.Err(err))
				continue
			}
			cmd.Payload = payload
		}
		t.appendEventLocked(row.NodeID, NodeRecoveryEvent{
			CommandID: cmd.ID, CommandType: cmd.Type, Source: "admin", Reason: row.Reason,
			Status: row.Status, Message: row.Message,
			CreatedAt: row.CreatedAt.Format(time.RFC3339), UpdatedAt: row.UpdatedAt.Format(time.RFC3339),
		})
		switch row.Status {
		case "queued":
			t.savedActive[cmd.ID] = true
			t.pendingCommands[row.NodeID] = append(t.pendingCommands[row.NodeID], cmd)
			t.pendingSince[cmd.ID] = row.CreatedAt
		case "dispatched":
			t.savedActive[cmd.ID] = true
			t.inflightCommands[cmd.ID] = inflightNodeCommand{
				NodeID: row.NodeID, Command: cmd, DispatchedAt: row.DispatchedAt, AgentInstance: row.AgentInstance,
			}
		}
	}
	return t
}

func (t *NodeRecoveryTracker) insertOperationLocked(row repository.NodeOperation) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := t.storage.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		var older []string
		if err := tx.Model(&repository.NodeOperation{}).
			Where("node_id = ? AND status NOT IN ?", row.NodeID, []string{"queued", "dispatched"}).
			Order("created_at DESC, command_id DESC").Offset(maxNodeRecoveryEvents).Limit(1000).
			Pluck("command_id", &older).Error; err != nil {
			return err
		}
		if len(older) == 0 {
			return nil
		}
		return tx.Where("command_id IN ?", older).Delete(&repository.NodeOperation{}).Error
	})
	if err != nil {
		t.logger.Error("Save queued node operation", logger.Err(err), logger.F("command_id", row.CommandID))
	}
	return err == nil
}

func (t *NodeRecoveryTracker) persistOperationStatusLocked(commandID string, fields map[string]any) bool {
	if !t.savedActive[commandID] {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := t.storage.WithContext(ctx).Model(&repository.NodeOperation{}).Where("command_id = ?", commandID).Updates(fields).Error
	if err != nil {
		t.logger.Error("Save node operation status", logger.Err(err), logger.F("command_id", commandID))
	}
	return err == nil
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

func (t *NodeRecoveryTracker) QueueAgentUpdateCommand(nodeID int64, source, reason, version string) (Command, bool) {
	if t == nil || nodeID <= 0 {
		return Command{}, false
	}
	return t.queueCommand(nodeID, commandTypeAgentUpdate, source, reason, map[string]any{"version": version})
}

func (t *NodeRecoveryTracker) ReportAgentVersion(nodeID int64, version, instance string) {
	if version == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.agentVersions[nodeID] = version
	if instance == "" || len(instance) > 64 {
		return
	}
	t.agentInstances[nodeID] = instance
	for id, entry := range t.inflightCommands {
		if entry.NodeID != nodeID || entry.Command.Type != commandTypeAgentUpdate || entry.AgentInstance == instance {
			continue
		}
		payload, ok := entry.Command.Payload.(map[string]any)
		if !ok || payload["version"] != version {
			continue
		}
		if t.updateEventStatusLocked(nodeID, id, "success", "Agent "+version+" 已重新连接并上报心跳") {
			delete(t.inflightCommands, id)
		}
	}
}

func (t *NodeRecoveryTracker) AgentVersion(nodeID int64) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.agentVersions[nodeID]
}

func (t *NodeRecoveryTracker) queueCommand(nodeID int64, commandType, source, reason string, payload any) (Command, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.pruneStaleCommandsLocked(nodeID)
	var conflictingType string
	switch commandType {
	case commandTypeAgentUpdate:
		conflictingType = commandTypeXrayInstallVersion
	case commandTypeXrayInstallVersion:
		conflictingType = commandTypeAgentUpdate
	}
	if conflictingType != "" && (t.hasPendingCommandLocked(nodeID, conflictingType) || t.hasInflightCommandLocked(nodeID, conflictingType)) {
		return Command{}, false
	}
	hasInflight := t.hasInflightCommandLocked(nodeID, commandType)
	if t.hasPendingCommandLocked(nodeID, commandType) ||
		(hasInflight && commandType != commandTypeConfigSync) {
		return Command{}, false
	}

	nodeCooldowns, ok := t.lastQueuedCommands[nodeID]
	if !ok {
		nodeCooldowns = make(map[string]time.Time)
		t.lastQueuedCommands[nodeID] = nodeCooldowns
	}
	if queuedAt, ok := nodeCooldowns[commandType]; ok && time.Since(queuedAt) < xrayRecoveryCommandCooldown && !hasInflight {
		return Command{}, false
	}

	queuedAt := time.Now()
	now := queuedAt.Format(time.RFC3339)
	cmd := Command{
		ID:      fmt.Sprintf("%s-%d-%d", commandType, nodeID, queuedAt.UnixNano()),
		Type:    commandType,
		Payload: payload,
	}
	if t.storage != nil && source == "admin" {
		if t.restoreFailed {
			return Command{queueError: errSaveNodeOperation}, false
		}
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.logger.Error("Encode node operation", logger.Err(err))
			return Command{queueError: errSaveNodeOperation}, false
		}
		if !t.insertOperationLocked(repository.NodeOperation{
			CommandID: cmd.ID, NodeID: nodeID, CommandType: commandType, Payload: string(encoded),
			Reason: reason, Status: "queued", Message: "已加入命令队列，等待节点心跳领取",
			CreatedAt: queuedAt, UpdatedAt: queuedAt,
		}) {
			return Command{queueError: errSaveNodeOperation}, false
		}
		t.savedActive[cmd.ID] = true
	}
	t.pendingCommands[nodeID] = append(t.pendingCommands[nodeID], cmd)
	t.pendingSince[cmd.ID] = queuedAt
	nodeCooldowns[commandType] = queuedAt
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

	commands := make([]Command, 0, len(queued))
	remaining := queued[:0]
	now := time.Now()
	for _, cmd := range queued {
		if !t.persistOperationStatusLocked(cmd.ID, map[string]any{
			"status": "dispatched", "message": "已通过心跳下发到节点", "updated_at": now,
			"dispatched_at": now, "agent_instance": t.agentInstances[nodeID],
		}) {
			remaining = append(remaining, cmd)
			continue
		}
		t.inflightCommands[cmd.ID] = inflightNodeCommand{
			NodeID: nodeID, Command: cmd, DispatchedAt: now, AgentInstance: t.agentInstances[nodeID],
		}
		t.setEventStatusLocked(nodeID, cmd.ID, "dispatched", "已通过心跳下发到节点", now)
		delete(t.pendingSince, cmd.ID)
		commands = append(commands, cmd)
	}
	if len(remaining) == 0 {
		delete(t.pendingCommands, nodeID)
	} else {
		t.pendingCommands[nodeID] = remaining
	}
	return commands
}

func (t *NodeRecoveryTracker) CompleteInflightCommand(nodeID int64, commandID string, success bool, message string, data any) (string, error) {
	if t == nil || commandID == "" {
		return "", nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	entry, ok := t.inflightCommands[commandID]
	if ok && entry.NodeID == nodeID {
		if entry.Command.Type == commandTypeAgentUpdate && success {
			if !t.updateEventStatusLocked(nodeID, commandID, "dispatched", "Agent 已接受更新，等待目标版本重新连接") {
				return "", errSaveNodeOperation
			}
			return entry.Command.Type, nil
		}
		status := "failed"
		if success {
			status = "success"
			if entry.Command.Type == commandTypeXrayInstallVersion {
				if reported, ok := extractReportedXrayStatus(data); !ok || !reported.Running || reported.Version == "" {
					status = "unverified"
					message = "Agent 已回报安装完成，请更新 Agent 以核对实际运行版本"
				}
			}
		}
		if !t.updateEventStatusLocked(nodeID, commandID, status, message) {
			return "", errSaveNodeOperation
		}
		delete(t.inflightCommands, commandID)
		return entry.Command.Type, nil
	}
	return "", nil
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
			if cmd.Type == commandType && t.updateEventStatusLocked(nodeID, cmd.ID, "success", message) {
				delete(t.pendingSince, cmd.ID)
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
		if entry.NodeID == nodeID && entry.Command.Type == commandType &&
			t.updateEventStatusLocked(nodeID, commandID, "success", message) {
			delete(t.inflightCommands, commandID)
		}
	}

}

func (t *NodeRecoveryTracker) GetRecentRecoveryEvents(nodeID int64) []NodeRecoveryEvent {
	if t == nil || nodeID <= 0 {
		return []NodeRecoveryEvent{}
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.pruneStaleCommandsLocked(nodeID)
	events := t.recentEvents[nodeID]
	if len(events) == 0 {
		return []NodeRecoveryEvent{}
	}
	return slices.Clone(events)
}

func (t *NodeRecoveryTracker) hasPendingCommandLocked(nodeID int64, commandType string) bool {
	for _, cmd := range t.pendingCommands[nodeID] {
		if cmd.Type == commandType {
			return true
		}
	}
	return false
}

func (t *NodeRecoveryTracker) hasInflightCommandLocked(nodeID int64, commandType string) bool {
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
			if now.Sub(t.pendingSince[cmd.ID]) > nodeCommandPendingTTL &&
				t.updateEventStatusLocked(nodeID, cmd.ID, "expired", "等待节点领取命令超时，请检查 Agent 心跳后重试") {
				delete(t.pendingSince, cmd.ID)
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
		if entry.NodeID != nodeID {
			continue
		}
		ttl := nodeCommandInflightTTL
		if entry.Command.Type == commandTypeXrayInstallVersion {
			ttl = xrayInstallInflightTTL
		}
		if now.Sub(entry.DispatchedAt) > ttl &&
			t.updateEventStatusLocked(nodeID, commandID, "expired", "命令下发后长时间未收到结果，已允许重新下发") {
			delete(t.inflightCommands, commandID)
		}
	}
}

func (t *NodeRecoveryTracker) appendEventLocked(nodeID int64, event NodeRecoveryEvent) {
	events := append([]NodeRecoveryEvent{event}, t.recentEvents[nodeID]...)
	if len(events) > maxNodeRecoveryEvents {
		events = events[:maxNodeRecoveryEvents]
	}
	t.recentEvents[nodeID] = events
}

func (t *NodeRecoveryTracker) updateEventStatusLocked(nodeID int64, commandID, status, message string) bool {
	now := time.Now()
	fields := map[string]any{"status": status, "updated_at": now}
	if message != "" {
		fields["message"] = message
	}
	if !t.persistOperationStatusLocked(commandID, fields) {
		return false
	}
	if status != "queued" && status != "dispatched" {
		delete(t.savedActive, commandID)
	}
	t.setEventStatusLocked(nodeID, commandID, status, message, now)
	return true
}

func (t *NodeRecoveryTracker) setEventStatusLocked(nodeID int64, commandID, status, message string, now time.Time) {
	for index := range t.recentEvents[nodeID] {
		event := &t.recentEvents[nodeID][index]
		if event.CommandID != commandID {
			continue
		}
		event.Status = status
		if message != "" {
			event.Message = message
		}
		event.UpdatedAt = now.Format(time.RFC3339)
		return
	}
}
