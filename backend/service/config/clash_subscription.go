package config

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

	"vea/backend/domain"
	"vea/backend/service/shared"
)

type clashSubscription struct {
	Proxies     []map[string]interface{} `yaml:"proxies"`
	ProxyGroups []clashProxyGroup        `yaml:"proxy-groups"`
	Rules       []string                 `yaml:"rules"`
}

type clashProxyGroup struct {
	Name    string   `yaml:"name"`
	Type    string   `yaml:"type"`
	Proxies []string `yaml:"proxies"`
	Use     []string `yaml:"use"`
}

type clashParseResult struct {
	Nodes    []domain.Node
	Chain    domain.ChainProxySettings
	Warnings []string
}

func parseClashSubscription(configID string, payload string) (clashParseResult, error) {
	var sub clashSubscription
	if err := yaml.Unmarshal([]byte(payload), &sub); err != nil {
		return clashParseResult{}, err
	}

	warnings := make([]string, 0, 8)

	nodes := make([]domain.Node, 0, len(sub.Proxies))
	proxyNames := make([]string, 0, len(sub.Proxies))
	for _, p := range sub.Proxies {
		node, proxyName, err := parseClashProxyToNode(p)
		if err != nil {
			warnings = append(warnings, err.Error())
			continue
		}
		nodes = append(nodes, node)
		proxyNames = append(proxyNames, proxyName)
	}
	if len(nodes) == 0 {
		return clashParseResult{}, fmt.Errorf("clash yaml has no supported proxies")
	}

	nodes = normalizeAndDisambiguateSubscriptionSourceKeys(nodes)
	proxyNameToNodeID := make(map[string]string, len(nodes))
	for i := range nodes {
		if strings.TrimSpace(nodes[i].SourceKey) != "" {
			nodes[i].ID = domain.StableNodeIDForSourceKey(configID, nodes[i].SourceKey)
		}
		if strings.TrimSpace(nodes[i].ID) == "" {
			nodes[i].ID = domain.StableNodeIDForConfig(configID, nodes[i])
		}
		proxyName := strings.TrimSpace(proxyNames[i])
		if proxyName == "" || strings.TrimSpace(nodes[i].ID) == "" {
			continue
		}
		if prev, ok := proxyNameToNodeID[proxyName]; ok && prev != "" && prev != nodes[i].ID {
			warnings = append(warnings, fmt.Sprintf("duplicate proxy name %q; overriding previous node %s", proxyName, prev))
		}
		proxyNameToNodeID[proxyName] = nodes[i].ID
	}

	groupMembers := make(map[string][]string, len(sub.ProxyGroups))
	groupNames := make([]string, 0, len(sub.ProxyGroups))
	for _, g := range sub.ProxyGroups {
		name := strings.TrimSpace(g.Name)
		if name == "" {
			continue
		}
		if _, ok := groupMembers[name]; ok {
			continue
		}
		members := make([]string, 0, len(g.Proxies))
		for _, m := range g.Proxies {
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			members = append(members, m)
		}
		groupMembers[name] = members
		groupNames = append(groupNames, name)
		if len(g.Use) > 0 {
			warnings = append(warnings, fmt.Sprintf("proxy-group %s uses providers (%d); proxy-providers are not parsed", name, len(g.Use)))
		}
	}
	sort.Strings(groupNames)

	resolvedGroups := make(map[string]string, len(groupMembers))
	resolving := make(map[string]bool, len(groupMembers))
	var resolveGroup func(string) string
	resolveGroup = func(name string) string {
		name = strings.TrimSpace(name)
		if name == "" {
			return ""
		}
		if isDirectTarget(name) || isBlockTarget(name) {
			return strings.ToUpper(name)
		}
		if _, ok := proxyNameToNodeID[name]; ok {
			return name
		}
		if resolved, ok := resolvedGroups[name]; ok {
			return resolved
		}
		if resolving[name] {
			return ""
		}
		resolving[name] = true
		defer func() { resolving[name] = false }()

		members, ok := groupMembers[name]
		if !ok {
			return ""
		}
		for _, m := range members {
			if resolved := resolveGroup(m); resolved != "" {
				resolvedGroups[name] = resolved
				return resolved
			}
		}
		return ""
	}

	slotsByGroup := make(map[string]domain.SlotNode, len(groupMembers))
	ensureSlot := func(groupName string, boundNodeID string) string {
		slot, ok := slotsByGroup[groupName]
		if !ok {
			slot = domain.SlotNode{
				ID:   stableSlotIDForConfig(configID, groupName),
				Name: groupName,
			}
		}
		slot.BoundNodeID = boundNodeID
		slotsByGroup[groupName] = slot
		return slot.ID
	}

	resolveTargetStrict := func(target string) (string, bool) {
		target = strings.TrimSpace(target)
		if target == "" {
			return "", false
		}
		if isDirectTarget(target) {
			return domain.EdgeNodeDirect, true
		}
		if isBlockTarget(target) {
			return domain.EdgeNodeBlock, true
		}
		if _, ok := groupMembers[target]; ok {
			resolved := resolveGroup(target)
			if resolved == "" {
				return "", false
			}
			if isDirectTarget(resolved) {
				return domain.EdgeNodeDirect, true
			}
			if isBlockTarget(resolved) {
				return domain.EdgeNodeBlock, true
			}
			nodeID := proxyNameToNodeID[resolved]
			if nodeID == "" {
				return "", false
			}
			return ensureSlot(target, nodeID), true
		}
		if nodeID, ok := proxyNameToNodeID[target]; ok && nodeID != "" {
			return nodeID, true
		}
		return "", false
	}

	resolveTargetDefault := func(target string) string {
		if to, ok := resolveTargetStrict(target); ok {
			return to
		}
		warnings = append(warnings, fmt.Sprintf("default target %q not resolved; fallback to DIRECT", strings.TrimSpace(target)))
		return domain.EdgeNodeDirect
	}

	edges := make([]domain.ProxyEdge, 0, len(sub.Rules)+8)
	defaultTarget := ""
	for _, raw := range sub.Rules {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := splitAndTrimComma(line)
		if len(parts) == 0 {
			continue
		}
		ruleType := strings.ToUpper(parts[0])
		switch ruleType {
		case "MATCH", "FINAL":
			if len(parts) >= 2 {
				defaultTarget = parts[1]
			} else {
				warnings = append(warnings, fmt.Sprintf("invalid %s rule: %q", ruleType, line))
			}
			continue
		}
		if len(parts) < 3 {
			warnings = append(warnings, fmt.Sprintf("invalid rule (need 3 fields): %q", line))
			continue
		}
		value := parts[1]
		target := parts[2]

		match, ok := toRouteMatchRule(ruleType, value)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("unsupported rule type %s: %q", ruleType, line))
			continue
		}
		to, ok := resolveTargetStrict(target)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("rule target %q not resolved: %q", strings.TrimSpace(target), line))
			continue
		}
		edges = append(edges, domain.ProxyEdge{
			ID:        uuid.NewString(),
			From:      domain.EdgeNodeLocal,
			To:        to,
			Priority:  0,
			Enabled:   true,
			RuleType:  domain.EdgeRuleRoute,
			RouteRule: &match,
		})
	}

	beforeCompact := len(edges)
	edges = compactClashSelectionEdges(edges)
	if after := len(edges); after != beforeCompact && beforeCompact > 0 {
		warnings = append(warnings, fmt.Sprintf("clash rules compacted by target: %d -> %d edges", beforeCompact, after))
	}
	// 压缩规则后重新归一化优先级，避免优先级依赖原始 rules 行号且出现超大/不连续值。
	assignPriorities(edges)

	if strings.TrimSpace(defaultTarget) == "" {
		defaultTarget = guessDefaultTarget(groupNames, proxyNameToNodeID)
	}

	edges = append(edges, domain.ProxyEdge{
		ID:       uuid.NewString(),
		From:     domain.EdgeNodeLocal,
		To:       resolveTargetDefault(defaultTarget),
		Priority: 0,
		Enabled:  true,
	})

	slots := make([]domain.SlotNode, 0, len(slotsByGroup))
	for _, g := range groupNames {
		if slot, ok := slotsByGroup[g]; ok {
			slots = append(slots, slot)
		}
	}
	sort.Slice(slots, func(i, j int) bool { return slots[i].Name < slots[j].Name })

	return clashParseResult{
		Nodes: nodes,
		Chain: domain.ChainProxySettings{
			Edges:     edges,
			Slots:     slots,
			UpdatedAt: time.Now(),
		},
		Warnings: warnings,
	}, nil
}

