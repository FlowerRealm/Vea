package adapters

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"vea/backend/domain"
)

// getDefaultInterface 获取系统默认网络接口名称
func getDefaultInterface() string {
	if runtime.GOOS != "linux" {
		return ""
	}

	// 使用 ip route 获取默认接口
	cmd := exec.Command("ip", "route", "get", "8.8.8.8")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	// 解析输出: "8.8.8.8 via 192.168.1.1 dev eno1 src 192.168.1.108"
	re := regexp.MustCompile(`dev\s+(\S+)`)
	matches := re.FindStringSubmatch(string(out))
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// SingBoxAdapter sing-box 适配器
type SingBoxAdapter struct {
	SupportsDNSAction bool
}

// Kind 返回内核类型
func (a *SingBoxAdapter) Kind() domain.CoreEngineKind {
	return domain.EngineSingBox
}

// BinaryNames 返回二进制文件可能的名称
func (a *SingBoxAdapter) BinaryNames() []string {
	return []string{"sing-box", "sing-box.exe"}
}

// SupportedProtocols 返回支持的协议（sing-box 支持所有协议）
func (a *SingBoxAdapter) SupportedProtocols() []domain.NodeProtocol {
	return []domain.NodeProtocol{
		domain.ProtocolVLESS,
		domain.ProtocolVMess,
		domain.ProtocolTrojan,
		domain.ProtocolShadowsocks,
		"hysteria2", // sing-box 独有
		"tuic",      // sing-box 独有
	}
}

// SupportsInbound 检查是否支持入站模式（sing-box 支持所有模式）
func (a *SingBoxAdapter) SupportsInbound(mode domain.InboundMode) bool {
	return true
}

// BuildConfig 生成 sing-box 配置
func (a *SingBoxAdapter) BuildConfig(profile domain.ProxyProfile, nodes []domain.Node, geo GeoFiles) ([]byte, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no nodes available to build sing-box config")
	}

	// 1. 构建入站配置
	inbounds, err := a.buildInbounds(profile)
	if err != nil {
		return nil, err
	}

	// 2. 构建出站配置
	outbounds, defaultTag := a.buildOutbounds(nodes, profile.DefaultNode, geo)

	// 添加 direct 和 block 出站
	// direct 需要 bind_interface 防止被 TUN 捕获（两层代理模式下 SOCKS 层的 direct 流量）
	directOutbound := map[string]interface{}{"type": "direct", "tag": "direct"}
	if iface := getDefaultInterface(); iface != "" {
		directOutbound["bind_interface"] = iface
	}
	outbounds = append(outbounds, directOutbound, map[string]interface{}{"type": "block", "tag": "block"})

	// 3. 构建路由规则
	route := a.buildRoute(profile, defaultTag, geo, nodes)

	// 4. 构建 DNS 配置（TUN 模式下优先走代理的远程 DNS）
	dnsConfig := a.buildDNS(profile, defaultTag)

	// 5. 构建日志配置
	logConfig := a.buildLog(profile)

	// 6. 构建 services 配置（resolved service）
	services := a.buildServices(profile)

	// 7. 组装完整配置
	config := map[string]interface{}{
		"log":       logConfig,
		"inbounds":  inbounds,
		"outbounds": outbounds,
		"route":     route,
		"dns":       dnsConfig,
	}

	// 添加 services（如果有）
	// 注意：services 必须是数组，即使只有一个元素
	if len(services) > 0 {
		config["services"] = services
	}

	// 8. 添加实验性功能配置
	experimental := a.buildExperimental(profile)
	if len(experimental) > 0 {
		config["experimental"] = experimental
	}

	return json.MarshalIndent(config, "", "  ")
}

// RequiresPrivileges 检查是否需要特权（TUN 模式需要）
func (a *SingBoxAdapter) RequiresPrivileges(profile domain.ProxyProfile) bool {
	return profile.InboundMode == domain.InboundTUN
}

