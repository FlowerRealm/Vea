package config

import (
	"fmt"
	"sort"
	"strings"

	"vea/backend/domain"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

type clashProxyGroup struct {
	Name    string
	Proxies []string
}

type clashRule struct {
	Kind    string
	Value   string
	Target  string
	RawLine string
}

func parseClashYAMLRulesAndGroups(payload string) ([]clashProxyGroup, []clashRule, bool, []error) {
	text := strings.TrimSpace(payload)
	if text == "" {
		return nil, nil, false, nil
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal([]byte(text), &raw); err != nil {
		return nil, nil, false, nil
	}

	norm := normalizeTopLevelKeys(raw)
	rules := parseClashRules(norm)
	groups := parseClashProxyGroups(norm)
	if len(rules) == 0 && len(groups) == 0 {
		return nil, nil, false, nil
	}
	return groups, rules, true, nil
}

func normalizeTopLevelKeys(m map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[normalizeKey(k)] = v
	}
	return out
}

func normalizeKey(k string) string {
	k = strings.ToLower(k)
	k = strings.ReplaceAll(k, "-", "")
	k = strings.ReplaceAll(k, "_", "")
	k = strings.ReplaceAll(k, " ", "")
	return k
}

func parseClashRules(norm map[string]interface{}) []clashRule {
	v, ok := norm["rules"]
	if !ok {
		return nil
	}
	list, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]clashRule, 0, len(list))
	for _, item := range list {
		line, ok := item.(string)
		if !ok {
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		rule, ok := parseClashRuleLine(line)
		if !ok {
			continue
		}
		out = append(out, rule)
	}
	return out
}

func parseClashRuleLine(line string) (clashRule, bool) {
	parts := strings.Split(line, ",")
	if len(parts) < 2 {
		return clashRule{}, false
	}
	kind := strings.ToUpper(strings.TrimSpace(parts[0]))
	switch kind {
	case "MATCH", "FINAL":
		target := strings.TrimSpace(parts[1])
		if target == "" {
			return clashRule{}, false
		}
		return clashRule{Kind: kind, Target: target, RawLine: line}, true
	default:
		if len(parts) < 3 {
			return clashRule{}, false
		}
		value := strings.TrimSpace(parts[1])
		target := strings.TrimSpace(parts[2])
		if value == "" || target == "" {
			return clashRule{}, false
		}
		return clashRule{Kind: kind, Value: value, Target: target, RawLine: line}, true
	}
}

func parseClashProxyGroups(norm map[string]interface{}) []clashProxyGroup {
	for _, key := range []string{"proxygroups", "proxygroup"} {
		if v, ok := norm[key]; ok {
			return parseClashProxyGroupList(v)
		}
	}
	return nil
}

func parseClashProxyGroupList(v interface{}) []clashProxyGroup {
	list, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]clashProxyGroup, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := m["name"].(string)
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		proxies := parseStringList(m["proxies"])
		out = append(out, clashProxyGroup{Name: name, Proxies: proxies})
	}
	return out
}

func parseStringList(v interface{}) []string {
	list, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		s, ok := item.(string)
		if !ok {
			continue
		}
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}

