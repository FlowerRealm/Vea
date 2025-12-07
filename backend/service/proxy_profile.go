package service

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"vea/backend/domain"
	"vea/backend/service/adapters"
)

const (
	// defaultTUNSOCKSPort 两层代理架构中 SOCKS 层的默认端口
	defaultTUNSOCKSPort = 46331
	// defaultTUNInterface 默认 TUN 接口名称
	defaultTUNInterface = "tun0"
	// configFileMode 配置文件权限（包含敏感信息，仅所有者可读写）
	configFileMode = 0600
)

// ProxyProfile CRUD methods

func (s *Service) ListProxyProfiles() []domain.ProxyProfile {
	return s.store.ListProxyProfiles()
}

func (s *Service) CreateProxyProfile(profile domain.ProxyProfile) (domain.ProxyProfile, error) {
	// 验证入站模式
	if profile.InboundMode == "" {
		profile.InboundMode = domain.InboundSOCKS
	}

	// TUN 模式强制 sing-box
	if profile.InboundMode == domain.InboundTUN {
		profile.PreferredEngine = domain.EngineSingBox
		profile.ActualEngine = domain.EngineSingBox
	}

	// 设置默认值
	if profile.PreferredEngine == "" {
		profile.PreferredEngine = domain.EngineAuto
	}

	// 设置默认 TUN 配置
	if profile.InboundMode == domain.InboundTUN && profile.TUNSettings == nil {
		profile.TUNSettings = &domain.TUNConfiguration{
			InterfaceName:          "tun0",
			MTU:                    9000,
			Address:                []string{"198.18.0.1/30"}, // 使用 IANA 保留地址段，避免冲突
			AutoRoute:              true,
			AutoRedirect:           false, // Linux: 默认禁用，可手动启用
			StrictRoute:            true,
			Stack:                  "mixed",
			DNSHijack:              false, // 不劫持 DNS 路由，通过 sniff 处理
			EndpointIndependentNat: false, // gvisor: 默认禁用
			UDPTimeout:             300,   // 5 分钟
		}
	}

	// 禁用 Resolved Service（太复杂，需要 DBUS 权限）
	if profile.InboundMode == domain.InboundTUN && profile.ResolvedService == nil {
		profile.ResolvedService = &domain.ResolvedServiceConfiguration{
			Enabled:    false, // 禁用，使用传统 DNS 劫持 + sniff
			Listen:     "127.0.0.53",
			ListenPort: 53,
		}
	}

	// 设置默认 DNS 配置
	if profile.DNSConfig == nil {
		useResolved := false
		// TUN 模式需要特殊 DNS 配置（address_resolver）
		if profile.InboundMode == domain.InboundTUN {
			useResolved = true
		}

		profile.DNSConfig = &domain.DNSConfiguration{
			UseResolved:            useResolved,
			AcceptDefaultResolvers: false,
			RemoteServers:          []string{"1.1.1.1", "8.8.8.8"}, // 使用 UDP DNS，避免 DoH 兼容性问题
			Strategy:               "prefer_ipv4",
		}
	}

	// 设置默认端口
	if profile.InboundPort == 0 && profile.InboundMode != domain.InboundTUN {
		profile.InboundPort = 38087
	}

	// 设置默认 Inbound 配置
	if profile.InboundMode != domain.InboundTUN && profile.InboundConfig == nil {
		profile.InboundConfig = &domain.InboundConfiguration{
			Listen:         "127.0.0.1",
			AllowLAN:       false,
			Sniff:          true,
			SniffOverride:  true,
			SetSystemProxy: false,
		}
	}

	// 设置默认日志配置
	if profile.LogConfig == nil {
		profile.LogConfig = &domain.LogConfiguration{
			Level:     "info",
			Timestamp: true,
			Output:    "stdout",
		}
	}

	// 设置默认性能配置
	if profile.PerformanceConfig == nil {
		profile.PerformanceConfig = &domain.PerformanceConfiguration{
			TCPFastOpen:    false, // 默认禁用，避免兼容性问题
			TCPMultiPath:   false, // 默认禁用 MPTCP
			UDPFragment:    false, // 默认禁用 UDP 分片
			UDPTimeout:     300,   // 5 分钟
			Sniff:          true,
			SniffOverride:  true,
			DomainStrategy: "prefer_ipv4",
			DomainMatcher:  "hybrid", // 平衡性能和内存
		}
	}

	// 设置默认 Xray 配置
	if profile.XrayConfig == nil {
		profile.XrayConfig = &domain.XrayConfiguration{
			MuxEnabled:     false, // 默认禁用多路复用
			MuxConcurrency: 8,     // 默认并发数
			DNSServers:     []string{"1.1.1.1", "8.8.8.8"},
			DomainStrategy: "AsIs", // 默认不解析域名
		}
	}

	return s.store.CreateProxyProfile(profile), nil
}