// BuildTUNOnlyConfig 生成纯 TUN 配置（v2rayN 风格）
// proxy outbound 指向本地 SOCKS，由另一个进程处理实际代理
func (a *SingBoxAdapter) BuildTUNOnlyConfig(localSOCKSPort int, geo GeoFiles) ([]byte, error) {
	// TUN inbound
	tun := map[string]interface{}{
		"type":                       "tun",
		"tag":                        "tun-in",
		"interface_name":             "singbox_tun",
		"address":                    []string{"172.18.0.1/30"},
		"mtu":                        9000,
		"auto_route":                 true,
		"strict_route":               true,
		"stack":                      "gvisor",
		"sniff":                      true,
		"sniff_override_destination": true, // 用 sniff 检测到的域名覆盖被污染的 IP
	}

	// Outbounds: proxy -> 本地 SOCKS, direct, block, dns
	outbounds := []map[string]interface{}{
		{
			"type":        "socks",
			"tag":         "proxy",
			"server":      "127.0.0.1",
			"server_port": localSOCKSPort,
			"version":     "5",
		},
		{"type": "direct", "tag": "direct"},
		{"type": "block", "tag": "block"},
		{"type": "dns", "tag": "dns_out"},
	}

	// DNS 配置
	dns := map[string]interface{}{
		"servers": []map[string]interface{}{
			{"tag": "remote", "address": "tcp://8.8.8.8", "strategy": "prefer_ipv4", "detour": "proxy"},
			{"tag": "local", "address": "223.5.5.5", "strategy": "prefer_ipv4", "detour": "direct"},
			{"tag": "local-fallback", "address": "8.8.8.8", "strategy": "prefer_ipv4", "detour": "direct"},
			{"tag": "block", "address": "rcode://success"},
		},
		"rules": []map[string]interface{}{
			{"server": "remote", "clash_mode": "Global"},
			{"server": "local", "clash_mode": "Direct"},
			{"server": "local", "rule_set": []string{"geosite-cn"}},
			{"server": "block", "rule_set": []string{"geosite-category-ads-all"}},
		},
		"final": "remote",
	}

	// 路由规则
	ruleSetDir := filepath.Join(geo.ArtifactsDir, "core", "sing-box", "rule-set")
	route := map[string]interface{}{
		"auto_detect_interface": true,
		"rules": []map[string]interface{}{
			// 排除代理进程，避免 SOCKS 层流量被 TUN 捕获形成循环
			{"outbound": "direct", "process_name": []string{"sing-box", "xray", "v2ray"}},
			{"outbound": "proxy", "clash_mode": "Global"},
			{"outbound": "direct", "clash_mode": "Direct"},
			{"outbound": "dns_out", "protocol": []string{"dns"}},
			{"outbound": "block", "rule_set": []string{"geosite-category-ads-all"}},
			{"outbound": "direct", "ip_is_private": true},
			{"outbound": "direct", "rule_set": []string{"geoip-cn"}},
			{"outbound": "direct", "rule_set": []string{"geosite-cn"}},
		},
		"rule_set": []map[string]interface{}{
			{"tag": "geosite-category-ads-all", "type": "local", "format": "binary", "path": ruleSetDir + "/geosite-category-ads-all.srs"},
			{"tag": "geosite-cn", "type": "local", "format": "binary", "path": ruleSetDir + "/geosite-cn.srs"},
			{"tag": "geoip-cn", "type": "local", "format": "binary", "path": ruleSetDir + "/geoip-cn.srs"},
		},
		"final": "proxy",
	}

	config := map[string]interface{}{
		"log":       map[string]interface{}{"level": "info", "timestamp": true},
		"inbounds":  []map[string]interface{}{tun},
		"outbounds": outbounds,
		"dns":       dns,
		"route":     route,
	}

	return json.MarshalIndent(config, "", "  ")
}

