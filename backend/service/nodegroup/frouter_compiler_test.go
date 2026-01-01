package nodegroup

import (
	"strings"
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

func TestCompileFRouter_MissingDefaultEdge(t *testing.T) {
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
					ID:       "e-rule",
					From:     domain.EdgeNodeLocal,
					To:       "n1",
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

	_, err := CompileFRouter(frouter, nodes)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "missing default edge") || !strings.Contains(err.Error(), "require exactly one") {
		t.Fatalf("expected missing default edge error, got: %v", err)
	}
}

func TestCompileFRouter_MultipleDefaultEdges(t *testing.T) {
	t.Parallel()

	frouter := domain.FRouter{
		ID:   "fr1",
		Name: "test",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{ID: "e1", From: domain.EdgeNodeLocal, To: domain.EdgeNodeDirect, Enabled: true},
				{ID: "e2", From: domain.EdgeNodeLocal, To: domain.EdgeNodeBlock, Enabled: true},
			},
		},
	}

	_, err := CompileFRouter(frouter, nil)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "multiple default edges") {
		t.Fatalf("expected multiple default edges error, got: %v", err)
	}
}

func TestCompileFRouter_RouteRuleProtocolsNotSupportedYet(t *testing.T) {
	t.Parallel()

	nodes := []domain.Node{
		{ID: "n1", Name: "n1"},
	}
	frouter := domain.FRouter{
		ID:   "fr1",
		Name: "test",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{ID: "e-default", From: domain.EdgeNodeLocal, To: domain.EdgeNodeDirect, Enabled: true},
				{
					ID:       "e-rule",
					From:     domain.EdgeNodeLocal,
					To:       "n1",
					Priority: 10,
					Enabled:  true,
					RuleType: domain.EdgeRuleRoute,
					RouteRule: &domain.RouteMatchRule{
						Protocols: []string{"tcp"},
					},
				},
			},
		},
	}

	_, err := CompileFRouter(frouter, nodes)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "protocols is not supported yet") {
		t.Fatalf("expected protocols not supported error, got: %v", err)
	}
}

func TestCompileFRouter_SlotUnboundEdgeSkippedWithWarning(t *testing.T) {
	t.Parallel()

	nodes := []domain.Node{
		{ID: "n1", Name: "n1"},
	}
	frouter := domain.FRouter{
		ID:   "fr1",
		Name: "test",
		ChainProxy: domain.ChainProxySettings{
			Slots: []domain.SlotNode{
				{ID: "slot-1", Name: "slot-1", BoundNodeID: ""}, // unbound
			},
			Edges: []domain.ProxyEdge{
				{ID: "e-default", From: domain.EdgeNodeLocal, To: domain.EdgeNodeDirect, Enabled: true},
				{ID: "e-slot", From: domain.EdgeNodeLocal, To: "slot-1", Enabled: true, RuleType: domain.EdgeRuleRoute, RouteRule: &domain.RouteMatchRule{Domains: []string{"domain:example.com"}}},
			},
		},
	}

	compiled, err := CompileFRouter(frouter, nodes)
	if err != nil {
		t.Fatalf("CompileFRouter() error: %v", err)
	}
	if len(compiled.Warnings) == 0 {
		t.Fatalf("expected warnings, got none")
	}
	found := false
	for _, w := range compiled.Warnings {
		if strings.Contains(w, "slot slot-1 is unbound") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected unbound slot warning, got: %v", compiled.Warnings)
	}
}

func TestCompileFRouter_SlotBoundResolvesToNode(t *testing.T) {
	t.Parallel()

	nodes := []domain.Node{
		{ID: "n1", Name: "n1"},
	}
	frouter := domain.FRouter{
		ID:   "fr1",
		Name: "test",
		ChainProxy: domain.ChainProxySettings{
			Slots: []domain.SlotNode{
				{ID: "slot-1", Name: "slot-1", BoundNodeID: "n1"},
			},
			Edges: []domain.ProxyEdge{
				{ID: "e-default", From: domain.EdgeNodeLocal, To: "slot-1", Enabled: true},
			},
		},
	}

	compiled, err := CompileFRouter(frouter, nodes)
	if err != nil {
		t.Fatalf("CompileFRouter() error: %v", err)
	}
	if compiled.Default.Kind != ActionNode || compiled.Default.NodeID != "n1" {
		t.Fatalf("expected default to resolve to node n1, got %v", compiled.Default)
	}
}

func TestCompileFRouter_DetourCycleIsError(t *testing.T) {
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
				{ID: "e-default", From: domain.EdgeNodeLocal, To: "n1", Enabled: true},
				{ID: "e12", From: "n1", To: "n2", Enabled: true},
				{ID: "e21", From: "n2", To: "n1", Enabled: true},
			},
		},
	}

	_, err := CompileFRouter(frouter, nodes)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "detour cycle:") {
		t.Fatalf("expected detour cycle error, got: %v", err)
	}
}

func TestActiveNodeIDs_IncludesDetourUpstreamClosure(t *testing.T) {
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
					To:      "n1",
					Via:     []string{"n2"},
					Enabled: true,
				},
			},
		},
	}

	compiled, err := CompileFRouter(frouter, nodes)
	if err != nil {
		t.Fatalf("CompileFRouter() error: %v", err)
	}
	ids := ActiveNodeIDs(compiled)
	if _, ok := ids["n1"]; !ok {
		t.Fatalf("expected active to include n1")
	}
	if _, ok := ids["n2"]; !ok {
		t.Fatalf("expected active to include detour upstream n2")
	}
}

func TestCompileFRouter_RulesSortedByPriorityThenEdgeID(t *testing.T) {
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
				{ID: "e-default", From: domain.EdgeNodeLocal, To: domain.EdgeNodeDirect, Enabled: true},
				{
					ID:       "b",
					From:     domain.EdgeNodeLocal,
					To:       "n1",
					Priority: 10,
					Enabled:  true,
					RuleType: domain.EdgeRuleRoute,
					RouteRule: &domain.RouteMatchRule{
						Domains: []string{"domain:b.example.com"},
					},
				},
				{
					ID:       "a",
					From:     domain.EdgeNodeLocal,
					To:       "n2",
					Priority: 10,
					Enabled:  true,
					RuleType: domain.EdgeRuleRoute,
					RouteRule: &domain.RouteMatchRule{
						Domains: []string{"domain:a.example.com"},
					},
				},
			},
		},
	}

	compiled, err := CompileFRouter(frouter, nodes)
	if err != nil {
		t.Fatalf("CompileFRouter() error: %v", err)
	}
	if len(compiled.Rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(compiled.Rules))
	}
	if compiled.Rules[0].EdgeID != "a" || compiled.Rules[1].EdgeID != "b" {
		t.Fatalf("expected rules sorted by edgeID for equal priority, got: %v then %v", compiled.Rules[0].EdgeID, compiled.Rules[1].EdgeID)
	}
}
