// Package entitlement centralizes user access checks for portal nodes and subscriptions.
package entitlement

import (
	"context"
	"time"

	"gorm.io/gorm"

	"v/internal/commercial/trial"
	"v/internal/database/repository"
	"v/internal/logger"
	"v/pkg/errors"
)

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

// Service provides user entitlement and node assignment logic.
type Service struct {
	userRepo       repository.UserRepository
	trialRepo      repository.TrialRepository
	proxyRepo      repository.ProxyRepository
	nodeRepo       repository.NodeRepository
	assignmentRepo repository.UserNodeAssignmentRepository
	trialService   *trial.Service
	logger         logger.Logger
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

// EvaluateAccess resolves the user's effective access state.
// If the user has no active paid subscription, the first portal access auto-activates one trial when enabled.
func (s *Service) EvaluateAccess(ctx context.Context, userID int64) (*AccessState, error) {
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

	repoTrial, err := s.getOrAutoActivateTrial(ctx, userID, state.HasActiveSubscription)
	if err != nil {
		return nil, err
	}
	if repoTrial != nil {
		if repoTrial.Status == "active" && now.After(repoTrial.ExpireAt) {
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
			}
		} else if repoTrial.Status == "active" && !now.After(repoTrial.ExpireAt) {
			state.Trial = repoTrial
			state.HasActiveTrial = true
			if !state.HasActiveSubscription {
				expireAt := repoTrial.ExpireAt
				state.EffectiveExpiresAt = &expireAt
				if cfg := s.trialConfig(); cfg != nil && cfg.TrafficLimit > 0 {
					state.EffectiveTrafficLimit = cfg.TrafficLimit
				}
			}
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
		return enabledOnly(proxies), state, nil
	}

	nodeID, err := s.getOrAssignNode(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	proxies, err = s.proxyRepo.GetByNodeID(ctx, nodeID)
	if err != nil {
		return nil, nil, err
	}
	proxies = enabledOnly(proxies)
	if len(proxies) == 0 {
		return nil, nil, errors.NewForbiddenError("当前暂无可用节点")
	}

	return []*repository.Proxy{selectPrimaryProxy(proxies)}, state, nil
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
	repoTrial, err := s.trialRepo.GetByUserID(ctx, userID)
	if err == nil {
		return repoTrial, nil
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
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
		if nodeErr == nil && node.Status == repository.NodeStatusOnline && (node.MaxUsers == 0 || node.CurrentUsers < node.MaxUsers) && proxyErr == nil && len(enabledOnly(proxies)) > 0 {
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
	)

	for _, candidate := range availableNodes {
		proxies, proxyErr := s.proxyRepo.GetByNodeID(ctx, candidate.ID)
		if proxyErr != nil || len(enabledOnly(proxies)) == 0 {
			continue
		}

		count, countErr := s.assignmentRepo.CountByNodeID(ctx, candidate.ID)
		if countErr != nil {
			return 0, countErr
		}

		if !found || count < selectedCount || (count == selectedCount && candidate.ID < selectedNodeID) {
			selectedNodeID = candidate.ID
			selectedCount = count
			found = true
		}
	}

	if !found {
		return 0, errors.NewForbiddenError("当前暂无可用节点")
	}

	if err := s.assignmentRepo.Assign(ctx, userID, selectedNodeID); err != nil {
		return 0, err
	}

	return selectedNodeID, nil
}

func (s *Service) trialConfig() *trial.Config {
	if s.trialService == nil {
		return nil
	}
	return s.trialService.GetConfig()
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
