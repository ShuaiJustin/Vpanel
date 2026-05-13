// Package entitlement centralizes user access checks for portal nodes and subscriptions.
package entitlement

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"gorm.io/gorm"

	"v/internal/commercial/trial"
	"v/internal/database/repository"
	"v/internal/logger"
	proxylib "v/internal/proxy"
	settingssvc "v/internal/settings"
	"v/pkg/errors"
)

const (
	autoProvisionPortMin = 20000
	autoProvisionPortMax = 60000
)

var defaultAutoProvisionProtocolPreference = []string{"trojan", "vmess", "vless", "shadowsocks"}

// AccessState describes the effective access state for a portal user.
type AccessState struct {
	User                  *repository.User
	Trial                 *repository.Trial
	HasActiveSubscription bool
	HasActiveTrial        bool
	EffectiveExpiresAt    *time.Time
	EffectiveTrafficLimit int64
	EffectiveTrafficUsed  int64
}

// NodeProvisionResult summarizes proactive proxy provisioning for a node.
type NodeProvisionResult struct {
	NodeID        int64
	ScannedUsers  int
	EntitledUsers int
	Created       int
	Existing      int
	Skipped       int
}

// AutoProvisionRebuildResult summarizes a user auto-proxy rebuild.
type AutoProvisionRebuildResult struct {
	UserID          int64   `json:"user_id"`
	NodeID          *int64  `json:"node_id,omitempty"`
	TargetNodeIDs   []int64 `json:"target_node_ids"`
	Deleted         int     `json:"deleted"`
	Created         int     `json:"created"`
	Skipped         int     `json:"skipped"`
	DeletedProxyIDs []int64 `json:"deleted_proxy_ids"`
	CreatedProxyIDs []int64 `json:"created_proxy_ids"`
}

// Service provides user entitlement and node assignment logic.
type Service struct {
	userRepo        repository.UserRepository
	trialRepo       repository.TrialRepository
	proxyRepo       repository.ProxyRepository
	nodeRepo        repository.NodeRepository
	assignmentRepo  repository.UserNodeAssignmentRepository
	pauseRepo       repository.PauseRepository
	trialService    *trial.Service
	proxyManager    proxylib.Manager
	settingsService *settingssvc.Service
	configSyncHook  func(nodeID int64, source, reason string)
	logger          logger.Logger
}

// NewService creates a new entitlement service.
func NewService(
	userRepo repository.UserRepository,
	trialRepo repository.TrialRepository,
	proxyRepo repository.ProxyRepository,
	nodeRepo repository.NodeRepository,
	assignmentRepo repository.UserNodeAssignmentRepository,
	trialService *trial.Service,
	log logger.Logger,
) *Service {
	return &Service{
		userRepo:       userRepo,
		trialRepo:      trialRepo,
		proxyRepo:      proxyRepo,
		nodeRepo:       nodeRepo,
		assignmentRepo: assignmentRepo,
		trialService:   trialService,
		logger:         log,
	}
}

// WithProxyManager enables automatic default proxy provisioning.
func (s *Service) WithProxyManager(proxyManager proxylib.Manager) *Service {
	s.proxyManager = proxyManager
	return s
}

// WithSettingsService enables configurable automatic proxy provisioning settings.
func (s *Service) WithSettingsService(settingsService *settingssvc.Service) *Service {
	s.settingsService = settingsService
	return s
}

// WithPauseRepository enables pause-aware cleanup decisions for expired subscriptions.
func (s *Service) WithPauseRepository(pauseRepo repository.PauseRepository) *Service {
	s.pauseRepo = pauseRepo
	return s
}

// WithConfigSyncHook registers a callback invoked after auto-provisioning a node proxy.
func (s *Service) WithConfigSyncHook(hook func(nodeID int64, source, reason string)) *Service {
	s.configSyncHook = hook
	return s
}

// EvaluateAccess resolves the user's effective access state.
// If the user has no active paid subscription, the first portal access auto-activates one trial when enabled.
func (s *Service) EvaluateAccess(ctx context.Context, userID int64) (*AccessState, error) {
	return s.evaluateAccess(ctx, userID, true)
}

// EvaluateExistingAccess resolves the user's current access state without auto-activating a trial.
// This is safe to use in read-only paths such as node config generation.
func (s *Service) EvaluateExistingAccess(ctx context.Context, userID int64) (*AccessState, error) {
	return s.evaluateAccess(ctx, userID, false)
}

