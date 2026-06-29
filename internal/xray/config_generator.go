// Package xray provides Xray configuration generation and management.
package xray

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"v/internal/database/repository"
	"v/internal/logger"
	"v/internal/node"
	apperrors "v/pkg/errors"
)

const (
	apiInboundPortBase       = 63000
	apiInboundPortSpan       = 2048
	defaultXrayAccessLogPath = "/tmp/vpanel-xray-access.log"
)

// ConfigGenerator generates Xray configurations for nodes.
type ConfigGenerator struct {
	proxyRepo       repository.ProxyRepository
	certRepo        repository.CertificateRepository
	nodeRepo        repository.NodeRepository
	userAccessCheck func(context.Context, int64) error
	logger          logger.Logger
}

// NewConfigGenerator creates a new Xray config generator.
func NewConfigGenerator(
	proxyRepo repository.ProxyRepository,
	certRepo repository.CertificateRepository,
	nodeRepo repository.NodeRepository,
	log logger.Logger,
) *ConfigGenerator {
	return &ConfigGenerator{
		proxyRepo: proxyRepo,
		certRepo:  certRepo,
		nodeRepo:  nodeRepo,
		logger:    log,
	}
}

// WithUserAccessCheck registers an optional access check for user-owned proxies.
// Shared proxies (user_id = 0) are never filtered by this hook.
func (g *ConfigGenerator) WithUserAccessCheck(check func(context.Context, int64) error) *ConfigGenerator {
	g.userAccessCheck = check
	return g
}

// XrayConfig represents the complete Xray configuration.
type XrayConfig struct {
	Log       *LogConfig       `json:"log"`
	API       *APIConfig       `json:"api,omitempty"`
	Stats     *StatsConfig     `json:"stats,omitempty"`
	Policy    *PolicyConfig    `json:"policy,omitempty"`
	Inbounds  []InboundConfig  `json:"inbounds"`
	Outbounds []OutboundConfig `json:"outbounds"`
	Routing   *RoutingConfig   `json:"routing,omitempty"`
}

// LogConfig represents Xray log configuration.
type LogConfig struct {
	LogLevel string `json:"loglevel"`
	Access   string `json:"access"`
	Error    string `json:"error"`
}

// APIConfig represents Xray API configuration.
type APIConfig struct {
	Tag      string   `json:"tag"`
	Services []string `json:"services"`
}

// StatsConfig represents Xray stats configuration.
type StatsConfig struct{}

// PolicyConfig represents Xray policy configuration.
type PolicyConfig struct {
	Levels map[string]*PolicyLevel `json:"levels,omitempty"`
	System *SystemPolicy           `json:"system,omitempty"`
}

// PolicyLevel represents policy for a specific level.
type PolicyLevel struct {
	StatsUserUplink   bool `json:"statsUserUplink"`
	StatsUserDownlink bool `json:"statsUserDownlink"`
}

// SystemPolicy represents system-wide policy.
type SystemPolicy struct {
	StatsInboundUplink    bool `json:"statsInboundUplink"`
	StatsInboundDownlink  bool `json:"statsInboundDownlink"`
	StatsOutboundUplink   bool `json:"statsOutboundUplink"`
	StatsOutboundDownlink bool `json:"statsOutboundDownlink"`
}

// InboundConfig represents an Xray inbound configuration.
type InboundConfig struct {
	Tag            string          `json:"tag"`
	Listen         string          `json:"listen,omitempty"`
	Port           int             `json:"port"`
	Protocol       string          `json:"protocol"`
	Settings       map[string]any  `json:"settings"`
	StreamSettings *StreamSettings `json:"streamSettings,omitempty"`
	Sniffing       *SniffingConfig `json:"sniffing,omitempty"`
}

// StreamSettings represents stream settings for transport.
type StreamSettings struct {
	Network         string         `json:"network"`
	Security        string         `json:"security,omitempty"`
	TLSSettings     *TLSSettings   `json:"tlsSettings,omitempty"`
	RealitySettings map[string]any `json:"realitySettings,omitempty"`
	Sockopt         map[string]any `json:"sockopt,omitempty"`
	TCPSettings     map[string]any `json:"tcpSettings,omitempty"`
	WSSettings      map[string]any `json:"wsSettings,omitempty"`
	HTTPSettings    map[string]any `json:"httpSettings,omitempty"`
	QUICSettings    map[string]any `json:"quicSettings,omitempty"`
	GRPCSettings    map[string]any `json:"grpcSettings,omitempty"`
}

// TLSSettings represents TLS configuration.
type TLSSettings struct {
	ServerName   string        `json:"serverName,omitempty"`
	Certificates []Certificate `json:"certificates,omitempty"`
	ALPN         []string      `json:"alpn,omitempty"`
}

