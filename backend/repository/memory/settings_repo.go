package memory

import (
	"context"
	"time"

	"vea/backend/domain"
	"vea/backend/repository"
	"vea/backend/repository/events"
)

// SettingsRepo 设置仓储实现
type SettingsRepo struct {
	store *Store
}

// NewSettingsRepo 创建设置仓储
func NewSettingsRepo(store *Store) *SettingsRepo {
	return &SettingsRepo{store: store}
}

// GetSystemProxy 获取系统代理设置
func (r *SettingsRepo) GetSystemProxy(ctx context.Context) (domain.SystemProxySettings, error) {
	r.store.RLock()
	defer r.store.RUnlock()
	return r.store.GetSystemProxy(), nil
}

// UpdateSystemProxy 更新系统代理设置
func (r *SettingsRepo) UpdateSystemProxy(ctx context.Context, settings domain.SystemProxySettings) (domain.SystemProxySettings, error) {
	// 确保默认忽略主机列表
	if len(settings.IgnoreHosts) == 0 {
		settings.IgnoreHosts = []string{"127.0.0.0/8", "::1", "localhost"}
	}
	settings.UpdatedAt = time.Now()

	r.store.Lock()
	r.store.SetSystemProxy(settings)
	r.store.Unlock()

	// 在锁外发布事件
	r.store.PublishEvent(events.SettingsEvent{
		EventType: events.EventSystemProxyChanged,
	})

	return settings, nil
}

// GetProxyConfig 获取代理运行配置
func (r *SettingsRepo) GetProxyConfig(ctx context.Context) (domain.ProxyConfig, error) {
	r.store.RLock()
	defer r.store.RUnlock()
	return r.store.GetProxyConfig(), nil
}

// UpdateProxyConfig 更新代理运行配置
func (r *SettingsRepo) UpdateProxyConfig(ctx context.Context, config domain.ProxyConfig) (domain.ProxyConfig, error) {
	config.UpdatedAt = time.Now()

	r.store.Lock()
	r.store.SetProxyConfig(config)
	r.store.Unlock()

	// 在锁外发布事件
	r.store.PublishEvent(events.SettingsEvent{
		EventType: events.EventProxyConfigChanged,
	})

	return config, nil
}

// GetFrontend 获取前端设置
func (r *SettingsRepo) GetFrontend(ctx context.Context) (map[string]interface{}, error) {
	r.store.RLock()
	defer r.store.RUnlock()
	return r.store.GetFrontendSettings(), nil
}

// UpdateFrontend 更新前端设置
func (r *SettingsRepo) UpdateFrontend(ctx context.Context, settings map[string]interface{}) (map[string]interface{}, error) {
	r.store.Lock()
	r.store.SetFrontendSettings(settings)
	r.store.Unlock()

	// 在锁外发布事件
	r.store.PublishEvent(events.SettingsEvent{
		EventType: events.EventFrontendSettingsChanged,
	})

	return settings, nil
}

// 确保实现接口
var _ repository.SettingsRepository = (*SettingsRepo)(nil)