func (s *Service) evaluateAccess(ctx context.Context, userID int64, allowAutoActivateTrial bool) (*AccessState, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	state := &AccessState{
		User:                  user,
		EffectiveTrafficUsed:  user.TrafficUsed,
		EffectiveTrafficLimit: user.TrafficLimit,
	}

	if !user.Enabled {
		if cleanupErr := s.cleanupRevokedUserRuntime(ctx, userID, "entitlement_user_disabled", "disabled user revoked user proxy runtime", false); cleanupErr != nil {
			s.logger.Warn("failed to cleanup disabled user runtime resources",
				logger.Err(cleanupErr),
				logger.UserID(userID),
			)
		}
		return state, errors.NewForbiddenError("user account is disabled")
	}

	now := time.Now()

	if user.ExpiresAt != nil && !now.After(*user.ExpiresAt) {
		state.HasActiveSubscription = true
		state.EffectiveExpiresAt = user.ExpiresAt
	}
	if user.ExpiresAt == nil && user.TrafficLimit > 0 {
		state.HasActiveSubscription = true
	}

	var repoTrial *repository.Trial
	if allowAutoActivateTrial {
		repoTrial, err = s.getOrAutoActivateTrial(ctx, userID, state.HasActiveSubscription)
	} else {
		repoTrial, err = s.getTrial(ctx, userID)
	}
	if err != nil {
		return nil, err
	}
	expiredTrialRequiresCleanup := false
	if repoTrial != nil {
		if repoTrial.Status == "active" && now.After(repoTrial.ExpireAt) {
			expiredTrialRequiresCleanup = !state.HasActiveSubscription
			if !state.HasActiveSubscription {
				expireAt := repoTrial.ExpireAt
				state.EffectiveExpiresAt = &expireAt
			}
			if updateErr := s.trialRepo.UpdateStatus(ctx, repoTrial.ID, "expired"); updateErr != nil {
				s.logger.Warn("failed to expire stale trial during entitlement check",
					logger.Err(updateErr),
					logger.UserID(userID),
					logger.F("trial_id", repoTrial.ID),
				)
			} else {
				repoTrial.Status = "expired"
			}
		} else if repoTrial.Status == "active" && !now.After(repoTrial.ExpireAt) {
			state.Trial = repoTrial
			state.HasActiveTrial = true
			if !state.HasActiveSubscription {
				expireAt := repoTrial.ExpireAt
				state.EffectiveExpiresAt = &expireAt
				if cfg := s.trialConfig(); cfg != nil && cfg.TrafficLimit > 0 {
					state.EffectiveTrafficLimit = cfg.TrafficLimit
					// 试用期应使用 trial.TrafficUsed 而非用户历史总流量
					state.EffectiveTrafficUsed = repoTrial.TrafficUsed
				}
			}
		} else if repoTrial.Status == "expired" && !state.HasActiveSubscription {
			expiredTrialRequiresCleanup = true
		}
	}

	if expiredTrialRequiresCleanup {
		if cleanupErr := s.cleanupRevokedUserRuntime(ctx, userID, "entitlement_trial_cleanup", "expired trial revoked user proxy runtime", false); cleanupErr != nil {
			s.logger.Warn("failed to cleanup expired trial runtime resources",
				logger.Err(cleanupErr),
				logger.UserID(userID),
			)
		}
	}

	if user.ExpiresAt != nil && now.After(*user.ExpiresAt) && !state.HasActiveSubscription && !state.HasActiveTrial {
		if cleanupErr := s.cleanupRevokedUserRuntime(ctx, userID, "entitlement_subscription_cleanup", "expired subscription revoked user proxy runtime", true); cleanupErr != nil {
			s.logger.Warn("failed to cleanup expired subscription runtime resources",
				logger.Err(cleanupErr),
				logger.UserID(userID),
			)
		}
	}

	if !state.HasActiveSubscription && !state.HasActiveTrial {
		return state, errors.NewForbiddenError("当前无有效订阅或试用")
	}

	if state.EffectiveExpiresAt != nil && now.After(*state.EffectiveExpiresAt) {
		return state, errors.NewForbiddenError("user account has expired")
	}

	if state.EffectiveTrafficLimit > 0 && state.EffectiveTrafficUsed >= state.EffectiveTrafficLimit {
		return state, errors.NewForbiddenError("traffic limit exceeded")
	}

	return state, nil
}

func (s *Service) cleanupRevokedUserRuntime(ctx context.Context, userID int64, source, reason string, skipIfActivePause bool) error {
	if s == nil || userID <= 0 {
		return nil
	}
	if skipIfActivePause {
		activePause, err := s.activePause(ctx, userID)
		if err != nil {
			return err
		}
		if activePause != nil {
			return nil
		}
	}

	affectedNodeIDs := make(map[int64]struct{})
	deletedProxyIDs := make([]int64, 0)

	if s.proxyRepo != nil {
		proxies, err := s.proxyRepo.GetByUserID(ctx, userID, 10000, 0)
		if err != nil {
			return err
		}
		for _, proxyModel := range proxies {
			if proxyModel == nil {
				continue
			}
			deletedProxyIDs = append(deletedProxyIDs, proxyModel.ID)
			if proxyModel.NodeID != nil && *proxyModel.NodeID > 0 {
				affectedNodeIDs[*proxyModel.NodeID] = struct{}{}
			}
		}
		if err := s.proxyRepo.DeleteByIDs(ctx, deletedProxyIDs); err != nil {
			return err
		}
	}

	if s.assignmentRepo != nil {
		if err := s.assignmentRepo.Unassign(ctx, userID); err != nil {
			return err
		}
	}

	if s.configSyncHook != nil {
		for nodeID := range affectedNodeIDs {
			s.configSyncHook(nodeID, source, reason)
		}
	}

	if len(deletedProxyIDs) > 0 || len(affectedNodeIDs) > 0 {
		s.logger.Info("cleaned up expired trial runtime resources",
			logger.UserID(userID),
			logger.F("deleted_proxy_count", len(deletedProxyIDs)),
			logger.F("affected_node_count", len(affectedNodeIDs)),
		)
	}

	return nil
}

func (s *Service) activePause(ctx context.Context, userID int64) (*repository.SubscriptionPause, error) {
	if s == nil || s.pauseRepo == nil || userID <= 0 {
		return nil, nil
	}
	return s.pauseRepo.GetActivePause(ctx, userID)
}

