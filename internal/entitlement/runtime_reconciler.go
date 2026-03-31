package entitlement

import (
	"context"
	"sync"
	"time"

	"v/internal/database/repository"
	"v/internal/logger"
	pkgerrors "v/pkg/errors"
)

// RuntimeReconcilerConfig controls periodic runtime cleanup checks.
type RuntimeReconcilerConfig struct {
	Interval time.Duration
}

// DefaultRuntimeReconcilerConfig returns the default reconciler config.
func DefaultRuntimeReconcilerConfig() *RuntimeReconcilerConfig {
	return &RuntimeReconcilerConfig{Interval: 30 * time.Minute}
}

// RuntimeReconcileStats describes one reconciliation run.
type RuntimeReconcileStats struct {
	ScannedProxies         int `json:"scanned_proxies"`
	DeletedMissingNode     int `json:"deleted_missing_node"`
	EvaluatedUsers         int `json:"evaluated_users"`
	ForbiddenUsersDetected int `json:"forbidden_users_detected"`
}

// RuntimeReconciler periodically cleans up stale runtime proxy state.
type RuntimeReconciler struct {
	config             *RuntimeReconcilerConfig
	entitlementService *Service
	proxyRepo          repository.ProxyRepository
	nodeRepo           repository.NodeRepository
	logger             logger.Logger

	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	running   bool
	runningMu sync.Mutex
}

// NewRuntimeReconciler creates a new runtime reconciler.
func NewRuntimeReconciler(
	cfg *RuntimeReconcilerConfig,
	entitlementService *Service,
	proxyRepo repository.ProxyRepository,
	nodeRepo repository.NodeRepository,
	log logger.Logger,
) *RuntimeReconciler {
	if cfg == nil {
		cfg = DefaultRuntimeReconcilerConfig()
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 30 * time.Minute
	}
	return &RuntimeReconciler{
		config:             cfg,
		entitlementService: entitlementService,
		proxyRepo:          proxyRepo,
		nodeRepo:           nodeRepo,
		logger:             log,
	}
}

// Start starts the reconciliation loop.
func (r *RuntimeReconciler) Start(ctx context.Context) error {
	r.runningMu.Lock()
	defer r.runningMu.Unlock()

	if r.running {
		return nil
	}

	r.ctx, r.cancel = context.WithCancel(ctx)
	r.running = true
	r.wg.Add(1)
	go r.runLoop()

	r.logger.Info("entitlement runtime reconciler started",
		logger.F("interval", r.config.Interval.String()))
	return nil
}

// Stop stops the reconciliation loop.
func (r *RuntimeReconciler) Stop(ctx context.Context) error {
	r.runningMu.Lock()
	if !r.running {
		r.runningMu.Unlock()
		return nil
	}
	r.cancel()
	r.running = false
	r.runningMu.Unlock()

	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		r.logger.Info("entitlement runtime reconciler stopped")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *RuntimeReconciler) runLoop() {
	defer r.wg.Done()

	r.runOnce(r.ctx)

	ticker := time.NewTicker(r.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.runOnce(r.ctx)
		}
	}
}

func (r *RuntimeReconciler) runOnce(ctx context.Context) {
	stats, err := r.RunOnce(ctx)
	if err != nil {
		r.logger.Warn("entitlement runtime reconciler run failed", logger.Err(err))
		return
	}
	if stats == nil {
		return
	}
	if stats.DeletedMissingNode == 0 && stats.ForbiddenUsersDetected == 0 {
		return
	}
	r.logger.Info("entitlement runtime reconciler processed stale runtime state",
		logger.F("scanned_proxies", stats.ScannedProxies),
		logger.F("deleted_missing_node", stats.DeletedMissingNode),
		logger.F("evaluated_users", stats.EvaluatedUsers),
		logger.F("forbidden_users_detected", stats.ForbiddenUsersDetected),
	)
}

// RunOnce runs one reconciliation pass.
func (r *RuntimeReconciler) RunOnce(ctx context.Context) (*RuntimeReconcileStats, error) {
	stats := &RuntimeReconcileStats{}
	if r == nil || r.entitlementService == nil || r.proxyRepo == nil {
		return stats, nil
	}

	userIDs, missingNodeProxyIDs, scanCount, err := r.scanStaleProxyState(ctx)
	if err != nil {
		return nil, err
	}
	stats.ScannedProxies = scanCount

	if len(missingNodeProxyIDs) > 0 {
		if err := r.proxyRepo.DeleteByIDs(ctx, missingNodeProxyIDs); err != nil {
			return nil, err
		}
		stats.DeletedMissingNode = len(missingNodeProxyIDs)
	}

	for userID := range userIDs {
		stats.EvaluatedUsers++
		if _, err := r.entitlementService.EvaluateExistingAccess(ctx, userID); err != nil {
			if pkgerrors.IsForbidden(err) {
				stats.ForbiddenUsersDetected++
				continue
			}
			r.logger.Warn("failed to reconcile user runtime state",
				logger.Err(err),
				logger.UserID(userID),
			)
		}
	}

	return stats, nil
}

func (r *RuntimeReconciler) scanStaleProxyState(ctx context.Context) (map[int64]struct{}, []int64, int, error) {
	userIDs := make(map[int64]struct{})
	missingNodeProxyIDs := make([]int64, 0)
	if r == nil || r.proxyRepo == nil {
		return userIDs, missingNodeProxyIDs, 0, nil
	}

	const batchSize = 500
	offset := 0
	scanned := 0
	nodeExistsCache := make(map[int64]bool)
	nodeMissingCache := make(map[int64]bool)

	for {
		proxies, err := r.proxyRepo.List(ctx, batchSize, offset)
		if err != nil {
			return nil, nil, scanned, err
		}
		if len(proxies) == 0 {
			break
		}
		scanned += len(proxies)

		for _, proxyModel := range proxies {
			if proxyModel == nil {
				continue
			}
			if proxyModel.UserID > 0 {
				userIDs[proxyModel.UserID] = struct{}{}
			}
			if proxyModel.NodeID == nil || *proxyModel.NodeID <= 0 || r.nodeRepo == nil {
				continue
			}

			nodeID := *proxyModel.NodeID
			if nodeMissingCache[nodeID] {
				missingNodeProxyIDs = append(missingNodeProxyIDs, proxyModel.ID)
				continue
			}
			if nodeExistsCache[nodeID] {
				continue
			}

			_, err := r.nodeRepo.GetByID(ctx, nodeID)
			if err == nil {
				nodeExistsCache[nodeID] = true
				continue
			}
			if pkgerrors.IsNotFound(err) {
				nodeMissingCache[nodeID] = true
				missingNodeProxyIDs = append(missingNodeProxyIDs, proxyModel.ID)
				continue
			}
			return nil, nil, scanned, err
		}

		if len(proxies) < batchSize {
			break
		}
		offset += len(proxies)
	}

	return userIDs, missingNodeProxyIDs, scanned, nil
}
