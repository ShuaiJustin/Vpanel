package proxy

import (
	"net"
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

	resolved := ""
	for _, candidate := range candidates {
		if normalized := NormalizeShareHost(candidate); normalized != "" {
			resolved = normalized
			break
		}
	}

	if resolved == "" {
		return ""
	}

	if HasTLSSettings(settings) && net.ParseIP(resolved) != nil {
		if sni := NormalizeShareHost(ResolveSNI(settings)); sni != "" {
			return sni
		}
	}

	return resolved
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
