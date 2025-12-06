package store

import (
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"vea/backend/domain"
)

var (
	errNodeNotFound      = errors.New("node not found")
	errConfigNotFound    = errors.New("config not found")
	errGeoNotFound       = errors.New("geo resource not found")
	errRuleNotFound      = errors.New("traffic rule not found")
	errComponentNotFound = errors.New("core component not found")
	errProfileNotFound = errors.New("proxy profile not found")
)

type MemoryStore struct {
	mu             sync.RWMutex
	nodes          map[string]domain.Node
	configs        map[string]domain.Config
	geo            map[string]domain.GeoResource
	rules          map[string]domain.TrafficRule
	components     map[string]domain.CoreComponent
	proxyProfiles  map[string]domain.ProxyProfile
	activeProfile  string
	trafficProfile domain.TrafficProfile
	systemProxy    domain.SystemProxySettings
	tunSettings    domain.TUNSettings
	afterMu        sync.RWMutex
	afterWrite     func()
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		nodes:         make(map[string]domain.Node),
		configs:       make(map[string]domain.Config),
		geo:           make(map[string]domain.GeoResource),
		rules:         make(map[string]domain.TrafficRule),
		components:    make(map[string]domain.CoreComponent),
		proxyProfiles: make(map[string]domain.ProxyProfile),
		trafficProfile: domain.TrafficProfile{
			DNS:       domain.DNSSetting{Strategy: "ipv4-only", Servers: []string{"8.8.8.8"}},
			Rules:     []domain.TrafficRule{},
			UpdatedAt: time.Now(),
		},
		systemProxy: defaultSystemProxySettings(),
		tunSettings: domain.TUNSettings{
			Enabled:   false,
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
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].CreatedAt.Before(items[j].CreatedAt)
		}
		return items[i].Name < items[j].Name
	})
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
	s.fireAfterWrite()
	return node
}

func (s *MemoryStore) GetNode(id string) (domain.Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	node, ok := s.nodes[id]
	if !ok {
		return domain.Node{}, errNodeNotFound
	}
	return node, nil
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
	if updated.SourceConfigID == "" {
		updated.SourceConfigID = node.SourceConfigID
	}
	s.nodes[id] = updated
	s.fireAfterWrite()
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
	s.fireAfterWrite()
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

func (s *MemoryStore) ListNodesByConfig(configID string) []domain.Node {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.Node, 0)
	for _, node := range s.nodes {
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
	return items
}

func (s *MemoryStore) ReplaceNodesForConfig(configID string, nodes []domain.Node) []domain.Node {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removeNodesForConfigLocked(configID)
	created := make([]domain.Node, 0, len(nodes))
	now := time.Now()
	for _, node := range nodes {
		if node.ID == "" {
			node.ID = uuid.NewString()
		}
		node.SourceConfigID = configID
		node.CreatedAt = now
		node.UpdatedAt = now
		s.nodes[node.ID] = node
		created = append(created, node)
	}
	s.fireAfterWrite()
	return created
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

func (s *MemoryStore) GetConfig(id string) (domain.Config, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg, ok := s.configs[id]
	if !ok {
		return domain.Config{}, errConfigNotFound
	}
	return cfg, nil
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
	s.fireAfterWrite()
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
	s.fireAfterWrite()
	return updated, nil
}

func (s *MemoryStore) DeleteConfig(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.configs[id]; !ok {
		return errConfigNotFound
	}
	s.removeNodesForConfigLocked(id)
	delete(s.configs, id)
	s.fireAfterWrite()
	return nil
}

func (s *MemoryStore) CleanupOrphanNodes() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	removed := s.cleanupOrphanNodesLocked()
	if removed > 0 {
		s.fireAfterWrite()
	}
	return removed
}

func (s *MemoryStore) removeNodesForConfigLocked(configID string) {
	if configID == "" {
		return
	}
	for id, node := range s.nodes {
		if node.SourceConfigID != configID {
			continue
		}
		delete(s.nodes, id)
		if s.trafficProfile.DefaultNodeID == id {
			s.trafficProfile.DefaultNodeID = ""
		}
	}
}

func (s *MemoryStore) cleanupOrphanNodesLocked() int {
	removed := 0
	for id, node := range s.nodes {
		if node.SourceConfigID == "" {
			continue
		}
		if _, ok := s.configs[node.SourceConfigID]; ok {
			continue
		}
		delete(s.nodes, id)
		if s.trafficProfile.DefaultNodeID == id {
			s.trafficProfile.DefaultNodeID = ""
		}
		removed++
	}
	return removed
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
	s.fireAfterWrite()
	return res
}

func (s *MemoryStore) DeleteGeo(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.geo[id]; !ok {
		return errGeoNotFound
	}
	delete(s.geo, id)
	s.fireAfterWrite()
	return nil
}

func (s *MemoryStore) ListComponents() []domain.CoreComponent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.CoreComponent, 0, len(s.components))
	for _, comp := range s.components {
		items = append(items, comp)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items
}