// buildInbounds 构建入站配置
func (a *SingBoxAdapter) buildInbounds(profile domain.ProxyProfile) ([]map[string]interface{}, error) {
	var inbounds []map[string]interface{}

	switch profile.InboundMode {
	case domain.InboundTUN:
		if profile.TUNSettings == nil {
			return nil, fmt.Errorf("TUN mode requires TUNSettings")
		}

		// sing-box 1.12+ 使用新的地址字段格式
		tun := map[string]interface{}{
			"type":                       "tun",
			"tag":                        "tun-in",
			"interface_name":             profile.TUNSettings.InterfaceName,
			"mtu":                        profile.TUNSettings.MTU,
			"address":                    profile.TUNSettings.Address, // 新格式：address 替代 inet4_address
			"auto_route":                 true,                        // 强制开启自动路由
			"strict_route":               false,                       // 关闭严格路由，允许 bind_interface 生效
			"stack":                      "gvisor", // 使用 gvisor stack，和 v2rayN 一致
			"sniff":                      true,
			"sniff_override_destination": true,
		}

		// 简化配置：只使用 auto_route，通过 process_name 规则排除代理进程避免循环

		inbounds = append(inbounds, tun)

	case domain.InboundMixed:
		// sing-box 的 mixed 模式（HTTP + SOCKS）
		mixed := map[string]interface{}{
			"type":        "mixed",
			"tag":         "mixed-in",
			"listen":      "127.0.0.1",
			"listen_port": profile.InboundPort,
		}
		a.applyInboundConfig(mixed, profile)
		inbounds = append(inbounds, mixed)

	case domain.InboundSOCKS:
		socks := map[string]interface{}{
			"type":        "socks",
			"tag":         "socks-in",
			"listen":      "127.0.0.1",
			"listen_port": profile.InboundPort,
		}
		a.applyInboundConfig(socks, profile)
		inbounds = append(inbounds, socks)

	case domain.InboundHTTP:
		http := map[string]interface{}{
			"type":        "http",
			"tag":         "http-in",
			"listen":      "127.0.0.1",
			"listen_port": profile.InboundPort,
		}
		a.applyInboundConfig(http, profile)
		inbounds = append(inbounds, http)
	}

	return inbounds, nil
}

// buildOutbounds 构建出站配置
func (a *SingBoxAdapter) buildOutbounds(nodes []domain.Node, defaultNodeID string, geo GeoFiles) ([]map[string]interface{}, string) {
	var (
		outbounds  []map[string]interface{}
		defaultTag string
	)

	// Legacy: 专用 DNS 出站，处理 DNS 数据包
	if !a.SupportsDNSAction {
		outbounds = append(outbounds, map[string]interface{}{
			"type": "dns",
			"tag":  "dns-out",
		})
	}

	for _, node := range nodes {
		outbound, tag := a.buildOutbound(node, geo)
		if outbound != nil {
			outbounds = append(outbounds, outbound)

			// 确定默认节点
			if defaultTag == "" || node.ID == defaultNodeID {
				defaultTag = tag
			}
		}
	}

	if defaultTag == "" && len(outbounds) > 0 {
		// 使用第一个节点作为默认
		if tag, ok := outbounds[0]["tag"].(string); ok {
			defaultTag = tag
		}
	}

	return outbounds, defaultTag
}

