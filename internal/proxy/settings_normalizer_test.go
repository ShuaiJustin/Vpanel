package proxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeSettings_RealityDerivesAliasesAndPublicKey(t *testing.T) {
	privateKey := "iVJ2Q4Q2rQ8zYGi5M6af0AvFp8ZUBKbjDg7sWXBJWh4"

	normalized, err := NormalizeSettings("vless", map[string]any{
		"security": "reality",
		"reality_settings": map[string]any{
			"dest":        "www.cloudflare.com:443",
			"serverNames": []string{"www.cloudflare.com"},
			"privateKey":  privateKey,
			"shortIds":    []string{"6ba85179e30d4fc2"},
		},
		"fingerprint": "chrome",
	})

	require.NoError(t, err)
	assert.Equal(t, "reality", normalized["security"])
	assert.Equal(t, "www.cloudflare.com", normalized["sni"])
	assert.Equal(t, "www.cloudflare.com", normalized["server_name"])
	assert.Equal(t, "chrome", normalized["fingerprint"])
	assert.Equal(t, "chrome", normalized["fp"])
	assert.Equal(t, "6ba85179e30d4fc2", normalized["shortId"])
	assert.Equal(t, "6ba85179e30d4fc2", normalized["sid"])

	publicKey, ok := normalized["publicKey"].(string)
	require.True(t, ok)
	require.NotEmpty(t, publicKey)
	assert.Equal(t, publicKey, normalized["pbk"])

	realitySettings, ok := normalized["reality_settings"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "www.cloudflare.com:443", realitySettings["dest"])
	assert.Equal(t, privateKey, realitySettings["privateKey"])
	assert.Equal(t, []string{"www.cloudflare.com"}, realitySettings["serverNames"])
	assert.Equal(t, []string{"6ba85179e30d4fc2"}, realitySettings["shortIds"])
}

func TestNormalizeSettings_InvalidRealityKeyFails(t *testing.T) {
	_, err := NormalizeSettings("vless", map[string]any{
		"security": "reality",
		"reality_settings": map[string]any{
			"dest":        "www.cloudflare.com:443",
			"serverNames": []string{"www.cloudflare.com"},
			"privateKey":  "invalid-key",
		},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Reality 私钥无效")
}
