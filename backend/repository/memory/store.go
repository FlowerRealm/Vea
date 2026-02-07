package memory

import (
	"sort"
	"sync"
	"time"

	"vea/backend/domain"
	"vea/backend/repository/events"

	"github.com/google/uuid"
)

// Store 内存存储引擎
type Store struct {
	mu sync.RWMutex

	// 数据存储
	nodes      map[string]domain.Node
	nodeGroups map[string]domain.NodeGroup
	frouters   map[string]domain.FRouter
	configs    map[string]domain.Config
	geo        map[string]domain.GeoResource
	components map[string]domain.CoreComponent

	// 单例设置
	systemProxy      domain.SystemProxySettings
	proxyConfig      domain.ProxyConfig
	frontendSettings map[string]interface{}

	// 事件总线
	eventBus *events.Bus
}

// NewStore 创建新的内存存储
func NewStore(eventBus *events.Bus) *Store {
	s := &Store{
		nodes:      make(map[string]domain.Node),
		nodeGroups: make(map[string]domain.NodeGroup),
		frouters:   make(map[string]domain.FRouter),
		configs:    make(map[string]domain.Config),
		geo:        make(map[string]domain.GeoResource),
		components: make(map[string]domain.CoreComponent),
		eventBus:   eventBus,
	}
	// 初始化默认设置
	s.systemProxy = domain.SystemProxySettings{
		IgnoreHosts: []string{"127.0.0.0/8", "::1", "localhost"},
	}
	s.proxyConfig = domain.ProxyConfig{
		InboundMode:     domain.InboundMixed,
		InboundPort:     31346,
		PreferredEngine: domain.EngineAuto,
	}
	return s
}

// ========== 锁操作（供仓储使用）==========

// RLock 获取读锁
func (s *Store) RLock() { s.mu.RLock() }

// RUnlock 释放读锁
func (s *Store) RUnlock() { s.mu.RUnlock() }

// Lock 获取写锁
func (s *Store) Lock() { s.mu.Lock() }

// Unlock 释放写锁
func (s *Store) Unlock() { s.mu.Unlock() }

// ========== 事件发布 ==========

// PublishEvent 发布事件（异步，应在锁外调用）
func (s *Store) PublishEvent(event events.Event) {
	if s.eventBus != nil {
		s.eventBus.Publish(event)
	}
}

// PublishEventSync 发布事件（同步，应在锁外调用）
func (s *Store) PublishEventSync(event events.Event) {
	if s.eventBus != nil {
		s.eventBus.PublishSync(event)
	}
}

// ========== 数据访问（供仓储内部使用）==========

// Nodes 返回节点映射（需持有锁）
func (s *Store) Nodes() map[string]domain.Node { return s.nodes }

// NodeGroups 返回节点组映射（需持有锁）
func (s *Store) NodeGroups() map[string]domain.NodeGroup { return s.nodeGroups }

// FRouters 返回 FRouter 映射（需持有锁）
func (s *Store) FRouters() map[string]domain.FRouter { return s.frouters }

// Configs 返回配置映射（需持有锁）
func (s *Store) Configs() map[string]domain.Config { return s.configs }

// Geo 返回 Geo 资源映射（需持有锁）
func (s *Store) Geo() map[string]domain.GeoResource { return s.geo }

// Components 返回组件映射（需持有锁）
func (s *Store) Components() map[string]domain.CoreComponent { return s.components }

// ========== 单例设置访问 ==========

// GetSystemProxy 获取系统代理设置（需持有锁）
func (s *Store) GetSystemProxy() domain.SystemProxySettings { return s.systemProxy }

// SetSystemProxy 设置系统代理设置（需持有锁）
func (s *Store) SetSystemProxy(settings domain.SystemProxySettings) { s.systemProxy = settings }

// GetProxyConfig 获取代理运行配置（需持有锁）
func (s *Store) GetProxyConfig() domain.ProxyConfig { return s.proxyConfig }

// SetProxyConfig 设置代理运行配置（需持有锁）
func (s *Store) SetProxyConfig(config domain.ProxyConfig) { s.proxyConfig = config }

// GetFrontendSettings 获取前端设置（需持有锁）
func (s *Store) GetFrontendSettings() map[string]interface{} {
	if s.frontendSettings == nil {
		return make(map[string]interface{})
	}
	return cloneFrontendSettings(s.frontendSettings)
}

