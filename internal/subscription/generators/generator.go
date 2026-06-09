// Package generators provides subscription format generators for various clients.
package generators

import (
	"fmt"
	"strconv"
	"strings"

	"v/internal/database/repository"
	proxylib "v/internal/proxy"
)

// FormatGenerator defines the interface for subscription format generators.
type FormatGenerator interface {
	// Generate creates subscription content for the specific format.
	Generate(proxies []*repository.Proxy, options *GeneratorOptions) ([]byte, error)

	// ContentType returns the MIME type for the generated content.
	ContentType() string

	// FileExtension returns the file extension for downloads.
	FileExtension() string

	// SupportsProtocol checks if the format supports a specific protocol.
	SupportsProtocol(protocol string) bool
}

// GeneratorOptions represents options for content generation.
type GeneratorOptions struct {
	// SubscriptionName is the name of the subscription for display.
	SubscriptionName string

	// ServerHost is the server hostname for proxy configurations.
	ServerHost string

	// RenameTemplate is a custom naming template for proxies.
	// Supported placeholders: {name}, {protocol}, {port}, {index}
	RenameTemplate string

	// IncludeProxyGroups indicates whether to include proxy groups (for Clash).
	IncludeProxyGroups bool

	// UpdateInterval is the suggested update interval in hours.
	UpdateInterval int
}

// DefaultOptions returns default generator options.
func DefaultOptions() *GeneratorOptions {
	return &GeneratorOptions{
		SubscriptionName:   "V Panel Subscription",
		IncludeProxyGroups: true,
		UpdateInterval:     24,
	}
}

// Protocol constants for supported proxy protocols.
const (
	ProtocolVMess       = "vmess"
	ProtocolVLESS       = "vless"
	ProtocolTrojan      = "trojan"
	ProtocolShadowsocks = "shadowsocks"
	ProtocolSS          = "ss" // Alias for shadowsocks
)

// ProxyInfo represents extracted proxy information for generation.
type ProxyInfo struct {
	Name     string
	Protocol string
	Server   string
	Port     int
	Settings map[string]interface{}
}

func isGenericProxyRemark(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "", "auto provisioned", "auto-provisioned":
		return normalized != ""
	default:
		return false
	}
}

func looksAutoProvisionName(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(normalized, "node-") || isGenericProxyRemark(normalized)
}

func humanReadableProtocolName(protocol string) string {
	switch normalizeProtocolName(protocol) {
	case ProtocolVMess:
		return "VMess"
	case ProtocolVLESS:
		return "VLESS"
	case ProtocolTrojan:
		return "Trojan"
	case ProtocolShadowsocks, ProtocolSS:
		return "Shadowsocks"
	default:
		return strings.ToUpper(strings.TrimSpace(protocol))
	}
}

func normalizeProtocolName(protocol string) string {
	return strings.ToLower(strings.TrimSpace(protocol))
}

func clashGroupBaseName(options *GeneratorOptions) string {
	if options == nil {
		return "Proxy"
	}
	name := strings.TrimSpace(options.SubscriptionName)
	if name == "" || strings.EqualFold(name, "V Panel Subscription") {
		return "Proxy"
	}
	return name
}

func clashSelectGroupName(options *GeneratorOptions) string {
	return clashGroupBaseName(options)
}

func clashAutoGroupName(options *GeneratorOptions) string {
	return clashGroupBaseName(options) + " Auto"
}

func clashFallbackGroupName(options *GeneratorOptions) string {
	return clashGroupBaseName(options) + " Fallback"
}

func buildSubscriptionProxyName(proxy *repository.Proxy, server string) string {
	name := strings.TrimSpace(proxy.Name)
	remark := strings.TrimSpace(proxy.Remark)

	switch {
	case remark != "" && !isGenericProxyRemark(remark):
		return remark
	case name != "" && !looksAutoProvisionName(name):
		return name
	}

	host := strings.TrimSpace(server)
	if host == "" {
		host = strings.TrimSpace(proxy.Host)
	}
	protocol := humanReadableProtocolName(proxy.Protocol)
	if host == "" {
		if protocol == "" {
			return "Proxy"
		}
		return protocol
	}
	port := proxylib.ResolveServerPort(proxy.Port, proxy.Settings)
	if port > 0 {
		return fmt.Sprintf("%s · %s:%d", protocol, host, port)
	}
	return fmt.Sprintf("%s · %s", protocol, host)
}

// ExtractProxyInfo extracts proxy information from a repository.Proxy.
func ExtractProxyInfo(proxy *repository.Proxy) *ProxyInfo {
	settingsCopy := normalizeGeneratorSettings(proxy.Settings)
	server := proxylib.ResolveExternalServerAddress(settingsCopy)
	if server == "" {
		server = proxylib.ResolveServerAddress(proxy.Host, settingsCopy)
	}
	settingsCopy["server"] = server
	return &ProxyInfo{
		Name:     buildSubscriptionProxyName(proxy, server),
		Protocol: normalizeProtocolName(proxy.Protocol),
		Server:   server,
		Port:     proxylib.ResolveServerPort(proxy.Port, proxy.Settings),
		Settings: settingsCopy,
	}
}