// buildOutbound 构建单个节点的出站配置
func (a *SingBoxAdapter) buildOutbound(node domain.Node, geo GeoFiles) (map[string]interface{}, string) {
	tag := fmt.Sprintf("node-%s", shortenID(node.ID))
	outbound := map[string]interface{}{
		"tag":         tag,
		"server":      node.Address,
		"server_port": node.Port,
	}

	sec := node.Security
	if sec == nil {
		sec = &domain.NodeSecurity{}
	}

	switch node.Protocol {
	case domain.ProtocolVLESS:
		outbound["type"] = "vless"
		outbound["uuid"] = sec.UUID
		if sec.Flow != "" {
			outbound["flow"] = sec.Flow
		}

	case domain.ProtocolVMess:
		outbound["type"] = "vmess"
		outbound["uuid"] = sec.UUID
		outbound["alter_id"] = sec.AlterID
		if sec.Encryption != "" {
			outbound["security"] = sec.Encryption
		}

	case domain.ProtocolTrojan:
		outbound["type"] = "trojan"
		outbound["password"] = sec.Password

	case domain.ProtocolShadowsocks:
		outbound["type"] = "shadowsocks"
		outbound["method"] = sec.Method
		outbound["password"] = sec.Password

		// 处理插件：sing-box 直接支持 obfs-local，不需要转换
		if sec.Plugin == "obfs-local" {
			// sing-box 原生支持 obfs-local (simple-obfs)
			outbound["plugin"] = "obfs-local"
			outbound["plugin_opts"] = sec.PluginOpts
		} else if sec.Plugin != "" {
			// 其他插件（v2ray-plugin 等）保留原样
			outbound["plugin"] = sec.Plugin
			if sec.PluginOpts != "" {
				outbound["plugin_opts"] = sec.PluginOpts
			}
		}

	case "hysteria2":
		outbound["type"] = "hysteria2"
		outbound["password"] = sec.Password
		outbound["up_mbps"] = 100
		outbound["down_mbps"] = 100

	case "tuic":
		outbound["type"] = "tuic"
		outbound["uuid"] = sec.UUID
		outbound["password"] = sec.Password
		outbound["congestion_control"] = "bbr"

	default:
		// 不支持的协议，跳过
		return nil, ""
	}

	// TLS 配置
	if node.TLS != nil && node.TLS.Enabled {
		tls := map[string]interface{}{
			"enabled":  true,
			"insecure": node.TLS.Insecure,
		}
		if node.TLS.ServerName != "" {
			tls["server_name"] = node.TLS.ServerName
		}
		if len(node.TLS.ALPN) > 0 {
			tls["alpn"] = node.TLS.ALPN
		}

		// Reality 配置
		if node.TLS.Type == "reality" || node.TLS.RealityPublicKey != "" {
			tls["reality"] = map[string]interface{}{
				"enabled":    true,
				"public_key": node.TLS.RealityPublicKey,
				"short_id":   node.TLS.RealityShortID,
			}
		}

		outbound["tls"] = tls
	}

	// 传输配置
	if node.Transport != nil {
		transport := map[string]interface{}{}

		switch node.Transport.Type {
		case "ws":
			transport["type"] = "ws"
			if node.Transport.Path != "" {
				transport["path"] = node.Transport.Path
			}
			if node.Transport.Host != "" {
				transport["headers"] = map[string]string{
					"Host": node.Transport.Host,
				}
			}

		case "grpc":
			transport["type"] = "grpc"
			if node.Transport.ServiceName != "" {
				transport["service_name"] = node.Transport.ServiceName
			}

		case "http", "h2":
			transport["type"] = "http"
			if node.Transport.Host != "" {
				transport["host"] = []string{node.Transport.Host}
			}
			if node.Transport.Path != "" {
				transport["path"] = node.Transport.Path
			}
		}

		if len(transport) > 0 {
			outbound["transport"] = transport
		}
	}

	// 添加 domain_resolver：用直连 DNS 解析代理服务器地址，打破 DNS bootstrap 循环
	outbound["domain_resolver"] = "dns-local"

	return outbound, tag
}

