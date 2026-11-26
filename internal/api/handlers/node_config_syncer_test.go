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
	"v/internal/xray"
	apperrors "v/pkg/errors"
)

// One node, one user-owned proxy. `entitled` stands in for the entitlement
// check the config generator applies to every proxy owner.
type nodeConfigSyncerFixture struct {
	db        *gorm.DB
	nodeRepo  repository.NodeRepository
	generator *xray.ConfigGenerator
	tracker   *NodeRecoveryTracker
	syncer    *NodeConfigSyncer
	entitled  bool
}

func newNodeConfigSyncerFixture(t *testing.T) *nodeConfigSyncerFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1) // as in production; each extra ":memory:" connection is an empty database
	require.NoError(t, db.AutoMigrate(&repository.User{}, &repository.Node{}, &repository.Proxy{}, &repository.Certificate{}))
	f := &nodeConfigSyncerFixture{db: db, nodeRepo: repository.NewNodeRepository(db), entitled: true}
	proxyRepo := repository.NewProxyRepository(db)
	require.NoError(t, db.Create(&repository.User{ID: 7, Username: "buyer", PasswordHash: "x", Enabled: true}).Error)
	require.NoError(t, db.Create(&repository.Node{ID: 1, Name: "node-1", Address: "198.51.100.10", Token: "t",
		Status: repository.NodeStatusOnline, SyncStatus: repository.NodeSyncStatusSynced}).Error)
	nodeID := int64(1)
	require.NoError(t, proxyRepo.Create(context.Background(), &repository.Proxy{UserID: 7, NodeID: &nodeID, Name: "p",
		Protocol: "vless", Port: 20007, Enabled: true, Settings: map[string]any{"uuid": "5f0c2b1e-6a55-4f0e-9f3a-0d8a1c2b3d4e"}}))

	f.generator = xray.NewConfigGenerator(proxyRepo, nil, f.nodeRepo, logger.NewNopLogger()).
		WithUserAccessCheck(func(context.Context, int64) error {
			if f.entitled {
				return nil
			}
			return apperrors.NewForbiddenError("traffic limit exceeded")
		})
	f.restartPanel()
	return f
}

// restartPanel drops everything the panel keeps in memory.
func (f *nodeConfigSyncerFixture) restartPanel() {
	f.tracker = NewNodeRecoveryTracker(logger.NewNopLogger())
	f.syncer = NewNodeConfigSyncer(f.generator, f.nodeRepo, f.tracker, logger.NewNopLogger())
}

func (f *nodeConfigSyncerFixture) status(t *testing.T) string {
	t.Helper()
	n, err := f.nodeRepo.GetByID(context.Background(), 1)
	require.NoError(t, err)
	return n.SyncStatus
}

// agentSyncs plays the agent: fetch the config, apply it, report success.
func (f *nodeConfigSyncerFixture) agentSyncs(t *testing.T) {
	t.Helper()
	version, err := f.syncer.currentVersion(context.Background(), 1)
	require.NoError(t, err)
	f.syncer.Served(1, version)
	require.NoError(t, f.syncer.Confirmed(context.Background(), 1, true))
}

func TestNodeConfigSyncer_SyncsOnlyWhenEntitlementChangesTheConfig(t *testing.T) {
	f := newNodeConfigSyncerFixture(t)
	ctx := context.Background()

	// The node never confirmed a config: sync once to establish what it runs.
	f.syncer.Request(ctx, 1, "test", "first contact")
	require.Equal(t, repository.NodeSyncStatusPending, f.status(t))
	commands := f.tracker.GetPendingCommands(1)
	require.Len(t, commands, 1, "one sync, no separate restart: the agent restarts Xray while applying")
	require.Equal(t, commandTypeConfigSync, commands[0].Type)
	f.agentSyncs(t)
	require.Equal(t, repository.NodeSyncStatusSynced, f.status(t))

	// An event that leaves the generated config identical must not restart Xray.
	f.syncer.Request(ctx, 1, "test", "renewal of a user who was never cut off")
	require.Equal(t, repository.NodeSyncStatusSynced, f.status(t))

	// The user runs out of quota: no proxy row changes, the config does.
	f.entitled = false
	f.syncer.Request(ctx, 1, "test", "quota exhausted")
	require.Equal(t, repository.NodeSyncStatusPending, f.status(t))
	f.agentSyncs(t)

	// The quota is reset: again no row changes, and the node must re-admit the user.
	f.entitled = true
	f.syncer.Request(ctx, 1, "test", "quota reset")
	require.Equal(t, repository.NodeSyncStatusPending, f.status(t),
		"a user who regains access must be pushed back to the node")
}

func TestNodeConfigSyncer_AppliedVersionSurvivesPanelRestart(t *testing.T) {
	f := newNodeConfigSyncerFixture(t)
	ctx := context.Background()
	f.syncer.Request(ctx, 1, "test", "first contact")
	f.agentSyncs(t)

	f.restartPanel()
	f.syncer.Request(ctx, 1, "test", "an event after the restart that changes nothing")
	require.Equal(t, repository.NodeSyncStatusSynced, f.status(t),
		"what the node runs is stored with the node, so a restart must not cost every node a resync")

	f.entitled = false
	f.syncer.Request(ctx, 1, "test", "quota exhausted")
	require.Equal(t, repository.NodeSyncStatusPending, f.status(t))
}

