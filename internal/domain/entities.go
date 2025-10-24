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
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Address       string       `json:"address"`
	Port          int          `json:"port"`
	Protocol      NodeProtocol `json:"protocol"`
	Tags          []string     `json:"tags"`
	UploadBytes   int64        `json:"uploadBytes"`
	DownloadBytes int64        `json:"downloadBytes"`
	LastLatencyMS int64        `json:"lastLatencyMs"`
	LastSpeedMbps float64      `json:"lastSpeedMbps"`
	CreatedAt     time.Time    `json:"createdAt"`
	UpdatedAt     time.Time    `json:"updatedAt"`
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
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Type       GeoResourceType `json:"type"`
	SourceURL  string          `json:"sourceUrl"`
	Checksum   string          `json:"checksum"`
	Version    string          `json:"version"`
	LastSynced time.Time       `json:"lastSynced"`
	CreatedAt  time.Time       `json:"createdAt"`
	UpdatedAt  time.Time       `json:"updatedAt"`
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

type ServiceState struct {
	Nodes          []Node         `json:"nodes"`
	Configs        []Config       `json:"configs"`
	GeoResources   []GeoResource  `json:"geoResources"`
	TrafficProfile TrafficProfile `json:"trafficProfile"`
	GeneratedAt    time.Time      `json:"generatedAt"`
}
