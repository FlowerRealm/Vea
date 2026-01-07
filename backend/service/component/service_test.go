package component

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"vea/backend/domain"
	"vea/backend/repository/memory"
	"vea/backend/service/shared"
)

func TestList_SeedsDefaultComponents(t *testing.T) {
	t.Parallel()

	store := memory.NewStore(nil)
	repo := memory.NewComponentRepo(store)
	svc := NewService(repo)

	components, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}

	if countKind(components, domain.ComponentXray) != 1 {
		t.Fatalf("expected exactly 1 xray component, got %d", countKind(components, domain.ComponentXray))
	}
	if countKind(components, domain.ComponentSingBox) != 1 {
		t.Fatalf("expected exactly 1 singbox component, got %d", countKind(components, domain.ComponentSingBox))
	}
	if countKind(components, domain.ComponentClash) != 1 {
		t.Fatalf("expected exactly 1 clash component, got %d", countKind(components, domain.ComponentClash))
	}

	components2, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List (second call) returned error: %v", err)
	}

	if countKind(components2, domain.ComponentXray) != 1 {
		t.Fatalf("expected exactly 1 xray component after second List, got %d", countKind(components2, domain.ComponentXray))
	}
	if countKind(components2, domain.ComponentSingBox) != 1 {
		t.Fatalf("expected exactly 1 singbox component after second List, got %d", countKind(components2, domain.ComponentSingBox))
	}
	if countKind(components2, domain.ComponentClash) != 1 {
		t.Fatalf("expected exactly 1 clash component after second List, got %d", countKind(components2, domain.ComponentClash))
	}
}

func TestCreate_CoreComponent_IsIdempotent(t *testing.T) {
	t.Parallel()

	store := memory.NewStore(nil)
	repo := memory.NewComponentRepo(store)
	svc := NewService(repo)

	xray1, err := svc.Create(context.Background(), domain.CoreComponent{Kind: domain.ComponentXray})
	if err != nil {
		t.Fatalf("Create xray returned error: %v", err)
	}
	if xray1.Kind != domain.ComponentXray {
		t.Fatalf("expected xray kind, got %q", xray1.Kind)
	}
	if xray1.Name != "Xray" {
		t.Fatalf("expected default xray name %q, got %q", "Xray", xray1.Name)
	}
	if xray1.Meta == nil || xray1.Meta["repo"] != "XTLS/Xray-core" {
		t.Fatalf("expected default xray meta repo %q, got %#v", "XTLS/Xray-core", xray1.Meta)
	}

	xray2, err := svc.Create(context.Background(), domain.CoreComponent{Kind: domain.ComponentXray})
	if err != nil {
		t.Fatalf("Create xray (second) returned error: %v", err)
	}
	if xray2.ID != xray1.ID {
		t.Fatalf("expected idempotent xray create to return same ID, got %q vs %q", xray1.ID, xray2.ID)
	}

	components, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if countKind(components, domain.ComponentXray) != 1 {
		t.Fatalf("expected exactly 1 xray component, got %d", countKind(components, domain.ComponentXray))
	}
	if countKind(components, domain.ComponentSingBox) != 1 {
		t.Fatalf("expected exactly 1 singbox component, got %d", countKind(components, domain.ComponentSingBox))
	}
	if countKind(components, domain.ComponentClash) != 1 {
		t.Fatalf("expected exactly 1 clash component, got %d", countKind(components, domain.ComponentClash))
	}
}

func TestList_DetectsInstalledSingBoxInSubdir(t *testing.T) {
	old := shared.ArtifactsRoot
	t.Cleanup(func() { shared.ArtifactsRoot = old })

	shared.ArtifactsRoot = t.TempDir()
	sub := filepath.Join(shared.ArtifactsRoot, "core", "sing-box", "sing-box-1.0.0-linux-amd64")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sub, "sing-box"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	store := memory.NewStore(nil)
	repo := memory.NewComponentRepo(store)
	svc := NewService(repo)

	components, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}

	var singbox *domain.CoreComponent
	for i := range components {
		if components[i].Kind == domain.ComponentSingBox {
			singbox = &components[i]
			break
		}
	}
	if singbox == nil {
		t.Fatalf("expected singbox component to exist")
	}
	if singbox.InstallDir == "" {
		t.Fatalf("expected singbox InstallDir to be set")
	}
	if singbox.LastInstalledAt.IsZero() {
		t.Fatalf("expected singbox LastInstalledAt to be set")
	}
}

func countKind(components []domain.CoreComponent, kind domain.CoreComponentKind) int {
	count := 0
	for _, comp := range components {
		if comp.Kind == kind {
			count++
		}
	}
	return count
}
