package repository

import (
	"context"

	"vea/backend/domain"
)

// FRouterRepository FRouter 仓储接口（主要对外操作单元）
type FRouterRepository interface {
	// 基础 CRUD
	Get(ctx context.Context, id string) (domain.FRouter, error)
	List(ctx context.Context) ([]domain.FRouter, error)
	Create(ctx context.Context, frouter domain.FRouter) (domain.FRouter, error)
	Update(ctx context.Context, id string, frouter domain.FRouter) (domain.FRouter, error)
	Delete(ctx context.Context, id string) error

	// 延迟/速度更新
	UpdateLatency(ctx context.Context, id string, latencyMS int64, latencyErr string) error
	UpdateSpeed(ctx context.Context, id string, speedMbps float64, speedErr string) error
}

// NodeRepository 节点仓储接口（独立实体）
type NodeRepository interface {
	// 基础 CRUD
	Get(ctx context.Context, id string) (domain.Node, error)
	List(ctx context.Context) ([]domain.Node, error)
	Create(ctx context.Context, node domain.Node) (domain.Node, error)
	Update(ctx context.Context, id string, node domain.Node) (domain.Node, error)
	Delete(ctx context.Context, id string) error

	// 按配置 ID 查询/批量替换（用于订阅更新）
	ListByConfigID(ctx context.Context, configID string) ([]domain.Node, error)
	ReplaceNodesForConfig(ctx context.Context, configID string, nodes []domain.Node) ([]domain.Node, error)

	// 延迟/速度更新
	UpdateLatency(ctx context.Context, id string, latencyMS int64, latencyErr string) error
	UpdateSpeed(ctx context.Context, id string, speedMbps float64, speedErr string) error
}

// NodeGroupRepository 节点组仓储接口（全局资源）
type NodeGroupRepository interface {
	// 基础 CRUD
	Get(ctx context.Context, id string) (domain.NodeGroup, error)
	List(ctx context.Context) ([]domain.NodeGroup, error)
	Create(ctx context.Context, group domain.NodeGroup) (domain.NodeGroup, error)
	Update(ctx context.Context, id string, group domain.NodeGroup) (domain.NodeGroup, error)
	Delete(ctx context.Context, id string) error
}

// ConfigRepository 订阅配置仓储接口
type ConfigRepository interface {
	// 基础 CRUD
	Get(ctx context.Context, id string) (domain.Config, error)
	List(ctx context.Context) ([]domain.Config, error)
	Create(ctx context.Context, cfg domain.Config) (domain.Config, error)
	Update(ctx context.Context, id string, cfg domain.Config) (domain.Config, error)
	Delete(ctx context.Context, id string) error

	// 同步状态更新
	UpdateSyncStatus(ctx context.Context, id string, payload, checksum string, syncErr error, usageUsedBytes, usageTotalBytes *int64) error
}

// GeoRepository Geo 资源仓储接口
type GeoRepository interface {
	// 基础 CRUD
	Get(ctx context.Context, id string) (domain.GeoResource, error)
	List(ctx context.Context) ([]domain.GeoResource, error)
	Create(ctx context.Context, res domain.GeoResource) (domain.GeoResource, error)
	Update(ctx context.Context, id string, res domain.GeoResource) (domain.GeoResource, error)
	Delete(ctx context.Context, id string) error

	// Upsert 语义（按 ID 或 Type 匹配）
	Upsert(ctx context.Context, res domain.GeoResource) (domain.GeoResource, error)

	// 按类型查询
	GetByType(ctx context.Context, geoType domain.GeoResourceType) (domain.GeoResource, error)
}

