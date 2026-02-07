package nodegroup

import (
	"errors"
	"testing"

	"vea/backend/domain"
	"vea/backend/repository"
)

func TestResolveFRouterNodeGroups_LowestLatency(t *testing.T) {
	t.Parallel()

	nodes := []domain.Node{
		{ID: "n1", Name: "n1", LastLatencyMS: 200},
		{ID: "n2", Name: "n2", LastLatencyMS: 50},
	}
	groups := []domain.NodeGroup{
		{ID: "g1", Name: "g1", Strategy: domain.NodeGroupStrategyLowestLatency, NodeIDs: []string{"n1", "n2"}},
	}
	frouter := domain.FRouter{
		ID:   "fr1",
		Name: "fr1",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{ID: "e1", From: domain.EdgeNodeLocal, To: "g1", Enabled: true},
			},
		},
	}

	resolved, err := ResolveFRouterNodeGroups(frouter, nodes, groups, ResolveOptions{})
	if err != nil {
		t.Fatalf("ResolveFRouterNodeGroups() error: %v", err)
	}
	if resolved.ChainProxy.Edges[0].To != "n2" {
		t.Fatalf("expected resolved to n2, got %q", resolved.ChainProxy.Edges[0].To)
	}
}

func TestResolveFRouterNodeGroups_FastestSpeed(t *testing.T) {
	t.Parallel()

	nodes := []domain.Node{
		{ID: "n1", Name: "n1", LastSpeedMbps: 1.2},
		{ID: "n2", Name: "n2", LastSpeedMbps: 9.9},
	}
	groups := []domain.NodeGroup{
		{ID: "g1", Name: "g1", Strategy: domain.NodeGroupStrategyFastestSpeed, NodeIDs: []string{"n1", "n2"}},
	}
	frouter := domain.FRouter{
		ID:   "fr1",
		Name: "fr1",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{ID: "e1", From: domain.EdgeNodeLocal, To: "g1", Enabled: true},
			},
		},
	}

	resolved, err := ResolveFRouterNodeGroups(frouter, nodes, groups, ResolveOptions{})
	if err != nil {
		t.Fatalf("ResolveFRouterNodeGroups() error: %v", err)
	}
	if resolved.ChainProxy.Edges[0].To != "n2" {
		t.Fatalf("expected resolved to n2, got %q", resolved.ChainProxy.Edges[0].To)
	}
}

func TestResolveFRouterNodeGroups_RoundRobin_UpdatesCursor(t *testing.T) {
	t.Parallel()

	nodes := []domain.Node{
		{ID: "n1", Name: "n1"},
		{ID: "n2", Name: "n2"},
	}
	groups := []domain.NodeGroup{
		{ID: "g1", Name: "g1", Strategy: domain.NodeGroupStrategyRoundRobin, NodeIDs: []string{"n1", "n2"}, Cursor: 0},
	}
	frouter := domain.FRouter{
		ID:   "fr1",
		Name: "fr1",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{ID: "e1", From: domain.EdgeNodeLocal, To: "g1", Enabled: true},
			},
		},
	}

	var updatedID string
	var updatedCursor int
	resolved, err := ResolveFRouterNodeGroups(frouter, nodes, groups, ResolveOptions{
		AdvanceCursor: true,
		UpdateCursor: func(groupID string, cursor int) error {
			updatedID = groupID
			updatedCursor = cursor
			return nil
		},
	})
	if err != nil {
		t.Fatalf("ResolveFRouterNodeGroups() error: %v", err)
	}
	if resolved.ChainProxy.Edges[0].To != "n1" {
		t.Fatalf("expected resolved to n1, got %q", resolved.ChainProxy.Edges[0].To)
	}
	if updatedID != "g1" || updatedCursor != 1 {
		t.Fatalf("expected cursor update g1->1, got %q->%d", updatedID, updatedCursor)
	}
}

func TestResolveFRouterNodeGroups_Failover_UsesNextAndUpdatesCursor(t *testing.T) {
	t.Parallel()

	nodes := []domain.Node{
		{ID: "n1", Name: "n1", LastLatencyError: "dial tcp: timeout"},
		{ID: "n2", Name: "n2"},
		{ID: "n3", Name: "n3"},
	}
	groups := []domain.NodeGroup{
		{ID: "g1", Name: "g1", Strategy: domain.NodeGroupStrategyFailover, NodeIDs: []string{"n1", "n2", "n3"}, Cursor: 0},
	}
	frouter := domain.FRouter{
		ID:   "fr1",
		Name: "fr1",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{ID: "e1", From: domain.EdgeNodeLocal, To: "g1", Enabled: true},
			},
		},
	}

	var updatedID string
	var updatedCursor int
	resolved, err := ResolveFRouterNodeGroups(frouter, nodes, groups, ResolveOptions{
		AdvanceCursor: true,
		UpdateCursor: func(groupID string, cursor int) error {
			updatedID = groupID
			updatedCursor = cursor
			return nil
		},
	})
	if err != nil {
		t.Fatalf("ResolveFRouterNodeGroups() error: %v", err)
	}
	if resolved.ChainProxy.Edges[0].To != "n2" {
		t.Fatalf("expected resolved to n2, got %q", resolved.ChainProxy.Edges[0].To)
	}
	if updatedID != "g1" || updatedCursor != 1 {
		t.Fatalf("expected cursor update g1->1, got %q->%d", updatedID, updatedCursor)
	}
}

