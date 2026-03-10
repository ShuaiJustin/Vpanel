package proxy

import (
	"net/url"
	"strings"
)

var invalidShareHosts = map[string]struct{}{
	"":                {},
	"0.0.0.0":         {},
	"::":              {},
	"[::]":            {},
	"0:0:0:0:0:0:0:0": {},
}

func NormalizeShareHost(raw string) string {
	host := strings.TrimSpace(raw)
	if host == "" {
		return ""
	}

	if strings.Contains(host, "://") {
		if parsed, err := url.Parse(host); err == nil {
			if parsedHost := parsed.Hostname(); parsedHost != "" {
				host = parsedHost
			}
		}
	} else {
		if parsedHost := (&url.URL{Host: host}).Hostname(); parsedHost != "" {
			host = parsedHost
		}
	}

	if _, invalid := invalidShareHosts[strings.ToLower(host)]; invalid {
		return ""
	}

	return host
}

func ResolveSNI(settings map[string]any) string {
	for _, key := range []string{"sni", "server_name", "tls_domain"} {
		if value := getSettingString(settings, key); value != "" {
			return value
		}
	}
	return ""
}

func HasTLSSettings(settings map[string]any) bool {
	if settings == nil {
		return false
	}

	if enabled, ok := settings["tls"].(bool); ok && enabled {
		return true
	}

	if strings.EqualFold(getSettingString(settings, "tls"), "tls") {
		return true
	}

	switch strings.ToLower(getSettingString(settings, "security")) {
	case "tls", "xtls", "reality":
		return true
	}

	return ResolveSNI(settings) != ""
}

func ResolveServerAddress(host string, settings map[string]any) string {
	candidates := []string{
		getSettingString(settings, "server"),
		getSettingString(settings, "address"),
		ResolveSNI(settings),
		host,
	}

	for _, candidate := range candidates {
		if normalized := NormalizeShareHost(candidate); normalized != "" {
			return normalized
		}
	}

	return ""
}

func getSettingString(settings map[string]any, key string) string {
	if settings == nil {
		return ""
	}
	value, ok := settings[key]
	if !ok {
		return ""
	}
	stringValue, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(stringValue)
}
