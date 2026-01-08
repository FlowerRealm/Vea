package shared

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateLegacyData_MovesDirsWhenDestMissing(t *testing.T) {
	legacyRoot := t.TempDir()
	dstRoot := t.TempDir()

	legacyData := filepath.Join(legacyRoot, "data")
	legacyArtifacts := filepath.Join(legacyRoot, "artifacts")

	if err := os.MkdirAll(legacyData, 0o755); err != nil {
		t.Fatalf("mkdir legacy data: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(legacyArtifacts, "runtime"), 0o755); err != nil {
		t.Fatalf("mkdir legacy artifacts: %v", err)
	}

	if err := os.WriteFile(filepath.Join(legacyData, "state.json"), []byte("old-state"), 0o644); err != nil {
		t.Fatalf("write legacy state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyData, "vea.dev.log"), []byte("log"), 0o644); err != nil {
		t.Fatalf("write legacy data file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyArtifacts, "runtime", "app.log"), []byte("app"), 0o644); err != nil {
		t.Fatalf("write legacy app log: %v", err)
	}

	if err := MigrateLegacyData(LegacyDataMigrationOptions{
		UserDataRoot: dstRoot,
		LegacyRoots:  []string{legacyRoot},
	}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dstRoot, "data", "state.json")); err != nil {
		t.Fatalf("expected migrated state.json, got err: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dstRoot, "data", "vea.dev.log")); err != nil {
		t.Fatalf("expected migrated data file, got err: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dstRoot, "artifacts", "runtime", "app.log")); err != nil {
		t.Fatalf("expected migrated artifacts file, got err: %v", err)
	}

	if _, err := os.Stat(legacyData); !os.IsNotExist(err) {
		t.Fatalf("expected legacy data dir removed, got err: %v", err)
	}
	if _, err := os.Stat(legacyArtifacts); !os.IsNotExist(err) {
		t.Fatalf("expected legacy artifacts dir removed, got err: %v", err)
	}
}

func TestMigrateLegacyData_MergesWithoutOverwriteWhenDestExists(t *testing.T) {
	legacyRoot := t.TempDir()
	dstRoot := t.TempDir()

	dstData := filepath.Join(dstRoot, "data")
	dstArtifacts := filepath.Join(dstRoot, "artifacts", "runtime")
	if err := os.MkdirAll(dstData, 0o755); err != nil {
		t.Fatalf("mkdir dst data: %v", err)
	}
	if err := os.MkdirAll(dstArtifacts, 0o755); err != nil {
		t.Fatalf("mkdir dst artifacts: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dstData, "state.json"), []byte("new-state"), 0o644); err != nil {
		t.Fatalf("write dst state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dstArtifacts, "app.log"), []byte("new-app"), 0o644); err != nil {
		t.Fatalf("write dst app: %v", err)
	}

	legacyData := filepath.Join(legacyRoot, "data")
	legacyArtifacts := filepath.Join(legacyRoot, "artifacts", "runtime")
	if err := os.MkdirAll(legacyData, 0o755); err != nil {
		t.Fatalf("mkdir legacy data: %v", err)
	}
	if err := os.MkdirAll(legacyArtifacts, 0o755); err != nil {
		t.Fatalf("mkdir legacy artifacts: %v", err)
	}

	if err := os.WriteFile(filepath.Join(legacyData, "state.json"), []byte("old-state"), 0o644); err != nil {
		t.Fatalf("write legacy state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyData, "legacy.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatalf("write legacy extra: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyArtifacts, "app.log"), []byte("old-app"), 0o644); err != nil {
		t.Fatalf("write legacy app: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyArtifacts, "legacy.log"), []byte("keep"), 0o644); err != nil {
		t.Fatalf("write legacy app extra: %v", err)
	}

	if err := MigrateLegacyData(LegacyDataMigrationOptions{
		UserDataRoot: dstRoot,
		LegacyRoots:  []string{legacyRoot},
	}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	gotState, err := os.ReadFile(filepath.Join(dstData, "state.json"))
	if err != nil {
		t.Fatalf("read dst state: %v", err)
	}
	if string(gotState) != "new-state" {
		t.Fatalf("expected dst state preserved, got %q", string(gotState))
	}

	gotApp, err := os.ReadFile(filepath.Join(dstArtifacts, "app.log"))
	if err != nil {
		t.Fatalf("read dst app: %v", err)
	}
	if string(gotApp) != "new-app" {
		t.Fatalf("expected dst app preserved, got %q", string(gotApp))
	}

	if _, err := os.Stat(filepath.Join(dstData, "legacy.txt")); err != nil {
		t.Fatalf("expected legacy extra migrated, got err: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dstArtifacts, "legacy.log")); err != nil {
		t.Fatalf("expected legacy artifacts extra migrated, got err: %v", err)
	}

	if _, err := os.Stat(filepath.Join(legacyRoot, "data")); !os.IsNotExist(err) {
		t.Fatalf("expected legacy data dir removed, got err: %v", err)
	}
	if _, err := os.Stat(filepath.Join(legacyRoot, "artifacts")); !os.IsNotExist(err) {
		t.Fatalf("expected legacy artifacts dir removed, got err: %v", err)
	}
}
