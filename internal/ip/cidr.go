package ip

import (
	"net"
	"strings"
)

// CIDRMatcher provides CIDR range matching functionality.
type CIDRMatcher struct{}

// NewCIDRMatcher creates a new CIDRMatcher instance.
func NewCIDRMatcher() *CIDRMatcher {
	return &CIDRMatcher{}
}

// MatchesCIDR checks if an IP address falls within a CIDR range.
// Supports both IPv4 and IPv6 addresses and CIDR notations.
func (m *CIDRMatcher) MatchesCIDR(ipStr, cidrStr string) bool {
	// Parse the IP address
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	// If cidrStr is empty, return false
	if cidrStr == "" {
		return false
	}

	// If cidrStr doesn't contain '/', treat it as a single IP
	if !strings.Contains(cidrStr, "/") {
		cidrIP := net.ParseIP(cidrStr)
		if cidrIP == nil {
			return false
		}
		return ip.Equal(cidrIP)
	}

	// Parse the CIDR
	_, network, err := net.ParseCIDR(cidrStr)
	if err != nil {
		return false
	}

	return network.Contains(ip)
}

// MatchesIP checks if two IP addresses are equal.
func (m *CIDRMatcher) MatchesIP(ip1, ip2 string) bool {
	parsedIP1 := net.ParseIP(ip1)
	parsedIP2 := net.ParseIP(ip2)
	if parsedIP1 == nil || parsedIP2 == nil {
		return false
	}
	return parsedIP1.Equal(parsedIP2)
}

// IsValidIP checks if a string is a valid IP address.
func (m *CIDRMatcher) IsValidIP(ipStr string) bool {
	return net.ParseIP(ipStr) != nil
}

// IsValidCIDR checks if a string is a valid CIDR notation.
func (m *CIDRMatcher) IsValidCIDR(cidrStr string) bool {
	if cidrStr == "" {
		return false
	}
	_, _, err := net.ParseCIDR(cidrStr)
	return err == nil
}

// IsIPv4 checks if an IP address is IPv4.
func (m *CIDRMatcher) IsIPv4(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	return ip.To4() != nil
}

// IsIPv6 checks if an IP address is IPv6.
func (m *CIDRMatcher) IsIPv6(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	return ip.To4() == nil
}

// NormalizeIP normalizes an IP address to its canonical form.
func (m *CIDRMatcher) NormalizeIP(ipStr string) string {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return ipStr
	}
	return ip.String()
}

// MatchesAny checks if an IP matches any of the given CIDR ranges or IPs.
func (m *CIDRMatcher) MatchesAny(ipStr string, cidrsOrIPs []string) bool {
	for _, cidrOrIP := range cidrsOrIPs {
		if m.MatchesCIDR(ipStr, cidrOrIP) {
			return true
		}
	}
	return false
}
