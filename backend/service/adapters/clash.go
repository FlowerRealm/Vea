package adapters

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"vea/backend/domain"
	"vea/backend/service/nodegroup"
	"vea/backend/service/shared"

	"gopkg.in/yaml.v3"
)

// ClashAdapter mihomo(Clash.Meta) 适配器
//
// 约定：
// - EngineKind 对外仍使用 "clash"，但安装默认指向 mihomo（Clash.Meta 继任）。
// - 配置使用 YAML；文件名扩展名不重要（由启动参数指定）。
type ClashAdapter struct{}

func (a *ClashAdapter) Kind() domain.CoreEngineKind {
	return domain.EngineClash
}

func (a *ClashAdapter) BinaryNames() []string {
	return []string{"mihomo", "mihomo.exe", "clash", "clash.exe"}
}

func (a *ClashAdapter) SupportedProtocols() []domain.NodeProtocol {
	return []domain.NodeProtocol{
		domain.ProtocolVLESS,
		domain.ProtocolVMess,
		domain.ProtocolTrojan,
		domain.ProtocolShadowsocks,
		domain.ProtocolHysteria2,
		domain.ProtocolTUIC,
	}
}

func (a *ClashAdapter) SupportsProtocol(protocol domain.NodeProtocol) bool {
	for _, p := range a.SupportedProtocols() {
		if p == protocol {
			return true
		}
	}
	return false
}

func (a *ClashAdapter) SupportsInbound(mode domain.InboundMode) bool {
	// mihomo 支持 socks/http/mixed + tun
	return true
}

func (a *ClashAdapter) BuildConfig(plan nodegroup.RuntimePlan, geo GeoFiles) ([]byte, error) {
	switch plan.Purpose {
	case nodegroup.PurposeProxy:
		return a.buildProxyConfig(plan, geo)
	case nodegroup.PurposeMeasurement:
		return a.buildMeasurementConfig(plan, geo)
	default:
		return nil, fmt.Errorf("unsupported plan purpose: %s", plan.Purpose)
	}
}

func (a *ClashAdapter) buildProxyConfig(plan nodegroup.RuntimePlan, geo GeoFiles) ([]byte, error) {
	cfg, err := a.buildBaseConfig(plan)
	if err != nil {
		return nil, err
	}

	a.applyInbound(cfg, plan.ProxyConfig, plan.InboundMode, plan.InboundPort)
	a.applyDNS(cfg, plan.ProxyConfig, plan.InboundMode)

	proxies, tagMap, err := a.buildProxies(plan)
	if err != nil {
		return nil, err
	}
	cfg["proxies"] = proxies

	rules, err := a.buildRules(plan.InboundMode, plan.Compiled, tagMap)
	if err != nil {
		return nil, err
	}
	cfg["rules"] = rules

	return yaml.Marshal(cfg)
}

func (a *ClashAdapter) buildMeasurementConfig(plan nodegroup.RuntimePlan, geo GeoFiles) ([]byte, error) {
	if plan.InboundPort <= 0 {
		return nil, fmt.Errorf("measurement plan missing inbound port")
	}

	cfg, err := a.buildBaseConfig(plan)
	if err != nil {
		return nil, err
	}

	// 测量只需要本地 socks 入站；不跑 TUN/系统代理。
	a.applyInbound(cfg, domain.ProxyConfig{}, domain.InboundSOCKS, plan.InboundPort)

	proxies, tagMap, err := a.buildProxies(plan)
	if err != nil {
		return nil, err
	}
	cfg["proxies"] = proxies

	rules, err := a.buildRules(plan.InboundMode, plan.Compiled, tagMap)
	if err != nil {
		return nil, err
	}
	cfg["rules"] = rules

	return yaml.Marshal(cfg)
}