func toRouteMatchRule(ruleType string, value string) (domain.RouteMatchRule, bool) {
	ruleType = strings.ToUpper(strings.TrimSpace(ruleType))
	value = strings.TrimSpace(value)
	if value == "" {
		return domain.RouteMatchRule{}, false
	}
	switch ruleType {
	case "DOMAIN":
		return domain.RouteMatchRule{Domains: []string{"full:" + value}}, true
	case "DOMAIN-SUFFIX":
		return domain.RouteMatchRule{Domains: []string{"domain:" + value}}, true
	case "DOMAIN-KEYWORD":
		return domain.RouteMatchRule{Domains: []string{"keyword:" + value}}, true
	case "DOMAIN-REGEX":
		return domain.RouteMatchRule{Domains: []string{"regexp:" + value}}, true
	case "GEOSITE":
		return domain.RouteMatchRule{Domains: []string{"geosite:" + value}}, true
	case "GEOIP":
		return domain.RouteMatchRule{IPs: []string{"geoip:" + value}}, true
	case "IP-CIDR", "IP-CIDR6":
		return domain.RouteMatchRule{IPs: []string{value}}, true
	default:
		return domain.RouteMatchRule{}, false
	}
}

func guessDefaultTarget(groupNames []string, proxyNameToNodeID map[string]string) string {
	for _, name := range groupNames {
		if strings.EqualFold(name, "PROXY") {
			return name
		}
	}
	if len(groupNames) > 0 {
		return groupNames[0]
	}
	if len(proxyNameToNodeID) > 0 {
		names := make([]string, 0, len(proxyNameToNodeID))
		for name := range proxyNameToNodeID {
			names = append(names, name)
		}
		sort.Strings(names)
		return names[0]
	}
	return domain.EdgeNodeDirect
}