func (s *Service) GetProxyProfile(id string) (domain.ProxyProfile, error) {
	return s.store.GetProxyProfile(id)
}

func (s *Service) UpdateProxyProfile(id string, updates domain.ProxyProfile) (domain.ProxyProfile, error) {
	return s.store.UpdateProxyProfile(id, func(current domain.ProxyProfile) (domain.ProxyProfile, error) {
		// 保留 ID 和时间戳
		updates.ID = current.ID
		updates.CreatedAt = current.CreatedAt

		// TUN 模式强制 sing-box
		if updates.InboundMode == domain.InboundTUN {
			updates.PreferredEngine = domain.EngineSingBox
			updates.ActualEngine = domain.EngineSingBox
		}

		return updates, nil
	})
}

func (s *Service) DeleteProxyProfile(id string) error {
	// 如果删除的是活跃 Profile，先停止代理
	if s.store.GetActiveProfile() == id {
		if err := s.StopProxy(); err != nil {
			return fmt.Errorf("failed to stop proxy before deletion: %w", err)
		}
	}
	return s.store.DeleteProxyProfile(id)
}

// StartProxyWithProfile 使用指定 Profile 启动代理
func (s *Service) StartProxyWithProfile(profileID string) error {
	s.proxyMu.Lock()
	defer s.proxyMu.Unlock()

	log.Printf("[Proxy] StartProxyWithProfile: %s", profileID)

	// 停止现有代理
	if s.proxyCmd != nil && s.proxyCmd.Process != nil {
		log.Printf("[Proxy] 停止现有代理进程")
		_ = s.proxyCmd.Process.Kill()
		s.proxyCmd = nil
	}

	// 获取 Profile
	profile, err := s.store.GetProxyProfile(profileID)
	if err != nil {
		return fmt.Errorf("profile not found: %w", err)
	}
	log.Printf("[Proxy] Profile: %s, InboundMode: %s, Engine: %s", profile.Name, profile.InboundMode, profile.PreferredEngine)

	// 获取默认节点
	log.Printf("[Proxy] Profile DefaultNode: %s", profile.DefaultNode)
	if profile.DefaultNode == "" {
		nodes := s.store.ListNodes()
		log.Printf("[Proxy] 没有指定节点，自动选择，当前节点数: %d", len(nodes))
		if len(nodes) == 0 {
			return fmt.Errorf("no nodes available")
		}
		profile.DefaultNode = nodes[0].ID
		log.Printf("[Proxy] 自动选择节点: %s", profile.DefaultNode)
	}

	node, err := s.store.GetNode(profile.DefaultNode)
	if err != nil {
		log.Printf("[Proxy] 获取节点失败: %v, DefaultNode ID: %s", err, profile.DefaultNode)
		return fmt.Errorf("default node not found: %w", err)
	}
	log.Printf("[Proxy] 使用节点: %s (%s:%d)", node.Name, node.Address, node.Port)

	// 检查是否需要 v2ray-plugin（shadowsocks 节点且配置了 plugin）
	if node.Protocol == domain.ProtocolShadowsocks && node.Security != nil && node.Security.Plugin != "" {
		log.Printf("[Proxy] 检测到 Shadowsocks 节点使用插件: %s", node.Security.Plugin)

		// 对于 v2ray-plugin 或 obfs-local（会被转换为 v2ray-plugin），确保已安装
		if node.Security.Plugin == "v2ray-plugin" || node.Security.Plugin == "obfs-local" {
			installed, err := s.ensureV2RayPlugin()
			if err != nil && !installed {
				return fmt.Errorf("v2ray-plugin 未安装: %w", err)
			}
			if installed {
				log.Printf("[Proxy] v2ray-plugin 已就绪")
			}
		}
	}

	// 自动选择引擎
	engine, err := s.SelectEngine(profile, node)
	if err != nil {
		return fmt.Errorf("engine selection failed: %w", err)
	}

	// 基于已安装 sing-box 版本决定是否使用新式 DNS action
	if engine == domain.EngineSingBox {
		if comp, compErr := s.getComponentByKind(domain.ComponentSingBox); compErr == nil {
			s.singBoxDNSActionSupported = singboxSupportsDNSAction(comp.LastVersion)
		}
	}
	log.Printf("[Proxy] 选择引擎: %s", engine)

	// 更新 Profile 的实际引擎
	profile.ActualEngine = engine
	_, _ = s.store.UpdateProxyProfile(profileID, func(p domain.ProxyProfile) (domain.ProxyProfile, error) {
		p.ActualEngine = engine
		return p, nil
	})

	// TUN 模式权限检查和自动配置
	if profile.InboundMode == domain.InboundTUN {
		// 使用新的自动配置方法，会检查并自动修复缺少的 capabilities
		_, err := s.EnsureTUNCapabilities()
		if err != nil {
			return fmt.Errorf("TUN 权限配置失败: %w", err)
		}
		log.Printf("[Proxy] TUN 权限检查通过")
	}

	// 获取适配器
	adapter := s.adapters[engine]
	if adapter == nil {
		return fmt.Errorf("adapter for engine %s not found", engine)
	}
	if sbAdapter, ok := adapter.(*adapters.SingBoxAdapter); ok {
		sbAdapter.SupportsDNSAction = s.singBoxDNSActionSupported
	}

	// 获取引擎二进制路径
	binaryPath, err := s.getEngineBinaryPath(engine)
	if err != nil {
		return fmt.Errorf("engine binary not found: %w", err)
	}
	log.Printf("[Proxy] 二进制路径: %s", binaryPath)

	// 准备 Geo 文件（Sing-box 使用 rule-set，不需要 Geo 文件）
	var geo adapters.GeoFiles
	if engine == domain.EngineXray {
		var err error
		geo, err = s.prepareGeoFiles()
		if err != nil {
			return fmt.Errorf("failed to prepare geo files: %w", err)
		}
		log.Printf("[Proxy] Geo 文件: IP=%s, Site=%s", geo.GeoIP, geo.GeoSite)
	} else {
		// Sing-box 使用 rule-set，不需要 Geo 文件，但仍需要 ArtifactsDir 用于插件路径
		geo = adapters.GeoFiles{
			ArtifactsDir: artifactsRoot,
		}
		log.Printf("[Proxy] Sing-box 使用 rule-set，跳过 Geo 文件检查")
	}

	// 生成配置
	nodes := s.filterNodesForEngine(s.store.ListNodes(), engine)
	log.Printf("[Proxy] 过滤后节点数: %d", len(nodes))
	configBytes, err := adapter.BuildConfig(profile, nodes, geo)
	if err != nil {
		return fmt.Errorf("failed to build config: %w", err)
	}
	log.Printf("[Proxy] 配置生成成功, 大小: %d bytes", len(configBytes))

	// 写入配置文件
	configDir := filepath.Join(artifactsRoot, "core", string(engine))
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}
	configPath := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(configPath, configBytes, configFileMode); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	log.Printf("[Proxy] 配置写入: %s", configPath)

	// 同时保存到 data 目录，方便用户排查问题
	debugConfigPath := filepath.Join("data", fmt.Sprintf("%s-config.json", engine))
	_ = os.MkdirAll("data", 0755)
	if err := os.WriteFile(debugConfigPath, configBytes, configFileMode); err != nil {
		log.Printf("[Proxy] 保存调试配置失败: %v", err)
	} else {
		log.Printf("[Proxy] 调试配置保存至: %s", debugConfigPath)
	}
	logConfigBytes(configBytes)

	// 启动进程
	var cmd *exec.Cmd
	if profile.InboundMode == domain.InboundTUN {
		// 两层代理架构（v2rayN 风格）：
		// 1. 先启动 Xray/sing-box 提供 SOCKS 入站
		// 2. 再启动 sing-box TUN，proxy 指向本地 SOCKS
		log.Printf("[Proxy] 以 TUN 模式启动（两层代理架构）...")

		// 第一层：启动代理核心，提供 SOCKS 入站
		socksPort := defaultTUNSOCKSPort
		socksProfile := profile
		socksProfile.InboundMode = domain.InboundSOCKS
		socksProfile.InboundPort = socksPort

		socksConfigBytes, err := adapter.BuildConfig(socksProfile, nodes, geo)
		if err != nil {
			return fmt.Errorf("failed to build SOCKS config: %w", err)
		}
		socksConfigPath := filepath.Join(configDir, "socks-config.json")
		if err := os.WriteFile(socksConfigPath, socksConfigBytes, configFileMode); err != nil {
			return fmt.Errorf("failed to write SOCKS config: %w", err)
		}
		log.Printf("[Proxy] SOCKS 配置写入: %s", socksConfigPath)

		pluginsDir := filepath.Join(artifactsRoot, "plugins", "v2ray-plugin")
		currentPath := os.Getenv("PATH")
		newPath := pluginsDir + ":" + currentPath

		socksCmd := exec.Command(binaryPath, "run", "-c", socksConfigPath)
		socksCmd.Env = append(os.Environ(), "ENABLE_DEPRECATED_SPECIAL_OUTBOUNDS=true", "PATH="+newPath)

		// 捕获 SOCKS 层的 stderr
		socksPipeR, socksPipeW, _ := os.Pipe()
		socksCmd.Stderr = socksPipeW

		if err := socksCmd.Start(); err != nil {
			return fmt.Errorf("failed to start SOCKS proxy: %w", err)
		}
		socksPipeW.Close()

		// 异步读取 SOCKS 层日志
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := socksPipeR.Read(buf)
				if n > 0 {
					log.Printf("[socks-layer] %s", string(buf[:n]))
				}
				if err != nil {
					break
				}
			}
		}()

		log.Printf("[Proxy] SOCKS 代理启动成功, PID: %d, 端口: %d", socksCmd.Process.Pid, socksPort)
		s.socksCmd = socksCmd

		// 等待 SOCKS 服务就绪并验证
		time.Sleep(500 * time.Millisecond)

		// 检查 SOCKS 进程是否还在运行
		if socksCmd.ProcessState != nil && socksCmd.ProcessState.Exited() {
			return fmt.Errorf("SOCKS proxy exited unexpectedly, check if port %d is already in use", socksPort)
		}

		// 第二层：启动 TUN，proxy 指向本地 SOCKS
		sbAdapter := adapter.(*adapters.SingBoxAdapter)
		tunConfigBytes, err := sbAdapter.BuildTUNOnlyConfig(socksPort, geo)
		if err != nil {
			socksCmd.Process.Kill()
			return fmt.Errorf("failed to build TUN config: %w", err)
		}
		tunConfigPath := filepath.Join(configDir, "tun-config.json")
		if err := os.WriteFile(tunConfigPath, tunConfigBytes, configFileMode); err != nil {
			socksCmd.Process.Kill()
			return fmt.Errorf("failed to write TUN config: %w", err)
		}
		log.Printf("[Proxy] TUN 配置写入: %s", tunConfigPath)

		// 获取 sing-box 路径（TUN 必须用 sing-box）
		singboxPath, err := s.getEngineBinaryPath(domain.EngineSingBox)
		if err != nil {
			socksCmd.Process.Kill()
			return fmt.Errorf("sing-box not found for TUN: %w", err)
		}

		cmd, err = s.StartTUNProcess(singboxPath, tunConfigPath)
		if err != nil {
			socksCmd.Process.Kill()
			return fmt.Errorf("failed to start TUN: %w", err)
		}
	} else {
		log.Printf("[Proxy] 以普通模式启动进程...")
		cmd = exec.Command(binaryPath, "run", "-c", configPath)
		// 设置环境变量：把 plugins 目录添加到 PATH，让 sing-box 能找到 v2ray-plugin 等插件
		pluginsDir := filepath.Join(artifactsRoot, "plugins", "v2ray-plugin")
		currentPath := os.Getenv("PATH")
		newPath := pluginsDir + ":" + currentPath
		cmd.Env = append(os.Environ(),
			"ENABLE_DEPRECATED_SPECIAL_OUTBOUNDS=true",
			"PATH="+newPath,
		)
		err = cmd.Start()
	}

	if err != nil {
		return fmt.Errorf("failed to start proxy: %w", err)
	}

	log.Printf("[Proxy] 进程启动成功, PID: %d", cmd.Process.Pid)
	s.proxyCmd = cmd
	s.activeProfile = profileID
	_ = s.store.SetActiveProfile(profileID)
	go s.monitorProxyProcess(cmd, profileID, profile.InboundMode)

	return nil
}

