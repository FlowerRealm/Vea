package adapters

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"vea/backend/domain"
	"vea/backend/service/nodegroup"
)

// getDefaultInterface 获取系统默认网络接口名称
func getDefaultInterface() string {
	if runtime.GOOS != "linux" {
		return ""
	}

	// 注意：TUN 模式开启后，系统的默认路由可能会被策略路由指向 tun 设备。
	// 这时如果直接用 `ip route get 8.8.8.8`，很容易拿到 tun0，导致生成的配置把出站绑回 tun0，形成死循环。
	// 所以优先从 main 表读默认路由，拿到真实的物理出口网卡。
	candidates := [][]string{
		{"ip", "route", "show", "default", "table", "main"},
		{"ip", "route", "get", "8.8.8.8"},
	}

	re := regexp.MustCompile(`dev\s+(\S+)`)
	for _, args := range candidates {
		out, err := exec.Command(args[0], args[1:]...).Output()
		if err != nil {
			continue
		}

		matches := re.FindStringSubmatch(string(out))
		if len(matches) < 2 {
			continue
		}

		iface := strings.TrimSpace(matches[1])
		if iface == "" {
			continue
		}
		if iface == "lo" || strings.HasPrefix(iface, "tun") || strings.HasPrefix(iface, "tap") {
			continue
		}
		return iface
	}

	return ""
}

func formatSingBoxDurationSeconds(seconds int) string {
	if seconds <= 0 {
		return ""
	}
	if seconds%3600 == 0 {
		return fmt.Sprintf("%dh", seconds/3600)
	}
	if seconds%60 == 0 {
		return fmt.Sprintf("%dm", seconds/60)
	}
	return fmt.Sprintf("%ds", seconds)
}

// SingBoxAdapter sing-box 适配器
type SingBoxAdapter struct{}

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

// BuildConfig 根据运行计划生成 sing-box 配置
func (a *SingBoxAdapter) BuildConfig(plan nodegroup.RuntimePlan, geo GeoFiles) ([]byte, error) {
	switch plan.Purpose {
	case nodegroup.PurposeProxy:
		return a.buildProxyConfig(plan, geo)
	case nodegroup.PurposeMeasurement:
		return a.buildMeasurementConfig(plan, geo)
	default:
		return nil, fmt.Errorf("unsupported plan purpose: %s", plan.Purpose)
	}
}

func (a *SingBoxAdapter) buildProxyConfig(plan nodegroup.RuntimePlan, geo GeoFiles) ([]byte, error) {
	inbounds, err := a.buildInbounds(plan.ProxyConfig)
	if err != nil {
		return nil, err
	}

	outbounds, tagMap, err := a.buildOutbounds(plan, geo)
	if err != nil {
		return nil, err
	}

	defaultTag, err := singboxOutboundTag(plan.Compiled.Default, tagMap)
	if err != nil {
		return nil, err
	}

	directOutbound := map[string]interface{}{"type": "direct", "tag": "direct"}
	if iface := getDefaultInterface(); iface != "" {
		directOutbound["bind_interface"] = iface
	}
	outbounds = append(outbounds, directOutbound, map[string]interface{}{"type": "block", "tag": "block"})

	route, err := a.buildRoute(plan, geo, tagMap, defaultTag)
	if err != nil {
		return nil, err
	}
	dnsConfig := a.buildDNS(plan.ProxyConfig, defaultTag)
	logConfig := a.buildLog(plan.ProxyConfig)
	services := a.buildServices(plan.ProxyConfig)

	config := map[string]interface{}{
		"log":       logConfig,
		"inbounds":  inbounds,
		"outbounds": outbounds,
		"route":     route,
		"dns":       dnsConfig,
	}

	if len(services) > 0 {
		config["services"] = services
	}

	experimental := a.buildExperimental(plan.ProxyConfig)
	if len(experimental) > 0 {
		config["experimental"] = experimental
	}

	return json.MarshalIndent(config, "", "  ")
}

