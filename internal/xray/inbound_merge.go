package xray

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"v/internal/database/repository"
	"v/internal/logger"
)

// mergedInboundTagPrefix marks an inbound produced by the merge path.
// Traffic accounting does NOT depend on the inbound tag: the agent parses
// user>>><email>>>>traffic>>>* stat names, and every merged client keeps its
// original email (user-<userID>-proxy-<proxyID>). Routing rules only
// reference the "api" tag, so changing proxy tags is safe.
const mergedInboundTagPrefix = "inbound-merged-"

// mergeableProtocols lists protocols whose inbound settings use a "clients"
// array. Shadowsocks uses a single flat settings object and is intentionally
// left untouched (one inbound per proxy).
var mergeableProtocols = map[string]bool{
	"vless":  true,
	"vmess":  true,
	"trojan": true,
}

// inboundCandidate keeps the association between a DB proxy and the inbound
// generated from it. The proxy is what carries the per-client identity.
type inboundCandidate struct {
	proxy   *repository.Proxy
	inbound InboundConfig
}

type inboundGroup struct {
	key      string
	inbounds []InboundConfig
}

// sharedSettings removes ONLY the per-client credentials array. Everything
// else (decryption, fallbacks, streamSettings, sniffing, port, protocol,
// Reality keys, SNI, sockopt, ...) stays in the fingerprint. This is
// deliberately strict: two inbounds are merged only when every shared
// field is byte-identical.
func sharedSettings(settings map[string]any) map[string]any {
	out := make(map[string]any, len(settings))
	for key, value := range settings {
		if key == "clients" {
			continue
		}
		out[key] = value
	}
	return out
}

