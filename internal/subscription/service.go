// Package subscription provides subscription link management functionality.
package subscription

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	stdErrors "errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"v/internal/database/repository"
	"v/internal/entitlement"
	"v/internal/logger"
	proxylib "v/internal/proxy"
	subgenerators "v/internal/subscription/generators"
	"v/pkg/errors"
)

// ClientFormat represents supported subscription client formats.
type ClientFormat string

const (
	FormatV2rayN       ClientFormat = "v2rayn"
	FormatClash        ClientFormat = "clash"
	FormatClashMeta    ClientFormat = "clashmeta"
	FormatShadowrocket ClientFormat = "shadowrocket"
	FormatSurge        ClientFormat = "surge"
	FormatQuantumultX  ClientFormat = "quantumultx"
	FormatSingbox      ClientFormat = "singbox"
	FormatAuto         ClientFormat = "auto"
)

// TokenLength is the minimum length for subscription tokens (32 characters = 64 hex chars).
const TokenLength = 32

// ShortCodeLength is the length for short subscription codes.
const ShortCodeLength = 8

const maxSubscriptionTokenAttempts = 5

// ContentOptions represents options for content generation.
type ContentOptions struct {
	Protocols        []string // Filter by protocols
	Include          []int64  // Include specific proxy IDs
	Exclude          []int64  // Exclude specific proxy IDs
	RenameTemplate   string   // Custom naming template
	SubscriptionName string   // Client-facing profile/group name
}

