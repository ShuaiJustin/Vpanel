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
	"v/pkg/errors"
)

const (
	autoProvisionPortMin = 20000
	autoProvisionPortMax = 60000
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
	proxyManager   proxylib.Manager
	configSyncHook func(nodeID int64, source, reason string)
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

// WithProxyManager enables automatic default proxy provisioning.
func (s *Service) WithProxyManager(proxyManager proxylib.Manager) *Service {
	s.proxyManager = proxyManager
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
		proxies, err = s.reconcileUserAutoProvisionedProxies(ctx, proxies)
		if err != nil {
			return nil, nil, err
		}
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
			if !found || count < selectedCount || (count == selectedCount && candidate.ID < selectedNodeID) {
				selectedNodeID = candidate.ID
				selectedCount = count
				found = true
			}
			continue
		}

		if !s.canAutoProvisionDefaultProxy() {
			continue
		}
		if !fallbackFound || count < fallbackCount || (count == fallbackCount && candidate.ID < fallbackNodeID) {
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

func (s *Service) autoProvisionDefaultProxy(ctx context.Context, userID, nodeID int64) (*repository.Proxy, error) {
	if !s.canAutoProvisionDefaultProxy() {
		return nil, nil
	}

	existing, err := s.proxyRepo.GetByUserID(ctx, userID, 10000, 0)
	if err != nil {
		return nil, err
	}
	if enabled := enabledOnly(existing); len(enabled) > 0 {
		return selectPrimaryProxy(enabled), nil
	}

	node, err := s.nodeRepo.GetByID(ctx, nodeID)
	if err != nil {
		return nil, err
	}

	protocolName, protocol, err := s.selectAutoProvisionProtocol(node)
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
	if strings.ToLower(strings.TrimSpace(proxyModel.Remark)) != "auto provisioned" {
		return proxyModel, nil
	}
	if !s.canAutoProvisionDefaultProxy() || !protocolSupportsAutoProvisionTLS(proxyModel.Protocol) {
		return proxyModel, nil
	}

	node, err := s.nodeRepo.GetByID(ctx, *proxyModel.NodeID)
	if err != nil {
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

func (s *Service) canAutoProvisionDefaultProxy() bool {
	return s.proxyManager != nil && s.nodeRepo != nil
}

func (s *Service) selectAutoProvisionProtocol(node *repository.Node) (string, proxylib.Protocol, error) {
	preferredProtocols := preferredAutoProvisionProtocols(node.Protocols)
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

func preferredAutoProvisionProtocols(raw string) []string {
	ordered := []string{}
	seen := map[string]struct{}{}
	appendProtocol := func(name string) {
		normalized := strings.ToLower(strings.TrimSpace(name))
		if normalized == "" {
			return
		}
		if _, exists := seen[normalized]; exists {
			return
		}
		seen[normalized] = struct{}{}
		ordered = append(ordered, normalized)
	}

	if strings.TrimSpace(raw) != "" {
		var configured []string
		if err := json.Unmarshal([]byte(raw), &configured); err == nil {
			for _, protocolName := range configured {
				appendProtocol(protocolName)
			}
		}
	}

	for _, protocolName := range []string{"vmess", "vless", "trojan", "shadowsocks"} {
		appendProtocol(protocolName)
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
