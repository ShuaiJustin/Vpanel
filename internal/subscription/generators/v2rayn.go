// Package generators provides subscription format generators for various clients.
package generators

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"v/internal/database/repository"
	proxylib "v/internal/proxy"
)

// V2rayNGenerator generates subscription content in V2rayN/V2rayNG format.
// This format uses base64 encoded links, one per line.
type V2rayNGenerator struct{}

// NewV2rayNGenerator creates a new V2rayN format generator.
func NewV2rayNGenerator() *V2rayNGenerator {
	return &V2rayNGenerator{}
}

// Generate creates subscription content in V2rayN format.
func (g *V2rayNGenerator) Generate(proxies []*repository.Proxy, options *GeneratorOptions) ([]byte, error) {
	if options == nil {
		options = DefaultOptions()
	}

	var links []string

	for _, proxy := range proxies {
		info := ExtractProxyInfo(proxy)

		var link string
		var err error

		switch strings.ToLower(info.Protocol) {
		case ProtocolVMess:
			link, err = g.generateVMessLink(info)
		case ProtocolVLESS:
			link, err = g.generateVLESSLink(info)
		case ProtocolTrojan:
			link, err = g.generateTrojanLink(info)
		case ProtocolShadowsocks, ProtocolSS:
			link, err = g.generateShadowsocksLink(info)
		default:
			continue // Skip unsupported protocols
		}

		if err != nil {
			continue // Skip on error
		}

		links = append(links, link)
	}

	// Join all links with newline and base64 encode
	content := strings.Join(links, "\n")
	encoded := base64.StdEncoding.EncodeToString([]byte(content))

	return []byte(encoded), nil
}

// ContentType returns the MIME type for V2rayN format.
func (g *V2rayNGenerator) ContentType() string {
	return "text/plain; charset=utf-8"
}

// FileExtension returns the file extension for V2rayN format.
func (g *V2rayNGenerator) FileExtension() string {
	return "txt"
}

// SupportsProtocol checks if V2rayN format supports a specific protocol.
func (g *V2rayNGenerator) SupportsProtocol(protocol string) bool {
	switch strings.ToLower(protocol) {
	case ProtocolVMess, ProtocolVLESS, ProtocolTrojan, ProtocolShadowsocks, ProtocolSS:
		return true
	default:
		return false
	}
}

// vmessConfig represents the VMess configuration for V2rayN.
type vmessConfig struct {
	V    string `json:"v"`
	PS   string `json:"ps"`
	Add  string `json:"add"`
	Port string `json:"port"`
	ID   string `json:"id"`
	Aid  string `json:"aid"`
	Scy  string `json:"scy"`
	Net  string `json:"net"`
	Type string `json:"type"`
	Host string `json:"host"`
	Path string `json:"path"`
	TLS  string `json:"tls"`
	SNI  string `json:"sni"`
	ALPN string `json:"alpn"`
	FP   string `json:"fp"`
}

// generateVMessLink generates a VMess link.
func (g *V2rayNGenerator) generateVMessLink(info *ProxyInfo) (string, error) {
	config := vmessConfig{
		V:    "2",
		PS:   info.Name,
		Add:  info.Server,
		Port: fmt.Sprintf("%d", info.Port),
		ID:   GetSettingString(info.Settings, "uuid", ""),
		Aid:  fmt.Sprintf("%d", GetSettingInt(info.Settings, "alterId", 0)),
		Scy:  proxylib.ResolveVMessCipher(info.Settings),
		Net:  GetSettingString(info.Settings, "network", "tcp"),
		Type: GetSettingString(info.Settings, "type", "none"),
		Host: GetSettingString(info.Settings, "host", ""),
		Path: GetSettingString(info.Settings, "path", ""),
		TLS:  "",
		SNI:  proxylib.ResolveSNI(info.Settings),
		ALPN: GetSettingString(info.Settings, "alpn", ""),
		FP:   GetSettingString(info.Settings, "fingerprint", ""),
	}

	if proxylib.HasTLSSettings(info.Settings) {
		config.TLS = "tls"
	}

	jsonData, err := json.Marshal(config)
	if err != nil {
		return "", err
	}

	encoded := base64.StdEncoding.EncodeToString(jsonData)
	return "vmess://" + encoded, nil
}

// generateVLESSLink generates a VLESS link.
func (g *V2rayNGenerator) generateVLESSLink(info *ProxyInfo) (string, error) {
	uuid := GetSettingString(info.Settings, "uuid", "")
	if uuid == "" {
		return "", fmt.Errorf("uuid is required for VLESS")
	}

	// Build query parameters
	params := url.Values{}
	params.Set("encryption", "none")

	if network := GetSettingString(info.Settings, "network", ""); network != "" {
		params.Set("type", network)
	}

	if security := GetSettingString(info.Settings, "security", ""); security != "" {
		params.Set("security", security)
	}

	if GetSettingBool(info.Settings, "tls", false) {
		params.Set("security", "tls")
	}

	if sni := GetSettingString(info.Settings, "sni", ""); sni != "" {
		params.Set("sni", sni)
	}

	if host := GetSettingString(info.Settings, "host", ""); host != "" {
		params.Set("host", host)
	}

	if path := GetSettingString(info.Settings, "path", ""); path != "" {
		params.Set("path", path)
	}

	if fp := GetSettingString(info.Settings, "fingerprint", ""); fp != "" {
		params.Set("fp", fp)
	}

	if alpn := GetSettingString(info.Settings, "alpn", ""); alpn != "" {
		params.Set("alpn", alpn)
	}

	if flow := GetSettingString(info.Settings, "flow", ""); flow != "" {
		params.Set("flow", flow)
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

// generateTrojanLink generates a Trojan link.
func (g *V2rayNGenerator) generateTrojanLink(info *ProxyInfo) (string, error) {
	password := GetSettingString(info.Settings, "password", "")
	if password == "" {
		return "", fmt.Errorf("password is required for Trojan")
	}

	// Build query parameters
	params := url.Values{}

	if sni := GetSettingString(info.Settings, "sni", ""); sni != "" {
		params.Set("sni", sni)
	}

	if alpn := GetSettingString(info.Settings, "alpn", ""); alpn != "" {
		params.Set("alpn", alpn)
	}

	if fp := GetSettingString(info.Settings, "fingerprint", ""); fp != "" {
		params.Set("fp", fp)
	}

	if network := GetSettingString(info.Settings, "network", ""); network != "" && network != "tcp" {
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

// generateShadowsocksLink generates a Shadowsocks link.
func (g *V2rayNGenerator) generateShadowsocksLink(info *ProxyInfo) (string, error) {
	method := GetSettingString(info.Settings, "method", "aes-256-gcm")
	password := GetSettingString(info.Settings, "password", "")
	if password == "" {
		return "", fmt.Errorf("password is required for Shadowsocks")
	}

	// SIP002 format: ss://base64(method:password)@server:port#name
	userInfo := base64.URLEncoding.EncodeToString([]byte(method + ":" + password))
	link := fmt.Sprintf("ss://%s@%s:%d#%s", userInfo, info.Server, info.Port, url.PathEscape(info.Name))

	return link, nil
}
