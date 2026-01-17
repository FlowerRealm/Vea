package shared

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRotateLogFile_RenamesNonEmptyFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write log: %v", err)
	}

	if err := RotateLogFile(path, 7*24*time.Hour); err != nil {
		t.Fatalf("RotateLogFile: %v", err)
	}

	if _, err := os.Stat(path); err == nil {
		t.Fatalf("expected %s to be rotated away", path)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	found := false
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, "app-") && strings.HasSuffix(name, ".log") {
			info, err := e.Info()
			if err != nil {
				t.Fatalf("stat rotated: %v", err)
			}
			if info.Size() <= 0 {
				t.Fatalf("expected rotated log to be non-empty")
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("expected rotated app-*.log to be created")
	}
}

func TestRotateLogFile_PrunesOldRotatedFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")

	old := filepath.Join(dir, "app-20000101-000000.log")
	if err := os.WriteFile(old, []byte("old"), 0o600); err != nil {
		t.Fatalf("write old rotated: %v", err)
	}
	oldTime := time.Now().Add(-8 * 24 * time.Hour)
	if err := os.Chtimes(old, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes old rotated: %v", err)
	}

	keep := filepath.Join(dir, "app-29990101-000000.log")
	if err := os.WriteFile(keep, []byte("keep"), 0o600); err != nil {
		t.Fatalf("write keep rotated: %v", err)
	}

	if err := RotateLogFile(path, 7*24*time.Hour); err != nil {
		t.Fatalf("RotateLogFile: %v", err)
	}

	if _, err := os.Stat(old); err == nil {
		t.Fatalf("expected old rotated log to be pruned")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("expected keep rotated log to remain, err=%v", err)
	}
}