func isDirectTarget(target string) bool {
	return strings.EqualFold(strings.TrimSpace(target), "DIRECT")
}

func isBlockTarget(target string) bool {
	switch strings.ToUpper(strings.TrimSpace(target)) {
	case "REJECT", "REJECT-DROP", "BLOCK", "DROP":
		return true
	default:
		return false
	}
}

func splitAndTrimComma(s string) []string {
	raw := strings.Split(s, ",")
	out := make([]string, 0, len(raw))
	for _, part := range raw {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func assignPriorities(edges []domain.ProxyEdge) {
	for i := range edges {
		priority := len(edges) - i
		if priority < 1 {
			priority = 1
		}
		edges[i].Priority = priority
	}
}

// compactClashSelectionEdges merges consecutive local->* route edges with the same target (To) to reduce edge count
// while preserving rule order semantics (only merges adjacent rules in evaluation order).
//
// Note: we only merge "selection edges" (from=local, ruleType=route, routeRule non-empty). Default edge is untouched.
func compactClashSelectionEdges(edges []domain.ProxyEdge) []domain.ProxyEdge {
	if len(edges) < 2 {
		return edges
	}

	type agg struct {
		edge       domain.ProxyEdge
		domainSeen map[string]struct{}
		ipSeen     map[string]struct{}
	}

	flush := func(out *[]domain.ProxyEdge, a *agg) {
		if a == nil {
			return
		}
		*out = append(*out, a.edge)
	}

	isCompactable := func(edge domain.ProxyEdge) bool {
		if edge.From != domain.EdgeNodeLocal {
			return false
		}
		if edge.RuleType != domain.EdgeRuleRoute {
			return false
		}
		if edge.RouteRule == nil {
			return false
		}
		return len(edge.RouteRule.Domains) > 0 || len(edge.RouteRule.IPs) > 0
	}

	viaEqual := func(a, b []string) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if strings.TrimSpace(a[i]) != strings.TrimSpace(b[i]) {
				return false
			}
		}
		return true
	}

	canMerge := func(a domain.ProxyEdge, b domain.ProxyEdge) bool {
		if a.From != b.From {
			return false
		}
		if a.RuleType != b.RuleType {
			return false
		}
		if a.To != b.To {
			return false
		}
		if a.Enabled != b.Enabled {
			return false
		}
		if !viaEqual(a.Via, b.Via) {
			return false
		}
		return true
	}

	addUnique := func(dst *[]string, seen map[string]struct{}, items []string) {
		for _, item := range items {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			*dst = append(*dst, item)
		}
	}

	newAgg := func(edge domain.ProxyEdge) *agg {
		rr := domain.RouteMatchRule{}
		if edge.RouteRule != nil {
			rr = *edge.RouteRule
		}
		rr.Domains = append([]string(nil), rr.Domains...)
		rr.IPs = append([]string(nil), rr.IPs...)

		domainSeen := make(map[string]struct{}, len(rr.Domains))
		ipSeen := make(map[string]struct{}, len(rr.IPs))
		compactDomains := make([]string, 0, len(rr.Domains))
		compactIPs := make([]string, 0, len(rr.IPs))
		addUnique(&compactDomains, domainSeen, rr.Domains)
		addUnique(&compactIPs, ipSeen, rr.IPs)
		rr.Domains = compactDomains
		rr.IPs = compactIPs
		edge.RouteRule = &rr

		return &agg{
			edge:       edge,
			domainSeen: domainSeen,
			ipSeen:     ipSeen,
		}
	}

	mergeInto := func(a *agg, edge domain.ProxyEdge) {
		if a == nil || a.edge.RouteRule == nil {
			return
		}
		if edge.RouteRule == nil {
			return
		}
		addUnique(&a.edge.RouteRule.Domains, a.domainSeen, edge.RouteRule.Domains)
		addUnique(&a.edge.RouteRule.IPs, a.ipSeen, edge.RouteRule.IPs)
	}

	out := make([]domain.ProxyEdge, 0, len(edges))
	var current *agg

	for _, edge := range edges {
		if !isCompactable(edge) {
			flush(&out, current)
			current = nil
			out = append(out, edge)
			continue
		}

		if current == nil {
			current = newAgg(edge)
			continue
		}
		if !canMerge(current.edge, edge) {
			flush(&out, current)
			current = newAgg(edge)
			continue
		}
		mergeInto(current, edge)
	}
	flush(&out, current)
	return out
}

