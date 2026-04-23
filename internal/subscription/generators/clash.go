// Package generators provides subscription format generators for various clients.
package generators

import (
	"strings"

	"gopkg.in/yaml.v3"

	"v/internal/database/repository"
	proxylib "v/internal/proxy"
)

// ClashGenerator generates subscription content in Clash YAML format.
type ClashGenerator struct{}

// NewClashGenerator creates a new Clash format generator.
func NewClashGenerator() *ClashGenerator {
	return &ClashGenerator{}
}

// ClashConfig represents the Clash configuration structure.
type ClashConfig struct {
	Port               int                      `yaml:"port,omitempty"`
	SocksPort          int                      `yaml:"socks-port,omitempty"`
	AllowLAN           bool                     `yaml:"allow-lan"`
	Mode               string                   `yaml:"mode"`
	LogLevel           string                   `yaml:"log-level"`
	ExternalController string                   `yaml:"external-controller,omitempty"`
	Proxies            []map[string]interface{} `yaml:"proxies"`
	ProxyGroups        []ClashProxyGroup        `yaml:"proxy-groups,omitempty"`
	Rules              []string                 `yaml:"rules,omitempty"`
}

// ClashProxyGroup represents a Clash proxy group.
type ClashProxyGroup struct {
	Name     string   `yaml:"name"`
	Type     string   `yaml:"type"`
	Proxies  []string `yaml:"proxies"`
	URL      string   `yaml:"url,omitempty"`
	Interval int      `yaml:"interval,omitempty"`
}

// Generate creates subscription content in Clash YAML format.
func (g *ClashGenerator) Generate(proxies []*repository.Proxy, options *GeneratorOptions) ([]byte, error) {
	if options == nil {
		options = DefaultOptions()
	}

	config := ClashConfig{
		Port:      7890,
		SocksPort: 7891,
		AllowLAN:  false,
		Mode:      "rule",
		LogLevel:  "info",
		Proxies:   make([]map[string]interface{}, 0),
	}

	var proxyNames []string

	for _, info := range ExtractProxyInfos(proxies) {
		var clashProxy map[string]interface{}
		var err error

		switch strings.ToLower(info.Protocol) {
		case ProtocolVMess:
			clashProxy, err = g.generateVMessProxy(info)
		case ProtocolTrojan:
			clashProxy, err = g.generateTrojanProxy(info)
		case ProtocolShadowsocks, ProtocolSS:
			clashProxy, err = g.generateShadowsocksProxy(info)
		default:
			continue // Skip unsupported protocols
		}

		if err != nil {
			continue
		}

		config.Proxies = append(config.Proxies, clashProxy)
		proxyNames = append(proxyNames, info.Name)
	}

	// Add proxy groups if enabled
	if options.IncludeProxyGroups && len(proxyNames) > 0 {
		config.ProxyGroups = g.generateProxyGroups(proxyNames)
		config.Rules = g.generateRules()
	}

	return yaml.Marshal(config)
}

// ContentType returns the MIME type for Clash format.
func (g *ClashGenerator) ContentType() string {
	return "text/yaml; charset=utf-8"
}

// FileExtension returns the file extension for Clash format.
func (g *ClashGenerator) FileExtension() string {
	return "yaml"
}

// SupportsProtocol checks if Clash format supports a specific protocol.
func (g *ClashGenerator) SupportsProtocol(protocol string) bool {
	switch strings.ToLower(protocol) {
	case ProtocolVMess, ProtocolTrojan, ProtocolShadowsocks, ProtocolSS:
		return true
	default:
		return false
	}
}