// Certificate represents a TLS certificate.
type Certificate struct {
	CertificateFile string   `json:"certificateFile,omitempty"`
	KeyFile         string   `json:"keyFile,omitempty"`
	Certificate     []string `json:"certificate,omitempty"`
	Key             []string `json:"key,omitempty"`
}

// SniffingConfig represents sniffing configuration.
type SniffingConfig struct {
	Enabled      bool     `json:"enabled"`
	DestOverride []string `json:"destOverride"`
}

// OutboundConfig represents an Xray outbound configuration.
type OutboundConfig struct {
	Tag      string         `json:"tag"`
	Protocol string         `json:"protocol"`
	Settings map[string]any `json:"settings"`
}

// RoutingConfig represents Xray routing configuration.
type RoutingConfig struct {
	Rules []RoutingRule `json:"rules"`
}

// RoutingRule represents a routing rule.
type RoutingRule struct {
	Type        string   `json:"type"`
	InboundTag  []string `json:"inboundTag,omitempty"`
	OutboundTag string   `json:"outboundTag"`
	Protocol    []string `json:"protocol,omitempty"`
}

// GenerateForNode generates Xray configuration for a specific node.
func (g *ConfigGenerator) GenerateForNode(ctx context.Context, nodeID int64) (*XrayConfig, error) {
	// Get all enabled proxies for users assigned to this node
	allProxies, err := g.proxyRepo.GetByNodeID(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get proxies for node: %w", err)
	}
	allProxies = g.filterAccessibleProxies(ctx, allProxies)

	var (
		optimizationSettings node.NetworkOptimizationSettings
		nodeData             *repository.Node
	)
	if g.nodeRepo != nil {
		loadedNode, nodeErr := g.nodeRepo.GetByID(ctx, nodeID)
		if nodeErr != nil {
			g.logger.Warn("failed to load node optimization settings",
				logger.F("node_id", nodeID),
				logger.F("error", nodeErr.Error()))
		} else if loadedNode != nil {
			nodeData = loadedNode
			optimizationSettings = node.ParseNetworkOptimizationSettings(nodeData.NetworkOptimizationSettings)
		}
	}

	if !shouldServeNodeProxyInbounds(nodeData) {
		g.logger.Warn("node is not serving proxy inbounds in generated config",
			logger.F("node_id", nodeID),
			logger.F("status", nodeStatusValue(nodeData)),
			logger.F("traffic_total", nodeTrafficTotalValue(nodeData)),
			logger.F("traffic_limit", nodeTrafficLimitValue(nodeData)))
		allProxies = nil
	}

	g.logger.Info("generating config for node",
		logger.F("node_id", nodeID),
		logger.F("proxy_count", len(allProxies)))

	// Generate configuration
	config := &XrayConfig{
		Log: &LogConfig{
			LogLevel: "warning",
			Access:   defaultXrayAccessLogPath,
			Error:    "",
		},
		API: &APIConfig{
			Tag:      "api",
			Services: []string{"HandlerService", "LoggerService", "StatsService"},
		},
		Stats: &StatsConfig{},
		Policy: &PolicyConfig{
			Levels: map[string]*PolicyLevel{
				"0": {
					StatsUserUplink:   true,
					StatsUserDownlink: true,
				},
			},
			System: &SystemPolicy{
				StatsInboundUplink:    true,
				StatsInboundDownlink:  true,
				StatsOutboundUplink:   true,
				StatsOutboundDownlink: true,
			},
		},
		Inbounds:  g.generateInbounds(ctx, nodeID, allProxies, optimizationSettings),
		Outbounds: g.generateOutbounds(),
		Routing:   g.generateRouting(),
	}

	return config, nil
}

func shouldServeNodeProxyInbounds(nodeData *repository.Node) bool {
	if nodeData == nil {
		return true
	}
	if nodeData.TrafficLimit > 0 && nodeData.TrafficTotal >= nodeData.TrafficLimit {
		return false
	}
	if nodeData.Status == repository.NodeStatusOffline {
		return false
	}
	return true
}

func nodeStatusValue(nodeData *repository.Node) string {
	if nodeData == nil {
		return "unknown"
	}
	return strings.TrimSpace(nodeData.Status)
}

func nodeTrafficTotalValue(nodeData *repository.Node) int64 {
	if nodeData == nil {
		return 0
	}
	return nodeData.TrafficTotal
}

func nodeTrafficLimitValue(nodeData *repository.Node) int64 {
	if nodeData == nil {
		return 0
	}
	return nodeData.TrafficLimit
}

