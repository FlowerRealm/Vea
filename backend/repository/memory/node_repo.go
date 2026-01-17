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

// NodeRepo Node 仓储实现（内存）
type NodeRepo struct {
	store *Store
}

func NewNodeRepo(store *Store) *NodeRepo {
	return &NodeRepo{store: store}
}

func (r *NodeRepo) Get(_ context.Context, id string) (domain.Node, error) {
	r.store.RLock()
	defer r.store.RUnlock()
	node, ok := r.store.Nodes()[id]
	if !ok {
		return domain.Node{}, repository.ErrNodeNotFound
	}
	return node, nil
}

func (r *NodeRepo) List(_ context.Context) ([]domain.Node, error) {
	r.store.RLock()
	defer r.store.RUnlock()
	items := make([]domain.Node, 0, len(r.store.Nodes()))
	for _, node := range r.store.Nodes() {
		items = append(items, node)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].CreatedAt.Before(items[j].CreatedAt)
		}
		return items[i].Name < items[j].Name
	})
	return items, nil
}

func (r *NodeRepo) Create(_ context.Context, node domain.Node) (domain.Node, error) {
	now := time.Now()
	r.store.Lock()
	if node.ID == "" {
		node.ID = uuid.NewString()
	}
	if node.CreatedAt.IsZero() {
		node.CreatedAt = now
	}
	node.UpdatedAt = now
	r.store.Nodes()[node.ID] = node
	r.store.Unlock()

	r.store.PublishEvent(events.NodeEvent{
		EventType: events.EventNodeCreated,
		NodeID:    node.ID,
		Node:      node,
	})
	return node, nil
}

func (r *NodeRepo) Update(_ context.Context, id string, node domain.Node) (domain.Node, error) {
	r.store.Lock()
	current, ok := r.store.Nodes()[id]
	if !ok {
		r.store.Unlock()
		return domain.Node{}, repository.ErrNodeNotFound
	}
	node.ID = id
	node.CreatedAt = current.CreatedAt
	node.UpdatedAt = time.Now()
	// 保留运行期指标（同 ID 代表同一节点）
	node.LastLatencyMS = current.LastLatencyMS
	node.LastLatencyAt = current.LastLatencyAt
	node.LastLatencyError = current.LastLatencyError
	node.LastSpeedMbps = current.LastSpeedMbps
	node.LastSpeedAt = current.LastSpeedAt
	node.LastSpeedError = current.LastSpeedError

	r.store.Nodes()[id] = node
	r.store.Unlock()

	r.store.PublishEvent(events.NodeEvent{
		EventType: events.EventNodeUpdated,
		NodeID:    id,
		Node:      node,
	})
	return node, nil
}

func (r *NodeRepo) Delete(_ context.Context, id string) error {
	r.store.Lock()
	current, ok := r.store.Nodes()[id]
	if !ok {
		r.store.Unlock()
		return repository.ErrNodeNotFound
	}
	delete(r.store.Nodes(), id)
	r.store.Unlock()

	r.store.PublishEvent(events.NodeEvent{
		EventType: events.EventNodeDeleted,
		NodeID:    id,
		Node:      current,
	})
	return nil
}

func (r *NodeRepo) ListByConfigID(_ context.Context, configID string) ([]domain.Node, error) {
	r.store.RLock()
	defer r.store.RUnlock()
	items := make([]domain.Node, 0)
	for _, node := range r.store.Nodes() {
		if node.SourceConfigID == configID {
			items = append(items, node)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].CreatedAt.Before(items[j].CreatedAt)
		}
		return items[i].Name < items[j].Name
	})
	return items, nil
}