// generateVMessProxy generates a Clash VMess proxy configuration.
func (g *ClashGenerator) generateVMessProxy(info *ProxyInfo) (map[string]interface{}, error) {
	proxy := map[string]interface{}{
		"name":    info.Name,
		"type":    "vmess",
		"server":  info.Server,
		"port":    info.Port,
		"uuid":    GetSettingString(info.Settings, "uuid", ""),
		"alterId": GetSettingInt(info.Settings, "alterId", 0),
		"cipher":  proxylib.ResolveVMessCipher(info.Settings),
	}

	// Network settings
	network := GetSettingString(info.Settings, "network", "tcp")
	proxy["network"] = network

	// TLS settings
	if proxylib.HasTLSSettings(info.Settings) {
		proxy["tls"] = true
		if sni := proxylib.ResolveSNI(info.Settings); sni != "" {
			proxy["servername"] = sni
		}
		if proxylib.ResolveTLSSkipVerify(info.Settings) {
			proxy["skip-cert-verify"] = true
		}
	}

	// WebSocket settings
	if network == "ws" {
		wsOpts := map[string]interface{}{}
		if path := GetSettingString(info.Settings, "path", ""); path != "" {
			wsOpts["path"] = path
		}
		if host := GetSettingString(info.Settings, "host", ""); host != "" {
			wsOpts["headers"] = map[string]string{"Host": host}
		}
		if len(wsOpts) > 0 {
			proxy["ws-opts"] = wsOpts
		}
	}

	// gRPC settings
	if network == "grpc" {
		grpcOpts := map[string]interface{}{}
		if serviceName := GetSettingString(info.Settings, "serviceName", ""); serviceName != "" {
			grpcOpts["grpc-service-name"] = serviceName
		}
		if len(grpcOpts) > 0 {
			proxy["grpc-opts"] = grpcOpts
		}
	}

	return proxy, nil
}

// generateVLESSProxy generates a Clash VLESS proxy configuration.
func (g *ClashGenerator) generateVLESSProxy(info *ProxyInfo) (map[string]interface{}, error) {
	proxy := map[string]interface{}{
		"name":   info.Name,
		"type":   "vless",
		"server": info.Server,
		"port":   info.Port,
		"uuid":   GetSettingString(info.Settings, "uuid", ""),
	}

	// Network settings
	network := GetSettingString(info.Settings, "network", "tcp")
	proxy["network"] = network

	// Flow settings (for XTLS)
	if flow := GetSettingString(info.Settings, "flow", ""); flow != "" {
		proxy["flow"] = flow
	}

	// TLS settings
	security := GetSettingString(info.Settings, "security", "")
	if security != "reality" && proxylib.HasTLSSettings(info.Settings) {
		proxy["tls"] = true
		if sni := GetSettingString(info.Settings, "sni", ""); sni != "" {
			proxy["servername"] = sni
		}
		if skipVerify := GetSettingBool(info.Settings, "skipCertVerify", false); skipVerify {
			proxy["skip-cert-verify"] = true
		}
		if alpn := GetSettingString(info.Settings, "alpn", ""); alpn != "" {
			proxy["alpn"] = strings.Split(alpn, ",")
		}
		if fp := GetSettingString(info.Settings, "fingerprint", ""); fp != "" {
			proxy["client-fingerprint"] = fp
		}
	}

	// Reality settings
	if security == "reality" {
		proxy["tls"] = true
		realityOpts := map[string]interface{}{}
		if pbk := GetSettingString(info.Settings, "publicKey", ""); pbk != "" {
			realityOpts["public-key"] = pbk
		}
		if sid := GetSettingString(info.Settings, "shortId", ""); sid != "" {
			realityOpts["short-id"] = sid
		}
		if sni := GetSettingString(info.Settings, "sni", ""); sni != "" {
			proxy["servername"] = sni
		}
		if fp := GetSettingString(info.Settings, "fingerprint", ""); fp != "" {
			proxy["client-fingerprint"] = fp
		}
		if len(realityOpts) > 0 {
			proxy["reality-opts"] = realityOpts
		}
	}

	// WebSocket settings
	if network == "ws" {
		wsOpts := map[string]interface{}{}
		if path := GetSettingString(info.Settings, "path", ""); path != "" {
			wsOpts["path"] = path
		}
		if host := GetSettingString(info.Settings, "host", ""); host != "" {
			wsOpts["headers"] = map[string]string{"Host": host}
		}
		if len(wsOpts) > 0 {
			proxy["ws-opts"] = wsOpts
		}
	}

	// gRPC settings
	if network == "grpc" {
		grpcOpts := map[string]interface{}{}
		if serviceName := GetSettingString(info.Settings, "serviceName", ""); serviceName != "" {
			grpcOpts["grpc-service-name"] = serviceName
		}
		if len(grpcOpts) > 0 {
			proxy["grpc-opts"] = grpcOpts
		}
	}

	return proxy, nil
}

