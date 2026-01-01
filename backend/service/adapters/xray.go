package adapters

import (
	"encoding/json"
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

// BuildConfig 根据运行计划生成 Xray 配置
func (a *XrayAdapter) BuildConfig(plan nodegroup.RuntimePlan, geo GeoFiles) ([]byte, error) {
	switch plan.Purpose {
	case nodegroup.PurposeProxy:
		return a.buildProxyConfig(plan, geo)
	case nodegroup.PurposeMeasurement:
		return a.buildMeasurementConfig(plan, geo)
	default:
		return nil, fmt.Errorf("unsupported plan purpose: %s", plan.Purpose)
	}
}

func (a *XrayAdapter) buildProxyConfig(plan nodegroup.RuntimePlan, geo GeoFiles) ([]byte, error) {
	outbounds, tagMap, err := a.buildOutbounds(plan)
	if err != nil {
		return nil, err
	}

	outbounds = append(outbounds,
		map[string]interface{}{"tag": xrayDirectTag, "protocol": "freedom"},
		map[string]interface{}{
			"tag":      xrayBlockTag,
			"protocol": "blackhole",
			"settings": map[string]interface{}{
				"response": map[string]interface{}{
					"type": "http",
				},
			},
		},
	)

	routing, err := a.buildRouting(plan.Compiled, tagMap, geo, plan.ProxyConfig.XrayConfig)
	if err != nil {
		return nil, err
	}

	config := map[string]interface{}{
		"log":       a.buildLog(plan.ProxyConfig),
		"inbounds":  a.buildInbounds(plan.ProxyConfig),
		"outbounds": outbounds,
		"routing":   routing,
		"dns":       a.buildDNS(plan.ProxyConfig),
	}

	return json.MarshalIndent(config, "", "  ")
}

func (a *XrayAdapter) buildMeasurementConfig(plan nodegroup.RuntimePlan, geo GeoFiles) ([]byte, error) {
	if plan.InboundPort <= 0 {
		return nil, fmt.Errorf("measurement plan missing inbound port")
	}

	outbounds, tagMap, err := a.buildOutbounds(plan)
	if err != nil {
		return nil, err
	}

	outbounds = append(outbounds,
		map[string]interface{}{"tag": xrayDirectTag, "protocol": "freedom"},
		map[string]interface{}{
			"tag":      xrayBlockTag,
			"protocol": "blackhole",
			"settings": map[string]interface{}{
				"response": map[string]interface{}{
					"type": "http",
				},
			},
		},
	)

	routing, err := a.buildRouting(plan.Compiled, tagMap, geo, nil)
	if err != nil {
		return nil, err
	}

	inbounds := []map[string]interface{}{
		{
			"tag":      "socks-in",
			"listen":   "127.0.0.1",
			"port":     plan.InboundPort,
			"protocol": "socks",
			"settings": map[string]interface{}{
				"auth": "noauth",
				"udp":  true,
			},
		},
	}

	config := map[string]interface{}{
		"log": map[string]interface{}{
			"loglevel": "debug",
		},
		"inbounds":  inbounds,
		"outbounds": outbounds,
		"routing":   routing,
	}

	return json.MarshalIndent(config, "", "  ")
}

// RequiresPrivileges Xray 不支持 TUN，所以永远不需要特权
func (a *XrayAdapter) RequiresPrivileges(profile domain.ProxyConfig) bool {
	return false
}

// buildInbounds 构建入站配置
func (a *XrayAdapter) buildInbounds(profile domain.ProxyConfig) []map[string]interface{} {
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

func (a *XrayAdapter) buildOutbounds(plan nodegroup.RuntimePlan) ([]map[string]interface{}, map[string]string, error) {
	tagMap := make(map[string]string, len(plan.Nodes))
	for _, node := range plan.Nodes {
		tagMap[node.ID] = fmt.Sprintf("node-%s", shortenID(node.ID))
	}

	outbounds := make([]map[string]interface{}, 0, len(plan.Nodes)+2)
	for _, node := range plan.Nodes {
		outbound, tag, err := a.buildOutbound(node)
		if err != nil {
			return nil, nil, fmt.Errorf("build node outbound %s: %w", node.ID, err)
		}

		// 应用 Mux 配置
		if plan.ProxyConfig.XrayConfig != nil {
			outbound["mux"] = map[string]interface{}{
				"enabled":     plan.ProxyConfig.XrayConfig.MuxEnabled,
				"concurrency": plan.ProxyConfig.XrayConfig.MuxConcurrency,
			}
		} else {
			outbound["mux"] = map[string]interface{}{
				"enabled":     false,
				"concurrency": 8,
			}
		}

		// detour chaining
		if upstreamID := plan.Compiled.DetourUpstream[node.ID]; upstreamID != "" {
			upstreamTag, ok := tagMap[upstreamID]
			if !ok {
				return nil, nil, fmt.Errorf("detour upstream node not found: %s", upstreamID)
			}
			outbound["proxySettings"] = map[string]interface{}{
				"tag": upstreamTag,
			}
		}

		outbounds = append(outbounds, outbound)
		tagMap[node.ID] = tag
	}

	return outbounds, tagMap, nil
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

func (a *XrayAdapter) buildRouting(compiled nodegroup.CompiledFRouter, tagMap map[string]string, geo GeoFiles, xrayConfig *domain.XrayConfiguration) (map[string]interface{}, error) {
	rules := make([]map[string]interface{}, 0, len(compiled.Rules)+1)
	for _, r := range compiled.Rules {
		outboundTag, err := xrayOutboundTag(r.Action, tagMap)
		if err != nil {
			return nil, fmt.Errorf("edge %s: %w", r.EdgeID, err)
		}
		rule := map[string]interface{}{
			"type":        "field",
			"outboundTag": outboundTag,
		}
		if len(r.Match.Domains) > 0 {
			rule["domain"] = r.Match.Domains
		}
		if len(r.Match.IPs) > 0 {
			rule["ip"] = r.Match.IPs
		}
		rules = append(rules, rule)
	}

	// 默认规则（广告拦截 + 私有/国内直连），放在用户规则之后。
	rules = append(rules,
		map[string]interface{}{
			"type":        "field",
			"domain":      []string{"geosite:category-ads-all"},
			"outboundTag": xrayBlockTag,
		},
		map[string]interface{}{
			"type":        "field",
			"domain":      []string{"geosite:cn", "geosite:private"},
			"outboundTag": xrayDirectTag,
		},
		map[string]interface{}{
			"type":        "field",
			"ip":          []string{"geoip:cn", "geoip:private"},
			"outboundTag": xrayDirectTag,
		},
	)

	defaultTag, err := xrayOutboundTag(compiled.Default, tagMap)
	if err != nil {
		return nil, err
	}
	rules = append(rules, map[string]interface{}{
		"type":        "field",
		"network":     "tcp,udp",
		"outboundTag": defaultTag,
	})

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
	return routing, nil
}

func xrayOutboundTag(action nodegroup.Action, tagMap map[string]string) (string, error) {
	switch action.Kind {
	case nodegroup.ActionDirect:
		return xrayDirectTag, nil
	case nodegroup.ActionBlock:
		return xrayBlockTag, nil
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
func (a *XrayAdapter) buildDNS(profile domain.ProxyConfig) map[string]interface{} {
	servers := []string{"1.1.1.1", "8.8.8.8"}
	if profile.XrayConfig != nil && len(profile.XrayConfig.DNSServers) > 0 {
		servers = profile.XrayConfig.DNSServers
	}
	return map[string]interface{}{
		"servers": servers,
	}
}

// buildLog 构建日志配置
func (a *XrayAdapter) buildLog(profile domain.ProxyConfig) map[string]interface{} {
	logLevel := "info"
	if profile.LogConfig != nil && profile.LogConfig.Level != "" {
		logLevel = profile.LogConfig.Level
	}
	return map[string]interface{}{
		"loglevel": logLevel,
	}
}

// ========== 新增接口方法实现 ==========

// SupportsProtocol 检查是否支持特定协议
func (a *XrayAdapter) SupportsProtocol(protocol domain.NodeProtocol) bool {
	for _, p := range a.SupportedProtocols() {
		if p == protocol {
			return true
		}
	}
	return false
}

// GetCommandArgs 返回启动 Xray 的命令行参数
func (a *XrayAdapter) GetCommandArgs(configPath string) []string {
	// Xray CLI 使用 `run -c <config>`（与集成测试/现有调用保持一致）。
	return []string{"run", "-c", configPath}
}

// Start 启动 Xray 进程
func (a *XrayAdapter) Start(cfg ProcessConfig, configPath string) (*ProcessHandle, error) {
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

	// 设置环境变量（确保 cfg.Environment 覆盖默认环境）
	env := mergeEnv(os.Environ(), cfg.Environment)
	// 将二进制所在目录添加到 PATH（用于加载插件）
	binDir := filepath.Dir(cfg.BinaryPath)
	env = prependPath(env, binDir)
	cmd.Env = env

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动 Xray 失败: %w", err)
	}

	return &ProcessHandle{
		Cmd:        cmd,
		ConfigPath: configPath,
		BinaryPath: cfg.BinaryPath,
		StartedAt:  time.Now(),
		Port:       0, // Xray 的端口需要从配置中读取
	}, nil
}

// Stop 停止 Xray 进程
func (a *XrayAdapter) Stop(handle *ProcessHandle) error {
	if handle == nil || handle.Cmd == nil || handle.Cmd.Process == nil {
		return nil
	}

	// 优先优雅退出，避免留下半写入的状态文件/日志。
	//（TUN 主要由 sing-box 承担，但这里统一行为，简单且可预测。）
	if runtime.GOOS == "windows" {
		_ = handle.Cmd.Process.Kill()
	} else {
		_ = handle.Cmd.Process.Signal(syscall.SIGTERM)
	}

	// 等待进程退出（如果外部已有 waiter，就等 Done，避免重复 Wait）
	if handle.Done != nil {
		select {
		case <-handle.Done:
		case <-time.After(5 * time.Second):
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
	case <-time.After(5 * time.Second):
	}

	_ = handle.Cmd.Process.Kill()
	<-exited
	return nil
}

// WaitForReady 等待 Xray 就绪（检测端口监听）
func (a *XrayAdapter) WaitForReady(handle *ProcessHandle, timeout time.Duration) error {
	if handle.Port <= 0 {
		// 没有指定端口，只等待一小段时间让进程启动
		time.Sleep(500 * time.Millisecond)
		return nil
	}

	deadline := time.Now().Add(timeout)
	addr := fmt.Sprintf("127.0.0.1:%d", handle.Port)

	for time.Now().Before(deadline) {
		// 不用 Dial 做 readiness probe：Dial 会制造一次“入站连接后立刻断开”，在 debug 日志下很吵。
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

	return fmt.Errorf("等待 Xray 就绪超时（端口 %d）", handle.Port)
}

// prependPath 将目录添加到 PATH 环境变量的前面
func prependPath(env []string, dir string) []string {
	for i, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			env[i] = "PATH=" + dir + string(os.PathListSeparator) + e[5:]
			return env
		}
	}
	return append(env, "PATH="+dir)
}
