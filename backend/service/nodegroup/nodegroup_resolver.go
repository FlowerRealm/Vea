package nodegroup

import (
	"fmt"
	"strings"

	"vea/backend/domain"
	"vea/backend/repository"
)

type ResolveError struct {
	Problems []string
}

func (e *ResolveError) Error() string {
	if e == nil || len(e.Problems) == 0 {
		return "resolve error"
	}
	return strings.Join(e.Problems, "; ")
}

func (e *ResolveError) Unwrap() error {
	return repository.ErrInvalidData
}

type ResolveOptions struct {
	// AdvanceCursor controls whether resolver should update node group cursor.
	// It should be enabled for real "execution" paths (start proxy / measurement),
	// and disabled for preview/validate paths.
	AdvanceCursor bool

	// UpdateCursor persists cursor changes when AdvanceCursor=true.
	// If nil, cursor updates will be skipped.
	UpdateCursor func(groupID string, cursor int) error

	// AllowFailoverFallback allows failover strategy to fall back to the cursor
	// target when all members are currently unhealthy. This is useful for
	// validation/preview paths where runtime probe state should not block writes.
	AllowFailoverFallback bool
}

func ResolveFRouterNodeGroups(frouter domain.FRouter, nodes []domain.Node, groups []domain.NodeGroup, opts ResolveOptions) (domain.FRouter, error) {
	nodesByID := make(map[string]domain.Node, len(nodes))
	for _, n := range nodes {
		id := strings.TrimSpace(n.ID)
		if id == "" {
			continue
		}
		nodesByID[id] = n
	}

	groupsByID := make(map[string]domain.NodeGroup, len(groups))
	for _, g := range groups {
		id := strings.TrimSpace(g.ID)
		if id == "" {
			continue
		}
		groupsByID[id] = g
	}

	problems := make([]string, 0)

	resolveEndpoint := func(raw string) (string, error) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return "", nil
		}
		if raw == domain.EdgeNodeLocal || raw == domain.EdgeNodeDirect || raw == domain.EdgeNodeBlock {
			return raw, nil
		}
		if domain.IsSlotNode(raw) {
			return raw, nil
		}

		if _, ok := nodesByID[raw]; ok {
			return raw, nil
		}
		g, ok := groupsByID[raw]
		if !ok {
			return "", fmt.Errorf("unknown node or node group id: %s", raw)
		}

		selected, nextCursor, err := selectNodeFromGroup(g, nodesByID, opts)
		if err != nil {
			return "", err
		}
		if selected == "" {
			return "", fmt.Errorf("node group %s resolved to empty node id", g.ID)
		}

		if opts.AdvanceCursor && opts.UpdateCursor != nil && nextCursor != nil {
			if err := opts.UpdateCursor(g.ID, *nextCursor); err != nil {
				return "", fmt.Errorf("update node group cursor %s: %w", g.ID, err)
			}
		}

		return selected, nil
	}

	next := frouter
	nextEdges := make([]domain.ProxyEdge, 0, len(frouter.ChainProxy.Edges))
	for _, edge := range frouter.ChainProxy.Edges {
		e := edge
		from, err := resolveEndpoint(e.From)
		if err != nil {
			problems = append(problems, fmt.Sprintf("edge %s from: %v", edge.ID, err))
		} else {
			e.From = from
		}

		to, err := resolveEndpoint(e.To)
		if err != nil {
			problems = append(problems, fmt.Sprintf("edge %s to: %v", edge.ID, err))
		} else {
			e.To = to
		}

		if len(e.Via) > 0 {
			nextVia := make([]string, 0, len(e.Via))
			for _, hop := range e.Via {
				resolved, err := resolveEndpoint(hop)
				if err != nil {
					problems = append(problems, fmt.Sprintf("edge %s via: %v", edge.ID, err))
					continue
				}
				if strings.TrimSpace(resolved) == "" {
					continue
				}
				nextVia = append(nextVia, resolved)
			}
			e.Via = nextVia
		}

		nextEdges = append(nextEdges, e)
	}
	next.ChainProxy.Edges = nextEdges

	if len(frouter.ChainProxy.Slots) > 0 {
		nextSlots := make([]domain.SlotNode, 0, len(frouter.ChainProxy.Slots))
		for _, slot := range frouter.ChainProxy.Slots {
			s := slot
			if strings.TrimSpace(s.BoundNodeID) != "" {
				resolved, err := resolveEndpoint(s.BoundNodeID)
				if err != nil {
					problems = append(problems, fmt.Sprintf("slot %s boundNodeId: %v", slot.ID, err))
				} else {
					s.BoundNodeID = resolved
				}
			}
			nextSlots = append(nextSlots, s)
		}
		next.ChainProxy.Slots = nextSlots
	}

	if len(problems) > 0 {
		return domain.FRouter{}, &ResolveError{Problems: problems}
	}
	return next, nil
}

