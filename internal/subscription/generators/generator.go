// Package generators provides subscription format generators for various clients.
package generators

import (
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

// ExtractProxyInfo extracts proxy information from a repository.Proxy.
func ExtractProxyInfo(proxy *repository.Proxy) *ProxyInfo {
	name := proxy.Name
	if proxy.Remark != "" {
		name = proxy.Remark
	}

	server := proxylib.ResolveServerAddress(proxy.Host, proxy.Settings)

	return &ProxyInfo{
		Name:     name,
		Protocol: proxy.Protocol,
		Server:   server,
		Port:     proxy.Port,
		Settings: proxy.Settings,
	}
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
			p.Name = p.Name + "-" + string(rune('0'+nameIndex[p.Name]))
		}
	}
}