// buildRoute 构建路由配置
func (a *SingBoxAdapter) buildRoute(profile domain.ProxyProfile, defaultTag string, geo GeoFiles, nodes []domain.Node) map[string]interface{} {
	// 使用本地 rule-set 文件（在组件下载时捆绑）
	// 必须使用绝对路径，因为 Electron 从 frontend/ 目录启动
	ruleSetDir := geo.ArtifactsDir + "/core/sing-box/rule-set"
	ruleSets := []map[string]interface{}{
		{
			"tag":    "geosite-category-ads-all",
			"type":   "local",
			"format": "binary",
			"path":   ruleSetDir + "/geosite-category-ads-all.srs",
		},
		{
			"tag":    "geosite-cn",
			"type":   "local",
			"format": "binary",
			"path":   ruleSetDir + "/geosite-cn.srs",
		},
		{
			"tag":    "geoip-cn",
			"type":   "local",
			"format": "binary",
			"path":   ruleSetDir + "/geoip-cn.srs",
		},
	}

	rules := []map[string]interface{}{
		{
			"rule_set": []string{"geosite-category-ads-all"},
			"outbound": "block",
		},
	}

	if a.SupportsDNSAction {
		rules = append(rules, map[string]interface{}{
			"protocol": []string{"dns"},
			"action":   "dns",
		})
	} else {
		rules = append(rules, map[string]interface{}{
			"protocol": []string{"dns"},
			"outbound": "dns-out",
		})
	}

	// TUN 模式：排除代理进程，避免流量循环
	// sing-box 发出的流量直接走 direct，不经过 TUN
	if profile.InboundMode == domain.InboundTUN {
		rules = append(rules, map[string]interface{}{
			"process_name": []string{
				"sing-box",
				"xray",
				"v2ray",
			},
			"outbound": "direct",
		})
	}

	// 提取所有代理服务器域名，确保它们走 direct（避免循环）
	// 代理服务器的 IP 地址必须直连，否则会形成循环：TUN → proxy → DNS → proxy IP → TUN
	proxyDomains := []string{}
	for _, node := range nodes {
		if node.Address != "" {
			// 检查是否是域名（不是 IP）
			if !isIPAddress(node.Address) {
				proxyDomains = append(proxyDomains, node.Address)
			}
		}
	}
	// 添加代理服务器域名直连规则（优先级高于 geosite-cn）
	if len(proxyDomains) > 0 {
		rules = append(rules, map[string]interface{}{
			"domain":   proxyDomains,
			"outbound": "direct",
		})
	}

	// 私有 IP 直连（局域网、本地回环等）
	// 官方文档推荐：https://sing-box.sagernet.org/configuration/route/rule/
	rules = append(rules, map[string]interface{}{
		"ip_is_private": true,
		"outbound":      "direct",
	})

	// 若要全局代理，把 direct 改为 defaultTag
	rules = append(rules,
		map[string]interface{}{
			"rule_set": []string{"geosite-cn"},
			"outbound": "direct",
		},
		map[string]interface{}{
			"rule_set": []string{"geoip-cn"},
			"outbound": "direct",
		},
	)

	// 确定 default_domain_resolver - 用于解析出站域名
	// 使用 dns-local 直连解析，避免 DNS 循环依赖
	defaultDomainResolver := "dns-local"

	route := map[string]interface{}{
		"rules":                   rules,
		"rule_set":                ruleSets,
		"final":                   defaultTag,
		"auto_detect_interface":   true,
		"default_domain_resolver": defaultDomainResolver,
	}

	// TUN 模式下设置默认出站接口
	if profile.InboundMode == domain.InboundTUN {
		if iface := getDefaultInterface(); iface != "" {
			route["default_interface"] = iface
		}
	}

	return route
}

// buildDNS 构建 DNS 配置
// 使用分流策略：中国域名走国内 DNS（直连），国际域名走远程 DNS（代理）
// 参考官方文档: https://sing-box.sagernet.org/configuration/dns/
func (a *SingBoxAdapter) buildDNS(profile domain.ProxyProfile, defaultTag string) map[string]interface{} {
	strategy := "prefer_ipv4"
	if profile.DNSConfig != nil && profile.DNSConfig.Strategy != "" {
		strategy = profile.DNSConfig.Strategy
	}

	// DNS 服务器配置 (sing-box 1.12.0+ 新格式)
	// 1. dns-local: 国内 DNS，用于解析国内域名（直连）
	// 2. dns-remote: 国际 DNS，用于解析国际域名（走代理）
	servers := []map[string]interface{}{
		{
			"tag":    "dns-local",
			"type":   "udp",
			"server": "223.5.5.5", // 阿里 DNS，国内访问快
			"detour": "direct",
		},
		{
			"tag":    "dns-remote",
			"type":   "udp",
			"server": "8.8.8.8", // Google DNS，走代理
			"detour": defaultTag,
		},
	}

	// DNS 规则：中国域名走国内 DNS，其他走远程 DNS
	rules := []map[string]interface{}{
		{
			"rule_set": []string{"geosite-cn"},
			"server":   "dns-local",
		},
	}

	return map[string]interface{}{
		"servers":  servers,
		"rules":    rules,
		"strategy": strategy,
		"final":    "dns-remote", // 默认走远程 DNS（代理）
	}
}

