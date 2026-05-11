// Package generators provides subscription format generators for various clients.
package generators

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"v/internal/database/repository"
	proxylib "v/internal/proxy"
)

// ShadowrocketGenerator generates subscription content in Shadowrocket format.
// This format is similar to V2rayN but with some Shadowrocket-specific extensions.
type ShadowrocketGenerator struct{}

// NewShadowrocketGenerator creates a new Shadowrocket format generator.
func NewShadowrocketGenerator() *ShadowrocketGenerator {
	return &ShadowrocketGenerator{}
}

// Generate creates subscription content in Shadowrocket format.
func (g *ShadowrocketGenerator) Generate(proxies []*repository.Proxy, options *GeneratorOptions) ([]byte, error) {
	if options == nil {
		options = DefaultOptions()
	}

	var links []string

	for _, info := range ExtractProxyInfos(proxies) {
		var link string
		var err error

		switch normalizeProtocolName(info.Protocol) {
		case ProtocolVMess:
			link, err = g.generateVMessLink(info)
		case ProtocolVLESS:
			link, err = g.generateVLESSLink(info)
		case ProtocolTrojan:
			link, err = g.generateTrojanLink(info)
		case ProtocolShadowsocks, ProtocolSS:
			link, err = g.generateShadowsocksLink(info)
		default:
			continue
		}

		if err != nil {
			continue
		}

		links = append(links, link)
	}

	// Join all links with newline and base64 encode
	content := strings.Join(links, "\n")
	encoded := base64.StdEncoding.EncodeToString([]byte(content))

	return []byte(encoded), nil
}

// ContentType returns the MIME type for Shadowrocket format.
func (g *ShadowrocketGenerator) ContentType() string {
	return "text/plain; charset=utf-8"
}

// FileExtension returns the file extension for Shadowrocket format.
func (g *ShadowrocketGenerator) FileExtension() string {
	return "txt"
}

// SupportsProtocol checks if Shadowrocket format supports a specific protocol.
func (g *ShadowrocketGenerator) SupportsProtocol(protocol string) bool {
	switch normalizeProtocolName(protocol) {
	case ProtocolVMess, ProtocolVLESS, ProtocolTrojan, ProtocolShadowsocks, ProtocolSS:
		return true
	default:
		return false
	}
}

// generateVMessLink generates a VMess link for Shadowrocket.
func (g *ShadowrocketGenerator) generateVMessLink(info *ProxyInfo) (string, error) {
	// Shadowrocket uses a different VMess format
	// vmess://method:uuid@server:port?params#name

	uuid := GetSettingString(info.Settings, "uuid", "")
	security := proxylib.ResolveVMessCipher(info.Settings)

	params := url.Values{}

	// Network type
	network := GetSettingString(info.Settings, "network", "tcp")
	if network != "tcp" {
		params.Set("obfs", network)
	}

	// WebSocket settings
	if network == "ws" {
		if path := GetSettingString(info.Settings, "path", ""); path != "" {
			params.Set("path", path)
		}
		if host := GetSettingString(info.Settings, "host", ""); host != "" {
			params.Set("obfsParam", host)
		}
	}

	// TLS settings
	if proxylib.HasTLSSettings(info.Settings) {
		params.Set("tls", "1")
		if sni := proxylib.ResolveSNI(info.Settings); sni != "" {
			params.Set("peer", sni)
		}
		if proxylib.ResolveTLSSkipVerify(info.Settings) {
			params.Set("allowInsecure", "1")
		}
	}

	// Alter ID
	alterId := GetSettingInt(info.Settings, "alterId", 0)
	params.Set("alterId", fmt.Sprintf("%d", alterId))

	// Build URL
	userInfo := base64.URLEncoding.EncodeToString([]byte(security + ":" + uuid))
	link := fmt.Sprintf("vmess://%s@%s:%d", userInfo, info.Server, info.Port)
	if len(params) > 0 {
		link += "?" + params.Encode()
	}
	link += "#" + url.PathEscape(info.Name)

	return link, nil
}

