package handlers

import (
	"context"
	"sync"
	"time"

	"v/internal/database/repository"
	"v/internal/logger"
	"v/internal/xray"
	apperrors "v/pkg/errors"
)

// nodeConfigReconcileInterval bounds how long a node can keep running a config
// that no longer matches the database when nobody reported the change.
const nodeConfigReconcileInterval = 5 * time.Minute

// NodeConfigSyncer keeps the config a node runs equal to the config the panel
// would generate for it now.
//
// A node's config depends on its proxy rows and on who is entitled right now:
// the generator leaves out users who are expired, paused, disabled or over
// quota. So it changes when someone pays, runs out of quota or resumes, without
// any proxy row changing. Callers report such events with Request. Reconcile
// repeats the same comparison on a timer, so a change nobody reported (a writer
// that forgot, an expiry that no event marks) is still applied.
//
// The comparison is between two hashes: the config generated now, and
// nodes.config_version, the config the agent last confirmed applying. When they
// differ the node is marked pending in the database; the heartbeat hands the
// agent a config_sync command for as long as that flag is set.
//
// ponytail: every check regenerates the node's whole config, O(proxies on the
// node) with one entitlement lookup per owner. Fine for tens of nodes; beyond
// that, cache entitlement per sweep or raise nodeConfigReconcileInterval.
type NodeConfigSyncer struct {
	generator *xray.ConfigGenerator
	nodeRepo  nodeConfigStore
	tracker   *NodeRecoveryTracker
	logger    logger.Logger

	mu          sync.Mutex
	fetched     map[int64]string // version each agent fetched and has not confirmed yet
	lastChecked map[int64]time.Time
}

type nodeConfigStore interface {
	GetByID(ctx context.Context, id int64) (*repository.Node, error)
	UpdateSyncStatus(ctx context.Context, id int64, status string, syncedAt *time.Time) error
	MarkConfigApplied(ctx context.Context, id int64, version string, appliedAt time.Time) error
}

func NewNodeConfigSyncer(generator *xray.ConfigGenerator, nodeRepo nodeConfigStore, tracker *NodeRecoveryTracker, log logger.Logger) *NodeConfigSyncer {
	return &NodeConfigSyncer{
		generator:   generator,
		nodeRepo:    nodeRepo,
		tracker:     tracker,
		logger:      log,
		fetched:     make(map[int64]string),
		lastChecked: make(map[int64]time.Time),
	}
}

// MarkStale records that a proxy row on the node was created or deleted, so
// its config is known to differ. It does not generate the config to compare:
// it is called from inside entitlement evaluation, which the generator itself
// runs for every proxy owner on the node.
func (s *NodeConfigSyncer) MarkStale(ctx context.Context, nodeID int64, source, reason string) {
	if s == nil || nodeID <= 0 {
		return
	}
	s.markPending(ctx, nodeID, source, reason)
}

// Request asks the node to sync unless it already runs the current config. Use
// it when access changed without a proxy row changing (payment, quota reset,
// pause, resume), where only a comparison can tell whether the node is affected.
func (s *NodeConfigSyncer) Request(ctx context.Context, nodeID int64, source, reason string) {
	if s == nil || nodeID <= 0 {
		return
	}
	node, err := s.nodeRepo.GetByID(ctx, nodeID)
	if apperrors.IsNotFound(err) {
		return
	}
	if err == nil {
		// A reported change that cannot be compared is synced anyway: an extra
		// restart is cheaper than a revocation that never reaches the node.
		if current, cmpErr := s.runsCurrentConfig(ctx, node); cmpErr == nil && current {
			return
		}
	}
	s.markPending(ctx, nodeID, source, reason)
}

// Reconcile compares a reporting node with the database at most once per
// nodeConfigReconcileInterval. Call it from the heartbeat.
func (s *NodeConfigSyncer) Reconcile(ctx context.Context, nodeID int64) {
	if s == nil || nodeID <= 0 {
		return
	}
	now := time.Now()
	s.mu.Lock()
	last, seen := s.lastChecked[nodeID]
	if !seen {
		// First heartbeat since the panel started: wait out the node's share of
		// the interval before its first comparison.
		last = now.Add(firstReconcileDelay(nodeID) - nodeConfigReconcileInterval)
		s.lastChecked[nodeID] = last
	}
	if now.Sub(last) < nodeConfigReconcileInterval {
		s.mu.Unlock()
		return
	}
	s.lastChecked[nodeID] = now
	s.mu.Unlock()

	node, err := s.nodeRepo.GetByID(ctx, nodeID)
	// A pending or failed node is already handed a sync on every heartbeat.
	if err != nil || node.SyncStatus != repository.NodeSyncStatusSynced {
		return
	}
	// Nothing was reported here, so a failed comparison just waits for the next round.
	if current, err := s.runsCurrentConfig(ctx, node); err == nil && !current {
		s.markPending(ctx, nodeID, "config_reconcile", "node config no longer matches the database")
	}
}