func (a *ClashAdapter) buildBaseConfig(plan nodegroup.RuntimePlan) (map[string]interface{}, error) {
	logLevel := "info"
	if plan.ProxyConfig.LogConfig != nil && strings.TrimSpace(plan.ProxyConfig.LogConfig.Level) != "" {
		switch strings.ToLower(strings.TrimSpace(plan.ProxyConfig.LogConfig.Level)) {
		case "debug":
			logLevel = "debug"
		case "info":
			logLevel = "info"
		case "warning", "warn":
			logLevel = "warning"
		case "error":
			logLevel = "error"
		case "none", "silent":
			logLevel = "silent"
		}
	}

	// 为了支持 geosite:/geoip: 规则，保持 geox-url 存在但关闭自动更新。
	cfg := map[string]interface{}{
		"mode":      "rule",
		"log-level": logLevel,
		"geox-url": map[string]string{
			"geoip":   "https://fastly.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geoip.dat",
			"geosite": "https://fastly.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geosite.dat",
			"mmdb":    "https://fastly.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geoip.metadb",
		},
		"geo-auto-update":     false,
		"geo-update-interval": 24,
	}

	// TUN 默认开启 sniffer：避免浏览器/系统启用 DoH 后，域名规则无法命中导致“看起来可启动但无法正常分流/访问”。
	if plan.InboundMode == domain.InboundTUN {
		cfg["sniffer"] = map[string]interface{}{
			"enable": true,
			"sniff": map[string]interface{}{
				"TLS": map[string]interface{}{
					"ports": []int{443},
				},
				"HTTP": map[string]interface{}{
					"ports": []int{80},
				},
			},
		}
	}

	// find-process-mode 决定 PROCESS-NAME 规则是否可用。
	// 主流 GUI（如 Clash Party）默认使用 strict；否则我们添加的自保规则会形同虚设。
	if runtime.GOOS == "linux" && plan.InboundMode == domain.InboundTUN {
		cfg["find-process-mode"] = "strict"
	}

	// Linux + TUN: 设置 routing-mark 以避免内核自身出站流量被 auto-route 回环进 TUN，导致“全网断开”。
	// mihomo 文档建议在 Linux 上为出站连接设置默认 mark。见官方 General 配置说明。
	if runtime.GOOS == "linux" && plan.InboundMode == domain.InboundTUN {
		cfg["routing-mark"] = 6666
		cfg["profile"] = map[string]interface{}{
			"store-selected": true,
			"store-fake-ip":  true,
		}
	}

	return cfg, nil
}

func (a *ClashAdapter) applyInbound(cfg map[string]interface{}, profile domain.ProxyConfig, mode domain.InboundMode, port int) {
	if cfg == nil {
		return
	}

	// bind-address / allow-lan（clash 是全局设置）
	bindAddr := "127.0.0.1"
	allowLan := false
	var auth *domain.InboundAuthentication
	if profile.InboundConfig != nil {
		host := strings.TrimSpace(profile.InboundConfig.Listen)
		allowLan = profile.InboundConfig.AllowLAN
		if host != "" {
			// allowLan 的语义是“允许局域网连接”，即在默认 loopback 监听时改为监听全网卡。
			// 若用户显式配置了非 loopback 地址，则保持用户配置。
			if allowLan && (host == "127.0.0.1" || host == "localhost") {
				bindAddr = "0.0.0.0"
			} else if allowLan && host == "::1" {
				bindAddr = "::"
			} else {
				bindAddr = host
			}
		} else if allowLan {
			bindAddr = "0.0.0.0"
		}
		auth = profile.InboundConfig.Authentication
	}
	cfg["bind-address"] = bindAddr
	cfg["allow-lan"] = allowLan

	if auth != nil && strings.TrimSpace(auth.Username) != "" && strings.TrimSpace(auth.Password) != "" {
		cfg["authentication"] = []string{strings.TrimSpace(auth.Username) + ":" + strings.TrimSpace(auth.Password)}
	}

	switch mode {
	case domain.InboundMixed:
		if port > 0 {
			cfg["mixed-port"] = port
		}
	case domain.InboundSOCKS:
		if port > 0 {
			cfg["socks-port"] = port
		}
	case domain.InboundHTTP:
		if port > 0 {
			cfg["port"] = port
		}
	case domain.InboundTUN:
		// TUN 模式下仍然提供本地 mixed 入站端口（HTTP + SOCKS），便于系统代理/手动代理共存。
		if port > 0 {
			cfg["mixed-port"] = port
		}
		cfg["tun"] = a.buildTUN(profile)
	}
}