// StopProxy 停止当前运行的代理
func (s *Service) StopProxy() error {
	s.proxyMu.Lock()
	defer s.proxyMu.Unlock()

	if s.proxyCmd == nil || s.proxyCmd.Process == nil {
		return nil
	}

	// 获取当前 profile 信息（用于清理）
	var profile domain.ProxyProfile
	if s.activeProfile != "" {
		if p, err := s.store.GetProxyProfile(s.activeProfile); err == nil {
			profile = p
		}
	}

	// 停止进程
	if err := s.proxyCmd.Process.Kill(); err != nil {
		log.Printf("[Proxy] 停止进程失败: %v", err)
	}

	// 停止 SOCKS 进程（两层代理模式）
	if s.socksCmd != nil && s.socksCmd.Process != nil {
		if err := s.socksCmd.Process.Kill(); err != nil {
			log.Printf("[Proxy] 停止 SOCKS 进程失败: %v", err)
		}
		s.socksCmd = nil
	}

	s.proxyCmd = nil
	s.activeProfile = ""
	_ = s.store.SetActiveProfile("")

	// 清理系统代理设置
	if err := configureSystemProxy(false, "", 0, nil); err != nil {
		log.Printf("[Proxy] 清理系统代理失败: %v", err)
	} else {
		log.Printf("[Proxy] 系统代理已清理")
	}

	// 清理 TUN 接口和路由
	if profile.InboundMode == domain.InboundTUN {
		tunInterface := defaultTUNInterface
		if profile.TUNSettings != nil && profile.TUNSettings.InterfaceName != "" {
			tunInterface = profile.TUNSettings.InterfaceName
		}
		if err := s.cleanupTUN(tunInterface); err != nil {
			log.Printf("[Proxy] 清理 TUN 失败: %v", err)
		}
	}

	log.Printf("[Proxy] 代理已停止")
	return nil
}

