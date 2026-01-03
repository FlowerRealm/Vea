package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"vea/backend/domain"
	"vea/backend/persist"
	"vea/backend/repository"
	"vea/backend/service/applog"
	"vea/backend/service/component"
	configsvc "vea/backend/service/config"
	"vea/backend/service/frouter"
	"vea/backend/service/geo"
	"vea/backend/service/nodes"
	"vea/backend/service/proxy"
	"vea/backend/service/shared"

	"github.com/google/uuid"
)

// Facade 服务门面（API 聚合层）
type Facade struct {
	nodes     *nodes.Service
	frouter   *frouter.Service
	config    *configsvc.Service
	proxy     *proxy.Service
	component *component.Service
	geo       *geo.Service

	appLogPath      string
	appLogStartedAt time.Time

	// Repositories 用于直接访问（settings/rules 等）
	repos repository.Repositories
}

// NewFacade 创建门面服务
func NewFacade(
	nodeSvc *nodes.Service,
	frouterSvc *frouter.Service,
	configSvc *configsvc.Service,
	proxySvc *proxy.Service,
	componentSvc *component.Service,
	geoSvc *geo.Service,
	repos repository.Repositories,
) *Facade {
	return &Facade{
		nodes:     nodeSvc,
		frouter:   frouterSvc,
		config:    configSvc,
		proxy:     proxySvc,
		component: componentSvc,
		geo:       geoSvc,
		repos:     repos,
	}
}

func (f *Facade) SetAppLog(path string, startedAt time.Time) {
	f.appLogPath = path
	f.appLogStartedAt = startedAt
}

// Errors 返回所有错误类型（用于 API 层错误处理）
func (f *Facade) Errors() (nodeNotFound, frouterNotFound, configNotFound, geoNotFound, componentNotFound error) {
	return repository.ErrNodeNotFound,
		repository.ErrFRouterNotFound,
		repository.ErrConfigNotFound,
		repository.ErrGeoNotFound,
		repository.ErrComponentNotFound
}

// Snapshot 获取完整状态快照
func (f *Facade) Snapshot() (domain.ServiceState, error) {
	ctx := context.Background()

	nodes, err := f.nodes.List(ctx)
	if err != nil {
		return domain.ServiceState{}, err
	}
	frouters, err := f.frouter.List(ctx)
	if err != nil {
		return domain.ServiceState{}, err
	}
	configs, err := f.config.List(ctx)
	if err != nil {
		return domain.ServiceState{}, err
	}
	geoResources, err := f.geo.List(ctx)
	if err != nil {
		return domain.ServiceState{}, err
	}
	components, err := f.component.List(ctx)
	if err != nil {
		return domain.ServiceState{}, err
	}

	systemProxy, err := f.repos.Settings().GetSystemProxy(ctx)
	if err != nil {
		return domain.ServiceState{}, err
	}
	proxyConfig, err := f.repos.Settings().GetProxyConfig(ctx)
	if err != nil {
		return domain.ServiceState{}, err
	}
	frontendSettings, err := f.repos.Settings().GetFrontend(ctx)
	if err != nil {
		return domain.ServiceState{}, err
	}

	return domain.ServiceState{
		SchemaVersion:    persist.SchemaVersion,
		Nodes:            nodes,
		FRouters:         frouters,
		Configs:          configs,
		GeoResources:     geoResources,
		Components:       components,
		SystemProxy:      systemProxy,
		ProxyConfig:      proxyConfig,
		FrontendSettings: frontendSettings,
		GeneratedAt:      time.Now(),
	}, nil
}

