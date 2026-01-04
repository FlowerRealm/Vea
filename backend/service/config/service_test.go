package config

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"vea/backend/domain"
	"vea/backend/repository/events"
	"vea/backend/repository/memory"
	"vea/backend/service/nodes"
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
	svc := NewService(repo, nil)

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
	svc := NewService(repo, nil)

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

func TestService_Sync_ParseFailure_DoesNotClearExistingNodes(t *testing.T) {
	t.Parallel()

	var payload atomic.Value
	payload.Store("")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, payload.Load().(string))
	}))
	t.Cleanup(srv.Close)

	store := memory.NewStore(events.NewBus())
	configRepo := memory.NewConfigRepo(store)
	nodeRepo := memory.NewNodeRepo(store)
	nodeSvc := nodes.NewService(nodeRepo)
	svc := NewService(configRepo, nodeSvc)

	payload.Store("vless://11111111-1111-1111-1111-111111111111@example.com:443?security=tls#n1")
	created, err := svc.Create(context.Background(), domain.Config{
		Name:      "cfg-1",
		Format:    domain.ConfigFormatXray,
		SourceURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	nodesBefore, err := nodeRepo.ListByConfigID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("list nodes before: %v", err)
	}
	if len(nodesBefore) != 1 {
		t.Fatalf("expected nodes=1 before sync, got %d", len(nodesBefore))
	}

	payload.Store("port: 7890\nsocks-port: 7891\nProxy:\n  - name: 您的客户端版本过旧\n    type: socks5\n    server: 127.0.0.1\n    port: 1080\n")
	if err := svc.Sync(context.Background(), created.ID); err == nil {
		t.Fatalf("expected sync to fail on unsupported subscription payload")
	}

	nodesAfter, err := nodeRepo.ListByConfigID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("list nodes after: %v", err)
	}
	if len(nodesAfter) != 1 {
		t.Fatalf("expected nodes preserved after parse failure, got %d", len(nodesAfter))
	}

	updatedCfg, err := configRepo.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("get config: %v", err)
	}
	if updatedCfg.LastSyncError == "" {
		t.Fatalf("expected lastSyncError to be set on parse failure")
	}
}
