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

// GeoRepo Geo 资源仓储实现
type GeoRepo struct {
	store *Store
}

// NewGeoRepo 创建 Geo 资源仓储
func NewGeoRepo(store *Store) *GeoRepo {
	return &GeoRepo{store: store}
}

// Get 获取 Geo 资源
func (r *GeoRepo) Get(ctx context.Context, id string) (domain.GeoResource, error) {
	r.store.RLock()
	defer r.store.RUnlock()

	geo, ok := r.store.Geo()[id]
	if !ok {
		return domain.GeoResource{}, repository.ErrGeoNotFound
	}
	return geo, nil
}

// List 列出所有 Geo 资源
func (r *GeoRepo) List(ctx context.Context) ([]domain.GeoResource, error) {
	r.store.RLock()
	geoMap := r.store.Geo()
	items := make([]domain.GeoResource, 0, len(geoMap))
	for _, geo := range geoMap {
		items = append(items, geo)
	}
	r.store.RUnlock()

	// 在锁外排序
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})

	return items, nil
}

// Create 创建 Geo 资源
func (r *GeoRepo) Create(ctx context.Context, geo domain.GeoResource) (domain.GeoResource, error) {
	if geo.ID == "" {
		geo.ID = uuid.NewString()
	}
	now := time.Now()
	geo.CreatedAt = now
	geo.UpdatedAt = now

	r.store.Lock()
	r.store.Geo()[geo.ID] = geo
	r.store.Unlock()

	// 在锁外发布事件
	r.store.PublishEvent(events.GeoEvent{
		EventType: events.EventGeoCreated,
		GeoID:     geo.ID,
		Geo:       geo,
	})

	return geo, nil
}

// Update 更新 Geo 资源
func (r *GeoRepo) Update(ctx context.Context, id string, geo domain.GeoResource) (domain.GeoResource, error) {
	r.store.Lock()

	existing, ok := r.store.Geo()[id]
	if !ok {
		r.store.Unlock()
		return domain.GeoResource{}, repository.ErrGeoNotFound
	}

	// 保留不可变字段
	geo.ID = id
	geo.CreatedAt = existing.CreatedAt
	geo.UpdatedAt = time.Now()

	r.store.Geo()[id] = geo
	r.store.Unlock()

	// 在锁外发布事件
	r.store.PublishEvent(events.GeoEvent{
		EventType: events.EventGeoUpdated,
		GeoID:     id,
		Geo:       geo,
	})

	return geo, nil
}

// Delete 删除 Geo 资源
func (r *GeoRepo) Delete(ctx context.Context, id string) error {
	r.store.Lock()

	if _, ok := r.store.Geo()[id]; !ok {
		r.store.Unlock()
		return repository.ErrGeoNotFound
	}

	delete(r.store.Geo(), id)
	r.store.Unlock()

	// 在锁外发布事件
	r.store.PublishEvent(events.GeoEvent{
		EventType: events.EventGeoDeleted,
		GeoID:     id,
	})

	return nil
}

// Upsert 插入或更新 Geo 资源
func (r *GeoRepo) Upsert(ctx context.Context, geo domain.GeoResource) (domain.GeoResource, error) {
	now := time.Now()

	r.store.Lock()

	// 如果有 ID，尝试更新
	if geo.ID != "" {
		if existing, ok := r.store.Geo()[geo.ID]; ok {
			geo.CreatedAt = existing.CreatedAt
			geo.UpdatedAt = now
			r.store.Geo()[geo.ID] = geo
			r.store.Unlock()

			r.store.PublishEvent(events.GeoEvent{
				EventType: events.EventGeoUpdated,
				GeoID:     geo.ID,
				Geo:       geo,
			})
			return geo, nil
		}
	}

	// 按类型查找已存在的资源
	for id, existing := range r.store.Geo() {
		if existing.Type == geo.Type {
			geo.ID = id
			geo.CreatedAt = existing.CreatedAt
			geo.UpdatedAt = now
			r.store.Geo()[id] = geo
			r.store.Unlock()

			r.store.PublishEvent(events.GeoEvent{
				EventType: events.EventGeoUpdated,
				GeoID:     id,
				Geo:       geo,
			})
			return geo, nil
		}
	}

	// 创建新资源
	if geo.ID == "" {
		geo.ID = uuid.NewString()
	}
	geo.CreatedAt = now
	geo.UpdatedAt = now
	r.store.Geo()[geo.ID] = geo
	r.store.Unlock()

	r.store.PublishEvent(events.GeoEvent{
		EventType: events.EventGeoCreated,
		GeoID:     geo.ID,
		Geo:       geo,
	})

	return geo, nil
}

// GetByType 按类型查询 Geo 资源
func (r *GeoRepo) GetByType(ctx context.Context, geoType domain.GeoResourceType) (domain.GeoResource, error) {
	r.store.RLock()
	defer r.store.RUnlock()

	for _, geo := range r.store.Geo() {
		if geo.Type == geoType {
			return geo, nil
		}
	}

	return domain.GeoResource{}, repository.ErrGeoNotFound
}

// 确保实现接口
var _ repository.GeoRepository = (*GeoRepo)(nil)