func stableSlotIDForConfig(configID string, groupName string) string {
	id := uuid.NewSHA1(uuid.NameSpaceOID, []byte(configID+"|clash-slot|"+strings.TrimSpace(groupName))).String()
	return domain.EdgeNodeSlotPrefix + id
}

func stableFRouterIDForConfig(configID string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(configID+"|clash-frouter")).String()
}

func parseClashProxyToNode(p map[string]interface{}) (domain.Node, string, error) {
	proxyName := strings.TrimSpace(mapString(p, "name"))
	proxyType := strings.ToLower(strings.TrimSpace(mapString(p, "type")))
	server := strings.TrimSpace(mapString(p, "server"))
	if server == "" {
		server = strings.TrimSpace(mapString(p, "address"))
	}
	port := mapInt(p, "port")
	if strings.TrimSpace(proxyType) == "" {
		return domain.Node{}, proxyName, fmt.Errorf("proxy %q: missing type", proxyName)
	}
	if server == "" || port <= 0 {
		return domain.Node{}, proxyName, fmt.Errorf("proxy %q: invalid server/port", proxyName)
	}

	node := domain.Node{
		Name:    proxyName,
		Address: server,
		Port:    port,
	}
	sec := &domain.NodeSecurity{}

	switch proxyType {
	case "vless":
		node.Protocol = domain.ProtocolVLESS
		sec.UUID = strings.TrimSpace(mapString(p, "uuid"))
		sec.Flow = strings.TrimSpace(mapString(p, "flow"))
		sec.Encryption = strings.TrimSpace(mapString(p, "encryption"))
		if sec.Encryption == "" {
			sec.Encryption = "none"
		}
	case "vmess":
		node.Protocol = domain.ProtocolVMess
		sec.UUID = strings.TrimSpace(mapString(p, "uuid"))
		sec.AlterID = mapInt(p, "alterId")
		sec.Encryption = strings.TrimSpace(mapString(p, "cipher"))
		if sec.Encryption == "" {
			sec.Encryption = "auto"
		}
	case "trojan":
		node.Protocol = domain.ProtocolTrojan
		sec.Password = strings.TrimSpace(mapString(p, "password"))
	case "ss", "shadowsocks":
		node.Protocol = domain.ProtocolShadowsocks
		sec.Method = strings.TrimSpace(mapString(p, "cipher"))
		sec.Password = strings.TrimSpace(mapString(p, "password"))
		plugin := strings.TrimSpace(mapString(p, "plugin"))
		var pluginOpts string
		if opts, ok := p["plugin-opts"]; ok {
			plugin, pluginOpts = normalizeClashShadowsocksPlugin(plugin, opts)
		} else {
			plugin, pluginOpts = shared.NormalizeShadowsocksPluginAlias(plugin, "")
		}
		sec.Plugin = plugin
		sec.PluginOpts = pluginOpts
	case "hysteria2":
		node.Protocol = domain.ProtocolHysteria2
		sec.Password = strings.TrimSpace(mapString(p, "password"))
	case "tuic":
		node.Protocol = domain.ProtocolTUIC
		sec.UUID = strings.TrimSpace(mapString(p, "uuid"))
		sec.Password = strings.TrimSpace(mapString(p, "password"))
	default:
		return domain.Node{}, proxyName, fmt.Errorf("proxy %q: unsupported type %s", proxyName, proxyType)
	}

	node.Security = sec

	if transport := parseClashTransport(p); transport != nil {
		node.Transport = transport
	}
	if tls := parseClashTLS(p); tls != nil {
		node.TLS = tls
	}

	if strings.TrimSpace(node.Name) == "" {
		node.Name = node.Address
	}
	return node, proxyName, nil
}

