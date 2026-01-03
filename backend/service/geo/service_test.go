package geo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"vea/backend/domain"
	"vea/backend/repository/events"
	"vea/backend/repository/memory"
	"vea/backend/service/shared"
)

func TestService_Sync_DownloadsToArtifactsAndUpdatesRepo(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	origArtifactsRoot := shared.ArtifactsRoot
	shared.ArtifactsRoot = tmp
	t.Cleanup(func() { shared.ArtifactsRoot = origArtifactsRoot })

	const content = "geoip-data"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(content))
	}))
	t.Cleanup(srv.Close)

	repo := memory.NewGeoRepo(memory.NewStore(events.NewBus()))
	svc := NewService(repo)

	created, err := repo.Create(context.Background(), domain.GeoResource{
		Name:      "GeoIP",
		Type:      domain.GeoIP,
		SourceURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := svc.Sync(context.Background(), created.ID); err != nil {
		t.Fatalf("sync: %v", err)
	}

	updated, err := repo.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	expectedPath := filepath.Join(tmp, shared.GeoDir, "geoip.dat")
	if updated.ArtifactPath != expectedPath {
		t.Fatalf("expected artifactPath %q, got %q", expectedPath, updated.ArtifactPath)
	}
	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != content {
		t.Fatalf("expected file content %q, got %q", content, string(data))
	}

	sum := sha256.Sum256([]byte(content))
	expectedChecksum := hex.EncodeToString(sum[:])
	if updated.Checksum != expectedChecksum {
		t.Fatalf("expected checksum %q, got %q", expectedChecksum, updated.Checksum)
	}
	if updated.FileSizeBytes != int64(len(content)) {
		t.Fatalf("expected fileSizeBytes %d, got %d", len(content), updated.FileSizeBytes)
	}
	if updated.LastSynced.IsZero() {
		t.Fatalf("expected lastSynced to be set")
	}
	if updated.LastSyncError != "" {
		t.Fatalf("expected lastSyncError empty, got %q", updated.LastSyncError)
	}
}
