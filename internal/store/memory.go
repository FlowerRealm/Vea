package store

import (
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"vea/internal/domain"
)

var (
	errNodeNotFound   = errors.New("node not found")
	errConfigNotFound = errors.New("config not found")
	errGeoNotFound    = errors.New("geo resource not found")
	errRuleNotFound   = errors.New("traffic rule not found")
)

type MemoryStore struct {
	mu             sync.RWMutex
	nodes          map[string]domain.Node
	configs        map[string]domain.Config
	geo            map[string]domain.GeoResource
	rules          map[string]domain.TrafficRule
	trafficProfile domain.TrafficProfile
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		nodes:   make(map[string]domain.Node),
		configs: make(map[string]domain.Config),
		geo:     make(map[string]domain.GeoResource),
		rules:   make(map[string]domain.TrafficRule),
		trafficProfile: domain.TrafficProfile{
			DNS:       domain.DNSSetting{Strategy: "ipv4-only", Servers: []string{"8.8.8.8"}},
			Rules:     []domain.TrafficRule{},
			UpdatedAt: time.Now(),
		},
	}
}

func (s *MemoryStore) ListNodes() []domain.Node {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.Node, 0, len(s.nodes))
	for _, node := range s.nodes {
		items = append(items, node)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items
}

func (s *MemoryStore) CreateNode(node domain.Node) domain.Node {
	s.mu.Lock()
	defer s.mu.Unlock()
	if node.ID == "" {
		node.ID = uuid.NewString()
	}
	node.CreatedAt = time.Now()
	node.UpdatedAt = node.CreatedAt
	s.nodes[node.ID] = node
	return node
}

func (s *MemoryStore) UpdateNode(id string, updateFn func(domain.Node) (domain.Node, error)) (domain.Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	node, ok := s.nodes[id]
	if !ok {
		return domain.Node{}, errNodeNotFound
	}
	updated, err := updateFn(node)
	if err != nil {
		return domain.Node{}, err
	}
	updated.ID = id
	updated.CreatedAt = node.CreatedAt
	updated.UpdatedAt = time.Now()
	s.nodes[id] = updated
	return updated, nil
}

func (s *MemoryStore) DeleteNode(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.nodes[id]; !ok {
		return errNodeNotFound
	}
	delete(s.nodes, id)
	if s.trafficProfile.DefaultNodeID == id {
		s.trafficProfile.DefaultNodeID = ""
	}
	return nil
}

func (s *MemoryStore) IncrementNodeTraffic(id string, up, down int64) (domain.Node, error) {
	return s.UpdateNode(id, func(node domain.Node) (domain.Node, error) {
		node.UploadBytes += up
		node.DownloadBytes += down
		return node, nil
	})
}

func (s *MemoryStore) ResetNodeTraffic(id string) (domain.Node, error) {
	return s.UpdateNode(id, func(node domain.Node) (domain.Node, error) {
		node.UploadBytes = 0
		node.DownloadBytes = 0
		return node, nil
	})
}

func (s *MemoryStore) ListConfigs() []domain.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.Config, 0, len(s.configs))
	for _, cfg := range s.configs {
		items = append(items, cfg)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items
}

func (s *MemoryStore) CreateConfig(cfg domain.Config) domain.Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cfg.ID == "" {
		cfg.ID = uuid.NewString()
	}
	now := time.Now()
	cfg.CreatedAt = now
	cfg.UpdatedAt = now
	cfg.LastSyncedAt = now
	s.configs[cfg.ID] = cfg
	return cfg
}

func (s *MemoryStore) UpdateConfig(id string, updateFn func(domain.Config) (domain.Config, error)) (domain.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg, ok := s.configs[id]
	if !ok {
		return domain.Config{}, errConfigNotFound
	}
	updated, err := updateFn(cfg)
	if err != nil {
		return domain.Config{}, err
	}
	updated.ID = id
	updated.CreatedAt = cfg.CreatedAt
	updated.UpdatedAt = time.Now()
	if updated.LastSyncedAt.IsZero() {
		updated.LastSyncedAt = cfg.LastSyncedAt
	}
	s.configs[id] = updated
	return updated, nil
}

func (s *MemoryStore) DeleteConfig(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.configs[id]; !ok {
		return errConfigNotFound
	}
	delete(s.configs, id)
	return nil
}

func (s *MemoryStore) IncrementConfigTraffic(id string, up, down int64) (domain.Config, error) {
	return s.UpdateConfig(id, func(cfg domain.Config) (domain.Config, error) {
		cfg.UploadBytes += up
		cfg.DownloadBytes += down
		return cfg, nil
	})
}

func (s *MemoryStore) ListGeo() []domain.GeoResource {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.GeoResource, 0, len(s.geo))
	for _, res := range s.geo {
		items = append(items, res)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items
}