func (s *MemoryStore) GetComponent(id string) (domain.CoreComponent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	comp, ok := s.components[id]
	if !ok {
		return domain.CoreComponent{}, errComponentNotFound
	}
	return comp, nil
}

func (s *MemoryStore) CreateComponent(comp domain.CoreComponent) domain.CoreComponent {
	s.mu.Lock()
	defer s.mu.Unlock()
	if comp.ID == "" {
		comp.ID = uuid.NewString()
	}
	now := time.Now()
	comp.CreatedAt = now
	comp.UpdatedAt = now
	s.components[comp.ID] = comp
	return comp
}

func (s *MemoryStore) UpdateComponent(id string, updateFn func(domain.CoreComponent) (domain.CoreComponent, error)) (domain.CoreComponent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	comp, ok := s.components[id]
	if !ok {
		return domain.CoreComponent{}, errComponentNotFound
	}
	updated, err := updateFn(comp)
	if err != nil {
		return domain.CoreComponent{}, err
	}
	updated.ID = id
	updated.CreatedAt = comp.CreatedAt
	updated.UpdatedAt = time.Now()
	if updated.LastInstalledAt.IsZero() {
		updated.LastInstalledAt = comp.LastInstalledAt
	}
	if updated.Checksum == "" {
		updated.Checksum = comp.Checksum
	}
	if updated.InstallDir == "" {
		updated.InstallDir = comp.InstallDir
	}
	if updated.Meta == nil && comp.Meta != nil {
		metaCopy := make(map[string]string, len(comp.Meta))
		for k, v := range comp.Meta {
			metaCopy[k] = v
		}
		updated.Meta = metaCopy
	}
	clearLastSync := false
	if updated.Meta != nil {
		if updated.Meta["_clearLastSyncError"] != "" {
			clearLastSync = true
			delete(updated.Meta, "_clearLastSyncError")
		}
	}
	if clearLastSync {
		updated.LastSyncError = ""
	}
	if updated.LastVersion == "" {
		updated.LastVersion = comp.LastVersion
	}
	if updated.LastSyncError == "" && comp.LastSyncError != "" && !clearLastSync {
		updated.LastSyncError = comp.LastSyncError
	}
	s.components[id] = updated
	return updated, nil
}

func (s *MemoryStore) DeleteComponent(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.components[id]; !ok {
		return errComponentNotFound
	}
	delete(s.components, id)
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
	s.fireAfterWrite()
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
	s.fireAfterWrite()
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
	s.fireAfterWrite()
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
	s.fireAfterWrite()
	return s.trafficProfile, nil
}

func (s *MemoryStore) GetTrafficProfile() domain.TrafficProfile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	profile := s.trafficProfile
	profile.Rules = append([]domain.TrafficRule(nil), s.collectRulesLocked()...)
	return profile
}

func (s *MemoryStore) GetSystemProxySettings() domain.SystemProxySettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.systemProxy
}