func (s *Service) monitorProxyProcess(cmd *exec.Cmd, profileID string, inboundMode domain.InboundMode) {
	err := cmd.Wait()
	if err != nil {
		log.Printf("[Proxy] core process exited with error: %v", err)
	} else {
		log.Printf("[Proxy] core process exited normally")
	}

	clearActive := false
	s.proxyMu.Lock()
	if s.proxyCmd == cmd {
		s.proxyCmd = nil
		if s.activeProfile == profileID {
			s.activeProfile = ""
			clearActive = true
		}
	}
	s.proxyMu.Unlock()

	if clearActive {
		_ = s.store.SetActiveProfile("")

		// 清理系统代理
		if err := configureSystemProxy(false, "", 0, nil); err != nil {
			log.Printf("[Proxy] 进程退出后清理系统代理失败: %v", err)
		} else {
			log.Printf("[Proxy] 进程退出后系统代理已清理")
		}

		if inboundMode == domain.InboundTUN {
			// 清理 TUN
			tunInterface := defaultTUNInterface
			if profile, err := s.store.GetProxyProfile(profileID); err == nil && profile.TUNSettings != nil && profile.TUNSettings.InterfaceName != "" {
				tunInterface = profile.TUNSettings.InterfaceName
			}
			if err := s.cleanupTUN(tunInterface); err != nil {
				log.Printf("[Proxy] 进程退出后清理 TUN 失败: %v", err)
			}

			if _, tunErr := s.store.UpdateTUNSettings(func(t domain.TUNSettings) (domain.TUNSettings, error) {
				t.Enabled = false
				return t, nil
			}); tunErr != nil {
				log.Printf("[Proxy] failed to mark TUN disabled after exit: %v", tunErr)
			}
		}
	}
}

