package memory

import (
	"context"
	"sort"
	"time"

	"vea/backend/domain"
	"vea/backend/repository"
	"vea/backend/repository/events"

	"github.com/google/uuid"
)

// ConfigRepo 配置仓储实现
type ConfigRepo struct {
	store *Store
}

// NewConfigRepo 创建配置仓储
func NewConfigRepo(store *Store) *ConfigRepo {
	return &ConfigRepo{store: store}
}

// Get 获取配置
func (r *ConfigRepo) Get(ctx context.Context, id string) (domain.Config, error) {
	r.store.RLock()
	defer r.store.RUnlock()

	cfg, ok := r.store.Configs()[id]
	if !ok {
		return domain.Config{}, repository.ErrConfigNotFound
	}
	return cfg, nil
}

// List 列出所有配置
func (r *ConfigRepo) List(ctx context.Context) ([]domain.Config, error) {
	r.store.RLock()
	configs := r.store.Configs()
	items := make([]domain.Config, 0, len(configs))
	for _, cfg := range configs {
		items = append(items, cfg)
	}
	r.store.RUnlock()

	// 在锁外排序
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})

	return items, nil
}

// Create 创建配置
func (r *ConfigRepo) Create(ctx context.Context, cfg domain.Config) (domain.Config, error) {
	if cfg.ID == "" {
		cfg.ID = uuid.NewString()
	}
	now := time.Now()
	cfg.CreatedAt = now
	cfg.UpdatedAt = now

	r.store.Lock()
	r.store.Configs()[cfg.ID] = cfg
	r.store.Unlock()

	// 在锁外发布事件
	r.store.PublishEvent(events.ConfigEvent{
		EventType: events.EventConfigCreated,
		ConfigID:  cfg.ID,
		Config:    cfg,
	})

	return cfg, nil
}

// Update 更新配置
func (r *ConfigRepo) Update(ctx context.Context, id string, cfg domain.Config) (domain.Config, error) {
	r.store.Lock()

	existing, ok := r.store.Configs()[id]
	if !ok {
		r.store.Unlock()
		return domain.Config{}, repository.ErrConfigNotFound
	}

	// 保留不可变字段
	cfg.ID = id
	cfg.CreatedAt = existing.CreatedAt
	cfg.UpdatedAt = time.Now()

	r.store.Configs()[id] = cfg
	r.store.Unlock()

	// 在锁外发布事件
	r.store.PublishEvent(events.ConfigEvent{
		EventType: events.EventConfigUpdated,
		ConfigID:  id,
		Config:    cfg,
	})

	return cfg, nil
}

// Delete 删除配置
func (r *ConfigRepo) Delete(ctx context.Context, id string) error {
	r.store.Lock()

	if _, ok := r.store.Configs()[id]; !ok {
		r.store.Unlock()
		return repository.ErrConfigNotFound
	}

	// 删除关联的节点
	for frouterID, frouter := range r.store.FRouters() {
		if frouter.SourceConfigID == id {
			delete(r.store.FRouters(), frouterID)
		}
	}

	delete(r.store.Configs(), id)
	r.store.Unlock()

	// 在锁外发布事件
	r.store.PublishEvent(events.ConfigEvent{
		EventType: events.EventConfigDeleted,
		ConfigID:  id,
	})

	return nil
}

// UpdateSyncStatus 更新同步状态
func (r *ConfigRepo) UpdateSyncStatus(ctx context.Context, id string, payload, checksum string, syncErr error) error {
	r.store.Lock()

	cfg, ok := r.store.Configs()[id]
	if !ok {
		r.store.Unlock()
		return repository.ErrConfigNotFound
	}

	cfg.Payload = payload
	cfg.Checksum = checksum
	cfg.LastSyncedAt = time.Now()
	if syncErr != nil {
		cfg.LastSyncError = syncErr.Error()
	} else {
		cfg.LastSyncError = ""
	}
	cfg.UpdatedAt = time.Now()
	r.store.Configs()[id] = cfg

	r.store.Unlock()

	// 在锁外发布事件
	r.store.PublishEvent(events.ConfigEvent{
		EventType: events.EventConfigUpdated,
		ConfigID:  id,
		Config:    cfg,
	})

	return nil
}

// 确保实现接口
var _ repository.ConfigRepository = (*ConfigRepo)(nil)