func (a *ClashAdapter) buildTUN(profile domain.ProxyConfig) map[string]interface{} {
	tun := map[string]interface{}{
		"enable": true,
		"stack":  "mixed",
	}

	if profile.TUNSettings == nil {
		// 主流 GUI（如 Clash Party）在 Linux 上倾向于把 TUN MTU 固定为 1500，避免某些网络/防火墙环境下
		// 因 PMTU/分片策略导致的“看起来全网断开”。
		if runtime.GOOS == "linux" {
			tun["mtu"] = 1500
		}
		return tun
	}

	// 统一接口名：与 ProxyConfig.TUNSettings.InterfaceName 对齐，避免后端无法判断 TUN 是否就绪。
	// Windows/macOS 下默认值（vea / legacy tun0）不强制写死 device，交给内核自动选择实际名称。
	if interfaceName := strings.TrimSpace(profile.TUNSettings.InterfaceName); interfaceName != "" {
		if runtime.GOOS != "linux" && (interfaceName == "vea" || interfaceName == "tun0") {
			// no-op
		} else {
			tun["device"] = interfaceName
		}
	}

	// mihomo 的 tun 默认地址/网段就是 198.18.0.1/30。
	// 实测：在部分版本里自定义 inet4-address 并不会生效（仍会回落到默认值），反而会造成“配置显示 172.19，但实际跑 198.18”的误判。
	// 所以这里直接与 mihomo 的默认保持一致，避免自找麻烦。
	tun["inet4-address"] = []string{"198.18.0.1/30"}

	stack := strings.TrimSpace(profile.TUNSettings.Stack)
	if stack != "" {
		tun["stack"] = stack
	}
	if profile.TUNSettings.MTU > 0 {
		tun["mtu"] = profile.TUNSettings.MTU
	} else if runtime.GOOS == "linux" {
		tun["mtu"] = 1500
	}
	if profile.TUNSettings.AutoRoute {
		tun["auto-route"] = true
		// auto-route 基本都需要自动识别默认物理网卡；否则很容易把自己路由回 TUN 里导致断网。
		tun["auto-detect-interface"] = true
	}
	if runtime.GOOS == "linux" && profile.TUNSettings.AutoRedirect {
		tun["auto-redirect"] = true
	}
	// strict-route 会在路由/排除规则不完整时造成“全网断开”。
	// 在 Vea 的默认配置里，RouteExcludeAddress 往往为空；mihomo 下直接开 strict-route 是在赌运气。
	// 这里仅在用户显式提供了排除地址时才开启，避免自残。
	if profile.TUNSettings.StrictRoute && len(profile.TUNSettings.RouteExcludeAddress) > 0 {
		tun["strict-route"] = true
	}
	if profile.TUNSettings.EndpointIndependentNat {
		tun["endpoint-independent-nat"] = true
	}
	if len(profile.TUNSettings.RouteAddress) > 0 {
		tun["route-address"] = profile.TUNSettings.RouteAddress
	}
	if len(profile.TUNSettings.RouteExcludeAddress) > 0 {
		tun["route-exclude-address"] = profile.TUNSettings.RouteExcludeAddress
	}
	if profile.TUNSettings.DNSHijack {
		// mihomo/clash 的常见做法是劫持 DNS 目的端口 53（UDP/TCP），并把本机 DNS 服务监听到非 53 端口（避免与系统 DNS 冲突）。
		// 参考：多数公开配置为 dns-hijack any:53 + dns.listen :1053。
		tun["dns-hijack"] = []string{"any:53", "tcp://any:53"}
	}
	return tun
}

