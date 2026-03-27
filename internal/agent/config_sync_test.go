package agent

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigVersion_IsStableForEquivalentConfig(t *testing.T) {
	config := json.RawMessage(`{"inbounds":[{"port":443}],"outbounds":[{"protocol":"freedom"}]}`)

	assert.NotEmpty(t, configVersion(config))
	assert.Equal(t, configVersion(config), configVersion(config))
}

func TestConfigVersion_ChangesWhenConfigChanges(t *testing.T) {
	first := json.RawMessage(`{"inbounds":[{"port":443}],"outbounds":[{"protocol":"freedom"}]}`)
	second := json.RawMessage(`{"inbounds":[{"port":8443}],"outbounds":[{"protocol":"freedom"}]}`)

	assert.NotEqual(t, configVersion(first), configVersion(second))
}