// generateVLESSLink generates a VLESS link for Shadowrocket.
func (g *ShadowrocketGenerator) generateVLESSLink(info *ProxyInfo) (string, error) {
	uuid := GetSettingString(info.Settings, "uuid", "")
	if uuid == "" {
		return "", fmt.Errorf("uuid is required for VLESS")
	}

	params := url.Values{}
	params.Set("encryption", "none")

	// Network type
	network := GetSettingString(info.Settings, "network", "tcp")
	params.Set("type", network)

	// Security
	security := GetSettingString(info.Settings, "security", "")
	if security != "" {
		params.Set("security", security)
	}

	if security == "" && proxylib.HasTLSSettings(info.Settings) {
		params.Set("security", "tls")
	}

	// SNI
	if sni := GetSettingString(info.Settings, "sni", ""); sni != "" {
		params.Set("sni", sni)
	}

	// Host
	if host := GetSettingString(info.Settings, "host", ""); host != "" {
		params.Set("host", host)
	}

	// Path
	if path := GetSettingString(info.Settings, "path", ""); path != "" {
		params.Set("path", path)
	}

	// Flow
	if flow := GetSettingString(info.Settings, "flow", ""); flow != "" {
		params.Set("flow", flow)
	}

	// Fingerprint
	if fp := GetSettingString(info.Settings, "fingerprint", ""); fp != "" {
		params.Set("fp", fp)
	}
	if GetSettingBool(info.Settings, "allowInsecure", false) {
		params.Set("allowInsecure", "1")
	}

	// Reality settings
	if pbk := GetSettingString(info.Settings, "publicKey", ""); pbk != "" {
		params.Set("pbk", pbk)
	}
	if sid := GetSettingString(info.Settings, "shortId", ""); sid != "" {
		params.Set("sid", sid)
	}

	// Build URL
	link := fmt.Sprintf("vless://%s@%s:%d", uuid, info.Server, info.Port)
	if len(params) > 0 {
		link += "?" + params.Encode()
	}
	link += "#" + url.PathEscape(info.Name)

	return link, nil
}

// generateTrojanLink generates a Trojan link for Shadowrocket.
func (g *ShadowrocketGenerator) generateTrojanLink(info *ProxyInfo) (string, error) {
	password := GetSettingString(info.Settings, "password", "")
	if password == "" {
		return "", fmt.Errorf("password is required for Trojan")
	}

	params := url.Values{}

	// SNI
	if sni := GetSettingString(info.Settings, "sni", ""); sni != "" {
		params.Set("sni", sni)
	}

	// ALPN
	if alpn := GetSettingString(info.Settings, "alpn", ""); alpn != "" {
		params.Set("alpn", alpn)
	}

	// Fingerprint
	if fp := GetSettingString(info.Settings, "fingerprint", ""); fp != "" {
		params.Set("fp", fp)
	}
	if GetSettingBool(info.Settings, "allowInsecure", false) {
		params.Set("allowInsecure", "1")
	}

	// Network type (for WebSocket/gRPC)
	network := GetSettingString(info.Settings, "network", "tcp")
	if network != "tcp" {
		params.Set("type", network)

		if host := GetSettingString(info.Settings, "host", ""); host != "" {
			params.Set("host", host)
		}
		if path := GetSettingString(info.Settings, "path", ""); path != "" {
			params.Set("path", path)
		}
	}

	// Build URL
	link := fmt.Sprintf("trojan://%s@%s:%d", url.PathEscape(password), info.Server, info.Port)
	if len(params) > 0 {
		link += "?" + params.Encode()
	}
	link += "#" + url.PathEscape(info.Name)

	return link, nil
}

// generateShadowsocksLink generates a Shadowsocks link for Shadowrocket.
func (g *ShadowrocketGenerator) generateShadowsocksLink(info *ProxyInfo) (string, error) {
	method := GetSettingString(info.Settings, "method", "aes-256-gcm")
	password := GetSettingString(info.Settings, "password", "")
	if password == "" {
		return "", fmt.Errorf("password is required for Shadowsocks")
	}

	// SIP002 requires URL-safe Base64 without padding; Shadowrocket rejects padded variants.
	userInfo := base64.RawURLEncoding.EncodeToString([]byte(method + ":" + password))
	link := fmt.Sprintf("ss://%s@%s:%d#%s", userInfo, info.Server, info.Port, url.PathEscape(info.Name))

	return link, nil
}
