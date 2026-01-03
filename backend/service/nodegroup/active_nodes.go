package nodegroup

import (
	"strings"

	"vea/backend/domain"
)

// ActiveNodeIDs 返回本次编译产物实际会用到的节点集合：
// - 来自 default/rules 选择到的节点
// - 加上 detour 上游链路闭包（A detour->B，则 A 被用到时 B 也必须存在）
func ActiveNodeIDs(compiled CompiledFRouter) map[string]struct{} {
	active := make(map[string]struct{})
	queue := make([]string, 0, 8)

	enqueue := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, ok := active[id]; ok {
			return
		}
		active[id] = struct{}{}
		queue = append(queue, id)
	}

	if compiled.Default.Kind == ActionNode {
		enqueue(compiled.Default.NodeID)
	}
	for _, r := range compiled.Rules {
		if r.Action.Kind == ActionNode {
			enqueue(r.Action.NodeID)
		}
	}

	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]

		if upstream := strings.TrimSpace(compiled.DetourUpstream[id]); upstream != "" {
			enqueue(upstream)
		}
	}

	return active
}

func FilterNodesByID(nodes []domain.Node, ids map[string]struct{}) []domain.Node {
	if len(ids) == 0 || len(nodes) == 0 {
		return nil
	}
	filtered := make([]domain.Node, 0, len(ids))
	for _, n := range nodes {
		if _, ok := ids[n.ID]; ok {
			filtered = append(filtered, n)
		}
	}
	return filtered
}
