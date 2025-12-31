package persist

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"vea/backend/domain"
	"vea/backend/repository"
	"vea/backend/repository/events"
)

// SnapshotterV2 新版快照管理器
type SnapshotterV2 struct {
	path     string
	store    repository.Snapshottable
	migrator *Migrator

	mu       sync.Mutex
	pending  bool
	dirty    bool
	debounce time.Duration

	saveMu sync.Mutex
}

// NewSnapshotterV2 创建新版快照管理器
func NewSnapshotterV2(path string, store repository.Snapshottable) *SnapshotterV2 {
	return &SnapshotterV2{
		path:     path,
		store:    store,
		migrator: NewMigrator(),
		debounce: 200 * time.Millisecond,
	}
}

// SetDebounce 设置防抖延迟
func (s *SnapshotterV2) SetDebounce(d time.Duration) {
	s.mu.Lock()
	s.debounce = d
	s.mu.Unlock()
}

// SubscribeEvents 订阅事件总线（所有写操作触发持久化）
func (s *SnapshotterV2) SubscribeEvents(bus *events.Bus) {
	bus.SubscribeAll(func(event events.Event) {
		s.Schedule()
	})
}

// Schedule 调度快照（防抖）
func (s *SnapshotterV2) Schedule() {
	s.mu.Lock()
	if s.pending {
		s.dirty = true
		s.mu.Unlock()
		return
	}
	s.pending = true
	s.dirty = false
	s.mu.Unlock()

	go func() {
		for {
			s.mu.Lock()
			debounce := s.debounce
			s.mu.Unlock()

			time.Sleep(debounce)
			_ = s.save()

			s.mu.Lock()
			if s.dirty {
				s.dirty = false
				s.mu.Unlock()
				continue
			}
			s.pending = false
			s.mu.Unlock()
			return
		}
	}()
}

// SaveNow 立即保存（同步）
func (s *SnapshotterV2) SaveNow() error {
	return s.save()
}

// doSave 执行保存
func (s *SnapshotterV2) doSave() error {
	state := s.store.Snapshot()
	state.SchemaVersion = SchemaVersion
	state.GeneratedAt = time.Now()

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		log.Printf("[Snapshot] marshal failed: %v", err)
		return err
	}

	if err := s.atomicWrite(data); err != nil {
		log.Printf("[Snapshot] write failed: %v", err)
		return err
	}

	return nil
}

func (s *SnapshotterV2) save() error {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	return s.doSave()
}

// atomicWrite 原子写入
func (s *SnapshotterV2) atomicWrite(data []byte) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Load 加载状态（严格版本校验）
func (s *SnapshotterV2) Load() (domain.ServiceState, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return domain.ServiceState{SchemaVersion: SchemaVersion}, nil
		}
		return domain.ServiceState{}, err
	}

	if len(data) == 0 {
		return domain.ServiceState{SchemaVersion: SchemaVersion}, nil
	}

	// 使用版本校验器加载
	return s.migrator.Migrate(data)
}

// LoadV2 静态函数：加载状态（严格版本校验）
func LoadV2(path string) (domain.ServiceState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return domain.ServiceState{SchemaVersion: SchemaVersion}, nil
		}
		return domain.ServiceState{}, err
	}

	if len(data) == 0 {
		return domain.ServiceState{SchemaVersion: SchemaVersion}, nil
	}

	migrator := NewMigrator()
	return migrator.Migrate(data)
}

// SaveV2 静态函数：保存状态
func SaveV2(path string, state domain.ServiceState) error {
	state.SchemaVersion = SchemaVersion
	state.GeneratedAt = time.Now()

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
