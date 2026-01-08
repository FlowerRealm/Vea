package memory

import (
	"context"
	"encoding/json"
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

func stableNodeIDForConfig(configID string, node domain.Node) string {
	type fingerprint struct {
		Protocol  domain.NodeProtocol   `json:"protocol"`
		Address   string                `json:"address"`
		Port      int                   `json:"port"`
		Security  *domain.NodeSecurity  `json:"security,omitempty"`
		Transport *domain.NodeTransport `json:"transport,omitempty"`
		TLS       *domain.NodeTLS       `json:"tls,omitempty"`
	}
	b, _ := json.Marshal(fingerprint{
		Protocol:  node.Protocol,
		Address:   node.Address,
		Port:      node.Port,
		Security:  node.Security,
		Transport: node.Transport,
		TLS:       node.TLS,
	})
	return uuid.NewSHA1(uuid.NameSpaceOID, append([]byte(configID+"|"), b...)).String()
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
			node.ID = stableNodeIDForConfig(configID, node)
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
	// 仅在显式清空（nodes == nil）时删除节点。
	// 调用方如需表达“显式清空”，建议传入 domain.ClearNodes（nil slice sentinel）。
	// 注意：nodes 为空切片（len == 0）时，不会删除已有节点（仍保留历史节点）。
	//
	// 订阅/拉取节点存在波动（服务端短暂缺失、返回不完整等）。
	// 如果每次拉取都按“差集删除”，会导致用户原有节点丢失，并进一步把引用这些节点的 FRouter 变成 invalid。
	// 由于前端目前没有 Node 删除入口，这里采取更保守的策略：保留历史节点，避免数据丢失。
	if nodes == nil {
		for id, existing := range r.store.Nodes() {
			if existing.SourceConfigID != configID {
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
