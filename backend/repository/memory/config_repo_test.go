package memory

import (
	"context"
	"errors"
	"testing"

	"vea/backend/domain"
	"vea/backend/repository"
)

func TestConfigRepoDeleteRemovesNodesAndFRouters(t *testing.T) {
	ctx := context.Background()

	store := NewStore(nil)
	nodeRepo := NewNodeRepo(store)
	frouterRepo := NewFRouterRepo(store)
	configRepo := NewConfigRepo(store)

	cfg1, err := configRepo.Create(ctx, domain.Config{Name: "cfg1", Format: domain.ConfigFormatXray})
	if err != nil {
		t.Fatalf("create cfg1: %v", err)
	}
	cfg2, err := configRepo.Create(ctx, domain.Config{Name: "cfg2", Format: domain.ConfigFormatXray})
	if err != nil {
		t.Fatalf("create cfg2: %v", err)
	}

	if _, err := nodeRepo.ReplaceNodesForConfig(ctx, cfg1.ID, []domain.Node{
		{Name: "n1", Protocol: domain.ProtocolVLESS, Address: "example.com", Port: 443},
		{Name: "n2", Protocol: domain.ProtocolTrojan, Address: "example.net", Port: 443},
	}); err != nil {
		t.Fatalf("replace nodes for cfg1: %v", err)
	}
	if _, err := nodeRepo.ReplaceNodesForConfig(ctx, cfg2.ID, []domain.Node{
		{Name: "m1", Protocol: domain.ProtocolVLESS, Address: "example.org", Port: 443},
	}); err != nil {
		t.Fatalf("replace nodes for cfg2: %v", err)
	}

	if _, err := frouterRepo.Create(ctx, domain.FRouter{Name: "fr1", SourceConfigID: cfg1.ID}); err != nil {
		t.Fatalf("create fr1: %v", err)
	}
	if _, err := frouterRepo.Create(ctx, domain.FRouter{Name: "fr2", SourceConfigID: cfg2.ID}); err != nil {
		t.Fatalf("create fr2: %v", err)
	}

	if err := configRepo.Delete(ctx, cfg1.ID); err != nil {
		t.Fatalf("delete cfg1: %v", err)
	}

	if _, err := configRepo.Get(ctx, cfg1.ID); !errors.Is(err, repository.ErrConfigNotFound) {
		t.Fatalf("expected ErrConfigNotFound for cfg1, got %v", err)
	}

	nodesCfg1, err := nodeRepo.ListByConfigID(ctx, cfg1.ID)
	if err != nil {
		t.Fatalf("list nodes cfg1: %v", err)
	}
	if len(nodesCfg1) != 0 {
		t.Fatalf("expected cfg1 nodes removed, got %d", len(nodesCfg1))
	}

	nodesCfg2, err := nodeRepo.ListByConfigID(ctx, cfg2.ID)
	if err != nil {
		t.Fatalf("list nodes cfg2: %v", err)
	}
	if len(nodesCfg2) != 1 {
		t.Fatalf("expected cfg2 nodes kept, got %d", len(nodesCfg2))
	}

	frouters, err := frouterRepo.List(ctx)
	if err != nil {
		t.Fatalf("list frouters: %v", err)
	}
	if len(frouters) != 1 || frouters[0].SourceConfigID != cfg2.ID {
		t.Fatalf("expected only cfg2 frouter kept, got %+v", frouters)
	}
}
