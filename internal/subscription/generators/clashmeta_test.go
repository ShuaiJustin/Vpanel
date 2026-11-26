package generators

import (
	"testing"

	"gopkg.in/yaml.v3"

	"v/internal/database/repository"
)

func TestClashMetaGenerator_GeneratesVLESSWhenProtocolHasWhitespace(t *testing.T) {
	generator := NewClashMetaGenerator()

	result, err := generator.Generate([]*repository.Proxy{
		{
			ID:       1,
			Name:     "Japan VLESS",
			Protocol: " VLESS ",
			Host:     "vless.example.com",
			Port:     443,
			Settings: map[string]any{
				"uuid":     "12345678-1234-1234-1234-123456789012",
				"security": "tls",
				"sni":      "vless.example.com",
			},
			Enabled: true,
		},
	}, nil)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	var config ClashMetaConfig
	if err := yaml.Unmarshal(result, &config); err != nil {
		t.Fatalf("failed to parse generated YAML: %v", err)
	}
	if len(config.Proxies) != 1 {
		t.Fatalf("expected one VLESS proxy, got %#v in YAML:\n%s", config.Proxies, string(result))
	}
	if config.Proxies[0]["type"] != "vless" {
		t.Fatalf("expected VLESS proxy type, got %#v", config.Proxies[0])
	}
	if len(config.ProxyGroups) == 0 {
		t.Fatalf("expected proxy groups to include generated VLESS node, got YAML:\n%s", string(result))
	}
}

func TestClashMetaGenerator_UsesSubscriptionNameForGroups(t *testing.T) {
	generator := NewClashMetaGenerator()

	result, err := generator.Generate([]*repository.Proxy{
		{
			ID:       1,
			Name:     "Japan VLESS",
			Protocol: "vless",
			Host:     "vless.example.com",
			Port:     443,
			Settings: map[string]any{
				"uuid":     "12345678-1234-1234-1234-123456789012",
				"security": "tls",
				"sni":      "vless.example.com",
			},
			Enabled: true,
		},
	}, &GeneratorOptions{
		SubscriptionName:   "shcrystal.top",
		IncludeProxyGroups: true,
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	var config ClashMetaConfig
	if err := yaml.Unmarshal(result, &config); err != nil {
		t.Fatalf("failed to parse generated YAML: %v", err)
	}
	if len(config.ProxyGroups) == 0 || config.ProxyGroups[0].Name != "shcrystal.top" {
		t.Fatalf("expected subscription-scoped select group, got %#v", config.ProxyGroups)
	}
	if len(config.ProxyGroups[0].Proxies) == 0 || config.ProxyGroups[0].Proxies[0] != "shcrystal.top Auto" {
		t.Fatalf("expected first select option to be automatic failover, got %#v", config.ProxyGroups[0].Proxies)
	}
	if len(config.Rules) != 1 || config.Rules[0] != "MATCH,shcrystal.top" {
		t.Fatalf("expected rules to target subscription-scoped group, got %#v", config.Rules)
	}
}

func TestClashMetaGenerator_SingleProxyKeepsStablePolicyGroups(t *testing.T) {
	generator := NewClashMetaGenerator()

	result, err := generator.Generate([]*repository.Proxy{
		{
			ID:       1,
			Name:     "Japan Trojan",
			Protocol: "trojan",
			Host:     "64.176.54.36",
			Port:     20039,
			Settings: map[string]any{
				"password": "secret",
				"sni":      "www.shcrystal.top",
			},
			Enabled: true,
		},
	}, &GeneratorOptions{
		SubscriptionName:   "shcrystal.top",
		IncludeProxyGroups: true,
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	var config ClashMetaConfig
	if err := yaml.Unmarshal(result, &config); err != nil {
		t.Fatalf("failed to parse generated YAML: %v", err)
	}
	if len(config.ProxyGroups) != 3 {
		t.Fatalf("expected select/Auto/Fallback groups for one proxy, got %#v", config.ProxyGroups)
	}
	got := config.ProxyGroups[0].Proxies
	want := []string{"shcrystal.top Auto", "shcrystal.top Fallback", "Japan Trojan", "DIRECT"}
	if len(got) != len(want) {
		t.Fatalf("expected select proxies %#v, got %#v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected select proxies %#v, got %#v", want, got)
		}
	}
}

func TestClashMetaGenerator_VLESSDefaultsUDPEnabled(t *testing.T) {
	generator := NewClashMetaGenerator()

	result, err := generator.Generate([]*repository.Proxy{
		{
			ID:       1,
			Name:     "Japan VLESS",
			Protocol: "vless",
			Host:     "64.176.54.36",
			Port:     20039,
			Settings: map[string]any{
				"uuid":     "12345678-1234-1234-1234-123456789012",
				"security": "tls",
				"sni":      "www.shcrystal.top",
			},
			Enabled: true,
		},
	}, nil)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	var config ClashMetaConfig
	if err := yaml.Unmarshal(result, &config); err != nil {
		t.Fatalf("failed to parse generated YAML: %v", err)
	}
	if len(config.Proxies) != 1 {
		t.Fatalf("expected one VLESS proxy, got %#v", config.Proxies)
	}
	if config.Proxies[0]["udp"] != true {
		t.Fatalf("expected VLESS proxy to enable udp by default, got %#v", config.Proxies[0])
	}
}

func TestClashGenerators_RealityEnablesMLKEM(t *testing.T) {
	info := &ProxyInfo{Name: "Reality", Server: "example.com", Port: 443, Settings: map[string]interface{}{
		"uuid": "12345678-1234-1234-1234-123456789012", "security": "reality", "flow": "xtls-rprx-vision",
		"publicKey": "test-public-key", "shortId": "0123456789abcdef", "sni": "www.cloudflare.com", "fingerprint": "chrome",
	}}
	for name, generate := range map[string]func(*ProxyInfo) (map[string]interface{}, error){
		"clash":      NewClashGenerator().generateVLESSProxy,
		"clash-meta": NewClashMetaGenerator().generateVLESSProxy,
	} {
		t.Run(name, func(t *testing.T) {
			proxy, err := generate(info)
			if err != nil {
				t.Fatal(err)
			}
			opts, ok := proxy["reality-opts"].(map[string]interface{})
			if !ok || opts["support-x25519mlkem768"] != true {
				t.Fatalf("missing REALITY MLKEM support: %#v", opts)
			}
			if opts["public-key"] != "test-public-key" || opts["short-id"] != "0123456789abcdef" || proxy["flow"] != "xtls-rprx-vision" {
				t.Fatalf("REALITY credentials/flow changed: %#v", proxy)
			}
			infoTLS := *info
			infoTLS.Settings = map[string]interface{}{"security": "tls", "sni": "example.com"}
			tlsProxy, err := generate(&infoTLS)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := tlsProxy["reality-opts"]; ok {
				t.Fatal("REALITY options leaked into TLS")
			}
		})
	}
}
