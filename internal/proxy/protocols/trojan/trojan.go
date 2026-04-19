// Package trojan implements the Trojan protocol.
package trojan

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"v/internal/proxy"
	"v/pkg/errors"
)

// Protocol implements the Trojan protocol.
type Protocol struct{}

// New creates a new Trojan protocol.
func New() *Protocol {
	return &Protocol{}
}

// Name returns the protocol name.
func (p *Protocol) Name() string {
	return "trojan"
}

// GenerateConfig generates Xray configuration for Trojan.
func (p *Protocol) GenerateConfig(settings *proxy.Settings) (json.RawMessage, error) {
	password := settings.GetString("password")
	if password == "" {
		return nil, errors.NewValidationError("password is required", nil)
	}

	security := settings.GetString("security")
	if security == "" {
		security = "tls"
	}

	config := map[string]any{
		"tag":      fmt.Sprintf("trojan-%d", settings.ID),
		"protocol": "trojan",
		"listen":   "0.0.0.0",
		"port":     settings.Port,
		"settings": map[string]any{
			"clients": []map[string]any{
				{
					"password": password,
				},
			},
		},
		"streamSettings": p.buildStreamSettings(settings, security),
	}

	return json.Marshal(config)
}

// buildStreamSettings builds stream settings for Trojan.
func (p *Protocol) buildStreamSettings(settings *proxy.Settings, security string) map[string]any {
	network := settings.GetString("network")
	if network == "" {
		network = "tcp"
	}

	streamSettings := map[string]any{
		"network":  network,
		"security": security,
	}

	// Add TLS settings
	if security == "tls" {
		tlsSettings := map[string]any{
			"allowInsecure": settings.GetBool("allowInsecure"),
		}
		if sni := settings.GetString("sni"); sni != "" {
			tlsSettings["serverName"] = sni
		}
		if alpn := settings.GetString("alpn"); alpn != "" {
			tlsSettings["alpn"] = strings.Split(alpn, ",")
		}
		streamSettings["tlsSettings"] = tlsSettings
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

	return streamSettings
}

// GenerateLink generates a Trojan share link.
func (p *Protocol) GenerateLink(settings *proxy.Settings) (string, error) {
	password := settings.GetString("password")
	if password == "" {
		return "", errors.NewValidationError("password is required", nil)
	}

	// Build query parameters
	params := url.Values{}
	if security := settings.GetString("security"); security != "" {
		params.Set("security", security)
	}
	if network := settings.GetString("network"); network != "" && network != "tcp" {
		params.Set("type", network)
	}
	if sni := settings.GetString("sni"); sni != "" {
		params.Set("sni", sni)
	}
	if alpn := settings.GetString("alpn"); alpn != "" {
		params.Set("alpn", alpn)
	}
	if path := settings.GetString("path"); path != "" {
		params.Set("path", path)
	}
	if host := settings.GetString("host"); host != "" {
		params.Set("host", host)
	}
	if fp := settings.GetString("fp"); fp != "" {
		params.Set("fp", fp)
	}
	if settings.GetBool("allowInsecure") {
		params.Set("allowInsecure", "1")
	}

	server := proxy.ResolveServerAddress(settings.Host, settings.Settings)
	if server == "" {
		return "", errors.NewValidationError("server address is required", nil)
	}

	// Build link: trojan://password@host:port?params#name
	link := fmt.Sprintf("trojan://%s@%s:%d", url.PathEscape(password), server, settings.Port)
	if len(params) > 0 {
		link += "?" + params.Encode()
	}
	if settings.Name != "" {
		link += "#" + url.PathEscape(settings.Name)
	}

	return link, nil
}

// ParseLink parses a Trojan share link.
func (p *Protocol) ParseLink(link string) (*proxy.Settings, error) {
	if !strings.HasPrefix(link, "trojan://") {
		return nil, errors.NewValidationError("invalid trojan link format", nil)
	}

	// Remove prefix
	link = strings.TrimPrefix(link, "trojan://")

	// Parse fragment (name)
	var name string
	if idx := strings.Index(link, "#"); idx != -1 {
		name, _ = url.PathUnescape(link[idx+1:])
		link = link[:idx]
	}

	// Parse query parameters
	var params url.Values
	if idx := strings.Index(link, "?"); idx != -1 {
		var err error
		params, err = url.ParseQuery(link[idx+1:])
		if err != nil {
			return nil, errors.NewValidationError("failed to parse query parameters", err)
		}
		link = link[:idx]
	}

	// Parse password@host:port
	atIdx := strings.Index(link, "@")
	if atIdx == -1 {
		return nil, errors.NewValidationError("invalid trojan link: missing @", nil)
	}

	password, _ := url.PathUnescape(link[:atIdx])
	hostPort := link[atIdx+1:]

	// Parse host:port
	colonIdx := strings.LastIndex(hostPort, ":")
	if colonIdx == -1 {
		return nil, errors.NewValidationError("invalid trojan link: missing port", nil)
	}

	host := hostPort[:colonIdx]
	port, err := strconv.Atoi(hostPort[colonIdx+1:])
	if err != nil {
		return nil, errors.NewValidationError("invalid port", err)
	}

	settings := &proxy.Settings{
		Name:     name,
		Protocol: "trojan",
		Host:     host,
		Port:     port,
		Settings: map[string]any{
			"password": password,
			"security": params.Get("security"),
			"network":  params.Get("type"),
			"sni":      params.Get("sni"),
			"alpn":     params.Get("alpn"),
			"path":     params.Get("path"),
			"host":     params.Get("host"),
			"fp":       params.Get("fp"),
			"allowInsecure": params.Get("allowInsecure") == "1" || strings.EqualFold(params.Get("allowInsecure"), "true"),
		},
		Enabled: true,
	}

	// Default security to tls if not specified
	if settings.Settings["security"] == "" {
		settings.Settings["security"] = "tls"
	}

	return settings, nil
}

// Validate validates Trojan settings.
func (p *Protocol) Validate(settings *proxy.Settings) error {
	if settings.Port < 1 || settings.Port > 65535 {
		return errors.NewValidationError("port must be between 1 and 65535", nil)
	}

	password := settings.GetString("password")
	if password == "" {
		return errors.NewValidationError("password is required", nil)
	}

	// Validate security type
	security := settings.GetString("security")
	validSecurities := map[string]bool{"": true, "tls": true, "none": true}
	if !validSecurities[security] {
		return errors.NewValidationError("invalid security type", nil)
	}

	return nil
}

// DefaultSettings returns default Trojan settings.
func (p *Protocol) DefaultSettings() map[string]any {
	return map[string]any{
		"password": generateRandomPassword(),
		"security": "tls",
		"network":  "tcp",
	}
}

// generateRandomPassword generates a cryptographically random Trojan password.
func generateRandomPassword() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const length = 32
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand should never fail on supported platforms; if it does, the
		// only safe action is to refuse a predictable fallback. Return empty so
		// the caller's validation ("password is required") rejects the proxy.
		return ""
	}
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b)
}
