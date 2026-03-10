// Package subscription provides subscription link management functionality.
package subscription

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"gopkg.in/yaml.v3"

	"v/internal/database/repository"
	proxylib "v/internal/proxy"
)

// Helper functions for extracting settings

func getSettingString(settings map[string]interface{}, key string, defaultValue string) string {
	if v, ok := settings[key].(string); ok {
		return v
	}
	return defaultValue
}

func getSettingInt(settings map[string]interface{}, key string, defaultValue int) int {
	if v, ok := settings[key].(float64); ok {
		return int(v)
	}
	if v, ok := settings[key].(int); ok {
		return v
	}
	return defaultValue
}

func getSettingBool(settings map[string]interface{}, key string, defaultValue bool) bool {
	if v, ok := settings[key].(bool); ok {
		return v
	}
	return defaultValue
}

// extractProxyInfo extracts proxy information from a repository.Proxy.
func extractProxyInfo(proxy *repository.Proxy) (name, server string, port int, settings map[string]interface{}) {
	name = proxy.Name
	if proxy.Remark != "" {
		name = proxy.Remark
	}

	server = proxylib.ResolveServerAddress(proxy.Host, proxy.Settings)

	return name, server, proxy.Port, proxy.Settings
}

// generateV2rayN generates V2rayN format subscription content.
func generateV2rayN(proxies []*repository.Proxy, options *GeneratorOptions) ([]byte, error) {
	var links []string

	for _, proxy := range proxies {
		name, server, port, settings := extractProxyInfo(proxy)
		protocol := strings.ToLower(proxy.Protocol)

		var link string
		var err error

		switch protocol {
		case "vmess":
			link, err = generateVMessLink(name, server, port, settings)
		case "vless":
			link, err = generateVLESSLink(name, server, port, settings)
		case "trojan":
			link, err = generateTrojanLink(name, server, port, settings)
		case "shadowsocks", "ss":
			link, err = generateSSLink(name, server, port, settings)
		default:
			continue
		}

		if err != nil {
			continue
		}
		links = append(links, link)
	}

	content := strings.Join(links, "\n")
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	return []byte(encoded), nil
}

// vmessConfig represents VMess configuration for V2rayN.
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
}

func generateVMessLink(name, server string, port int, settings map[string]interface{}) (string, error) {
	config := vmessConfig{
		V:    "2",
		PS:   name,
		Add:  server,
		Port: fmt.Sprintf("%d", port),
		ID:   getSettingString(settings, "uuid", ""),
		Aid:  fmt.Sprintf("%d", getSettingInt(settings, "alterId", 0)),
		Scy:  getSettingString(settings, "security", "auto"),
		Net:  getSettingString(settings, "network", "tcp"),
		Type: getSettingString(settings, "type", "none"),
		Host: getSettingString(settings, "host", ""),
		Path: getSettingString(settings, "path", ""),
		SNI:  proxylib.ResolveSNI(settings),
	}

	if proxylib.HasTLSSettings(settings) {
		config.TLS = "tls"
	}

	jsonData, err := json.Marshal(config)
	if err != nil {
		return "", err
	}

	encoded := base64.StdEncoding.EncodeToString(jsonData)
	return "vmess://" + encoded, nil
}

func generateVLESSLink(name, server string, port int, settings map[string]interface{}) (string, error) {
	uuid := getSettingString(settings, "uuid", "")
	if uuid == "" {
		return "", fmt.Errorf("uuid is required")
	}

	params := url.Values{}
	if network := getSettingString(settings, "network", ""); network != "" {
		params.Set("type", network)
	}
	if security := getSettingString(settings, "security", ""); security != "" {
		params.Set("security", security)
	}
	if getSettingBool(settings, "tls", false) {
		params.Set("security", "tls")
	}
	if sni := getSettingString(settings, "sni", ""); sni != "" {
		params.Set("sni", sni)
	}
	if host := getSettingString(settings, "host", ""); host != "" {
		params.Set("host", host)
	}
	if path := getSettingString(settings, "path", ""); path != "" {
		params.Set("path", path)
	}
	if flow := getSettingString(settings, "flow", ""); flow != "" {
		params.Set("flow", flow)
	}
	if pbk := getSettingString(settings, "publicKey", ""); pbk != "" {
		params.Set("pbk", pbk)
	}
	if sid := getSettingString(settings, "shortId", ""); sid != "" {
		params.Set("sid", sid)
	}

	link := fmt.Sprintf("vless://%s@%s:%d", uuid, server, port)
	if len(params) > 0 {
		link += "?" + params.Encode()
	}
	link += "#" + url.PathEscape(name)
	return link, nil
}

