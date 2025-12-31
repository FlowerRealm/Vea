package nodegroup

import (
	"testing"

	"vea/backend/domain"
)

func TestCompileFRouter_ViaBuildsDetourChain(t *testing.T) {
	t.Parallel()

	nodes := []domain.Node{
		{ID: "n1", Name: "n1"},
		{ID: "n2", Name: "n2"},
		{ID: "n3", Name: "n3"},
	}
	frouter := domain.FRouter{
		ID:   "fr1",
		Name: "test",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{
					ID:      "e-default",
					From:    domain.EdgeNodeLocal,
					To:      domain.EdgeNodeDirect,
					Enabled: true,
				},
				{
					ID:       "e-rule",
					From:     domain.EdgeNodeLocal,
					To:       "n1",
					Via:      []string{"n2", "n3"},
					Priority: 10,
					Enabled:  true,
					RuleType: domain.EdgeRuleRoute,
					RouteRule: &domain.RouteMatchRule{
						Domains: []string{"domain:example.com"},
					},
				},
			},
		},
	}

	compiled, err := CompileFRouter(frouter, nodes)
	if err != nil {
		t.Fatalf("CompileFRouter() error: %v", err)
	}

	if got, want := compiled.DetourUpstream["n1"], "n2"; got != want {
		t.Fatalf("detour n1 mismatch: got %q want %q", got, want)
	}
	if got, want := compiled.DetourUpstream["n2"], "n3"; got != want {
		t.Fatalf("detour n2 mismatch: got %q want %q", got, want)
	}
}

func TestCompileFRouter_ViaAndDetourEdgeSameDoesNotConflict(t *testing.T) {
	t.Parallel()

	nodes := []domain.Node{
		{ID: "n1", Name: "n1"},
		{ID: "n2", Name: "n2"},
	}
	frouter := domain.FRouter{
		ID:   "fr1",
		Name: "test",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{
					ID:      "e-default",
					From:    domain.EdgeNodeLocal,
					To:      domain.EdgeNodeDirect,
					Enabled: true,
				},
				{
					ID:       "e-rule",
					From:     domain.EdgeNodeLocal,
					To:       "n1",
					Via:      []string{"n2"},
					Priority: 10,
					Enabled:  true,
					RuleType: domain.EdgeRuleRoute,
					RouteRule: &domain.RouteMatchRule{
						Domains: []string{"domain:example.com"},
					},
				},
				{
					ID:      "e-detour",
					From:    "n1",
					To:      "n2",
					Enabled: true,
				},
			},
		},
	}

	if _, err := CompileFRouter(frouter, nodes); err != nil {
		t.Fatalf("CompileFRouter() error: %v", err)
	}
}

func TestCompileFRouter_ViaConflictsWithDetourEdge(t *testing.T) {
	t.Parallel()

	nodes := []domain.Node{
		{ID: "n1", Name: "n1"},
		{ID: "n2", Name: "n2"},
		{ID: "n3", Name: "n3"},
	}
	frouter := domain.FRouter{
		ID:   "fr1",
		Name: "test",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{
					ID:      "e-default",
					From:    domain.EdgeNodeLocal,
					To:      domain.EdgeNodeDirect,
					Enabled: true,
				},
				{
					ID:       "e-rule",
					From:     domain.EdgeNodeLocal,
					To:       "n1",
					Via:      []string{"n2"},
					Priority: 10,
					Enabled:  true,
					RuleType: domain.EdgeRuleRoute,
					RouteRule: &domain.RouteMatchRule{
						Domains: []string{"domain:example.com"},
					},
				},
				{
					ID:      "e-detour",
					From:    "n1",
					To:      "n3",
					Enabled: true,
				},
			},
		},
	}

	if _, err := CompileFRouter(frouter, nodes); err == nil {
		t.Fatalf("CompileFRouter() expected error, got nil")
	}
}

func TestCompileFRouter_ViaOnDirectIsError(t *testing.T) {
	t.Parallel()

	nodes := []domain.Node{
		{ID: "n1", Name: "n1"},
	}
	frouter := domain.FRouter{
		ID:   "fr1",
		Name: "test",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{
					ID:      "e-default",
					From:    domain.EdgeNodeLocal,
					To:      domain.EdgeNodeDirect,
					Via:     []string{"n1"},
					Enabled: true,
				},
			},
		},
	}

	if _, err := CompileFRouter(frouter, nodes); err == nil {
		t.Fatalf("CompileFRouter() expected error, got nil")
	}
}