func TestNodeConfigSyncer_ChangeDuringInFlightSyncStaysPending(t *testing.T) {
	f := newNodeConfigSyncerFixture(t)
	ctx := context.Background()

	f.syncer.Request(ctx, 1, "test", "first contact")
	version, err := f.syncer.currentVersion(ctx, 1)
	require.NoError(t, err)
	f.syncer.Served(1, version) // the agent fetched its config ...

	f.entitled = false // ... and the user is cut off before the agent reports back.

	require.NoError(t, f.syncer.Confirmed(ctx, 1, true))
	require.Equal(t, repository.NodeSyncStatusPending, f.status(t),
		"the applied config predates the change, so the node is not in sync")

	f.agentSyncs(t)
	require.Equal(t, repository.NodeSyncStatusSynced, f.status(t))

	require.NoError(t, f.syncer.Confirmed(ctx, 1, false))
	require.Equal(t, repository.NodeSyncStatusFailed, f.status(t))
}

// Nobody calls Request when a subscription simply runs out of time. The
// periodic comparison is what removes that user from the node.
func TestNodeConfigSyncer_ReconcileAppliesChangesNobodyReported(t *testing.T) {
	f := newNodeConfigSyncerFixture(t)
	ctx := context.Background()
	f.syncer.Request(ctx, 1, "test", "first contact")
	f.agentSyncs(t)
	f.tracker.GetPendingCommands(1)
	due := func() {
		f.syncer.mu.Lock()
		f.syncer.lastChecked[1] = time.Now().Add(-nodeConfigReconcileInterval - time.Second)
		f.syncer.mu.Unlock()
	}

	due()
	f.syncer.Reconcile(ctx, 1)
	require.Equal(t, repository.NodeSyncStatusSynced, f.status(t), "nothing changed")

	f.entitled = false // expiry: no event, no hook

	f.syncer.Reconcile(ctx, 1)
	require.Equal(t, repository.NodeSyncStatusSynced, f.status(t), "checks are rate limited per node")

	due()
	f.syncer.Reconcile(ctx, 1)
	require.Equal(t, repository.NodeSyncStatusPending, f.status(t))
	commands := f.tracker.GetPendingCommands(1)
	require.Len(t, commands, 1)
	require.Equal(t, commandTypeConfigSync, commands[0].Type)
}

func TestNodeConfigSyncer_IgnoresDeletedNode(t *testing.T) {
	f := newNodeConfigSyncerFixture(t)
	f.syncer.Request(context.Background(), 404, "test", "node was deleted")
	require.Empty(t, f.tracker.GetPendingCommands(404))
}

// Deleting or creating a proxy row is reported from inside entitlement
// evaluation, which the generator runs for every owner. Comparing there would
// generate the node config once per revoked user, recursively.
func TestNodeConfigSyncer_MarkStaleDoesNotGenerate(t *testing.T) {
	f := newNodeConfigSyncerFixture(t)
	ctx := context.Background()
	f.syncer.Request(ctx, 1, "test", "first contact")
	f.agentSyncs(t)
	f.tracker.GetPendingCommands(1)

	generations := 0
	f.generator.WithUserAccessCheck(func(context.Context, int64) error { generations++; return nil })
	f.syncer.MarkStale(ctx, 1, "entitlement_cleanup", "proxy rows deleted")

	require.Zero(t, generations)
	require.Equal(t, repository.NodeSyncStatusPending, f.status(t))
	require.Len(t, f.tracker.GetPendingCommands(1), 1)
}

// After a panel start no node has been compared yet. Nodes take turns, so a
// release that changes every config does not restart the whole fleet at once.
func TestNodeConfigSyncer_FirstReconcileIsStaggered(t *testing.T) {
	seen := map[time.Duration]bool{}
	for _, nodeID := range []int64{28, 29, 30, 32, 34} {
		delay := firstReconcileDelay(nodeID)
		require.Less(t, delay, nodeConfigReconcileInterval)
		require.False(t, seen[delay], "node %d shares its slot with another node", nodeID)
		seen[delay] = true
	}

	f := newNodeConfigSyncerFixture(t) // node 1 never confirmed a config; its slot is 30s in
	f.syncer.Reconcile(context.Background(), 1)
	require.Equal(t, repository.NodeSyncStatusSynced, f.status(t), "not this node's turn yet")

	f.syncer.mu.Lock()
	f.syncer.lastChecked[1] = f.syncer.lastChecked[1].Add(-firstReconcileDelay(1) - time.Second)
	f.syncer.mu.Unlock()
	f.syncer.Reconcile(context.Background(), 1)
	require.Equal(t, repository.NodeSyncStatusPending, f.status(t), "a node that never confirmed a config is synced on its turn")
}
