// Package shadowsocks implements the Shadowsocks protocol.
package shadowsocks

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"v/internal/proxy"
	"v/pkg/errors"
)

// Supported encryption methods.
var supportedMethods = map[string]bool{
	"aes-128-gcm":             true,
	"aes-256-gcm":             true,
	"chacha20-ietf-poly1305":  true,
	"2022-blake3-aes-128-gcm": true,
	"2022-blake3-aes-256-gcm": true,
	"none":                    true,
	"plain":                   true,
}

// Protocol implements the Shadowsocks protocol.
type Protocol struct{}

// New creates a new Shadowsocks protocol.
func New() *Protocol {
	return &Protocol{}
}

// Name returns the protocol name.
func (p *Protocol) Name() string {
	return "shadowsocks"
}

// GenerateConfig generates Xray configuration for Shadowsocks.
func (p *Protocol) GenerateConfig(settings *proxy.Settings) (json.RawMessage, error) {
	method := settings.GetString("method")
	if method == "" {
		method = "aes-256-gcm"
	}

	password := settings.GetString("password")
	if password == "" {
		return nil, errors.NewValidationError("password is required", nil)
	}

	config := map[string]any{
		"tag":      fmt.Sprintf("shadowsocks-%d", settings.ID),
		"protocol": "shadowsocks",
		"listen":   "0.0.0.0",
		"port":     settings.Port,
		"settings": map[string]any{
			"method":   method,
			"password": password,
			"network":  settings.GetString("network"),
		},
	}

	// Add stream settings if needed
	network := settings.GetString("network")
	if network != "" && network != "tcp,udp" {
		config["streamSettings"] = p.buildStreamSettings(settings)
	}

	return json.Marshal(config)
}

// buildStreamSettings builds stream settings for Shadowsocks.
func (p *Protocol) buildStreamSettings(settings *proxy.Settings) map[string]any {
	network := settings.GetString("network")
	if network == "" {
		network = "tcp"
	}

	streamSettings := map[string]any{
		"network": network,
	}

	// Add network-specific settings
	switch network {
	case "ws":
		wsSettings := map[string]any{
			"path": settings.GetString("path"),
		}
		if host := settings.GetString("host"); host != "" {
			wsSettings["headers"] = map[string]any{"Host": host}
		}
		streamSettings["wsSettings"] = wsSettings
	case "grpc":
		streamSettings["grpcSettings"] = map[string]any{
			"serviceName": settings.GetString("serviceName"),
		}
	}

	// Add TLS settings if enabled
	if security := settings.GetString("security"); security == "tls" {
		streamSettings["security"] = "tls"
		tlsSettings := map[string]any{}
		if sni := settings.GetString("sni"); sni != "" {
			tlsSettings["serverName"] = sni
		}
		streamSettings["tlsSettings"] = tlsSettings
	}

	return streamSettings
}

// GenerateLink generates a Shadowsocks share link.
// Supports both SIP002 and legacy formats.
func (p *Protocol) GenerateLink(settings *proxy.Settings) (string, error) {
	method := settings.GetString("method")
	if method == "" {
		method = "aes-256-gcm"
	}

	password := settings.GetString("password")
	if password == "" {
		return "", errors.NewValidationError("password is required", nil)
	}

	server := proxy.ResolveServerAddress(settings.Host, settings.Settings)
	if server == "" {
		return "", errors.NewValidationError("server address is required", nil)
	}

	// Use SIP002 format: ss://base64(method:password)@host:port#name
	userInfo := base64.URLEncoding.EncodeToString([]byte(method + ":" + password))

	link := fmt.Sprintf("ss://%s@%s:%d", userInfo, server, settings.Port)

	// Add plugin if specified
	if plugin := settings.GetString("plugin"); plugin != "" {
		params := url.Values{}
		pluginOpts := settings.GetString("plugin-opts")
		if pluginOpts != "" {
			params.Set("plugin", plugin+";"+pluginOpts)
		} else {
			params.Set("plugin", plugin)
		}
		link += "?" + params.Encode()
	}

	if settings.Name != "" {
		link += "#" + url.PathEscape(settings.Name)
	}

	return link, nil
}