func (g *ConfigGenerator) filterAccessibleProxies(ctx context.Context, proxies []*repository.Proxy) []*repository.Proxy {
	if len(proxies) == 0 || g.userAccessCheck == nil {
		return proxies
	}

	filtered := make([]*repository.Proxy, 0, len(proxies))
	accessCache := make(map[int64]error)

	for _, proxy := range proxies {
		if proxy == nil {
			continue
		}
		if proxy.UserID == 0 {
			filtered = append(filtered, proxy)
			continue
		}

		accessErr, checked := accessCache[proxy.UserID]
		if !checked {
			accessErr = g.userAccessCheck(ctx, proxy.UserID)
			accessCache[proxy.UserID] = accessErr
		}

		if accessErr == nil {
			filtered = append(filtered, proxy)
			continue
		}

		if apperrors.IsForbidden(accessErr) {
			g.logger.Debug("skipping inaccessible user proxy from node config",
				logger.UserID(proxy.UserID),
				logger.F("proxy_id", proxy.ID),
			)
			continue
		}

		g.logger.Warn("skipping proxy after access check failure",
			logger.Err(accessErr),
			logger.UserID(proxy.UserID),
			logger.F("proxy_id", proxy.ID),
		)
	}

	return filtered
}

// generateInbounds generates inbound configurations from proxies.
func (g *ConfigGenerator) generateInbounds(ctx context.Context, nodeID int64, proxies []*repository.Proxy, optimizationSettings node.NetworkOptimizationSettings) []InboundConfig {
	apiPort := resolveAPIInboundPort(nodeID, proxies)

	inbounds := []InboundConfig{
		// API inbound for stats
		{
			Tag:      "api",
			Listen:   "127.0.0.1",
			Port:     apiPort,
			Protocol: "dokodemo-door",
			Settings: map[string]any{
				"address": "127.0.0.1",
			},
		},
	}

	// Generate inbound for each proxy
	for _, proxy := range proxies {
		inbound := g.proxyToInbound(ctx, proxy, optimizationSettings)
		if inbound != nil {
			inbounds = append(inbounds, *inbound)
		}
	}

	return inbounds
}

func resolveAPIInboundPort(nodeID int64, proxies []*repository.Proxy) int {
	usedPorts := make(map[int]struct{}, len(proxies))
	for _, proxy := range proxies {
		if proxy == nil || proxy.Port <= 0 {
			continue
		}
		usedPorts[proxy.Port] = struct{}{}
	}

	startOffset := 0
	if nodeID > 0 {
		startOffset = int(nodeID % apiInboundPortSpan)
	}

	for i := 0; i < apiInboundPortSpan; i++ {
		candidate := apiInboundPortBase + ((startOffset + i) % apiInboundPortSpan)
		if _, exists := usedPorts[candidate]; exists {
			continue
		}
		return candidate
	}

	return apiInboundPortBase
}

// proxyToInbound converts a proxy to an Xray inbound configuration.
func (g *ConfigGenerator) proxyToInbound(ctx context.Context, proxy *repository.Proxy, optimizationSettings node.NetworkOptimizationSettings) *InboundConfig {
	// Validate port range
	if proxy.Port < 1 || proxy.Port > 65535 {
		g.logger.Warn("Invalid proxy port, skipping",
			logger.F("proxy_id", proxy.ID),
			logger.F("port", proxy.Port))
		return nil
	}

	// Validate protocol
	validProtocols := map[string]bool{
		"vless": true, "vmess": true, "trojan": true, "shadowsocks": true,
	}
	if !validProtocols[proxy.Protocol] {
		g.logger.Warn("Unsupported protocol, skipping",
			logger.F("proxy_id", proxy.ID),
			logger.F("protocol", proxy.Protocol))
		return nil
	}

	tag := fmt.Sprintf("inbound-%d", proxy.ID)

	inbound := &InboundConfig{
		Tag:      tag,
		Port:     proxy.Port,
		Protocol: proxy.Protocol,
		Settings: make(map[string]any),
		Sniffing: &SniffingConfig{
			Enabled:      true,
			DestOverride: []string{"http", "tls"},
		},
	}

	// Extract settings from proxy
	settings := proxy.Settings
	if settings == nil {
		settings = make(map[string]any)
	}
	settings = normalizeInboundSettings(proxy.Protocol, settings)

	switch proxy.Protocol {
	case "vless":
		inbound.Settings = g.generateVLESSSettings(proxy, settings)
		inbound.StreamSettings = g.generateStreamSettings(ctx, settings, optimizationSettings)
	case "vmess":
		inbound.Settings = g.generateVMessSettings(proxy, settings)
		inbound.StreamSettings = g.generateStreamSettings(ctx, settings, optimizationSettings)
	case "trojan":
		inbound.Settings = g.generateTrojanSettings(proxy, settings)
		inbound.StreamSettings = g.generateStreamSettings(ctx, settings, optimizationSettings)
	case "shadowsocks":
		inbound.Settings = g.generateShadowsocksSettings(proxy, settings)
	}

	return inbound
}