func cloneGeneratorSettings(settings map[string]interface{}) map[string]interface{} {
	if len(settings) == 0 {
		return nil
	}

	cloned := make(map[string]interface{}, len(settings))
	for key, value := range settings {
		cloned[key] = value
	}
	return cloned
}

func normalizeGeneratorSettings(settings map[string]interface{}) map[string]interface{} {
	normalized := cloneGeneratorSettings(settings)
	if normalized == nil {
		normalized = map[string]interface{}{}
	}

	normalizeAlterIDSettings(normalized)
	normalizeLegacyTLSSettings(normalized)
	normalizeClientSNITLSSettings(normalized)
	normalizeNestedTransportSettings(normalized)
	return normalized
}

func normalizeAlterIDSettings(settings map[string]interface{}) {
	value, exists := firstExistingSetting(settings, "alterId", "alter_id")
	if !exists {
		return
	}
	settings["alterId"] = value
	settings["alter_id"] = value
}

func normalizeLegacyTLSSettings(settings map[string]interface{}) {
	security := strings.ToLower(strings.TrimSpace(GetSettingString(settings, "security", "")))
	if security == "" && GetSettingBool(settings, "tls", false) {
		settings["security"] = "tls"
		return
	}
	if security == "none" {
		delete(settings, "tls")
	}
}

func normalizeClientSNITLSSettings(settings map[string]interface{}) {
	sni := proxylib.ResolveSNI(settings)
	if sni == "" {
		return
	}
	settings["sni"] = sni
	settings["server_name"] = sni
}

func normalizeNestedTransportSettings(settings map[string]interface{}) {
	network := strings.ToLower(strings.TrimSpace(GetSettingString(settings, "network", "")))
	switch network {
	case "ws":
		normalizeWebSocketSettings(settings)
	case "grpc":
		normalizeGRPCSettings(settings)
	case "http", "h2":
		normalizeHTTPSettings(settings)
	case "tcp":
		normalizeTCPHeaderSettings(settings)
	default:
		if _, ok := settings["ws_settings"]; ok {
			normalizeWebSocketSettings(settings)
		}
		if _, ok := settings["grpc_settings"]; ok {
			normalizeGRPCSettings(settings)
		}
	}
}

func normalizeWebSocketSettings(settings map[string]interface{}) {
	wsSettings := getSettingMap(settings, "ws_settings")
	if path := GetSettingString(settings, "path", ""); path == "" {
		if nestedPath := GetSettingString(wsSettings, "path", ""); nestedPath != "" {
			settings["path"] = nestedPath
		}
	}
	if host := GetSettingString(settings, "host", ""); host == "" {
		if nestedHost := GetSettingString(wsSettings, "host", ""); nestedHost != "" {
			settings["host"] = nestedHost
		} else if headerHost := getHeaderHost(wsSettings["headers"]); headerHost != "" {
			settings["host"] = headerHost
		}
	}
}

func normalizeGRPCSettings(settings map[string]interface{}) {
	grpcSettings := getSettingMap(settings, "grpc_settings")
	serviceName := GetSettingString(settings, "serviceName", "")
	if serviceName == "" {
		serviceName = GetSettingString(grpcSettings, "serviceName", "")
	}
	if serviceName == "" {
		return
	}
	settings["serviceName"] = serviceName
	settings["service_name"] = serviceName
}

func normalizeHTTPSettings(settings map[string]interface{}) {
	httpSettings := getSettingMap(settings, "http_settings")
	if path := GetSettingString(settings, "path", ""); path == "" {
		if nestedPath := GetSettingString(httpSettings, "path", ""); nestedPath != "" {
			settings["path"] = nestedPath
		}
	}
	if host := GetSettingString(settings, "host", ""); host == "" {
		if nestedHost := GetSettingString(httpSettings, "host", ""); nestedHost != "" {
			settings["host"] = nestedHost
		}
	}
}

func normalizeTCPHeaderSettings(settings map[string]interface{}) {
	if GetSettingString(settings, "type", "") != "" {
		return
	}
	tcpSettings := getSettingMap(settings, "tcp_settings")
	header := getSettingMap(tcpSettings, "header")
	if !strings.EqualFold(GetSettingString(header, "type", ""), "http") {
		return
	}
	settings["type"] = "http"
	settings["headerType"] = "http"

	request := getSettingMap(header, "request")
	if path := GetSettingString(settings, "path", ""); path == "" {
		if nestedPath := GetSettingString(request, "path", ""); nestedPath != "" {
			settings["path"] = nestedPath
		}
	}
	if host := GetSettingString(settings, "host", ""); host == "" {
		headers := getSettingMap(request, "headers")
		if headerHost := getHeaderHost(headers); headerHost != "" {
			settings["host"] = headerHost
		}
	}
}