// ParseLink parses a Shadowsocks share link.
// Supports both SIP002 and legacy formats.
func (p *Protocol) ParseLink(link string) (*proxy.Settings, error) {
	if !strings.HasPrefix(link, "ss://") {
		return nil, errors.NewValidationError("invalid shadowsocks link format", nil)
	}

	// Remove prefix
	link = strings.TrimPrefix(link, "ss://")

	// Parse fragment (name)
	var name string
	if idx := strings.Index(link, "#"); idx != -1 {
		name, _ = url.PathUnescape(link[idx+1:])
		link = link[:idx]
	}

	// Parse query parameters (for plugin)
	var params url.Values
	if idx := strings.Index(link, "?"); idx != -1 {
		var err error
		params, err = url.ParseQuery(link[idx+1:])
		if err != nil {
			return nil, errors.NewValidationError("failed to parse query parameters", err)
		}
		link = link[:idx]
	}

	var method, password, host string
	var port int

	// Try SIP002 format first: base64(method:password)@host:port
	if atIdx := strings.Index(link, "@"); atIdx != -1 {
		// SIP002 format
		userInfo := link[:atIdx]
		hostPort := link[atIdx+1:]

		// Decode userinfo
		decoded, err := base64.URLEncoding.DecodeString(userInfo)
		if err != nil {
			// Try standard base64
			decoded, err = base64.StdEncoding.DecodeString(userInfo)
			if err != nil {
				return nil, errors.NewValidationError("failed to decode userinfo", err)
			}
		}

		parts := strings.SplitN(string(decoded), ":", 2)
		if len(parts) != 2 {
			return nil, errors.NewValidationError("invalid userinfo format", nil)
		}
		method = parts[0]
		password = parts[1]

		// Parse host:port
		colonIdx := strings.LastIndex(hostPort, ":")
		if colonIdx == -1 {
			return nil, errors.NewValidationError("invalid shadowsocks link: missing port", nil)
		}
		host = hostPort[:colonIdx]
		var err2 error
		port, err2 = strconv.Atoi(hostPort[colonIdx+1:])
		if err2 != nil {
			return nil, errors.NewValidationError("invalid port", err2)
		}
	} else {
		// Legacy format: base64(method:password@host:port)
		decoded, err := base64.URLEncoding.DecodeString(link)
		if err != nil {
			decoded, err = base64.StdEncoding.DecodeString(link)
			if err != nil {
				return nil, errors.NewValidationError("failed to decode legacy link", err)
			}
		}

		// Parse method:password@host:port
		atIdx := strings.LastIndex(string(decoded), "@")
		if atIdx == -1 {
			return nil, errors.NewValidationError("invalid legacy link format", nil)
		}

		methodPassword := string(decoded[:atIdx])
		hostPort := string(decoded[atIdx+1:])

		parts := strings.SplitN(methodPassword, ":", 2)
		if len(parts) != 2 {
			return nil, errors.NewValidationError("invalid method:password format", nil)
		}
		method = parts[0]
		password = parts[1]

		colonIdx := strings.LastIndex(hostPort, ":")
		if colonIdx == -1 {
			return nil, errors.NewValidationError("invalid host:port format", nil)
		}
		host = hostPort[:colonIdx]
		var err2 error
		port, err2 = strconv.Atoi(hostPort[colonIdx+1:])
		if err2 != nil {
			return nil, errors.NewValidationError("invalid port", err2)
		}
	}

	settingsMap := map[string]any{
		"method":   method,
		"password": password,
	}

	// Parse plugin if present
	if params != nil {
		if plugin := params.Get("plugin"); plugin != "" {
			parts := strings.SplitN(plugin, ";", 2)
			settingsMap["plugin"] = parts[0]
			if len(parts) > 1 {
				settingsMap["plugin-opts"] = parts[1]
			}
		}
	}

	settings := &proxy.Settings{
		Name:     name,
		Protocol: "shadowsocks",
		Host:     host,
		Port:     port,
		Settings: settingsMap,
		Enabled:  true,
	}

	return settings, nil
}

// Validate validates Shadowsocks settings.
func (p *Protocol) Validate(settings *proxy.Settings) error {
	if settings.Port < 1 || settings.Port > 65535 {
		return errors.NewValidationError("port must be between 1 and 65535", nil)
	}

	method := settings.GetString("method")
	if method != "" && !supportedMethods[method] {
		return errors.NewValidationError("unsupported encryption method: "+method, nil)
	}

	password := settings.GetString("password")
	if password == "" {
		return errors.NewValidationError("password is required", nil)
	}

	return nil
}

// DefaultSettings returns default Shadowsocks settings.
func (p *Protocol) DefaultSettings() map[string]any {
	return map[string]any{
		"method":   "aes-256-gcm",
		"password": generateRandomPassword(),
		"network":  "tcp,udp",
	}
}

// generateRandomPassword generates a random password for Shadowsocks.
func generateRandomPassword() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	password := make([]byte, 16)
	for i := range password {
		password[i] = charset[i%len(charset)]
	}
	return string(password)
}