func parseClashTransport(p map[string]interface{}) *domain.NodeTransport {
	network := strings.ToLower(strings.TrimSpace(mapString(p, "network")))
	switch network {
	case "ws":
		ws := mapMap(p, "ws-opts")
		headers := mapMap(ws, "headers")
		outHeaders := make(map[string]string)
		host := ""
		for k, v := range headers {
			key := strings.TrimSpace(k)
			if key == "" {
				continue
			}
			val := strings.TrimSpace(fmt.Sprint(v))
			if val == "" {
				continue
			}
			if strings.EqualFold(key, "Host") {
				host = val
				continue
			}
			outHeaders[key] = val
		}
		t := &domain.NodeTransport{
			Type:    "ws",
			Host:    host,
			Path:    strings.TrimSpace(mapString(ws, "path")),
			Headers: outHeaders,
		}
		if len(t.Headers) == 0 {
			t.Headers = nil
		}
		if t.Host == "" && t.Path == "" && t.Headers == nil {
			return nil
		}
		return t

	case "grpc":
		grpc := mapMap(p, "grpc-opts")
		serviceName := strings.TrimSpace(mapString(grpc, "grpc-service-name"))
		if serviceName == "" {
			serviceName = strings.TrimSpace(mapString(grpc, "service-name"))
		}
		if serviceName == "" {
			return nil
		}
		return &domain.NodeTransport{
			Type:        "grpc",
			ServiceName: serviceName,
		}

	case "h2":
		h2 := mapMap(p, "h2-opts")
		host := firstString(mapStringSlice(h2, "host"))
		path := strings.TrimSpace(mapString(h2, "path"))
		if host == "" && path == "" {
			return nil
		}
		return &domain.NodeTransport{
			Type: "h2",
			Host: host,
			Path: path,
		}

	case "http":
		httpOpts := mapMap(p, "http-opts")
		host := firstString(mapStringSlice(httpOpts, "host"))
		path := firstString(mapStringSlice(httpOpts, "path"))
		if path == "" {
			path = strings.TrimSpace(mapString(httpOpts, "path"))
		}
		if host == "" && path == "" {
			return nil
		}
		return &domain.NodeTransport{
			Type: "http",
			Host: host,
			Path: path,
		}
	default:
		return nil
	}
}

func parseClashTLS(p map[string]interface{}) *domain.NodeTLS {
	tlsEnabled := mapBool(p, "tls")
	serverName := strings.TrimSpace(mapString(p, "servername"))
	sni := strings.TrimSpace(mapString(p, "sni"))
	if serverName != "" || sni != "" {
		tlsEnabled = true
	}
	insecure := mapBool(p, "skip-cert-verify")
	fingerprint := strings.TrimSpace(mapString(p, "client-fingerprint"))
	alpn := mapStringSlice(p, "alpn")
	if insecure || fingerprint != "" || len(alpn) > 0 {
		tlsEnabled = true
	}
	reality := mapMap(p, "reality-opts")
	realityPub := strings.TrimSpace(mapString(reality, "public-key"))
	realitySID := strings.TrimSpace(mapString(reality, "short-id"))
	if realityPub != "" {
		tlsEnabled = true
	}

	if !tlsEnabled {
		return nil
	}

	typ := "tls"
	if realityPub != "" {
		typ = "reality"
	}
	out := &domain.NodeTLS{
		Enabled:          true,
		Type:             typ,
		ServerName:       firstNonEmpty(serverName, sni),
		Insecure:         insecure,
		Fingerprint:      fingerprint,
		RealityPublicKey: realityPub,
		RealityShortID:   realitySID,
	}
	if len(alpn) > 0 {
		out.ALPN = alpn
	}
	return out
}