func (a *SingBoxAdapter) buildMeasurementConfig(plan nodegroup.RuntimePlan, geo GeoFiles) ([]byte, error) {
	if plan.InboundPort <= 0 {
		return nil, fmt.Errorf("measurement plan missing inbound port")
	}

	outbounds, tagMap, err := a.buildOutbounds(plan, geo)
	if err != nil {
		return nil, err
	}

	defaultTag, err := singboxOutboundTag(plan.Compiled.Default, tagMap)
	if err != nil {
		return nil, err
	}

	outbounds = append(outbounds,
		map[string]interface{}{"type": "direct", "tag": "direct"},
		map[string]interface{}{"type": "block", "tag": "block"},
	)

	inbounds := []map[string]interface{}{
		{
			"type":        "socks",
			"tag":         "socks-in",
			"listen":      "127.0.0.1",
			"listen_port": plan.InboundPort,
		},
	}

	route, err := a.buildRoute(plan, geo, tagMap, defaultTag)
	if err != nil {
		return nil, err
	}

	dns := a.buildDNS(plan.ProxyConfig, defaultTag)

	config := map[string]interface{}{
		"log": map[string]interface{}{
			"level":     "debug",
			"timestamp": true,
		},
		"inbounds":  inbounds,
		"outbounds": outbounds,
		"route":     route,
		"dns":       dns,
	}

	return json.MarshalIndent(config, "", "  ")
}

// RequiresPrivileges 检查是否需要特权（TUN 模式需要）
func (a *SingBoxAdapter) RequiresPrivileges(profile domain.ProxyConfig) bool {
	return profile.InboundMode == domain.InboundTUN
}