// GetProxyStatus 获取代理状态
func (s *Service) GetProxyStatus() map[string]interface{} {
	s.proxyMu.Lock()
	defer s.proxyMu.Unlock()

	status := map[string]interface{}{
		"running":       s.proxyCmd != nil && s.proxyCmd.Process != nil,
		"activeProfile": s.activeProfile,
	}

	if s.proxyCmd != nil && s.proxyCmd.Process != nil {
		status["pid"] = s.proxyCmd.Process.Pid
	}

	if s.activeProfile != "" {
		if profile, err := s.store.GetProxyProfile(s.activeProfile); err == nil {
			status["inboundMode"] = profile.InboundMode
			status["engine"] = profile.ActualEngine
			status["inboundPort"] = profile.InboundPort
		}
	}

	return status
}

func logConfigBytes(config []byte) {
	const maxLogBytes = 16 * 1024
	if len(config) == 0 {
		log.Printf("[Proxy] 配置为空")
		return
	}
	if len(config) > maxLogBytes {
		log.Printf("[Proxy] 配置内容（前 %d/%d bytes）:\n%s\n[Proxy] ...已截断", maxLogBytes, len(config), string(config[:maxLogBytes]))
		return
	}
	log.Printf("[Proxy] 配置内容:\n%s", string(config))
}