func (f *Facade) EnsureDefaultFRouter(ctx context.Context) error {
	frouters, err := f.frouter.List(ctx)
	if err != nil {
		return err
	}
	var picked domain.FRouter
	if len(frouters) == 0 {
		created, err := f.frouter.Create(ctx, domain.FRouter{
			Name: "默认 FRouter",
			ChainProxy: domain.ChainProxySettings{
				Slots: []domain.SlotNode{
					{ID: "slot-1", Name: "配置槽"},
				},
				Edges: []domain.ProxyEdge{
					{
						ID:       uuid.NewString(),
						From:     domain.EdgeNodeLocal,
						To:       domain.EdgeNodeDirect,
						Priority: 0,
						Enabled:  true,
					},
				},
			},
		})
		if err != nil {
			return err
		}
		picked = created
	} else {
		picked = frouters[0]
		for i := range frouters {
			if frouters[i].CreatedAt.Before(picked.CreatedAt) {
				picked = frouters[i]
			}
		}
	}

	cfg, err := f.repos.Settings().GetProxyConfig(ctx)
	if err != nil {
		return err
	}
	if strings.TrimSpace(cfg.FRouterID) != "" {
		if _, err := f.frouter.Get(ctx, cfg.FRouterID); err == nil {
			return nil
		}
	}

	cfg.FRouterID = picked.ID
	cfg.UpdatedAt = time.Now()
	_, err = f.repos.Settings().UpdateProxyConfig(ctx, cfg)
	return err
}

// ========== FRouter 操作 ==========

// ListFRouters 列出所有 FRouter
func (f *Facade) ListFRouters() ([]domain.FRouter, error) {
	return f.frouter.List(context.Background())
}

// GetFRouter 获取 FRouter
func (f *Facade) GetFRouter(id string) (domain.FRouter, error) {
	return f.frouter.Get(context.Background(), id)
}

// CreateFRouter 创建 FRouter
func (f *Facade) CreateFRouter(frouter domain.FRouter) (domain.FRouter, error) {
	return f.frouter.Create(context.Background(), frouter)
}

// UpdateFRouter 更新 FRouter
func (f *Facade) UpdateFRouter(id string, updateFn func(domain.FRouter) (domain.FRouter, error)) (domain.FRouter, error) {
	frouter, err := f.frouter.Get(context.Background(), id)
	if err != nil {
		return domain.FRouter{}, err
	}
	updated, err := updateFn(frouter)
	if err != nil {
		return domain.FRouter{}, err
	}
	return f.frouter.Update(context.Background(), id, updated)
}

// DeleteFRouter 删除 FRouter
func (f *Facade) DeleteFRouter(id string) error {
	return f.frouter.Delete(context.Background(), id)
}

// MeasureFRouterLatencyAsync 异步测试 FRouter 延迟
func (f *Facade) MeasureFRouterLatencyAsync(id string) {
	f.frouter.ProbeLatencyAsync(id)
}

// MeasureFRouterSpeedAsync 异步测试 FRouter 速度
func (f *Facade) MeasureFRouterSpeedAsync(id string) {
	f.frouter.ProbeSpeedAsync(id)
}

// ========== Node 操作 ==========

func (f *Facade) ListNodes() ([]domain.Node, error) {
	return f.nodes.List(context.Background())
}

func (f *Facade) CreateNode(node domain.Node) (domain.Node, error) {
	return f.nodes.Create(context.Background(), node)
}

func (f *Facade) UpdateNode(id string, updateFn func(domain.Node) (domain.Node, error)) (domain.Node, error) {
	node, err := f.nodes.Get(context.Background(), id)
	if err != nil {
		return domain.Node{}, err
	}
	updated, err := updateFn(node)
	if err != nil {
		return domain.Node{}, err
	}
	return f.nodes.Update(context.Background(), id, updated)
}

func (f *Facade) MeasureNodeLatencyAsync(id string) {
	f.nodes.ProbeLatencyAsync(id)
}

func (f *Facade) MeasureNodeSpeedAsync(id string) {
	f.nodes.ProbeSpeedAsync(id)
}

// ========== Config 操作 ==========

// ListConfigs 列出所有配置
func (f *Facade) ListConfigs() ([]domain.Config, error) {
	return f.config.List(context.Background())
}

// CreateConfig 创建配置
func (f *Facade) CreateConfig(cfg domain.Config) (domain.Config, error) {
	return f.config.Create(context.Background(), cfg)
}