// ComponentRepository 组件仓储接口
type ComponentRepository interface {
	// 基础 CRUD
	Get(ctx context.Context, id string) (domain.CoreComponent, error)
	List(ctx context.Context) ([]domain.CoreComponent, error)
	Create(ctx context.Context, comp domain.CoreComponent) (domain.CoreComponent, error)
	Update(ctx context.Context, id string, comp domain.CoreComponent) (domain.CoreComponent, error)
	Delete(ctx context.Context, id string) error

	// 按 Kind 查询
	GetByKind(ctx context.Context, kind domain.CoreComponentKind) (domain.CoreComponent, error)

	// 安装状态更新
	UpdateInstallStatus(ctx context.Context, id string, status domain.InstallStatus, progress int, message string) error

	// 安装完成
	SetInstalled(ctx context.Context, id string, dir, version, checksum string) error

	// ClearInstalled 清除安装信息（用于卸载）
	ClearInstalled(ctx context.Context, id string) error

	// 清除同步错误
	ClearSyncError(ctx context.Context, id string) error

	// 更新元数据
	UpdateMeta(ctx context.Context, id string, key, value string) error
}

// SettingsRepository 设置仓储接口（单例设置）
type SettingsRepository interface {
	// 系统代理
	GetSystemProxy(ctx context.Context) (domain.SystemProxySettings, error)
	UpdateSystemProxy(ctx context.Context, settings domain.SystemProxySettings) (domain.SystemProxySettings, error)

	// 代理运行配置
	GetProxyConfig(ctx context.Context) (domain.ProxyConfig, error)
	UpdateProxyConfig(ctx context.Context, config domain.ProxyConfig) (domain.ProxyConfig, error)

	// 前端设置
	GetFrontend(ctx context.Context) (map[string]interface{}, error)
	UpdateFrontend(ctx context.Context, settings map[string]interface{}) (map[string]interface{}, error)
}

// Repositories 聚合所有仓储的容器接口
type Repositories interface {
	Node() NodeRepository
	NodeGroup() NodeGroupRepository
	FRouter() FRouterRepository
	Config() ConfigRepository
	Geo() GeoRepository
	Component() ComponentRepository
	Settings() SettingsRepository
}

// RepositoriesImpl 仓储容器实现
type RepositoriesImpl struct {
	store Snapshottable

	nodeRepo      NodeRepository
	nodeGroupRepo NodeGroupRepository
	frouterRepo   FRouterRepository
	configRepo    ConfigRepository
	geoRepo       GeoRepository
	componentRepo ComponentRepository
	settingsRepo  SettingsRepository
}

func NewRepositories(
	store Snapshottable,
	nodeRepo NodeRepository,
	nodeGroupRepo NodeGroupRepository,
	frouterRepo FRouterRepository,
	configRepo ConfigRepository,
	geoRepo GeoRepository,
	componentRepo ComponentRepository,
	settingsRepo SettingsRepository,
) *RepositoriesImpl {
	return &RepositoriesImpl{
		store: store,

		nodeRepo:      nodeRepo,
		nodeGroupRepo: nodeGroupRepo,
		frouterRepo:   frouterRepo,
		configRepo:    configRepo,
		geoRepo:       geoRepo,
		componentRepo: componentRepo,
		settingsRepo:  settingsRepo,
	}
}

// 实现 Repositories 接口
func (r *RepositoriesImpl) Node() NodeRepository           { return r.nodeRepo }
func (r *RepositoriesImpl) NodeGroup() NodeGroupRepository { return r.nodeGroupRepo }
func (r *RepositoriesImpl) FRouter() FRouterRepository     { return r.frouterRepo }
func (r *RepositoriesImpl) Config() ConfigRepository       { return r.configRepo }
func (r *RepositoriesImpl) Geo() GeoRepository             { return r.geoRepo }
func (r *RepositoriesImpl) Component() ComponentRepository { return r.componentRepo }
func (r *RepositoriesImpl) Settings() SettingsRepository   { return r.settingsRepo }

func (r *RepositoriesImpl) Snapshot() domain.ServiceState {
	if r.store == nil {
		return domain.ServiceState{}
	}
	return r.store.Snapshot()
}

func (r *RepositoriesImpl) LoadState(state domain.ServiceState) {
	if r.store == nil {
		return
	}
	r.store.LoadState(state)
}

// Snapshottable 可快照的存储接口
type Snapshottable interface {
	// Snapshot 生成状态快照
	Snapshot() domain.ServiceState

	// LoadState 加载状态
	LoadState(state domain.ServiceState)
}
