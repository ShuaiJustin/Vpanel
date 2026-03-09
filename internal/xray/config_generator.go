// Package xray provides Xray configuration generation and management.
package xray

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"v/internal/database/repository"
	"v/internal/logger"
	apperrors "v/pkg/errors"
)

// ConfigGenerator generates Xray configurations for nodes.
type ConfigGenerator struct {
	proxyRepo repository.ProxyRepository
	certRepo  repository.CertificateRepository
	logger    logger.Logger
}

// NewConfigGenerator creates a new Xray config generator.
func NewConfigGenerator(
	proxyRepo repository.ProxyRepository,
	certRepo repository.CertificateRepository,
	log logger.Logger,
) *ConfigGenerator {
	return &ConfigGenerator{
		proxyRepo: proxyRepo,
		certRepo:  certRepo,
		logger:    log,
	}
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
	Network      string         `json:"network"`
	Security     string         `json:"security,omitempty"`
	TLSSettings  *TLSSettings   `json:"tlsSettings,omitempty"`
	TCPSettings  map[string]any `json:"tcpSettings,omitempty"`
	WSSettings   map[string]any `json:"wsSettings,omitempty"`
	HTTPSettings map[string]any `json:"httpSettings,omitempty"`
	QUICSettings map[string]any `json:"quicSettings,omitempty"`
	GRPCSettings map[string]any `json:"grpcSettings,omitempty"`
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

	g.logger.Info("generating config for node",
		logger.F("node_id", nodeID),
		logger.F("proxy_count", len(allProxies)))

	// Generate configuration
	config := &XrayConfig{
		Log: &LogConfig{
			LogLevel: "warning",
			Access:   "",
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
		Inbounds:  g.generateInbounds(ctx, allProxies),
		Outbounds: g.generateOutbounds(),
		Routing:   g.generateRouting(),
	}

	return config, nil
}

// generateInbounds generates inbound configurations from proxies.
func (g *ConfigGenerator) generateInbounds(ctx context.Context, proxies []*repository.Proxy) []InboundConfig {
	inbounds := []InboundConfig{
		// API inbound for stats
		{
			Tag:      "api",
			Listen:   "127.0.0.1",
			Port:     62789,
			Protocol: "dokodemo-door",
			Settings: map[string]any{
				"address": "127.0.0.1",
			},
		},
	}

	// Generate inbound for each proxy
	for _, proxy := range proxies {
		inbound := g.proxyToInbound(ctx, proxy)
		if inbound != nil {
			inbounds = append(inbounds, *inbound)
		}
	}

	return inbounds
}

// proxyToInbound converts a proxy to an Xray inbound configuration.
func (g *ConfigGenerator) proxyToInbound(ctx context.Context, proxy *repository.Proxy) *InboundConfig {
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

	switch proxy.Protocol {
	case "vless":
		inbound.Settings = g.generateVLESSSettings(proxy, settings)
		inbound.StreamSettings = g.generateStreamSettings(ctx, settings)
	case "vmess":
		inbound.Settings = g.generateVMessSettings(proxy, settings)
		inbound.StreamSettings = g.generateStreamSettings(ctx, settings)
	case "trojan":
		inbound.Settings = g.generateTrojanSettings(proxy, settings)
		inbound.StreamSettings = g.generateStreamSettings(ctx, settings)
	case "shadowsocks":
		inbound.Settings = g.generateShadowsocksSettings(proxy, settings)
	default:
		g.logger.Warn("Unsupported protocol",
			logger.F("proxy_id", proxy.ID),
			logger.F("protocol", proxy.Protocol))
		return nil
	}

	return inbound
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

		// Optional: alterId
		if alterId, ok := settings["alter_id"]; ok {
			client["alterId"] = alterId
		} else {
			client["alterId"] = 0
		}

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
func (g *ConfigGenerator) generateStreamSettings(ctx context.Context, settings map[string]any) *StreamSettings {
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

			if serverName, ok := settings["server_name"].(string); ok {
				stream.TLSSettings.ServerName = serverName
			}

			stream.TLSSettings.Certificates = g.resolveTLSCertificates(ctx, settings)

			if len(stream.TLSSettings.Certificates) == 0 {
				g.logMissingTLSCertificate(settings)
			}

			// ALPN
			if alpn, ok := settings["alpn"].([]string); ok {
				stream.TLSSettings.ALPN = alpn
			}
		}
	}

	// Network-specific settings
	switch stream.Network {
	case "ws":
		if wsSettings, ok := settings["ws_settings"].(map[string]any); ok {
			stream.WSSettings = wsSettings
		}
	case "tcp":
		if tcpSettings, ok := settings["tcp_settings"].(map[string]any); ok {
			stream.TCPSettings = tcpSettings
		}
	case "http":
		if httpSettings, ok := settings["http_settings"].(map[string]any); ok {
			stream.HTTPSettings = httpSettings
		}
	case "quic":
		if quicSettings, ok := settings["quic_settings"].(map[string]any); ok {
			stream.QUICSettings = quicSettings
		}
	case "grpc":
		if grpcSettings, ok := settings["grpc_settings"].(map[string]any); ok {
			stream.GRPCSettings = grpcSettings
		}
	}

	return stream
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
		if value := normalizeTLSDomain(getStringSetting(settings, key)); value != "" {
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
		if !hasRepositoryCertificateMaterial(cert) {
			g.logger.Warn("certificate found but material missing", logger.F("domain", candidate))
			continue
		}
		return cert, candidate, nil
	}
	return nil, "", nil
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
	if cert.CertPath != "" && cert.KeyPath != "" {
		return []Certificate{{
			CertificateFile: cert.CertPath,
			KeyFile:         cert.KeyPath,
		}}
	}
	if cert.Certificate != "" && cert.PrivateKey != "" {
		return []Certificate{{
			Certificate: []string{cert.Certificate},
			Key:         []string{cert.PrivateKey},
		}}
	}
	return nil
}

func (g *ConfigGenerator) logMissingTLSCertificate(settings map[string]any) {
	domain := g.getTLSDomain(settings)
	if domain == "" {
		return
	}
	g.logger.Warn("tls enabled but no matching certificate found", logger.F("domain", domain))
}

func buildCertificateCandidates(domain string) []string {
	domain = normalizeTLSDomain(domain)
	if domain == "" {
		return nil
	}

	candidates := []string{domain}
	if wildcard := wildcardDomain(domain); wildcard != "" && wildcard != domain {
		candidates = append(candidates, wildcard)
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

func getStringSetting(settings map[string]any, key string) string {
	value, ok := settings[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []string:
		if len(typed) > 0 {
			return strings.TrimSpace(typed[0])
		}
	}
	return ""
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
