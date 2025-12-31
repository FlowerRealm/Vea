package shared

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// 常量定义
const (
	DefaultConfigSyncInterval      = time.Hour
	DefaultComponentUpdateInterval = 12 * time.Hour
	MaxDownloadSize                = 50 << 20        // 50 MiB
	DownloadTimeout                = 5 * time.Minute // 支持慢速网络
	MeasurementPort                = 17891

	GeoDir        = "geo"
	ComponentFile = "artifact.bin"
)

// 全局变量
var (
	// ArtifactsRoot 是 artifacts 目录的绝对路径
	ArtifactsRoot string

	// SpeedTestTimeout 速度测试超时时间
	SpeedTestTimeout = 30 * time.Second
)

// GetArtifactsRoot 返回 artifacts 目录的绝对路径
func GetArtifactsRoot() string {
	return ArtifactsRoot
}

// HTTP 客户端
var (
	// HTTPClient 默认 HTTP 客户端
	HTTPClient = &http.Client{
		Timeout: DownloadTimeout,
	}

	// HTTPClientDirect 不使用代理的 HTTP 客户端
	HTTPClientDirect = newHTTPClient(true, false)

	// HTTPClientHTTP11 强制 HTTP/1.1 的客户端
	HTTPClientHTTP11 = newHTTPClient(false, true)

	// HTTPClientDirectHTTP11 不使用代理且强制 HTTP/1.1
	HTTPClientDirectHTTP11 = newHTTPClient(true, true)
)

func newHTTPClient(bypassProxy, forceHTTP11 bool) *http.Client {
	tr := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DialContext:         (&net.Dialer{Timeout: 60 * time.Second, KeepAlive: 60 * time.Second}).DialContext,
		ForceAttemptHTTP2:   !forceHTTP11,
		DisableKeepAlives:   true,
		TLSHandshakeTimeout: 30 * time.Second,
	}
	if bypassProxy {
		tr.Proxy = nil
	}
	if forceHTTP11 {
		tr.TLSClientConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			NextProtos: []string{"http/1.1"},
		}
	}
	return &http.Client{
		Timeout:   DownloadTimeout,
		Transport: tr,
	}
}

// Task 后台任务接口
type Task interface {
	Run()
	Stop()
}