// buildInbounds 构建入站配置
func (a *SingBoxAdapter) buildInbounds(profile domain.ProxyConfig) ([]map[string]interface{}, error) {
	var inbounds []map[string]interface{}

	switch profile.InboundMode {
	case domain.InboundTUN:
		if profile.TUNSettings == nil {
			return nil, fmt.Errorf("TUN mode requires TUNSettings")
		}

		// sing-box 1.12+ 使用新的地址字段格式
		stack := profile.TUNSettings.Stack
		if stack == "" {
			stack = "mixed"
		}
		tun := map[string]interface{}{
			"type":                       "tun",
			"tag":                        "tun-in",
			"interface_name":             profile.TUNSettings.InterfaceName,
			"mtu":                        profile.TUNSettings.MTU,
			"address":                    profile.TUNSettings.Address, // 新格式：address 替代 inet4_address
			"auto_route":                 profile.TUNSettings.AutoRoute,
			"strict_route":               profile.TUNSettings.StrictRoute,
			"stack":                      stack,
			"sniff":                      true,
			"sniff_override_destination": false,
		}

		if runtime.GOOS == "linux" && profile.TUNSettings.AutoRedirect {
			tun["auto_redirect"] = true
		}
		if profile.TUNSettings.EndpointIndependentNat {
			tun["endpoint_independent_nat"] = true
		}
		if profile.TUNSettings.UDPTimeout > 0 {
			tun["udp_timeout"] = formatSingBoxDurationSeconds(profile.TUNSettings.UDPTimeout)
		}
		if len(profile.TUNSettings.RouteAddress) > 0 {
			tun["route_address"] = profile.TUNSettings.RouteAddress
		}
		if len(profile.TUNSettings.RouteExcludeAddress) > 0 {
			tun["route_exclude_address"] = profile.TUNSettings.RouteExcludeAddress
		}

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

func (a *SingBoxAdapter) buildOutbounds(plan nodegroup.RuntimePlan, geo GeoFiles) ([]map[string]interface{}, map[string]string, error) {
	tagMap := make(map[string]string, len(plan.Nodes))
	for _, node := range plan.Nodes {
		tagMap[node.ID] = fmt.Sprintf("node-%s", shortenID(node.ID))
	}

	outbounds := make([]map[string]interface{}, 0, len(plan.Nodes)+2)

	for _, node := range plan.Nodes {
		outbound, tag := a.buildOutbound(node, geo)
		if outbound == nil {
			return nil, nil, fmt.Errorf("build outbound failed: %s", node.ID)
		}

		// detour chaining
		if upstreamID := plan.Compiled.DetourUpstream[node.ID]; upstreamID != "" {
			upstreamTag, ok := tagMap[upstreamID]
			if !ok {
				return nil, nil, fmt.Errorf("detour upstream node not found: %s", upstreamID)
			}
			outbound["detour"] = upstreamTag
		}

		outbounds = append(outbounds, outbound)
		tagMap[node.ID] = tag
	}

	return outbounds, tagMap, nil
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

func (a *SingBoxAdapter) buildRoute(plan nodegroup.RuntimePlan, geo GeoFiles, tagMap map[string]string, defaultTag string) (map[string]interface{}, error) {
	ruleSetManager := NewRuleSetManager(geo.ArtifactsDir)
	// buildDNS() 会引用 geosite-cn；这里必须声明对应 rule-set，否则 sing-box 会在运行期报错。
	ruleSetManager.AddGeoSite("cn")

	rules := make([]map[string]interface{}, 0, len(plan.Compiled.Rules)+4)

	// DNS hijack（TUN 可选）：避免 DNS 泄漏；关闭时让 DNS 流量走普通路由。
	hijackDNS := true
	if plan.InboundMode == domain.InboundTUN && plan.ProxyConfig.TUNSettings != nil {
		hijackDNS = plan.ProxyConfig.TUNSettings.DNSHijack
	}
	if hijackDNS {
		rules = append(rules, map[string]interface{}{
			"protocol": []string{"dns"},
			"action":   "hijack-dns",
		})
	}

	// TUN + 浏览器：QUIC(UDP/443) 经常在部分链路上表现很差（UDP 不通/丢包/被限速），
	// 表现为“Google 打不开但其他 TCP 应用还能用”。直接拦 QUIC，强制浏览器回落到 TCP/HTTPS。
	if plan.InboundMode == domain.InboundTUN {
		rules = append(rules, map[string]interface{}{
			"protocol": []string{"quic"},
			"outbound": "block",
		})
	}

	// TUN 模式：排除代理进程，避免流量循环（自保规则，不属于用户路由语义）
	if plan.InboundMode == domain.InboundTUN {
		rules = append(rules, map[string]interface{}{
			"process_name": []string{"sing-box", "xray", "v2ray"},
			"outbound":     "direct",
		})
	}

	for _, r := range plan.Compiled.Rules {
		outbound, err := singboxOutboundTag(r.Action, tagMap)
		if err != nil {
			return nil, fmt.Errorf("edge %s: %w", r.EdgeID, err)
		}
		entry, err := ruleSetManager.ConvertRouteMatchRule(&r.Match, outbound)
		if err != nil {
			return nil, fmt.Errorf("edge %s: %w", r.EdgeID, err)
		}
		rules = append(rules, entry.ToSingBoxRule())
	}

	route := map[string]interface{}{
		"rules":                   rules,
		"final":                   defaultTag,
		"auto_detect_interface":   true,
		"default_domain_resolver": "dns-local",
	}
	if ruleSets := ruleSetManager.GetRuleSets(); len(ruleSets) > 0 {
		route["rule_set"] = ruleSets
	}

	if plan.InboundMode == domain.InboundTUN {
		if iface := getDefaultInterface(); iface != "" {
			route["default_interface"] = iface
		}
	}

	return route, nil
}

func singboxOutboundTag(action nodegroup.Action, tagMap map[string]string) (string, error) {
	switch action.Kind {
	case nodegroup.ActionDirect:
		return "direct", nil
	case nodegroup.ActionBlock:
		return "block", nil
	case nodegroup.ActionNode:
		tag, ok := tagMap[action.NodeID]
		if !ok {
			return "", fmt.Errorf("node outbound not found: %s", action.NodeID)
		}
		return tag, nil
	default:
		return "", fmt.Errorf("unknown action: %s", action.Kind)
	}
}

// buildDNS 构建 DNS 配置
// 使用分流策略：中国域名走国内 DNS（直连），国际域名走远程 DNS（代理）
// 参考官方文档: https://sing-box.sagernet.org/configuration/dns/
func (a *SingBoxAdapter) buildDNS(profile domain.ProxyConfig, defaultTag string) map[string]interface{} {
	strategy := "prefer_ipv4"
	if profile.DNSConfig != nil && profile.DNSConfig.Strategy != "" {
		strategy = profile.DNSConfig.Strategy
	}

	// 创建规则集管理器（仅用于获取 geosite-cn 标签）
	ruleSetManager := NewRuleSetManager("")
	geositeCN := ruleSetManager.AddGeoSite("cn")

	// DNS 服务器配置 (sing-box 1.12.0+ 新格式)
	// 1. dns-local: 国内 DNS，用于解析国内域名（直连）
	// 2. dns-remote: 国际 DNS，用于解析国际域名（走代理）
	servers := []map[string]interface{}{
		{
			"tag":    "dns-local",
			"type":   "udp",
			"server": "223.5.5.5", // 阿里 DNS，国内访问快
		},
	}
	dnsRemote := map[string]interface{}{
		"tag": "dns-remote",
		// 重要：默认用 TCP，避免很多代理链路/插件不支持 UDP 导致外网域名解析卡死。
		"type":   "tcp",
		"server": "8.8.8.8", // Google DNS，走代理
	}
	// sing-box 会在运行期拒绝「DNS server detour 指向一个空的 direct outbound」。
	// 当 defaultTag=direct 且 direct outbound 没有 dialer 参数时，显式 detour="direct" 会直接 FATAL。
	// 因此：仅在 defaultTag 不是 direct 时才显式写 detour。
	if defaultTag != "" && defaultTag != "direct" {
		dnsRemote["detour"] = defaultTag
	}
	servers = append(servers, dnsRemote)

	// DNS 规则：中国域名走国内 DNS，其他走远程 DNS
	rules := []map[string]interface{}{
		{
			"rule_set": []string{geositeCN},
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
func (a *SingBoxAdapter) applyInboundConfig(inbound map[string]interface{}, profile domain.ProxyConfig) {
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
func (a *SingBoxAdapter) buildLog(profile domain.ProxyConfig) map[string]interface{} {
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
func (a *SingBoxAdapter) buildServices(profile domain.ProxyConfig) []map[string]interface{} {
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
func (a *SingBoxAdapter) buildExperimental(profile domain.ProxyConfig) map[string]interface{} {
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

// ========== 新增 CoreAdapter 接口方法实现 ==========

// SupportsProtocol 检查是否支持特定协议
func (a *SingBoxAdapter) SupportsProtocol(protocol domain.NodeProtocol) bool {
	for _, p := range a.SupportedProtocols() {
		if p == protocol {
			return true
		}
	}
	return false
}

// GetCommandArgs 返回启动 sing-box 的命令行参数
func (a *SingBoxAdapter) GetCommandArgs(configPath string) []string {
	return []string{"run", "-c", configPath}
}

// Start 启动 sing-box 进程
func (a *SingBoxAdapter) Start(cfg ProcessConfig, configPath string) (*ProcessHandle, error) {
	args := a.GetCommandArgs(configPath)
	command := cfg.BinaryPath
	commandArgs := args
	if cfg.UsePkexec {
		if _, err := exec.LookPath("pkexec"); err == nil {
			command = "pkexec"
			commandArgs = append([]string{"--keep-cwd", cfg.BinaryPath}, args...)
		}
	}

	cmd := exec.Command(command, commandArgs...)
	cmd.Dir = cfg.ConfigDir
	if cfg.Stdout != nil {
		cmd.Stdout = cfg.Stdout
	} else {
		cmd.Stdout = os.Stdout
	}
	if cfg.Stderr != nil {
		cmd.Stderr = cfg.Stderr
	} else {
		cmd.Stderr = os.Stderr
	}

	// 设置环境变量
	cmd.Env = mergeEnv(os.Environ(), cfg.Environment)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动 sing-box 失败: %w", err)
	}

	return &ProcessHandle{
		Cmd:        cmd,
		ConfigPath: configPath,
		BinaryPath: cfg.BinaryPath,
		StartedAt:  time.Now(),
		Port:       0,
	}, nil
}

// Stop 停止 sing-box 进程
func (a *SingBoxAdapter) Stop(handle *ProcessHandle) error {
	if handle == nil || handle.Cmd == nil || handle.Cmd.Process == nil {
		return nil
	}

	// 重要：不要直接 SIGKILL。
	// sing-box(TUN/auto_route) 需要在退出时清理 ip rule/route 等系统状态；
	// SIGKILL 会留下残留规则，导致下一次启动报：
	//   "configure tun interface: device or resource busy"
	if runtime.GOOS == "windows" {
		_ = handle.Cmd.Process.Kill()
	} else {
		_ = handle.Cmd.Process.Signal(syscall.SIGTERM)
	}

	// 等待进程退出（如果外部已有 waiter，就等 Done，避免重复 Wait）
	if handle.Done != nil {
		select {
		case <-handle.Done:
		case <-time.After(10 * time.Second):
		}
		return nil
	}

	// 兜底：如果没有 Done，直接 Wait（并在超时后强杀）
	exited := make(chan struct{})
	go func() {
		_ = handle.Cmd.Wait()
		close(exited)
	}()
	select {
	case <-exited:
		return nil
	case <-time.After(10 * time.Second):
	}

	_ = handle.Cmd.Process.Kill()
	<-exited
	return nil
}

// WaitForReady 等待 sing-box 就绪（检测端口监听）
func (a *SingBoxAdapter) WaitForReady(handle *ProcessHandle, timeout time.Duration) error {
	if handle.Port <= 0 {
		// 没有指定端口，只等待一小段时间让进程启动
		time.Sleep(500 * time.Millisecond)
		return nil
	}

	deadline := time.Now().Add(timeout)
	addr := fmt.Sprintf("127.0.0.1:%d", handle.Port)

	for time.Now().Before(deadline) {
		// 不用 Dial 做 readiness probe：Dial 会在 debug 日志下制造一条“inbound connection + EOF”的噪音。
		// 用 Listen 探测端口是否已被占用即可（占用 = 有进程监听）。
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			if errors.Is(err, syscall.EADDRINUSE) {
				return nil
			}
		} else {
			_ = ln.Close()
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("等待 sing-box 就绪超时（端口 %d）", handle.Port)
}
