package adapters

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"vea/backend/domain"
)

const (
	xrayDirectTag         = "direct"
	xrayBlockTag          = "block"
	xrayDefaultListenAddr = "127.0.0.1"
)

// XrayAdapter Xray-core 适配器
type XrayAdapter struct{}

// Kind 返回内核类型
func (a *XrayAdapter) Kind() domain.CoreEngineKind {
	return domain.EngineXray
}

// BinaryNames 返回二进制文件可能的名称
func (a *XrayAdapter) BinaryNames() []string {
	return []string{"xray", "xray.exe"}
}

// SupportedProtocols 返回支持的协议
func (a *XrayAdapter) SupportedProtocols() []domain.NodeProtocol {
	return []domain.NodeProtocol{
		domain.ProtocolVLESS,
		domain.ProtocolVMess,
		domain.ProtocolTrojan,
		domain.ProtocolShadowsocks,
	}
}

// SupportsInbound 检查是否支持入站模式
func (a *XrayAdapter) SupportsInbound(mode domain.InboundMode) bool {
	// Xray 不支持 TUN 模式（遇到 TUN 强制转到 sing-box）
	return mode == domain.InboundSOCKS ||
		mode == domain.InboundHTTP ||
		mode == domain.InboundMixed
}

// BuildConfig 生成 Xray 配置
func (a *XrayAdapter) BuildConfig(profile domain.ProxyProfile, nodes []domain.Node, geo GeoFiles) ([]byte, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no nodes available to build xray config")
	}

	// 1. 构建出站配置
	outbounds, defaultTag, err := a.buildOutbounds(nodes, profile.DefaultNode, profile.XrayConfig)
	if err != nil {
		return nil, err
	}

	// 添加 direct 和 block 出站
	outbounds = append(outbounds,
		map[string]interface{}{"tag": xrayDirectTag, "protocol": "freedom"},
		map[string]interface{}{"tag": xrayBlockTag, "protocol": "blackhole"},
	)

	// 2. 构建入站配置
	inbounds := a.buildInbounds(profile)

	// 3. 构建路由规则
	routing := a.buildRouting(defaultTag, geo, profile.XrayConfig)

	// 4. DNS 配置
	dnsConfig := a.buildDNS(profile)

	// 5. 日志配置
	logConfig := a.buildLog(profile)

	// 6. 组装完整配置
	config := map[string]interface{}{
		"log":       logConfig,
		"inbounds":  inbounds,
		"outbounds": outbounds,
		"routing":   routing,
		"dns":       dnsConfig,
	}

	return json.MarshalIndent(config, "", "  ")
}

// RequiresPrivileges Xray 不支持 TUN，所以永远不需要特权
func (a *XrayAdapter) RequiresPrivileges(profile domain.ProxyProfile) bool {
	return false
}

// buildInbounds 构建入站配置
func (a *XrayAdapter) buildInbounds(profile domain.ProxyProfile) []map[string]interface{} {
	var inbounds []map[string]interface{}

	switch profile.InboundMode {
	case domain.InboundSOCKS:
		inbounds = append(inbounds, map[string]interface{}{
			"tag":      "socks-in",
			"listen":   xrayDefaultListenAddr,
			"port":     profile.InboundPort,
			"protocol": "socks",
			"settings": map[string]interface{}{
				"auth": "noauth",
				"udp":  true,
			},
			"sniffing": map[string]interface{}{
				"enabled":      true,
				"destOverride": []string{"http", "tls"},
			},
		})

	case domain.InboundHTTP:
		inbounds = append(inbounds, map[string]interface{}{
			"tag":      "http-in",
			"listen":   xrayDefaultListenAddr,
			"port":     profile.InboundPort,
			"protocol": "http",
			"settings": map[string]interface{}{
				"allowTransparent": false,
			},
		})

	case domain.InboundMixed:
		// Xray 的 mixed 模式通过组合 HTTP + SOCKS 实现
		inbounds = append(inbounds,
			map[string]interface{}{
				"tag":      "http-in",
				"listen":   xrayDefaultListenAddr,
				"port":     profile.InboundPort,
				"protocol": "http",
			},
			map[string]interface{}{
				"tag":      "socks-in",
				"listen":   xrayDefaultListenAddr,
				"port":     profile.InboundPort + 1,
				"protocol": "socks",
				"settings": map[string]interface{}{
					"auth": "noauth",
					"udp":  true,
				},
			},
		)
	}

	return inbounds
}