func firstExistingSetting(settings map[string]interface{}, keys ...string) (interface{}, bool) {
	for _, key := range keys {
		if value, ok := settings[key]; ok && value != nil {
			return value, true
		}
	}
	return nil, false
}

func getSettingMap(settings map[string]interface{}, key string) map[string]interface{} {
	if settings == nil {
		return map[string]interface{}{}
	}
	value, ok := settings[key]
	if !ok || value == nil {
		return map[string]interface{}{}
	}
	if typed, ok := value.(map[string]interface{}); ok {
		return typed
	}
	if typed, ok := value.(map[string]string); ok {
		converted := make(map[string]interface{}, len(typed))
		for mapKey, mapValue := range typed {
			converted[mapKey] = mapValue
		}
		return converted
	}
	return map[string]interface{}{}
}

func getHeaderHost(value interface{}) string {
	headers, ok := value.(map[string]interface{})
	if !ok {
		if stringHeaders, ok := value.(map[string]string); ok {
			for _, key := range []string{"Host", "host"} {
				if host := strings.TrimSpace(stringHeaders[key]); host != "" {
					return host
				}
			}
		}
		return ""
	}
	for _, key := range []string{"Host", "host"} {
		if host := strings.TrimSpace(GetSettingString(headers, key, "")); host != "" {
			return host
		}
	}
	return ""
}

// GetSettingString safely gets a string setting value.
func GetSettingString(settings map[string]interface{}, key string, defaultValue string) string {
	for _, candidate := range settingAliases(key) {
		if v, ok := settings[candidate]; ok {
			switch typed := v.(type) {
			case string:
				if trimmed := strings.TrimSpace(typed); trimmed != "" {
					return trimmed
				}
			case []string:
				if joined := joinStringValues(typed); joined != "" {
					return joined
				}
			case []interface{}:
				values := make([]string, 0, len(typed))
				for _, item := range typed {
					if text, ok := item.(string); ok {
						values = append(values, text)
					}
				}
				if joined := joinStringValues(values); joined != "" {
					return joined
				}
			}
		}
	}
	return defaultValue
}

func joinStringValues(values []string) string {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}
	return strings.Join(normalized, ",")
}

// GetSettingInt safely gets an int setting value.
func GetSettingInt(settings map[string]interface{}, key string, defaultValue int) int {
	for _, candidate := range settingAliases(key) {
		if v, ok := settings[candidate]; ok {
			switch typed := v.(type) {
			case int:
				return typed
			case int64:
				return int(typed)
			case float64:
				return int(typed)
			case string:
				if parsed, err := strconv.Atoi(strings.TrimSpace(typed)); err == nil {
					return parsed
				}
			}
		}
	}
	return defaultValue
}

// GetSettingBool safely gets a bool setting value.
func GetSettingBool(settings map[string]interface{}, key string, defaultValue bool) bool {
	for _, candidate := range settingAliases(key) {
		if v, ok := settings[candidate]; ok {
			switch typed := v.(type) {
			case bool:
				return typed
			case string:
				trimmed := strings.TrimSpace(typed)
				return strings.EqualFold(trimmed, "true") || strings.EqualFold(trimmed, "tls") || trimmed == "1"
			case int:
				return typed != 0
			case float64:
				return typed != 0
			}
		}
	}
	return defaultValue
}

func settingAliases(key string) []string {
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
	case "allowInsecure", "skipCertVerify":
		return []string{"allowInsecure", "skipCertVerify"}
	case "serviceName", "service_name":
		return []string{"serviceName", "service_name"}
	case "type", "headerType":
		return []string{"type", "headerType"}
	default:
		return []string{key}
	}
}

func ExtractProxyInfos(proxies []*repository.Proxy) []*ProxyInfo {
	infos := make([]*ProxyInfo, 0, len(proxies))
	for _, proxy := range proxies {
		if proxy == nil {
			continue
		}
		infos = append(infos, ExtractProxyInfo(proxy))
	}
	MakeUniqueNames(infos)
	return infos
}

// MakeUniqueNames ensures all proxy names are unique by appending suffixes if needed.
func MakeUniqueNames(proxies []*ProxyInfo) {
	nameCount := make(map[string]int)

	// First pass: count occurrences
	for _, p := range proxies {
		nameCount[p.Name]++
	}

	// Second pass: rename duplicates
	nameIndex := make(map[string]int)
	for _, p := range proxies {
		if nameCount[p.Name] > 1 {
			nameIndex[p.Name]++
			p.Name = p.Name + "-" + strconv.Itoa(nameIndex[p.Name])
		}
	}
}
