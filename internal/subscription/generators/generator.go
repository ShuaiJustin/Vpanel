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
	switch strings.ToLower(strings.TrimSpace(protocol)) {
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
	server := proxylib.ResolveExternalServerAddress(proxy.Settings)
	if server == "" {
		server = proxylib.ResolveServerAddress(proxy.Host, proxy.Settings)
	}
	settingsCopy := cloneGeneratorSettings(proxy.Settings)
	if settingsCopy == nil {
		settingsCopy = map[string]interface{}{}
	}
	settingsCopy["server"] = server
	return &ProxyInfo{
		Name:     buildSubscriptionProxyName(proxy, server),
		Protocol: proxy.Protocol,
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

// GetSettingString safely gets a string setting value.
func GetSettingString(settings map[string]interface{}, key string, defaultValue string) string {
	for _, candidate := range settingAliases(key) {
		if v, ok := settings[candidate].(string); ok {
			if trimmed := strings.TrimSpace(v); trimmed != "" {
				return trimmed
			}
		}
	}
	return defaultValue
}

// GetSettingInt safely gets an int setting value.
func GetSettingInt(settings map[string]interface{}, key string, defaultValue int) int {
	if v, ok := settings[key].(float64); ok {
		return int(v)
	}
	if v, ok := settings[key].(int); ok {
		return v
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
				return strings.EqualFold(trimmed, "true") || trimmed == "1"
			}
		}
	}
	return defaultValue
}

func settingAliases(key string) []string {
	switch key {
	case "fingerprint", "fp":
		return []string{"fingerprint", "fp"}
	case "publicKey", "pbk":
		return []string{"publicKey", "pbk"}
	case "shortId", "sid":
		return []string{"shortId", "sid"}
	case "sni", "server_name":
		return []string{"sni", "server_name"}
	default:
		return []string{key}
	}
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
