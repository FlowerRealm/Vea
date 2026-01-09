package proxy

import (
	"os"
	"path/filepath"
	"testing"

	"vea/backend/service/shared"
)

func TestEnsureClashGeoData_CopiesFiles(t *testing.T) {
	tmp := t.TempDir()

	originalArtifactsRoot := shared.ArtifactsRoot
	shared.ArtifactsRoot = tmp
	t.Cleanup(func() {
		shared.ArtifactsRoot = originalArtifactsRoot
	})

	geoDir := filepath.Join(tmp, shared.GeoDir)
	if err := os.MkdirAll(geoDir, 0o755); err != nil {
		t.Fatalf("mkdir geo dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(geoDir, "geoip.dat"), []byte("ip"), 0o644); err != nil {
		t.Fatalf("write geoip.dat: %v", err)
	}
	if err := os.WriteFile(filepath.Join(geoDir, "geosite.dat"), []byte("site"), 0o644); err != nil {
		t.Fatalf("write geosite.dat: %v", err)
	}
	if err := os.WriteFile(filepath.Join(geoDir, "geoip.metadb"), []byte("metadb"), 0o644); err != nil {
		t.Fatalf("write geoip.metadb: %v", err)
	}

	configDir := filepath.Join(tmp, "core", "clash")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	if err := ensureClashGeoData(configDir); err != nil {
		t.Fatalf("ensureClashGeoData() error: %v", err)
	}

	if got, err := os.ReadFile(filepath.Join(configDir, "GeoIP.dat")); err != nil || string(got) != "ip" {
		t.Fatalf("GeoIP.dat mismatch: err=%v, got=%q", err, string(got))
	}
	if got, err := os.ReadFile(filepath.Join(configDir, "GeoSite.dat")); err != nil || string(got) != "site" {
		t.Fatalf("GeoSite.dat mismatch: err=%v, got=%q", err, string(got))
	}
	if got, err := os.ReadFile(filepath.Join(configDir, "geoip.metadb")); err != nil || string(got) != "metadb" {
		t.Fatalf("geoip.metadb mismatch: err=%v, got=%q", err, string(got))
	}
}