func (a *ClashAdapter) applyDNS(cfg map[string]interface{}, profile domain.ProxyConfig, mode domain.InboundMode) {
	if cfg == nil {
		return
	}
	if mode != domain.InboundTUN {
		return
	}
	cfg["dns"] = a.buildDNS(profile)
}

func (a *ClashAdapter) buildDNS(profile domain.ProxyConfig) map[string]interface{} {
	// TUN + dns-hijack 若不启用 Clash DNS，会导致系统 DNS 被劫持但无人响应，表现为“全网断开”。
	// 参考主流客户端（如 Clash Party）的默认做法：
	// - nameserver 使用 DoH（更抗污染），default-nameserver 用 IP（解决 DoH 域名自举）
	// - 允许用户传入 scheme（https/tls/quic/dhcp 等），不要粗暴过滤 "://"
	nameserver := []string{}
	if profile.DNSConfig != nil {
		for _, s := range profile.DNSConfig.RemoteServers {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			nameserver = append(nameserver, s)
		}
	}
	if len(nameserver) == 0 {
		nameserver = []string{"https://doh.pub/dns-query", "https://dns.alidns.com/dns-query"}
	}

	// 用作 DNS server 域名自举/代理服务器域名解析的“直连解析器”；仅放 IP（可带 scheme，但避免域名）。
	bootstrap := []string{"223.5.5.5", "119.29.29.29", "1.1.1.1", "8.8.8.8"}

	ipv6 := false
	if profile.DNSConfig != nil {
		switch strings.ToLower(strings.TrimSpace(profile.DNSConfig.Strategy)) {
		case "prefer_ipv6", "ipv6_only":
			ipv6 = true
		}
	}

	dns := map[string]interface{}{
		"enable": true,
		// 不要占用 53 端口（systemd-resolved 常见占用 127.0.0.53:53）。
		"listen":        "0.0.0.0:1053",
		"ipv6":          ipv6,
		"enhanced-mode": "fake-ip",
		"fake-ip-range": "198.18.0.1/16",
		// 避免局域网/本地域名被 fake-ip 破坏（主流配置都会带这一类 filter）。
		"fake-ip-filter":     []string{"+.lan", "+.local", "time.*.com", "ntp.*.com"},
		"nameserver":         nameserver,
		"default-nameserver": bootstrap,
		// 关键：代理服务器域名解析必须走“直连 DNS”，否则非常容易出现 bootstrap 死锁（DNS 要走代理，但代理又需要 DNS 才能连上）。
		"proxy-server-nameserver": bootstrap,
	}
	// direct-nameserver 仅接受 IP：用同一组 bootstrap 即可。
	dns["direct-nameserver"] = bootstrap
	return dns
}

func (a *ClashAdapter) buildProxies(plan nodegroup.RuntimePlan) ([]map[string]interface{}, map[string]string, error) {
	tagMap := make(map[string]string, len(plan.Nodes))
	for _, node := range plan.Nodes {
		tagMap[node.ID] = fmt.Sprintf("node-%s", shortenID(node.ID))
	}

	proxies := make([]map[string]interface{}, 0, len(plan.Nodes))
	for _, node := range plan.Nodes {
		p, name, err := a.buildProxy(node)
		if err != nil {
			return nil, nil, fmt.Errorf("build node proxy %s: %w", node.ID, err)
		}

		// detour chaining（如果上游存在，使用 dialer-proxy）
		if upstreamID := plan.Compiled.DetourUpstream[node.ID]; strings.TrimSpace(upstreamID) != "" {
			upstreamName, ok := tagMap[upstreamID]
			if !ok {
				return nil, nil, fmt.Errorf("detour upstream node not found: %s", upstreamID)
			}
			p["dialer-proxy"] = upstreamName
		}

		proxies = append(proxies, p)
		tagMap[node.ID] = name
	}

	return proxies, tagMap, nil
}

