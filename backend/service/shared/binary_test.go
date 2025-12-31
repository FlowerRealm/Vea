package shared

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindBinaryInDir_FindsInRoot(t *testing.T) {
	dir := t.TempDir()

	bin := filepath.Join(dir, "sing-box")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	got, err := FindBinaryInDir(dir, []string{"sing-box"})
	if err != nil {
		t.Fatalf("expected to find binary, got err: %v", err)
	}
	if got != bin {
		t.Fatalf("expected %q, got %q", bin, got)
	}
}

func TestFindBinaryInDir_FindsInSubdir(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sing-box-1.0.0-linux-amd64")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	bin := filepath.Join(sub, "sing-box")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	got, err := FindBinaryInDir(dir, []string{"sing-box"})
	if err != nil {
		t.Fatalf("expected to find binary, got err: %v", err)
	}
	if got != bin {
		t.Fatalf("expected %q, got %q", bin, got)
	}
}

func TestFindBinaryInDir_NotFound(t *testing.T) {
	dir := t.TempDir()

	_, err := FindBinaryInDir(dir, []string{"sing-box"})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), dir) {
		t.Fatalf("expected error to mention dir %q, got: %v", dir, err)
	}
}

func TestFindSingBoxBinary_RespectsArtifactsRoot(t *testing.T) {
	old := ArtifactsRoot
	t.Cleanup(func() { ArtifactsRoot = old })

	ArtifactsRoot = t.TempDir()

	sub := filepath.Join(ArtifactsRoot, "core", "sing-box", "sing-box-1.0.0-linux-amd64")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	bin := filepath.Join(sub, "sing-box")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	got, err := FindSingBoxBinary()
	if err != nil {
		t.Fatalf("expected to find binary, got err: %v", err)
	}
	if got != bin {
		t.Fatalf("expected %q, got %q", bin, got)
	}
}
