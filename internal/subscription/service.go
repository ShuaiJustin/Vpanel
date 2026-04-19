// Package subscription provides subscription link management functionality.
package subscription

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"v/internal/database/repository"
	"v/internal/entitlement"
	"v/internal/logger"
	proxylib "v/internal/proxy"
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
	Protocols      []string // Filter by protocols
	Include        []int64  // Include specific proxy IDs
	Exclude        []int64  // Exclude specific proxy IDs
	RenameTemplate string   // Custom naming template
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
}

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

// GetSubscriptionInfoWithBaseURL returns subscription information using request-aware base URL when provided.
func (s *Service) GetSubscriptionInfoWithBaseURL(ctx context.Context, userID int64, baseURLOverride string) (*SubscriptionInfo, error) {
	subscription, err := s.GetOrCreateSubscription(ctx, userID)
	if err != nil {
		return nil, err
	}

	baseURL := strings.TrimSuffix(strings.TrimSpace(baseURLOverride), "/")
	if baseURL == "" {
		baseURL = strings.TrimSuffix(strings.TrimSpace(s.baseURL), "/")
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

// buildFormatLinks builds format-specific subscription links.
func (s *Service) buildFormatLinks(baseLink string) []FormatInfo {
	return []FormatInfo{
		{Name: string(FormatV2rayN), DisplayName: "V2rayN/V2rayNG", Link: baseLink + "?format=v2rayn"},
		{Name: string(FormatClash), DisplayName: "Clash", Link: baseLink + "?format=clash"},
		{Name: string(FormatClashMeta), DisplayName: "Clash Meta", Link: baseLink + "?format=clashmeta"},
		{Name: string(FormatShadowrocket), DisplayName: "Shadowrocket", Link: baseLink + "?format=shadowrocket"},
		{Name: string(FormatSurge), DisplayName: "Surge", Link: baseLink + "?format=surge"},
		{Name: string(FormatQuantumultX), DisplayName: "Quantumult X", Link: baseLink + "?format=quantumultx"},
		{Name: string(FormatSingbox), DisplayName: "Sing-box", Link: baseLink + "?format=singbox"},
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
	ua := strings.ToLower(userAgent)

	switch {
	case strings.Contains(ua, "clash"):
		if strings.Contains(ua, "meta") {
			return FormatClashMeta
		}
		return FormatClash
	case strings.Contains(ua, "mihomo"):
		return FormatClashMeta
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
		// Default to V2rayN format for unknown clients
		return FormatV2rayN
	}
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

		proxyCopy := *proxy
		if settingsCopy == nil {
			settingsCopy = map[string]any{}
		}
		settingsCopy["server"] = resolvedServer
		proxyCopy.Settings = settingsCopy
		enabledProxies = append(enabledProxies, &proxyCopy)
	}

	return enabledProxies, nil
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

	// Import generators
	generator, contentType, fileExt := s.getGenerator(format)
	if generator == nil {
		return nil, "", "", fmt.Errorf("unsupported format: %s", format)
	}

	// Generate content
	content, err := generator.Generate(proxies, s.buildGeneratorOptions(options))
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to generate content: %w", err)
	}

	return content, contentType, fileExt, nil
}

// FormatGenerator defines the interface for subscription format generators.
type FormatGenerator interface {
	Generate(proxies []*repository.Proxy, options *GeneratorOptions) ([]byte, error)
	ContentType() string
	FileExtension() string
	SupportsProtocol(protocol string) bool
}

// GeneratorOptions represents options for content generation.
type GeneratorOptions struct {
	SubscriptionName   string
	ServerHost         string
	RenameTemplate     string
	IncludeProxyGroups bool
	UpdateInterval     int
}

// getGenerator returns the appropriate generator for the format.
func (s *Service) getGenerator(format ClientFormat) (FormatGenerator, string, string) {
	// Lazy initialization of generators would be better in production
	// For now, we create them on demand
	switch format {
	case FormatV2rayN:
		g := &v2rayNGenerator{}
		return g, g.ContentType(), g.FileExtension()
	case FormatClash:
		g := &clashGenerator{}
		return g, g.ContentType(), g.FileExtension()
	case FormatClashMeta:
		g := &clashMetaGenerator{}
		return g, g.ContentType(), g.FileExtension()
	case FormatShadowrocket:
		g := &shadowrocketGenerator{}
		return g, g.ContentType(), g.FileExtension()
	case FormatSurge:
		g := &surgeGenerator{}
		return g, g.ContentType(), g.FileExtension()
	case FormatQuantumultX:
		g := &quantumultXGenerator{}
		return g, g.ContentType(), g.FileExtension()
	case FormatSingbox:
		g := &singboxGenerator{}
		return g, g.ContentType(), g.FileExtension()
	default:
		// Default to V2rayN
		g := &v2rayNGenerator{}
		return g, g.ContentType(), g.FileExtension()
	}
}

// buildGeneratorOptions builds generator options from content options.
func (s *Service) buildGeneratorOptions(options *ContentOptions) *GeneratorOptions {
	genOpts := &GeneratorOptions{
		SubscriptionName:   "V Panel Subscription",
		IncludeProxyGroups: true,
		UpdateInterval:     24,
	}

	if options != nil && options.RenameTemplate != "" {
		genOpts.RenameTemplate = options.RenameTemplate
	}

	return genOpts
}

// Inline generator implementations that delegate to the generators package
// These are thin wrappers to avoid circular imports

type v2rayNGenerator struct{}

func (g *v2rayNGenerator) Generate(proxies []*repository.Proxy, options *GeneratorOptions) ([]byte, error) {
	return generateV2rayN(proxies, options)
}
func (g *v2rayNGenerator) ContentType() string            { return "text/plain; charset=utf-8" }
func (g *v2rayNGenerator) FileExtension() string          { return "txt" }
func (g *v2rayNGenerator) SupportsProtocol(p string) bool { return true }

type clashGenerator struct{}

func (g *clashGenerator) Generate(proxies []*repository.Proxy, options *GeneratorOptions) ([]byte, error) {
	return generateClash(proxies, options)
}
func (g *clashGenerator) ContentType() string            { return "text/yaml; charset=utf-8" }
func (g *clashGenerator) FileExtension() string          { return "yaml" }
func (g *clashGenerator) SupportsProtocol(p string) bool { return true }

type clashMetaGenerator struct{}

func (g *clashMetaGenerator) Generate(proxies []*repository.Proxy, options *GeneratorOptions) ([]byte, error) {
	return generateClashMeta(proxies, options)
}
func (g *clashMetaGenerator) ContentType() string            { return "text/yaml; charset=utf-8" }
func (g *clashMetaGenerator) FileExtension() string          { return "yaml" }
func (g *clashMetaGenerator) SupportsProtocol(p string) bool { return true }

type shadowrocketGenerator struct{}

func (g *shadowrocketGenerator) Generate(proxies []*repository.Proxy, options *GeneratorOptions) ([]byte, error) {
	return generateShadowrocket(proxies, options)
}
func (g *shadowrocketGenerator) ContentType() string            { return "text/plain; charset=utf-8" }
func (g *shadowrocketGenerator) FileExtension() string          { return "txt" }
func (g *shadowrocketGenerator) SupportsProtocol(p string) bool { return true }

type surgeGenerator struct{}

func (g *surgeGenerator) Generate(proxies []*repository.Proxy, options *GeneratorOptions) ([]byte, error) {
	return generateSurge(proxies, options)
}
func (g *surgeGenerator) ContentType() string            { return "text/plain; charset=utf-8" }
func (g *surgeGenerator) FileExtension() string          { return "conf" }
func (g *surgeGenerator) SupportsProtocol(p string) bool { return true }

type quantumultXGenerator struct{}

func (g *quantumultXGenerator) Generate(proxies []*repository.Proxy, options *GeneratorOptions) ([]byte, error) {
	return generateQuantumultX(proxies, options)
}
func (g *quantumultXGenerator) ContentType() string            { return "text/plain; charset=utf-8" }
func (g *quantumultXGenerator) FileExtension() string          { return "conf" }
func (g *quantumultXGenerator) SupportsProtocol(p string) bool { return true }

type singboxGenerator struct{}

func (g *singboxGenerator) Generate(proxies []*repository.Proxy, options *GeneratorOptions) ([]byte, error) {
	return generateSingbox(proxies, options)
}
func (g *singboxGenerator) ContentType() string            { return "application/json; charset=utf-8" }
func (g *singboxGenerator) FileExtension() string          { return "json" }
func (g *singboxGenerator) SupportsProtocol(p string) bool { return true }
