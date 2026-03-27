// Package vmess implements the VMess protocol.
package vmess

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"v/internal/proxy"
	"v/pkg/errors"
)

// Protocol implements the VMess protocol.
type Protocol struct{}

type linkConfig struct {
	V             string `json:"v"`
	PS            string `json:"ps"`
	Add           string `json:"add"`
	Port          string `json:"port"`
	ID            string `json:"id"`
	Aid           string `json:"aid"`
	Scy           string `json:"scy"`
	Net           string `json:"net"`
	Type          string `json:"type"`
	Host          string `json:"host"`
	Path          string `json:"path"`
	TLS           string `json:"tls"`
	SNI           string `json:"sni"`
	ALPN          string `json:"alpn,omitempty"`
	FP            string `json:"fp,omitempty"`
	AllowInsecure bool   `json:"allowInsecure,omitempty"`
}

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
		"streamSettings": p.buildStreamSettings(settings, security),
	}

	return json.Marshal(config)
}

func (p *Protocol) buildStreamSettings(settings *proxy.Settings, security string) map[string]any {
	network := settings.GetString("network")
	if network == "" {
		network = "tcp"
	}

	streamSettings := map[string]any{
		"network":  network,
		"security": security,
	}

	if security == "tls" {
		tlsSettings := map[string]any{}
		if sni := proxy.ResolveSNI(settings.Settings); sni != "" {
			tlsSettings["serverName"] = sni
		}
		if alpn := settings.GetString("alpn"); alpn != "" {
			tlsSettings["alpn"] = strings.Split(alpn, ",")
		}
		if settings.GetBool("allowInsecure") {
			tlsSettings["allowInsecure"] = true
		}
		streamSettings["tlsSettings"] = tlsSettings
	}

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

	linkData := linkConfig{
		V:             "2",
		PS:            settings.Name,
		Add:           server,
		Port:          strconv.Itoa(proxy.ResolveServerPort(settings.Port, settings.Settings)),
		ID:            userID,
		Aid:           strconv.Itoa(settings.GetInt("alterId")),
		Scy:           proxy.ResolveVMessCipher(settings.Settings),
		Net:           firstNonEmpty(settings.GetString("network"), "tcp"),
		Type:          firstNonEmpty(settings.GetString("type"), "none"),
		Host:          settings.GetString("host"),
		Path:          settings.GetString("path"),
		TLS:           tlsValue,
		SNI:           proxy.ResolveSNI(settings.Settings),
		ALPN:          settings.GetString("alpn"),
		FP:            firstNonEmpty(settings.GetString("fingerprint"), settings.GetString("fp")),
		AllowInsecure: settings.GetBool("allowInsecure"),
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

	port := getInt(linkData, "port")

	settings := &proxy.Settings{
		Name:     getString(linkData, "ps"),
		Protocol: "vmess",
		Host:     getString(linkData, "add"),
		Port:     port,
		Settings: map[string]any{
			"uuid":          getString(linkData, "id"),
			"alterId":       getInt(linkData, "aid"),
			"network":       getString(linkData, "net"),
			"type":          getString(linkData, "type"),
			"host":          getString(linkData, "host"),
			"path":          getString(linkData, "path"),
			"tls":           getString(linkData, "tls"),
			"sni":           getString(linkData, "sni"),
			"alpn":          getString(linkData, "alpn"),
			"fp":            getString(linkData, "fp"),
			"cipher":        getString(linkData, "scy"),
			"scy":           getString(linkData, "scy"),
			"allowInsecure": getBool(linkData, "allowInsecure"),
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
		switch typed := v.(type) {
		case string:
			return typed
		case float64:
			return strconv.Itoa(int(typed))
		case int:
			return strconv.Itoa(typed)
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
		case string:
			parsed, err := strconv.Atoi(strings.TrimSpace(val))
			if err == nil {
				return parsed
			}
		}
	}
	return 0
}

func getBool(m map[string]any, key string) bool {
	if v, ok := m[key]; ok {
		switch typed := v.(type) {
		case bool:
			return typed
		case string:
			return typed == "1" || typed == "true"
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
