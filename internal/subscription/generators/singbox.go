// Package generators provides subscription format generators for various clients.
package generators

import (
	"encoding/json"
	"strings"

	"v/internal/database/repository"
	proxylib "v/internal/proxy"
)

// SingboxGenerator generates subscription content in Sing-box JSON format.
type SingboxGenerator struct{}

// NewSingboxGenerator creates a new Sing-box format generator.
func NewSingboxGenerator() *SingboxGenerator {
	return &SingboxGenerator{}
}

// SingboxConfig represents the Sing-box configuration structure.
type SingboxConfig struct {
	Outbounds []map[string]interface{} `json:"outbounds"`
}

// Generate creates subscription content in Sing-box JSON format.
func (g *SingboxGenerator) Generate(proxies []*repository.Proxy, options *GeneratorOptions) ([]byte, error) {
	if options == nil {
		options = DefaultOptions()
	}

	config := SingboxConfig{
		Outbounds: make([]map[string]interface{}, 0),
	}

	for _, info := range ExtractProxyInfos(proxies) {
		var outbound map[string]interface{}
		var err error

		switch strings.ToLower(info.Protocol) {
		case ProtocolVMess:
			outbound, err = g.generateVMessOutbound(info)
		case ProtocolVLESS:
			outbound, err = g.generateVLESSOutbound(info)
		case ProtocolTrojan:
			outbound, err = g.generateTrojanOutbound(info)
		case ProtocolShadowsocks, ProtocolSS:
			outbound, err = g.generateShadowsocksOutbound(info)
		default:
			continue
		}

		if err != nil {
			continue
		}

		config.Outbounds = append(config.Outbounds, outbound)
	}

	return json.MarshalIndent(config, "", "  ")
}

// ContentType returns the MIME type for Sing-box format.
func (g *SingboxGenerator) ContentType() string {
	return "application/json; charset=utf-8"
}

// FileExtension returns the file extension for Sing-box format.
func (g *SingboxGenerator) FileExtension() string {
	return "json"
}

// SupportsProtocol checks if Sing-box format supports a specific protocol.
func (g *SingboxGenerator) SupportsProtocol(protocol string) bool {
	switch strings.ToLower(protocol) {
	case ProtocolVMess, ProtocolVLESS, ProtocolTrojan, ProtocolShadowsocks, ProtocolSS:
		return true
	default:
		return false
	}
}

// generateVMessOutbound generates a Sing-box VMess outbound configuration.
func (g *SingboxGenerator) generateVMessOutbound(info *ProxyInfo) (map[string]interface{}, error) {
	outbound := map[string]interface{}{
		"type":        "vmess",
		"tag":         info.Name,
		"server":      info.Server,
		"server_port": info.Port,
		"uuid":        GetSettingString(info.Settings, "uuid", ""),
		"security":    proxylib.ResolveVMessCipher(info.Settings),
		"alter_id":    GetSettingInt(info.Settings, "alterId", 0),
	}

	// TLS settings
	if proxylib.HasTLSSettings(info.Settings) {
		tls := map[string]interface{}{
			"enabled": true,
		}
		if sni := proxylib.ResolveSNI(info.Settings); sni != "" {
			tls["server_name"] = sni
		}
		if proxylib.ResolveTLSSkipVerify(info.Settings) {
			tls["insecure"] = true
		}
		if alpn := GetSettingString(info.Settings, "alpn", ""); alpn != "" {
			tls["alpn"] = strings.Split(alpn, ",")
		}
		outbound["tls"] = tls
	}

	// Transport settings
	network := GetSettingString(info.Settings, "network", "tcp")
	if network != "tcp" {
		transport := map[string]interface{}{
			"type": network,
		}

		if network == "ws" {
			if path := GetSettingString(info.Settings, "path", ""); path != "" {
				transport["path"] = path
			}
			if host := GetSettingString(info.Settings, "host", ""); host != "" {
				transport["headers"] = map[string]string{"Host": host}
			}
		}

		if network == "grpc" {
			if serviceName := GetSettingString(info.Settings, "serviceName", ""); serviceName != "" {
				transport["service_name"] = serviceName
			}
		}

		outbound["transport"] = transport
	}

	return outbound, nil
}

