package adapters

import (
	"encoding/json"
	"testing"

	"vea/backend/domain"
	"vea/backend/service/nodegroup"
)

func TestSingBoxAdapter_BuildConfig_InboundAllowLANSetsListen(t *testing.T) {
	t.Parallel()

	a := &SingBoxAdapter{}
	nodes := []domain.Node{
		{
			ID:       "n1",
			Name:     "test-ss",
			Protocol: domain.ProtocolShadowsocks,
			Address:  "1.1.1.1",
			Port:     443,
			Security: &domain.NodeSecurity{Method: "aes-128-gcm", Password: "pass"},
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

	cases := []struct {
		name       string
		inbound    *domain.InboundConfiguration
		wantListen string
	}{
		{
			name:       "allowlan_empty_listen",
			inbound:    &domain.InboundConfiguration{AllowLAN: true},
			wantListen: "0.0.0.0",
		},
		{
			name:       "allowlan_loopback_listen",
			inbound:    &domain.InboundConfiguration{Listen: "127.0.0.1", AllowLAN: true},
			wantListen: "0.0.0.0",
		},
		{
			name:       "allowlan_localhost_listen",
			inbound:    &domain.InboundConfiguration{Listen: "localhost", AllowLAN: true},
			wantListen: "0.0.0.0",
		},
		{
			name:       "allowlan_ipv6_loopback_listen",
			inbound:    &domain.InboundConfiguration{Listen: "::1", AllowLAN: true},
			wantListen: "::",
		},
		{
			name:       "allowlan_custom_listen_not_overridden",
			inbound:    &domain.InboundConfiguration{Listen: "192.168.1.10", AllowLAN: true},
			wantListen: "192.168.1.10",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			profile := domain.ProxyConfig{
				InboundMode:   domain.InboundMixed,
				InboundPort:   1080,
				InboundConfig: tc.inbound,
			}

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

			inbounds, ok := cfg["inbounds"].([]any)
			if !ok || len(inbounds) == 0 {
				t.Fatalf("expected inbounds, got %T", cfg["inbounds"])
			}

			var mixed map[string]any
			for _, it := range inbounds {
				m, ok := it.(map[string]any)
				if !ok {
					continue
				}
				if typ, _ := m["type"].(string); typ == "mixed" {
					mixed = m
					break
				}
			}
			if mixed == nil {
				t.Fatalf("expected mixed inbound to exist")
			}

			got, _ := mixed["listen"].(string)
			if got != tc.wantListen {
				t.Fatalf("expected mixed.listen=%q, got %q", tc.wantListen, got)
			}
		})
	}
}