// SetFrontendSettings 设置前端设置（需持有锁）
func (s *Store) SetFrontendSettings(settings map[string]interface{}) {
	if settings == nil {
		s.frontendSettings = nil
		return
	}
	s.frontendSettings = cloneFrontendSettings(settings)
}

// ========== 快照与恢复 ==========

// Snapshot 生成状态快照
func (s *Store) Snapshot() domain.ServiceState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 复制节点
	nodes := make([]domain.Node, 0, len(s.nodes))
	for _, node := range s.nodes {
		nodes = append(nodes, stripNodeMetrics(node))
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Name == nodes[j].Name {
			return nodes[i].CreatedAt.Before(nodes[j].CreatedAt)
		}
		return nodes[i].Name < nodes[j].Name
	})

	// 复制 NodeGroup
	nodeGroups := make([]domain.NodeGroup, 0, len(s.nodeGroups))
	for _, g := range s.nodeGroups {
		nodeGroups = append(nodeGroups, g)
	}
	sort.Slice(nodeGroups, func(i, j int) bool {
		if nodeGroups[i].Name == nodeGroups[j].Name {
			return nodeGroups[i].CreatedAt.Before(nodeGroups[j].CreatedAt)
		}
		return nodeGroups[i].Name < nodeGroups[j].Name
	})

	// 复制 FRouter
	frouters := make([]domain.FRouter, 0, len(s.frouters))
	for _, frouter := range s.frouters {
		frouters = append(frouters, stripFRouterMetrics(frouter))
	}
	sort.Slice(frouters, func(i, j int) bool {
		if frouters[i].Name == frouters[j].Name {
			return frouters[i].CreatedAt.Before(frouters[j].CreatedAt)
		}
		return frouters[i].Name < frouters[j].Name
	})

	// 复制配置
	configs := make([]domain.Config, 0, len(s.configs))
	for _, cfg := range s.configs {
		configs = append(configs, cfg)
	}
	sort.Slice(configs, func(i, j int) bool {
		return configs[i].CreatedAt.Before(configs[j].CreatedAt)
	})

	// 复制 Geo 资源
	geoResources := make([]domain.GeoResource, 0, len(s.geo))
	for _, g := range s.geo {
		geoResources = append(geoResources, g)
	}
	sort.Slice(geoResources, func(i, j int) bool {
		return geoResources[i].CreatedAt.Before(geoResources[j].CreatedAt)
	})

	// 复制组件
	components := make([]domain.CoreComponent, 0, len(s.components))
	for _, c := range s.components {
		components = append(components, stripComponentRuntime(c))
	}
	sort.Slice(components, func(i, j int) bool {
		return components[i].CreatedAt.Before(components[j].CreatedAt)
	})

	return domain.ServiceState{
		Nodes:            nodes,
		NodeGroups:       nodeGroups,
		FRouters:         frouters,
		Configs:          configs,
		GeoResources:     geoResources,
		Components:       components,
		SystemProxy:      s.systemProxy,
		ProxyConfig:      s.proxyConfig,
		FrontendSettings: cloneFrontendSettings(s.frontendSettings),
		GeneratedAt:      time.Now(),
	}
}