// SubscriptionInfo represents subscription information for display.
type SubscriptionInfo struct {
	Link         string       `json:"link"`
	ShortLink    string       `json:"short_link,omitempty"`
	Token        string       `json:"token"`
	ShortCode    string       `json:"short_code,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
	LastAccessAt *time.Time   `json:"last_access_at,omitempty"`
	AccessCount  int64        `json:"access_count"`
	Formats      []FormatInfo `json:"formats"`
}

// SubscriptionUserinfo represents the traffic and expiry metadata exposed to clients.
type SubscriptionUserinfo struct {
	Upload   int64
	Download int64
	Total    int64
	Expire   int64
}

// FormatInfo represents information about a supported format.
type FormatInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Link        string `json:"link"`
	Icon        string `json:"icon,omitempty"`
}

// Service provides subscription business logic.
type Service struct {
	subscriptionRepo repository.SubscriptionRepository
	userRepo         repository.UserRepository
	proxyRepo        repository.ProxyRepository
	nodeRepo         repository.NodeRepository
	entitlement      *entitlement.Service
	logger           logger.Logger
	baseURL          string
	proxyDial        subscriptionProxyDialFunc
}

type subscriptionProxyDialFunc func(ctx context.Context, target string, timeout time.Duration) (net.Conn, error)

// NewService creates a new subscription service.
func NewService(
	subscriptionRepo repository.SubscriptionRepository,
	userRepo repository.UserRepository,
	proxyRepo repository.ProxyRepository,
	log logger.Logger,
	baseURL string,
) *Service {
	return &Service{
		subscriptionRepo: subscriptionRepo,
		userRepo:         userRepo,
		proxyRepo:        proxyRepo,
		logger:           log,
		baseURL:          baseURL,
		proxyDial:        defaultSubscriptionProxyDial,
	}
}

// WithNodeRepository injects node repository for node-aware server resolution.
func (s *Service) WithNodeRepository(nodeRepo repository.NodeRepository) *Service {
	s.nodeRepo = nodeRepo
	return s
}

// WithEntitlementService injects user entitlement logic for subscription access.
func (s *Service) WithEntitlementService(entitlementService *entitlement.Service) *Service {
	s.entitlement = entitlementService
	return s
}

// GenerateToken generates a cryptographically secure random token.
// The token is at least 32 characters (64 hex characters).
func (s *Service) GenerateToken() (string, error) {
	bytes := make([]byte, TokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateShortCode generates an 8-character alphanumeric short code.
func (s *Service) GenerateShortCode() (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	bytes := make([]byte, ShortCodeLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random short code: %w", err)
	}
	for i := range bytes {
		bytes[i] = charset[int(bytes[i])%len(charset)]
	}
	return string(bytes), nil
}

func (s *Service) generateUniqueToken(ctx context.Context) (string, error) {
	for attempt := 0; attempt < maxSubscriptionTokenAttempts; attempt++ {
		token, err := s.GenerateToken()
		if err != nil {
			return "", err
		}
		_, err = s.subscriptionRepo.GetByToken(ctx, token)
		if errors.IsNotFound(err) {
			return token, nil
		}
		if err != nil {
			return "", err
		}
	}

	return "", fmt.Errorf("failed to generate unique subscription token after %d attempts", maxSubscriptionTokenAttempts)
}

func (s *Service) generateUniqueShortCode(ctx context.Context) (string, error) {
	for attempt := 0; attempt < maxSubscriptionTokenAttempts; attempt++ {
		shortCode, err := s.GenerateShortCode()
		if err != nil {
			return "", err
		}
		_, err = s.subscriptionRepo.GetByShortCode(ctx, shortCode)
		if errors.IsNotFound(err) {
			return shortCode, nil
		}
		if err != nil {
			return "", err
		}
	}

	return "", fmt.Errorf("failed to generate unique subscription short code after %d attempts", maxSubscriptionTokenAttempts)
}

// ValidateToken validates a subscription token and returns the subscription if valid.
func (s *Service) ValidateToken(ctx context.Context, token string) (*repository.Subscription, error) {
	subscription, err := s.subscriptionRepo.GetByToken(ctx, token)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, errors.NewNotFoundError("subscription", token)
		}
		return nil, err
	}
	return subscription, nil
}

// ValidateShortCode validates a short code and returns the subscription if valid.
func (s *Service) ValidateShortCode(ctx context.Context, shortCode string) (*repository.Subscription, error) {
	subscription, err := s.subscriptionRepo.GetByShortCode(ctx, shortCode)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, errors.NewNotFoundError("subscription", shortCode)
		}
		return nil, err
	}
	return subscription, nil
}

// GetOrCreateSubscription gets an existing subscription or creates a new one for the user.
func (s *Service) GetOrCreateSubscription(ctx context.Context, userID int64) (*repository.Subscription, error) {
	// Try to get existing subscription
	subscription, err := s.subscriptionRepo.GetByUserID(ctx, userID)
	if err == nil {
		return subscription, nil
	}

	// If not found, create a new one
	if !errors.IsNotFound(err) {
		return nil, err
	}

	// Generate new token and short code
	token, err := s.generateUniqueToken(ctx)
	if err != nil {
		return nil, err
	}

	shortCode, err := s.generateUniqueShortCode(ctx)
	if err != nil {
		return nil, err
	}

	subscription = &repository.Subscription{
		UserID:    userID,
		Token:     token,
		ShortCode: shortCode,
	}

	if err := s.subscriptionRepo.Create(ctx, subscription); err != nil {
		return nil, err
	}

	s.logger.Info("Created new subscription for user", logger.UserID(userID))
	return subscription, nil
}

// GetSubscriptionByUserID returns an existing subscription for a user without creating one.
func (s *Service) GetSubscriptionByUserID(ctx context.Context, userID int64) (*repository.Subscription, error) {
	return s.subscriptionRepo.GetByUserID(ctx, userID)
}

// RegenerateToken regenerates the subscription token for a user.
// This invalidates the old token immediately and, because the user expressed a
// desire to "reset" the link, also clears the historic access counters and
// last-seen IP/UA so no prior footprint leaks into the new link.
func (s *Service) RegenerateToken(ctx context.Context, userID int64) (*repository.Subscription, error) {
	// Get existing subscription
	subscription, err := s.subscriptionRepo.GetByUserID(ctx, userID)
	if err != nil {
		if errors.IsNotFound(err) {
			// Create new subscription if none exists
			return s.GetOrCreateSubscription(ctx, userID)
		}
		return nil, err
	}

	// Generate new token
	newToken, err := s.generateUniqueToken(ctx)
	if err != nil {
		return nil, err
	}

	// Generate new short code
	newShortCode, err := s.generateUniqueShortCode(ctx)
	if err != nil {
		return nil, err
	}

	// Update subscription with new token + short code, and wipe access footprint.
	oldToken := subscription.Token
	subscription.Token = newToken
	subscription.ShortCode = newShortCode
	subscription.AccessCount = 0
	subscription.LastAccessAt = nil
	subscription.LastIP = ""
	subscription.LastUA = ""
	subscription.UpdatedAt = time.Now()

	if err := s.subscriptionRepo.Update(ctx, subscription); err != nil {
		return nil, err
	}

	s.logger.Info("Regenerated subscription token for user",
		logger.UserID(userID),
		logger.F("old_token_prefix", oldToken[:8]+"..."),
		logger.F("new_token_prefix", newToken[:8]+"..."),
	)

	return subscription, nil
}

// GetSubscriptionInfo returns subscription information for display.
func (s *Service) GetSubscriptionInfo(ctx context.Context, userID int64) (*SubscriptionInfo, error) {
	return s.GetSubscriptionInfoWithBaseURL(ctx, userID, "")
}

// GetSubscriptionInfoWithBaseURL returns subscription information.
//
// baseURL resolution order (highest priority first):
//  1. service.baseURL (from V_SERVER_PUBLIC_URL / V_SERVER_BASE_URL, i.e. the
//     admin-configured canonical public URL). This is what subscription clients
//     MUST reach, and may differ from the URL the admin uses to browse the
//     panel UI (common when the panel is fronted by Cloudflare / an internal
//     reverse-proxy exposing the admin UI on a different port than the
//     subscription endpoint).
//  2. baseURLOverride (derived from the current request's Host header). Used
//     as a best-effort fallback when no public URL is configured, or when the
//     configured URL points at a clearly local address (0.0.0.0, localhost,
//     loopback) — in those cases the request host is more likely to actually
//     be reachable by clients.
//  3. service.baseURL again (even if it looks local) as a last resort, so
//     the response is never empty.
//
// The OLD behaviour preferred the request host unconditionally, which baked
// the admin's browser address (e.g. `shcrystal.top:13212`) into the
// subscription URL — breaking clients when that port/host was only reachable
// through the admin path.
func (s *Service) GetSubscriptionInfoWithBaseURL(ctx context.Context, userID int64, baseURLOverride string) (*SubscriptionInfo, error) {
	subscription, err := s.GetOrCreateSubscription(ctx, userID)
	if err != nil {
		return nil, err
	}

	configured := strings.TrimSuffix(strings.TrimSpace(s.baseURL), "/")
	override := strings.TrimSuffix(strings.TrimSpace(baseURLOverride), "/")

	var baseURL string
	switch {
	case configured != "" && !isLocalBaseURL(configured):
		baseURL = configured
	case override != "":
		baseURL = override
	default:
		baseURL = configured
	}

	baseLink := fmt.Sprintf("%s/api/subscription/%s", baseURL, subscription.Token)
	shortLink := ""
	if subscription.ShortCode != "" {
		shortLink = fmt.Sprintf("%s/s/%s", baseURL, subscription.ShortCode)
	}

	formats := s.buildFormatLinks(baseLink)

	return &SubscriptionInfo{
		Link:         baseLink,
		ShortLink:    shortLink,
		Token:        subscription.Token,
		ShortCode:    subscription.ShortCode,
		CreatedAt:    subscription.CreatedAt,
		LastAccessAt: subscription.LastAccessAt,
		AccessCount:  subscription.AccessCount,
		Formats:      formats,
	}, nil
}

// isLocalBaseURL reports whether a base URL points at a loopback / wildcard /
// "empty" host and therefore isn't useful as a canonical subscription URL.
// The check is deliberately loose: we'd rather occasionally fall back to the
// request host than silently ship a 127.0.0.1-bound URL to a mobile client.
func isLocalBaseURL(rawURL string) bool {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return true
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return false
	}
	host := strings.TrimSpace(parsed.Hostname())
	switch strings.ToLower(host) {
	case "", "localhost", "127.0.0.1", "0.0.0.0", "::", "::1":
		return true
	default:
		return false
	}
}

// buildFormatLinks builds format-specific subscription links.
func (s *Service) buildFormatLinks(baseLink string) []FormatInfo {
	return []FormatInfo{
		{Name: string(FormatAuto), DisplayName: "通用订阅（自动识别）", Link: baseLink},
		{Name: string(FormatClashMeta), DisplayName: "Clash Meta/Mihomo", Link: baseLink + "?format=clashmeta"},
		{Name: string(FormatSingbox), DisplayName: "Sing-box", Link: baseLink + "?format=singbox"},
		{Name: string(FormatV2rayN), DisplayName: "V2rayN/V2rayNG", Link: baseLink + "?format=v2rayn"},
		{Name: string(FormatShadowrocket), DisplayName: "Shadowrocket", Link: baseLink + "?format=shadowrocket"},
		{Name: string(FormatClash), DisplayName: "Clash Legacy", Link: baseLink + "?format=clash"},
		{Name: string(FormatSurge), DisplayName: "Surge", Link: baseLink + "?format=surge"},
		{Name: string(FormatQuantumultX), DisplayName: "Quantumult X", Link: baseLink + "?format=quantumultx"},
	}
}

// UpdateAccessStats updates the access statistics for a subscription.
func (s *Service) UpdateAccessStats(ctx context.Context, subscriptionID int64, ip string, userAgent string) error {
	return s.subscriptionRepo.UpdateAccessStats(ctx, subscriptionID, ip, userAgent)
}

// GetSubscriptionUserinfo returns subscription traffic and expiry metadata for clients.
func (s *Service) GetSubscriptionUserinfo(ctx context.Context, userID int64) (*SubscriptionUserinfo, error) {
	if s.entitlement != nil {
		state, err := s.entitlement.EvaluateAccess(ctx, userID)
		if err != nil {
			return nil, err
		}

		userinfo := &SubscriptionUserinfo{
			Upload:   0,
			Download: normalizeSubscriptionMetric(state.EffectiveTrafficUsed),
			Total:    normalizeSubscriptionMetric(state.EffectiveTrafficLimit),
		}
		if state.EffectiveExpiresAt != nil {
			userinfo.Expire = state.EffectiveExpiresAt.UTC().Unix()
		}
		return userinfo, nil
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	userinfo := &SubscriptionUserinfo{
		Upload:   0,
		Download: normalizeSubscriptionMetric(user.TrafficUsed),
		Total:    normalizeSubscriptionMetric(user.TrafficLimit),
	}
	if user.ExpiresAt != nil {
		userinfo.Expire = user.ExpiresAt.UTC().Unix()
	}

	return userinfo, nil
}

func normalizeSubscriptionMetric(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

// CheckUserAccess checks if a user can access their subscription.
// Returns an error if the user is disabled, expired, or has exceeded traffic limits.
func (s *Service) CheckUserAccess(ctx context.Context, userID int64) error {
	if s.entitlement != nil {
		_, err := s.entitlement.EvaluateAccess(ctx, userID)
		return err
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	if !user.Enabled {
		return errors.NewForbiddenError("user account is disabled")
	}

	if user.IsExpired() {
		return errors.NewForbiddenError("user account has expired")
	}

	if user.IsTrafficExceeded() {
		return errors.NewForbiddenError("traffic limit exceeded")
	}

	return nil
}

// DetectClientFormat detects the client format from User-Agent string.
func (s *Service) DetectClientFormat(userAgent string) ClientFormat {
	return DetectClientFormat(userAgent)
}

// DetectClientFormat detects the client format from a User-Agent string.
func DetectClientFormat(userAgent string) ClientFormat {
	ua := strings.ToLower(userAgent)

	switch {
	case looksLikeClashMetaUserAgent(ua):
		return FormatClashMeta
	case strings.Contains(ua, "clash"):
		return FormatClash
	case strings.Contains(ua, "shadowrocket"):
		return FormatShadowrocket
	case strings.Contains(ua, "surge"):
		return FormatSurge
	case strings.Contains(ua, "quantumult"):
		return FormatQuantumultX
	case strings.Contains(ua, "sing-box") || strings.Contains(ua, "singbox"):
		return FormatSingbox
	case strings.Contains(ua, "v2rayn") || strings.Contains(ua, "v2rayng"):
		return FormatV2rayN
	default:
		// Prefer the modern Clash/Mihomo schema for unknown clients because it
		// supports VLESS and avoids silently producing empty legacy Clash configs.
		return FormatClashMeta
	}
}

func looksLikeClashMetaUserAgent(ua string) bool {
	for _, marker := range []string{
		"mihomo",
		"clash.meta",
		"clash-meta",
		"clash meta",
		"clashverge",
		"clash-verge",
		"clash verge",
		"stash",
		"flclash",
		"mihomo-party",
		"nikki",
	} {
		if strings.Contains(ua, marker) {
			return true
		}
	}
	return false
}

// GetUserEnabledProxies returns all enabled proxies for a user.
func (s *Service) GetUserEnabledProxies(ctx context.Context, userID int64) ([]*repository.Proxy, error) {
	proxies, err := s.getAccessibleProxies(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Filter to only enabled proxies
	var enabledProxies []*repository.Proxy
	for _, proxy := range proxies {
		if !proxy.Enabled {
			continue
		}

		settingsCopy := cloneSubscriptionProxySettings(proxy.Settings)
		resolvedServer := proxylib.ResolveExternalServerAddress(settingsCopy)
		if resolvedServer == "" && proxy.NodeID != nil && s.nodeRepo != nil {
			node, nodeErr := s.nodeRepo.GetByID(ctx, *proxy.NodeID)
			if nodeErr == nil {
				resolvedServer = proxylib.NormalizeShareHost(node.Address)
			}
		}
		if resolvedServer == "" {
			resolvedServer = proxylib.ResolveServerAddress(proxy.Host, settingsCopy)
		}
		if resolvedServer == "" {
			s.logger.Warn("skip proxy in subscription due to unresolved server address",
				logger.F("proxy_id", proxy.ID),
				logger.F("protocol", proxy.Protocol),
				logger.UserID(userID),
			)
			continue
		}
		if !s.proxyReachableEnoughForSubscription(ctx, proxy, resolvedServer) {
			s.logger.Warn("skip proxy in subscription because endpoint is unreachable",
				logger.F("proxy_id", proxy.ID),
				logger.F("protocol", proxy.Protocol),
				logger.F("server", resolvedServer),
				logger.F("port", proxy.Port),
				logger.UserID(userID),
			)
			continue
		}

		proxyCopy := *proxy
		if settingsCopy == nil {
			settingsCopy = map[string]any{}
		}
		settingsCopy["server"] = resolvedServer
		normalizeSubscriptionTLSNames(settingsCopy)
		proxyCopy.Host = resolvedServer
		proxyCopy.Settings = settingsCopy
		enabledProxies = append(enabledProxies, &proxyCopy)
	}

	return enabledProxies, nil
}

const (
	subscriptionProxyDialTimeout      = 800 * time.Millisecond
	subscriptionProxyRetryDialTimeout = 2 * time.Second
)

func (s *Service) proxyReachableEnoughForSubscription(ctx context.Context, proxy *repository.Proxy, server string) bool {
	if s == nil || s.nodeRepo == nil || proxy == nil || proxy.NodeID == nil || *proxy.NodeID <= 0 {
		return true
	}

	nodeModel, err := s.nodeRepo.GetByID(ctx, *proxy.NodeID)
	if err != nil {
		return false
	}
	if nodeReadyForSubscriptionList(nodeModel) {
		return true
	}

	port := proxylib.ResolveServerPort(proxy.Port, proxy.Settings)
	if port <= 0 {
		return false
	}
	target := net.JoinHostPort(server, strconv.Itoa(port))
	dial := s.proxyDial
	if dial == nil {
		dial = defaultSubscriptionProxyDial
	}
	reachable, dialErr := subscriptionProxyDialAttempt(ctx, dial, target, subscriptionProxyDialTimeout)
	if reachable {
		return true
	}
	if !shouldRetrySubscriptionProxyDial(dialErr) {
		return false
	}

	reachable, _ = subscriptionProxyDialAttempt(ctx, dial, target, subscriptionProxyRetryDialTimeout)
	return reachable
}

func subscriptionProxyDialAttempt(ctx context.Context, dial subscriptionProxyDialFunc, target string, timeout time.Duration) (bool, error) {
	conn, dialErr := dial(ctx, target, timeout)
	if dialErr != nil {
		return false, dialErr
	}
	_ = conn.Close()
	return true, nil
}

func shouldRetrySubscriptionProxyDial(err error) bool {
	if err == nil {
		return false
	}
	if stdErrors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return stdErrors.As(err, &netErr) && netErr.Timeout()
}

func defaultSubscriptionProxyDial(ctx context.Context, target string, timeout time.Duration) (net.Conn, error) {
	dialer := net.Dialer{Timeout: timeout}
	return dialer.DialContext(ctx, "tcp", target)
}

func nodeReadyForSubscriptionList(node *repository.Node) bool {
	if node == nil || node.Status != repository.NodeStatusOnline {
		return false
	}
	return strings.TrimSpace(node.SyncStatus) != repository.NodeSyncStatusFailed
}

func normalizeSubscriptionTLSNames(settings map[string]any) {
	if settings == nil {
		return
	}

	serverName := proxylib.ResolveSNI(settings)
	if serverName == "" {
		return
	}

	settings["sni"] = serverName
	settings["server_name"] = serverName
}

func cloneSubscriptionProxySettings(settings map[string]any) map[string]any {
	if len(settings) == 0 {
		return nil
	}

	cloned := make(map[string]any, len(settings))
	for key, value := range settings {
		cloned[key] = value
	}
	return cloned
}

func (s *Service) getAccessibleProxies(ctx context.Context, userID int64) ([]*repository.Proxy, error) {
	if s.entitlement != nil {
		proxies, _, err := s.entitlement.GetSubscriptionProxies(ctx, userID)
		return proxies, err
	}

	proxies, err := s.proxyRepo.GetByUserID(ctx, userID, 10000, 0)
	if err != nil {
		return nil, err
	}
	if len(proxies) > 0 {
		return proxies, nil
	}

	// Shared-node deployments expose global enabled proxies rather than per-user copies.
	return s.proxyRepo.GetEnabled(ctx)
}

// FilterProxies filters proxies based on content options.
func (s *Service) FilterProxies(proxies []*repository.Proxy, options *ContentOptions) []*repository.Proxy {
	if options == nil {
		return proxies
	}

	var filtered []*repository.Proxy

	for _, proxy := range proxies {
		// Check protocol filter
		if len(options.Protocols) > 0 {
			found := false
			for _, p := range options.Protocols {
				if strings.EqualFold(proxy.Protocol, p) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Check include filter
		if len(options.Include) > 0 {
			found := false
			for _, id := range options.Include {
				if proxy.ID == id {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Check exclude filter
		if len(options.Exclude) > 0 {
			excluded := false
			for _, id := range options.Exclude {
				if proxy.ID == id {
					excluded = true
					break
				}
			}
			if excluded {
				continue
			}
		}

		filtered = append(filtered, proxy)
	}

	return filtered
}

// RevokeSubscription revokes a user's subscription (admin function).
func (s *Service) RevokeSubscription(ctx context.Context, userID int64) error {
	err := s.subscriptionRepo.DeleteByUserID(ctx, userID)
	if err != nil {
		return err
	}
	s.logger.Info("Revoked subscription for user", logger.UserID(userID))
	return nil
}

// ResetAccessStats resets the access statistics for a subscription (admin function).
func (s *Service) ResetAccessStats(ctx context.Context, subscriptionID int64) error {
	return s.subscriptionRepo.ResetAccessStats(ctx, subscriptionID)
}

// ListAllSubscriptions lists all subscriptions with optional filtering (admin function).
func (s *Service) ListAllSubscriptions(ctx context.Context, filter *repository.SubscriptionFilter) ([]*repository.Subscription, int64, error) {
	return s.subscriptionRepo.ListAll(ctx, filter)
}

// GenerateContent generates subscription content for a user in the specified format.
func (s *Service) GenerateContent(ctx context.Context, userID int64, format ClientFormat, options *ContentOptions) ([]byte, string, string, error) {
	// Get user's enabled proxies
	proxies, err := s.GetUserEnabledProxies(ctx, userID)
	if err != nil {
		return nil, "", "", err
	}

	// Apply filters
	proxies = s.FilterProxies(proxies, options)
	if len(proxies) == 0 {
		return nil, "", "", errors.NewValidationError(
			"当前订阅没有可用节点，请确认套餐有效、节点在线，且订阅筛选条件没有排除全部节点",
			map[string]any{
				"user_id": userID,
				"format":  string(format),
			},
		)
	}

	// Import generators
	generator, contentType, fileExt := s.getGenerator(format)
	if generator == nil {
		return nil, "", "", fmt.Errorf("unsupported format: %s", format)
	}
	if err := ensureFormatCanEmitProxy(format, proxies, generator); err != nil {
		return nil, "", "", err
	}

	// Generate content
	content, err := generator.Generate(proxies, s.buildGeneratorOptions(options))
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to generate content: %w", err)
	}

	return content, contentType, fileExt, nil
}

func ensureFormatCanEmitProxy(format ClientFormat, proxies []*repository.Proxy, generator subgenerators.FormatGenerator) error {
	if len(proxies) == 0 || generator == nil {
		return nil
	}

	for _, proxy := range proxies {
		if proxy == nil {
			continue
		}
		if generator.SupportsProtocol(proxy.Protocol) {
			return nil
		}
	}

	protocols := subscriptionProxyProtocolList(proxies)
	formatName := subscriptionFormatName(format)
	return errors.NewValidationError(
		fmt.Sprintf("当前订阅格式 %s 不支持现有节点协议（%s），请切换到 Clash Meta/Mihomo、Sing-box、V2RayN 或 Shadowrocket", formatName, protocols),
		map[string]any{
			"format":    string(format),
			"protocols": protocols,
		},
	)
}

func subscriptionProxyProtocolList(proxies []*repository.Proxy) string {
	seen := make(map[string]bool)
	protocols := make([]string, 0)
	for _, proxy := range proxies {
		if proxy == nil {
			continue
		}
		protocol := strings.ToUpper(strings.TrimSpace(proxy.Protocol))
		if protocol == "" || seen[protocol] {
			continue
		}
		seen[protocol] = true
		protocols = append(protocols, protocol)
	}
	if len(protocols) == 0 {
		return "未知协议"
	}
	return strings.Join(protocols, "、")
}

func subscriptionFormatName(format ClientFormat) string {
	switch format {
	case FormatClash:
		return "Clash Legacy/Clash for Windows"
	case FormatClashMeta:
		return "Clash Meta/Mihomo"
	case FormatSingbox:
		return "Sing-box"
	case FormatV2rayN:
		return "V2RayN/V2RayNG"
	case FormatShadowrocket:
		return "Shadowrocket"
	case FormatSurge:
		return "Surge"
	case FormatQuantumultX:
		return "Quantumult X"
	default:
		return string(format)
	}
}

// getGenerator returns the appropriate generator for the format.
func (s *Service) getGenerator(format ClientFormat) (subgenerators.FormatGenerator, string, string) {
	switch format {
	case FormatV2rayN:
		g := subgenerators.NewV2rayNGenerator()
		return g, g.ContentType(), g.FileExtension()
	case FormatClash:
		g := subgenerators.NewClashGenerator()
		return g, g.ContentType(), g.FileExtension()
	case FormatClashMeta:
		g := subgenerators.NewClashMetaGenerator()
		return g, g.ContentType(), g.FileExtension()
	case FormatShadowrocket:
		g := subgenerators.NewShadowrocketGenerator()
		return g, g.ContentType(), g.FileExtension()
	case FormatSurge:
		g := subgenerators.NewSurgeGenerator()
		return g, g.ContentType(), g.FileExtension()
	case FormatQuantumultX:
		g := subgenerators.NewQuantumultXGenerator()
		return g, g.ContentType(), g.FileExtension()
	case FormatSingbox:
		g := subgenerators.NewSingboxGenerator()
		return g, g.ContentType(), g.FileExtension()
	default:
		g := subgenerators.NewV2rayNGenerator()
		return g, g.ContentType(), g.FileExtension()
	}
}

// buildGeneratorOptions builds generator options from content options.
func (s *Service) buildGeneratorOptions(options *ContentOptions) *subgenerators.GeneratorOptions {
	genOpts := subgenerators.DefaultOptions()

	if options != nil {
		if options.SubscriptionName != "" {
			genOpts.SubscriptionName = options.SubscriptionName
		}
		if options.RenameTemplate != "" {
			genOpts.RenameTemplate = options.RenameTemplate
		}
	}

	return genOpts
}