// filterNodesForEngine 过滤出引擎支持的节点
func (s *Service) filterNodesForEngine(nodes []domain.Node, engine domain.CoreEngineKind) []domain.Node {
	adapter := s.adapters[engine]
	if adapter == nil {
		return nodes
	}

	supportedProtocols := adapter.SupportedProtocols()
	protocolMap := make(map[domain.NodeProtocol]bool)
	for _, p := range supportedProtocols {
		protocolMap[p] = true
	}

	var filtered []domain.Node
	for _, node := range nodes {
		if protocolMap[node.Protocol] {
			filtered = append(filtered, node)
		}
	}

	return filtered
}

// prepareGeoFiles 准备 Geo 文件
func (s *Service) prepareGeoFiles() (adapters.GeoFiles, error) {
	geoDir := filepath.Join(artifactsRoot, "geo")

	geo := adapters.GeoFiles{
		GeoIP:        filepath.Join(geoDir, "geoip.dat"),
		GeoSite:      filepath.Join(geoDir, "geosite.dat"),
		ArtifactsDir: artifactsRoot, // 传递 artifacts 目录绝对路径
	}

	// 尝试从 GeoResource 获取
	for _, res := range s.store.ListGeo() {
		if res.Type == domain.GeoIP && res.ArtifactPath != "" {
			geo.GeoIP = res.ArtifactPath
		}
		if res.Type == domain.GeoSite && res.ArtifactPath != "" {
			geo.GeoSite = res.ArtifactPath
		}
	}

	// 检查文件是否存在
	if _, err := os.Stat(geo.GeoIP); os.IsNotExist(err) {
		return geo, fmt.Errorf("GeoIP 文件不存在，请先在「组件」面板安装 Geo 数据")
	}
	if _, err := os.Stat(geo.GeoSite); os.IsNotExist(err) {
		return geo, fmt.Errorf("GeoSite 文件不存在，请先在「组件」面板安装 Geo 数据")
	}

	return geo, nil
}