// buildOutbounds 构建出站配置
func (a *XrayAdapter) buildOutbounds(nodes []domain.Node, defaultNodeID string, xrayConfig *domain.XrayConfiguration) ([]map[string]interface{}, string, error) {
	var (
		outbounds  []map[string]interface{}
		defaultTag string
	)

	for _, node := range nodes {
		outbound, tag, err := a.buildOutbound(node)
		if err != nil {
			return nil, "", fmt.Errorf("build outbound for %s: %w", node.Name, err)
		}

		// 应用 Mux 配置
		if xrayConfig != nil {
			outbound["mux"] = map[string]interface{}{
				"enabled":     xrayConfig.MuxEnabled,
				"concurrency": xrayConfig.MuxConcurrency,
			}
		} else {
			outbound["mux"] = map[string]interface{}{
				"enabled":     false,
				"concurrency": 8,
			}
		}

		outbounds = append(outbounds, outbound)

		// 确定默认节点
		if defaultTag == "" || node.ID == defaultNodeID {
			defaultTag = tag
		}
	}

	if defaultTag == "" {
		return nil, "", fmt.Errorf("no valid node tags")
	}

	return outbounds, defaultTag, nil
}

// buildOutbound 构建单个节点的出站配置
func (a *XrayAdapter) buildOutbound(node domain.Node) (map[string]interface{}, string, error) {
	tag := fmt.Sprintf("node-%s", shortenID(node.ID))
	outbound := map[string]interface{}{
		"tag":      tag,
		"protocol": strings.ToLower(string(node.Protocol)),
	}

	sec := node.Security
	if sec == nil {
		sec = &domain.NodeSecurity{}
	}

	var settings map[string]interface{}

	switch node.Protocol {
	case domain.ProtocolVMess:
		settings = map[string]interface{}{
			"vnext": []map[string]interface{}{
				{
					"address": node.Address,
					"port":    node.Port,
					"users": []map[string]interface{}{
						{
							"id":       sec.UUID,
							"alterId":  sec.AlterID,
							"security": firstNonEmpty(sec.Encryption, "auto"),
						},
					},
				},
			},
		}

	case domain.ProtocolVLESS:
		settings = map[string]interface{}{
			"vnext": []map[string]interface{}{
				{
					"address": node.Address,
					"port":    node.Port,
					"users": []map[string]interface{}{
						{
							"id":         sec.UUID,
							"encryption": firstNonEmpty(sec.Encryption, "none"),
							"flow":       sec.Flow,
						},
					},
				},
			},
		}

	case domain.ProtocolTrojan:
		settings = map[string]interface{}{
			"servers": []map[string]interface{}{
				{
					"address":  node.Address,
					"port":     node.Port,
					"password": sec.Password,
					"flow":     sec.Flow,
				},
			},
		}

	case domain.ProtocolShadowsocks:
		settings = map[string]interface{}{
			"servers": []map[string]interface{}{
				{
					"address":  node.Address,
					"port":     node.Port,
					"method":   sec.Method,
					"password": sec.Password,
				},
			},
		}

	default:
		return nil, "", fmt.Errorf("unsupported protocol %s", node.Protocol)
	}

	outbound["settings"] = settings

	// 添加 streamSettings（传输和 TLS）
	if node.Transport != nil || (node.TLS != nil && node.TLS.Enabled) {
		outbound["streamSettings"] = buildXrayStreamSettings(node.Transport, node.TLS)
	}

	return outbound, tag, nil
}