// GetAccessibleProxies returns only the proxies the user is entitled to use.
func (s *Service) GetAccessibleProxies(ctx context.Context, userID int64) ([]*repository.Proxy, *AccessState, error) {
	state, err := s.EvaluateAccess(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	proxies, err := s.proxyRepo.GetByUserID(ctx, userID, 10000, 0)
	if err != nil {
		return nil, nil, err
	}
	if len(proxies) > 0 {
		proxies, err = s.reconcileUserAutoProvisionedProxies(ctx, proxies)
		if err != nil {
			return nil, nil, err
		}
		usableProxies, filterErr := s.filterUsableUserProxies(ctx, proxies)
		if filterErr != nil {
			return nil, nil, filterErr
		}
		if enabled := enabledOnly(usableProxies); len(enabled) > 0 {
			return enabled, state, nil
		}
	}

	nodeID, err := s.getOrAssignNode(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	proxies, err = s.proxyRepo.GetByNodeID(ctx, nodeID)
	if err != nil {
		return nil, nil, err
	}
	proxies = sharedOnly(enabledOnly(proxies))
	if len(proxies) == 0 {
		proxyModel, provisionErr := s.autoProvisionDefaultProxy(ctx, userID, nodeID)
		if provisionErr != nil {
			return nil, nil, provisionErr
		}
		if proxyModel != nil {
			return []*repository.Proxy{proxyModel}, state, nil
		}
		return nil, nil, errors.NewForbiddenError("当前暂无可用节点")
	}

	return []*repository.Proxy{selectPrimaryProxy(proxies)}, state, nil
}

// GetSubscriptionProxies returns all proxies a user can use in subscription clients.
func (s *Service) GetSubscriptionProxies(ctx context.Context, userID int64) ([]*repository.Proxy, *AccessState, error) {
	state, err := s.EvaluateAccess(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	result := make([]*repository.Proxy, 0)
	existingNodeIDs := map[int64]struct{}{}

	proxies, err := s.proxyRepo.GetByUserID(ctx, userID, 10000, 0)
	if err != nil {
		return nil, nil, err
	}
	if len(proxies) > 0 {
		proxies, err = s.reconcileUserAutoProvisionedProxies(ctx, proxies)
		if err != nil {
			return nil, nil, err
		}
		usableProxies, filterErr := s.filterUsableUserProxies(ctx, proxies)
		if filterErr != nil {
			return nil, nil, filterErr
		}
		for _, proxyModel := range enabledOnly(usableProxies) {
			result = append(result, proxyModel)
			if proxyModel.NodeID != nil {
				existingNodeIDs[*proxyModel.NodeID] = struct{}{}
			}
		}
	}

	if s.nodeRepo == nil || s.proxyRepo == nil {
		return result, state, nil
	}

	availableNodes, err := s.nodeRepo.GetAvailable(ctx)
	if err != nil {
		if len(result) > 0 {
			s.logger.Warn("failed to list available nodes for subscription proxies",
				logger.Err(err),
				logger.UserID(userID),
			)
			return result, state, nil
		}
		return nil, nil, err
	}

	for _, nodeModel := range availableNodes {
		if nodeModel == nil {
			continue
		}
		if _, exists := existingNodeIDs[nodeModel.ID]; exists {
			continue
		}

		nodeProxies, proxyErr := s.proxyRepo.GetByNodeID(ctx, nodeModel.ID)
		if proxyErr != nil {
			s.logger.Warn("failed to load node proxies for subscription",
				logger.Err(proxyErr),
				logger.UserID(userID),
				logger.F("node_id", nodeModel.ID),
			)
			continue
		}

		shared := sharedOnly(enabledOnly(nodeProxies))
		if len(shared) > 0 {
			selected := selectPrimaryProxy(shared)
			if selected != nil {
				result = append(result, selected)
				existingNodeIDs[nodeModel.ID] = struct{}{}
			}
			continue
		}

		if !s.canAutoProvisionDefaultProxy() {
			continue
		}

		proxyModel, provisionErr := s.ensureAutoProvisionedProxyOnNode(ctx, userID, nodeModel.ID)
		if provisionErr != nil {
			s.logger.Warn("failed to auto provision subscription proxy",
				logger.Err(provisionErr),
				logger.UserID(userID),
				logger.F("node_id", nodeModel.ID),
			)
			continue
		}
		if proxyModel != nil {
			result = append(result, proxyModel)
			existingNodeIDs[nodeModel.ID] = struct{}{}
		}
	}

	if len(result) == 0 {
		return nil, nil, errors.NewForbiddenError("当前暂无可用节点")
	}

	return result, state, nil
}

// ProvisionNodeProxies proactively creates default proxies for all currently entitled portal users on a node.
func (s *Service) ProvisionNodeProxies(ctx context.Context, nodeID int64) (*NodeProvisionResult, error) {
	result := &NodeProvisionResult{NodeID: nodeID}
	if s == nil || !s.canAutoProvisionDefaultProxy() || s.userRepo == nil {
		return result, nil
	}
	if nodeID <= 0 {
		return result, errors.NewValidationError("invalid node id", fmt.Sprintf("%d", nodeID))
	}

	nodeModel, err := s.nodeRepo.GetByID(ctx, nodeID)
	if err != nil {
		return result, err
	}
	if nodeModel.Status != repository.NodeStatusOnline {
		result.Skipped++
		return result, nil
	}

	const pageLimit = 200
	for offset := 0; ; offset += pageLimit {
		users, listErr := s.userRepo.List(ctx, pageLimit, offset)
		if listErr != nil {
			return result, listErr
		}
		if len(users) == 0 {
			break
		}

		for _, user := range users {
			if user == nil {
				continue
			}
			result.ScannedUsers++
			if !user.Enabled || strings.TrimSpace(user.Role) != "user" {
				result.Skipped++
				continue
			}

			if _, accessErr := s.EvaluateExistingAccess(ctx, user.ID); accessErr != nil {
				result.Skipped++
				continue
			}
			result.EntitledUsers++

			existed := false
			existing, proxyErr := s.proxyRepo.GetByUserID(ctx, user.ID, 10000, 0)
			if proxyErr != nil {
				return result, proxyErr
			}
			for _, proxyModel := range enabledOnly(existing) {
				if proxyModel != nil && proxyModel.NodeID != nil && *proxyModel.NodeID == nodeID {
					existed = true
					break
				}
			}
			if existed {
				result.Existing++
				continue
			}

			proxyModel, provisionErr := s.ensureAutoProvisionedProxyOnNode(ctx, user.ID, nodeID)
			if provisionErr != nil {
				s.logger.Warn("failed to provision proxy on newly available node",
					logger.Err(provisionErr),
					logger.UserID(user.ID),
					logger.F("node_id", nodeID),
				)
				result.Skipped++
				continue
			}
			if proxyModel != nil {
				result.Created++
			}
		}

		if len(users) < pageLimit {
			break
		}
	}

	return result, nil
}

// RebuildAutoProvisionedProxies recreates a user's auto provisioned proxies using
// the current protocol selection policy. Manually created proxies are untouched.
func (s *Service) RebuildAutoProvisionedProxies(ctx context.Context, userID int64, nodeID *int64) (*AutoProvisionRebuildResult, error) {
	result := &AutoProvisionRebuildResult{UserID: userID, NodeID: nodeID}
	if s == nil || !s.canAutoProvisionDefaultProxy() {
		return result, errors.NewValidationError("automatic proxy provisioning is not available", nil)
	}
	if userID <= 0 {
		return result, errors.NewValidationError("invalid user id", fmt.Sprintf("%d", userID))
	}
	if nodeID != nil && *nodeID <= 0 {
		return result, errors.NewValidationError("invalid node id", fmt.Sprintf("%d", *nodeID))
	}

	if err := s.ensureAutoProvisionRebuildAccess(ctx, userID); err != nil {
		return result, err
	}

	existing, err := s.proxyRepo.GetByUserID(ctx, userID, 10000, 0)
	if err != nil {
		return result, err
	}

	oldAutoProxyIDsByNode := make(map[int64][]int64)
	targetNodeIDs := make([]int64, 0)
	targetSeen := make(map[int64]struct{})
	addTargetNode := func(id int64) {
		if id <= 0 {
			return
		}
		if _, exists := targetSeen[id]; exists {
			return
		}
		targetSeen[id] = struct{}{}
		targetNodeIDs = append(targetNodeIDs, id)
	}

	if nodeID != nil {
		addTargetNode(*nodeID)
	}

	for _, proxyModel := range existing {
		if proxyModel == nil || !isAutoProvisionedProxy(proxyModel) || proxyModel.NodeID == nil {
			continue
		}
		if nodeID != nil && *proxyModel.NodeID != *nodeID {
			continue
		}
		oldAutoProxyIDsByNode[*proxyModel.NodeID] = append(oldAutoProxyIDsByNode[*proxyModel.NodeID], proxyModel.ID)
		addTargetNode(*proxyModel.NodeID)
	}

	result.TargetNodeIDs = append(result.TargetNodeIDs, targetNodeIDs...)
	if len(targetNodeIDs) == 0 {
		return result, nil
	}

	deletableOldProxyIDs := make([]int64, 0)
	affectedNodeIDs := make(map[int64]struct{})
	for _, targetNodeID := range targetNodeIDs {
		createdProxy, provisionErr := s.createAutoProvisionedProxy(ctx, userID, targetNodeID)
		if provisionErr != nil {
			s.logger.Warn("failed to rebuild auto provisioned proxy",
				logger.Err(provisionErr),
				logger.UserID(userID),
				logger.F("node_id", targetNodeID),
			)
			result.Skipped++
			continue
		}
		if createdProxy == nil {
			result.Skipped++
			continue
		}

		result.Created++
		result.CreatedProxyIDs = append(result.CreatedProxyIDs, createdProxy.ID)
		affectedNodeIDs[targetNodeID] = struct{}{}
		deletableOldProxyIDs = append(deletableOldProxyIDs, oldAutoProxyIDsByNode[targetNodeID]...)
	}

	if len(deletableOldProxyIDs) > 0 {
		if err := s.proxyRepo.DeleteByIDs(ctx, deletableOldProxyIDs); err != nil {
			return result, err
		}
		result.Deleted = len(deletableOldProxyIDs)
		result.DeletedProxyIDs = append(result.DeletedProxyIDs, deletableOldProxyIDs...)
	}

	if s.configSyncHook != nil {
		for affectedNodeID := range affectedNodeIDs {
			s.configSyncHook(affectedNodeID, "entitlement_auto_provision_rebuild", "rebuilt auto provisioned proxies")
		}
	}

	if result.Created > 0 || result.Deleted > 0 {
		s.logger.Info("rebuilt auto provisioned proxies",
			logger.UserID(userID),
			logger.F("target_node_count", len(targetNodeIDs)),
			logger.F("created_proxy_count", result.Created),
			logger.F("deleted_proxy_count", result.Deleted),
		)
	}

	return result, nil
}

func (s *Service) ensureAutoProvisionRebuildAccess(ctx context.Context, userID int64) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if !user.Enabled {
		return errors.NewForbiddenError("user account is disabled")
	}

	now := time.Now()
	hasActiveSubscription := false
	effectiveTrafficLimit := user.TrafficLimit
	effectiveTrafficUsed := user.TrafficUsed

	if user.ExpiresAt != nil && !now.After(*user.ExpiresAt) {
		hasActiveSubscription = true
	}
	if user.ExpiresAt == nil && user.TrafficLimit > 0 {
		hasActiveSubscription = true
	}

	hasActiveTrial := false
	if trialModel, trialErr := s.getTrial(ctx, userID); trialErr != nil {
		return trialErr
	} else if trialModel != nil && trialModel.Status == "active" && !now.After(trialModel.ExpireAt) {
		hasActiveTrial = true
		if !hasActiveSubscription {
			if cfg := s.trialConfig(); cfg != nil && cfg.TrafficLimit > 0 {
				effectiveTrafficLimit = cfg.TrafficLimit
				effectiveTrafficUsed = trialModel.TrafficUsed
			}
		}
	}

	if !hasActiveSubscription && !hasActiveTrial {
		return errors.NewForbiddenError("当前无有效订阅或试用")
	}
	if effectiveTrafficLimit > 0 && effectiveTrafficUsed >= effectiveTrafficLimit {
		return errors.NewForbiddenError("traffic limit exceeded")
	}

	return nil
}

// GetAccessibleProxy returns a specific accessible proxy for a user.
func (s *Service) GetAccessibleProxy(ctx context.Context, userID, proxyID int64) (*repository.Proxy, *AccessState, error) {
	proxies, state, err := s.GetAccessibleProxies(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	for _, proxy := range proxies {
		if proxy.ID == proxyID {
			return proxy, state, nil
		}
	}

	return nil, nil, errors.NewForbiddenError("该节点当前不可用")
}

func (s *Service) getOrAutoActivateTrial(ctx context.Context, userID int64, hasActiveSubscription bool) (*repository.Trial, error) {
	repoTrial, err := s.getTrial(ctx, userID)
	if err != nil {
		return nil, err
	}
	if repoTrial != nil {
		return repoTrial, nil
	}

	cfg := s.trialConfig()
	if hasActiveSubscription || cfg == nil || !cfg.Enabled || !cfg.AutoActivate || s.trialService == nil {
		return nil, nil
	}

	if _, err := s.trialService.ActivateTrial(ctx, userID); err != nil && err != trial.ErrTrialAlreadyUsed {
		return nil, err
	}

	repoTrial, err = s.trialRepo.GetByUserID(ctx, userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return repoTrial, nil
}

func (s *Service) getTrial(ctx context.Context, userID int64) (*repository.Trial, error) {
	repoTrial, err := s.trialRepo.GetByUserID(ctx, userID)
	if err == nil {
		return repoTrial, nil
	}
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return nil, err
}

func (s *Service) getOrAssignNode(ctx context.Context, userID int64) (int64, error) {
	if s.assignmentRepo == nil || s.nodeRepo == nil || s.proxyRepo == nil {
		return 0, errors.NewForbiddenError("当前暂无可用节点")
	}

	assignment, err := s.assignmentRepo.GetByUserID(ctx, userID)
	if err != nil {
		return 0, err
	}
	if assignment != nil {
		node, nodeErr := s.nodeRepo.GetByID(ctx, assignment.NodeID)
		proxies, proxyErr := s.proxyRepo.GetByNodeID(ctx, assignment.NodeID)
		if nodeErr == nil && node.Status == repository.NodeStatusOnline && (node.MaxUsers == 0 || node.CurrentUsers < node.MaxUsers) && ((proxyErr == nil && len(sharedOnly(enabledOnly(proxies))) > 0) || s.canAutoProvisionDefaultProxy()) {
			return assignment.NodeID, nil
		}
	}

	availableNodes, err := s.nodeRepo.GetAvailable(ctx)
	if err != nil {
		return 0, err
	}
	if len(availableNodes) == 0 {
		return 0, errors.NewForbiddenError("当前暂无可用节点")
	}

	var (
		selectedNodeID int64
		selectedCount  int64
		found          bool
		fallbackNodeID int64
		fallbackCount  int64
		fallbackFound  bool
	)

	for _, candidate := range availableNodes {
		proxies, proxyErr := s.proxyRepo.GetByNodeID(ctx, candidate.ID)
		if proxyErr != nil {
			continue
		}

		count, countErr := s.assignmentRepo.CountByNodeID(ctx, candidate.ID)
		if countErr != nil {
			return 0, countErr
		}

		if len(sharedOnly(enabledOnly(proxies))) > 0 {
			if !found || shouldPreferAssignedNodeCandidate(candidate, count, selectedNodeID, selectedCount, availableNodes) {
				selectedNodeID = candidate.ID
				selectedCount = count
				found = true
			}
			continue
		}

		if !s.canAutoProvisionDefaultProxy() {
			continue
		}
		if !fallbackFound || shouldPreferAssignedNodeCandidate(candidate, count, fallbackNodeID, fallbackCount, availableNodes) {
			fallbackNodeID = candidate.ID
			fallbackCount = count
			fallbackFound = true
		}
	}

	if !found {
		if !fallbackFound {
			return 0, errors.NewForbiddenError("当前暂无可用节点")
		}
		selectedNodeID = fallbackNodeID
		selectedCount = fallbackCount
		found = true
	}

	if !found {
		return 0, errors.NewForbiddenError("当前暂无可用节点")
	}

	if err := s.assignmentRepo.Assign(ctx, userID, selectedNodeID); err != nil {
		return 0, err
	}

	return selectedNodeID, nil
}

func shouldPreferAssignedNodeCandidate(candidate *repository.Node, candidateCount int64, currentNodeID, currentCount int64, availableNodes []*repository.Node) bool {
	if candidate == nil {
		return false
	}
	if candidateCount != currentCount {
		return candidateCount < currentCount
	}

	current := findAvailableNodeByID(availableNodes, currentNodeID)
	if current == nil {
		return true
	}

	candidateTrafficPressure := nodeAssignmentTrafficPressure(candidate)
	currentTrafficPressure := nodeAssignmentTrafficPressure(current)
	if candidateTrafficPressure != currentTrafficPressure {
		return candidateTrafficPressure < currentTrafficPressure
	}

	candidateCapacityPressure := nodeAssignmentCapacityPressure(candidate)
	currentCapacityPressure := nodeAssignmentCapacityPressure(current)
	if candidateCapacityPressure != currentCapacityPressure {
		return candidateCapacityPressure < currentCapacityPressure
	}

	if candidate.CurrentUsers != current.CurrentUsers {
		return candidate.CurrentUsers < current.CurrentUsers
	}
	return candidate.ID < current.ID
}

func findAvailableNodeByID(nodes []*repository.Node, nodeID int64) *repository.Node {
	for _, node := range nodes {
		if node != nil && node.ID == nodeID {
			return node
		}
	}
	return nil
}

func nodeAssignmentTrafficPressure(node *repository.Node) float64 {
	if node == nil {
		return 1e9
	}
	if node.TrafficLimit <= 0 {
		return -1
	}
	threshold := node.AlertTrafficThreshold
	if threshold <= 0 || threshold > 100 {
		threshold = 100
	}
	return (float64(node.TrafficTotal) * 100 / float64(node.TrafficLimit)) / threshold
}

func nodeAssignmentCapacityPressure(node *repository.Node) float64 {
	if node == nil {
		return 1e9
	}
	if node.MaxUsers <= 0 {
		return 0
	}
	return float64(node.CurrentUsers) / float64(node.MaxUsers)
}

func (s *Service) autoProvisionDefaultProxy(ctx context.Context, userID, nodeID int64) (*repository.Proxy, error) {
	if !s.canAutoProvisionDefaultProxy() {
		return nil, nil
	}

	existing, err := s.proxyRepo.GetByUserID(ctx, userID, 10000, 0)
	if err != nil {
		return nil, err
	}
	if enabled := enabledOnly(existing); len(enabled) > 0 {
		usable, filterErr := s.filterUsableUserProxies(ctx, enabled)
		if filterErr != nil {
			return nil, filterErr
		}
		if len(usable) > 0 {
			return selectPrimaryProxy(usable), nil
		}
	}

	return s.createAutoProvisionedProxy(ctx, userID, nodeID)
}

func (s *Service) ensureAutoProvisionedProxyOnNode(ctx context.Context, userID, nodeID int64) (*repository.Proxy, error) {
	if !s.canAutoProvisionDefaultProxy() {
		return nil, nil
	}

	existing, err := s.proxyRepo.GetByUserID(ctx, userID, 10000, 0)
	if err != nil {
		return nil, err
	}

	for _, proxyModel := range enabledOnly(existing) {
		if proxyModel.NodeID != nil && *proxyModel.NodeID == nodeID {
			return proxyModel, nil
		}
	}

	return s.createAutoProvisionedProxy(ctx, userID, nodeID)
}

func (s *Service) createAutoProvisionedProxy(ctx context.Context, userID, nodeID int64) (*repository.Proxy, error) {
	node, err := s.nodeRepo.GetByID(ctx, nodeID)
	if err != nil {
		return nil, err
	}

	protocolName, protocol, err := s.selectAutoProvisionProtocol(ctx, node)
	if err != nil {
		return nil, err
	}

	port, err := s.allocateAutoProvisionPort(ctx, userID)
	if err != nil {
		return nil, err
	}

	settings := protocol.DefaultSettings()
	settings = applyAutoProvisionNodeSecurity(node, protocolName, settings)
	normalizedSettings, err := proxylib.NormalizeSettings(protocolName, settings)
	if err != nil {
		return nil, errors.NewInternalError("failed to normalize auto provisioned proxy settings", err)
	}

	proxySettings := &proxylib.Settings{
		Name:     fmt.Sprintf("%s-%s", node.Name, protocolName),
		Protocol: protocolName,
		Port:     port,
		Host:     node.Address,
		Settings: normalizedSettings,
		Enabled:  true,
		Remark:   "auto provisioned",
	}
	if err := protocol.Validate(proxySettings); err != nil {
		return nil, errors.NewInternalError("failed to validate auto provisioned proxy settings", err)
	}

	nodeRef := node.ID
	proxyModel := &repository.Proxy{
		UserID:   userID,
		NodeID:   &nodeRef,
		Name:     fmt.Sprintf("%s-%s", node.Name, protocolName),
		Protocol: protocolName,
		Port:     port,
		Host:     node.Address,
		Settings: normalizedSettings,
		Enabled:  true,
		Remark:   "auto provisioned",
	}
	if err := s.proxyRepo.Create(ctx, proxyModel); err != nil {
		return nil, err
	}

	if s.configSyncHook != nil {
		s.configSyncHook(node.ID, "entitlement_auto_provision", "auto provisioned default proxy for entitled user")
	}

	s.logger.Info("auto provisioned default proxy for entitled user",
		logger.UserID(userID),
		logger.F("proxy_id", proxyModel.ID),
		logger.F("node_id", node.ID),
		logger.F("protocol", protocolName),
		logger.F("port", port),
	)

	return proxyModel, nil
}

func (s *Service) reconcileUserAutoProvisionedProxies(ctx context.Context, proxies []*repository.Proxy) ([]*repository.Proxy, error) {
	if len(proxies) == 0 {
		return proxies, nil
	}

	reconciled := make([]*repository.Proxy, len(proxies))
	for i, proxyModel := range proxies {
		updatedProxy, err := s.reconcileUserAutoProvisionedProxy(ctx, proxyModel)
		if err != nil {
			return nil, err
		}
		reconciled[i] = updatedProxy
	}

	return reconciled, nil
}

func (s *Service) reconcileUserAutoProvisionedProxy(ctx context.Context, proxyModel *repository.Proxy) (*repository.Proxy, error) {
	if proxyModel == nil || proxyModel.NodeID == nil || proxyModel.UserID <= 0 {
		return proxyModel, nil
	}
	if !isAutoProvisionedProxy(proxyModel) {
		return proxyModel, nil
	}
	if !s.canAutoProvisionDefaultProxy() || !protocolSupportsAutoProvisionTLS(proxyModel.Protocol) {
		return proxyModel, nil
	}

	node, err := s.nodeRepo.GetByID(ctx, *proxyModel.NodeID)
	if err != nil {
		if errors.IsNotFound(err) {
			s.logger.Warn("skip reconciliation for auto provisioned proxy on missing node",
				logger.UserID(proxyModel.UserID),
				logger.F("proxy_id", proxyModel.ID),
				logger.F("node_id", *proxyModel.NodeID),
			)
			return proxyModel, nil
		}
		return nil, err
	}
	if !nodeSupportsAutoProvisionTLS(node) {
		return proxyModel, nil
	}

	settingsCopy := cloneSettings(proxyModel.Settings)
	currentSettings, normalizeErr := proxylib.NormalizeSettings(proxyModel.Protocol, settingsCopy)
	if normalizeErr != nil {
		currentSettings = settingsCopy
	}
	desiredSettings, err := proxylib.NormalizeSettings(proxyModel.Protocol, applyAutoProvisionNodeSecurity(node, proxyModel.Protocol, cloneSettings(currentSettings)))
	if err != nil {
		return nil, errors.NewInternalError("failed to normalize reconciled auto provisioned proxy settings", err)
	}
	if reflect.DeepEqual(currentSettings, desiredSettings) {
		return proxyModel, nil
	}

	protocol, ok := s.proxyManager.GetProtocol(proxyModel.Protocol)
	if !ok {
		return proxyModel, nil
	}
	if err := protocol.Validate(&proxylib.Settings{
		ID:       proxyModel.ID,
		Name:     proxyModel.Name,
		Protocol: proxyModel.Protocol,
		Port:     proxyModel.Port,
		Host:     proxyModel.Host,
		Settings: desiredSettings,
		Enabled:  proxyModel.Enabled,
		Remark:   proxyModel.Remark,
	}); err != nil {
		return nil, errors.NewInternalError("failed to validate reconciled auto provisioned proxy settings", err)
	}

	updatedProxy := *proxyModel
	updatedProxy.Settings = desiredSettings
	if err := s.proxyRepo.Update(ctx, &updatedProxy); err != nil {
		return nil, err
	}

	if s.configSyncHook != nil && updatedProxy.NodeID != nil {
		s.configSyncHook(*updatedProxy.NodeID, "entitlement_auto_provision_reconcile", "reconciled auto provisioned proxy security settings")
	}

	s.logger.Info("reconciled auto provisioned proxy security settings",
		logger.UserID(updatedProxy.UserID),
		logger.F("proxy_id", updatedProxy.ID),
		logger.F("node_id", *updatedProxy.NodeID),
		logger.F("protocol", updatedProxy.Protocol),
	)

	return &updatedProxy, nil
}

func isAutoProvisionedProxy(proxyModel *repository.Proxy) bool {
	if proxyModel == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(proxyModel.Remark)) {
	case "auto provisioned", "auto-provisioned":
		return true
	default:
		return false
	}
}

func (s *Service) canAutoProvisionDefaultProxy() bool {
	return s.proxyManager != nil && s.nodeRepo != nil
}

func (s *Service) selectAutoProvisionProtocol(ctx context.Context, node *repository.Node) (string, proxylib.Protocol, error) {
	preferredProtocols := preferredAutoProvisionProtocols(node.Protocols, s.autoProvisionProtocolPreference(ctx))
	if nodeSupportsAutoProvisionTLS(node) {
		for _, protocolName := range preferredProtocols {
			if !protocolSupportsAutoProvisionTLS(protocolName) {
				continue
			}
			protocol, ok := s.proxyManager.GetProtocol(protocolName)
			if ok {
				return protocolName, protocol, nil
			}
		}
	}

	for _, protocolName := range preferredProtocols {
		protocol, ok := s.proxyManager.GetProtocol(protocolName)
		if ok {
			return protocolName, protocol, nil
		}
	}

	return "", nil, errors.NewForbiddenError("当前暂无可用节点")
}

func (s *Service) autoProvisionProtocolPreference(ctx context.Context) []string {
	fallback := append([]string{}, defaultAutoProvisionProtocolPreference...)
	if s == nil || s.settingsService == nil {
		return fallback
	}

	settings, err := s.settingsService.GetAutoProxySettings(ctx)
	if err != nil {
		s.logger.Warn("failed to load auto proxy protocol preference; using default",
			logger.F("error", err))
		return fallback
	}
	if settings == nil || len(settings.ProtocolPriority) == 0 {
		return fallback
	}
	return append([]string{}, settings.ProtocolPriority...)
}

func (s *Service) allocateAutoProvisionPort(ctx context.Context, userID int64) (int, error) {
	totalPorts := autoProvisionPortMax - autoProvisionPortMin + 1
	start := autoProvisionPortMin + int(userID%int64(totalPorts))

	for offset := 0; offset < totalPorts; offset++ {
		port := autoProvisionPortMin + (start-autoProvisionPortMin+offset)%totalPorts
		existing, err := s.proxyRepo.GetByPort(ctx, port)
		if err != nil {
			return 0, err
		}
		if existing == nil {
			return port, nil
		}
	}

	return 0, errors.NewInternalError("failed to allocate auto provisioned proxy port", fmt.Errorf("no available ports in range %d-%d", autoProvisionPortMin, autoProvisionPortMax))
}

func preferredAutoProvisionProtocols(raw string, preference []string) []string {
	if len(preference) == 0 {
		preference = defaultAutoProvisionProtocolPreference
	}

	configured := []string{}
	seen := map[string]struct{}{}
	appendConfigured := func(name string) {
		normalized := strings.ToLower(strings.TrimSpace(name))
		if normalized == "" {
			return
		}
		if _, exists := seen[normalized]; exists {
			return
		}
		seen[normalized] = struct{}{}
		configured = append(configured, normalized)
	}

	if strings.TrimSpace(raw) != "" {
		var configuredProtocols []string
		if err := json.Unmarshal([]byte(raw), &configuredProtocols); err == nil {
			for _, protocolName := range configuredProtocols {
				appendConfigured(protocolName)
			}
		}
	}

	if len(configured) == 1 {
		return configured
	}

	allowed := map[string]bool{}
	for _, protocolName := range configured {
		allowed[protocolName] = true
	}
	hasConfiguredPreference := len(allowed) > 0

	ordered := make([]string, 0, len(preference)+len(configured))
	orderedSeen := map[string]struct{}{}
	appendPreferred := func(protocolName string) {
		normalized := strings.ToLower(strings.TrimSpace(protocolName))
		if normalized == "" {
			return
		}
		if hasConfiguredPreference && !allowed[normalized] {
			return
		}
		if _, exists := orderedSeen[normalized]; exists {
			return
		}
		orderedSeen[normalized] = struct{}{}
		ordered = append(ordered, normalized)
	}

	for _, protocolName := range preference {
		appendPreferred(protocolName)
	}

	if hasConfiguredPreference {
		for _, protocolName := range configured {
			appendPreferred(protocolName)
		}
	}

	return ordered
}

func cloneSettings(settings map[string]any) map[string]any {
	if settings == nil {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(settings))
	for key, value := range settings {
		cloned[key] = value
	}
	return cloned
}

func applyAutoProvisionNodeSecurity(node *repository.Node, protocolName string, settings map[string]any) map[string]any {
	if settings == nil {
		settings = map[string]any{}
	}

	if !nodeSupportsAutoProvisionTLS(node) || !protocolSupportsAutoProvisionTLS(protocolName) {
		return settings
	}

	settings["security"] = "tls"
	if connectHost := autoProvisionConnectHost(node); connectHost != "" {
		settings["server"] = connectHost
	}
	settings["server_name"] = node.TLSDomain
	settings["tls_domain"] = node.TLSDomain
	settings["sni"] = node.TLSDomain
	return settings
}

func autoProvisionConnectHost(node *repository.Node) string {
	if node == nil {
		return ""
	}
	if normalized := proxylib.NormalizeShareHost(node.Address); normalized != "" {
		return normalized
	}
	return strings.TrimSpace(node.TLSDomain)
}

func nodeSupportsAutoProvisionTLS(node *repository.Node) bool {
	return node != nil && node.TLSEnabled && strings.TrimSpace(node.TLSDomain) != ""
}

func protocolSupportsAutoProvisionTLS(protocolName string) bool {
	switch strings.ToLower(strings.TrimSpace(protocolName)) {
	case "vmess", "vless", "trojan":
		return true
	default:
		return false
	}
}

func (s *Service) trialConfig() *trial.Config {
	if s.trialService == nil {
		return nil
	}
	return s.trialService.GetConfig()
}

func (s *Service) filterUsableUserProxies(ctx context.Context, proxies []*repository.Proxy) ([]*repository.Proxy, error) {
	if len(proxies) == 0 || s == nil || s.nodeRepo == nil {
		return proxies, nil
	}

	usable := make([]*repository.Proxy, 0, len(proxies))
	nodeOnlineCache := make(map[int64]bool)

	for _, proxyModel := range proxies {
		if proxyModel == nil {
			continue
		}
		if proxyModel.NodeID == nil {
			usable = append(usable, proxyModel)
			continue
		}

		nodeID := *proxyModel.NodeID
		online, ok := nodeOnlineCache[nodeID]
		if !ok {
			nodeModel, err := s.nodeRepo.GetByID(ctx, nodeID)
			if err != nil {
				if errors.IsNotFound(err) {
					s.logger.Warn("skip user proxy on missing node",
						logger.UserID(proxyModel.UserID),
						logger.F("proxy_id", proxyModel.ID),
						logger.F("node_id", nodeID),
					)
					nodeOnlineCache[nodeID] = false
					continue
				}
				return nil, err
			}
			online = nodeModel != nil && nodeModel.Status == repository.NodeStatusOnline
			nodeOnlineCache[nodeID] = online
		}
		if online {
			usable = append(usable, proxyModel)
		}
	}

	return usable, nil
}

func enabledOnly(proxies []*repository.Proxy) []*repository.Proxy {
	filtered := make([]*repository.Proxy, 0, len(proxies))
	for _, proxy := range proxies {
		if proxy != nil && proxy.Enabled {
			filtered = append(filtered, proxy)
		}
	}
	return filtered
}

func sharedOnly(proxies []*repository.Proxy) []*repository.Proxy {
	filtered := make([]*repository.Proxy, 0, len(proxies))
	for _, proxy := range proxies {
		if proxy != nil && proxy.UserID == 0 {
			filtered = append(filtered, proxy)
		}
	}
	return filtered
}

func selectPrimaryProxy(proxies []*repository.Proxy) *repository.Proxy {
	if len(proxies) == 0 {
		return nil
	}

	selected := proxies[0]
	for _, proxy := range proxies[1:] {
		if proxy.ID < selected.ID {
			selected = proxy
		}
	}
	return selected
}
