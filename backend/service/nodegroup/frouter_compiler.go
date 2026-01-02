package nodegroup

import (
	"fmt"
	"sort"
	"strings"

	"vea/backend/domain"
)

type ActionKind string

const (
	ActionNode   ActionKind = "node"
	ActionDirect ActionKind = "direct"
	ActionBlock  ActionKind = "block"
)

type Action struct {
	Kind   ActionKind
	NodeID string
}

func (a Action) String() string {
	switch a.Kind {
	case ActionNode:
		return "node:" + a.NodeID
	case ActionDirect, ActionBlock:
		return string(a.Kind)
	default:
		return ""
	}
}

type RouteRule struct {
	EdgeID   string
	Priority int
	Match    domain.RouteMatchRule
	Action   Action
}

type CompiledFRouter struct {
	Rules          []RouteRule
	Default        Action
	DetourUpstream map[string]string
	Warnings       []string
}

type CompileError struct {
	Problems []string
}

func (e *CompileError) Error() string {
	if e == nil || len(e.Problems) == 0 {
		return "compile error"
	}
	return strings.Join(e.Problems, "; ")
}

func CompileFRouter(frouter domain.FRouter, nodes []domain.Node) (CompiledFRouter, error) {
	compiled := CompiledFRouter{
		DetourUpstream: make(map[string]string),
	}

	problems := make([]string, 0)
	warnings := make([]string, 0)

	nodesByID := make(map[string]domain.Node, len(nodes))
	for _, n := range nodes {
		id := strings.TrimSpace(n.ID)
		if id == "" {
			problems = append(problems, "node has empty id")
			continue
		}
		if _, ok := nodesByID[id]; ok {
			problems = append(problems, fmt.Sprintf("duplicate node id: %s", id))
			continue
		}
		nodesByID[id] = n
	}

	slotBindings := buildSlotBindingMap(frouter.ChainProxy.Slots, nodesByID, &problems)

	var rules []RouteRule
	var defaultAction Action
	var hasDefault bool

	addDetour := func(fromNode string, toNode string, sourceEdgeID string) {
		if strings.TrimSpace(fromNode) == "" || strings.TrimSpace(toNode) == "" {
			problems = append(problems, fmt.Sprintf("edge %s: detour has empty node id", sourceEdgeID))
			return
		}
		if fromNode == toNode {
			problems = append(problems, fmt.Sprintf("edge %s: detour is self-loop: %s", sourceEdgeID, fromNode))
			return
		}
		if _, ok := nodesByID[fromNode]; !ok {
			problems = append(problems, fmt.Sprintf("edge %s: detour from node not found: %s", sourceEdgeID, fromNode))
			return
		}
		if _, ok := nodesByID[toNode]; !ok {
			problems = append(problems, fmt.Sprintf("edge %s: detour to node not found: %s", sourceEdgeID, toNode))
			return
		}

		if existing, ok := compiled.DetourUpstream[fromNode]; ok && strings.TrimSpace(existing) != "" && existing != toNode {
			problems = append(problems, fmt.Sprintf("node %s has multiple detour upstreams: %s and %s", fromNode, existing, toNode))
			return
		}
		compiled.DetourUpstream[fromNode] = toNode
	}

	for _, edge := range frouter.ChainProxy.Edges {
		if !edge.Enabled {
			continue
		}

		from, skip, warning := resolveSlot(edge.From, slotBindings)
		if skip {
			warnings = append(warnings, fmt.Sprintf("edge %s skipped: %s", edge.ID, warning))
			continue
		}
		to, skip, warning := resolveSlot(edge.To, slotBindings)
		if skip {
			warnings = append(warnings, fmt.Sprintf("edge %s skipped: %s", edge.ID, warning))
			continue
		}

		if strings.TrimSpace(from) == "" {
			problems = append(problems, fmt.Sprintf("edge %s has empty from", edge.ID))
			continue
		}
		if strings.TrimSpace(to) == "" {
			problems = append(problems, fmt.Sprintf("edge %s has empty to", edge.ID))
			continue
		}
		if from == to {
			problems = append(problems, fmt.Sprintf("edge %s is self-loop: %s", edge.ID, from))
			continue
		}

		if from == domain.EdgeNodeLocal {
			action, ok := actionFromTo(to, nodesByID)
			if !ok {
				problems = append(problems, fmt.Sprintf("edge %s: invalid local->%s", edge.ID, to))
				continue
			}
			if action.Kind != ActionNode && len(edge.Via) > 0 {
				problems = append(problems, fmt.Sprintf("edge %s: via is only allowed when local->node", edge.ID))
				continue
			}

			if action.Kind == ActionNode && len(edge.Via) > 0 {
				chain := make([]string, 0, len(edge.Via)+1)
				chain = append(chain, action.NodeID)
				for _, hop := range edge.Via {
					hop = strings.TrimSpace(hop)
					if hop == "" {
						continue
					}
					resolvedHop, skip, warning := resolveSlot(hop, slotBindings)
					if skip {
						warnings = append(warnings, fmt.Sprintf("edge %s: via %s skipped: %s", edge.ID, hop, warning))
						continue
					}
					if resolvedHop == domain.EdgeNodeDirect || resolvedHop == domain.EdgeNodeBlock || resolvedHop == domain.EdgeNodeLocal {
						problems = append(problems, fmt.Sprintf("edge %s: via contains invalid node %s", edge.ID, resolvedHop))
						continue
					}
					chain = append(chain, resolvedHop)
				}
				for i := 0; i+1 < len(chain); i++ {
					addDetour(chain[i], chain[i+1], edge.ID)
				}
			}

			isDefault, err := validateAndIsDefaultSelectionEdge(edge)
			if err != nil {
				problems = append(problems, fmt.Sprintf("edge %s: %s", edge.ID, err.Error()))
				continue
			}

			if isDefault {
				if hasDefault {
					problems = append(problems, fmt.Sprintf("multiple default edges: %s and %s", describeAction(defaultAction), describeAction(action)))
					continue
				}
				hasDefault = true
				defaultAction = action
				if edge.Priority != 0 {
					warnings = append(warnings, fmt.Sprintf("edge %s is default; priority forced to 0", edge.ID))
				}
				continue
			}

			match := domain.RouteMatchRule{}
			if edge.RouteRule != nil {
				match = *edge.RouteRule
			}

			rules = append(rules, RouteRule{
				EdgeID:   edge.ID,
				Priority: edge.Priority,
				Match:    match,
				Action:   action,
			})
			continue
		}

		if from == domain.EdgeNodeDirect || from == domain.EdgeNodeBlock {
			problems = append(problems, fmt.Sprintf("edge %s: detour from %s is not allowed", edge.ID, from))
			continue
		}
		if to == domain.EdgeNodeLocal {
			problems = append(problems, fmt.Sprintf("edge %s: detour to %s is not allowed", edge.ID, to))
			continue
		}
		// slot → direct/block: 有效边但无 detour 效果，静默跳过
		if to == domain.EdgeNodeDirect || to == domain.EdgeNodeBlock {
			if !strings.HasPrefix(edge.From, domain.EdgeNodeSlotPrefix) {
				problems = append(problems, fmt.Sprintf("edge %s: detour to %s is not allowed", edge.ID, to))
			}
			continue
		}

		if err := validateDetourEdge(edge); err != nil {
			problems = append(problems, fmt.Sprintf("edge %s: %s", edge.ID, err.Error()))
			continue
		}

		addDetour(from, to, edge.ID)
	}

	if !hasDefault {
		problems = append(problems, "missing default edge: require exactly one local -> {node|direct|block} without match (or routeRule empty)")
	}

	for _, rule := range rules {
		if err := validateRouteMatchRule(rule.Match); err != nil {
			problems = append(problems, fmt.Sprintf("edge %s: %s", rule.EdgeID, err.Error()))
		}
	}

	detectDetourCycles(compiled.DetourUpstream, &problems)

	if len(problems) > 0 {
		return CompiledFRouter{}, &CompileError{Problems: problems}
	}

	sort.Slice(rules, func(i, j int) bool {
		if rules[i].Priority != rules[j].Priority {
			return rules[i].Priority > rules[j].Priority
		}
		return rules[i].EdgeID < rules[j].EdgeID
	})

	compiled.Rules = rules
	compiled.Default = defaultAction
	compiled.Warnings = warnings
	return compiled, nil
}

