package adapters

import (
	"encoding/json"
	"testing"

	"vea/backend/domain"
	"vea/backend/service/nodegroup"
)

func TestSingBoxAdapter_BuildConfig_MeasurementDNSRemoteDetourOmittedWhenFinalDirect(t *testing.T) {
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
					To:       "direct",
					Priority: 100,
					Enabled:  true,
				},
				{
					ID:       "e-openai",
					From:     domain.EdgeNodeLocal,
					To:       "n1",
					Priority: 90,
					Enabled:  true,
					RuleType: domain.EdgeRuleRoute,
					RouteRule: &domain.RouteMatchRule{
						Domains: []string{"geosite:openai"},
					},
				},
			},
		},
	}

	plan, err := nodegroup.CompileMeasurementPlan(domain.EngineSingBox, 17891, frouter, nodes)
	if err != nil {
		t.Fatalf("CompileMeasurementPlan() error: %v", err)
	}

	b, err := a.BuildConfig(plan, GeoFiles{ArtifactsDir: t.TempDir()})
	if err != nil {
		t.Fatalf("BuildConfig() error: %v", err)
	}

	cfg := mustUnmarshalJSONMap(t, b)
	dns := mustMap(t, cfg["dns"])
	servers := mustSlice(t, dns["servers"])
	dnsRemote := findServerByTag(t, servers, "dns-remote")

	if _, ok := dnsRemote["detour"]; ok {
		t.Fatalf(`dns-remote should omit "detour" when final is direct (sing-box 会在运行期拒绝 detour -> 空 direct outbound)`)
	}
}

func TestSingBoxAdapter_BuildConfig_DNSRemoteDetourOmittedWhenFinalDirect(t *testing.T) {
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
					To:       "direct",
					Priority: 100,
					Enabled:  true,
				},
				{
					ID:       "e-openai",
					From:     domain.EdgeNodeLocal,
					To:       "n1",
					Priority: 90,
					Enabled:  true,
					RuleType: domain.EdgeRuleRoute,
					RouteRule: &domain.RouteMatchRule{
						Domains: []string{"geosite:openai"},
					},
				},
			},
		},
	}

	profile := domain.ProxyConfig{
		InboundMode: domain.InboundSOCKS,
		InboundPort: 1080,
	}

	plan, err := nodegroup.CompileProxyPlan(domain.EngineSingBox, profile, frouter, nodes)
	if err != nil {
		t.Fatalf("CompileProxyPlan() error: %v", err)
	}

	b, err := a.BuildConfig(plan, GeoFiles{ArtifactsDir: t.TempDir()})
	if err != nil {
		t.Fatalf("BuildConfig() error: %v", err)
	}

	cfg := mustUnmarshalJSONMap(t, b)
	dns := mustMap(t, cfg["dns"])
	servers := mustSlice(t, dns["servers"])
	dnsRemote := findServerByTag(t, servers, "dns-remote")

	if _, ok := dnsRemote["detour"]; ok {
		t.Fatalf(`dns-remote should omit "detour" when final is direct (sing-box 会在运行期拒绝 detour -> 空 direct outbound)`)
	}
}

func TestSingBoxAdapter_BuildConfig_DNSRemoteDetourSetWhenFinalProxy(t *testing.T) {
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

	plan, err := nodegroup.CompileMeasurementPlan(domain.EngineSingBox, 17891, frouter, nodes)
	if err != nil {
		t.Fatalf("CompileMeasurementPlan() error: %v", err)
	}

	b, err := a.BuildConfig(plan, GeoFiles{ArtifactsDir: t.TempDir()})
	if err != nil {
		t.Fatalf("BuildConfig() error: %v", err)
	}

	cfg := mustUnmarshalJSONMap(t, b)
	dns := mustMap(t, cfg["dns"])
	servers := mustSlice(t, dns["servers"])
	dnsRemote := findServerByTag(t, servers, "dns-remote")

	detour, ok := dnsRemote["detour"].(string)
	if !ok || detour == "" {
		t.Fatalf(`dns-remote should set "detour" when final is proxy outbound`)
	}
	if detour != "node-n1" {
		t.Fatalf(`dns-remote detour mismatch: got %q want %q`, detour, "node-n1")
	}
}

func TestSingBoxAdapter_BuildConfig_HijackDNSUsesHijackDNSAction(t *testing.T) {
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
					To:       "direct",
					Priority: 100,
					Enabled:  true,
				},
				{
					ID:       "e-openai",
					From:     domain.EdgeNodeLocal,
					To:       "n1",
					Priority: 90,
					Enabled:  true,
					RuleType: domain.EdgeRuleRoute,
					RouteRule: &domain.RouteMatchRule{
						Domains: []string{"geosite:openai"},
					},
				},
			},
		},
	}

	profile := domain.ProxyConfig{
		InboundMode: domain.InboundSOCKS,
		InboundPort: 1080,
	}

	plan, err := nodegroup.CompileProxyPlan(domain.EngineSingBox, profile, frouter, nodes)
	if err != nil {
		t.Fatalf("CompileProxyPlan() error: %v", err)
	}

	b, err := a.BuildConfig(plan, GeoFiles{ArtifactsDir: t.TempDir()})
	if err != nil {
		t.Fatalf("BuildConfig() error: %v", err)
	}

	cfg := mustUnmarshalJSONMap(t, b)
	route := mustMap(t, cfg["route"])
	rules := mustSlice(t, route["rules"])

	found := false
	for _, it := range rules {
		rule, ok := it.(map[string]any)
		if !ok {
			continue
		}
		action, _ := rule["action"].(string)
		if action != "hijack-dns" {
			continue
		}
		proto, ok := rule["protocol"].([]any)
		if !ok {
			continue
		}
		for _, p := range proto {
			if s, ok := p.(string); ok && s == "dns" {
				found = true
				break
			}
		}
	}

	if !found {
		t.Fatalf("expected a DNS hijack rule with action=hijack-dns")
	}
}

func mustUnmarshalJSONMap(t *testing.T, b []byte) map[string]any {
	t.Helper()

	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}
	return m
}

func mustMap(t *testing.T, v any) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", v)
	}
	return m
}

func mustSlice(t *testing.T, v any) []any {
	t.Helper()
	s, ok := v.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", v)
	}
	return s
}

func findServerByTag(t *testing.T, servers []any, tag string) map[string]any {
	t.Helper()
	for _, it := range servers {
		srv, ok := it.(map[string]any)
		if !ok {
			continue
		}
		if got, _ := srv["tag"].(string); got == tag {
			return srv
		}
	}
	t.Fatalf("dns server tag not found: %s", tag)
	return nil
}