func selectNodeFromGroup(group domain.NodeGroup, nodesByID map[string]domain.Node, opts ResolveOptions) (selected string, nextCursor *int, err error) {
	if strings.TrimSpace(group.ID) == "" {
		return "", nil, fmt.Errorf("node group has empty id")
	}

	ordered := make([]string, 0, len(group.NodeIDs))
	seen := make(map[string]struct{}, len(group.NodeIDs))
	for _, id := range group.NodeIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		if _, ok := nodesByID[id]; !ok {
			continue
		}
		ordered = append(ordered, id)
	}
	if len(ordered) == 0 {
		return "", nil, fmt.Errorf("node group %s has no available nodes", group.ID)
	}

	switch strings.TrimSpace(string(group.Strategy)) {
	case string(domain.NodeGroupStrategyLowestLatency):
		bestID := ""
		bestMS := int64(0)
		for _, id := range ordered {
			n := nodesByID[id]
			if strings.TrimSpace(n.LastLatencyError) != "" {
				continue
			}
			if n.LastLatencyMS <= 0 {
				continue
			}
			if bestID == "" || n.LastLatencyMS < bestMS {
				bestID = id
				bestMS = n.LastLatencyMS
			}
		}
		if bestID != "" {
			return bestID, nil, nil
		}
		return ordered[0], nil, nil

	case string(domain.NodeGroupStrategyFastestSpeed):
		bestID := ""
		bestMbps := float64(0)
		for _, id := range ordered {
			n := nodesByID[id]
			if strings.TrimSpace(n.LastSpeedError) != "" {
				continue
			}
			if n.LastSpeedMbps <= 0 {
				continue
			}
			if bestID == "" || n.LastSpeedMbps > bestMbps {
				bestID = id
				bestMbps = n.LastSpeedMbps
			}
		}
		if bestID != "" {
			return bestID, nil, nil
		}
		return ordered[0], nil, nil

	case string(domain.NodeGroupStrategyRoundRobin):
		idx := normalizeCursor(group.Cursor, len(ordered))
		chosen := ordered[idx]
		next := idx + 1
		if next >= len(ordered) {
			next = 0
		}
		return chosen, &next, nil

	case string(domain.NodeGroupStrategyFailover):
		start := normalizeCursor(group.Cursor, len(ordered))
		if isNodeAvailableForFailover(nodesByID[ordered[start]]) {
			return ordered[start], nil, nil
		}

		for i := 1; i < len(ordered); i++ {
			idx := start + i
			if idx >= len(ordered) {
				idx = idx % len(ordered)
			}
			if isNodeAvailableForFailover(nodesByID[ordered[idx]]) {
				next := idx
				return ordered[idx], &next, nil
			}
		}
		if opts.AllowFailoverFallback {
			return ordered[start], nil, nil
		}
		return "", nil, fmt.Errorf("node group %s has no available nodes", group.ID)

	case "":
		return "", nil, fmt.Errorf("node group %s missing strategy", group.ID)
	default:
		return "", nil, fmt.Errorf("node group %s has invalid strategy: %s", group.ID, group.Strategy)
	}
}

func normalizeCursor(cursor int, length int) int {
	if length <= 0 {
		return 0
	}
	if cursor < 0 {
		return 0
	}
	return cursor % length
}

func isNodeAvailableForFailover(node domain.Node) bool {
	// "连不上都算失败"：依赖现有延迟测试逻辑，失败会写入 lastLatencyError。
	return strings.TrimSpace(node.LastLatencyError) == ""
}