func generateTrojanLink(name, server string, port int, settings map[string]interface{}) (string, error) {
	password := getSettingString(settings, "password", "")
	if password == "" {
		return "", fmt.Errorf("password is required")
	}

	params := url.Values{}
	if sni := getSettingString(settings, "sni", ""); sni != "" {
		params.Set("sni", sni)
	}
	if alpn := getSettingString(settings, "alpn", ""); alpn != "" {
		params.Set("alpn", alpn)
	}

	link := fmt.Sprintf("trojan://%s@%s:%d", url.PathEscape(password), server, port)
	if len(params) > 0 {
		link += "?" + params.Encode()
	}
	link += "#" + url.PathEscape(name)
	return link, nil
}

func generateSSLink(name, server string, port int, settings map[string]interface{}) (string, error) {
	method := getSettingString(settings, "method", "aes-256-gcm")
	password := getSettingString(settings, "password", "")
	if password == "" {
		return "", fmt.Errorf("password is required")
	}

	userInfo := base64.URLEncoding.EncodeToString([]byte(method + ":" + password))
	link := fmt.Sprintf("ss://%s@%s:%d#%s", userInfo, server, port, url.PathEscape(name))
	return link, nil
}

// Clash configuration structures
type clashConfig struct {
	Port        int                      `yaml:"port,omitempty"`
	SocksPort   int                      `yaml:"socks-port,omitempty"`
	AllowLAN    bool                     `yaml:"allow-lan"`
	Mode        string                   `yaml:"mode"`
	LogLevel    string                   `yaml:"log-level"`
	Proxies     []map[string]interface{} `yaml:"proxies"`
	ProxyGroups []clashProxyGroup        `yaml:"proxy-groups,omitempty"`
	Rules       []string                 `yaml:"rules,omitempty"`
}

type clashProxyGroup struct {
	Name     string   `yaml:"name"`
	Type     string   `yaml:"type"`
	Proxies  []string `yaml:"proxies"`
	URL      string   `yaml:"url,omitempty"`
	Interval int      `yaml:"interval,omitempty"`
}

// generateClash generates Clash format subscription content.
func generateClash(proxies []*repository.Proxy, options *GeneratorOptions) ([]byte, error) {
	config := clashConfig{
		Port:      7890,
		SocksPort: 7891,
		AllowLAN:  false,
		Mode:      "rule",
		LogLevel:  "info",
		Proxies:   make([]map[string]interface{}, 0),
	}

	var proxyNames []string

	for _, proxy := range proxies {
		name, server, port, settings := extractProxyInfo(proxy)
		protocol := strings.ToLower(proxy.Protocol)

		var clashProxy map[string]interface{}

		switch protocol {
		case "vmess":
			clashProxy = generateClashVMess(name, server, port, settings)
		case "vless":
			clashProxy = generateClashVLESS(name, server, port, settings)
		case "trojan":
			clashProxy = generateClashTrojan(name, server, port, settings)
		case "shadowsocks", "ss":
			clashProxy = generateClashSS(name, server, port, settings)
		default:
			continue
		}

		config.Proxies = append(config.Proxies, clashProxy)
		proxyNames = append(proxyNames, name)
	}

	if options != nil && options.IncludeProxyGroups && len(proxyNames) > 0 {
		config.ProxyGroups = generateClashProxyGroups(proxyNames)
		config.Rules = []string{"MATCH,Proxy"}
	}

	return yaml.Marshal(config)
}

func generateClashVMess(name, server string, port int, settings map[string]interface{}) map[string]interface{} {
	proxy := map[string]interface{}{
		"name":    name,
		"type":    "vmess",
		"server":  server,
		"port":    port,
		"uuid":    getSettingString(settings, "uuid", ""),
		"alterId": getSettingInt(settings, "alterId", 0),
		"cipher":  getSettingString(settings, "security", "auto"),
		"network": getSettingString(settings, "network", "tcp"),
	}

	if getSettingBool(settings, "tls", false) {
		proxy["tls"] = true
		if sni := getSettingString(settings, "sni", ""); sni != "" {
			proxy["servername"] = sni
		}
	}

	network := getSettingString(settings, "network", "tcp")
	if network == "ws" {
		wsOpts := map[string]interface{}{}
		if path := getSettingString(settings, "path", ""); path != "" {
			wsOpts["path"] = path
		}
		if host := getSettingString(settings, "host", ""); host != "" {
			wsOpts["headers"] = map[string]string{"Host": host}
		}
		if len(wsOpts) > 0 {
			proxy["ws-opts"] = wsOpts
		}
	}

	return proxy
}

