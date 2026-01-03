package config

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"vea/backend/domain"
	"vea/backend/repository/events"
	"vea/backend/repository/memory"
)

func TestService_Create_WithSourceURL_DownloadsPayloadAndChecksum(t *testing.T) {
	t.Parallel()

	const payload = "hello"
	var gotUserAgent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserAgent = r.UserAgent()
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(srv.Close)

	repo := memory.NewConfigRepo(memory.NewStore(events.NewBus()))
	svc := NewService(repo, nil, nil)

	created, err := svc.Create(context.Background(), domain.Config{
		Name:      "cfg-1",
		Format:    domain.ConfigFormatXray,
		SourceURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if gotUserAgent != subscriptionUserAgent {
		t.Fatalf("expected User-Agent %q, got %q", subscriptionUserAgent, gotUserAgent)
	}
	if created.Payload != payload {
		t.Fatalf("expected payload %q, got %q", payload, created.Payload)
	}
	sum := sha256.Sum256([]byte(payload))
	expectedChecksum := hex.EncodeToString(sum[:])
	if created.Checksum != expectedChecksum {
		t.Fatalf("expected checksum %q, got %q", expectedChecksum, created.Checksum)
	}
	if created.LastSyncedAt.IsZero() {
		t.Fatalf("expected lastSyncedAt to be set")
	}
	if created.LastSyncError != "" {
		t.Fatalf("expected lastSyncError empty, got %q", created.LastSyncError)
	}
}

func TestService_Sync_UnchangedChecksum_OnlyUpdatesLastSyncedAt(t *testing.T) {
	t.Parallel()

	const payload = "same-content"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(srv.Close)

	repo := memory.NewConfigRepo(memory.NewStore(events.NewBus()))
	svc := NewService(repo, nil, nil)

	sum := sha256.Sum256([]byte(payload))
	checksum := hex.EncodeToString(sum[:])
	created, err := repo.Create(context.Background(), domain.Config{
		Name:      "cfg-1",
		Format:    domain.ConfigFormatXray,
		SourceURL: srv.URL,
		Payload:   payload,
		Checksum:  checksum,
		// 让 Sync “确实有更新”的可比较基准
		LastSyncedAt: time.Now().Add(-2 * time.Hour),
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
	if updated.Payload != payload {
		t.Fatalf("expected payload unchanged %q, got %q", payload, updated.Payload)
	}
	if updated.Checksum != created.Checksum {
		t.Fatalf("expected checksum unchanged %q, got %q", created.Checksum, updated.Checksum)
	}
	if !updated.LastSyncedAt.After(created.LastSyncedAt) {
		t.Fatalf("expected lastSyncedAt to move forward, before=%v after=%v", created.LastSyncedAt, updated.LastSyncedAt)
	}
	if updated.LastSyncError != "" {
		t.Fatalf("expected lastSyncError empty, got %q", updated.LastSyncError)
	}
}
