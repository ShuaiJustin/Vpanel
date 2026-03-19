// Package proxy provides proxy protocol management for the V Panel application.
package proxy

import (
	"encoding/json"
	"strings"
)

// Protocol defines the interface for proxy protocols.
type Protocol interface {
	// Name returns the protocol name.
	Name() string

	// GenerateConfig generates Xray configuration for this protocol.
	GenerateConfig(settings *Settings) (json.RawMessage, error)

	// GenerateLink generates a share link for this protocol.
	GenerateLink(settings *Settings) (string, error)

	// ParseLink parses a share link and returns settings.
	ParseLink(link string) (*Settings, error)

	// Validate validates the protocol settings.
	Validate(settings *Settings) error

	// DefaultSettings returns default settings for this protocol.
	DefaultSettings() map[string]any
}

// Settings represents proxy configuration settings.
type Settings struct {
	ID       int64          `json:"id"`
	Name     string         `json:"name"`
	Protocol string         `json:"protocol"`
	Port     int            `json:"port"`
	Host     string         `json:"host,omitempty"`
	Settings map[string]any `json:"settings"`
	Enabled  bool           `json:"enabled"`
	Remark   string         `json:"remark,omitempty"`
}

// GetString gets a string value from settings.
func (s *Settings) GetString(key string) string {
	if s.Settings == nil {
		return ""
	}
	for _, candidate := range settingAliasKeys(key) {
		if v, ok := s.Settings[candidate]; ok {
			if str, ok := v.(string); ok {
				return strings.TrimSpace(str)
			}
		}
	}
	return ""
}

// GetInt gets an int value from settings.
func (s *Settings) GetInt(key string) int {
	if s.Settings == nil {
		return 0
	}
	if v, ok := s.Settings[key]; ok {
		switch val := v.(type) {
		case int:
			return val
		case int64:
			return int(val)
		case float64:
			return int(val)
		}
	}
	return 0
}

// GetBool gets a bool value from settings.
func (s *Settings) GetBool(key string) bool {
	if s.Settings == nil {
		return false
	}
	for _, candidate := range settingAliasKeys(key) {
		if v, ok := s.Settings[candidate]; ok {
			switch typed := v.(type) {
			case bool:
				return typed
			case string:
				return strings.EqualFold(strings.TrimSpace(typed), "true") || strings.TrimSpace(typed) == "1"
			case int:
				return typed != 0
			case float64:
				return typed != 0
			}
		}
	}
	return false
}

// SetValue sets a value in settings.
func (s *Settings) SetValue(key string, value any) {
	if s.Settings == nil {
		s.Settings = make(map[string]any)
	}
	s.Settings[key] = value
}

func settingAliasKeys(key string) []string {
	switch key {
	case "fp", "fingerprint":
		return []string{"fp", "fingerprint"}
	case "pbk", "publicKey":
		return []string{"pbk", "publicKey"}
	case "sid", "shortId":
		return []string{"sid", "shortId"}
	case "sni", "server_name":
		return []string{"sni", "server_name"}
	case "serviceName", "service_name":
		return []string{"serviceName", "service_name"}
	default:
		return []string{key}
	}
}
