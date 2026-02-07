package domain

import (
	"time"
)

type NodeProtocol string

const (
	ProtocolVLESS       NodeProtocol = "vless"
	ProtocolTrojan      NodeProtocol = "trojan"
	ProtocolShadowsocks NodeProtocol = "shadowsocks"
	ProtocolVMess       NodeProtocol = "vmess"
	ProtocolHysteria2   NodeProtocol = "hysteria2"
	ProtocolTUIC        NodeProtocol = "tuic"
)

type Node struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	Address          string         `json:"address"`
	Port             int            `json:"port"`
	Protocol         NodeProtocol   `json:"protocol"`
	Tags             []string       `json:"tags"`
	Security         *NodeSecurity  `json:"security,omitempty"`
	Transport        *NodeTransport `json:"transport,omitempty"`
	TLS              *NodeTLS       `json:"tls,omitempty"`
	SourceConfigID   string         `json:"sourceConfigId,omitempty"`
	SourceKey        string         `json:"sourceKey,omitempty"`
	LastLatencyMS    int64          `json:"lastLatencyMs"`
	LastLatencyAt    time.Time      `json:"lastLatencyAt"`
	LastLatencyError string         `json:"lastLatencyError,omitempty"`
	LastSpeedMbps    float64        `json:"lastSpeedMbps"`
	LastSpeedAt      time.Time      `json:"lastSpeedAt"`
	LastSpeedError   string         `json:"lastSpeedError,omitempty"`
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
}

type NodeGroupStrategy string

const (
	NodeGroupStrategyLowestLatency NodeGroupStrategy = "lowest-latency"
	NodeGroupStrategyFastestSpeed  NodeGroupStrategy = "fastest-speed"
	NodeGroupStrategyRoundRobin    NodeGroupStrategy = "round-robin"
	NodeGroupStrategyFailover      NodeGroupStrategy = "failover"
)

type NodeGroup struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	NodeIDs   []string          `json:"nodeIds"`
	Strategy  NodeGroupStrategy `json:"strategy"`
	Tags      []string          `json:"tags,omitempty"`
	Cursor    int               `json:"cursor,omitempty"`
	CreatedAt time.Time         `json:"createdAt"`
	UpdatedAt time.Time         `json:"updatedAt"`
}

// FRouter 转发路由定义（主要对外操作单元）
// - Node 为独立实体（食材）；FRouter 仅通过 ChainProxy 图引用 NodeID（工具使用食材，但不“包含食材”）。
type FRouter struct {
	ID               string             `json:"id"`
	Name             string             `json:"name"`
	ChainProxy       ChainProxySettings `json:"chainProxy"`
	Tags             []string           `json:"tags,omitempty"`
	SourceConfigID   string             `json:"sourceConfigId,omitempty"`
	LastLatencyMS    int64              `json:"lastLatencyMs"`
	LastLatencyAt    time.Time          `json:"lastLatencyAt"`
	LastLatencyError string             `json:"lastLatencyError,omitempty"`
	LastSpeedMbps    float64            `json:"lastSpeedMbps"`
	LastSpeedAt      time.Time          `json:"lastSpeedAt"`
	LastSpeedError   string             `json:"lastSpeedError,omitempty"`
	CreatedAt        time.Time          `json:"createdAt"`
	UpdatedAt        time.Time          `json:"updatedAt"`
}

type NodeSecurity struct {
	UUID       string   `json:"uuid,omitempty"`
	Password   string   `json:"password,omitempty"`
	Method     string   `json:"method,omitempty"`
	Flow       string   `json:"flow,omitempty"`
	Encryption string   `json:"encryption,omitempty"`
	AlterID    int      `json:"alterId,omitempty"`
	Plugin     string   `json:"plugin,omitempty"`
	PluginOpts string   `json:"pluginOpts,omitempty"`
	ALPN       []string `json:"alpn,omitempty"`
}