// UpdateConfig 更新配置
func (f *Facade) UpdateConfig(id string, updateFn func(domain.Config) (domain.Config, error)) (domain.Config, error) {
	cfg, err := f.config.Get(context.Background(), id)
	if err != nil {
		return domain.Config{}, err
	}
	updated, err := updateFn(cfg)
	if err != nil {
		return domain.Config{}, err
	}
	return f.config.Update(context.Background(), id, updated)
}

// DeleteConfig 删除配置
func (f *Facade) DeleteConfig(id string) error {
	return f.config.Delete(context.Background(), id)
}

// RefreshConfig 刷新配置
func (f *Facade) RefreshConfig(id string) (domain.Config, error) {
	if err := f.config.Sync(context.Background(), id); err != nil {
		return domain.Config{}, err
	}
	return f.config.Get(context.Background(), id)
}

// SyncConfigFRouters 同步配置 FRouter
func (f *Facade) SyncConfigNodes(configID string) ([]domain.Node, error) {
	ctx := context.Background()
	cfg, err := f.config.Get(ctx, configID)
	if err != nil {
		return nil, err
	}

	// 有订阅链接时：先同步配置内容（下载/更新 payload），再解析节点。
	if strings.TrimSpace(cfg.SourceURL) != "" {
		if err := f.config.Sync(ctx, configID); err != nil {
			return nil, err
		}
	}
	return f.config.PullNodes(ctx, configID)
}

// ========== Proxy 操作 ==========

// GetProxyStatus 获取代理状态
func (f *Facade) GetProxyStatus() map[string]interface{} {
	return f.proxy.Status(context.Background())
}

// MarkProxyRestartScheduled 记录“代理重启已触发”（用于前端轮询提示）。
func (f *Facade) MarkProxyRestartScheduled() {
	f.proxy.MarkRestartScheduled()
}

// MarkProxyRestartFailed 记录“代理重启失败”（用于前端轮询提示）。
func (f *Facade) MarkProxyRestartFailed(err error) {
	f.proxy.MarkRestartFailed(err)
}

// StopProxy 停止代理
func (f *Facade) StopProxy() error {
	ctx := context.Background()
	if err := f.proxy.Stop(ctx); err != nil {
		return err
	}

	// 停止内核后继续保持系统代理是灾难：用户网络直接断。
	// 因此这里强制关闭系统代理并持久化状态。
	settings, err := f.repos.Settings().GetSystemProxy(ctx)
	if err != nil {
		return err
	}
	if settings.Enabled {
		settings.Enabled = false
		if _, _, err := f.UpdateSystemProxySettings(settings); err != nil {
			return fmt.Errorf("disable system proxy: %w", err)
		}
	}

	return nil
}

// StartProxy 启动代理（以 FRouter 为中心）
func (f *Facade) StartProxy(config domain.ProxyConfig) error {
	ctx := context.Background()

	if err := f.proxy.Start(ctx, config); err != nil {
		var engineErr *proxy.EngineNotInstalledError
		if errors.As(err, &engineErr) {
			if err2 := f.ensureCoreEngineInstalled(ctx, engineErr.Engine); err2 != nil {
				return err2
			}
			return f.proxy.Start(ctx, config)
		}
		return err
	}
	return nil
}

func (f *Facade) GetKernelLogs(since int64) proxy.KernelLogSnapshot {
	return f.proxy.KernelLogsSince(since)
}

func (f *Facade) GetAppLogs(since int64) applog.AppLogSnapshot {
	return applog.LogsSince(f.appLogPath, since, os.Getpid(), f.appLogStartedAt)
}

