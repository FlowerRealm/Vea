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
)

type Node struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Address        string         `json:"address"`
	Port           int            `json:"port"`
	Protocol       NodeProtocol   `json:"protocol"`
	Tags           []string       `json:"tags"`
	Security       *NodeSecurity  `json:"security,omitempty"`
	Transport      *NodeTransport `json:"transport,omitempty"`
	TLS            *NodeTLS       `json:"tls,omitempty"`
	SourceConfigID string         `json:"sourceConfigId,omitempty"`
	UploadBytes    int64          `json:"uploadBytes"`
	DownloadBytes  int64          `json:"downloadBytes"`
	LastLatencyMS  int64          `json:"lastLatencyMs"`
	LastLatencyAt  time.Time      `json:"lastLatencyAt"`
	LastSpeedMbps  float64        `json:"lastSpeedMbps"`
	LastSpeedAt    time.Time      `json:"lastSpeedAt"`
	LastSpeedError string         `json:"lastSpeedError,omitempty"`
	LastSelectedAt time.Time      `json:"lastSelectedAt"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}

type NodeSecurity struct {
	UUID         string   `json:"uuid,omitempty"`
	Password     string   `json:"password,omitempty"`
	Method       string   `json:"method,omitempty"`
	Flow         string   `json:"flow,omitempty"`
	Encryption   string   `json:"encryption,omitempty"`
	AlterID      int      `json:"alterId,omitempty"`
	Plugin       string   `json:"plugin,omitempty"`
	PluginOpts   string   `json:"pluginOpts,omitempty"`
	PluginBinary string   `json:"pluginBinary,omitempty"`
	ALPN         []string `json:"alpn,omitempty"`
}

type NodeTransport struct {
	Type        string            `json:"type,omitempty"`
	Host        string            `json:"host,omitempty"`
	Path        string            `json:"path,omitempty"`
	ServiceName string            `json:"serviceName,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
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
	ConfigFormatXray    ConfigFormat = "xray-json"
	ConfigFormatSingbox ConfigFormat = "singbox-json"
	ConfigFormatV2RayN  ConfigFormat = "v2rayn"
	ConfigFormatClash   ConfigFormat = "clash"
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
	UploadBytes        int64         `json:"uploadBytes"`
	DownloadBytes      int64         `json:"downloadBytes"`
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
	ComponentXray    CoreComponentKind = "xray"
	ComponentSingBox CoreComponentKind = "singbox"
	ComponentGeo     CoreComponentKind = "geo"
	ComponentGeneric CoreComponentKind = "generic"
)

type CoreComponent struct {
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	Kind               CoreComponentKind `json:"kind"`
	SourceURL          string            `json:"sourceUrl"`
	ArchiveType        string            `json:"archiveType"`
	AutoUpdateInterval time.Duration     `json:"autoUpdateInterval"`
	LastInstalledAt    time.Time         `json:"lastInstalledAt"`
	InstallDir         string            `json:"installDir"`
	LastVersion        string            `json:"lastVersion"`
	Checksum           string            `json:"checksum"`
	LastSyncError      string            `json:"lastSyncError"`
	Meta               map[string]string `json:"meta,omitempty"`
	CreatedAt          time.Time         `json:"createdAt"`
	UpdatedAt          time.Time         `json:"updatedAt"`
}

type TrafficRule struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Targets   []string  `json:"targets"`
	NodeID    string    `json:"nodeId"`
	Priority  int       `json:"priority"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type DNSSetting struct {
	Strategy string   `json:"strategy"`
	Servers  []string `json:"servers"`
}

type TrafficProfile struct {
	DefaultNodeID string        `json:"defaultNodeId"`
	DNS           DNSSetting    `json:"dns"`
	Rules         []TrafficRule `json:"rules"`
	UpdatedAt     time.Time     `json:"updatedAt"`
}

type SystemProxySettings struct {
	Enabled     bool      `json:"enabled"`
	IgnoreHosts []string  `json:"ignoreHosts"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type ServiceState struct {
	Nodes          []Node               `json:"nodes"`
	Configs        []Config             `json:"configs"`
	GeoResources   []GeoResource        `json:"geoResources"`
	Components     []CoreComponent      `json:"components"`
	TrafficProfile TrafficProfile       `json:"trafficProfile"`
	SystemProxy    SystemProxySettings  `json:"systemProxy"`
	GeneratedAt    time.Time            `json:"generatedAt"`
}