func generateClashVLESS(name, server string, port int, settings map[string]interface{}) map[string]interface{} {
	proxy := map[string]interface{}{
		"name":    name,
		"type":    "vless",
		"server":  server,
		"port":    port,
		"uuid":    getSettingString(settings, "uuid", ""),
		"network": getSettingString(settings, "network", "tcp"),
	}

	if flow := getSettingString(settings, "flow", ""); flow != "" {
		proxy["flow"] = flow
	}

	security := getSettingString(settings, "security", "")
	if security == "tls" || getSettingBool(settings, "tls", false) {
		proxy["tls"] = true
		if sni := getSettingString(settings, "sni", ""); sni != "" {
			proxy["servername"] = sni
		}
	}

	if security == "reality" {
		proxy["tls"] = true
		realityOpts := map[string]interface{}{}
		if pbk := getSettingString(settings, "publicKey", ""); pbk != "" {
			realityOpts["public-key"] = pbk
		}
		if sid := getSettingString(settings, "shortId", ""); sid != "" {
			realityOpts["short-id"] = sid
		}
		if len(realityOpts) > 0 {
			proxy["reality-opts"] = realityOpts
		}
	}

	return proxy
}

func generateClashTrojan(name, server string, port int, settings map[string]interface{}) map[string]interface{} {
	proxy := map[string]interface{}{
		"name":     name,
		"type":     "trojan",
		"server":   server,
		"port":     port,
		"password": getSettingString(settings, "password", ""),
	}

	if sni := getSettingString(settings, "sni", ""); sni != "" {
		proxy["sni"] = sni
	}

	return proxy
}

func generateClashSS(name, server string, port int, settings map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"name":     name,
		"type":     "ss",
		"server":   server,
		"port":     port,
		"cipher":   getSettingString(settings, "method", "aes-256-gcm"),
		"password": getSettingString(settings, "password", ""),
		"udp":      true,
	}
}

func generateClashProxyGroups(proxyNames []string) []clashProxyGroup {
	selectProxies := append([]string{"DIRECT", "REJECT"}, proxyNames...)
	return []clashProxyGroup{
		{Name: "Proxy", Type: "select", Proxies: selectProxies},
		{Name: "Auto", Type: "url-test", Proxies: proxyNames, URL: "http://www.gstatic.com/generate_204", Interval: 300},
	}
}

// generateClashMeta generates Clash Meta format (same as Clash with extended features).
func generateClashMeta(proxies []*repository.Proxy, options *GeneratorOptions) ([]byte, error) {
	return generateClash(proxies, options)
}

// generateShadowrocket generates Shadowrocket format (same as V2rayN).
func generateShadowrocket(proxies []*repository.Proxy, options *GeneratorOptions) ([]byte, error) {
	return generateV2rayN(proxies, options)
}

// generateSurge generates Surge format subscription content.
func generateSurge(proxies []*repository.Proxy, options *GeneratorOptions) ([]byte, error) {
	var lines []string
	lines = append(lines, "[Proxy]")

	for _, proxy := range proxies {
		name, server, port, settings := extractProxyInfo(proxy)
		protocol := strings.ToLower(proxy.Protocol)

		var line string

		switch protocol {
		case "vmess":
			uuid := getSettingString(settings, "uuid", "")
			parts := []string{
				fmt.Sprintf("%s = vmess", name),
				server,
				fmt.Sprintf("%d", port),
				fmt.Sprintf("username=%s", uuid),
			}
			if getSettingBool(settings, "tls", false) {
				parts = append(parts, "tls=true")
			}
			line = strings.Join(parts, ", ")
		case "trojan":
			password := getSettingString(settings, "password", "")
			parts := []string{
				fmt.Sprintf("%s = trojan", name),
				server,
				fmt.Sprintf("%d", port),
				fmt.Sprintf("password=%s", password),
			}
			if sni := getSettingString(settings, "sni", ""); sni != "" {
				parts = append(parts, fmt.Sprintf("sni=%s", sni))
			}
			line = strings.Join(parts, ", ")
		case "shadowsocks", "ss":
			method := getSettingString(settings, "method", "aes-256-gcm")
			password := getSettingString(settings, "password", "")
			parts := []string{
				fmt.Sprintf("%s = ss", name),
				server,
				fmt.Sprintf("%d", port),
				fmt.Sprintf("encrypt-method=%s", method),
				fmt.Sprintf("password=%s", password),
			}
			line = strings.Join(parts, ", ")
		default:
			continue
		}

		lines = append(lines, line)
	}

	return []byte(strings.Join(lines, "\n")), nil
}

