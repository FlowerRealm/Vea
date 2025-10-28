package persist

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"vea/internal/domain"
	"vea/internal/service"
)

type Snapshotter struct {
	path    string
	service *service.Service
	mu      sync.Mutex
	pending bool
}

func NewSnapshotter(path string, svc *service.Service) *Snapshotter {
	return &Snapshotter{path: path, service: svc}
}

func (s *Snapshotter) Schedule() {
	s.mu.Lock()
	if s.pending {
		s.mu.Unlock()
		return
	}
	s.pending = true
	s.mu.Unlock()

	go func() {
		time.Sleep(200 * time.Millisecond)
		state := s.service.Snapshot()
		if err := Save(s.path, state); err != nil {
			log.Printf("snapshot save failed: %v", err)
		}
		s.mu.Lock()
		s.pending = false
		s.mu.Unlock()
	}()
}

func Save(path string, state domain.ServiceState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	state.GeneratedAt = time.Now()
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func Load(path string) (domain.ServiceState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return domain.ServiceState{}, nil
		}
		return domain.ServiceState{}, err
	}
	if len(data) == 0 {
		return domain.ServiceState{}, nil
	}
	var state domain.ServiceState
	if err := json.Unmarshal(data, &state); err != nil {
		return domain.ServiceState{}, err
	}
	return state, nil
}