type NodeTransport struct {
	Type        string            `json:"type,omitempty"`
	Host        string            `json:"host,omitempty"`
	Path        string            `json:"path,omitempty"`
	ServiceName string            `json:"serviceName,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	HeaderType  string            `json:"headerType,omitempty"` // VMess 伪装类型（http/srtp/utp/wechat-video/dtls/wireguard）
}

type NodeTLS struct {
	Enabled          bool     `json:"enabled,omitempty"`
	Type             string   `json:"type,omitempty"`
	ServerName       string   `json:"serverName,omitempty"`
	Insecure         bool     `json:"insecure,omitempty"`
	Fingerprint      string   `json:"fingerprint,omitempty"`
	RealityPublicKey string   `json:"realityPublicKey,omitempty"`
	RealityShortID   string   `json:"realityShortId,omitempty"`
	ALPN             []string `json:"alpn,omitempty"`
}

type ConfigFormat string

const (
	// ConfigFormatSubscription 订阅/配置源（分享链接与 Clash YAML）。
	//
	// 注意：Vea 不再区分“特定内核 JSON”等具体格式；解析逻辑以实际 payload 内容为准。
	ConfigFormatSubscription ConfigFormat = "subscription"
)

type Config struct {
	ID                 string        `json:"id"`
	Name               string        `json:"name"`
	Format             ConfigFormat  `json:"format"`
	Payload            string        `json:"payload"`
	SourceURL          string        `json:"sourceUrl,omitempty"`
	Checksum           string        `json:"checksum,omitempty"`
	LastSyncError      string        `json:"lastSyncError,omitempty"`
	AutoUpdateInterval time.Duration `json:"autoUpdateInterval"`
	LastSyncedAt       time.Time     `json:"lastSyncedAt"`
	ExpireAt           *time.Time    `json:"expireAt"`
	UsageUsedBytes     *int64        `json:"usageUsedBytes,omitempty"`
	UsageTotalBytes    *int64        `json:"usageTotalBytes,omitempty"`
	CreatedAt          time.Time     `json:"createdAt"`
	UpdatedAt          time.Time     `json:"updatedAt"`
}

type GeoResourceType string

const (
	GeoIP   GeoResourceType = "geoip"
	GeoSite GeoResourceType = "geosite"
)

type GeoResource struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Type          GeoResourceType `json:"type"`
	SourceURL     string          `json:"sourceUrl"`
	Checksum      string          `json:"checksum"`
	Version       string          `json:"version"`
	ArtifactPath  string          `json:"artifactPath,omitempty"`
	FileSizeBytes int64           `json:"fileSizeBytes,omitempty"`
	LastSyncError string          `json:"lastSyncError,omitempty"`
	LastSynced    time.Time       `json:"lastSynced"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
}

type CoreComponentKind string

const (
	ComponentSingBox CoreComponentKind = "singbox"
	ComponentClash   CoreComponentKind = "clash"
	ComponentGeo     CoreComponentKind = "geo"
	ComponentGeneric CoreComponentKind = "generic"
)

type InstallStatus string

const (
	InstallStatusIdle        InstallStatus = ""
	InstallStatusDownloading InstallStatus = "downloading"
	InstallStatusExtracting  InstallStatus = "extracting"
	InstallStatusDone        InstallStatus = "done"
	InstallStatusError       InstallStatus = "error"
)

type CoreComponent struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Kind            CoreComponentKind `json:"kind"`
	SourceURL       string            `json:"sourceUrl"`
	ArchiveType     string            `json:"archiveType"`
	LastInstalledAt time.Time         `json:"lastInstalledAt"`
	InstallDir      string            `json:"installDir"`
	LastVersion     string            `json:"lastVersion"`
	Checksum        string            `json:"checksum"`
	LastSyncError   string            `json:"lastSyncError"`
	Meta            map[string]string `json:"meta,omitempty"`
	CreatedAt       time.Time         `json:"createdAt"`
	UpdatedAt       time.Time         `json:"updatedAt"`
	// 配套组件（如 sing-box 的 v2ray-plugin）
	Accessories []string `json:"accessories,omitempty"`
	// 安装进度相关
	InstallStatus   InstallStatus `json:"installStatus,omitempty"`
	InstallProgress int           `json:"installProgress,omitempty"` // 0-100
	InstallMessage  string        `json:"installMessage,omitempty"`
}