func normalizeInboundSettings(protocol string, settings map[string]any) map[string]any {
	normalized := make(map[string]any, len(settings)+2)
	for key, value := range settings {
		normalized[key] = value
	}

	protocol = strings.ToLower(strings.TrimSpace(protocol))
	security := strings.ToLower(strings.TrimSpace(getStringSetting(normalized, "security")))
	if protocol == "vmess" && security != "" && !isInboundStreamSecurity(security) {
		if getStringSetting(normalized, "cipher") == "" && getStringSetting(normalized, "scy") == "" {
			normalized["cipher"] = security
			normalized["scy"] = security
		}
		security = ""
	}
	if security == "" {
		if protocol == "trojan" || hasInboundTLSIntent(normalized) {
			security = "tls"
		}
	}
	if security != "" {
		normalized["security"] = security
	}
	if security == "none" {
		delete(normalized, "tls")
	}

	return normalized
}

func isInboundStreamSecurity(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "none", "tls", "xtls", "reality":
		return true
	default:
		return false
	}
}

func hasInboundTLSIntent(settings map[string]any) bool {
	switch typed := settings["tls"].(type) {
	case bool:
		if typed {
			return true
		}
	case string:
		normalized := strings.ToLower(strings.TrimSpace(typed))
		if normalized == "tls" || normalized == "true" || normalized == "1" {
			return true
		}
	}
	return getStringSetting(settings, "sni") != "" || getStringSetting(settings, "server_name") != "" || getStringSetting(settings, "tls_domain") != ""
}

// generateVLESSSettings generates VLESS protocol settings.
func (g *ConfigGenerator) generateVLESSSettings(proxy *repository.Proxy, settings map[string]any) map[string]any {
	clients := []map[string]any{}

	// Extract UUID from settings
	if uuid, ok := settings["uuid"].(string); ok && uuid != "" {
		clients = append(clients, map[string]any{
			"id":    uuid,
			"email": fmt.Sprintf("user-%d-proxy-%d", proxy.UserID, proxy.ID),
			"level": 0,
		})
	}

	return map[string]any{
		"clients":    clients,
		"decryption": "none",
		"fallbacks":  []map[string]any{},
	}
}

// generateVMessSettings generates VMess protocol settings.
func (g *ConfigGenerator) generateVMessSettings(proxy *repository.Proxy, settings map[string]any) map[string]any {
	clients := []map[string]any{}

	// Extract UUID from settings
	if uuid, ok := settings["uuid"].(string); ok && uuid != "" {
		client := map[string]any{
			"id":    uuid,
			"email": fmt.Sprintf("user-%d-proxy-%d", proxy.UserID, proxy.ID),
			"level": 0,
		}

		client["alterId"] = getIntSetting(settings, "alterId")

		clients = append(clients, client)
	}

	return map[string]any{
		"clients": clients,
	}
}

// generateTrojanSettings generates Trojan protocol settings.
func (g *ConfigGenerator) generateTrojanSettings(proxy *repository.Proxy, settings map[string]any) map[string]any {
	clients := []map[string]any{}

	// Extract password from settings
	if password, ok := settings["password"].(string); ok && password != "" {
		clients = append(clients, map[string]any{
			"password": password,
			"email":    fmt.Sprintf("user-%d-proxy-%d", proxy.UserID, proxy.ID),
			"level":    0,
		})
	}

	return map[string]any{
		"clients":   clients,
		"fallbacks": []map[string]any{},
	}
}

// generateShadowsocksSettings generates Shadowsocks protocol settings.
func (g *ConfigGenerator) generateShadowsocksSettings(proxy *repository.Proxy, settings map[string]any) map[string]any {
	result := map[string]any{
		"network": "tcp,udp",
		"level":   0,
		"email":   fmt.Sprintf("user-%d-proxy-%d", proxy.UserID, proxy.ID),
	}

	// Extract method and password
	if method, ok := settings["method"].(string); ok {
		result["method"] = method
	} else {
		result["method"] = "aes-256-gcm"
	}

	if password, ok := settings["password"].(string); ok {
		result["password"] = password
	}

	return result
}

