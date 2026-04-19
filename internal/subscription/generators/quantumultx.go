// Package generators provides subscription format generators for various clients.
package generators

import (
	"fmt"
	"strings"

	"v/internal/database/repository"
)

// QuantumultXGenerator generates subscription content in Quantumult X format.
type QuantumultXGenerator struct{}

// NewQuantumultXGenerator creates a new Quantumult X format generator.
func NewQuantumultXGenerator() *QuantumultXGenerator {
	return &QuantumultXGenerator{}
}

// Generate creates subscription content in Quantumult X format.
func (g *QuantumultXGenerator) Generate(proxies []*repository.Proxy, options *GeneratorOptions) ([]byte, error) {
	if options == nil {
		options = DefaultOptions()
	}

	var lines []string

	for _, proxy := range proxies {
		info := ExtractProxyInfo(proxy)
		
		var line string
		var err error

		switch strings.ToLower(info.Protocol) {
		case ProtocolVMess:
			line, err = g.generateVMessLine(info)
		case ProtocolTrojan:
			line, err = g.generateTrojanLine(info)
		case ProtocolShadowsocks, ProtocolSS:
			line, err = g.generateShadowsocksLine(info)
		default:
			continue // Quantumult X doesn't support VLESS natively
		}

		if err != nil {
			continue
		}

		lines = append(lines, line)
	}

	return []byte(strings.Join(lines, "\n")), nil
}

// ContentType returns the MIME type for Quantumult X format.
func (g *QuantumultXGenerator) ContentType() string {
	return "text/plain; charset=utf-8"
}

// FileExtension returns the file extension for Quantumult X format.
func (g *QuantumultXGenerator) FileExtension() string {
	return "conf"
}

// SupportsProtocol checks if Quantumult X format supports a specific protocol.
func (g *QuantumultXGenerator) SupportsProtocol(protocol string) bool {
	switch strings.ToLower(protocol) {
	case ProtocolVMess, ProtocolTrojan, ProtocolShadowsocks, ProtocolSS:
		return true
	default:
		return false
	}
}

// generateVMessLine generates a Quantumult X VMess proxy line.
func (g *QuantumultXGenerator) generateVMessLine(info *ProxyInfo) (string, error) {
	uuid := GetSettingString(info.Settings, "uuid", "")
	security := GetSettingString(info.Settings, "security", "auto")
	
	// Quantumult X VMess format:
	// vmess=server:port, method=security, password=uuid, tag=name
	parts := []string{
		fmt.Sprintf("vmess=%s:%d", info.Server, info.Port),
		fmt.Sprintf("method=%s", security),
		fmt.Sprintf("password=%s", uuid),
	}

	// TLS settings
	if GetSettingBool(info.Settings, "tls", false) {
		parts = append(parts, "obfs=over-tls")
		if sni := GetSettingString(info.Settings, "sni", ""); sni != "" {
			parts = append(parts, fmt.Sprintf("obfs-host=%s", sni))
		}
		if skipVerify := GetSettingBool(info.Settings, "skipCertVerify", false); skipVerify {
			parts = append(parts, "tls-verification=false")
		}
	}

	// WebSocket settings
	network := GetSettingString(info.Settings, "network", "tcp")
	if network == "ws" {
		if !GetSettingBool(info.Settings, "tls", false) {
			parts = append(parts, "obfs=ws")
		} else {
			parts = append(parts, "obfs=wss")
		}
		if path := GetSettingString(info.Settings, "path", ""); path != "" {
			parts = append(parts, fmt.Sprintf("obfs-uri=%s", path))
		}
		if host := GetSettingString(info.Settings, "host", ""); host != "" {
			parts = append(parts, fmt.Sprintf("obfs-host=%s", host))
		}
	}

	// Tag (name)
	parts = append(parts, fmt.Sprintf("tag=%s", info.Name))

	return strings.Join(parts, ", "), nil
}

// generateTrojanLine generates a Quantumult X Trojan proxy line.
func (g *QuantumultXGenerator) generateTrojanLine(info *ProxyInfo) (string, error) {
	password := GetSettingString(info.Settings, "password", "")
	
	// Quantumult X Trojan format:
	// trojan=server:port, password=xxx, tag=name
	parts := []string{
		fmt.Sprintf("trojan=%s:%d", info.Server, info.Port),
		fmt.Sprintf("password=%s", password),
	}

	// TLS settings
	parts = append(parts, "over-tls=true")
	
	if sni := GetSettingString(info.Settings, "sni", ""); sni != "" {
		parts = append(parts, fmt.Sprintf("tls-host=%s", sni))
	}

	if skipVerify := GetSettingBool(info.Settings, "skipCertVerify", false); skipVerify {
		parts = append(parts, "tls-verification=false")
	}

	// Tag (name)
	parts = append(parts, fmt.Sprintf("tag=%s", info.Name))

	return strings.Join(parts, ", "), nil
}

// generateShadowsocksLine generates a Quantumult X Shadowsocks proxy line.
func (g *QuantumultXGenerator) generateShadowsocksLine(info *ProxyInfo) (string, error) {
	method := GetSettingString(info.Settings, "method", "aes-256-gcm")
	password := GetSettingString(info.Settings, "password", "")

	// Quantumult X Shadowsocks format expects the raw password; base64 encoding
	// would make the client authenticate with the wrong bytes.
	// shadowsocks=server:port, method=xxx, password=xxx, tag=name
	parts := []string{
		fmt.Sprintf("shadowsocks=%s:%d", info.Server, info.Port),
		fmt.Sprintf("method=%s", method),
		fmt.Sprintf("password=%s", password),
	}

	// UDP relay
	if udp := GetSettingBool(info.Settings, "udp", true); udp {
		parts = append(parts, "udp-relay=true")
	}

	// Tag (name)
	parts = append(parts, fmt.Sprintf("tag=%s", info.Name))

	return strings.Join(parts, ", "), nil
}
