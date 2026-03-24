package xray

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"v/internal/database/repository"
	"v/internal/logger"
	"v/internal/node"
)

func TestResolveAPIInboundPort_UsesStableHighPortPerNode(t *testing.T) {
	port := resolveAPIInboundPort(3, nil)

	assert.Equal(t, 63003, port)
	assert.NotEqual(t, 62789, port)
}

func TestResolveAPIInboundPort_SkipsProxyPorts(t *testing.T) {
	proxies := []*repository.Proxy{
		{Port: 63003},
		{Port: 63004},
	}

	port := resolveAPIInboundPort(3, proxies)

	assert.Equal(t, 63005, port)
}

func TestGenerateInbounds_UsesResolvedAPIPort(t *testing.T) {
	generator := &ConfigGenerator{logger: logger.NewNopLogger()}
	proxies := []*repository.Proxy{{
		ID:       1,
		UserID:   1,
		Protocol: "vmess",
		Port:     63003,
		Settings: map[string]any{
			"uuid": "a1e01ccc-6a4f-45c5-bf41-abd09427bbb4",
		},
	}}

	inbounds := generator.generateInbounds(context.Background(), 3, proxies, node.NetworkOptimizationSettings{})

	require.Len(t, inbounds, 2)
	assert.Equal(t, "api", inbounds[0].Tag)
	assert.Equal(t, 63004, inbounds[0].Port)
}

func TestGenerateOutbounds_UsesDirectAsDefaultOutbound(t *testing.T) {
	generator := &ConfigGenerator{logger: logger.NewNopLogger()}

	outbounds := generator.generateOutbounds()

	require.Len(t, outbounds, 2)
	assert.Equal(t, "direct", outbounds[0].Tag)
	assert.Equal(t, "freedom", outbounds[0].Protocol)
	assert.Equal(t, "blocked", outbounds[1].Tag)
}
