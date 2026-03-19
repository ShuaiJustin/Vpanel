package proxy

import (
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/curve25519"
)

// NormalizeSettings standardizes proxy settings so all downstream generators
// can rely on a stable field layout regardless of which aliases the caller sent.
func NormalizeSettings(protocol string, settings map[string]any) (map[string]any, error) {
	normalized := cloneSettingsMap(settings)

	serviceName := firstSettingString(normalized, "serviceName", "service_name")
	if serviceName != "" {
		normalized["serviceName"] = serviceName
		normalized["service_name"] = serviceName
	}

	sni := firstSettingString(normalized, "sni", "server_name")
	if sni != "" {
		normalized["sni"] = sni
		normalized["server_name"] = sni
	}

	fingerprint := firstSettingString(normalized, "fingerprint", "fp")
	if fingerprint != "" {
		normalized["fingerprint"] = fingerprint
		normalized["fp"] = fingerprint
	} else {
		delete(normalized, "fingerprint")
		delete(normalized, "fp")
	}

	security := strings.ToLower(strings.TrimSpace(firstSettingString(normalized, "security")))
	if security == "" {
		security = "none"
	}
	normalized["security"] = security

	if security == "tls" {
		normalized["allowInsecure"] = getSettingBoolValue(normalized, "allowInsecure")
	} else {
		delete(normalized, "allowInsecure")
	}

	if security == "reality" {
		if strings.ToLower(strings.TrimSpace(protocol)) != "vless" {
			return nil, fmt.Errorf("Reality 目前仅支持 VLESS")
		}

		realitySettings := getSettingMap(normalized, "reality_settings")
		dest := firstSettingString(realitySettings, "dest")
		privateKey := firstSettingString(realitySettings, "privateKey")
		publicKey := firstSettingString(realitySettings, "publicKey")
		if publicKey == "" {
			publicKey = firstSettingString(normalized, "publicKey", "pbk")
		}
		if privateKey == "" {
			privateKey = firstSettingString(normalized, "privateKey")
		}
		serverNames := getSettingStringSlice(realitySettings, "serverNames")
		if len(serverNames) == 0 {
			serverNames = splitCommaValues(firstSettingString(normalized, "sni", "server_name"))
		}
		shortIDs := getSettingStringSlice(realitySettings, "shortIds")
		if len(shortIDs) == 0 {
			if shortID := firstSettingString(normalized, "shortId", "sid"); shortID != "" {
				shortIDs = []string{shortID}
			}
		}
		if len(shortIDs) == 0 {
			shortIDs = []string{""}
		}

		if dest == "" {
			return nil, fmt.Errorf("Reality 需要填写目标地址")
		}
		if len(serverNames) == 0 {
			return nil, fmt.Errorf("Reality 需要填写至少一个 Server Name")
		}
		if privateKey == "" {
			return nil, fmt.Errorf("Reality 需要填写私钥")
		}

		if publicKey == "" {
			derivedKey, err := DeriveRealityPublicKey(privateKey)
			if err != nil {
				return nil, fmt.Errorf("Reality 私钥无效: %w", err)
			}
			publicKey = derivedKey
		}

		normalized["sni"] = serverNames[0]
		normalized["server_name"] = serverNames[0]
		normalized["publicKey"] = publicKey
		normalized["pbk"] = publicKey
		normalized["shortId"] = strings.TrimSpace(shortIDs[0])
		normalized["sid"] = strings.TrimSpace(shortIDs[0])
		normalized["privateKey"] = privateKey

		normalized["reality_settings"] = map[string]any{
			"show":        getSettingBoolValue(realitySettings, "show"),
			"dest":        dest,
			"xver":        getSettingIntValue(realitySettings, "xver"),
			"serverNames": serverNames,
			"privateKey":  privateKey,
			"shortIds":    shortIDs,
		}
	} else {
		delete(normalized, "reality_settings")
		delete(normalized, "publicKey")
		delete(normalized, "pbk")
		delete(normalized, "shortId")
		delete(normalized, "sid")
		delete(normalized, "privateKey")
	}

	return normalized, nil
}

// DeriveRealityPublicKey derives the X25519 public key from an Xray-style
// Reality private key.
func DeriveRealityPublicKey(privateKey string) (string, error) {
	privateKey = strings.TrimSpace(privateKey)
	if privateKey == "" {
		return "", fmt.Errorf("empty private key")
	}

	privateBytes, err := decodeRealityKey(privateKey)
	if err != nil {
		return "", err
	}
	if len(privateBytes) != 32 {
		return "", fmt.Errorf("expected 32 bytes, got %d", len(privateBytes))
	}

	publicKey, err := curve25519.X25519(privateBytes, curve25519.Basepoint)
	if err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(publicKey), nil
}

func cloneSettingsMap(settings map[string]any) map[string]any {
	if settings == nil {
		return map[string]any{}
	}

	cloned := make(map[string]any, len(settings))
	for key, value := range settings {
		cloned[key] = value
	}
	return cloned
}

func firstSettingString(settings map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := settings[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if trimmed := strings.TrimSpace(typed); trimmed != "" {
				return trimmed
			}
		case []string:
			for _, item := range typed {
				if trimmed := strings.TrimSpace(item); trimmed != "" {
					return trimmed
				}
			}
		case []any:
			for _, item := range typed {
				if text, ok := item.(string); ok {
					if trimmed := strings.TrimSpace(text); trimmed != "" {
						return trimmed
					}
				}
			}
		}
	}
	return ""
}

func getSettingMap(settings map[string]any, key string) map[string]any {
	value, ok := settings[key]
	if !ok || value == nil {
		return map[string]any{}
	}
	if typed, ok := value.(map[string]any); ok {
		return cloneSettingsMap(typed)
	}
	return map[string]any{}
}

func getSettingStringSlice(settings map[string]any, key string) []string {
	value, ok := settings[key]
	if !ok || value == nil {
		return nil
	}

	switch typed := value.(type) {
	case string:
		return splitCommaValues(typed)
	case []string:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			result = append(result, splitCommaValues(item)...)
		}
		if len(result) == 0 {
			return nil
		}
		return result
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				result = append(result, splitCommaValues(text)...)
			}
		}
		if len(result) == 0 {
			return nil
		}
		return result
	default:
		return nil
	}
}

func splitCommaValues(value string) []string {
	parts := strings.Split(strings.TrimSpace(value), ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func getSettingBoolValue(settings map[string]any, key string) bool {
	value, ok := settings[key]
	if !ok || value == nil {
		return false
	}

	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		trimmed := strings.TrimSpace(typed)
		return strings.EqualFold(trimmed, "true") || trimmed == "1"
	case int:
		return typed != 0
	case float64:
		return typed != 0
	default:
		return false
	}
}

func getSettingIntValue(settings map[string]any, key string) int {
	value, ok := settings[key]
	if !ok || value == nil {
		return 0
	}

	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func decodeRealityKey(value string) ([]byte, error) {
	decoders := []*base64.Encoding{
		base64.RawURLEncoding,
		base64.URLEncoding,
		base64.RawStdEncoding,
		base64.StdEncoding,
	}

	for _, decoder := range decoders {
		decoded, err := decoder.DecodeString(value)
		if err == nil {
			return decoded, nil
		}
	}

	return nil, fmt.Errorf("invalid base64 key")
}