// generateVLESSOutbound generates a Sing-box VLESS outbound configuration.
func (g *SingboxGenerator) generateVLESSOutbound(info *ProxyInfo) (map[string]interface{}, error) {
	outbound := map[string]interface{}{
		"type":        "vless",
		"tag":         info.Name,
		"server":      info.Server,
		"server_port": info.Port,
		"uuid":        GetSettingString(info.Settings, "uuid", ""),
	}

	// Flow settings
	if flow := GetSettingString(info.Settings, "flow", ""); flow != "" {
		outbound["flow"] = flow
	}

	// Security settings
	security := GetSettingString(info.Settings, "security", "")

	if security == "reality" {
		tls := map[string]interface{}{
			"enabled":     true,
			"server_name": GetSettingString(info.Settings, "sni", ""),
		}

		reality := map[string]interface{}{
			"enabled": true,
		}
		if pbk := GetSettingString(info.Settings, "publicKey", ""); pbk != "" {
			reality["public_key"] = pbk
		}
		if sid := GetSettingString(info.Settings, "shortId", ""); sid != "" {
			reality["short_id"] = sid
		}

		tls["reality"] = reality

		if fp := GetSettingString(info.Settings, "fingerprint", ""); fp != "" {
			tls["utls"] = map[string]interface{}{
				"enabled":     true,
				"fingerprint": fp,
			}
		}

		outbound["tls"] = tls
	} else if proxylib.HasTLSSettings(info.Settings) {
		tls := map[string]interface{}{
			"enabled": true,
		}
		if sni := GetSettingString(info.Settings, "sni", ""); sni != "" {
			tls["server_name"] = sni
		}
		if skipVerify := GetSettingBool(info.Settings, "skipCertVerify", false); skipVerify {
			tls["insecure"] = true
		}
		if alpn := GetSettingString(info.Settings, "alpn", ""); alpn != "" {
			tls["alpn"] = strings.Split(alpn, ",")
		}
		if fp := GetSettingString(info.Settings, "fingerprint", ""); fp != "" {
			tls["utls"] = map[string]interface{}{
				"enabled":     true,
				"fingerprint": fp,
			}
		}
		outbound["tls"] = tls
	}

	// Transport settings
	network := GetSettingString(info.Settings, "network", "tcp")
	if network != "tcp" {
		transport := map[string]interface{}{
			"type": network,
		}

		if network == "ws" {
			if path := GetSettingString(info.Settings, "path", ""); path != "" {
				transport["path"] = path
			}
			if host := GetSettingString(info.Settings, "host", ""); host != "" {
				transport["headers"] = map[string]string{"Host": host}
			}
		}

		if network == "grpc" {
			if serviceName := GetSettingString(info.Settings, "serviceName", ""); serviceName != "" {
				transport["service_name"] = serviceName
			}
		}

		outbound["transport"] = transport
	}

	return outbound, nil
}

// generateTrojanOutbound generates a Sing-box Trojan outbound configuration.
func (g *SingboxGenerator) generateTrojanOutbound(info *ProxyInfo) (map[string]interface{}, error) {
	outbound := map[string]interface{}{
		"type":        "trojan",
		"tag":         info.Name,
		"server":      info.Server,
		"server_port": info.Port,
		"password":    GetSettingString(info.Settings, "password", ""),
	}

	// TLS settings (Trojan always uses TLS)
	tls := map[string]interface{}{
		"enabled": true,
	}
	if sni := GetSettingString(info.Settings, "sni", ""); sni != "" {
		tls["server_name"] = sni
	}
	if skipVerify := GetSettingBool(info.Settings, "skipCertVerify", false); skipVerify {
		tls["insecure"] = true
	}
	if alpn := GetSettingString(info.Settings, "alpn", ""); alpn != "" {
		tls["alpn"] = strings.Split(alpn, ",")
	}
	if fp := GetSettingString(info.Settings, "fingerprint", ""); fp != "" {
		tls["utls"] = map[string]interface{}{
			"enabled":     true,
			"fingerprint": fp,
		}
	}
	outbound["tls"] = tls

	// Transport settings
	network := GetSettingString(info.Settings, "network", "tcp")
	if network != "tcp" {
		transport := map[string]interface{}{
			"type": network,
		}

		if network == "ws" {
			if path := GetSettingString(info.Settings, "path", ""); path != "" {
				transport["path"] = path
			}
			if host := GetSettingString(info.Settings, "host", ""); host != "" {
				transport["headers"] = map[string]string{"Host": host}
			}
		}

		if network == "grpc" {
			if serviceName := GetSettingString(info.Settings, "serviceName", ""); serviceName != "" {
				transport["service_name"] = serviceName
			}
		}

		outbound["transport"] = transport
	}

	return outbound, nil
}

// generateShadowsocksOutbound generates a Sing-box Shadowsocks outbound configuration.
func (g *SingboxGenerator) generateShadowsocksOutbound(info *ProxyInfo) (map[string]interface{}, error) {
	outbound := map[string]interface{}{
		"type":        "shadowsocks",
		"tag":         info.Name,
		"server":      info.Server,
		"server_port": info.Port,
		"method":      GetSettingString(info.Settings, "method", "aes-256-gcm"),
		"password":    GetSettingString(info.Settings, "password", ""),
	}

	return outbound, nil
}
