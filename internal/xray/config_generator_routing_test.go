package xray

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"v/internal/database/repository"
	"v/internal/logger"
	apperrors "v/pkg/errors"
)

func realityProxy(nodeID, userID int64, port int, uuid string) *repository.Proxy {
	return &repository.Proxy{
		UserID: userID, NodeID: &nodeID, Enabled: true, Protocol: "vless", Port: port,
		Settings: map[string]any{
			"uuid": uuid, "flow": "xtls-rprx-vision", "network": "tcp",
			"security": "reality", "server_name": "www.cloudflare.com",
			"reality_settings": map[string]any{
				"dest":        "www.cloudflare.com:443",
				"privateKey":  "2KZ4uouMKgI8nR-LDJNP1_MHisCJOmKGj9jUjZLncVU",
				"serverNames": []string{"www.cloudflare.com"},
				"shortIds":    []string{"6ba85179e30d4fc2"},
			},
		},
	}
}

func generateNodeConfig(t *testing.T) *XrayConfig {
	t.Helper()
	repo := newMockProxyRepoForSync()
	for i, uuid := range []string{
		"5f0c2b1e-6a55-4f0e-9f3a-0d8a1c2b3d4e", "0b9b6f1c-93c1-4a53-8d42-6d2f1c8f3a10", "c4a0a1d2-7f3e-4c0b-a1d5-9e8b7c6d5f40",
	} {
		require.NoError(t, repo.Create(context.Background(), realityProxy(7, int64(100+i), 20001+i, uuid)))
	}
	config, err := NewConfigGenerator(repo, nil, nil, logger.NewNopLogger()).GenerateForNode(context.Background(), 7)
	require.NoError(t, err)
	return config
}

// A customer can ask their own inbound to connect anywhere. Unless the node's
// own addresses are blocked, "anywhere" includes 127.0.0.1:<api port>, where the
// unauthenticated Xray API listens: verified against a real Xray pair, the
// stats API answered through a customer tunnel by IP and by hostname.
func TestGenerateForNode_BlocksNodeLocalDestinations(t *testing.T) {
	config := generateNodeConfig(t)

	require.Equal(t, "IPIfNonMatch", config.Routing.DomainStrategy,
		"without it a hostname resolving to 127.0.0.1 skips the IP rule")
	rules := config.Routing.Rules
	require.GreaterOrEqual(t, len(rules), 2)
	require.Equal(t, []string{"api"}, rules[0].InboundTag, "the agent's own API traffic must be routed first")
	require.Equal(t, "blocked", rules[1].OutboundTag)
	require.Empty(t, rules[1].InboundTag, "the block applies to every user inbound")
	for _, network := range []string{"::/128", "127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "169.254.0.0/16", "100.64.0.0/10", "::1/128", "fc00::/7", "fe80::/10"} {
		require.Contains(t, rules[1].IP, network)
	}
	// The API port is also blocked whatever the destination resolves to.
	require.Equal(t, "api", config.Inbounds[0].Tag)
	require.Equal(t, strconv.Itoa(config.Inbounds[0].Port), rules[2].Port)
	require.Equal(t, "blocked", rules[2].OutboundTag)
	require.Empty(t, rules[2].InboundTag)
	require.Equal(t, []string{"StatsService"}, config.API.Services,
		"the agent only reads counters; HandlerService can add users without authentication")
}

// The panel decides whether a node needs a re-sync by hashing this output.
// Output that changed between two calls would restart Xray on every check.
func TestGenerateForNode_OutputIsStable(t *testing.T) {
	first, err := generateNodeConfig(t).ToJSON()
	require.NoError(t, err)
	for i := 0; i < 50; i++ {
		next, err := generateNodeConfig(t).ToJSON()
		require.NoError(t, err)
		require.Equal(t, sha256.Sum256(first), sha256.Sum256(next), "run %d produced a different config", i)
	}
}

// Skipped where no xray binary is installed; nodes validate the same way
// (xray run -test) before applying a config.
func TestGenerateForNode_ConfigIsAcceptedByXray(t *testing.T) {
	binary, err := exec.LookPath("xray")
	if err != nil {
		t.Skip("xray binary not installed")
	}
	data, err := generateNodeConfig(t).ToJSON()
	require.NoError(t, err)
	require.True(t, json.Valid(data))
	path := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(path, data, 0o600))
	output, err := exec.Command(binary, "run", "-test", "-c", path).CombinedOutput()
	require.NoError(t, err, string(output))
}

// A lookup that failed says nothing about the user. Dropping them anyway cut
// paying users off until the next sync; a deleted owner is still dropped.
func TestGenerateForNode_FailsWhenAccessCannotBeChecked(t *testing.T) {
	repo := newMockProxyRepoForSync()
	require.NoError(t, repo.Create(context.Background(), realityProxy(7, 101, 20001, "5f0c2b1e-6a55-4f0e-9f3a-0d8a1c2b3d4e")))
	require.NoError(t, repo.Create(context.Background(), realityProxy(7, 102, 20002, "0b9b6f1c-93c1-4a53-8d42-6d2f1c8f3a10")))

	failing := NewConfigGenerator(repo, nil, nil, logger.NewNopLogger()).
		WithUserAccessCheck(func(context.Context, int64) error { return context.DeadlineExceeded })
	_, err := failing.GenerateForNode(context.Background(), 7)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	deletedOwner := NewConfigGenerator(repo, nil, nil, logger.NewNopLogger()).
		WithUserAccessCheck(func(_ context.Context, userID int64) error {
			if userID == 102 {
				return apperrors.NewNotFoundError("user", userID)
			}
			return nil
		})
	config, err := deletedOwner.GenerateForNode(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, config.Inbounds, 2, "api inbound plus the one remaining user")
}