func (a *ClashAdapter) buildProxy(node domain.Node) (map[string]interface{}, string, error) {
	name := fmt.Sprintf("node-%s", shortenID(node.ID))
	p := map[string]interface{}{
		"name":   name,
		"server": node.Address,
		"port":   node.Port,
		"udp":    true,
	}

	sec := node.Security
	if sec == nil {
		sec = &domain.NodeSecurity{}
	}

	switch node.Protocol {
	case domain.ProtocolVLESS:
		p["type"] = "vless"
		p["uuid"] = sec.UUID
		p["network"] = "tcp"
		if strings.TrimSpace(sec.Flow) != "" {
			p["flow"] = strings.TrimSpace(sec.Flow)
		}
	case domain.ProtocolVMess:
		p["type"] = "vmess"
		p["uuid"] = sec.UUID
		p["alterId"] = sec.AlterID
		cipher := strings.TrimSpace(sec.Encryption)
		if cipher == "" {
			cipher = "auto"
		}
		p["cipher"] = cipher
	case domain.ProtocolTrojan:
		p["type"] = "trojan"
		p["password"] = sec.Password
	case domain.ProtocolShadowsocks:
		p["type"] = "ss"
		p["cipher"] = sec.Method
		p["password"] = sec.Password
		if strings.TrimSpace(sec.Plugin) != "" {
			plugin, opts := normalizeSSPlugin(sec.Plugin, sec.PluginOpts)
			if plugin != "" {
				p["plugin"] = plugin
				if len(opts) > 0 {
					p["plugin-opts"] = opts
				}
			}
		}
	case domain.ProtocolHysteria2:
		p["type"] = "hysteria2"
		p["password"] = sec.Password
	case domain.ProtocolTUIC:
		p["type"] = "tuic"
		p["uuid"] = sec.UUID
		p["password"] = sec.Password
		// 这些字段在多数场景下能提高可用性；保持“可运行”的默认，不做过度配置化。
		p["reduce-rtt"] = true
		p["request-timeout"] = 8000
		p["udp-relay-mode"] = "native"
	default:
		return nil, "", fmt.Errorf("unsupported protocol: %s", node.Protocol)
	}

	a.applyTransport(p, node.Transport)
	a.applyTLS(p, node.Protocol, node.TLS)

	return p, name, nil
}

func (a *ClashAdapter) applyTransport(p map[string]interface{}, transport *domain.NodeTransport) {
	if p == nil || transport == nil {
		return
	}

	switch strings.TrimSpace(transport.Type) {
	case "ws":
		p["network"] = "ws"
		ws := map[string]interface{}{}
		if strings.TrimSpace(transport.Path) != "" {
			ws["path"] = transport.Path
		}
		headers := map[string]string{}
		if strings.TrimSpace(transport.Host) != "" {
			headers["Host"] = transport.Host
		}
		for k, v := range transport.Headers {
			k = strings.TrimSpace(k)
			if k == "" {
				continue
			}
			headers[k] = v
		}
		if len(headers) > 0 {
			ws["headers"] = headers
		}
		if len(ws) > 0 {
			p["ws-opts"] = ws
		}

	case "grpc":
		p["network"] = "grpc"
		if strings.TrimSpace(transport.ServiceName) != "" {
			p["grpc-opts"] = map[string]interface{}{
				"grpc-service-name": transport.ServiceName,
			}
		}

	case "h2":
		p["network"] = "h2"
		h2 := map[string]interface{}{}
		if strings.TrimSpace(transport.Host) != "" {
			h2["host"] = []string{transport.Host}
		}
		if strings.TrimSpace(transport.Path) != "" {
			h2["path"] = transport.Path
		}
		if len(h2) > 0 {
			p["h2-opts"] = h2
		}

	case "http":
		p["network"] = "http"
		httpOpts := map[string]interface{}{}
		if strings.TrimSpace(transport.Path) != "" {
			httpOpts["path"] = []string{transport.Path}
		}
		headers := map[string][]string{}
		if strings.TrimSpace(transport.Host) != "" {
			headers["Host"] = []string{transport.Host}
		}
		for k, v := range transport.Headers {
			k = strings.TrimSpace(k)
			if k == "" {
				continue
			}
			headers[k] = []string{v}
		}
		if len(headers) > 0 {
			httpOpts["headers"] = headers
		}
		if len(httpOpts) > 0 {
			p["http-opts"] = httpOpts
		}
	}
}

