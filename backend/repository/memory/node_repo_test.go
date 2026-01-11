package memory

import (
	"context"
	"testing"

	"vea/backend/domain"
)

func TestNodeRepoReplaceNodesForConfig_StableIDAndPreservesRuntimeMetrics(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewStore(nil)
	repo := NewNodeRepo(store)

	nodes, err := repo.ReplaceNodesForConfig(ctx, "cfg1", []domain.Node{
		{Name: "n1", Protocol: domain.ProtocolVLESS, Address: "example.com", Port: 443},
	})
	if err != nil {
		t.Fatalf("ReplaceNodesForConfig() error: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	id := nodes[0].ID
	if id == "" {
		t.Fatalf("expected node id to be set")
	}
	createdAt := nodes[0].CreatedAt

	if err := repo.UpdateLatency(ctx, id, 123, ""); err != nil {
		t.Fatalf("UpdateLatency() error: %v", err)
	}
	if err := repo.UpdateSpeed(ctx, id, 88.8, ""); err != nil {
		t.Fatalf("UpdateSpeed() error: %v", err)
	}

	nodes2, err := repo.ReplaceNodesForConfig(ctx, "cfg1", []domain.Node{
		{Name: "n1", Protocol: domain.ProtocolVLESS, Address: "example.com", Port: 443},
	})
	if err != nil {
		t.Fatalf("ReplaceNodesForConfig() second error: %v", err)
	}
	if len(nodes2) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes2))
	}
	if nodes2[0].ID != id {
		t.Fatalf("expected stable node id %q, got %q", id, nodes2[0].ID)
	}
	if !nodes2[0].CreatedAt.Equal(createdAt) {
		t.Fatalf("expected CreatedAt preserved, got %v vs %v", nodes2[0].CreatedAt, createdAt)
	}
	if nodes2[0].LastLatencyMS != 123 || nodes2[0].LastLatencyAt.IsZero() {
		t.Fatalf("expected latency preserved, got ms=%d at=%v", nodes2[0].LastLatencyMS, nodes2[0].LastLatencyAt)
	}
	if nodes2[0].LastSpeedMbps != 88.8 || nodes2[0].LastSpeedAt.IsZero() {
		t.Fatalf("expected speed preserved, got mbps=%v at=%v", nodes2[0].LastSpeedMbps, nodes2[0].LastSpeedAt)
	}
}

func TestNodeRepoReplaceNodesForConfig_RemovesMissingNodes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewStore(nil)
	repo := NewNodeRepo(store)

	if _, err := repo.ReplaceNodesForConfig(ctx, "cfg1", []domain.Node{
		{Name: "n1", Protocol: domain.ProtocolVLESS, Address: "example.com", Port: 443},
		{Name: "n2", Protocol: domain.ProtocolTrojan, Address: "example.net", Port: 443},
	}); err != nil {
		t.Fatalf("replace cfg1: %v", err)
	}
	if _, err := repo.ReplaceNodesForConfig(ctx, "cfg2", []domain.Node{
		{Name: "m1", Protocol: domain.ProtocolVLESS, Address: "example.org", Port: 443},
	}); err != nil {
		t.Fatalf("replace cfg2: %v", err)
	}

	if _, err := repo.ReplaceNodesForConfig(ctx, "cfg1", []domain.Node{
		{Name: "n1", Protocol: domain.ProtocolVLESS, Address: "example.com", Port: 443},
	}); err != nil {
		t.Fatalf("replace cfg1 second: %v", err)
	}

	nodesCfg1, err := repo.ListByConfigID(ctx, "cfg1")
	if err != nil {
		t.Fatalf("ListByConfigID cfg1: %v", err)
	}
	if len(nodesCfg1) != 1 {
		t.Fatalf("expected cfg1 nodes=1, got %d", len(nodesCfg1))
	}

	nodesCfg2, err := repo.ListByConfigID(ctx, "cfg2")
	if err != nil {
		t.Fatalf("ListByConfigID cfg2: %v", err)
	}
	if len(nodesCfg2) != 1 {
		t.Fatalf("expected cfg2 nodes=1, got %d", len(nodesCfg2))
	}
}

func TestStableNodeIDForConfig_DiffConfigProducesDiffID(t *testing.T) {
	t.Parallel()

	n := domain.Node{Name: "n1", Protocol: domain.ProtocolVLESS, Address: "example.com", Port: 443}
	id1 := stableNodeIDForConfig("cfg1", n)
	id2 := stableNodeIDForConfig("cfg2", n)
	if id1 == "" || id2 == "" {
		t.Fatalf("expected stable ids to be non-empty")
	}
	if id1 == id2 {
		t.Fatalf("expected different config to produce different IDs, got %q", id1)
	}
}

func TestNodeRepoReplaceNodesForConfig_NilSliceClearsOnlyThatConfig(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewStore(nil)
	repo := NewNodeRepo(store)

	if _, err := repo.ReplaceNodesForConfig(ctx, "cfg1", []domain.Node{
		{Name: "n1", Protocol: domain.ProtocolVLESS, Address: "example.com", Port: 443},
		{Name: "n2", Protocol: domain.ProtocolTrojan, Address: "example.net", Port: 443},
	}); err != nil {
		t.Fatalf("replace cfg1: %v", err)
	}
	if _, err := repo.ReplaceNodesForConfig(ctx, "cfg2", []domain.Node{
		{Name: "m1", Protocol: domain.ProtocolVLESS, Address: "example.org", Port: 443},
	}); err != nil {
		t.Fatalf("replace cfg2: %v", err)
	}

	if _, err := repo.ReplaceNodesForConfig(ctx, "cfg1", domain.ClearNodes); err != nil {
		t.Fatalf("clear cfg1: %v", err)
	}

	nodesCfg1, err := repo.ListByConfigID(ctx, "cfg1")
	if err != nil {
		t.Fatalf("ListByConfigID cfg1: %v", err)
	}
	if len(nodesCfg1) != 0 {
		t.Fatalf("expected cfg1 nodes=0 after clear, got %d", len(nodesCfg1))
	}

	nodesCfg2, err := repo.ListByConfigID(ctx, "cfg2")
	if err != nil {
		t.Fatalf("ListByConfigID cfg2: %v", err)
	}
	if len(nodesCfg2) != 1 {
		t.Fatalf("expected cfg2 nodes=1, got %d", len(nodesCfg2))
	}
}