// buildRouting 构建路由规则
func (a *XrayAdapter) buildRouting(defaultTag string, geo GeoFiles, xrayConfig *domain.XrayConfiguration) map[string]interface{} {
	rules := []map[string]interface{}{
		{
			"type":        "field",
			"domain":      []string{"geosite:category-ads-all"},
			"outboundTag": xrayBlockTag,
		},
		{
			"type":        "field",
			"domain":      []string{"geosite:cn", "geosite:private"},
			"outboundTag": xrayDirectTag,
		},
		{
			"type":        "field",
			"ip":          []string{"geoip:cn", "geoip:private"},
			"outboundTag": xrayDirectTag,
		},
		{
			"type":        "field",
			"network":     "tcp,udp",
			"outboundTag": defaultTag,
		},
	}

	domainStrategy := "AsIs"
	if xrayConfig != nil && xrayConfig.DomainStrategy != "" {
		domainStrategy = xrayConfig.DomainStrategy
	}

	routing := map[string]interface{}{
		"domainStrategy": domainStrategy,
		"rules":          rules,
	}

	// 添加 Geo 文件路径
	if geo.GeoIP != "" {
		routing["geoip"] = []map[string]interface{}{
			{"file": filepath.Clean(geo.GeoIP)},
		}
	}
	if geo.GeoSite != "" {
		routing["geosite"] = []map[string]interface{}{
			{"file": filepath.Clean(geo.GeoSite)},
		}
	}

	return routing
}

// buildXrayStreamSettings 构建传输层配置
func buildXrayStreamSettings(transport *domain.NodeTransport, tls *domain.NodeTLS) map[string]interface{} {
	stream := map[string]interface{}{}

	// 传输协议
	if transport != nil {
		network := strings.ToLower(transport.Type)
		if network == "" {
			network = "tcp"
		}
		stream["network"] = network

		switch network {
		case "ws":
			wsSettings := map[string]interface{}{}
			if transport.Path != "" {
				wsSettings["path"] = transport.Path
			}
			if transport.Host != "" {
				wsSettings["headers"] = map[string]string{
					"Host": transport.Host,
				}
			}
			stream["wsSettings"] = wsSettings

		case "grpc":
			grpcSettings := map[string]interface{}{}
			if transport.ServiceName != "" {
				grpcSettings["serviceName"] = transport.ServiceName
			}
			stream["grpcSettings"] = grpcSettings

		case "http", "h2":
			httpSettings := map[string]interface{}{}
			if transport.Host != "" {
				httpSettings["host"] = []string{transport.Host}
			}
			if transport.Path != "" {
				httpSettings["path"] = transport.Path
			}
			stream["httpSettings"] = httpSettings
		}
	}

	// TLS 配置
	if tls != nil && tls.Enabled {
		tlsSettings := map[string]interface{}{
			"allowInsecure": tls.Insecure,
		}

		if tls.ServerName != "" {
			tlsSettings["serverName"] = tls.ServerName
		}
		if tls.Fingerprint != "" {
			tlsSettings["fingerprint"] = tls.Fingerprint
		}
		if len(tls.ALPN) > 0 {
			tlsSettings["alpn"] = tls.ALPN
		}

		// Reality 配置
		if tls.Type == "reality" || tls.RealityPublicKey != "" {
			tlsSettings["reality"] = map[string]interface{}{
				"publicKey": tls.RealityPublicKey,
				"shortId":   tls.RealityShortID,
			}
		}

		stream["security"] = "tls"
		stream["tlsSettings"] = tlsSettings
	}

	return stream
}

// shortenID 截短 ID 用于标签
func shortenID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// firstNonEmpty 返回第一个非空字符串
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// buildDNS 构建 DNS 配置
func (a *XrayAdapter) buildDNS(profile domain.ProxyProfile) map[string]interface{} {
	servers := []string{"1.1.1.1", "8.8.8.8"}
	if profile.XrayConfig != nil && len(profile.XrayConfig.DNSServers) > 0 {
		servers = profile.XrayConfig.DNSServers
	}
	return map[string]interface{}{
		"servers": servers,
	}
}

// buildLog 构建日志配置
func (a *XrayAdapter) buildLog(profile domain.ProxyProfile) map[string]interface{} {
	logLevel := "info"
	if profile.LogConfig != nil && profile.LogConfig.Level != "" {
		logLevel = profile.LogConfig.Level
	}
	return map[string]interface{}{
		"loglevel": logLevel,
	}
}