func (f *Facade) ensureCoreEngineInstalled(ctx context.Context, engine domain.CoreEngineKind) error {
	if f.component == nil {
		return errors.New("component service is not configured")
	}

	var kind domain.CoreComponentKind
	switch engine {
	case domain.EngineXray:
		kind = domain.ComponentXray
	case domain.EngineSingBox:
		kind = domain.ComponentSingBox
	default:
		return fmt.Errorf("unknown engine: %s", engine)
	}

	comp, err := f.component.Create(ctx, domain.CoreComponent{Kind: kind})
	if err != nil {
		return fmt.Errorf("resolve component %s: %w", kind, err)
	}
	if _, err := f.component.Install(ctx, comp.ID); err != nil {
		return fmt.Errorf("install %s: %w", kind, err)
	}

	deadline := time.Now().Add(shared.DownloadTimeout)
	for time.Now().Before(deadline) {
		current, err := f.component.Get(ctx, comp.ID)
		if err != nil {
			return fmt.Errorf("get %s component: %w", kind, err)
		}
		if current.InstallStatus == domain.InstallStatusError {
			return fmt.Errorf("install %s failed: %s", kind, strings.TrimSpace(current.InstallMessage))
		}
		if current.InstallDir != "" && !current.LastInstalledAt.IsZero() {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("install %s timeout", kind)
}

// ========== Component 操作 ==========

// ListComponents 列出所有组件
func (f *Facade) ListComponents() ([]domain.CoreComponent, error) {
	return f.component.List(context.Background())
}

// CreateComponent 创建组件
func (f *Facade) CreateComponent(comp domain.CoreComponent) (domain.CoreComponent, error) {
	return f.component.Create(context.Background(), comp)
}

// UpdateComponent 更新组件
func (f *Facade) UpdateComponent(id string, updateFn func(domain.CoreComponent) (domain.CoreComponent, error)) (domain.CoreComponent, error) {
	comp, err := f.component.Get(context.Background(), id)
	if err != nil {
		return domain.CoreComponent{}, err
	}
	updated, err := updateFn(comp)
	if err != nil {
		return domain.CoreComponent{}, err
	}
	return f.component.Update(context.Background(), id, updated)
}

// DeleteComponent 删除组件
func (f *Facade) DeleteComponent(id string) error {
	return f.component.Delete(context.Background(), id)
}

// InstallComponentAsync 异步安装组件
func (f *Facade) InstallComponentAsync(id string) (domain.CoreComponent, error) {
	return f.component.Install(context.Background(), id)
}

// ========== Geo 操作 ==========

// ListGeo 列出所有 Geo 资源
func (f *Facade) ListGeo() ([]domain.GeoResource, error) {
	return f.geo.List(context.Background())
}

// UpsertGeo 插入或更新 Geo 资源
func (f *Facade) UpsertGeo(geo domain.GeoResource) (domain.GeoResource, error) {
	return f.geo.Upsert(context.Background(), geo)
}

// DeleteGeo 删除 Geo 资源
func (f *Facade) DeleteGeo(id string) error {
	return f.geo.Delete(context.Background(), id)
}

// RefreshGeo 刷新 Geo 资源
func (f *Facade) RefreshGeo(id string) (domain.GeoResource, error) {
	if err := f.geo.Sync(context.Background(), id); err != nil {
		return domain.GeoResource{}, err
	}
	return f.geo.Get(context.Background(), id)
}

// ========== Settings 操作 ==========

// SystemProxySettings 获取系统代理设置
func (f *Facade) SystemProxySettings() (domain.SystemProxySettings, error) {
	return f.repos.Settings().GetSystemProxy(context.Background())
}

// UpdateSystemProxySettings 更新系统代理设置
func (f *Facade) UpdateSystemProxySettings(settings domain.SystemProxySettings) (domain.SystemProxySettings, string, error) {
	ctx := context.Background()
	// 与 SettingsRepo 的默认行为对齐，避免出现“已应用到系统但持久化后变了”的不一致。
	if len(settings.IgnoreHosts) == 0 {
		settings.IgnoreHosts = []string{"127.0.0.0/8", "::1", "localhost"}
	}

	// 启用系统代理时，至少需要知道当前运行内核/端口，否则就是把系统网络指向一个黑洞。
	if settings.Enabled {
		status := f.proxy.Status(ctx)
		running, _ := status["running"].(bool)
		if !running {
			return domain.SystemProxySettings{}, "", fmt.Errorf("proxy not running")
		}

		engine, _ := status["engine"].(string)
		inboundMode, _ := status["inboundMode"].(string)
		inboundPort, _ := status["inboundPort"].(int)
		if inboundPort <= 0 {
			inboundPort = 1080
		}
		if inboundMode == string(domain.InboundTUN) {
			return domain.SystemProxySettings{}, "", fmt.Errorf("inboundMode=tun: 系统代理无需启用（请关闭系统代理，使用 TUN 接管系统流量）")
		}

		httpPort := 0
		httpsPort := 0
		socksPort := 0
		switch inboundMode {
		case string(domain.InboundHTTP):
			httpPort, httpsPort = inboundPort, inboundPort
		case string(domain.InboundSOCKS):
			socksPort = inboundPort
		case string(domain.InboundMixed):
			httpPort, httpsPort = inboundPort, inboundPort
			socksPort = inboundPort
			if engine == string(domain.EngineXray) {
				// Xray mixed = HTTP(port) + SOCKS(port+1)
				socksPort = inboundPort + 1
			}
		default:
			// 兜底：按 mixed 处理
			httpPort, httpsPort, socksPort = inboundPort, inboundPort, inboundPort
			if engine == string(domain.EngineXray) {
				socksPort = inboundPort + 1
			}
		}

		message, err := shared.ApplySystemProxy(shared.SystemProxyConfig{
			Enabled: true,

			HTTPHost:  "127.0.0.1",
			HTTPPort:  httpPort,
			HTTPSHost: "127.0.0.1",
			HTTPSPort: httpsPort,

			SOCKSHost: "127.0.0.1",
			SOCKSPort: socksPort,

			IgnoreHosts: settings.IgnoreHosts,
		})
		if err != nil {
			return domain.SystemProxySettings{}, "", err
		}

		updated, err := f.repos.Settings().UpdateSystemProxy(ctx, settings)
		if err != nil {
			return domain.SystemProxySettings{}, "", err
		}

		return updated, message, nil
	}

	message, err := shared.ApplySystemProxy(shared.SystemProxyConfig{Enabled: false})
	if err != nil {
		return domain.SystemProxySettings{}, "", err
	}
	updated, err := f.repos.Settings().UpdateSystemProxy(ctx, settings)
	if err != nil {
		return domain.SystemProxySettings{}, "", err
	}
	return updated, message, nil
}

// GetProxyConfig 获取代理运行配置（单例）
func (f *Facade) GetProxyConfig() (domain.ProxyConfig, error) {
	return f.repos.Settings().GetProxyConfig(context.Background())
}

// UpdateProxyConfig 更新代理运行配置
func (f *Facade) UpdateProxyConfig(updateFn func(domain.ProxyConfig) (domain.ProxyConfig, error)) (domain.ProxyConfig, error) {
	current, err := f.repos.Settings().GetProxyConfig(context.Background())
	if err != nil {
		return domain.ProxyConfig{}, err
	}
	updated, err := updateFn(current)
	if err != nil {
		return domain.ProxyConfig{}, err
	}
	return f.repos.Settings().UpdateProxyConfig(context.Background(), updated)
}

// GetFrontendSettings 获取前端设置
func (f *Facade) GetFrontendSettings() (map[string]interface{}, error) {
	return f.repos.Settings().GetFrontend(context.Background())
}

// SaveFrontendSettings 保存前端设置
func (f *Facade) SaveFrontendSettings(settings map[string]interface{}) error {
	_, err := f.repos.Settings().UpdateFrontend(context.Background(), settings)
	return err
}

// ========== TUN 操作 ==========

// CheckTUNCapabilities 检查 TUN 权限
func (f *Facade) CheckTUNCapabilities() (bool, error) {
	return shared.CheckTUNCapabilities()
}

// SetupTUN 配置 TUN 权限
func (f *Facade) SetupTUN() error {
	_, err := shared.EnsureTUNCapabilities()
	return err
}

// GetIPGeo 获取 IP 地理位置
func (f *Facade) GetIPGeo() (map[string]interface{}, error) {
	return shared.GetIPGeo()
}

// ========== 引擎推荐 ==========

// RecommendEngine 获取引擎推荐
func (f *Facade) RecommendEngine() proxy.EngineRecommendation {
	return f.proxy.RecommendEngine(context.Background())
}

// GetEngineStatus 获取引擎状态
func (f *Facade) GetEngineStatus() proxy.EngineStatus {
	return f.proxy.GetEngineStatus(context.Background())
}
