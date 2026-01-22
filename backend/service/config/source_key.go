package config

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"

	"vea/backend/domain"
)

type existingSubscriptionNodeIndex struct {
	bySourceKey map[string]string
	byStableKey map[string]string
	byLegacyKey map[string]string

	ambiguousSourceKey map[string]struct{}
	ambiguousStableKey map[string]struct{}
	ambiguousLegacyKey map[string]struct{}
}

func buildExistingSubscriptionNodeIndex(nodes []domain.Node) existingSubscriptionNodeIndex {
	if len(nodes) == 0 {
		return existingSubscriptionNodeIndex{}
	}

	bySourceKey := make(map[string]string, len(nodes))
	byStableKey := make(map[string]string, len(nodes))
	byLegacyKey := make(map[string]string, len(nodes))
	ambiguousSourceKey := make(map[string]struct{}, 8)
	ambiguousStableKey := make(map[string]struct{}, 8)
	ambiguousLegacyKey := make(map[string]struct{}, 8)

	addUnique := func(by map[string]string, ambiguous map[string]struct{}, key string, id string) {
		if key == "" || id == "" {
			return
		}
		if _, ok := ambiguous[key]; ok {
			return
		}
		if prev, ok := by[key]; ok && prev != id {
			ambiguous[key] = struct{}{}
			delete(by, key)
			return
		}
		if _, ok := by[key]; !ok {
			by[key] = id
		}
	}

	for _, n := range nodes {
		id := strings.TrimSpace(n.ID)
		if id == "" {
			continue
		}
		if key := normalizeSourceKey(n.SourceKey); key != "" {
			addUnique(bySourceKey, ambiguousSourceKey, key, id)
		}
		if key := normalizeSourceKey(subscriptionStableKey(n)); key != "" {
			addUnique(byStableKey, ambiguousStableKey, key, id)
		}
		// legacy key: do NOT depend on SourceKey (SourceKey 可能来自旧版本的“易漂移”派生策略)
		if key := subscriptionLegacyKey(n); key != "" {
			addUnique(byLegacyKey, ambiguousLegacyKey, key, id)
		}
	}

	return existingSubscriptionNodeIndex{
		bySourceKey:        bySourceKey,
		byStableKey:        byStableKey,
		byLegacyKey:        byLegacyKey,
		ambiguousSourceKey: ambiguousSourceKey,
		ambiguousStableKey: ambiguousStableKey,
		ambiguousLegacyKey: ambiguousLegacyKey,
	}
}

func normalizeAndDisambiguateSubscriptionSourceKeys(nodes []domain.Node) []domain.Node {
	if len(nodes) == 0 {
		return nodes
	}

	// normalize
	for i := range nodes {
		nodes[i].SourceKey = deriveSubscriptionKey(nodes[i])
	}

	// group by base key
	dup := make(map[string][]int, 8)
	for i := range nodes {
		key := strings.TrimSpace(nodes[i].SourceKey)
		if key == "" {
			continue
		}
		dup[key] = append(dup[key], i)
	}

	for base, idxs := range dup {
		if len(idxs) <= 1 {
			continue
		}

		nextKeys := make(map[string]int, len(idxs))
		hasAmbiguity := false
		for _, idx := range idxs {
			stable := subscriptionStableKey(nodes[idx])
			if stable == "" {
				hasAmbiguity = true
				break
			}
			sum := sha256.Sum256([]byte(stable))
			suffix := hex.EncodeToString(sum[:4])
			nextKey := base + "|" + suffix
			nextKeys[nextKey]++
		}
		if hasAmbiguity {
			for _, idx := range idxs {
				nodes[idx].SourceKey = ""
			}
			continue
		}

		for _, idx := range idxs {
			stable := subscriptionStableKey(nodes[idx])
			sum := sha256.Sum256([]byte(stable))
			suffix := hex.EncodeToString(sum[:4])
			nextKey := base + "|" + suffix
			if nextKeys[nextKey] != 1 {
				hasAmbiguity = true
				break
			}
			nodes[idx].SourceKey = nextKey
		}
		if hasAmbiguity {
			for _, idx := range idxs {
				nodes[idx].SourceKey = ""
			}
		}
	}

	return nodes
}

func reuseNodeIDsBySubscriptionKey(index existingSubscriptionNodeIndex, nodes []domain.Node) ([]domain.Node, map[string]string) {
	if len(nodes) == 0 || (len(index.bySourceKey) == 0 && len(index.byStableKey) == 0 && len(index.byLegacyKey) == 0) {
		return nodes, nil
	}

	used := make(map[string]struct{}, len(nodes))
	for i := range nodes {
		id := strings.TrimSpace(nodes[i].ID)
		if id != "" {
			used[id] = struct{}{}
		}
	}

	mapping := make(map[string]string, 8)

	for i := range nodes {
		originalID := strings.TrimSpace(nodes[i].ID)
		if originalID != "" {
			continue
		}

		sourceKey := normalizeSourceKey(nodes[i].SourceKey)
		stableKey := normalizeSourceKey(subscriptionStableKey(nodes[i]))
		legacyKey := subscriptionLegacyKey(nodes[i])

		existingID := ""
		if sourceKey != "" {
			existingID = strings.TrimSpace(index.bySourceKey[sourceKey])
		}
		if existingID == "" && stableKey != "" {
			existingID = strings.TrimSpace(index.byStableKey[stableKey])
		}
		if existingID == "" && legacyKey != "" {
			existingID = strings.TrimSpace(index.byLegacyKey[legacyKey])
		}
		if existingID == "" {
			continue
		}
		if _, ok := used[existingID]; ok {
			continue
		}
		nodes[i].ID = existingID
		used[existingID] = struct{}{}
		if originalID != "" {
			mapping[originalID] = existingID
		}
	}

	if len(mapping) == 0 {
		return nodes, nil
	}
	return nodes, mapping
}