func (a *ClashAdapter) applyTLS(p map[string]interface{}, protocol domain.NodeProtocol, tls *domain.NodeTLS) {
	if p == nil || tls == nil || !tls.Enabled {
		return
	}

	// Reality 需要显式字段（vless）
	if strings.TrimSpace(tls.RealityPublicKey) != "" {
		p["reality-opts"] = map[string]interface{}{
			"public-key": strings.TrimSpace(tls.RealityPublicKey),
		}
		if strings.TrimSpace(tls.RealityShortID) != "" {
			p["reality-opts"].(map[string]interface{})["short-id"] = strings.TrimSpace(tls.RealityShortID)
		}
	}

	if strings.TrimSpace(tls.Fingerprint) != "" {
		p["client-fingerprint"] = strings.TrimSpace(tls.Fingerprint)
	}
	if tls.Insecure {
		p["skip-cert-verify"] = true
	}

	switch protocol {
	case domain.ProtocolTrojan, domain.ProtocolHysteria2, domain.ProtocolTUIC:
		if strings.TrimSpace(tls.ServerName) != "" {
			p["sni"] = strings.TrimSpace(tls.ServerName)
		}
	default:
		p["tls"] = true
		if strings.TrimSpace(tls.ServerName) != "" {
			p["servername"] = strings.TrimSpace(tls.ServerName)
		}
	}

	if len(tls.ALPN) > 0 {
		p["alpn"] = append([]string(nil), tls.ALPN...)
	}
}

type routeTarget string

const (
	targetDirect routeTarget = "DIRECT"
	targetReject routeTarget = "REJECT"
)

func parseGeoRule(rule string) (geoType string, tag string, isGeo bool) {
	if strings.HasPrefix(rule, "geosite:") {
		return "geosite", strings.TrimPrefix(rule, "geosite:"), true
	}
	if strings.HasPrefix(rule, "geoip:") {
		return "geoip", strings.TrimPrefix(rule, "geoip:"), true
	}
	return "", "", false
}

func parseDomainRule(rule string) (ruleType string, value string) {
	switch {
	case strings.HasPrefix(rule, "domain:"):
		return "suffix", strings.TrimPrefix(rule, "domain:")
	case strings.HasPrefix(rule, "full:"):
		return "domain", strings.TrimPrefix(rule, "full:")
	case strings.HasPrefix(rule, "regexp:"):
		return "regex", strings.TrimPrefix(rule, "regexp:")
	case strings.HasPrefix(rule, "keyword:"):
		return "keyword", strings.TrimPrefix(rule, "keyword:")
	default:
		return "suffix", rule
	}
}