type SystemProxySettings struct {
	Enabled     bool      `json:"enabled"`
	IgnoreHosts []string  `json:"ignoreHosts"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// 特殊节点常量（用于 ProxyEdge 的 From/To 字段）
const (
	EdgeNodeLocal      = "local"  // 本机入口（唯一入口）
	EdgeNodeDirect     = "direct" // 直连动作
	EdgeNodeBlock      = "block"  // 阻断动作（立即失败）
	EdgeNodeSlotPrefix = "slot-"  // 插槽节点前缀（如 "slot-1"）
)

// IsSlotNode 检查节点 ID 是否是插槽节点
func IsSlotNode(nodeID string) bool {
	return len(nodeID) > len(EdgeNodeSlotPrefix) && nodeID[:len(EdgeNodeSlotPrefix)] == EdgeNodeSlotPrefix
}

// EdgeRuleType 边规则类型
type EdgeRuleType string

const (
	EdgeRuleNone  EdgeRuleType = ""      // 默认路径（无条件）
	EdgeRuleRoute EdgeRuleType = "route" // 路由规则（域名/IP 匹配）
)

// RouteMatchRule 路由匹配规则
type RouteMatchRule struct {
	Domains []string `json:"domains,omitempty"` // 域名匹配（支持通配符 *.google.com）
	IPs     []string `json:"ips,omitempty"`     // IP/CIDR 匹配
}

// ProxyEdge 代理边 - 定义两个节点之间的连接关系
type ProxyEdge struct {
	ID          string          `json:"id"`                    // 边的唯一标识
	From        string          `json:"from"`                  // 源: "local" | nodeID | slotID
	To          string          `json:"to"`                    // 目标: nodeID | slotID | "direct" | "block"
	Via         []string        `json:"via,omitempty"`         // 链式代理：在 local->node 的“选择边”上定义后续 hop（会被编译成 detour）
	Tag         string          `json:"tag,omitempty"`         // 边标签（用于路由规则引用）
	Priority    int             `json:"priority"`              // 优先级（多出口时，数值大优先）
	Enabled     bool            `json:"enabled"`               // 是否启用
	RuleType    EdgeRuleType    `json:"ruleType,omitempty"`    // 规则类型
	RouteRule   *RouteMatchRule `json:"routeRule,omitempty"`   // 路由匹配规则
	Description string          `json:"description,omitempty"` // 边描述/标签
}

// GraphPosition 节点在画布上的位置
type GraphPosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// SlotNode 插槽节点 - 可绑定到实际代理节点的占位符
// 未绑定时流量直接穿透，绑定后使用绑定节点的代理配置
type SlotNode struct {
	ID          string `json:"id"`                    // 插槽 ID（如 "slot-1"）
	Name        string `json:"name"`                  // 显示名称
	BoundNodeID string `json:"boundNodeId,omitempty"` // 绑定的实际节点 ID（空表示未绑定/穿透）
}

// ChainProxySettings 链式代理设置（图结构）
// 通过边定义代理拓扑：
// - local -> {node|direct|block} 为“选择边”（可写规则/priority；可选 via 生成链路）
// - {node|slot} -> {node|slot} 为 detour 边（不写规则；描述节点之间的上游链路）
// 支持插槽节点：slot 为占位符，可绑定到实际节点；未绑定的 slot 会被静默跳过。
type ChainProxySettings struct {
	Edges     []ProxyEdge              `json:"edges"`               // 所有代理边
	Positions map[string]GraphPosition `json:"positions,omitempty"` // 节点位置（nodeID -> position）
	Slots     []SlotNode               `json:"slots,omitempty"`     // 插槽节点列表
	UpdatedAt time.Time                `json:"updatedAt"`           // 最后更新时间
}

type ServiceState struct {
	// 版本管理
	SchemaVersion string `json:"schemaVersion,omitempty"`

	Nodes            []Node                 `json:"nodes"`
	NodeGroups       []NodeGroup            `json:"nodeGroups,omitempty"`
	FRouters         []FRouter              `json:"frouters"`
	Configs          []Config               `json:"configs"`
	GeoResources     []GeoResource          `json:"geoResources"`
	Components       []CoreComponent        `json:"components"`
	SystemProxy      SystemProxySettings    `json:"systemProxy"`
	ProxyConfig      ProxyConfig            `json:"proxyConfig"`
	FrontendSettings map[string]interface{} `json:"frontendSettings,omitempty"`

	GeneratedAt time.Time `json:"generatedAt"`
}

// InboundMode 入站模式
type InboundMode string

const (
	InboundSOCKS InboundMode = "socks"
	InboundHTTP  InboundMode = "http"
	InboundMixed InboundMode = "mixed"
	InboundTUN   InboundMode = "tun"
)

// CoreEngineKind 内核引擎类型
type CoreEngineKind string

const (
	EngineSingBox CoreEngineKind = "singbox"
	EngineClash   CoreEngineKind = "clash"
	EngineAuto    CoreEngineKind = "auto"
)

// ProxyConfig 代理运行配置（单例）
// 注意：对外一等单元是 FRouter；该配置只是“如何运行当前 FRouter”的参数集合，不存在多 Profile 的切换概念。
type ProxyConfig struct {
	InboundMode       InboundMode                   `json:"inboundMode"`
	InboundPort       int                           `json:"inboundPort,omitempty"`
	InboundConfig     *InboundConfiguration         `json:"inboundConfig,omitempty"`
	TUNSettings       *TUNConfiguration             `json:"tunSettings,omitempty"`
	ResolvedService   *ResolvedServiceConfiguration `json:"resolvedService,omitempty"`
	DNSConfig         *DNSConfiguration             `json:"dnsConfig,omitempty"`
	LogConfig         *LogConfiguration             `json:"logConfig,omitempty"`
	PerformanceConfig *PerformanceConfiguration     `json:"performanceConfig,omitempty"`
	PreferredEngine   CoreEngineKind                `json:"preferredEngine"`
	FRouterID         string                        `json:"frouterId"`
	UpdatedAt         time.Time                     `json:"updatedAt"`
}

// TUNConfiguration TUN 模式配置
type TUNConfiguration struct {
	InterfaceName          string   `json:"interfaceName"`
	MTU                    int      `json:"mtu"`
	Address                []string `json:"address"`
	AutoRoute              bool     `json:"autoRoute"`
	AutoRedirect           bool     `json:"autoRedirect"` // Linux: 使用 nftables 提供更好的路由性能
	StrictRoute            bool     `json:"strictRoute"`
	Stack                  string   `json:"stack"` // system, gvisor, mixed
	DNSHijack              bool     `json:"dnsHijack"`
	EndpointIndependentNat bool     `json:"endpointIndependentNat"`        // gvisor stack: 启用端点独立 NAT
	UDPTimeout             int      `json:"udpTimeout"`                    // UDP 会话超时时间（秒），默认 300
	RouteAddress           []string `json:"routeAddress,omitempty"`        // 自定义包含路由
	RouteExcludeAddress    []string `json:"routeExcludeAddress,omitempty"` // 自定义排除路由
}

// InboundConfiguration 入站配置（SOCKS/HTTP/Mixed 模式）
type InboundConfiguration struct {
	Listen         string                 `json:"listen"`                   // 监听地址，默认 127.0.0.1
	AllowLAN       bool                   `json:"allowLan"`                 // 允许局域网连接
	Authentication *InboundAuthentication `json:"authentication,omitempty"` // 认证配置
	Sniff          bool                   `json:"sniff"`                    // 嗅探域名
	SniffOverride  bool                   `json:"sniffOverride"`            // 覆盖目标地址
	SetSystemProxy bool                   `json:"setSystemProxy"`           // 自动设置系统代理
}

// InboundAuthentication 入站认证配置
type InboundAuthentication struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// LogConfiguration 日志配置
type LogConfiguration struct {
	Level     string `json:"level"`     // debug, info, warning, error, none
	Timestamp bool   `json:"timestamp"` // 显示时间戳
	Output    string `json:"output"`    // 输出位置：stdout, stderr, file path
}

// PerformanceConfiguration 性能配置
type PerformanceConfiguration struct {
	TCPFastOpen    bool   `json:"tcpFastOpen"`    // TCP Fast Open
	TCPMultiPath   bool   `json:"tcpMultiPath"`   // TCP MultiPath (MPTCP)
	UDPFragment    bool   `json:"udpFragment"`    // UDP 分片
	UDPTimeout     int    `json:"udpTimeout"`     // UDP 超时（秒）
	Sniff          bool   `json:"sniff"`          // 全局嗅探
	SniffOverride  bool   `json:"sniffOverride"`  // 全局嗅探覆盖
	DomainStrategy string `json:"domainStrategy"` // prefer_ipv4, prefer_ipv6, ipv4_only, ipv6_only
	DomainMatcher  string `json:"domainMatcher"`  // hybrid, linear (性能 vs 内存)
}

// ResolvedServiceConfiguration systemd-resolved 集成服务配置
type ResolvedServiceConfiguration struct {
	Enabled    bool   `json:"enabled"`    // 是否启用 resolved service
	Listen     string `json:"listen"`     // 监听地址，默认 127.0.0.53
	ListenPort int    `json:"listenPort"` // 监听端口，默认 53
}

// DNSConfiguration DNS 配置
type DNSConfiguration struct {
	UseResolved            bool     `json:"useResolved"`            // 使用 systemd-resolved 集成
	AcceptDefaultResolvers bool     `json:"acceptDefaultResolvers"` // 接受默认解析器作为 fallback
	RemoteServers          []string `json:"remoteServers"`          // 远程 DNS 服务器列表
	Strategy               string   `json:"strategy"`               // DNS 解析策略：prefer_ipv4, prefer_ipv6
}

// CoreEngineInfo 内核引擎信息
type CoreEngineInfo struct {
	Kind         CoreEngineKind `json:"kind"`
	BinaryPath   string         `json:"binaryPath"`
	Version      string         `json:"version"`
	Capabilities []string       `json:"capabilities"`
	Installed    bool           `json:"installed"`
}