func stringifyPluginOpts(raw interface{}) string {
	if raw == nil {
		return ""
	}
	if s, ok := raw.(string); ok {
		return strings.TrimSpace(s)
	}
	m := asStringMap(raw)
	if len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		v := m[k]
		switch vv := v.(type) {
		case bool:
			if vv {
				parts = append(parts, k)
			}
		default:
			val := strings.TrimSpace(fmt.Sprint(v))
			if val == "" {
				continue
			}
			parts = append(parts, k+"="+val)
		}
	}
	return strings.Join(parts, ";")
}

func normalizeClashShadowsocksPlugin(plugin string, rawOpts interface{}) (string, string) {
	plugin = strings.TrimSpace(plugin)
	if plugin == "" {
		return "", stringifyPluginOpts(rawOpts)
	}
	return shared.NormalizeShadowsocksPluginAlias(plugin, stringifyPluginOpts(rawOpts))
}

func mapString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key]; ok {
		switch vv := v.(type) {
		case string:
			return vv
		default:
			return fmt.Sprint(v)
		}
	}
	for k, v := range m {
		if strings.EqualFold(k, key) {
			switch vv := v.(type) {
			case string:
				return vv
			default:
				return fmt.Sprint(v)
			}
		}
	}
	return ""
}

func mapInt(m map[string]interface{}, key string) int {
	if m == nil {
		return 0
	}
	if v, ok := m[key]; ok {
		return anyInt(v)
	}
	for k, v := range m {
		if strings.EqualFold(k, key) {
			return anyInt(v)
		}
	}
	return 0
}

func anyInt(v interface{}) int {
	switch vv := v.(type) {
	case int:
		return vv
	case int64:
		return int(vv)
	case float64:
		return int(vv)
	case float32:
		return int(vv)
	case string:
		i, _ := strconv.Atoi(strings.TrimSpace(vv))
		return i
	default:
		i, _ := strconv.Atoi(strings.TrimSpace(fmt.Sprint(v)))
		return i
	}
}

func mapBool(m map[string]interface{}, key string) bool {
	if m == nil {
		return false
	}
	if v, ok := m[key]; ok {
		return anyBool(v)
	}
	for k, v := range m {
		if strings.EqualFold(k, key) {
			return anyBool(v)
		}
	}
	return false
}

func anyBool(v interface{}) bool {
	switch vv := v.(type) {
	case bool:
		return vv
	case string:
		return strings.EqualFold(strings.TrimSpace(vv), "true")
	default:
		return strings.EqualFold(strings.TrimSpace(fmt.Sprint(v)), "true")
	}
}

func mapStringSlice(m map[string]interface{}, key string) []string {
	raw := interface{}(nil)
	if m != nil {
		if v, ok := m[key]; ok {
			raw = v
		} else {
			for k, v := range m {
				if strings.EqualFold(k, key) {
					raw = v
					break
				}
			}
		}
	}
	if raw == nil {
		return nil
	}
	switch vv := raw.(type) {
	case []string:
		out := make([]string, 0, len(vv))
		for _, s := range vv {
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	case []interface{}:
		out := make([]string, 0, len(vv))
		for _, it := range vv {
			s := strings.TrimSpace(fmt.Sprint(it))
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		s := strings.TrimSpace(vv)
		if s == "" {
			return nil
		}
		return []string{s}
	default:
		s := strings.TrimSpace(fmt.Sprint(raw))
		if s == "" {
			return nil
		}
		return []string{s}
	}
}

func mapMap(m map[string]interface{}, key string) map[string]interface{} {
	if m == nil {
		return nil
	}
	if v, ok := m[key]; ok {
		return asStringMap(v)
	}
	for k, v := range m {
		if strings.EqualFold(k, key) {
			return asStringMap(v)
		}
	}
	return nil
}

func asStringMap(v interface{}) map[string]interface{} {
	switch vv := v.(type) {
	case map[string]interface{}:
		return vv
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(vv))
		for k, v := range vv {
			ks := strings.TrimSpace(fmt.Sprint(k))
			if ks == "" {
				continue
			}
			out[ks] = v
		}
		return out
	default:
		return nil
	}
}

func firstString(list []string) string {
	if len(list) == 0 {
		return ""
	}
	return strings.TrimSpace(list[0])
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			return v
		}
	}
	return ""
}
