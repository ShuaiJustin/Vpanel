// Package vless implements the VLESS protocol.
package vless

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"v/internal/proxy"
	"v/pkg/errors"
)

// Protocol implements the VLESS protocol.
type Protocol struct{}

// New creates a new VLESS protocol.
func New() *Protocol {
	return &Protocol{}
}

// Name returns the protocol name.
func (p *Protocol) Name() string {
	return "vless"
}

// GenerateConfig generates Xray configuration for VLESS.
func (p *Protocol) GenerateConfig(settings *proxy.Settings) (json.RawMessage, error) {
	userID := settings.GetString("uuid")
	if userID == "" {
		userID = uuid.New().String()
	}

	flow := settings.GetString("flow")
	security := settings.GetString("security")
	if security == "" {
		security = "none"
	}

	config := map[string]any{
		"tag":      fmt.Sprintf("vless-%d", settings.ID),
		"protocol": "vless",
		"listen":   "0.0.0.0",
		"port":     settings.Port,
		"settings": map[string]any{
			"clients": []map[string]any{
				{
					"id":   userID,
					"flow": flow,
				},
			},
			"decryption": "none",
		},
		"streamSettings": p.buildStreamSettings(settings, security),
	}

	return json.Marshal(config)
}

// buildStreamSettings builds stream settings for VLESS.
func (p *Protocol) buildStreamSettings(settings *proxy.Settings, security string) map[string]any {
	network := settings.GetString("network")
	if network == "" {
		network = "tcp"
	}

	streamSettings := map[string]any{
		"network":  network,
		"security": security,
	}

	// Add TLS settings if enabled
	if security == "tls" || security == "reality" {
		tlsSettings := map[string]any{}
		if sni := settings.GetString("sni"); sni != "" {
			tlsSettings["serverName"] = sni
		}
		if alpn := settings.GetString("alpn"); alpn != "" {
			tlsSettings["alpn"] = strings.Split(alpn, ",")
		}
		if security == "tls" {
			streamSettings["tlsSettings"] = tlsSettings
		} else {
			// Reality settings
			realitySettings := map[string]any{
				"serverName": settings.GetString("sni"),
				"publicKey":  settings.GetString("pbk"),
				"shortId":    settings.GetString("sid"),
			}
			streamSettings["realitySettings"] = realitySettings
		}
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
	case "tcp":
		if headerType := settings.GetString("headerType"); headerType == "http" {
			streamSettings["tcpSettings"] = map[string]any{
				"header": map[string]any{
					"type": "http",
					"request": map[string]any{
						"path": []string{settings.GetString("path")},
					},
				},
			}
		}
	}

	return streamSettings
}

// GenerateLink generates a VLESS share link.
func (p *Protocol) GenerateLink(settings *proxy.Settings) (string, error) {
	userID := settings.GetString("uuid")
	if userID == "" {
		return "", errors.NewValidationError("uuid is required", nil)
	}

	// Build query parameters
	params := url.Values{}
	if flow := settings.GetString("flow"); flow != "" {
		params.Set("flow", flow)
	}
	if security := settings.GetString("security"); security != "" {
		params.Set("security", security)
	}
	if network := settings.GetString("network"); network != "" {
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
	if pbk := settings.GetString("pbk"); pbk != "" {
		params.Set("pbk", pbk)
	}
	if sid := settings.GetString("sid"); sid != "" {
		params.Set("sid", sid)
	}
	if fp := settings.GetString("fp"); fp != "" {
		params.Set("fp", fp)
	}

	server := proxy.ResolveServerAddress(settings.Host, settings.Settings)
	if server == "" {
		return "", errors.NewValidationError("server address is required", nil)
	}

	// Build link: vless://uuid@host:port?params#name
	link := fmt.Sprintf("vless://%s@%s:%d", userID, server, settings.Port)
	if len(params) > 0 {
		link += "?" + params.Encode()
	}
	if settings.Name != "" {
		link += "#" + url.PathEscape(settings.Name)
	}

	return link, nil
}

// ParseLink parses a VLESS share link.
func (p *Protocol) ParseLink(link string) (*proxy.Settings, error) {
	if !strings.HasPrefix(link, "vless://") {
		return nil, errors.NewValidationError("invalid vless link format", nil)
	}

	// Remove prefix
	link = strings.TrimPrefix(link, "vless://")

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

	// Parse uuid@host:port
	atIdx := strings.Index(link, "@")
	if atIdx == -1 {
		return nil, errors.NewValidationError("invalid vless link: missing @", nil)
	}

	userID := link[:atIdx]
	hostPort := link[atIdx+1:]

	// Parse host:port
	colonIdx := strings.LastIndex(hostPort, ":")
	if colonIdx == -1 {
		return nil, errors.NewValidationError("invalid vless link: missing port", nil)
	}

	host := hostPort[:colonIdx]
	port, err := strconv.Atoi(hostPort[colonIdx+1:])
	if err != nil {
		return nil, errors.NewValidationError("invalid port", err)
	}

	settings := &proxy.Settings{
		Name:     name,
		Protocol: "vless",
		Host:     host,
		Port:     port,
		Settings: map[string]any{
			"uuid":     userID,
			"flow":     params.Get("flow"),
			"security": params.Get("security"),
			"network":  params.Get("type"),
			"sni":      params.Get("sni"),
			"alpn":     params.Get("alpn"),
			"path":     params.Get("path"),
			"host":     params.Get("host"),
			"pbk":      params.Get("pbk"),
			"sid":      params.Get("sid"),
			"fp":       params.Get("fp"),
		},
		Enabled: true,
	}

	return settings, nil
}

// Validate validates VLESS settings.
func (p *Protocol) Validate(settings *proxy.Settings) error {
	if settings.Port < 1 || settings.Port > 65535 {
		return errors.NewValidationError("port must be between 1 and 65535", nil)
	}

	userID := settings.GetString("uuid")
	if userID != "" {
		if _, err := uuid.Parse(userID); err != nil {
			return errors.NewValidationError("invalid uuid format", err)
		}
	}

	// Validate security type
	security := settings.GetString("security")
	validSecurities := map[string]bool{"": true, "none": true, "tls": true, "reality": true}
	if !validSecurities[security] {
		return errors.NewValidationError("invalid security type", nil)
	}

	return nil
}

// DefaultSettings returns default VLESS settings.
func (p *Protocol) DefaultSettings() map[string]any {
	return map[string]any{
		"uuid":     uuid.New().String(),
		"flow":     "",
		"security": "none",
		"network":  "tcp",
	}
}

// EncodeBase64 encodes settings to base64 (for compatibility).
func EncodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
