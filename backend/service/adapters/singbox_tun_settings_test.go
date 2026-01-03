package adapters

import (
	"encoding/json"
	"runtime"
	"testing"

	"vea/backend/domain"
	"vea/backend/service/nodegroup"
)

func TestSingBoxAdapter_BuildConfig_TUNRespectsSettings(t *testing.T) {
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

	profile := domain.ProxyConfig{
		InboundMode: domain.InboundTUN,
		TUNSettings: &domain.TUNConfiguration{
			InterfaceName:          "tun9",
			MTU:                    8000,
			Address:                []string{"172.19.0.1/30"},
			AutoRoute:              false,
			AutoRedirect:           true,
			StrictRoute:            true,
			Stack:                  "mixed",
			DNSHijack:              false,
			EndpointIndependentNat: true,
			UDPTimeout:             90,
			RouteAddress:           []string{"0.0.0.0/1"},
			RouteExcludeAddress:    []string{"10.0.0.0/8"},
		},
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
	tun, ok := inbounds[0].(map[string]any)
	if !ok {
		t.Fatalf("expected tun inbound map, got %T", inbounds[0])
	}

	if got, _ := tun["type"].(string); got != "tun" {
		t.Fatalf("expected tun.type=tun, got %v", tun["type"])
	}
	if got, _ := tun["interface_name"].(string); got != "tun9" {
		t.Fatalf("expected tun.interface_name=tun9, got %v", tun["interface_name"])
	}
	if got, _ := tun["mtu"].(float64); got != 8000 {
		t.Fatalf("expected tun.mtu=8000, got %v", tun["mtu"])
	}
	if got, _ := tun["auto_route"].(bool); got != false {
		t.Fatalf("expected tun.auto_route=false, got %v", tun["auto_route"])
	}
	if got, _ := tun["strict_route"].(bool); got != true {
		t.Fatalf("expected tun.strict_route=true, got %v", tun["strict_route"])
	}
	if got, _ := tun["stack"].(string); got != "mixed" {
		t.Fatalf("expected tun.stack=mixed, got %v", tun["stack"])
	}
	if got, ok := tun["sniff"].(bool); !ok || got != true {
		t.Fatalf("expected tun.sniff=true, got %v", tun["sniff"])
	}
	if got, ok := tun["sniff_override_destination"].(bool); !ok || got != false {
		t.Fatalf("expected tun.sniff_override_destination=false, got %v", tun["sniff_override_destination"])
	}
	if runtime.GOOS == "linux" {
		if got, ok := tun["auto_redirect"].(bool); !ok || got != true {
			t.Fatalf("expected tun.auto_redirect=true on linux, got %v", tun["auto_redirect"])
		}
	} else {
		if _, ok := tun["auto_redirect"]; ok {
			t.Fatalf("expected tun.auto_redirect to be omitted on %s", runtime.GOOS)
		}
	}
	if got, ok := tun["endpoint_independent_nat"].(bool); !ok || got != true {
		t.Fatalf("expected tun.endpoint_independent_nat=true, got %v", tun["endpoint_independent_nat"])
	}
	if got, ok := tun["udp_timeout"].(string); !ok || got != "90s" {
		t.Fatalf("expected tun.udp_timeout=90s, got %v", tun["udp_timeout"])
	}
	if got, ok := tun["route_address"].([]any); !ok || len(got) != 1 || got[0] != "0.0.0.0/1" {
		t.Fatalf("expected tun.route_address=[0.0.0.0/1], got %v", tun["route_address"])
	}
	if got, ok := tun["route_exclude_address"].([]any); !ok || len(got) != 1 || got[0] != "10.0.0.0/8" {
		t.Fatalf("expected tun.route_exclude_address=[10.0.0.0/8], got %v", tun["route_exclude_address"])
	}

	route, ok := cfg["route"].(map[string]any)
	if !ok {
		t.Fatalf("expected route map, got %T", cfg["route"])
	}
	rules, ok := route["rules"].([]any)
	if !ok {
		t.Fatalf("expected route.rules, got %T", route["rules"])
	}
	for _, it := range rules {
		rule, ok := it.(map[string]any)
		if !ok {
			continue
		}
		if action, ok := rule["action"].(string); ok && action == "hijack-dns" {
			t.Fatalf("expected dns hijack rule to be omitted when tunSettings.dnsHijack=false")
		}
	}
}
