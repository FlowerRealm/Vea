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

// ComponentRepo 组件仓储实现
type ComponentRepo struct {
	store *Store
}

// NewComponentRepo 创建组件仓储
func NewComponentRepo(store *Store) *ComponentRepo {
	return &ComponentRepo{store: store}
}

// Get 获取组件
func (r *ComponentRepo) Get(ctx context.Context, id string) (domain.CoreComponent, error) {
	r.store.RLock()
	defer r.store.RUnlock()

	comp, ok := r.store.Components()[id]
	if !ok {
		return domain.CoreComponent{}, repository.ErrComponentNotFound
	}
	return comp, nil
}

// List 列出所有组件
func (r *ComponentRepo) List(ctx context.Context) ([]domain.CoreComponent, error) {
	r.store.RLock()
	components := r.store.Components()
	items := make([]domain.CoreComponent, 0, len(components))
	for _, comp := range components {
		items = append(items, comp)
	}
	r.store.RUnlock()

	// 在锁外排序
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})

	return items, nil
}

// Create 创建组件
func (r *ComponentRepo) Create(ctx context.Context, comp domain.CoreComponent) (domain.CoreComponent, error) {
	if comp.ID == "" {
		comp.ID = uuid.NewString()
	}
	now := time.Now()
	comp.CreatedAt = now
	comp.UpdatedAt = now

	r.store.Lock()
	r.store.Components()[comp.ID] = comp
	r.store.Unlock()

	// 在锁外发布事件（修复原有 bug：CreateComponent 未触发持久化）
	r.store.PublishEvent(events.ComponentEvent{
		EventType:   events.EventComponentCreated,
		ComponentID: comp.ID,
		Component:   comp,
	})

	return comp, nil
}

// Update 更新组件
func (r *ComponentRepo) Update(ctx context.Context, id string, comp domain.CoreComponent) (domain.CoreComponent, error) {
	r.store.Lock()

	existing, ok := r.store.Components()[id]
	if !ok {
		r.store.Unlock()
		return domain.CoreComponent{}, repository.ErrComponentNotFound
	}

	// 保留不可变字段
	comp.ID = id
	comp.CreatedAt = existing.CreatedAt
	comp.UpdatedAt = time.Now()

	// 保留关键字段（如果未显式设置）
	if comp.InstallDir == "" {
		comp.InstallDir = existing.InstallDir
	}
	if comp.LastInstalledAt.IsZero() {
		comp.LastInstalledAt = existing.LastInstalledAt
	}
	if comp.Checksum == "" {
		comp.Checksum = existing.Checksum
	}
	if comp.Meta == nil {
		comp.Meta = existing.Meta
	}

	r.store.Components()[id] = comp
	r.store.Unlock()

	// 在锁外发布事件
	r.store.PublishEvent(events.ComponentEvent{
		EventType:   events.EventComponentUpdated,
		ComponentID: id,
		Component:   comp,
	})

	return comp, nil
}

// Delete 删除组件
func (r *ComponentRepo) Delete(ctx context.Context, id string) error {
	r.store.Lock()

	if _, ok := r.store.Components()[id]; !ok {
		r.store.Unlock()
		return repository.ErrComponentNotFound
	}

	delete(r.store.Components(), id)
	r.store.Unlock()

	// 在锁外发布事件
	r.store.PublishEvent(events.ComponentEvent{
		EventType:   events.EventComponentDeleted,
		ComponentID: id,
	})

	return nil
}

// GetByKind 按类型查询组件
func (r *ComponentRepo) GetByKind(ctx context.Context, kind domain.CoreComponentKind) (domain.CoreComponent, error) {
	r.store.RLock()
	defer r.store.RUnlock()

	for _, comp := range r.store.Components() {
		if comp.Kind == kind {
			return comp, nil
		}
	}

	return domain.CoreComponent{}, repository.ErrComponentNotFound
}

// UpdateInstallStatus 更新安装状态
func (r *ComponentRepo) UpdateInstallStatus(ctx context.Context, id string, status domain.InstallStatus, progress int, message string) error {
	r.store.Lock()

	comp, ok := r.store.Components()[id]
	if !ok {
		r.store.Unlock()
		return repository.ErrComponentNotFound
	}

	comp.InstallStatus = status
	comp.InstallProgress = progress
	comp.InstallMessage = message
	r.store.Components()[id] = comp

	r.store.Unlock()

	return nil
}

// SetInstalled 设置安装完成
func (r *ComponentRepo) SetInstalled(ctx context.Context, id string, dir, version, checksum string) error {
	r.store.Lock()

	comp, ok := r.store.Components()[id]
	if !ok {
		r.store.Unlock()
		return repository.ErrComponentNotFound
	}

	now := time.Now()
	comp.InstallDir = dir
	comp.LastVersion = version
	comp.Checksum = checksum
	comp.LastInstalledAt = now
	comp.InstallStatus = domain.InstallStatusIdle
	comp.InstallProgress = 0
	comp.InstallMessage = ""
	comp.LastSyncError = ""
	comp.UpdatedAt = now
	r.store.Components()[id] = comp

	r.store.Unlock()

	// 在锁外发布事件
	r.store.PublishEvent(events.ComponentEvent{
		EventType:   events.EventComponentUpdated,
		ComponentID: id,
		Component:   comp,
	})

	return nil
}

// ClearSyncError 清除同步错误
func (r *ComponentRepo) ClearSyncError(ctx context.Context, id string) error {
	r.store.Lock()

	comp, ok := r.store.Components()[id]
	if !ok {
		r.store.Unlock()
		return repository.ErrComponentNotFound
	}

	comp.LastSyncError = ""
	comp.UpdatedAt = time.Now()
	r.store.Components()[id] = comp

	r.store.Unlock()

	// 在锁外发布事件
	r.store.PublishEvent(events.ComponentEvent{
		EventType:   events.EventComponentUpdated,
		ComponentID: id,
		Component:   comp,
	})

	return nil
}

// UpdateMeta 更新元数据
func (r *ComponentRepo) UpdateMeta(ctx context.Context, id string, key, value string) error {
	r.store.Lock()

	comp, ok := r.store.Components()[id]
	if !ok {
		r.store.Unlock()
		return repository.ErrComponentNotFound
	}

	if comp.Meta == nil {
		comp.Meta = make(map[string]string)
	}
	comp.Meta[key] = value
	comp.UpdatedAt = time.Now()
	r.store.Components()[id] = comp

	r.store.Unlock()

	// 在锁外发布事件
	r.store.PublishEvent(events.ComponentEvent{
		EventType:   events.EventComponentUpdated,
		ComponentID: id,
		Component:   comp,
	})

	return nil
}

// 确保实现接口
var _ repository.ComponentRepository = (*ComponentRepo)(nil)