// IsDefaultSelectionEdge reports whether the edge is a "match-all" selection edge from `local`.
// It is treated as the single required default edge (priority will be forced to 0 on save).
func IsDefaultSelectionEdge(edge domain.ProxyEdge) (bool, error) {
	if edge.From != domain.EdgeNodeLocal {
		return false, nil
	}
	return validateAndIsDefaultSelectionEdge(edge)
}

func describeAction(action Action) string {
	switch action.Kind {
	case ActionNode:
		return "node:" + action.NodeID
	case ActionDirect:
		return "direct"
	case ActionBlock:
		return "block"
	default:
		return "unknown"
	}
}

func actionFromTo(to string, nodesByID map[string]domain.Node) (Action, bool) {
	switch to {
	case domain.EdgeNodeDirect:
		return Action{Kind: ActionDirect}, true
	case domain.EdgeNodeBlock:
		return Action{Kind: ActionBlock}, true
	default:
		if _, ok := nodesByID[to]; ok {
			return Action{Kind: ActionNode, NodeID: to}, true
		}
		return Action{}, false
	}
}

func validateAndIsDefaultSelectionEdge(edge domain.ProxyEdge) (bool, error) {
	if (edge.To == domain.EdgeNodeDirect || edge.To == domain.EdgeNodeBlock) && len(edge.Via) > 0 {
		return false, fmt.Errorf("via is not allowed when to=%s", edge.To)
	}

	switch edge.RuleType {
	case domain.EdgeRuleNone:
		if edge.RouteRule != nil {
			return false, fmt.Errorf("ruleType=empty must not carry routeRule")
		}
		return true, nil
	case domain.EdgeRuleRoute:
		if edge.RouteRule == nil {
			return true, nil
		}
		if len(edge.RouteRule.Domains) == 0 && len(edge.RouteRule.IPs) == 0 {
			return true, nil
		}
		return false, nil
	default:
		return false, fmt.Errorf("unsupported ruleType: %s", edge.RuleType)
	}
}

