package store

import (
	"testing"

	"vea/backend/domain"
)

func TestDeleteConfigRemovesAssociatedNodes(t *testing.T) {
	t.Parallel()

	s := NewMemoryStore()

	cfg := s.CreateConfig(domain.Config{Name: "subscription"})
	other := s.CreateConfig(domain.Config{Name: "manual"})

	nodeFromConfig := s.CreateNode(domain.Node{
		Name:           "cfg-node",
		SourceConfigID: cfg.ID,
	})
	s.CreateNode(domain.Node{
		Name:           "other-node",
		SourceConfigID: other.ID,
	})

	if _, err := s.UpdateTrafficProfile(func(profile domain.TrafficProfile) (domain.TrafficProfile, error) {
		profile.DefaultNodeID = nodeFromConfig.ID
		return profile, nil
	}); err != nil {
		t.Fatalf("set default node: %v", err)
	}

	if err := s.DeleteConfig(cfg.ID); err != nil {
		t.Fatalf("DeleteConfig(%s) returned error: %v", cfg.ID, err)
	}

	if nodes := s.ListNodesByConfig(cfg.ID); len(nodes) != 0 {
		t.Fatalf("expected nodes for config to be removed, got %d", len(nodes))
	}

	if nodes := s.ListNodesByConfig(other.ID); len(nodes) != 1 {
		t.Fatalf("expected nodes for other config to remain, got %d", len(nodes))
	}

	if profile := s.GetTrafficProfile(); profile.DefaultNodeID != "" {
		t.Fatalf("expected default node to be cleared, got %q", profile.DefaultNodeID)
	}
}

func TestCleanupOrphanNodes(t *testing.T) {
	t.Parallel()

	s := NewMemoryStore()

	cfg := s.CreateConfig(domain.Config{Name: "alive"})
	orphan := s.CreateNode(domain.Node{
		Name:           "dangling",
		SourceConfigID: "missing-config",
	})
	keep := s.CreateNode(domain.Node{
		Name:           "valid",
		SourceConfigID: cfg.ID,
	})

	if _, err := s.UpdateTrafficProfile(func(profile domain.TrafficProfile) (domain.TrafficProfile, error) {
		profile.DefaultNodeID = orphan.ID
		return profile, nil
	}); err != nil {
		t.Fatalf("set default node: %v", err)
	}

	if removed := s.CleanupOrphanNodes(); removed != 1 {
		t.Fatalf("CleanupOrphanNodes removed %d nodes, want 1", removed)
	}

	if _, err := s.GetNode(orphan.ID); err == nil {
		t.Fatalf("orphan node still exists after cleanup")
	}
	if _, err := s.GetNode(keep.ID); err != nil {
		t.Fatalf("valid node removed unexpectedly: %v", err)
	}
	if profile := s.GetTrafficProfile(); profile.DefaultNodeID != "" {
		t.Fatalf("expected default node cleared, got %q", profile.DefaultNodeID)
	}
}
