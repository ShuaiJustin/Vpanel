// Package generators provides subscription format generators for various clients.
package generators

import (
	"strings"

	"gopkg.in/yaml.v3"

	"v/internal/database/repository"
	proxylib "v/internal/proxy"
)

// ClashMetaGenerator generates subscription content in Clash Meta (Mihomo) YAML format.
// This format extends Clash with additional features like Reality, XTLS, etc.
type ClashMetaGenerator struct{}

// NewClashMetaGenerator creates a new Clash Meta format generator.
func NewClashMetaGenerator() *ClashMetaGenerator {
	return &ClashMetaGenerator{}
}

// ClashMetaConfig represents the Clash Meta configuration structure.
type ClashMetaConfig struct {
	Port               int                      `yaml:"port,omitempty"`
	SocksPort          int                      `yaml:"socks-port,omitempty"`
	AllowLAN           bool                     `yaml:"allow-lan"`
	Mode               string                   `yaml:"mode"`
	LogLevel           string                   `yaml:"log-level"`
	ExternalController string                   `yaml:"external-controller,omitempty"`
	UnifiedDelay       bool                     `yaml:"unified-delay,omitempty"`
	TCPConcurrent      bool                     `yaml:"tcp-concurrent,omitempty"`
	FindProcessMode    string                   `yaml:"find-process-mode,omitempty"`
	GlobalClientFP     string                   `yaml:"global-client-fingerprint,omitempty"`
	Proxies            []map[string]interface{} `yaml:"proxies"`
	ProxyGroups        []ClashProxyGroup        `yaml:"proxy-groups,omitempty"`
	Rules              []string                 `yaml:"rules,omitempty"`
}

// Generate creates subscription content in Clash Meta YAML format.
func (g *ClashMetaGenerator) Generate(proxies []*repository.Proxy, options *GeneratorOptions) ([]byte, error) {
	if options == nil {
		options = DefaultOptions()
	}

	config := ClashMetaConfig{
		Port:            7890,
		SocksPort:       7891,
		AllowLAN:        false,
		Mode:            "rule",
		LogLevel:        "info",
		UnifiedDelay:    true,
		TCPConcurrent:   true,
		FindProcessMode: "strict",
		GlobalClientFP:  "chrome",
		Proxies:         make([]map[string]interface{}, 0),
	}

	var proxyNames []string

	for _, info := range ExtractProxyInfos(proxies) {
		var clashProxy map[string]interface{}
		var err error

		switch strings.ToLower(info.Protocol) {
		case ProtocolVMess:
			clashProxy, err = g.generateVMessProxy(info)
		case ProtocolVLESS:
			clashProxy, err = g.generateVLESSProxy(info)
		case ProtocolTrojan:
			clashProxy, err = g.generateTrojanProxy(info)
		case ProtocolShadowsocks, ProtocolSS:
			clashProxy, err = g.generateShadowsocksProxy(info)
		default:
			continue
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

// ContentType returns the MIME type for Clash Meta format.
func (g *ClashMetaGenerator) ContentType() string {
	return "text/yaml; charset=utf-8"
}

// FileExtension returns the file extension for Clash Meta format.
func (g *ClashMetaGenerator) FileExtension() string {
	return "yaml"
}

// SupportsProtocol checks if Clash Meta format supports a specific protocol.
func (g *ClashMetaGenerator) SupportsProtocol(protocol string) bool {
	switch strings.ToLower(protocol) {
	case ProtocolVMess, ProtocolVLESS, ProtocolTrojan, ProtocolShadowsocks, ProtocolSS:
		return true
	default:
		return false
	}
}

// generateVMessProxy generates a Clash Meta VMess proxy configuration.
func (g *ClashMetaGenerator) generateVMessProxy(info *ProxyInfo) (map[string]interface{}, error) {
	proxy := map[string]interface{}{
		"name":    info.Name,
		"type":    "vmess",
		"server":  info.Server,
		"port":    info.Port,
		"uuid":    GetSettingString(info.Settings, "uuid", ""),
		"alterId": GetSettingInt(info.Settings, "alterId", 0),
		"cipher":  proxylib.ResolveVMessCipher(info.Settings),
	}

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
		if fp := GetSettingString(info.Settings, "fingerprint", ""); fp != "" {
			proxy["client-fingerprint"] = fp
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

// generateVLESSProxy generates a Clash Meta VLESS proxy configuration with Reality support.
func (g *ClashMetaGenerator) generateVLESSProxy(info *ProxyInfo) (map[string]interface{}, error) {
	proxy := map[string]interface{}{
		"name":   info.Name,
		"type":   "vless",
		"server": info.Server,
		"port":   info.Port,
		"uuid":   GetSettingString(info.Settings, "uuid", ""),
	}

	network := GetSettingString(info.Settings, "network", "tcp")
	proxy["network"] = network

	// Flow settings (for XTLS)
	if flow := GetSettingString(info.Settings, "flow", ""); flow != "" {
		proxy["flow"] = flow
	}

	// Security settings
	security := GetSettingString(info.Settings, "security", "")

	if security == "reality" {
		// Reality settings (Clash Meta specific)
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
	} else if security == "tls" || GetSettingBool(info.Settings, "tls", false) {
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

// generateTrojanProxy generates a Clash Meta Trojan proxy configuration.
func (g *ClashMetaGenerator) generateTrojanProxy(info *ProxyInfo) (map[string]interface{}, error) {
	proxy := map[string]interface{}{
		"name":     info.Name,
		"type":     "trojan",
		"server":   info.Server,
		"port":     info.Port,
		"password": GetSettingString(info.Settings, "password", ""),
	}

	if sni := GetSettingString(info.Settings, "sni", ""); sni != "" {
		proxy["sni"] = sni
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

// generateShadowsocksProxy generates a Clash Meta Shadowsocks proxy configuration.
func (g *ClashMetaGenerator) generateShadowsocksProxy(info *ProxyInfo) (map[string]interface{}, error) {
	proxy := map[string]interface{}{
		"name":     info.Name,
		"type":     "ss",
		"server":   info.Server,
		"port":     info.Port,
		"cipher":   GetSettingString(info.Settings, "method", "aes-256-gcm"),
		"password": GetSettingString(info.Settings, "password", ""),
	}

	if udp := GetSettingBool(info.Settings, "udp", true); udp {
		proxy["udp"] = true
	}

	return proxy, nil
}

// generateProxyGroups generates default proxy groups for Clash Meta.
func (g *ClashMetaGenerator) generateProxyGroups(proxyNames []string) []ClashProxyGroup {
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

// generateRules generates default rules for Clash Meta.
func (g *ClashMetaGenerator) generateRules() []string {
	return []string{
		"MATCH,Proxy",
	}
}
