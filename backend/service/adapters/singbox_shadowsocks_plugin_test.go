package adapters

import (
	"encoding/json"
	"testing"

	"vea/backend/domain"
	"vea/backend/service/nodegroup"
)

func TestSingBoxAdapter_BuildConfig_ShadowsocksObfsAliasNormalizesToObfsLocal(t *testing.T) {
	t.Parallel()

	a := &SingBoxAdapter{}
	nodes := []domain.Node{
		{
			ID:       "n1",
			Name:     "test-ss-obfs",
			Protocol: domain.ProtocolShadowsocks,
			Address:  "1.1.1.1",
			Port:     443,
			Security: &domain.NodeSecurity{
				Method:     "aes-128-gcm",
				Password:   "pass",
				Plugin:     "obfs",
				PluginOpts: "host=example.com;mode=tls",
			},
		},
	}
	frouter := domain.FRouter{
		ID:   "fr1",
		Name: "test",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{
					ID:       "e-default",
					From:     domain.EdgeNodeLocal,
					To:       "n1",
					Priority: 100,
					Enabled:  true,
				},
			},
		},
	}

	profile := domain.ProxyConfig{InboundMode: domain.InboundMixed}
	plan, err := nodegroup.CompileProxyPlan(domain.EngineSingBox, profile, frouter, nodes)
	if err != nil {
		t.Fatalf("CompileProxyPlan() error: %v", err)
	}

	b, err := a.BuildConfig(plan, GeoFiles{ArtifactsDir: t.TempDir()})
	if err != nil {
		t.Fatalf("BuildConfig() error: %v", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(b, &cfg); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}

	outbounds, ok := cfg["outbounds"].([]any)
	if !ok || len(outbounds) == 0 {
		t.Fatalf("expected outbounds, got %T", cfg["outbounds"])
	}

	var ss map[string]any
	for _, it := range outbounds {
		ob, ok := it.(map[string]any)
		if !ok {
			continue
		}
		if typ, _ := ob["type"].(string); typ != "shadowsocks" {
			continue
		}
		if server, _ := ob["server"].(string); server != "1.1.1.1" {
			continue
		}
		ss = ob
		break
	}
	if ss == nil {
		t.Fatalf("expected shadowsocks outbound to be present")
	}
	if got, _ := ss["plugin"].(string); got != "obfs-local" {
		t.Fatalf("expected plugin=obfs-local, got %v", ss["plugin"])
	}
	if got, _ := ss["plugin_opts"].(string); got != "obfs=tls;obfs-host=example.com" {
		t.Fatalf("expected plugin_opts normalized, got %v", ss["plugin_opts"])
	}
}