func TestResolveFRouterNodeGroups_Failover_AllUnhealthy_AllowFallback(t *testing.T) {
	t.Parallel()

	nodes := []domain.Node{
		{ID: "n1", Name: "n1", LastLatencyError: "dial tcp: timeout"},
		{ID: "n2", Name: "n2", LastLatencyError: "dial tcp: timeout"},
	}
	groups := []domain.NodeGroup{
		{ID: "g1", Name: "g1", Strategy: domain.NodeGroupStrategyFailover, NodeIDs: []string{"n1", "n2"}, Cursor: 1},
	}
	frouter := domain.FRouter{
		ID:   "fr1",
		Name: "fr1",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{ID: "e1", From: domain.EdgeNodeLocal, To: "g1", Enabled: true},
			},
		},
	}

	resolved, err := ResolveFRouterNodeGroups(frouter, nodes, groups, ResolveOptions{
		AllowFailoverFallback: true,
	})
	if err != nil {
		t.Fatalf("ResolveFRouterNodeGroups() error: %v", err)
	}
	if resolved.ChainProxy.Edges[0].To != "n2" {
		t.Fatalf("expected resolved to cursor target n2, got %q", resolved.ChainProxy.Edges[0].To)
	}
}

func TestResolveFRouterNodeGroups_Failover_AllUnhealthy_ReturnsErrorWithoutFallback(t *testing.T) {
	t.Parallel()

	nodes := []domain.Node{
		{ID: "n1", Name: "n1", LastLatencyError: "dial tcp: timeout"},
		{ID: "n2", Name: "n2", LastLatencyError: "dial tcp: timeout"},
	}
	groups := []domain.NodeGroup{
		{ID: "g1", Name: "g1", Strategy: domain.NodeGroupStrategyFailover, NodeIDs: []string{"n1", "n2"}, Cursor: 0},
	}
	frouter := domain.FRouter{
		ID:   "fr1",
		Name: "fr1",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{ID: "e1", From: domain.EdgeNodeLocal, To: "g1", Enabled: true},
			},
		},
	}

	_, err := ResolveFRouterNodeGroups(frouter, nodes, groups, ResolveOptions{})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestResolveFRouterNodeGroups_NodeIDWinsOverNodeGroupID(t *testing.T) {
	t.Parallel()

	nodes := []domain.Node{
		{ID: "same", Name: "n1"},
	}
	groups := []domain.NodeGroup{
		{ID: "same", Name: "g1", Strategy: domain.NodeGroupStrategyRoundRobin, NodeIDs: []string{"same"}},
	}
	frouter := domain.FRouter{
		ID:   "fr1",
		Name: "fr1",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{ID: "e1", From: domain.EdgeNodeLocal, To: "same", Enabled: true},
			},
		},
	}

	called := false
	resolved, err := ResolveFRouterNodeGroups(frouter, nodes, groups, ResolveOptions{
		AdvanceCursor: true,
		UpdateCursor: func(string, int) error {
			called = true
			return nil
		},
	})
	if err != nil {
		t.Fatalf("ResolveFRouterNodeGroups() error: %v", err)
	}
	if resolved.ChainProxy.Edges[0].To != "same" {
		t.Fatalf("expected resolved to same, got %q", resolved.ChainProxy.Edges[0].To)
	}
	if called {
		t.Fatalf("expected UpdateCursor not to be called when id matches node")
	}
}

func TestResolveFRouterNodeGroups_ResolvesSlotBoundNodeID(t *testing.T) {
	t.Parallel()

	nodes := []domain.Node{
		{ID: "n1", Name: "n1"},
		{ID: "n2", Name: "n2"},
	}
	groups := []domain.NodeGroup{
		{ID: "g1", Name: "g1", Strategy: domain.NodeGroupStrategyRoundRobin, NodeIDs: []string{"n1", "n2"}, Cursor: 0},
	}
	frouter := domain.FRouter{
		ID:   "fr1",
		Name: "fr1",
		ChainProxy: domain.ChainProxySettings{
			Slots: []domain.SlotNode{
				{ID: "slot-1", Name: "slot-1", BoundNodeID: "g1"},
			},
			Edges: []domain.ProxyEdge{
				{ID: "e1", From: domain.EdgeNodeLocal, To: "slot-1", Enabled: true},
			},
		},
	}

	resolved, err := ResolveFRouterNodeGroups(frouter, nodes, groups, ResolveOptions{})
	if err != nil {
		t.Fatalf("ResolveFRouterNodeGroups() error: %v", err)
	}
	if len(resolved.ChainProxy.Slots) != 1 {
		t.Fatalf("expected 1 slot, got %d", len(resolved.ChainProxy.Slots))
	}
	if resolved.ChainProxy.Slots[0].BoundNodeID != "n1" {
		t.Fatalf("expected slot boundNodeId resolved to n1, got %q", resolved.ChainProxy.Slots[0].BoundNodeID)
	}
}

func TestResolveFRouterNodeGroups_UnknownID_ReturnsResolveError(t *testing.T) {
	t.Parallel()

	frouter := domain.FRouter{
		ID:   "fr1",
		Name: "fr1",
		ChainProxy: domain.ChainProxySettings{
			Edges: []domain.ProxyEdge{
				{ID: "e1", From: domain.EdgeNodeLocal, To: "unknown", Enabled: true},
			},
		},
	}

	_, err := ResolveFRouterNodeGroups(frouter, nil, nil, ResolveOptions{})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	var re *ResolveError
	if !errors.As(err, &re) {
		t.Fatalf("expected ResolveError, got %T", err)
	}
	if !errors.Is(err, repository.ErrInvalidData) {
		t.Fatalf("expected ErrInvalidData unwrap, got %v", err)
	}
}
