package handlers

import (
	"fmt"
	"net"
	"strings"
)

func splitIPWhitelist(raw string) []string {
	normalized := strings.NewReplacer("\r\n", "\n", ",", "\n", ";", "\n", "\t", "\n").Replace(raw)
	parts := strings.Split(normalized, "\n")
	entries := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))

	for _, part := range parts {
		entry := strings.TrimSpace(part)
		if entry == "" {
			continue
		}
		if _, exists := seen[entry]; exists {
			continue
		}
		seen[entry] = struct{}{}
		entries = append(entries, entry)
	}

	return entries
}

func validateIPWhitelist(raw string) error {
	for _, entry := range splitIPWhitelist(raw) {
		if ip := net.ParseIP(entry); ip != nil {
			continue
		}
		if _, _, err := net.ParseCIDR(entry); err == nil {
			continue
		}
		return fmt.Errorf("invalid IP whitelist entry: %s", entry)
	}
	return nil
}

func isIPAllowedByWhitelist(clientIP, raw string) bool {
	ip := net.ParseIP(strings.TrimSpace(clientIP))
	if ip == nil {
		return false
	}

	entries := splitIPWhitelist(raw)
	if len(entries) == 0 {
		return false
	}

	for _, entry := range entries {
		if candidate := net.ParseIP(entry); candidate != nil && candidate.Equal(ip) {
			return true
		}
		if _, network, err := net.ParseCIDR(entry); err == nil && network.Contains(ip) {
			return true
		}
	}

	return false
}
