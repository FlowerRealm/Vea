package events

import "vea/backend/domain"

// EventType 事件类型
type EventType string

const (
	// FRouter 事件
	EventFRouterCreated EventType = "frouter.created"
	EventFRouterUpdated EventType = "frouter.updated"
	EventFRouterDeleted EventType = "frouter.deleted"

	// Node 事件
	EventNodeCreated EventType = "node.created"
	EventNodeUpdated EventType = "node.updated"
	EventNodeDeleted EventType = "node.deleted"

	// 配置事件
	EventConfigCreated EventType = "config.created"
	EventConfigUpdated EventType = "config.updated"
	EventConfigDeleted EventType = "config.deleted"

	// Geo 资源事件
	EventGeoCreated EventType = "geo.created"
	EventGeoUpdated EventType = "geo.updated"
	EventGeoDeleted EventType = "geo.deleted"

	// 组件事件
	EventComponentCreated EventType = "component.created"
	EventComponentUpdated EventType = "component.updated"
	EventComponentDeleted EventType = "component.deleted"

	// 设置事件
	EventSystemProxyChanged      EventType = "settings.system_proxy_changed"
	EventProxyConfigChanged      EventType = "settings.proxy_config_changed"
	EventFrontendSettingsChanged EventType = "settings.frontend_changed"

	// 通配符事件（用于订阅所有事件）
	EventAll EventType = "*"
)

// Event 事件接口
type Event interface {
	Type() EventType
}

// FRouterEvent FRouter 事件
type FRouterEvent struct {
	EventType EventType
	FRouterID string
	FRouter   domain.FRouter
}

func (e FRouterEvent) Type() EventType { return e.EventType }

// NodeEvent Node 事件
type NodeEvent struct {
	EventType EventType
	NodeID    string
	Node      domain.Node
}

func (e NodeEvent) Type() EventType { return e.EventType }

// ConfigEvent 配置事件
type ConfigEvent struct {
	EventType EventType
	ConfigID  string
	Config    domain.Config
}

func (e ConfigEvent) Type() EventType { return e.EventType }

// GeoEvent Geo 资源事件
type GeoEvent struct {
	EventType EventType
	GeoID     string
	Geo       domain.GeoResource
}

func (e GeoEvent) Type() EventType { return e.EventType }

// ComponentEvent 组件事件
type ComponentEvent struct {
	EventType   EventType
	ComponentID string
	Component   domain.CoreComponent
}

func (e ComponentEvent) Type() EventType { return e.EventType }

// SettingsEvent 设置事件
type SettingsEvent struct {
	EventType EventType
}

func (e SettingsEvent) Type() EventType { return e.EventType }
