package theme

import (
	"archive/zip"
	"context"
	"errors"
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

func TestImportZipThemePackSucceeds(t *testing.T) {
	root := t.TempDir()
	svc := NewService(Options{UserDataRoot: root})

	zipPath := filepath.Join(root, "pack.zip")
	manifest := `{"schemaVersion":1,"name":"Pack","themes":[{"id":"dark","entry":"dark/index.html"},{"id":"light","entry":"light/index.html"}],"defaultTheme":"light"}`
	if err := writeZip(zipPath, map[string]string{
		"myPack/manifest.json":     manifest,
		"myPack/dark/index.html":   "<!doctype html><title>dark</title>",
		"myPack/light/index.html":  "<!doctype html><title>light</title>",
		"myPack/light/css/app.css": "body{color:#000}",
	}); err != nil {
		t.Fatalf("write zip: %v", err)
	}

	id, err := svc.ImportZip(context.Background(), zipPath)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if id != "myPack/light" {
		t.Fatalf("expected virtual theme id myPack/light, got %q", id)
	}

	manifestPath, err := shared.SafeJoin(filepath.Join(root, "themes"), "myPack/manifest.json")
	if err != nil {
		t.Fatalf("safe join: %v", err)
	}
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("expected manifest to exist: %v", err)
	}

	entryPath, err := shared.SafeJoin(filepath.Join(root, "themes"), "myPack/light/index.html")
	if err != nil {
		t.Fatalf("safe join: %v", err)
	}
	if _, err := os.Stat(entryPath); err != nil {
		t.Fatalf("expected entry to exist: %v", err)
	}
}

func TestImportZipThemePackRejectsPathTraversalEntry(t *testing.T) {
	root := t.TempDir()
	svc := NewService(Options{UserDataRoot: root})

	zipPath := filepath.Join(root, "pack-traversal.zip")
	manifest := `{"schemaVersion":1,"themes":[{"id":"dark","entry":"../evil/index.html"}]}`
	if err := writeZip(zipPath, map[string]string{
		"myPack/manifest.json":   manifest,
		"myPack/dark/index.html": "<!doctype html><title>dark</title>",
	}); err != nil {
		t.Fatalf("write zip: %v", err)
	}

	if _, err := svc.ImportZip(context.Background(), zipPath); err == nil {
		t.Fatalf("expected error, got nil")
	} else if !errors.Is(err, ErrThemeManifestInvalid) {
		t.Fatalf("expected ErrThemeManifestInvalid, got %v", err)
	}
}

func TestListExpandsThemePack(t *testing.T) {
	root := t.TempDir()
	themesRoot := filepath.Join(root, "themes")
	if err := os.MkdirAll(filepath.Join(themesRoot, "myPack", "dark"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(themesRoot, "myPack", "manifest.json"), []byte(`{"schemaVersion":1,"name":"Pack","themes":[{"id":"dark","name":"Dark","entry":"dark/index.html"}]}`), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(themesRoot, "myPack", "dark", "index.html"), []byte("<!doctype html>"), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}

	svc := NewService(Options{UserDataRoot: root})
	themes, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	found := false
	for _, theme := range themes {
		if theme.ID != "myPack/dark" {
			continue
		}
		if theme.Entry != "myPack/dark/index.html" {
			t.Fatalf("expected entry myPack/dark/index.html, got %q", theme.Entry)
		}
		if theme.PackID != "myPack" {
			t.Fatalf("expected packId myPack, got %q", theme.PackID)
		}
		if theme.PackName != "Pack" {
			t.Fatalf("expected packName Pack, got %q", theme.PackName)
		}
		if theme.Name != "Dark" {
			t.Fatalf("expected name Dark, got %q", theme.Name)
		}
		if !theme.HasIndex {
			t.Fatalf("expected hasIndex true")
		}
		found = true
	}
	if !found {
		t.Fatalf("expected myPack/dark theme to exist")
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