func (a *ClashAdapter) buildRules(mode domain.InboundMode, compiled nodegroup.CompiledFRouter, tagMap map[string]string) ([]string, error) {
	rules := make([]string, 0, len(compiled.Rules)*4+8)

	// TUN 自保规则：避免 mihomo 自己的外连（DNS/节点握手/订阅）被路由回 TUN 里形成循环。
	// sing-box 有 process_name 直连；clash 用 PROCESS-NAME。
	if mode == domain.InboundTUN {
		rules = append(rules,
			"PROCESS-NAME,mihomo,DIRECT",
			"PROCESS-NAME,clash,DIRECT",
			"PROCESS-NAME,vea,DIRECT",
			// Chrome/Chromium 在 TUN 下常优先 QUIC(UDP/443)，在部分链路/校园网/公司网会表现为“能解析但打不开”。
			// 这里直接拒绝 UDP/443，强制回落到 TCP/HTTPS（与 sing-box 的默认行为保持一致）。
			"AND,((NETWORK,UDP),(DST-PORT,443)),REJECT",
		)
	}

	toTarget := func(action nodegroup.Action) (routeTarget, error) {
		switch action.Kind {
		case nodegroup.ActionDirect:
			return targetDirect, nil
		case nodegroup.ActionBlock:
			return targetReject, nil
		case nodegroup.ActionNode:
			name, ok := tagMap[action.NodeID]
			if !ok || strings.TrimSpace(name) == "" {
				return "", fmt.Errorf("node target not found: %s", action.NodeID)
			}
			return routeTarget(name), nil
		default:
			return "", fmt.Errorf("unsupported action kind: %s", action.Kind)
		}
	}

	for _, rr := range compiled.Rules {
		target, err := toTarget(rr.Action)
		if err != nil {
			return nil, fmt.Errorf("edge %s: %w", rr.EdgeID, err)
		}

		for _, raw := range rr.Match.Domains {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			geoType, tag, isGeo := parseGeoRule(raw)
			if isGeo {
				if geoType == "geosite" {
					rules = append(rules, fmt.Sprintf("GEOSITE,%s,%s", tag, target))
					continue
				}
				return nil, fmt.Errorf("geoip rule must be in IPs, not Domains: %s", raw)
			}

			rt, value := parseDomainRule(raw)
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			switch rt {
			case "domain":
				rules = append(rules, fmt.Sprintf("DOMAIN,%s,%s", value, target))
			case "suffix":
				rules = append(rules, fmt.Sprintf("DOMAIN-SUFFIX,%s,%s", value, target))
			case "keyword":
				rules = append(rules, fmt.Sprintf("DOMAIN-KEYWORD,%s,%s", value, target))
			case "regex":
				rules = append(rules, fmt.Sprintf("DOMAIN-REGEX,%s,%s", value, target))
			default:
				return nil, fmt.Errorf("unsupported domain rule type: %s", rt)
			}
		}

		for _, raw := range rr.Match.IPs {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			geoType, tag, isGeo := parseGeoRule(raw)
			if isGeo {
				if geoType == "geoip" {
					rules = append(rules, fmt.Sprintf("GEOIP,%s,%s", tag, target))
				}
				continue
			}

			ipType, ipValue := normalizeCIDR(raw)
			if ipValue == "" {
				continue
			}
			rules = append(rules, fmt.Sprintf("%s,%s,%s,no-resolve", ipType, ipValue, target))
		}
	}

	// 默认规则（广告拦截 + 私有/国内直连），放在用户规则之后。
	rules = append(rules,
		fmt.Sprintf("GEOSITE,category-ads-all,%s", targetReject),
		fmt.Sprintf("GEOIP,private,%s", targetDirect),
		fmt.Sprintf("GEOSITE,cn,%s", targetDirect),
		fmt.Sprintf("GEOIP,cn,%s", targetDirect),
	)

	defaultTarget, err := toTarget(compiled.Default)
	if err != nil {
		return nil, fmt.Errorf("default: %w", err)
	}
	rules = append(rules, fmt.Sprintf("MATCH,%s", defaultTarget))
	return rules, nil
}