func (s *MemoryStore) GetGeo(id string) (domain.GeoResource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res, ok := s.geo[id]
	if !ok {
		return domain.GeoResource{}, errGeoNotFound
	}
	return res, nil
}

func (s *MemoryStore) UpsertGeo(res domain.GeoResource) domain.GeoResource {
	s.mu.Lock()
	defer s.mu.Unlock()
	if res.ID == "" {
		res.ID = uuid.NewString()
	}
	now := time.Now()
	if existing, ok := s.geo[res.ID]; ok {
		res.CreatedAt = existing.CreatedAt
	} else {
		res.CreatedAt = now
	}
	if res.LastSynced.IsZero() {
		res.LastSynced = now
	}
	res.UpdatedAt = now
	s.geo[res.ID] = res
	return res
}

func (s *MemoryStore) DeleteGeo(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.geo[id]; !ok {
		return errGeoNotFound
	}
	delete(s.geo, id)
	return nil
}

func (s *MemoryStore) ListTrafficRules() []domain.TrafficRule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.collectRulesLocked()
}

func (s *MemoryStore) CreateTrafficRule(rule domain.TrafficRule) domain.TrafficRule {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rule.ID == "" {
		rule.ID = uuid.NewString()
	}
	now := time.Now()
	rule.CreatedAt = now
	rule.UpdatedAt = now
	s.rules[rule.ID] = rule
	s.rebuildTrafficProfileLocked()
	return rule
}

func (s *MemoryStore) UpdateTrafficRule(id string, updateFn func(domain.TrafficRule) (domain.TrafficRule, error)) (domain.TrafficRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rule, ok := s.rules[id]
	if !ok {
		return domain.TrafficRule{}, errRuleNotFound
	}
	updated, err := updateFn(rule)
	if err != nil {
		return domain.TrafficRule{}, err
	}
	updated.ID = id
	updated.CreatedAt = rule.CreatedAt
	updated.UpdatedAt = time.Now()
	s.rules[id] = updated
	s.rebuildTrafficProfileLocked()
	return updated, nil
}

func (s *MemoryStore) DeleteTrafficRule(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.rules[id]; !ok {
		return errRuleNotFound
	}
	delete(s.rules, id)
	s.rebuildTrafficProfileLocked()
	return nil
}

func (s *MemoryStore) UpdateTrafficProfile(mutator func(domain.TrafficProfile) (domain.TrafficProfile, error)) (domain.TrafficProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	profile, err := mutator(s.trafficProfile)
	if err != nil {
		return domain.TrafficProfile{}, err
	}
	profile.UpdatedAt = time.Now()
	s.trafficProfile = profile
	s.rebuildTrafficProfileLocked()
	return s.trafficProfile, nil
}

func (s *MemoryStore) GetTrafficProfile() domain.TrafficProfile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	profile := s.trafficProfile
	profile.Rules = append([]domain.TrafficRule(nil), s.collectRulesLocked()...)
	return profile
}

func (s *MemoryStore) rebuildTrafficProfileLocked() {
	s.trafficProfile.Rules = s.collectRulesLocked()
	if s.trafficProfile.UpdatedAt.IsZero() {
		s.trafficProfile.UpdatedAt = time.Now()
	}
}

func (s *MemoryStore) Snapshot() domain.ServiceState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	nodes := make([]domain.Node, 0, len(s.nodes))
	for _, node := range s.nodes {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].CreatedAt.Before(nodes[j].CreatedAt) })

	configs := make([]domain.Config, 0, len(s.configs))
	for _, cfg := range s.configs {
		configs = append(configs, cfg)
	}
	sort.Slice(configs, func(i, j int) bool { return configs[i].CreatedAt.Before(configs[j].CreatedAt) })

	geo := make([]domain.GeoResource, 0, len(s.geo))
	for _, res := range s.geo {
		geo = append(geo, res)
	}
	sort.Slice(geo, func(i, j int) bool { return geo[i].CreatedAt.Before(geo[j].CreatedAt) })

	profile := s.trafficProfile
	profile.Rules = append([]domain.TrafficRule(nil), profile.Rules...)

	return domain.ServiceState{
		Nodes:          nodes,
		Configs:        configs,
		GeoResources:   geo,
		TrafficProfile: profile,
		GeneratedAt:    time.Now(),
	}
}

func (s *MemoryStore) collectRulesLocked() []domain.TrafficRule {
	rules := make([]domain.TrafficRule, 0, len(s.rules))
	for _, rule := range s.rules {
		rules = append(rules, rule)
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].Priority > rules[j].Priority })
	return rules
}

func (s *MemoryStore) Errors() (error, error, error, error) {
	return errNodeNotFound, errConfigNotFound, errGeoNotFound, errRuleNotFound
}