// firstReconcileDelay spreads the first comparison after a panel start over the
// interval (0s, 30s, ... 270s by node ID). A release that changes every node's
// generated config would otherwise restart Xray on the whole fleet within one
// heartbeat, leaving users with no node to fail over to.
func firstReconcileDelay(nodeID int64) time.Duration {
	return time.Duration(nodeID%10) * nodeConfigReconcileInterval / 10
}

// Served records the config version an agent just fetched.
func (s *NodeConfigSyncer) Served(nodeID int64, version string) {
	if s == nil || nodeID <= 0 {
		return
	}
	s.mu.Lock()
	s.fetched[nodeID] = version
	s.mu.Unlock()
}

// Confirmed stores the outcome of a config sync reported by the agent.
func (s *NodeConfigSyncer) Confirmed(ctx context.Context, nodeID int64, success bool) error {
	return s.ConfirmedVersion(ctx, nodeID, success, "")
}

func (s *NodeConfigSyncer) ConfirmedVersion(ctx context.Context, nodeID int64, success bool, version string) error {
	if s == nil || nodeID <= 0 {
		return nil
	}
	if !success {
		return s.nodeRepo.UpdateSyncStatus(ctx, nodeID, repository.NodeSyncStatusFailed, nil)
	}

	s.mu.Lock()
	if version == "" {
		version = s.fetched[nodeID]
	}
	delete(s.fetched, nodeID)
	s.mu.Unlock()

	now := time.Now()
	if version == "" {
		return s.nodeRepo.UpdateSyncStatus(ctx, nodeID, repository.NodeSyncStatusPending, nil)
	}
	if err := s.nodeRepo.MarkConfigApplied(ctx, nodeID, version, now); err != nil {
		return err
	}
	// A change that landed while the agent was applying is not in that version.
	if current, err := s.currentVersion(ctx, nodeID); err == nil && current != version {
		s.markPending(ctx, nodeID, "config_sync_result", "config changed while the node was syncing")
	}
	return nil
}

func (s *NodeConfigSyncer) runsCurrentConfig(ctx context.Context, node *repository.Node) (bool, error) {
	if node.SyncStatus != repository.NodeSyncStatusSynced || node.ConfigVersion == "" {
		return false, nil
	}
	version, err := s.currentVersion(ctx, node.ID)
	if err != nil {
		return false, err
	}
	return version == node.ConfigVersion, nil
}

func (s *NodeConfigSyncer) CheckReportedVersion(ctx context.Context, nodeID int64, version string) {
	node, err := s.nodeRepo.GetByID(ctx, nodeID)
	if err == nil && node.SyncStatus == repository.NodeSyncStatusSynced && node.ConfigVersion != "" && node.ConfigVersion != version {
		s.markPending(ctx, nodeID, "heartbeat", "Agent reported a different Xray configuration on disk")
	}
}

func (s *NodeConfigSyncer) markPending(ctx context.Context, nodeID int64, source, reason string) {
	// NotFound is a node deleted in the meantime; there is nothing to sync.
	if err := s.nodeRepo.UpdateSyncStatus(ctx, nodeID, repository.NodeSyncStatusPending, nil); apperrors.IsNotFound(err) {
		return
	} else if err != nil {
		s.logger.Warn("failed to mark node config sync pending",
			logger.Err(err), logger.F("node_id", nodeID), logger.F("source", source))
	}
	// The agent restarts Xray itself while applying a synced config.
	s.tracker.QueueConfigSyncCommand(nodeID, source, reason)
}

func (s *NodeConfigSyncer) currentVersion(ctx context.Context, nodeID int64) (string, error) {
	config, err := s.generator.GenerateForNode(ctx, nodeID)
	if err != nil {
		return "", err
	}
	data, err := config.ToJSON()
	if err != nil {
		return "", err
	}
	return hashConfigVersion(data), nil
}