// generateStreamSettings generates stream settings from proxy settings.
func (g *ConfigGenerator) generateStreamSettings(ctx context.Context, settings map[string]any, optimizationSettings node.NetworkOptimizationSettings) *StreamSettings {
	stream := &StreamSettings{
		Network: "tcp", // default
	}

	// Extract network type
	if network, ok := settings["network"].(string); ok {
		stream.Network = network
	}

	// Extract security settings
	if security, ok := settings["security"].(string); ok {
		stream.Security = security

		if security == "tls" {
			stream.TLSSettings = &TLSSettings{}

			if serverName := getStringSetting(settings, "server_name"); serverName != "" {
				stream.TLSSettings.ServerName = serverName
			}

			stream.TLSSettings.Certificates = g.resolveTLSCertificates(ctx, settings)

			if len(stream.TLSSettings.Certificates) == 0 {
				g.logMissingTLSCertificate(settings)
			}

			// ALPN
			if alpn := getStringSliceSetting(settings, "alpn"); len(alpn) > 0 {
				stream.TLSSettings.ALPN = alpn
			}
		} else if security == "reality" {
			stream.RealitySettings = buildRealitySettings(settings)
		}
	}

	// Network-specific settings
	switch stream.Network {
	case "ws":
		if wsSettings := buildWSSettings(settings); len(wsSettings) > 0 {
			stream.WSSettings = wsSettings
		}
	case "tcp":
		if tcpSettings := buildTCPSettings(settings); len(tcpSettings) > 0 {
			stream.TCPSettings = tcpSettings
		}
	case "http":
		if httpSettings := buildHTTPSettings(settings); len(httpSettings) > 0 {
			stream.HTTPSettings = httpSettings
		}
	case "quic":
		if quicSettings, ok := settings["quic_settings"].(map[string]any); ok {
			stream.QUICSettings = quicSettings
		}
	case "grpc":
		if grpcSettings := buildGRPCSettings(settings); len(grpcSettings) > 0 {
			stream.GRPCSettings = grpcSettings
		}
	}

	if sockopt := buildStreamSockopt(stream.Network, optimizationSettings); len(sockopt) > 0 {
		stream.Sockopt = sockopt
	}

	return stream
}

func buildWSSettings(settings map[string]any) map[string]any {
	wsSettings := cloneMapSetting(getMapSetting(settings, "ws_settings"))
	path := getStringSetting(wsSettings, "path")
	if path == "" {
		path = getStringSetting(settings, "path")
	}
	host := getStringSetting(wsSettings, "host")
	if host == "" {
		host = getHeaderHost(wsSettings["headers"])
	}
	if host == "" {
		host = getStringSetting(settings, "host")
	}

	if path != "" {
		wsSettings["path"] = path
	}
	if host != "" {
		headers := cloneMapSetting(getMapFromValue(wsSettings["headers"]))
		headers["Host"] = host
		wsSettings["headers"] = headers
	}
	return wsSettings
}

func buildTCPSettings(settings map[string]any) map[string]any {
	tcpSettings := cloneMapSetting(getMapSetting(settings, "tcp_settings"))
	if len(tcpSettings) > 0 {
		return tcpSettings
	}

	headerType := getStringSetting(settings, "headerType")
	if headerType == "" {
		headerType = getStringSetting(settings, "type")
	}
	if !strings.EqualFold(headerType, "http") {
		return nil
	}

	path := getStringSetting(settings, "path")
	if path == "" {
		path = "/"
	}
	request := map[string]any{
		"path": []string{path},
	}
	if host := getStringSetting(settings, "host"); host != "" {
		request["headers"] = map[string]any{"Host": []string{host}}
	}
	return map[string]any{
		"header": map[string]any{
			"type":    "http",
			"request": request,
		},
	}
}

func buildHTTPSettings(settings map[string]any) map[string]any {
	httpSettings := cloneMapSetting(getMapSetting(settings, "http_settings"))
	path := getStringSetting(httpSettings, "path")
	if path == "" {
		path = getStringSetting(settings, "path")
	}
	host := getStringSetting(httpSettings, "host")
	if host == "" {
		host = getStringSetting(settings, "host")
	}
	if path != "" {
		httpSettings["path"] = path
	}
	if host != "" {
		httpSettings["host"] = []string{host}
	}
	return httpSettings
}

func buildGRPCSettings(settings map[string]any) map[string]any {
	grpcSettings := cloneMapSetting(getMapSetting(settings, "grpc_settings"))
	serviceName := getStringSetting(grpcSettings, "serviceName")
	if serviceName == "" {
		serviceName = getStringSetting(settings, "serviceName")
	}
	if serviceName != "" {
		grpcSettings["serviceName"] = serviceName
	}
	return grpcSettings
}

