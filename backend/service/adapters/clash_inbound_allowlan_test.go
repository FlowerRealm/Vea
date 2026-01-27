package adapters

import (
	"testing"

	"vea/backend/domain"
	"vea/backend/service/nodegroup"

	"gopkg.in/yaml.v3"
)

func TestClashAdapter_BindAddressAllowLANOverridesLoopbackListen(t *testing.T) {
	t.Parallel()

	frouter := domain.FRouter{
		ID:   "fr1",
		Name: "test",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{
					ID:       "e1",
					From:     domain.EdgeNodeLocal,
					To:       domain.EdgeNodeDirect,
					Priority: 0,
					Enabled:  true,
				},
			},
		},
	}

	cfg := domain.ProxyConfig{
		InboundMode:     domain.InboundMixed,
		InboundPort:     1080,
		PreferredEngine: domain.EngineClash,
		FRouterID:       frouter.ID,
		InboundConfig: &domain.InboundConfiguration{
			Listen:   "127.0.0.1",
			AllowLAN: true,
		},
	}

	plan, err := nodegroup.CompileProxyPlan(domain.EngineClash, cfg, frouter, nil)
	if err != nil {
		t.Fatalf("CompileProxyPlan: %v", err)
	}

	out, err := (&ClashAdapter{}).BuildConfig(plan, GeoFiles{})
	if err != nil {
		t.Fatalf("BuildConfig: %v", err)
	}

	var m map[string]interface{}
	if err := yaml.Unmarshal(out, &m); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}

	if got, _ := m["bind-address"].(string); got != "0.0.0.0" {
		t.Fatalf("expected bind-address=0.0.0.0, got %q", got)
	}
	if got, _ := m["allow-lan"].(bool); got != true {
		t.Fatalf("expected allow-lan=true, got %v", m["allow-lan"])
	}
}