func buildSubscriptionNodeIDRewriteMap(existing []domain.Node, next []domain.Node) map[string]string {
	if len(existing) == 0 || len(next) == 0 {
		return nil
	}

	buildUniqueIDIndex := func(nodes []domain.Node, keyFn func(domain.Node) string) map[string]string {
		byKey := make(map[string]string, len(nodes))
		ambiguous := make(map[string]struct{}, 8)
		for _, n := range nodes {
			id := strings.TrimSpace(n.ID)
			if id == "" {
				continue
			}
			key := keyFn(n)
			if key == "" {
				continue
			}
			if _, ok := ambiguous[key]; ok {
				continue
			}
			if prev, ok := byKey[key]; ok && prev != id {
				ambiguous[key] = struct{}{}
				delete(byKey, key)
				continue
			}
			if _, ok := byKey[key]; !ok {
				byKey[key] = id
			}
		}
		return byKey
	}

	out := make(map[string]string, 8)

	mergeByKey := func(existingByKey map[string]string, nextByKey map[string]string) {
		for key, oldID := range existingByKey {
			newID := nextByKey[key]
			if newID == "" || newID == oldID {
				continue
			}
			// 只在“一个旧ID对应一个新ID”时写入；如发现冲突则移除该映射，避免误改。
			if prev, ok := out[oldID]; ok && prev != newID {
				delete(out, oldID)
				continue
			}
			if _, ok := out[oldID]; !ok {
				out[oldID] = newID
			}
		}
	}

	// 优先按稳定键（不包含 uuid/password/path/fp 等易漂移字段）做映射。
	existingByStable := buildUniqueIDIndex(existing, func(n domain.Node) string { return normalizeSourceKey(subscriptionStableKey(n)) })
	nextByStable := buildUniqueIDIndex(next, func(n domain.Node) string { return normalizeSourceKey(subscriptionStableKey(n)) })
	mergeByKey(existingByStable, nextByStable)

	// 其次按 legacy key（name/address）补齐少量无法生成 stable key 的节点。
	existingByLegacy := buildUniqueIDIndex(existing, func(n domain.Node) string { return subscriptionLegacyKey(n) })
	nextByLegacy := buildUniqueIDIndex(next, func(n domain.Node) string { return subscriptionLegacyKey(n) })
	mergeByKey(existingByLegacy, nextByLegacy)

	// 最后按 sourceKey（历史遗留/已持久化的 SourceKey）。
	existingBySource := buildUniqueIDIndex(existing, func(n domain.Node) string { return normalizeSourceKey(n.SourceKey) })
	nextBySource := buildUniqueIDIndex(next, func(n domain.Node) string { return normalizeSourceKey(n.SourceKey) })
	mergeByKey(existingBySource, nextBySource)

	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeSourceKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func deriveSubscriptionKey(node domain.Node) string {
	if v := normalizeSourceKey(node.SourceKey); v != "" {
		return v
	}
	if v := normalizeSourceKey(node.Name); v != "" {
		return v
	}
	if v := normalizeSourceKey(node.Address); v != "" {
		return v
	}
	return ""
}

// subscriptionStableKey 用于订阅节点的“稳定指纹”，用于：
// - 同名节点的稳定去重/消歧（避免 suffix 依赖 uuid/password 等易滚动字段）
// - 订阅同步时的节点 ID 复用与 FRouter 引用重写
//
// 设计目标：尽量在“订阅参数滚动更新（uuid/password/path/fp等）”时保持不变。
func subscriptionStableKey(node domain.Node) string {
	proto := strings.ToLower(strings.TrimSpace(string(node.Protocol)))
	addr := strings.ToLower(strings.TrimSpace(node.Address))
	if proto == "" || addr == "" || node.Port <= 0 {
		return ""
	}

	transport := ""
	if node.Transport != nil {
		typ := strings.ToLower(strings.TrimSpace(node.Transport.Type))
		if typ != "" && typ != "tcp" {
			transport = typ
		}
	}

	tls := ""
	if node.TLS != nil && node.TLS.Enabled {
		typ := strings.ToLower(strings.TrimSpace(node.TLS.Type))
		if typ == "" {
			typ = "tls"
		}
		tls = typ
	}

	return proto + "|" + addr + "|" + strconv.Itoa(node.Port) + "|" + transport + "|" + tls
}

func subscriptionLegacyKey(node domain.Node) string {
	if v := normalizeSourceKey(node.Name); v != "" {
		return v
	}
	if v := normalizeSourceKey(node.Address); v != "" {
		return v
	}
	return ""
}