// generateQuantumultX generates Quantumult X format subscription content.
func generateQuantumultX(proxies []*repository.Proxy, options *GeneratorOptions) ([]byte, error) {
	var lines []string

	for _, proxy := range proxies {
		name, server, port, settings := extractProxyInfo(proxy)
		protocol := strings.ToLower(proxy.Protocol)

		var line string

		switch protocol {
		case "vmess":
			uuid := getSettingString(settings, "uuid", "")
			security := getSettingString(settings, "security", "auto")
			parts := []string{
				fmt.Sprintf("vmess=%s:%d", server, port),
				fmt.Sprintf("method=%s", security),
				fmt.Sprintf("password=%s", uuid),
				fmt.Sprintf("tag=%s", name),
			}
			line = strings.Join(parts, ", ")
		case "trojan":
			password := getSettingString(settings, "password", "")
			parts := []string{
				fmt.Sprintf("trojan=%s:%d", server, port),
				fmt.Sprintf("password=%s", password),
				"over-tls=true",
				fmt.Sprintf("tag=%s", name),
			}
			line = strings.Join(parts, ", ")
		case "shadowsocks", "ss":
			method := getSettingString(settings, "method", "aes-256-gcm")
			password := getSettingString(settings, "password", "")
			parts := []string{
				fmt.Sprintf("shadowsocks=%s:%d", server, port),
				fmt.Sprintf("method=%s", method),
				fmt.Sprintf("password=%s", password),
				fmt.Sprintf("tag=%s", name),
			}
			line = strings.Join(parts, ", ")
		default:
			continue
		}

		lines = append(lines, line)
	}

	return []byte(strings.Join(lines, "\n")), nil
}

// Sing-box configuration structures
type singboxConfig struct {
	Outbounds []map[string]interface{} `json:"outbounds"`
}

// generateSingbox generates Sing-box format subscription content.
func generateSingbox(proxies []*repository.Proxy, options *GeneratorOptions) ([]byte, error) {
	config := singboxConfig{
		Outbounds: make([]map[string]interface{}, 0),
	}

	for _, proxy := range proxies {
		name, server, port, settings := extractProxyInfo(proxy)
		protocol := strings.ToLower(proxy.Protocol)

		var outbound map[string]interface{}

		switch protocol {
		case "vmess":
			outbound = map[string]interface{}{
				"type":        "vmess",
				"tag":         name,
				"server":      server,
				"server_port": port,
				"uuid":        getSettingString(settings, "uuid", ""),
				"security":    getSettingString(settings, "security", "auto"),
				"alter_id":    getSettingInt(settings, "alterId", 0),
			}
		case "vless":
			outbound = map[string]interface{}{
				"type":        "vless",
				"tag":         name,
				"server":      server,
				"server_port": port,
				"uuid":        getSettingString(settings, "uuid", ""),
			}
			if flow := getSettingString(settings, "flow", ""); flow != "" {
				outbound["flow"] = flow
			}
		case "trojan":
			outbound = map[string]interface{}{
				"type":        "trojan",
				"tag":         name,
				"server":      server,
				"server_port": port,
				"password":    getSettingString(settings, "password", ""),
			}
		case "shadowsocks", "ss":
			outbound = map[string]interface{}{
				"type":        "shadowsocks",
				"tag":         name,
				"server":      server,
				"server_port": port,
				"method":      getSettingString(settings, "method", "aes-256-gcm"),
				"password":    getSettingString(settings, "password", ""),
			}
		default:
			continue
		}

		config.Outbounds = append(config.Outbounds, outbound)
	}

	return json.MarshalIndent(config, "", "  ")
}