// sharedFieldFingerprint hashes the inbound with the tag and client array
// removed. encoding/json sorts map keys, so the hash is stable regardless of
// map iteration order.
func sharedFieldFingerprint(inbound InboundConfig) (string, error) {
	clone := inbound
	clone.Tag = ""
	clone.Settings = sharedSettings(inbound.Settings)

	data, err := json.Marshal(clone)
	if err != nil {
		return "", fmt.Errorf("marshal shared inbound fields: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:12]), nil
}

// extractClients tolerates both the generator's []map[string]any and the
// []any shape produced by a JSON round-trip (tests, config preview).
func extractClients(settings map[string]any) []map[string]any {
	switch value := settings["clients"].(type) {
	case []map[string]any:
		return value
	case []any:
		out := make([]map[string]any, 0, len(value))
		for _, item := range value {
			if client, ok := item.(map[string]any); ok {
				out = append(out, client)
			}
		}
		return out
	default:
		return nil
	}
}

func isMergeableInbound(inbound InboundConfig) bool {
	if !mergeableProtocols[strings.ToLower(strings.TrimSpace(inbound.Protocol))] {
		return false
	}
	return len(extractClients(inbound.Settings)) > 0
}

// clientDedupKey is based on the credential, not the whole client object:
// Xray rejects an inbound containing two clients with the same id/password.
// A duplicated credential is a data error; keeping the first client avoids
// Xray refusing the entire config (and avoids double traffic accounting).
func clientDedupKey(client map[string]any) string {
	if id, ok := client["id"].(string); ok && id != "" {
		return "id:" + id
	}
	if password, ok := client["password"].(string); ok && password != "" {
		return "password:" + password
	}
	data, err := json.Marshal(client)
	if err != nil {
		return fmt.Sprint(client)
	}
	return "raw:" + string(data)
}

// buildMergedInbound creates one inbound holding all clients of a group.
// Shared fields come from the first member (all members have the same
// fingerprint, so this is safe). Duplicate client credentials are dropped
// so Xray never rejects the config with "duplicated user".
func buildMergedInbound(key string, inbounds []InboundConfig) InboundConfig {
	base := inbounds[0]
	merged := base
	merged.Tag = mergedInboundTagPrefix + key

	settings := sharedSettings(base.Settings)
	clients := make([]map[string]any, 0, len(inbounds))
	seen := make(map[string]struct{}, len(inbounds))
	for _, inbound := range inbounds {
		for _, client := range extractClients(inbound.Settings) {
			dedupKey := clientDedupKey(client)
			if _, exists := seen[dedupKey]; exists {
				continue
			}
			seen[dedupKey] = struct{}{}
			clients = append(clients, client)
		}
	}
	settings["clients"] = clients
	merged.Settings = settings

	return merged
}

// mergeInboundCandidates groups candidates by their shared-field fingerprint
// and emits one inbound per group. Groups with a single member are emitted
// unchanged (same tag and shape as the legacy path), so enabling the flag is a
// no-op for heterogeneous data. Output order follows the original proxy
// order, which keeps generated configs stable across syncs.
func (g *ConfigGenerator) mergeInboundCandidates(candidates []inboundCandidate) []InboundConfig {
	groups := make(map[string]*inboundGroup)

	for _, candidate := range candidates {
		if !isMergeableInbound(candidate.inbound) {
			continue
		}
		fingerprint, err := sharedFieldFingerprint(candidate.inbound)
		if err != nil {
			if g.logger != nil {
				g.logger.Warn("failed to fingerprint inbound for merging",
					logger.F("proxy_id", candidate.proxy.ID), logger.Err(err))
			}
			continue
		}
		if groups[fingerprint] == nil {
			groups[fingerprint] = &inboundGroup{key: fingerprint}
		}
		groups[fingerprint].inbounds = append(groups[fingerprint].inbounds, candidate.inbound)
	}

	result := make([]InboundConfig, 0, len(candidates))
	emitted := make(map[string]bool, len(groups))
	for _, candidate := range candidates {
		if !isMergeableInbound(candidate.inbound) {
			result = append(result, candidate.inbound)
			continue
		}
		fingerprint, err := sharedFieldFingerprint(candidate.inbound)
		if err != nil {
			result = append(result, candidate.inbound)
			continue
		}
		if emitted[fingerprint] {
			continue
		}
		emitted[fingerprint] = true

		group := groups[fingerprint]
		if group == nil || len(group.inbounds) < 2 {
			result = append(result, candidate.inbound)
			continue
		}
		result = append(result, buildMergedInbound(fingerprint, group.inbounds))
	}

	return result
}

// ClientCount returns the total number of client credentials across all
// inbounds. Used by the config preview so a merged inbound is not mistaken
// for lost users.
func ClientCount(inbounds []InboundConfig) int {
	total := 0
	for _, inbound := range inbounds {
		total += len(extractClients(inbound.Settings))
	}
	return total
}

// MergedInboundCount returns how many inbounds were produced by the merge
// path (used by the config preview).
func MergedInboundCount(inbounds []InboundConfig) int {
	count := 0
	for _, inbound := range inbounds {
		if strings.HasPrefix(inbound.Tag, mergedInboundTagPrefix) {
			count++
		}
	}
	return count
}

// ValidateMergedInbounds checks the invariants the merge path must preserve:
// non-empty client arrays, unique emails (traffic accounting identity) and
// non-empty credentials. It is safe to call on legacy configs too.
func ValidateMergedInbounds(inbounds []InboundConfig) error {
	emails := make(map[string]struct{})
	for _, inbound := range inbounds {
		clients := extractClients(inbound.Settings)
		if len(clients) == 0 {
			continue
		}
		for _, client := range clients {
			email, _ := client["email"].(string)
			if email == "" {
				return fmt.Errorf("inbound %q has a client without email", inbound.Tag)
			}
			if _, exists := emails[email]; exists {
				return fmt.Errorf("duplicate client email %q (traffic accounting would collide)", email)
			}
			emails[email] = struct{}{}

			id, _ := client["id"].(string)
			password, _ := client["password"].(string)
			if id == "" && password == "" {
				return fmt.Errorf("inbound %q client %q has no credential", inbound.Tag, email)
			}
		}
	}
	return nil
}