func (s *MemoryStore) UpdateSystemProxySettings(updateFn func(domain.SystemProxySettings) (domain.SystemProxySettings, error)) (domain.SystemProxySettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	updated, err := updateFn(s.systemProxy)
	if err != nil {
		return domain.SystemProxySettings{}, err
	}
	if updated.UpdatedAt.IsZero() {
		updated.UpdatedAt = time.Now()
	}
	if updated.IgnoreHosts == nil {
		updated.IgnoreHosts = []string{}
	}
	updated.IgnoreHosts = append([]string(nil), updated.IgnoreHosts...)
	s.systemProxy = updated
	s.fireAfterWrite()
	return s.systemProxy, nil
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

	components := make([]domain.CoreComponent, 0, len(s.components))
	for _, comp := range s.components {
		components = append(components, comp)
	}
	sort.Slice(components, func(i, j int) bool { return components[i].CreatedAt.Before(components[j].CreatedAt) })

	profile := s.trafficProfile
	profile.Rules = append([]domain.TrafficRule(nil), profile.Rules...)

	profiles := make([]domain.ProxyProfile, 0, len(s.proxyProfiles))
	for _, p := range s.proxyProfiles {
		profiles = append(profiles, p)
	}

	return domain.ServiceState{
		Nodes:          nodes,
		Configs:        configs,
		GeoResources:   geo,
		Components:     components,
		ProxyProfiles:  profiles,
		ActiveProfile:  s.activeProfile,
		TrafficProfile: profile,
		SystemProxy:    s.systemProxy,
		TUNSettings:    s.tunSettings,
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

func (s *MemoryStore) LoadState(state domain.ServiceState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	s.nodes = make(map[string]domain.Node)
	for _, node := range state.Nodes {
		if node.ID == "" {
			node.ID = uuid.NewString()
		}
		if node.CreatedAt.IsZero() {
			node.CreatedAt = now
		}
		if node.UpdatedAt.IsZero() {
			node.UpdatedAt = node.CreatedAt
		}
		s.nodes[node.ID] = node
	}

	s.configs = make(map[string]domain.Config)
	for _, cfg := range state.Configs {
		if cfg.ID == "" {
			cfg.ID = uuid.NewString()
		}
		if cfg.CreatedAt.IsZero() {
			cfg.CreatedAt = now
		}
		if cfg.UpdatedAt.IsZero() {
			cfg.UpdatedAt = cfg.CreatedAt
		}
		if cfg.LastSyncedAt.IsZero() {
			cfg.LastSyncedAt = cfg.UpdatedAt
		}
		s.configs[cfg.ID] = cfg
	}

	s.geo = make(map[string]domain.GeoResource)
	for _, res := range state.GeoResources {
		if res.ID == "" {
			res.ID = uuid.NewString()
		}
		if res.CreatedAt.IsZero() {
			res.CreatedAt = now
		}
		if res.UpdatedAt.IsZero() {
			res.UpdatedAt = res.CreatedAt
		}
		if res.LastSynced.IsZero() {
			res.LastSynced = res.UpdatedAt
		}
		s.geo[res.ID] = res
	}

	s.components = make(map[string]domain.CoreComponent)
	for _, comp := range state.Components {
		if comp.ID == "" {
			comp.ID = uuid.NewString()
		}
		if comp.CreatedAt.IsZero() {
			comp.CreatedAt = now
		}
		if comp.UpdatedAt.IsZero() {
			comp.UpdatedAt = comp.CreatedAt
		}
		s.components[comp.ID] = comp
	}

	s.proxyProfiles = make(map[string]domain.ProxyProfile)
	for _, profile := range state.ProxyProfiles {
		if profile.ID == "" {
			profile.ID = uuid.NewString()
		}
		if profile.CreatedAt.IsZero() {
			profile.CreatedAt = now
		}
		if profile.UpdatedAt.IsZero() {
			profile.UpdatedAt = profile.CreatedAt
		}
		s.proxyProfiles[profile.ID] = profile
	}


	s.activeProfile = state.ActiveProfile

	if state.TrafficProfile.UpdatedAt.IsZero() {
		state.TrafficProfile.UpdatedAt = now
	}
	s.trafficProfile = state.TrafficProfile
	defaults := defaultSystemProxySettings()
	if len(state.SystemProxy.IgnoreHosts) == 0 {
		state.SystemProxy.IgnoreHosts = append([]string{}, defaults.IgnoreHosts...)
	}
	if state.SystemProxy.UpdatedAt.IsZero() {
		state.SystemProxy.UpdatedAt = now
	}
	state.SystemProxy.IgnoreHosts = append([]string(nil), state.SystemProxy.IgnoreHosts...)
	if !state.SystemProxy.Enabled && equalStringSlices(state.SystemProxy.IgnoreHosts, defaults.IgnoreHosts) {
		s.systemProxy = defaults
	} else {
		s.systemProxy = state.SystemProxy
	}

	// Load TUN settings
	if state.TUNSettings.UpdatedAt.IsZero() {
		state.TUNSettings.UpdatedAt = now
	}
	// 强制 TUN 在启动时默认关闭（安全考虑）
	// state.TUNSettings.Enabled = false
	s.tunSettings = state.TUNSettings

	s.cleanupOrphanNodesLocked()
}

func defaultSystemProxySettings() domain.SystemProxySettings {
	defaults := []string{"127.0.0.0/8", "::1", "localhost"}
	sort.Strings(defaults)
	return domain.SystemProxySettings{
		Enabled:     false,
		IgnoreHosts: defaults,
		UpdatedAt:   time.Now(),
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (s *MemoryStore) Errors() (error, error, error, error, error) {
	return errNodeNotFound, errConfigNotFound, errGeoNotFound, errRuleNotFound, errComponentNotFound
}

func (s *MemoryStore) SetAfterWrite(cb func()) {
	s.afterMu.Lock()
	s.afterWrite = cb
	s.afterMu.Unlock()
}

func (s *MemoryStore) fireAfterWrite() {
	s.afterMu.RLock()
	cb := s.afterWrite
	s.afterMu.RUnlock()
	if cb != nil {
		cb()
	}
}

// ProxyProfile CRUD methods

func (s *MemoryStore) ListProxyProfiles() []domain.ProxyProfile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]domain.ProxyProfile, 0, len(s.proxyProfiles))
	for _, profile := range s.proxyProfiles {
		items = append(items, profile)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
	return items
}

func (s *MemoryStore) CreateProxyProfile(profile domain.ProxyProfile) domain.ProxyProfile {
	s.mu.Lock()
	defer s.mu.Unlock()
	if profile.ID == "" {
		profile.ID = uuid.NewString()
	}
	profile.CreatedAt = time.Now()
	profile.UpdatedAt = profile.CreatedAt
	s.proxyProfiles[profile.ID] = profile
	s.fireAfterWrite()
	return profile
}

func (s *MemoryStore) GetProxyProfile(id string) (domain.ProxyProfile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	profile, ok := s.proxyProfiles[id]
	if !ok {
		return domain.ProxyProfile{}, errProfileNotFound
	}
	return profile, nil
}

func (s *MemoryStore) UpdateProxyProfile(id string, updateFn func(domain.ProxyProfile) (domain.ProxyProfile, error)) (domain.ProxyProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	profile, ok := s.proxyProfiles[id]
	if !ok {
		return domain.ProxyProfile{}, errProfileNotFound
	}
	updated, err := updateFn(profile)
	if err != nil {
		return domain.ProxyProfile{}, err
	}
	updated.UpdatedAt = time.Now()
	s.proxyProfiles[id] = updated
	s.fireAfterWrite()
	return updated, nil
}

func (s *MemoryStore) DeleteProxyProfile(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.proxyProfiles[id]; !ok {
		return errProfileNotFound
	}
	delete(s.proxyProfiles, id)
	// 如果删除的是活跃 Profile，清空活跃状态
	if s.activeProfile == id {
		s.activeProfile = ""
	}
	s.fireAfterWrite()
	return nil
}

func (s *MemoryStore) GetActiveProfile() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeProfile
}

func (s *MemoryStore) SetActiveProfile(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// 验证 Profile 存在
	if id != "" {
		if _, ok := s.proxyProfiles[id]; !ok {
			return errProfileNotFound
		}
	}
	s.activeProfile = id
	s.fireAfterWrite()
	return nil
}

// TUN Settings
func (s *MemoryStore) GetTUNSettings() domain.TUNSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tunSettings
}

func (s *MemoryStore) UpdateTUNSettings(updateFn func(domain.TUNSettings) (domain.TUNSettings, error)) (domain.TUNSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	updated, err := updateFn(s.tunSettings)
	if err != nil {
		return domain.TUNSettings{}, err
	}
	updated.UpdatedAt = time.Now()
	s.tunSettings = updated
	s.fireAfterWrite()
	return s.tunSettings, nil
}

