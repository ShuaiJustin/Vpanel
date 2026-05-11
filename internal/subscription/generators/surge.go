// Package generators provides subscription format generators for various clients.
package generators

import (
	"fmt"
	"strings"

	"v/internal/database/repository"
	proxylib "v/internal/proxy"
)

// SurgeGenerator generates subscription content in Surge proxy list format.
type SurgeGenerator struct{}

// NewSurgeGenerator creates a new Surge format generator.
func NewSurgeGenerator() *SurgeGenerator {
	return &SurgeGenerator{}
}

// Generate creates subscription content in Surge proxy list format.
func (g *SurgeGenerator) Generate(proxies []*repository.Proxy, options *GeneratorOptions) ([]byte, error) {
	if options == nil {
		options = DefaultOptions()
	}

	var lines []string
	lines = append(lines, "[Proxy]")

	for _, info := range ExtractProxyInfos(proxies) {
		var line string
		var err error

		switch normalizeProtocolName(info.Protocol) {
		case ProtocolVMess:
			line, err = g.generateVMessLine(info)
		case ProtocolTrojan:
			line, err = g.generateTrojanLine(info)
		case ProtocolShadowsocks, ProtocolSS:
			line, err = g.generateShadowsocksLine(info)
		default:
			continue // Surge doesn't support VLESS natively
		}

		if err != nil {
			continue
		}

		lines = append(lines, line)
	}

	return []byte(strings.Join(lines, "\n")), nil
}

// ContentType returns the MIME type for Surge format.
func (g *SurgeGenerator) ContentType() string {
	return "text/plain; charset=utf-8"
}

// FileExtension returns the file extension for Surge format.
func (g *SurgeGenerator) FileExtension() string {
	return "conf"
}

// SupportsProtocol checks if Surge format supports a specific protocol.
func (g *SurgeGenerator) SupportsProtocol(protocol string) bool {
	switch normalizeProtocolName(protocol) {
	case ProtocolVMess, ProtocolTrojan, ProtocolShadowsocks, ProtocolSS:
		return true
	default:
		return false
	}
}

// generateVMessLine generates a Surge VMess proxy line.
func (g *SurgeGenerator) generateVMessLine(info *ProxyInfo) (string, error) {
	uuid := GetSettingString(info.Settings, "uuid", "")

	// Basic format: name = vmess, server, port, username=uuid
	parts := []string{
		fmt.Sprintf("%s = vmess", info.Name),
		info.Server,
		fmt.Sprintf("%d", info.Port),
		fmt.Sprintf("username=%s", uuid),
	}

	// Encryption method
	cipher := proxylib.ResolveVMessCipher(info.Settings)
	parts = append(parts, fmt.Sprintf("encrypt-method=%s", cipher))

	// TLS settings
	if proxylib.HasTLSSettings(info.Settings) {
		parts = append(parts, "tls=true")
		if sni := proxylib.ResolveSNI(info.Settings); sni != "" {
			parts = append(parts, fmt.Sprintf("sni=%s", sni))
		}
		if proxylib.ResolveTLSSkipVerify(info.Settings) {
			parts = append(parts, "skip-cert-verify=true")
		}
	}

	// WebSocket settings
	network := GetSettingString(info.Settings, "network", "tcp")
	if network == "ws" {
		parts = append(parts, "ws=true")
		if path := GetSettingString(info.Settings, "path", ""); path != "" {
			parts = append(parts, fmt.Sprintf("ws-path=%s", path))
		}
		if host := GetSettingString(info.Settings, "host", ""); host != "" {
			parts = append(parts, fmt.Sprintf("ws-headers=Host:%s", host))
		}
	}

	return strings.Join(parts, ", "), nil
}

// generateTrojanLine generates a Surge Trojan proxy line.
func (g *SurgeGenerator) generateTrojanLine(info *ProxyInfo) (string, error) {
	password := GetSettingString(info.Settings, "password", "")

	// Basic format: name = trojan, server, port, password=xxx
	parts := []string{
		fmt.Sprintf("%s = trojan", info.Name),
		info.Server,
		fmt.Sprintf("%d", info.Port),
		fmt.Sprintf("password=%s", password),
	}

	// SNI
	if sni := GetSettingString(info.Settings, "sni", ""); sni != "" {
		parts = append(parts, fmt.Sprintf("sni=%s", sni))
	}

	// Skip cert verify
	if skipVerify := GetSettingBool(info.Settings, "skipCertVerify", false); skipVerify {
		parts = append(parts, "skip-cert-verify=true")
	}

	return strings.Join(parts, ", "), nil
}

// generateShadowsocksLine generates a Surge Shadowsocks proxy line.
func (g *SurgeGenerator) generateShadowsocksLine(info *ProxyInfo) (string, error) {
	method := GetSettingString(info.Settings, "method", "aes-256-gcm")
	password := GetSettingString(info.Settings, "password", "")

	// Basic format: name = ss, server, port, encrypt-method=xxx, password=xxx
	parts := []string{
		fmt.Sprintf("%s = ss", info.Name),
		info.Server,
		fmt.Sprintf("%d", info.Port),
		fmt.Sprintf("encrypt-method=%s", method),
		fmt.Sprintf("password=%s", password),
	}

	// UDP relay
	if udp := GetSettingBool(info.Settings, "udp", true); udp {
		parts = append(parts, "udp-relay=true")
	}

	return strings.Join(parts, ", "), nil
}