func (r *NodeRepo) ReplaceNodesForConfig(_ context.Context, configID string, nodes []domain.Node) ([]domain.Node, error) {
	now := time.Now()
	next := make([]domain.Node, 0, len(nodes))
	nextIDs := make(map[string]struct{}, len(nodes))
	for _, node := range nodes {
		node.SourceConfigID = configID
		if strings.TrimSpace(node.Name) == "" && strings.TrimSpace(node.Address) != "" {
			node.Name = node.Address
		}
		if node.ID == "" {
			node.ID = domain.StableNodeIDForConfig(configID, node)
		}
		if node.CreatedAt.IsZero() {
			node.CreatedAt = now
		}
		node.UpdatedAt = now
		next = append(next, node)
		nextIDs[node.ID] = struct{}{}
	}

	eventsToPublish := make([]events.Event, 0, len(next)+8)

	r.store.Lock()
	// ReplaceNodesForConfig 语义：对指定 config 的节点集合做“替换”
	// - nodes == nil（domain.ClearNodes）: 显式清空该 config 的全部节点
	// - len(nodes) > 0: 以入参 nodes 作为最新快照，删除不在快照内的历史节点（避免节点越积越多）
	// - len(nodes) == 0: 不做删除（保持历史节点），用于调用方表达“本次不更新节点集合”
	for id, existing := range r.store.Nodes() {
		if existing.SourceConfigID != configID {
			continue
		}
		if nodes == nil {
			delete(r.store.Nodes(), id)
			eventsToPublish = append(eventsToPublish, events.NodeEvent{
				EventType: events.EventNodeDeleted,
				NodeID:    id,
				Node:      existing,
			})
			continue
		}
		if len(nodes) > 0 {
			if _, ok := nextIDs[id]; ok {
				continue
			}
			delete(r.store.Nodes(), id)
			eventsToPublish = append(eventsToPublish, events.NodeEvent{
				EventType: events.EventNodeDeleted,
				NodeID:    id,
				Node:      existing,
			})
		}
	}

	// Upsert 节点集合
	for i := range next {
		node := next[i]
		if existing, ok := r.store.Nodes()[node.ID]; ok {
			node.CreatedAt = existing.CreatedAt
			node.LastLatencyMS = existing.LastLatencyMS
			node.LastLatencyAt = existing.LastLatencyAt
			node.LastLatencyError = existing.LastLatencyError
			node.LastSpeedMbps = existing.LastSpeedMbps
			node.LastSpeedAt = existing.LastSpeedAt
			node.LastSpeedError = existing.LastSpeedError
			if strings.TrimSpace(existing.Name) != "" {
				node.Name = existing.Name
			}
			if existing.Tags != nil {
				node.Tags = existing.Tags
			}
			next[i] = node
			r.store.Nodes()[node.ID] = node
			eventsToPublish = append(eventsToPublish, events.NodeEvent{
				EventType: events.EventNodeUpdated,
				NodeID:    node.ID,
				Node:      node,
			})
			continue
		}
		r.store.Nodes()[node.ID] = node
		eventsToPublish = append(eventsToPublish, events.NodeEvent{
			EventType: events.EventNodeCreated,
			NodeID:    node.ID,
			Node:      node,
		})
	}
	r.store.Unlock()

	for _, event := range eventsToPublish {
		r.store.PublishEvent(event)
	}

	// 返回按名称排序后的结果（对前端更友好）
	sort.Slice(next, func(i, j int) bool {
		if next[i].Name == next[j].Name {
			return next[i].CreatedAt.Before(next[j].CreatedAt)
		}
		return next[i].Name < next[j].Name
	})
	return next, nil
}

func (r *NodeRepo) UpdateLatency(_ context.Context, id string, latencyMS int64, latencyErr string) error {
	r.store.Lock()
	node, ok := r.store.Nodes()[id]
	if !ok {
		r.store.Unlock()
		return repository.ErrNodeNotFound
	}
	now := time.Now()
	node.LastLatencyMS = latencyMS
	node.LastLatencyAt = now
	node.LastLatencyError = latencyErr
	r.store.Nodes()[id] = node
	r.store.Unlock()
	return nil
}

func (r *NodeRepo) UpdateSpeed(_ context.Context, id string, speedMbps float64, speedErr string) error {
	r.store.Lock()
	node, ok := r.store.Nodes()[id]
	if !ok {
		r.store.Unlock()
		return repository.ErrNodeNotFound
	}
	now := time.Now()
	node.LastSpeedMbps = speedMbps
	node.LastSpeedAt = now
	node.LastSpeedError = speedErr
	r.store.Nodes()[id] = node
	r.store.Unlock()
	return nil
}
