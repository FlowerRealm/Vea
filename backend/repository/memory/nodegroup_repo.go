package memory

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"vea/backend/domain"
	"vea/backend/repository"
	"vea/backend/repository/events"
)

// NodeGroupRepo NodeGroup 仓储实现（内存）
type NodeGroupRepo struct {
	store *Store
}

func NewNodeGroupRepo(store *Store) *NodeGroupRepo {
	return &NodeGroupRepo{store: store}
}

func (r *NodeGroupRepo) Get(_ context.Context, id string) (domain.NodeGroup, error) {
	r.store.RLock()
	defer r.store.RUnlock()
	group, ok := r.store.NodeGroups()[id]
	if !ok {
		return domain.NodeGroup{}, repository.ErrNodeGroupNotFound
	}
	return group, nil
}

func (r *NodeGroupRepo) List(_ context.Context) ([]domain.NodeGroup, error) {
	r.store.RLock()
	defer r.store.RUnlock()
	items := make([]domain.NodeGroup, 0, len(r.store.NodeGroups()))
	for _, g := range r.store.NodeGroups() {
		items = append(items, g)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].CreatedAt.Before(items[j].CreatedAt)
		}
		return items[i].Name < items[j].Name
	})
	return items, nil
}

func (r *NodeGroupRepo) Create(_ context.Context, group domain.NodeGroup) (domain.NodeGroup, error) {
	now := time.Now()
	r.store.Lock()
	if group.ID == "" {
		group.ID = uuid.NewString()
	}
	if group.CreatedAt.IsZero() {
		group.CreatedAt = now
	}
	group.UpdatedAt = now
	if group.NodeIDs == nil {
		group.NodeIDs = []string{}
	}
	group.Tags = normalizeTags(group.Tags)
	r.store.NodeGroups()[group.ID] = group
	r.store.Unlock()

	r.store.PublishEvent(events.NodeGroupEvent{
		EventType:   events.EventNodeGroupCreated,
		NodeGroupID: group.ID,
		NodeGroup:   group,
	})
	return group, nil
}

func (r *NodeGroupRepo) Update(_ context.Context, id string, group domain.NodeGroup) (domain.NodeGroup, error) {
	r.store.Lock()
	current, ok := r.store.NodeGroups()[id]
	if !ok {
		r.store.Unlock()
		return domain.NodeGroup{}, repository.ErrNodeGroupNotFound
	}

	group.ID = id
	group.CreatedAt = current.CreatedAt
	group.UpdatedAt = time.Now()
	if group.NodeIDs == nil {
		group.NodeIDs = []string{}
	}
	group.Tags = normalizeTags(group.Tags)
	r.store.NodeGroups()[id] = group
	r.store.Unlock()

	r.store.PublishEvent(events.NodeGroupEvent{
		EventType:   events.EventNodeGroupUpdated,
		NodeGroupID: id,
		NodeGroup:   group,
	})
	return group, nil
}

func (r *NodeGroupRepo) Delete(_ context.Context, id string) error {
	r.store.Lock()
	current, ok := r.store.NodeGroups()[id]
	if !ok {
		r.store.Unlock()
		return repository.ErrNodeGroupNotFound
	}
	delete(r.store.NodeGroups(), id)
	r.store.Unlock()

	r.store.PublishEvent(events.NodeGroupEvent{
		EventType:   events.EventNodeGroupDeleted,
		NodeGroupID: id,
		NodeGroup:   current,
	})
	return nil
}

func normalizeTags(tags []string) []string {
	if tags == nil {
		return nil
	}
	out := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}
