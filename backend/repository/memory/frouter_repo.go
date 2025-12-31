package memory

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"

	"vea/backend/domain"
	"vea/backend/repository"
	"vea/backend/repository/events"
)

// FRouterRepo FRouter 仓储实现（内存）
type FRouterRepo struct {
	store *Store
}

// NewFRouterRepo 创建 FRouterRepo
func NewFRouterRepo(store *Store) *FRouterRepo {
	return &FRouterRepo{store: store}
}

// Get 获取 FRouter
func (r *FRouterRepo) Get(_ context.Context, id string) (domain.FRouter, error) {
	r.store.RLock()
	defer r.store.RUnlock()
	frouter, ok := r.store.FRouters()[id]
	if !ok {
		return domain.FRouter{}, repository.ErrFRouterNotFound
	}
	return frouter, nil
}

// List 列出 FRouter
func (r *FRouterRepo) List(_ context.Context) ([]domain.FRouter, error) {
	r.store.RLock()
	defer r.store.RUnlock()
	items := make([]domain.FRouter, 0, len(r.store.FRouters()))
	for _, frouter := range r.store.FRouters() {
		items = append(items, frouter)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].CreatedAt.Before(items[j].CreatedAt)
		}
		return items[i].Name < items[j].Name
	})
	return items, nil
}

// Create 创建 FRouter
func (r *FRouterRepo) Create(_ context.Context, frouter domain.FRouter) (domain.FRouter, error) {
	now := time.Now()
	r.store.Lock()
	if frouter.ID == "" {
		frouter.ID = uuid.NewString()
	}
	if frouter.CreatedAt.IsZero() {
		frouter.CreatedAt = now
	}
	frouter.UpdatedAt = now
	frouter = normalizeFRouter(frouter, now)

	r.store.FRouters()[frouter.ID] = frouter
	r.store.Unlock()

	r.store.PublishEvent(events.FRouterEvent{
		EventType: events.EventFRouterCreated,
		FRouterID: frouter.ID,
		FRouter:   frouter,
	})
	return frouter, nil
}

// Update 更新 FRouter
func (r *FRouterRepo) Update(_ context.Context, id string, frouter domain.FRouter) (domain.FRouter, error) {
	r.store.Lock()
	current, ok := r.store.FRouters()[id]
	if !ok {
		r.store.Unlock()
		return domain.FRouter{}, repository.ErrFRouterNotFound
	}
	frouter.ID = id
	frouter.CreatedAt = current.CreatedAt
	frouter.UpdatedAt = time.Now()
	frouter = normalizeFRouter(frouter, frouter.UpdatedAt)

	r.store.FRouters()[id] = frouter
	r.store.Unlock()

	r.store.PublishEvent(events.FRouterEvent{
		EventType: events.EventFRouterUpdated,
		FRouterID: id,
		FRouter:   frouter,
	})
	return frouter, nil
}

// Delete 删除 FRouter
func (r *FRouterRepo) Delete(_ context.Context, id string) error {
	r.store.Lock()
	current, ok := r.store.FRouters()[id]
	if !ok {
		r.store.Unlock()
		return repository.ErrFRouterNotFound
	}
	delete(r.store.FRouters(), id)
	r.store.Unlock()

	r.store.PublishEvent(events.FRouterEvent{
		EventType: events.EventFRouterDeleted,
		FRouterID: id,
		FRouter:   current,
	})
	return nil
}

// UpdateLatency 更新延迟
func (r *FRouterRepo) UpdateLatency(_ context.Context, id string, latencyMS int64, latencyErr string) error {
	r.store.Lock()
	frouter, ok := r.store.FRouters()[id]
	if !ok {
		r.store.Unlock()
		return repository.ErrFRouterNotFound
	}
	now := time.Now()
	frouter.LastLatencyMS = latencyMS
	frouter.LastLatencyAt = now
	frouter.LastLatencyError = latencyErr
	r.store.FRouters()[id] = frouter
	r.store.Unlock()

	return nil
}

// UpdateSpeed 更新速度
func (r *FRouterRepo) UpdateSpeed(_ context.Context, id string, speedMbps float64, speedErr string) error {
	r.store.Lock()
	frouter, ok := r.store.FRouters()[id]
	if !ok {
		r.store.Unlock()
		return repository.ErrFRouterNotFound
	}
	now := time.Now()
	frouter.LastSpeedMbps = speedMbps
	frouter.LastSpeedAt = now
	frouter.LastSpeedError = speedErr
	r.store.FRouters()[id] = frouter
	r.store.Unlock()

	return nil
}

func normalizeFRouter(frouter domain.FRouter, now time.Time) domain.FRouter {
	if frouter.ChainProxy.Edges == nil {
		frouter.ChainProxy.Edges = []domain.ProxyEdge{}
	}
	if frouter.ChainProxy.Positions == nil {
		frouter.ChainProxy.Positions = make(map[string]domain.GraphPosition)
	}
	if frouter.ChainProxy.Slots == nil {
		frouter.ChainProxy.Slots = []domain.SlotNode{}
	}
	if frouter.ChainProxy.UpdatedAt.IsZero() {
		frouter.ChainProxy.UpdatedAt = now
	}
	return frouter
}