func buildFRouterFromClash(cfg domain.Config, nodes []domain.Node, groups []clashProxyGroup, rules []clashRule) domain.FRouter {
	nodesByName := make(map[string]domain.Node, len(nodes))
	for _, n := range nodes {
		if strings.TrimSpace(n.Name) == "" {
			continue
		}
		nodesByName[n.Name] = n
	}

	groupMap := make(map[string][]string, len(groups))
	for _, g := range groups {
		groupMap[g.Name] = g.Proxies
	}

	groupNames := make([]string, 0, len(groupMap))
	for name := range groupMap {
		groupNames = append(groupNames, name)
	}
	sort.Strings(groupNames)

	slots := make([]domain.SlotNode, 0, len(groupNames))
	groupSlotID := make(map[string]string, len(groupNames))
	for i, name := range groupNames {
		id := fmt.Sprintf("slot-%d", i+1)
		groupSlotID[name] = id
		slot := domain.SlotNode{ID: id, Name: name}
		if nodeID := resolveGroupNodeID(name, groupMap, nodesByName, map[string]struct{}{}); nodeID != "" {
			slot.BoundNodeID = nodeID
		}
		slots = append(slots, slot)
	}

	defaultTo := domain.EdgeNodeDirect
	for i := len(rules) - 1; i >= 0; i-- {
		r := rules[i]
		if r.Kind != "MATCH" && r.Kind != "FINAL" {
			continue
		}
		if to, ok := resolveClashTargetTo(r.Target, nodesByName, groupSlotID, slots); ok {
			defaultTo = to
		}
		break
	}

	edges := make([]domain.ProxyEdge, 0, len(rules)+1)
	edges = append(edges, domain.ProxyEdge{
		ID:       uuid.NewString(),
		From:     domain.EdgeNodeLocal,
		To:       defaultTo,
		Priority: 0,
		Enabled:  true,
	})

	priority := len(rules)
	for _, r := range rules {
		if r.Kind == "MATCH" || r.Kind == "FINAL" {
			continue
		}
		match, ok := convertClashRuleToMatch(r)
		if !ok {
			priority--
			continue
		}
		to, ok := resolveClashTargetTo(r.Target, nodesByName, groupSlotID, slots)
		if !ok {
			priority--
			continue
		}
		edges = append(edges, domain.ProxyEdge{
			ID:          uuid.NewString(),
			From:        domain.EdgeNodeLocal,
			To:          to,
			Priority:    priority,
			Enabled:     true,
			RuleType:    domain.EdgeRuleRoute,
			RouteRule:   &match,
			Description: r.RawLine,
		})
		priority--
	}

	return domain.FRouter{
		Name:           cfg.Name,
		SourceConfigID: cfg.ID,
		ChainProxy: domain.ChainProxySettings{
			Slots: slots,
			Edges: edges,
		},
	}
}

func resolveGroupNodeID(group string, groupMap map[string][]string, nodesByName map[string]domain.Node, visited map[string]struct{}) string {
	if _, ok := visited[group]; ok {
		return ""
	}
	visited[group] = struct{}{}

	proxies := groupMap[group]
	for _, name := range proxies {
		if isClashSpecialTarget(name) {
			continue
		}
		if n, ok := nodesByName[name]; ok && strings.TrimSpace(n.ID) != "" {
			return n.ID
		}
		if _, ok := groupMap[name]; ok {
			if nodeID := resolveGroupNodeID(name, groupMap, nodesByName, visited); nodeID != "" {
				return nodeID
			}
		}
	}
	return ""
}

func resolveClashTargetTo(target string, nodesByName map[string]domain.Node, groupSlotID map[string]string, slots []domain.SlotNode) (string, bool) {
	if isClashDirect(target) {
		return domain.EdgeNodeDirect, true
	}
	if isClashBlock(target) {
		return domain.EdgeNodeBlock, true
	}
	if n, ok := nodesByName[target]; ok && strings.TrimSpace(n.ID) != "" {
		return n.ID, true
	}
	if slotID, ok := groupSlotID[target]; ok {
		for _, s := range slots {
			if s.ID == slotID && strings.TrimSpace(s.BoundNodeID) != "" {
				return slotID, true
			}
		}
	}
	return "", false
}

func isClashDirect(target string) bool {
	return strings.EqualFold(strings.TrimSpace(target), "DIRECT")
}

func isClashBlock(target string) bool {
	switch strings.ToUpper(strings.TrimSpace(target)) {
	case "REJECT", "REJECT-DROP":
		return true
	default:
		return false
	}
}

func isClashSpecialTarget(target string) bool {
	return isClashDirect(target) || isClashBlock(target)
}

func convertClashRuleToMatch(rule clashRule) (domain.RouteMatchRule, bool) {
	kind := strings.ToUpper(strings.TrimSpace(rule.Kind))
	value := strings.TrimSpace(rule.Value)
	if value == "" {
		return domain.RouteMatchRule{}, false
	}
	switch kind {
	case "DOMAIN":
		return domain.RouteMatchRule{Domains: []string{"full:" + value}}, true
	case "DOMAIN-SUFFIX":
		return domain.RouteMatchRule{Domains: []string{"domain:" + value}}, true
	case "DOMAIN-KEYWORD":
		return domain.RouteMatchRule{Domains: []string{"keyword:" + value}}, true
	case "DOMAIN-REGEX":
		return domain.RouteMatchRule{Domains: []string{"regexp:" + value}}, true
	case "GEOSITE":
		return domain.RouteMatchRule{Domains: []string{"geosite:" + strings.ToLower(value)}}, true
	case "IP-CIDR", "IP-CIDR6":
		return domain.RouteMatchRule{IPs: []string{value}}, true
	case "GEOIP":
		return domain.RouteMatchRule{IPs: []string{"geoip:" + strings.ToLower(value)}}, true
	default:
		return domain.RouteMatchRule{}, false
	}
}
