package theme

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"testing"

	"vea/backend/service/shared"
)

func TestImportZipRejectsPathTraversal(t *testing.T) {
	root := t.TempDir()
	svc := NewService(Options{
		UserDataRoot:     root,
		MaxZipBytes:      10 << 20,
		MaxUnpackedBytes: 10 << 20,
		MaxFiles:         100,
		MaxDepth:         10,
	})

	zipPath := filepath.Join(root, "traversal.zip")
	if err := writeZip(zipPath, map[string]string{
		"dark/index.html":     "<!doctype html>",
		"dark/../../evil.txt": "oops",
	}); err != nil {
		t.Fatalf("write zip: %v", err)
	}

	if _, err := svc.ImportZip(context.Background(), zipPath); err == nil {
		t.Fatalf("expected error, got nil")
	}

	if _, err := os.Stat(filepath.Join(root, "evil.txt")); err == nil {
		t.Fatalf("unexpected evil file extracted")
	}
}

func TestImportZipRejectsMissingIndex(t *testing.T) {
	root := t.TempDir()
	svc := NewService(Options{UserDataRoot: root})

	zipPath := filepath.Join(root, "missing-index.zip")
	if err := writeZip(zipPath, map[string]string{
		"dark/readme.txt": "no index",
	}); err != nil {
		t.Fatalf("write zip: %v", err)
	}

	_, err := svc.ImportZip(context.Background(), zipPath)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err != ErrThemeMissingIndex {
		t.Fatalf("expected ErrThemeMissingIndex, got %v", err)
	}
}

func TestImportZipSucceeds(t *testing.T) {
	root := t.TempDir()
	svc := NewService(Options{UserDataRoot: root})

	zipPath := filepath.Join(root, "ok.zip")
	if err := writeZip(zipPath, map[string]string{
		"dark/index.html":            "<!doctype html><title>ok</title>",
		"dark/css/app.css":           "body{background:#000}",
		"__MACOSX/._dark/index.html": "ignored",
		"dark/.DS_Store":             "ignored",
	}); err != nil {
		t.Fatalf("write zip: %v", err)
	}

	id, err := svc.ImportZip(context.Background(), zipPath)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if id != "dark" {
		t.Fatalf("expected theme id dark, got %q", id)
	}

	indexPath, err := shared.SafeJoin(filepath.Join(root, "themes"), "dark/index.html")
	if err != nil {
		t.Fatalf("safe join: %v", err)
	}
	if _, err := os.Stat(indexPath); err != nil {
		t.Fatalf("expected index to exist: %v", err)
	}
}

func writeZip(path string, files map[string]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			zw.Close()
			return err
		}
		if _, err := w.Write([]byte(content)); err != nil {
			zw.Close()
			return err
		}
	}
	return zw.Close()
}
