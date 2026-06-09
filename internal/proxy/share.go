package proxy

import (
	"net/url"
	"strconv"
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

// NormalizeTLSServerName returns a concrete DNS name suitable for client SNI.
// Certificates may be stored as wildcard domains (for example *.example.com),
// but TLS clients should send a real host name, not the wildcard pattern.
func NormalizeTLSServerName(raw string) string {
	host := NormalizeShareHost(raw)
	if host == "" {
		return ""
	}

	if strings.HasPrefix(host, "*.") {
		suffix := strings.TrimPrefix(host, "*.")
		if suffix == "" {
			return ""
		}
		return "www." + suffix
	}

	return host
}

func ResolveSNI(settings map[string]any) string {
	for _, key := range []string{"sni", "server_name", "tls_domain"} {
		if value := NormalizeTLSServerName(getSettingString(settings, key)); value != "" {
			return value
		}
	}
	return ""
}

func ResolveExternalServerAddress(settings map[string]any) string {
	for _, key := range []string{"external_host", "externalHost", "server_host", "serverHost"} {
		if normalized := NormalizeShareHost(getSettingString(settings, key)); normalized != "" {
			return normalized
		}
	}
	return ""
}

func ResolveTLSSkipVerify(settings map[string]any) bool {
	for _, key := range []string{"allowInsecure", "skipCertVerify"} {
		value, ok := settings[key]
		if !ok {
			continue
		}

		switch typed := value.(type) {
		case bool:
			return typed
		case string:
			normalized := strings.TrimSpace(strings.ToLower(typed))
			return normalized == "1" || normalized == "true"
		}
	}

	return false
}

func ResolveVMessCipher(settings map[string]any) string {
	for _, key := range []string{"cipher", "scy"} {
		if value := strings.TrimSpace(strings.ToLower(getSettingString(settings, key))); value != "" {
			return value
		}
	}

	security := strings.TrimSpace(strings.ToLower(getSettingString(settings, "security")))
	switch security {
	case "", "tls", "xtls", "reality":
		return "auto"
	default:
		return security
	}
}

func HasTLSSettings(settings map[string]any) bool {
	if settings == nil {
		return false
	}

	switch strings.ToLower(getSettingString(settings, "security")) {
	case "none":
		return false
	case "tls", "xtls", "reality":
		return true
	}

	if enabled, ok := settings["tls"].(bool); ok && enabled {
		return true
	}

	switch strings.ToLower(getSettingString(settings, "tls")) {
	case "tls", "true", "1":
		return true
	}

	return ResolveSNI(settings) != ""
}

func ResolveServerAddress(host string, settings map[string]any) string {
	candidates := []string{
		ResolveExternalServerAddress(settings),
		host,
		getSettingString(settings, "server"),
		getSettingString(settings, "address"),
		ResolveSNI(settings),
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

func ResolveServerPort(defaultPort int, settings map[string]any) int {
	for _, key := range []string{"external_port", "externalPort", "server_port", "serverPort"} {
		if value, ok := settings[key]; ok {
			switch typed := value.(type) {
			case int:
				if typed >= 1 && typed <= 65535 {
					return typed
				}
			case int64:
				if typed >= 1 && typed <= 65535 {
					return int(typed)
				}
			case float64:
				port := int(typed)
				if port >= 1 && port <= 65535 {
					return port
				}
			case string:
				if parsed, err := strconv.Atoi(strings.TrimSpace(typed)); err == nil {
					if parsed >= 1 && parsed <= 65535 {
						return parsed
					}
				}
			}
		}
	}

	return defaultPort
}