func buildStreamSockopt(network string, optimizationSettings node.NetworkOptimizationSettings) map[string]any {
	settings := optimizationSettings.Normalize()
	if !settings.EnableXraySockopt {
		return nil
	}

	switch strings.ToLower(strings.TrimSpace(network)) {
	case "quic", "kcp":
		return nil
	}

	sockopt := make(map[string]any)
	if settings.XrayTCPFastOpen {
		sockopt["tcpFastOpen"] = true
	}
	if congestion := strings.TrimSpace(settings.XrayTCPCongestion); congestion != "" {
		sockopt["tcpcongestion"] = congestion
	}
	if len(sockopt) == 0 {
		return nil
	}
	return sockopt
}

func (g *ConfigGenerator) resolveTLSCertificates(ctx context.Context, settings map[string]any) []Certificate {
	certFile := getStringSetting(settings, "cert_file")
	keyFile := getStringSetting(settings, "key_file")
	if certFile != "" && keyFile != "" {
		return []Certificate{{
			CertificateFile: certFile,
			KeyFile:         keyFile,
		}}
	}

	certificateContent := getStringSetting(settings, "certificate")
	keyContent := getStringSetting(settings, "key")
	if certificateContent != "" && keyContent != "" {
		return []Certificate{{
			Certificate: []string{certificateContent},
			Key:         []string{keyContent},
		}}
	}

	domain := g.getTLSDomain(settings)
	if domain == "" || g.certRepo == nil {
		return nil
	}

	cert, matchedDomain, err := g.findCertificateForDomain(ctx, domain)
	if err != nil {
		g.logger.Warn("failed to resolve tls certificate",
			logger.F("domain", domain),
			logger.F("error", err.Error()))
		return nil
	}
	if cert == nil {
		return nil
	}

	resolvedCertificates := buildRepositoryTLSCertificates(cert)
	if len(resolvedCertificates) == 0 {
		return nil
	}

	g.logger.Info("auto matched tls certificate",
		logger.F("domain", domain),
		logger.F("matched_domain", matchedDomain),
		logger.F("cert_path", cert.CertPath))

	return resolvedCertificates
}

func (g *ConfigGenerator) getTLSDomain(settings map[string]any) string {
	for _, key := range []string{"tls_domain", "server_name", "host"} {
		if value := strings.TrimSpace(strings.ToLower(getStringSetting(settings, key))); value != "" {
			return value
		}
	}
	return ""
}

func (g *ConfigGenerator) findCertificateForDomain(ctx context.Context, domain string) (*repository.Certificate, string, error) {
	for _, candidate := range buildCertificateCandidates(domain) {
		cert, err := g.certRepo.GetByDomain(ctx, candidate)
		if err != nil {
			if apperrors.IsNotFound(err) {
				continue
			}
			return nil, "", err
		}
		if cert == nil {
			continue
		}
		if cert.Status != "active" {
			g.logger.Warn("certificate found but not active",
				logger.F("domain", candidate),
				logger.F("status", cert.Status))
			continue
		}
		if certificateIsExpired(cert) {
			g.logger.Warn("certificate found but expired",
				logger.F("domain", candidate))
			continue
		}
		if !hasRepositoryCertificateMaterial(cert) {
			g.logger.Warn("certificate found but material missing", logger.F("domain", candidate))
			continue
		}
		return cert, candidate, nil
	}
	return nil, "", nil
}

func certificateIsExpired(cert *repository.Certificate) bool {
	if cert == nil {
		return true
	}
	expiresAt := cert.ExpiresAt
	if expiresAt.IsZero() && cert.ExpireDate != nil {
		expiresAt = *cert.ExpireDate
	}
	return !expiresAt.IsZero() && !expiresAt.After(time.Now())
}

func hasRepositoryCertificateMaterial(cert *repository.Certificate) bool {
	if cert == nil {
		return false
	}
	if cert.CertPath != "" && cert.KeyPath != "" {
		return true
	}
	return cert.Certificate != "" && cert.PrivateKey != ""
}

func buildRepositoryTLSCertificates(cert *repository.Certificate) []Certificate {
	if cert == nil {
		return nil
	}
	if cert.Certificate != "" && cert.PrivateKey != "" {
		return []Certificate{{
			Certificate: []string{cert.Certificate},
			Key:         []string{cert.PrivateKey},
		}}
	}
	if cert.CertPath != "" && cert.KeyPath != "" {
		if inlineCert, inlineKey, err := loadCertificatePair(cert.CertPath, cert.KeyPath); err == nil {
			return []Certificate{{
				Certificate: []string{inlineCert},
				Key:         []string{inlineKey},
			}}
		}
		return []Certificate{{
			CertificateFile: cert.CertPath,
			KeyFile:         cert.KeyPath,
		}}
	}
	return nil
}

