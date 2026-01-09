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

	singbox1, err := svc.Create(context.Background(), domain.CoreComponent{Kind: domain.ComponentSingBox})
	if err != nil {
		t.Fatalf("Create singbox returned error: %v", err)
	}
	if singbox1.Kind != domain.ComponentSingBox {
		t.Fatalf("expected singbox kind, got %q", singbox1.Kind)
	}
	if singbox1.Name != "sing-box" {
		t.Fatalf("expected default singbox name %q, got %q", "sing-box", singbox1.Name)
	}
	if singbox1.Meta == nil || singbox1.Meta["repo"] != "SagerNet/sing-box" {
		t.Fatalf("expected default singbox meta repo %q, got %#v", "SagerNet/sing-box", singbox1.Meta)
	}

	singbox2, err := svc.Create(context.Background(), domain.CoreComponent{Kind: domain.ComponentSingBox})
	if err != nil {
		t.Fatalf("Create singbox (second) returned error: %v", err)
	}
	if singbox2.ID != singbox1.ID {
		t.Fatalf("expected idempotent singbox create to return same ID, got %q vs %q", singbox1.ID, singbox2.ID)
	}

	components, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List returned error: %v", err)
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

func TestUninstall_RemovesInstallDirAndClearsState(t *testing.T) {
	old := shared.ArtifactsRoot
	t.Cleanup(func() { shared.ArtifactsRoot = old })

	shared.ArtifactsRoot = t.TempDir()
	installDir := filepath.Join(shared.ArtifactsRoot, "core", "sing-box")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "sing-box"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	store := memory.NewStore(nil)
	repo := memory.NewComponentRepo(store)
	svc := NewService(repo)

	components, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}

	var singboxID string
	for _, comp := range components {
		if comp.Kind == domain.ComponentSingBox {
			singboxID = comp.ID
			break
		}
	}
	if singboxID == "" {
		t.Fatalf("expected singbox component to exist")
	}

	before, err := repo.GetByKind(context.Background(), domain.ComponentSingBox)
	if err != nil {
		t.Fatalf("GetByKind returned error: %v", err)
	}
	if before.InstallDir == "" || before.LastInstalledAt.IsZero() {
		t.Fatalf("expected singbox component to be detected as installed before uninstall")
	}

	updated, err := svc.Uninstall(context.Background(), singboxID)
	if err != nil {
		t.Fatalf("Uninstall returned error: %v", err)
	}
	if updated.InstallDir != "" {
		t.Fatalf("expected InstallDir to be cleared after uninstall")
	}
	if !updated.LastInstalledAt.IsZero() {
		t.Fatalf("expected LastInstalledAt to be cleared after uninstall")
	}

	if _, err := os.Stat(installDir); err == nil || !os.IsNotExist(err) {
		t.Fatalf("expected install dir to be removed, got err=%v", err)
	}

	after, err := repo.GetByKind(context.Background(), domain.ComponentSingBox)
	if err != nil {
		t.Fatalf("GetByKind (after) returned error: %v", err)
	}
	if after.InstallDir != "" || !after.LastInstalledAt.IsZero() {
		t.Fatalf("expected repo state to be cleared after uninstall, got dir=%q installedAt=%v", after.InstallDir, after.LastInstalledAt)
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
