package adapters

import (
	"vea/backend/domain"
)

// CoreAdapter 内核适配器接口
// 每个代理内核（Xray、sing-box）都需要实现这个接口
type CoreAdapter interface {
	// Kind 返回内核类型
	Kind() domain.CoreEngineKind

	// BinaryNames 返回二进制文件的可能名称
	BinaryNames() []string

	// SupportedProtocols 返回支持的节点协议列表
	SupportedProtocols() []domain.NodeProtocol

	// SupportsInbound 检查是否支持特定入站模式
	SupportsInbound(mode domain.InboundMode) bool

	// BuildConfig 生成内核配置文件
	// 返回配置 JSON 字节数组
	BuildConfig(profile domain.ProxyProfile, nodes []domain.Node, geo GeoFiles) ([]byte, error)

	// RequiresPrivileges 检查是否需要特权（主要用于 TUN 模式）
	RequiresPrivileges(profile domain.ProxyProfile) bool
}

// GeoFiles Geo 资源文件路径
type GeoFiles struct {
	GeoIP        string
	GeoSite      string
	ArtifactsDir string // artifacts 目录的绝对路径，用于构建插件路径等
}
