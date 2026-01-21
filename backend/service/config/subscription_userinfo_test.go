package config

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"vea/backend/domain"
	"vea/backend/repository/events"
	"vea/backend/repository/memory"
)

func TestParseSubscriptionUserinfo_Basic(t *testing.T) {
	t.Parallel()

	used, total := parseSubscriptionUserinfo("upload=1; download=2; total=10; expire=0")
	if used == nil || total == nil {
		t.Fatalf("expected used/total, got nil")
	}
	if *used != 3 {
		t.Fatalf("expected used=3, got %d", *used)
	}
	if *total != 10 {
		t.Fatalf("expected total=10, got %d", *total)
	}
}

func TestParseSubscriptionUserinfo_CommaSeparated(t *testing.T) {
	t.Parallel()

	used, total := parseSubscriptionUserinfo("upload=1000, download=2000, total=5000")
	if used == nil || total == nil {
		t.Fatalf("expected used/total, got nil")
	}
	if *used != 3000 {
		t.Fatalf("expected used=3000, got %d", *used)
	}
	if *total != 5000 {
		t.Fatalf("expected total=5000, got %d", *total)
	}
}

func TestParseSubscriptionUserinfo_MissingFields(t *testing.T) {
	t.Parallel()

	used, total := parseSubscriptionUserinfo("upload=1; total=10")
	if used != nil || total != nil {
		t.Fatalf("expected nil, got used=%v total=%v", used, total)
	}
}

func TestParseSubscriptionUserinfo_InvalidNumber(t *testing.T) {
	t.Parallel()

	used, total := parseSubscriptionUserinfo("upload=abc; download=1; total=10")
	if used != nil || total != nil {
		t.Fatalf("expected nil, got used=%v total=%v", used, total)
	}
}

func TestParseSubscriptionUserinfo_Overflow(t *testing.T) {
	t.Parallel()

	used, total := parseSubscriptionUserinfo("upload=18446744073709551615; download=1; total=10")
	if used != nil || total != nil {
		t.Fatalf("expected nil, got used=%v total=%v", used, total)
	}
}

func TestService_Sync_UserinfoHeader_UpdatesUsageEvenWhenChecksumUnchanged(t *testing.T) {
	t.Parallel()

	var call atomic.Int64
	const payload = "same-content"
	sum := sha256.Sum256([]byte(payload))
	checksum := hex.EncodeToString(sum[:])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := call.Add(1)
		if n == 1 {
			w.Header().Set("subscription-userinfo", "upload=1; download=2; total=10")
		} else {
			w.Header().Set("subscription-userinfo", "upload=3; download=4; total=10")
		}
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(srv.Close)

	repo := memory.NewConfigRepo(memory.NewStore(events.NewBus()))
	svc := NewService(context.Background(), repo, nil, nil)

	created, err := repo.Create(context.Background(), domain.Config{
		Name:         "cfg-1",
		Format:       domain.ConfigFormatSubscription,
		SourceURL:    srv.URL,
		Payload:      payload,
		Checksum:     checksum,
		LastSyncedAt: time.Now().Add(-2 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := svc.Sync(context.Background(), created.ID); err != nil {
		t.Fatalf("sync 1: %v", err)
	}
	updated, err := repo.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("get 1: %v", err)
	}
	if updated.UsageUsedBytes == nil || updated.UsageTotalBytes == nil {
		t.Fatalf("expected usage to be set after first sync")
	}
	if *updated.UsageUsedBytes != 3 || *updated.UsageTotalBytes != 10 {
		t.Fatalf("expected usage 3/10, got %d/%d", *updated.UsageUsedBytes, *updated.UsageTotalBytes)
	}

	if err := svc.Sync(context.Background(), created.ID); err != nil {
		t.Fatalf("sync 2: %v", err)
	}
	updated2, err := repo.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("get 2: %v", err)
	}
	if updated2.UsageUsedBytes == nil || updated2.UsageTotalBytes == nil {
		t.Fatalf("expected usage to be set after second sync")
	}
	if *updated2.UsageUsedBytes != 7 || *updated2.UsageTotalBytes != 10 {
		t.Fatalf("expected usage 7/10, got %d/%d", *updated2.UsageUsedBytes, *updated2.UsageTotalBytes)
	}
}

func TestService_Sync_UserinfoHeaderMissing_PreservesExistingUsage(t *testing.T) {
	t.Parallel()

	const payload = "same-content"
	sum := sha256.Sum256([]byte(payload))
	checksum := hex.EncodeToString(sum[:])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(srv.Close)

	repo := memory.NewConfigRepo(memory.NewStore(events.NewBus()))
	svc := NewService(context.Background(), repo, nil, nil)

	used := int64(123)
	total := int64(456)
	created, err := repo.Create(context.Background(), domain.Config{
		Name:            "cfg-1",
		Format:          domain.ConfigFormatSubscription,
		SourceURL:       srv.URL,
		Payload:         payload,
		Checksum:        checksum,
		UsageUsedBytes:  &used,
		UsageTotalBytes: &total,
		LastSyncedAt:    time.Now().Add(-2 * time.Hour),
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
	if updated.UsageUsedBytes == nil || updated.UsageTotalBytes == nil {
		t.Fatalf("expected usage to be preserved")
	}
	if *updated.UsageUsedBytes != 123 || *updated.UsageTotalBytes != 456 {
		t.Fatalf("expected usage 123/456, got %d/%d", *updated.UsageUsedBytes, *updated.UsageTotalBytes)
	}
}