// applyInboundConfig 应用 InboundConfig 到 inbound 配置
func (a *SingBoxAdapter) applyInboundConfig(inbound map[string]interface{}, profile domain.ProxyProfile) {
	if profile.InboundConfig == nil {
		return
	}

	cfg := profile.InboundConfig

	// 监听地址
	if cfg.Listen != "" {
		inbound["listen"] = cfg.Listen
	}

	// 嗅探配置
	if cfg.Sniff {
		inbound["sniff"] = true
		if cfg.SniffOverride {
			inbound["sniff_override_destination"] = true
		}
	}

	// 认证配置
	if cfg.Authentication != nil && cfg.Authentication.Username != "" {
		inbound["users"] = []map[string]interface{}{
			{
				"username": cfg.Authentication.Username,
				"password": cfg.Authentication.Password,
			},
		}
	}
}

// buildLog 构建日志配置
func (a *SingBoxAdapter) buildLog(profile domain.ProxyProfile) map[string]interface{} {
	if profile.LogConfig == nil {
		return map[string]interface{}{
			"level":     "info",
			"timestamp": true,
		}
	}

	logCfg := map[string]interface{}{
		"level":     profile.LogConfig.Level,
		"timestamp": profile.LogConfig.Timestamp,
	}

	// 输出位置（如果不是 stdout/stderr）
	if profile.LogConfig.Output != "stdout" && profile.LogConfig.Output != "stderr" && profile.LogConfig.Output != "" {
		logCfg["output"] = profile.LogConfig.Output
	}

	return logCfg
}

// buildServices 构建 services 配置（resolved service）
func (a *SingBoxAdapter) buildServices(profile domain.ProxyProfile) []map[string]interface{} {
	var services []map[string]interface{}

	// 如果启用 resolved service
	if profile.ResolvedService != nil && profile.ResolvedService.Enabled {
		services = append(services, map[string]interface{}{
			"type":        "resolved",
			"tag":         "resolved",
			"listen":      profile.ResolvedService.Listen,
			"listen_port": profile.ResolvedService.ListenPort,
		})
	}

	return services
}

// buildExperimental 构建实验性功能配置
func (a *SingBoxAdapter) buildExperimental(profile domain.ProxyProfile) map[string]interface{} {
	if profile.PerformanceConfig == nil {
		return nil
	}

	experimental := make(map[string]interface{})

	// Cache file（可选，用于缓存）
	// experimental["cache_file"] = map[string]interface{}{
	// 	"enabled": true,
	// }

	// Clash API（可选）
	// experimental["clash_api"] = map[string]interface{}{
	// 	"external_controller": "127.0.0.1:9090",
	// }

	return experimental
}

// isIPAddress 检查字符串是否是 IP 地址（IPv4 或 IPv6）
func isIPAddress(s string) bool {
	// 简单检查：包含冒号是 IPv6，全是数字和点是 IPv4
	if strings.Contains(s, ":") {
		return true // 可能是 IPv6
	}
	// IPv4: 检查是否符合 x.x.x.x 格式
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return false
	}
	for _, part := range parts {
		if len(part) == 0 || len(part) > 3 {
			return false
		}
		for _, c := range part {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	return true
}