// generateTrojanProxy generates a Clash Trojan proxy configuration.
func (g *ClashGenerator) generateTrojanProxy(info *ProxyInfo) (map[string]interface{}, error) {
	proxy := map[string]interface{}{
		"name":     info.Name,
		"type":     "trojan",
		"server":   info.Server,
		"port":     info.Port,
		"password": GetSettingString(info.Settings, "password", ""),
	}

	// SNI
	if sni := GetSettingString(info.Settings, "sni", ""); sni != "" {
		proxy["sni"] = sni
	}

	// Skip cert verify
	if skipVerify := GetSettingBool(info.Settings, "skipCertVerify", false); skipVerify {
		proxy["skip-cert-verify"] = true
	}

	// ALPN
	if alpn := GetSettingString(info.Settings, "alpn", ""); alpn != "" {
		proxy["alpn"] = strings.Split(alpn, ",")
	}

	// Network settings (for WebSocket/gRPC over Trojan)
	network := GetSettingString(info.Settings, "network", "tcp")
	if network != "tcp" {
		proxy["network"] = network

		if network == "ws" {
			wsOpts := map[string]interface{}{}
			if path := GetSettingString(info.Settings, "path", ""); path != "" {
				wsOpts["path"] = path
			}
			if host := GetSettingString(info.Settings, "host", ""); host != "" {
				wsOpts["headers"] = map[string]string{"Host": host}
			}
			if len(wsOpts) > 0 {
				proxy["ws-opts"] = wsOpts
			}
		}

		if network == "grpc" {
			grpcOpts := map[string]interface{}{}
			if serviceName := GetSettingString(info.Settings, "serviceName", ""); serviceName != "" {
				grpcOpts["grpc-service-name"] = serviceName
			}
			if len(grpcOpts) > 0 {
				proxy["grpc-opts"] = grpcOpts
			}
		}
	}

	return proxy, nil
}

// generateShadowsocksProxy generates a Clash Shadowsocks proxy configuration.
func (g *ClashGenerator) generateShadowsocksProxy(info *ProxyInfo) (map[string]interface{}, error) {
	proxy := map[string]interface{}{
		"name":     info.Name,
		"type":     "ss",
		"server":   info.Server,
		"port":     info.Port,
		"cipher":   GetSettingString(info.Settings, "method", "aes-256-gcm"),
		"password": GetSettingString(info.Settings, "password", ""),
	}

	// UDP support
	if udp := GetSettingBool(info.Settings, "udp", true); udp {
		proxy["udp"] = true
	}

	return proxy, nil
}

// generateProxyGroups generates default proxy groups.
func (g *ClashGenerator) generateProxyGroups(proxyNames []string) []ClashProxyGroup {
	// Add DIRECT and REJECT to proxy options
	selectProxies := append([]string{"DIRECT", "REJECT"}, proxyNames...)
	autoProxies := proxyNames

	return []ClashProxyGroup{
		{
			Name:    "Proxy",
			Type:    "select",
			Proxies: selectProxies,
		},
		{
			Name:     "Auto",
			Type:     "url-test",
			Proxies:  autoProxies,
			URL:      "http://www.gstatic.com/generate_204",
			Interval: 300,
		},
		{
			Name:     "Fallback",
			Type:     "fallback",
			Proxies:  autoProxies,
			URL:      "http://www.gstatic.com/generate_204",
			Interval: 300,
		},
	}
}

// generateRules generates default rules.
func (g *ClashGenerator) generateRules() []string {
	return []string{
		"MATCH,Proxy",
	}
}