func loadCertificatePair(certPath, keyPath string) (string, string, error) {
	resolvedCertPath, err := resolveCertificatePath(certPath)
	if err != nil {
		return "", "", err
	}
	resolvedKeyPath, err := resolveCertificatePath(keyPath)
	if err != nil {
		return "", "", err
	}

	certContent, err := os.ReadFile(resolvedCertPath)
	if err != nil {
		return "", "", err
	}
	keyContent, err := os.ReadFile(resolvedKeyPath)
	if err != nil {
		return "", "", err
	}

	return string(certContent), string(keyContent), nil
}

func resolveCertificatePath(path string) (string, error) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return "", fmt.Errorf("empty certificate path")
	}

	candidates := []string{trimmedPath}
	if !filepath.IsAbs(trimmedPath) {
		candidates = append(candidates, filepath.Join("/app", trimmedPath))
		if dataDir := strings.TrimSpace(os.Getenv("VPANEL_DATA_DIR")); dataDir != "" {
			candidates = append(candidates, filepath.Join(dataDir, trimmedPath))
			if strings.HasPrefix(trimmedPath, "data/") {
				candidates = append(candidates, filepath.Join(dataDir, strings.TrimPrefix(trimmedPath, "data/")))
			}
		}
	}

	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		candidate = filepath.Clean(candidate)
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("certificate path not found: %s", trimmedPath)
}

func (g *ConfigGenerator) logMissingTLSCertificate(settings map[string]any) {
	domain := g.getTLSDomain(settings)
	if domain == "" {
		return
	}
	g.logger.Warn("tls enabled but no matching certificate found", logger.F("domain", domain))
}

func buildCertificateCandidates(domain string) []string {
	rawDomain := strings.TrimSpace(strings.ToLower(domain))
	if rawDomain == "" {
		return nil
	}

	candidates := make([]string, 0, 2)
	seen := make(map[string]struct{})
	addCandidate := func(candidate string) {
		candidate = strings.TrimSpace(strings.ToLower(candidate))
		if candidate == "" {
			return
		}
		if _, exists := seen[candidate]; exists {
			return
		}
		seen[candidate] = struct{}{}
		candidates = append(candidates, candidate)
	}

	if strings.HasPrefix(rawDomain, "*.") {
		addCandidate(rawDomain)
		addCandidate(normalizeTLSDomain(rawDomain))
		return candidates
	}

	normalizedDomain := normalizeTLSDomain(rawDomain)
	addCandidate(normalizedDomain)
	if wildcard := wildcardDomain(normalizedDomain); wildcard != "" {
		addCandidate(wildcard)
	}
	return candidates
}

func wildcardDomain(domain string) string {
	parts := strings.Split(normalizeTLSDomain(domain), ".")
	if len(parts) < 3 {
		return ""
	}
	return "*." + strings.Join(parts[1:], ".")
}

func normalizeTLSDomain(domain string) string {
	domain = strings.TrimSpace(strings.ToLower(domain))
	return strings.TrimPrefix(domain, "*.")
}

func xraySettingAliases(key string) []string {
	switch key {
	case "alterId", "alter_id":
		return []string{"alterId", "alter_id"}
	case "fingerprint", "fp":
		return []string{"fingerprint", "fp"}
	case "publicKey", "pbk":
		return []string{"publicKey", "pbk"}
	case "shortId", "sid":
		return []string{"shortId", "sid"}
	case "sni", "server_name":
		return []string{"sni", "server_name"}
	case "serviceName", "service_name":
		return []string{"serviceName", "service_name"}
	case "type", "headerType":
		return []string{"type", "headerType"}
	default:
		return []string{key}
	}
}

func getStringSetting(settings map[string]any, key string) string {
	for _, candidate := range xraySettingAliases(key) {
		value, ok := settings[candidate]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if trimmed := strings.TrimSpace(typed); trimmed != "" {
				return trimmed
			}
		case []string:
			for _, item := range typed {
				if trimmed := strings.TrimSpace(item); trimmed != "" {
					return trimmed
				}
			}
		case []any:
			for _, item := range typed {
				if text, ok := item.(string); ok {
					text = strings.TrimSpace(text)
					if text != "" {
						return text
					}
				}
			}
		}
	}
	return ""
}

func getStringSliceSetting(settings map[string]any, key string) []string {
	values := make([]string, 0)
	appendValue := func(item string) {
		for _, part := range strings.Split(item, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			values = append(values, part)
		}
	}

	for _, candidate := range xraySettingAliases(key) {
		value, ok := settings[candidate]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			appendValue(typed)
		case []string:
			for _, item := range typed {
				appendValue(item)
			}
		case []any:
			for _, item := range typed {
				if text, ok := item.(string); ok {
					appendValue(text)
				}
			}
		}
		if len(values) > 0 {
			return values
		}
	}

	return nil
}