func validateDetourEdge(edge domain.ProxyEdge) error {
	if edge.RuleType != "" {
		return fmt.Errorf("detour edge must not have ruleType")
	}
	if edge.RouteRule != nil {
		return fmt.Errorf("detour edge must not have routeRule")
	}
	if len(edge.Via) > 0 {
		return fmt.Errorf("detour edge must not have via")
	}
	return nil
}

func validateRouteMatchRule(rule domain.RouteMatchRule) error {
	if len(rule.Domains) == 0 && len(rule.IPs) == 0 {
		return fmt.Errorf("empty routeRule is not allowed on non-default edge")
	}
	return nil
}

type slotBindingMap map[string]string

func buildSlotBindingMap(slots []domain.SlotNode, nodesByID map[string]domain.Node, problems *[]string) slotBindingMap {
	m := make(slotBindingMap)
	for _, slot := range slots {
		id := strings.TrimSpace(slot.ID)
		if id == "" {
			*problems = append(*problems, "slot has empty id")
			continue
		}
		if !domain.IsSlotNode(id) {
			*problems = append(*problems, fmt.Sprintf("slot %s: id must start with %s", id, domain.EdgeNodeSlotPrefix))
			continue
		}
		if _, ok := m[id]; ok {
			*problems = append(*problems, fmt.Sprintf("slot %s: duplicated", id))
			continue
		}
		bound := strings.TrimSpace(slot.BoundNodeID)
		if bound != "" {
			if _, ok := nodesByID[bound]; !ok {
				*problems = append(*problems, fmt.Sprintf("slot %s: boundNodeId not found: %s", id, bound))
			}
		}
		m[id] = bound
	}
	return m
}

func resolveSlot(id string, slots slotBindingMap) (resolved string, skip bool, warning string) {
	if !domain.IsSlotNode(id) {
		return id, false, ""
	}
	bound, ok := slots[id]
	if ok && bound != "" {
		return bound, false, ""
	}
	return "", true, fmt.Sprintf("slot %s is unbound (passthrough)", id)
}

func detectDetourCycles(detours map[string]string, problems *[]string) {
	visitState := make(map[string]int, len(detours)) // 0=unvisited 1=visiting 2=done
	stack := make([]string, 0, 16)
	stackIndex := make(map[string]int, 16)

	var visit func(node string)
	visit = func(node string) {
		if visitState[node] == 2 {
			return
		}
		if visitState[node] == 1 {
			start := stackIndex[node]
			cycle := append([]string(nil), stack[start:]...)
			cycle = append(cycle, node)
			*problems = append(*problems, "detour cycle: "+strings.Join(cycle, " -> "))
			return
		}
		visitState[node] = 1
		stackIndex[node] = len(stack)
		stack = append(stack, node)

		if upstream := detours[node]; upstream != "" {
			visit(upstream)
		}

		stack = stack[:len(stack)-1]
		delete(stackIndex, node)
		visitState[node] = 2
	}

	for node := range detours {
		visit(node)
	}
}
