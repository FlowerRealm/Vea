package config

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"vea/backend/domain"
)

type existingSubscriptionNodeIndex struct {
	bySourceKey map[string]string
	byLegacyKey map[string]string

	ambiguousSourceKey map[string]struct{}
	ambiguousLegacyKey map[string]struct{}
}

func buildExistingSubscriptionNodeIndex(nodes []domain.Node) existingSubscriptionNodeIndex {
	if len(nodes) == 0 {
		return existingSubscriptionNodeIndex{}
	}

	bySourceKey := make(map[string]string, len(nodes))
	byLegacyKey := make(map[string]string, len(nodes))
	ambiguousSourceKey := make(map[string]struct{}, 8)
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
			continue
		}

		// legacy nodes: early versions didn't persist sourceKey; best-effort reuse by subscription key
		if key := deriveSubscriptionKey(n); key != "" {
			addUnique(byLegacyKey, ambiguousLegacyKey, key, id)
		}
	}

	return existingSubscriptionNodeIndex{
		bySourceKey:        bySourceKey,
		byLegacyKey:        byLegacyKey,
		ambiguousSourceKey: ambiguousSourceKey,
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
			identity := identityKey(nodes[idx])
			if identity == "" {
				hasAmbiguity = true
				break
			}
			sum := sha256.Sum256([]byte(identity))
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
			identity := identityKey(nodes[idx])
			sum := sha256.Sum256([]byte(identity))
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
	if len(nodes) == 0 || (len(index.bySourceKey) == 0 && len(index.byLegacyKey) == 0) {
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

		key := normalizeSourceKey(nodes[i].SourceKey)
		if key == "" {
			continue
		}

		existingID := strings.TrimSpace(index.bySourceKey[key])
		if existingID == "" {
			existingID = strings.TrimSpace(index.byLegacyKey[key])
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

	existingByKey := buildUniqueIDIndex(existing, func(n domain.Node) string {
		if normalizeSourceKey(n.SourceKey) != "" {
			return normalizeSourceKey(n.SourceKey)
		}
		return deriveSubscriptionKey(n)
	})
	nextByKey := buildUniqueIDIndex(next, func(n domain.Node) string {
		if normalizeSourceKey(n.SourceKey) != "" {
			return normalizeSourceKey(n.SourceKey)
		}
		return deriveSubscriptionKey(n)
	})

	out := make(map[string]string, 8)
	for key, oldID := range existingByKey {
		newID := nextByKey[key]
		if newID == "" || newID == oldID {
			continue
		}
		out[oldID] = newID
	}
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