func getMapSetting(settings map[string]any, key string) map[string]any {
	for _, candidate := range xraySettingAliases(key) {
		if mapped := getMapFromValue(settings[candidate]); mapped != nil {
			return mapped
		}
	}
	return nil
}

func getMapFromValue(value any) map[string]any {
	if value == nil {
		return nil
	}
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	if typed, ok := value.(map[string]string); ok {
		converted := make(map[string]any, len(typed))
		for mapKey, mapValue := range typed {
			converted[mapKey] = mapValue
		}
		return converted
	}
	return nil
}

func cloneMapSetting(settings map[string]any) map[string]any {
	if len(settings) == 0 {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(settings))
	for key, value := range settings {
		cloned[key] = value
	}
	return cloned
}

func getHeaderHost(value any) string {
	headers := getMapFromValue(value)
	for _, key := range []string{"Host", "host"} {
		if host := getStringSetting(headers, key); host != "" {
			return host
		}
	}
	return ""
}

func getBoolSetting(settings map[string]any, key string) bool {
	value, ok := settings[key]
	if !ok || value == nil {
		return false
	}

	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		trimmed := strings.TrimSpace(typed)
		return strings.EqualFold(trimmed, "true") || trimmed == "1"
	case int:
		return typed != 0
	case float64:
		return typed != 0
	default:
		return false
	}
}

func getIntSetting(settings map[string]any, key string) int {
	for _, candidate := range xraySettingAliases(key) {
		value, ok := settings[candidate]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case int:
			return typed
		case int64:
			return int(typed)
		case float64:
			return int(typed)
		default:
			continue
		}
	}
	return 0
}

func buildRealitySettings(settings map[string]any) map[string]any {
	realitySettings := getMapSetting(settings, "reality_settings")
	if realitySettings == nil {
		realitySettings = make(map[string]any)
	}

	dest := getStringSetting(realitySettings, "dest")
	if dest == "" {
		dest = getStringSetting(settings, "dest")
	}
	privateKey := getStringSetting(realitySettings, "privateKey")
	if privateKey == "" {
		privateKey = getStringSetting(settings, "privateKey")
	}
	serverNames := getStringSliceSetting(realitySettings, "serverNames")
	if len(serverNames) == 0 {
		if sni := getStringSetting(settings, "server_name"); sni != "" {
			serverNames = []string{sni}
		} else if sni := getStringSetting(settings, "sni"); sni != "" {
			serverNames = []string{sni}
		}
	}
	shortIDs := getStringSliceSetting(realitySettings, "shortIds")
	if len(shortIDs) == 0 {
		if shortID := getStringSetting(settings, "shortId"); shortID != "" {
			shortIDs = []string{shortID}
		} else if shortID := getStringSetting(settings, "sid"); shortID != "" {
			shortIDs = []string{shortID}
		} else {
			shortIDs = []string{""}
		}
	}

	result := map[string]any{}
	if show := getBoolSetting(realitySettings, "show"); show {
		result["show"] = true
	}
	if dest != "" {
		result["dest"] = dest
	}
	result["xver"] = getIntSetting(realitySettings, "xver")
	if len(serverNames) > 0 {
		result["serverNames"] = serverNames
	}
	if privateKey != "" {
		result["privateKey"] = privateKey
	}
	if len(shortIDs) > 0 {
		result["shortIds"] = shortIDs
	}

	if len(result) == 1 {
		if _, ok := result["xver"]; ok {
			return nil
		}
	}

	return result
}

// generateOutbounds generates outbound configurations.
func (g *ConfigGenerator) generateOutbounds() []OutboundConfig {
	return []OutboundConfig{
		{
			Tag:      "direct",
			Protocol: "freedom",
			Settings: map[string]any{},
		},
		{
			Tag:      "blocked",
			Protocol: "blackhole",
			Settings: map[string]any{},
		},
	}
}

// generateRouting generates routing configuration.
func (g *ConfigGenerator) generateRouting() *RoutingConfig {
	return &RoutingConfig{
		Rules: []RoutingRule{
			{
				Type:        "field",
				InboundTag:  []string{"api"},
				OutboundTag: "api",
			},
			{
				Type:        "field",
				Protocol:    []string{"bittorrent"},
				OutboundTag: "blocked",
			},
		},
	}
}

// ToJSON converts the configuration to JSON.
func (c *XrayConfig) ToJSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}