// ensureV2RayPlugin 检查并自动安装 v2ray-plugin（如果需要）
// 返回: (是否已安装或成功安装, 错误信息)
func (s *Service) ensureV2RayPlugin() (bool, error) {
	// 检查是否已存在 v2ray-plugin 组件配置
	var pluginComp domain.CoreComponent
	components := s.store.ListComponents()
	found := false
	for _, comp := range components {
		if comp.ID == "v2ray-plugin" {
			pluginComp = comp
			found = true
			break
		}
	}

	// 如果组件存在且安装完成，检查实际文件
	if found && pluginComp.InstallStatus == domain.InstallStatusDone && pluginComp.InstallDir != "" {
		// 检查安装目录下是否有可执行文件
		files, err := filepath.Glob(filepath.Join(pluginComp.InstallDir, "*"))
		if err == nil && len(files) > 0 {
			// 检查是否有可执行文件（通过扩展名或权限判断）
			for _, f := range files {
				info, err := os.Stat(f)
				if err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
					log.Printf("[V2Ray-Plugin] 已安装: %s", f)
					return true, nil
				}
			}
		}
	}

	log.Printf("[V2Ray-Plugin] 未安装，开始自动安装...")
	if found {
		log.Printf("[V2Ray-Plugin] 组件状态: InstallStatus=%s, InstallDir=%s", pluginComp.InstallStatus, pluginComp.InstallDir)
	}

	// 如果不存在，创建组件配置
	if !found {
		pluginComp = domain.CoreComponent{
			Kind: domain.ComponentGeneric,
			ID:   "v2ray-plugin",
			Name: "v2ray-plugin",
		}
		var err error
		pluginComp, err = s.CreateComponent(pluginComp)
		if err != nil {
			return false, fmt.Errorf("创建 v2ray-plugin 组件失败: %w", err)
		}
		log.Printf("[V2Ray-Plugin] 创建组件配置: %s", pluginComp.ID)
	}

	// 触发安装
	log.Printf("[V2Ray-Plugin] 开始安装组件...")
	_, err := s.InstallComponent(pluginComp.ID)
	if err != nil {
		return false, fmt.Errorf("安装 v2ray-plugin 失败: %w", err)
	}

	// 等待安装完成（最多等待 60 秒）
	// 注意：实际环境中应该使用更优雅的等待机制，这里简化处理
	// 由于 InstallComponent 是异步的，我们只是触发安装，不等待完成
	log.Printf("[V2Ray-Plugin] 安装已触发，请稍后在组件面板查看安装进度")

	return false, fmt.Errorf("v2ray-plugin 安装已触发，请等待安装完成后重试")
}