// LoadState 加载状态
func (s *Store) LoadState(state domain.ServiceState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	// 加载节点
	s.nodes = make(map[string]domain.Node)
	for _, node := range state.Nodes {
		node = stripNodeMetrics(node)
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

	// 加载 NodeGroup
	s.nodeGroups = make(map[string]domain.NodeGroup)
	for _, group := range state.NodeGroups {
		if group.ID == "" {
			group.ID = uuid.NewString()
		}
		if group.CreatedAt.IsZero() {
			group.CreatedAt = now
		}
		if group.UpdatedAt.IsZero() {
			group.UpdatedAt = group.CreatedAt
		}
		if group.NodeIDs == nil {
			group.NodeIDs = []string{}
		}
		s.nodeGroups[group.ID] = group
	}

	// 加载 FRouter
	s.frouters = make(map[string]domain.FRouter)
	for _, frouter := range state.FRouters {
		frouter = stripFRouterMetrics(frouter)
		if frouter.ID == "" {
			frouter.ID = uuid.NewString()
		}
		if frouter.CreatedAt.IsZero() {
			frouter.CreatedAt = now
		}
		if frouter.UpdatedAt.IsZero() {
			frouter.UpdatedAt = frouter.CreatedAt
		}
		if frouter.ChainProxy.Edges == nil {
			frouter.ChainProxy.Edges = []domain.ProxyEdge{}
		}
		if frouter.ChainProxy.Positions == nil {
			frouter.ChainProxy.Positions = make(map[string]domain.GraphPosition)
		}
		if frouter.ChainProxy.Slots == nil {
			frouter.ChainProxy.Slots = []domain.SlotNode{}
		}
		if frouter.ChainProxy.UpdatedAt.IsZero() {
			frouter.ChainProxy.UpdatedAt = now
		}
		s.frouters[frouter.ID] = frouter
	}

	// 加载配置
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
		s.configs[cfg.ID] = cfg
	}

	// 加载 Geo 资源
	s.geo = make(map[string]domain.GeoResource)
	for _, g := range state.GeoResources {
		if g.ID == "" {
			g.ID = uuid.NewString()
		}
		if g.CreatedAt.IsZero() {
			g.CreatedAt = now
		}
		if g.UpdatedAt.IsZero() {
			g.UpdatedAt = g.CreatedAt
		}
		s.geo[g.ID] = g
	}

	// 加载组件
	s.components = make(map[string]domain.CoreComponent)
	for _, c := range state.Components {
		c = stripComponentRuntime(c)
		if c.ID == "" {
			c.ID = uuid.NewString()
		}
		if c.CreatedAt.IsZero() {
			c.CreatedAt = now
		}
		if c.UpdatedAt.IsZero() {
			c.UpdatedAt = c.CreatedAt
		}
		s.components[c.ID] = c
	}

	s.systemProxy = state.SystemProxy
	if len(s.systemProxy.IgnoreHosts) == 0 {
		s.systemProxy.IgnoreHosts = []string{"127.0.0.0/8", "::1", "localhost"}
	}

	s.proxyConfig = state.ProxyConfig
	if s.proxyConfig.InboundMode == "" {
		s.proxyConfig.InboundMode = domain.InboundMixed
	}
	if s.proxyConfig.InboundPort == 0 && s.proxyConfig.InboundMode != domain.InboundTUN {
		s.proxyConfig.InboundPort = 31346
	}
	if s.proxyConfig.PreferredEngine == "" {
		s.proxyConfig.PreferredEngine = domain.EngineAuto
	}
	if s.proxyConfig.UpdatedAt.IsZero() {
		s.proxyConfig.UpdatedAt = now
	}

	s.frontendSettings = cloneFrontendSettings(state.FrontendSettings)

}

func stripNodeMetrics(node domain.Node) domain.Node {
	node.LastLatencyMS = 0
	node.LastLatencyAt = time.Time{}
	node.LastLatencyError = ""
	node.LastSpeedMbps = 0
	node.LastSpeedAt = time.Time{}
	node.LastSpeedError = ""
	return node
}

func stripComponentRuntime(comp domain.CoreComponent) domain.CoreComponent {
	comp.InstallStatus = ""
	comp.InstallProgress = 0
	comp.InstallMessage = ""
	return comp
}

func stripFRouterMetrics(frouter domain.FRouter) domain.FRouter {
	frouter.LastLatencyMS = 0
	frouter.LastLatencyAt = time.Time{}
	frouter.LastLatencyError = ""
	frouter.LastSpeedMbps = 0
	frouter.LastSpeedAt = time.Time{}
	frouter.LastSpeedError = ""
	return frouter
}

func cloneFrontendSettings(in map[string]interface{}) map[string]interface{} {
	if in == nil {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = cloneFrontendValue(v)
	}
	return out
}

func cloneFrontendValue(v interface{}) interface{} {
	switch x := v.(type) {
	case map[string]interface{}:
		return cloneFrontendSettings(x)
	case []interface{}:
		out := make([]interface{}, 0, len(x))
		for _, item := range x {
			out = append(out, cloneFrontendValue(item))
		}
		return out
	case map[string]string:
		out := make(map[string]string, len(x))
		for k, v := range x {
			out[k] = v
		}
		return out
	case []string:
		out := make([]string, len(x))
		copy(out, x)
		return out
	default:
		return v
	}
}
