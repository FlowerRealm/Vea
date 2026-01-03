package persist

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"vea/backend/domain"
)

type countingStore struct {
	mu     sync.Mutex
	count  int
	notify chan struct{}
}

func (s *countingStore) Snapshot() domain.ServiceState {
	s.mu.Lock()
	s.count++
	if s.notify != nil {
		select {
		case s.notify <- struct{}{}:
		default:
		}
	}
	s.mu.Unlock()

	nodes := make([]domain.Node, 0, s.count)
	for i := 0; i < s.count; i++ {
		nodes = append(nodes, domain.Node{ID: fmt.Sprintf("n%d", i+1), Name: "n"})
	}
	return domain.ServiceState{
		Nodes: nodes,
	}
}

func (s *countingStore) LoadState(_ domain.ServiceState) {}

func TestSnapshotterV2_Load_NoFileReturnsDefaultState(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "state.json")
	s := NewSnapshotterV2(path, &countingStore{})
	state, err := s.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if state.SchemaVersion != SchemaVersion {
		t.Fatalf("expected schemaVersion %q, got %q", SchemaVersion, state.SchemaVersion)
	}
}

func TestSnapshotterV2_SaveNow_WritesSchemaVersion(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "state.json")
	store := &countingStore{}
	s := NewSnapshotterV2(path, store)

	if err := s.SaveNow(); err != nil {
		t.Fatalf("SaveNow() error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read state file: %v", err)
	}

	var state domain.ServiceState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}
	if state.SchemaVersion != SchemaVersion {
		t.Fatalf("expected schemaVersion %q, got %q", SchemaVersion, state.SchemaVersion)
	}
	if state.GeneratedAt.IsZero() {
		t.Fatalf("expected GeneratedAt to be set")
	}
	if got, want := len(state.Nodes), 1; got != want {
		t.Fatalf("expected nodes length %d, got %d", want, got)
	}
}

func TestSnapshotterV2_Schedule_DirtyTriggersSecondSave(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "state.json")
	notify := make(chan struct{}, 4)
	store := &countingStore{notify: notify}
	s := NewSnapshotterV2(path, store)
	s.SetDebounce(20 * time.Millisecond)

	s.Schedule()
	s.Schedule() // should mark dirty and cause a second save

	deadline := time.After(1 * time.Second)
	for i := 0; i < 2; i++ {
		select {
		case <-notify:
		case <-deadline:
			t.Fatalf("timeout waiting for snapshot to be saved twice")
		}
	}

	if err := s.WaitIdle(1 * time.Second); err != nil {
		t.Fatalf("WaitIdle() error: %v", err)
	}
}
