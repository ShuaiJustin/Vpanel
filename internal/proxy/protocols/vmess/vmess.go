// Package vmess implements the VMess protocol.
package vmess

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"v/internal/proxy"
	"v/pkg/errors"
)

// Protocol implements the VMess protocol.
type Protocol struct{}

// New creates a new VMess protocol.
func New() *Protocol {
	return &Protocol{}
}

// Name returns the protocol name.
func (p *Protocol) Name() string {
	return "vmess"
}

// GenerateConfig generates Xray configuration for VMess.
func (p *Protocol) GenerateConfig(settings *proxy.Settings) (json.RawMessage, error) {
	userID := settings.GetString("uuid")
	if userID == "" {
		userID = uuid.New().String()
	}

	alterId := settings.GetInt("alterId")
	security := settings.GetString("security")
	if security == "" {
		security = "none"
	}

	config := map[string]any{
		"tag":      fmt.Sprintf("vmess-%d", settings.ID),
		"protocol": "vmess",
		"listen":   "0.0.0.0",
		"port":     settings.Port,
		"settings": map[string]any{
			"clients": []map[string]any{
				{
					"id":      userID,
					"alterId": alterId,
				},
			},
		},
		"streamSettings": map[string]any{
			"network":  settings.GetString("network"),
			"security": security,
		},
	}

	return json.Marshal(config)
}

// GenerateLink generates a VMess share link.
func (p *Protocol) GenerateLink(settings *proxy.Settings) (string, error) {
	userID := settings.GetString("uuid")
	if userID == "" {
		return "", errors.NewValidationError("uuid is required", nil)
	}

	server := proxy.ResolveServerAddress(settings.Host, settings.Settings)
	if server == "" {
		return "", errors.NewValidationError("server address is required", nil)
	}

	tlsValue := ""
	if proxy.HasTLSSettings(settings.Settings) {
		tlsValue = "tls"
	}

	linkData := map[string]any{
		"v":    "2",
		"ps":   settings.Name,
		"add":  server,
		"port": settings.Port,
		"id":   userID,
		"aid":  settings.GetInt("alterId"),
		"net":  settings.GetString("network"),
		"type": settings.GetString("type"),
		"host": settings.GetString("host"),
		"path": settings.GetString("path"),
		"tls":  tlsValue,
		"sni":  proxy.ResolveSNI(settings.Settings),
	}

	jsonData, err := json.Marshal(linkData)
	if err != nil {
		return "", errors.NewInternalError("failed to marshal link data", err)
	}

	return "vmess://" + base64.StdEncoding.EncodeToString(jsonData), nil
}

// ParseLink parses a VMess share link.
func (p *Protocol) ParseLink(link string) (*proxy.Settings, error) {
	if len(link) < 8 || link[:8] != "vmess://" {
		return nil, errors.NewValidationError("invalid vmess link format", nil)
	}

	encoded := link[8:]
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errors.NewValidationError("failed to decode vmess link", err)
	}

	var linkData map[string]any
	if err := json.Unmarshal(decoded, &linkData); err != nil {
		return nil, errors.NewValidationError("failed to parse vmess link", err)
	}

	port := 0
	if v, ok := linkData["port"]; ok {
		switch val := v.(type) {
		case float64:
			port = int(val)
		case int:
			port = val
		}
	}

	settings := &proxy.Settings{
		Name:     getString(linkData, "ps"),
		Protocol: "vmess",
		Host:     getString(linkData, "add"),
		Port:     port,
		Settings: map[string]any{
			"uuid":    getString(linkData, "id"),
			"alterId": getInt(linkData, "aid"),
			"network": getString(linkData, "net"),
			"type":    getString(linkData, "type"),
			"host":    getString(linkData, "host"),
			"path":    getString(linkData, "path"),
			"tls":     getString(linkData, "tls"),
		},
		Enabled: true,
	}

	return settings, nil
}

// Validate validates VMess settings.
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

	return nil
}

// DefaultSettings returns default VMess settings.
func (p *Protocol) DefaultSettings() map[string]any {
	return map[string]any{
		"uuid":     uuid.New().String(),
		"alterId":  0,
		"network":  "tcp",
		"security": "none",
	}
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getInt(m map[string]any, key string) int {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case float64:
			return int(val)
		case int:
			return val
		}
	}
	return 0
}