func normalizeCIDR(raw string) (ruleType string, cidr string) {
	ip := strings.TrimSpace(raw)
	if ip == "" {
		return "", ""
	}

	if strings.Contains(ip, "/") {
		if strings.Contains(ip, ":") {
			return "IP-CIDR6", ip
		}
		return "IP-CIDR", ip
	}

	parsed := net.ParseIP(ip)
	if parsed == nil {
		return "", ""
	}
	if parsed.To4() == nil {
		return "IP-CIDR6", ip + "/128"
	}
	return "IP-CIDR", ip + "/32"
}

func normalizeSSPlugin(plugin, opts string) (string, map[string]interface{}) {
	plugin = strings.TrimSpace(plugin)
	opts = strings.TrimSpace(opts)
	if plugin == "" {
		return "", nil
	}

	// ss:// plugin=obfs-local;obfs=http;obfs-host=xxx （订阅常见格式）
	if plugin == "obfs-local" {
		kv := parsePluginOpts(opts)
		out := make(map[string]interface{})
		if v, ok := kv["obfs"]; ok {
			out["mode"] = v
		}
		if v, ok := kv["obfs-host"]; ok {
			out["host"] = v
		}
		return "obfs", out
	}

	return plugin, parsePluginOpts(opts)
}

func parsePluginOpts(opts string) map[string]interface{} {
	kv := shared.ParsePluginOptsString(opts)
	if kv == nil {
		return nil
	}

	out := make(map[string]interface{}, len(kv))
	for k, v := range kv {
		if v == "" {
			out[k] = true
			continue
		}
		out[k] = v
	}
	return out
}

func (a *ClashAdapter) RequiresPrivileges(profile domain.ProxyConfig) bool {
	return profile.InboundMode == domain.InboundTUN
}

func (a *ClashAdapter) GetCommandArgs(configPath string) []string {
	// mihomo/clash:
	// -d <dir> 设置工作目录（GeoSite.dat/GeoIP.dat 等缓存也在这里）
	// -f <config> 指定配置文件
	configDir := filepath.Dir(configPath)
	return []string{"-d", configDir, "-f", configPath}
}

func (a *ClashAdapter) Start(cfg ProcessConfig, configPath string) (*ProcessHandle, error) {
	args := a.GetCommandArgs(configPath)
	cmd := exec.Command(cfg.BinaryPath, args...)
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

	cmd.Env = mergeEnv(os.Environ(), cfg.Environment)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动 clash 失败: %w", err)
	}

	return &ProcessHandle{
		Cmd:        cmd,
		ConfigPath: configPath,
		BinaryPath: cfg.BinaryPath,
		StartedAt:  time.Now(),
		Port:       0,
	}, nil
}

func (a *ClashAdapter) Stop(handle *ProcessHandle) error {
	if handle == nil || handle.Cmd == nil || handle.Cmd.Process == nil {
		return nil
	}

	// TUN(auto-route) 需要退出清理路由；尽量走 SIGTERM。
	if runtime.GOOS == "windows" {
		_ = handle.Cmd.Process.Kill()
	} else {
		_ = handle.Cmd.Process.Signal(syscall.SIGTERM)
	}

	if handle.Done != nil {
		select {
		case <-handle.Done:
		case <-time.After(10 * time.Second):
		}
		return nil
	}

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

func (a *ClashAdapter) WaitForReady(handle *ProcessHandle, timeout time.Duration) error {
	if handle.Port <= 0 {
		time.Sleep(500 * time.Millisecond)
		return nil
	}

	deadline := time.Now().Add(timeout)
	addr := fmt.Sprintf("127.0.0.1:%d", handle.Port)

	for time.Now().Before(deadline) {
		if runtime.GOOS == "windows" {
			conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
			if err == nil {
				_ = conn.Close()
				return nil
			}
		} else {
			ln, err := net.Listen("tcp", addr)
			if err != nil {
				if errors.Is(err, syscall.EADDRINUSE) {
					return nil
				}
			} else {
				_ = ln.Close()
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("等待 clash 就绪超时（端口 %d）", handle.Port)
}
