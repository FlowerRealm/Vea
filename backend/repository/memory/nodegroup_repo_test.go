package memory

import (
	"context"
	"testing"

	"vea/backend/domain"
)

func TestNodeGroupRepo_CRUD(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewStore(nil)
	repo := NewNodeGroupRepo(store)

	created, err := repo.Create(ctx, domain.NodeGroup{
		Name:     "g1",
		NodeIDs:  []string{"n1", "n2"},
		Strategy: domain.NodeGroupStrategyLowestLatency,
		Tags:     []string{"a", " b ", "a"},
	})
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("expected id to be set")
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatalf("expected createdAt/updatedAt to be set")
	}

	got, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got.Name != "g1" {
		t.Fatalf("expected name g1, got %q", got.Name)
	}
	if got.Strategy != domain.NodeGroupStrategyLowestLatency {
		t.Fatalf("expected strategy %q, got %q", domain.NodeGroupStrategyLowestLatency, got.Strategy)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "a" || got.Tags[1] != "b" {
		t.Fatalf("expected tags normalized, got %+v", got.Tags)
	}

	got.Cursor = 2
	updated, err := repo.Update(ctx, got.ID, domain.NodeGroup{
		Name:     "g1-updated",
		NodeIDs:  got.NodeIDs,
		Strategy: got.Strategy,
		Tags:     got.Tags,
		Cursor:   got.Cursor,
	})
	if err != nil {
		t.Fatalf("Update() error: %v", err)
	}
	if updated.Name != "g1-updated" {
		t.Fatalf("expected updated name, got %q", updated.Name)
	}
	if updated.Cursor != 2 {
		t.Fatalf("expected cursor=2, got %d", updated.Cursor)
	}
	if !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Fatalf("expected CreatedAt preserved")
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("expected list to contain created group")
	}

	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
	list, err = repo.List(ctx)
	if err != nil {
		t.Fatalf("List() after delete error: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected empty list after delete, got %d", len(list))
	}
}
