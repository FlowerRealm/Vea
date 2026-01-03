package proxy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadKernelLogChunk_FileNotFoundReturnsEmpty(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "missing.log")
	from, to, end, lost, text, err := readKernelLogChunk(path, 0, 10)
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if from != 0 || to != 0 || end != 0 || lost {
		t.Fatalf("unexpected offsets: from=%d to=%d end=%d lost=%v", from, to, end, lost)
	}
	if text != "" {
		t.Fatalf("expected empty text, got %q", text)
	}
}

func TestReadKernelLogChunk_SinceBeyondEndResetsToZeroAndMarksLost(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "kernel.log")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	from, to, end, lost, text, err := readKernelLogChunk(path, 999, 1024)
	if err != nil {
		t.Fatalf("readKernelLogChunk() error: %v", err)
	}
	if !lost {
		t.Fatalf("expected lost=true")
	}
	if from != 0 {
		t.Fatalf("expected from reset to 0, got %d", from)
	}
	if end != 5 {
		t.Fatalf("expected end=5, got %d", end)
	}
	if to != 5 {
		t.Fatalf("expected to=5, got %d", to)
	}
	if text != "hello" {
		t.Fatalf("expected text %q, got %q", "hello", text)
	}
}

func TestReadKernelLogChunk_RespectsMaxBytes(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "kernel.log")
	content := strings.Repeat("a", 1024)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, to, end, lost, text, err := readKernelLogChunk(path, 0, 10)
	if err != nil {
		t.Fatalf("readKernelLogChunk() error: %v", err)
	}
	if lost {
		t.Fatalf("expected lost=false")
	}
	if end != int64(len(content)) {
		t.Fatalf("expected end=%d, got %d", len(content), end)
	}
	if to != 10 {
		t.Fatalf("expected to=10, got %d", to)
	}
	if len(text) != 10 {
		t.Fatalf("expected text length 10, got %d", len(text))
	}
}

func TestReadKernelLogChunk_MaxBytesMustBePositive(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "kernel.log")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if _, _, _, _, _, err := readKernelLogChunk(path, 0, 0); err == nil {
		t.Fatalf("expected error, got nil")
	}
}
